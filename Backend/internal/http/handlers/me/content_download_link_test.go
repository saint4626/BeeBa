package me

import (
	"context"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	authdomain "beeba.org/internal/domain/auth"
	"beeba.org/internal/domain/content"
	"beeba.org/internal/domain/users"
	"beeba.org/internal/http/middleware/authz"

	"github.com/gofiber/fiber/v3"
)

func TestDownloadLinkReturnsUnlistedPublicDownloadPath(t *testing.T) {
	t.Parallel()

	const ownerID = "00000000-0000-0000-0000-000000000001"
	const contentID = "11111111-1111-1111-1111-111111111111"

	store := &downloadLinkStore{token: "bb_dl_secret"}
	authStore := &downloadLinkAuthStore{
		session: authdomain.SessionWithUser{
			SessionID: "22222222-2222-2222-2222-222222222222",
			User: users.PublicUser{
				ID:        ownerID,
				Email:     "owner@example.com",
				Username:  "owner",
				CreatedAt: time.Now().UTC(),
				UpdatedAt: time.Now().UTC(),
			},
		},
	}

	app := fiber.New()
	handler := NewContentHandler(store, 10)
	app.Post("/me/content/:contentID/download-link", authz.RequireBearer(authStore), handler.DownloadLink)

	req := httptest.NewRequest("POST", "/me/content/"+contentID+"/download-link", nil)
	req.Header.Set("Authorization", "Bearer bb_at_test")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test returned error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != fiber.StatusOK {
		payload, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected status %d, got %d: %s", fiber.StatusOK, resp.StatusCode, string(payload))
	}
	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	responseText := string(payload)
	if !strings.Contains(responseText, `"/content/`+contentID+`/download?access=bb_dl_secret"`) {
		t.Fatalf("expected unlisted download path in response, got %s", responseText)
	}
	if store.ownerID != ownerID || store.contentID != contentID {
		t.Fatalf("expected store lookup owner=%s content=%s, got owner=%s content=%s", ownerID, contentID, store.ownerID, store.contentID)
	}
}

type downloadLinkStore struct {
	token     string
	ownerID   string
	contentID string
}

func (s *downloadLinkStore) ListOwned(context.Context, string, content.OwnerListFilter) (content.OwnerPage, error) {
	return content.OwnerPage{}, nil
}

func (s *downloadLinkStore) OwnerStorageUsage(context.Context, string, int64) (content.OwnerStorageUsage, error) {
	return content.OwnerStorageUsage{}, nil
}

func (s *downloadLinkStore) UpdateOwned(context.Context, string, string, content.OwnerUpdateInput) (content.OwnerItem, error) {
	return content.OwnerItem{}, nil
}

func (s *downloadLinkStore) EnsureUnlistedDownloadToken(_ context.Context, ownerID string, contentID string) (string, error) {
	s.ownerID = ownerID
	s.contentID = contentID
	return s.token, nil
}

func (s *downloadLinkStore) DeleteOwned(context.Context, string, string) error {
	return nil
}

type downloadLinkAuthStore struct {
	session authdomain.SessionWithUser
}

func (s *downloadLinkAuthStore) FindByAccessTokenHash(context.Context, string, time.Time) (authdomain.SessionWithUser, error) {
	return s.session, nil
}
