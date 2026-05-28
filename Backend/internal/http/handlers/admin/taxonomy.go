package admin

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"

	"beeba.org/internal/domain/categories"
	"beeba.org/internal/domain/tags"
	"beeba.org/internal/http/middleware/authz"
	"beeba.org/internal/repository/postgres"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var taxonomySlugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

type CategoryAdminStore interface {
	ListAdminCategories(ctx context.Context) ([]categories.Category, error)
	UpdateAdminCategory(ctx context.Context, actorUserID string, categoryID string, input categories.AdminUpdateInput, ipAddress string, userAgent string) (categories.Category, error)
}

type TagAdminStore interface {
	ListAdminTags(ctx context.Context) ([]tags.Tag, error)
	CreateAdminTag(ctx context.Context, actorUserID string, input tags.AdminCreateInput, ipAddress string, userAgent string) (tags.Tag, error)
	UpdateAdminTag(ctx context.Context, actorUserID string, tagID string, input tags.AdminUpdateInput, ipAddress string, userAgent string) (tags.Tag, error)
}

type TaxonomyHandler struct {
	categories CategoryAdminStore
	tags       TagAdminStore
}

func NewTaxonomyHandler(categories CategoryAdminStore, tags TagAdminStore) TaxonomyHandler {
	return TaxonomyHandler{categories: categories, tags: tags}
}

type categoryUpdateRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	IconKey     *string `json:"icon_key"`
	SortOrder   *int    `json:"sort_order"`
	IsActive    *bool   `json:"is_active"`
}

type tagCreateRequest struct {
	Slug     string `json:"slug"`
	Name     string `json:"name"`
	IsSystem bool   `json:"is_system"`
}

type tagUpdateRequest struct {
	Name     *string `json:"name"`
	IsSystem *bool   `json:"is_system"`
}

func (h TaxonomyHandler) ListCategories(c fiber.Ctx) error {
	if h.categories == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "taxonomy administration unavailable")
	}
	ctx, cancel := context.WithTimeout(c.Context(), 2*time.Second)
	defer cancel()
	items, err := h.categories.ListAdminCategories(ctx)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to load categories")
	}
	return c.JSON(fiber.Map{"data": items})
}

func (h TaxonomyHandler) UpdateCategory(c fiber.Ctx) error {
	if h.categories == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "taxonomy administration unavailable")
	}
	session, ok := authz.CurrentSession(c)
	if !ok {
		return fiber.NewError(fiber.StatusUnauthorized, "Missing authenticated session")
	}
	categoryID := strings.TrimSpace(c.Params("categoryID"))
	if _, err := uuid.Parse(categoryID); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "category id is invalid")
	}
	var request categoryUpdateRequest
	if err := c.Bind().Body(&request); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}
	input, err := normalizeCategoryUpdate(request)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}
	ctx, cancel := context.WithTimeout(c.Context(), 3*time.Second)
	defer cancel()
	item, err := h.categories.UpdateAdminCategory(ctx, session.User.ID, categoryID, input, c.IP(), c.Get("User-Agent"))
	if errors.Is(err, pgx.ErrNoRows) {
		return fiber.NewError(fiber.StatusNotFound, "Category not found")
	}
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to update category")
	}
	return c.JSON(fiber.Map{"data": item})
}

func (h TaxonomyHandler) ListTags(c fiber.Ctx) error {
	if h.tags == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "taxonomy administration unavailable")
	}
	ctx, cancel := context.WithTimeout(c.Context(), 2*time.Second)
	defer cancel()
	items, err := h.tags.ListAdminTags(ctx)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to load tags")
	}
	return c.JSON(fiber.Map{"data": items})
}

func (h TaxonomyHandler) CreateTag(c fiber.Ctx) error {
	if h.tags == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "taxonomy administration unavailable")
	}
	session, ok := authz.CurrentSession(c)
	if !ok {
		return fiber.NewError(fiber.StatusUnauthorized, "Missing authenticated session")
	}
	var request tagCreateRequest
	if err := c.Bind().Body(&request); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}
	input, err := normalizeTagCreate(request)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}
	ctx, cancel := context.WithTimeout(c.Context(), 3*time.Second)
	defer cancel()
	item, err := h.tags.CreateAdminTag(ctx, session.User.ID, input, c.IP(), c.Get("User-Agent"))
	if errors.Is(err, postgres.ErrTagConflict) {
		return fiber.NewError(fiber.StatusConflict, "Tag slug already exists")
	}
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to create tag")
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"data": item})
}

func (h TaxonomyHandler) UpdateTag(c fiber.Ctx) error {
	if h.tags == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "taxonomy administration unavailable")
	}
	session, ok := authz.CurrentSession(c)
	if !ok {
		return fiber.NewError(fiber.StatusUnauthorized, "Missing authenticated session")
	}
	tagID := strings.TrimSpace(c.Params("tagID"))
	if _, err := uuid.Parse(tagID); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "tag id is invalid")
	}
	var request tagUpdateRequest
	if err := c.Bind().Body(&request); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}
	input, err := normalizeTagUpdate(request)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}
	ctx, cancel := context.WithTimeout(c.Context(), 3*time.Second)
	defer cancel()
	item, err := h.tags.UpdateAdminTag(ctx, session.User.ID, tagID, input, c.IP(), c.Get("User-Agent"))
	if errors.Is(err, pgx.ErrNoRows) {
		return fiber.NewError(fiber.StatusNotFound, "Tag not found")
	}
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to update tag")
	}
	return c.JSON(fiber.Map{"data": item})
}

func normalizeCategoryUpdate(request categoryUpdateRequest) (categories.AdminUpdateInput, error) {
	if request.Name != nil {
		name := strings.TrimSpace(*request.Name)
		if len(name) < 2 || len(name) > 80 {
			return categories.AdminUpdateInput{}, errors.New("name must be between 2 and 80 characters")
		}
		request.Name = &name
	}
	if request.Description != nil {
		description := strings.TrimSpace(*request.Description)
		if len(description) > 500 {
			return categories.AdminUpdateInput{}, errors.New("description must be 500 characters or fewer")
		}
		request.Description = &description
	}
	if request.IconKey != nil {
		iconKey := strings.TrimSpace(*request.IconKey)
		if len(iconKey) > 80 {
			return categories.AdminUpdateInput{}, errors.New("icon_key must be 80 characters or fewer")
		}
		request.IconKey = &iconKey
	}
	if request.SortOrder != nil && (*request.SortOrder < 0 || *request.SortOrder > 10000) {
		return categories.AdminUpdateInput{}, errors.New("sort_order must be between 0 and 10000")
	}
	return categories.AdminUpdateInput{
		Name:        request.Name,
		Description: request.Description,
		IconKey:     request.IconKey,
		SortOrder:   request.SortOrder,
		IsActive:    request.IsActive,
	}, nil
}

func normalizeTagCreate(request tagCreateRequest) (tags.AdminCreateInput, error) {
	slug := strings.ToLower(strings.TrimSpace(request.Slug))
	name := strings.TrimSpace(request.Name)
	if !taxonomySlugPattern.MatchString(slug) || len(slug) > 80 {
		return tags.AdminCreateInput{}, errors.New("slug must be lowercase kebab-case and 80 characters or fewer")
	}
	if len(name) < 2 || len(name) > 80 {
		return tags.AdminCreateInput{}, errors.New("name must be between 2 and 80 characters")
	}
	return tags.AdminCreateInput{Slug: slug, Name: name, IsSystem: request.IsSystem}, nil
}

func normalizeTagUpdate(request tagUpdateRequest) (tags.AdminUpdateInput, error) {
	if request.Name != nil {
		name := strings.TrimSpace(*request.Name)
		if len(name) < 2 || len(name) > 80 {
			return tags.AdminUpdateInput{}, errors.New("name must be between 2 and 80 characters")
		}
		request.Name = &name
	}
	return tags.AdminUpdateInput{Name: request.Name, IsSystem: request.IsSystem}, nil
}
