package authz

import (
	"context"
	"crypto/subtle"
	"errors"
	"strings"
	"time"

	authdomain "beeba.org/internal/domain/auth"
	"beeba.org/internal/security/tokens"

	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5"
)

type sessionKeyType struct{}

var sessionKey sessionKeyType

type Store interface {
	FindByAccessTokenHash(ctx context.Context, tokenHash string, now time.Time) (authdomain.SessionWithUser, error)
}

const (
	AccessCookieName = "beeba_access_token"
	CSRFCookieName   = "beeba_csrf_token"
	csrfHeaderName   = "X-CSRF-Token"
)

type tokenSource string

const (
	tokenSourceBearer tokenSource = "bearer"
	tokenSourceCookie tokenSource = "cookie"
)

func RequireBearer(store Store) fiber.Handler {
	return func(c fiber.Ctx) error {
		if store == nil {
			return fiber.NewError(fiber.StatusServiceUnavailable, "auth repository unavailable")
		}

		token, source, ok := tokenFromRequestWithSource(c)
		if !ok {
			return fiber.NewError(fiber.StatusUnauthorized, "Missing authenticated session")
		}
		if source == tokenSourceCookie && methodRequiresCSRF(c.Method()) && !ValidCSRF(c) {
			return fiber.NewError(fiber.StatusForbidden, "Invalid CSRF token")
		}

		ctx, cancel := context.WithTimeout(c.Context(), 2*time.Second)
		defer cancel()

		session, err := store.FindByAccessTokenHash(ctx, tokens.Hash(token), time.Now().UTC())
		if errors.Is(err, pgx.ErrNoRows) {
			return fiber.NewError(fiber.StatusUnauthorized, "Invalid bearer token")
		}
		if err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, "Failed to load session")
		}
		if session.DeletedAt != nil || session.BannedAt != nil {
			return fiber.NewError(fiber.StatusForbidden, "Account is not allowed")
		}

		fiber.Locals[authdomain.SessionWithUser](c, sessionKey, session)
		return c.Next()
	}
}

func TokenFromRequest(c fiber.Ctx) (string, bool) {
	token, _, ok := tokenFromRequestWithSource(c)
	return token, ok
}

func tokenFromRequestWithSource(c fiber.Ctx) (string, tokenSource, bool) {
	if token, ok := bearerToken(c.Get("Authorization")); ok {
		return token, tokenSourceBearer, true
	}
	token := strings.TrimSpace(c.Cookies(AccessCookieName))
	return token, tokenSourceCookie, token != ""
}

func RequireRole(allowed ...string) fiber.Handler {
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, role := range allowed {
		allowedSet[role] = struct{}{}
	}

	return func(c fiber.Ctx) error {
		session, ok := CurrentSession(c)
		if !ok {
			return fiber.NewError(fiber.StatusUnauthorized, "Missing authenticated session")
		}
		for _, role := range session.Roles {
			if _, ok := allowedSet[role]; ok {
				return c.Next()
			}
		}
		return fiber.NewError(fiber.StatusForbidden, "Insufficient role")
	}
}

func CurrentSession(c fiber.Ctx) (authdomain.SessionWithUser, bool) {
	session := fiber.Locals[authdomain.SessionWithUser](c, sessionKey)
	return session, session.SessionID != ""
}

func bearerToken(header string) (string, bool) {
	parts := strings.Fields(strings.TrimSpace(header))
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || strings.TrimSpace(parts[1]) == "" {
		return "", false
	}
	return parts[1], true
}

func methodRequiresCSRF(method string) bool {
	switch strings.ToUpper(method) {
	case fiber.MethodGet, fiber.MethodHead, fiber.MethodOptions, fiber.MethodTrace:
		return false
	default:
		return true
	}
}

func ValidCSRF(c fiber.Ctx) bool {
	cookie := strings.TrimSpace(c.Cookies(CSRFCookieName))
	header := strings.TrimSpace(c.Get(csrfHeaderName))
	if cookie == "" || header == "" || len(cookie) != len(header) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(cookie), []byte(header)) == 1
}
