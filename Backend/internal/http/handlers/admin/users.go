package admin

import (
	"context"
	"errors"
	"strings"
	"time"

	"beeba.org/internal/domain/users"
	"beeba.org/internal/http/middleware/authz"
	"beeba.org/internal/repository/postgres"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type UserStore interface {
	ListAdminUsers(ctx context.Context, filter users.AdminListFilter) ([]users.AdminUser, error)
	BanUser(ctx context.Context, input users.AdminUserActionInput) (users.AdminUser, error)
	UnbanUser(ctx context.Context, input users.AdminUserActionInput) (users.AdminUser, error)
	SetUserRoles(ctx context.Context, input users.AdminUserRolesInput) (users.AdminUser, error)
}

type UsersHandler struct {
	store UserStore
}

func NewUsersHandler(store UserStore) UsersHandler {
	return UsersHandler{store: store}
}

type adminUserReasonRequest struct {
	Reason string `json:"reason"`
}

type adminUserRolesRequest struct {
	Roles []string `json:"roles"`
}

func (h UsersHandler) List(c fiber.Ctx) error {
	if h.store == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "user administration unavailable")
	}
	role := strings.TrimSpace(c.Query("role"))
	if role != "" && !allowedAdminUserRole(role) {
		return fiber.NewError(fiber.StatusBadRequest, "role is invalid")
	}
	limit, err := parseModerationLimit(c.Query("limit"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	ctx, cancel := context.WithTimeout(c.Context(), 2*time.Second)
	defer cancel()

	items, err := h.store.ListAdminUsers(ctx, users.AdminListFilter{
		Query: strings.TrimSpace(c.Query("q")),
		Role:  role,
		Limit: limit,
	})
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to load users")
	}

	return c.JSON(fiber.Map{
		"data": items,
		"pagination": fiber.Map{
			"limit": limit,
		},
	})
}

func (h UsersHandler) Ban(c fiber.Ctx) error {
	return h.userAction(c, "ban")
}

func (h UsersHandler) Unban(c fiber.Ctx) error {
	return h.userAction(c, "unban")
}

func (h UsersHandler) SetRoles(c fiber.Ctx) error {
	if h.store == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "user administration unavailable")
	}
	session, ok := authz.CurrentSession(c)
	if !ok {
		return fiber.NewError(fiber.StatusUnauthorized, "Missing authenticated session")
	}

	userID := strings.TrimSpace(c.Params("userID"))
	if _, err := uuid.Parse(userID); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "user id is invalid")
	}

	var request adminUserRolesRequest
	if err := c.Bind().Body(&request); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}
	roles, err := normalizeAdminRoles(request.Roles)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}
	if containsString(roles, "owner") && !containsString(session.Roles, "owner") {
		return fiber.NewError(fiber.StatusForbidden, "Only owners can grant owner role")
	}

	ctx, cancel := context.WithTimeout(c.Context(), 3*time.Second)
	defer cancel()

	user, err := h.store.SetUserRoles(ctx, users.AdminUserRolesInput{
		ActorUserID:    session.User.ID,
		TargetUserID:   userID,
		Roles:          roles,
		AllowOwnerRole: containsString(session.Roles, "owner"),
		IPAddress:      c.IP(),
		UserAgent:      c.Get("User-Agent"),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return fiber.NewError(fiber.StatusNotFound, "User not found")
	}
	if errors.Is(err, postgres.ErrAdminUserProtected) {
		return fiber.NewError(fiber.StatusForbidden, "User role change is protected")
	}
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to update user roles")
	}

	return c.JSON(fiber.Map{"data": user})
}

func (h UsersHandler) userAction(c fiber.Ctx, action string) error {
	if h.store == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "user administration unavailable")
	}
	session, ok := authz.CurrentSession(c)
	if !ok {
		return fiber.NewError(fiber.StatusUnauthorized, "Missing authenticated session")
	}

	userID := strings.TrimSpace(c.Params("userID"))
	if _, err := uuid.Parse(userID); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "user id is invalid")
	}
	if userID == session.User.ID {
		return fiber.NewError(fiber.StatusConflict, "Cannot change your own ban state")
	}

	var request adminUserReasonRequest
	if err := c.Bind().Body(&request); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}
	reason := strings.TrimSpace(request.Reason)
	if action == "ban" && reason == "" {
		return fiber.NewError(fiber.StatusBadRequest, "reason is required")
	}
	if len(reason) > 500 {
		return fiber.NewError(fiber.StatusBadRequest, "reason must be 500 characters or fewer")
	}

	ctx, cancel := context.WithTimeout(c.Context(), 3*time.Second)
	defer cancel()

	input := users.AdminUserActionInput{
		ActorUserID:  session.User.ID,
		TargetUserID: userID,
		Reason:       reason,
		IPAddress:    c.IP(),
		UserAgent:    c.Get("User-Agent"),
	}

	var user users.AdminUser
	var err error
	if action == "ban" {
		user, err = h.store.BanUser(ctx, input)
	} else {
		user, err = h.store.UnbanUser(ctx, input)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return fiber.NewError(fiber.StatusNotFound, "User not found")
	}
	if errors.Is(err, postgres.ErrAdminUserProtected) {
		return fiber.NewError(fiber.StatusForbidden, "User is protected")
	}
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to update user")
	}

	return c.JSON(fiber.Map{"data": user})
}

func normalizeAdminRoles(raw []string) ([]string, error) {
	seen := map[string]struct{}{}
	roles := make([]string, 0, len(raw)+1)
	for _, role := range raw {
		role = strings.ToLower(strings.TrimSpace(role))
		if role == "" {
			continue
		}
		if !allowedAdminUserRole(role) {
			return nil, errors.New("roles contain an invalid role")
		}
		if _, ok := seen[role]; !ok {
			seen[role] = struct{}{}
			roles = append(roles, role)
		}
	}
	if _, ok := seen["user"]; !ok {
		roles = append([]string{"user"}, roles...)
	}
	return roles, nil
}

func allowedAdminUserRole(role string) bool {
	switch role {
	case "user", "moderator", "admin", "owner":
		return true
	default:
		return false
	}
}

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
