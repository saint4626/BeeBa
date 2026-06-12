package content

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	authdomain "beeba.org/internal/domain/auth"
	domain "beeba.org/internal/domain/content"
	"beeba.org/internal/domain/users"
	"beeba.org/internal/http/middleware/authz"
	"beeba.org/internal/security/tokens"

	"github.com/gofiber/fiber/v3"
)

func TestListReturnsEmptyPage(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Get("/content", New(&fakeStore{}).List)

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
	app.Get("/content", New(&fakeStore{}).List)

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

func TestDetailPassesViewerFromAccessCookie(t *testing.T) {
	store := &fakeStore{
		detail: domain.PublicDetail{
			PublicItem: domain.PublicItem{ID: "11111111-1111-1111-1111-111111111111"},
		},
	}
	app := fiber.New()
	app.Get("/content/:contentID", New(store, fakeAuthStore{
		session: authdomain.SessionWithUser{
			SessionID: "session-id",
			User: users.PublicUser{
				ID:       "22222222-2222-2222-2222-222222222222",
				Username: "viewer",
			},
		},
	}).Detail)

	req := httptest.NewRequest("GET", "/content/11111111-1111-1111-1111-111111111111", nil)
	req.AddCookie(&http.Cookie{Name: authz.AccessCookieName, Value: "viewer-token"})
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}
	if store.viewerUserID != "22222222-2222-2222-2222-222222222222" {
		t.Fatalf("viewer user id = %q", store.viewerUserID)
	}
}

type fakeStore struct {
	page         domain.Page
	detail       domain.PublicDetail
	err          error
	viewerUserID string
}

func (s *fakeStore) ListPublished(context.Context, domain.ListFilter) (domain.Page, error) {
	if s.err != nil {
		return domain.Page{}, s.err
	}
	return s.page, nil
}

func (s *fakeStore) GetPublished(_ context.Context, _ string, viewerUserID string) (domain.PublicDetail, error) {
	if s.err != nil {
		return domain.PublicDetail{}, s.err
	}
	s.viewerUserID = viewerUserID
	return s.detail, nil
}

type fakeAuthStore struct {
	session authdomain.SessionWithUser
	err     error
}

func (s fakeAuthStore) FindByAccessTokenHash(_ context.Context, tokenHash string, _ time.Time) (authdomain.SessionWithUser, error) {
	if s.err != nil {
		return authdomain.SessionWithUser{}, s.err
	}
	if tokenHash != tokens.Hash("viewer-token") {
		return authdomain.SessionWithUser{}, nil
	}
	return s.session, nil
}
