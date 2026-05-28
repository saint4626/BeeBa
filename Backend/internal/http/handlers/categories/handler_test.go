package categories

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	domain "beeba.org/internal/domain/categories"

	"github.com/gofiber/fiber/v3"
)

func TestListReturnsCategories(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Get("/categories", New(fakeStore{
		items: []domain.Category{
			{
				ID:             "category-id",
				Slug:           "worlds",
				Name:           "Worlds",
				Description:    "VR worlds",
				SortOrder:      10,
				IsActive:       true,
				PublishedCount: 0,
				CreatedAt:      time.Unix(0, 0).UTC(),
				UpdatedAt:      time.Unix(0, 0).UTC(),
			},
		},
	}).List)

	req := httptest.NewRequest("GET", "/categories", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}
}

func TestListWithoutStoreReturnsUnavailable(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Get("/categories", New(nil).List)

	req := httptest.NewRequest("GET", "/categories", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != fiber.StatusServiceUnavailable {
		t.Fatalf("expected status 503, got %d", resp.StatusCode)
	}
}

type fakeStore struct {
	items []domain.Category
	err   error
}

func (s fakeStore) ListActive(context.Context) ([]domain.Category, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.items, nil
}
