package content

import (
	"context"
	"net/http/httptest"
	"testing"

	domain "beeba.org/internal/domain/content"

	"github.com/gofiber/fiber/v3"
)

func TestListReturnsEmptyPage(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Get("/content", New(fakeStore{}).List)

	resp, err := app.Test(httptest.NewRequest("GET", "/content?sort=newest&limit=2", nil))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}
}

func TestListRejectsUnsupportedSort(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Get("/content", New(fakeStore{}).List)

	resp, err := app.Test(httptest.NewRequest("GET", "/content?sort=offset", nil))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", resp.StatusCode)
	}
}

func TestListWithoutStoreReturnsUnavailable(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Get("/content", New(nil).List)

	resp, err := app.Test(httptest.NewRequest("GET", "/content", nil))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != fiber.StatusServiceUnavailable {
		t.Fatalf("expected status 503, got %d", resp.StatusCode)
	}
}

type fakeStore struct {
	page domain.Page
	err  error
}

func (s fakeStore) ListPublished(context.Context, domain.ListFilter) (domain.Page, error) {
	if s.err != nil {
		return domain.Page{}, s.err
	}
	return s.page, nil
}

func (s fakeStore) GetPublished(context.Context, string) (domain.PublicDetail, error) {
	if s.err != nil {
		return domain.PublicDetail{}, s.err
	}
	return domain.PublicDetail{}, nil
}
