package me

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	authdomain "beeba.org/internal/domain/auth"
	"beeba.org/internal/domain/users"
	"beeba.org/internal/http/httperror"
	"beeba.org/internal/http/middleware/authz"

	"github.com/gofiber/fiber/v3"
)

func TestServerProbeRequiresAuthentication(t *testing.T) {
	t.Parallel()

	app := serverProbeTestApp(&serverProbeAuthStore{})
	req := httptest.NewRequest("POST", "/me/servers/probe", strings.NewReader(`{"connection_string":"example.org:4296"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test returned error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != fiber.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", resp.StatusCode, fiber.StatusUnauthorized)
	}
}

func TestServerProbeRejectsUnsafeEndpointBeforeNetwork(t *testing.T) {
	t.Parallel()

	app := serverProbeTestApp(newServerProbeAuthStore())
	req := httptest.NewRequest("POST", "/me/servers/probe", strings.NewReader(`{"connection_string":"127.0.0.1:4296"}`))
	req.Header.Set("Authorization", "Bearer bb_at_test")
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test returned error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, fiber.StatusBadRequest)
	}
}

func serverProbeTestApp(authStore *serverProbeAuthStore) *fiber.App {
	app := fiber.New(fiber.Config{ErrorHandler: httperror.Handler})
	handler := NewServerHandler(nil)
	app.Post("/me/servers/probe", authz.RequireBearer(authStore), handler.Probe)
	return app
}

func newServerProbeAuthStore() *serverProbeAuthStore {
	return &serverProbeAuthStore{
		session: authdomain.SessionWithUser{
			SessionID: "22222222-2222-2222-2222-222222222222",
			User: users.PublicUser{
				ID:        "00000000-0000-0000-0000-000000000001",
				Email:     "owner@example.com",
				Username:  "owner",
				CreatedAt: time.Now().UTC(),
				UpdatedAt: time.Now().UTC(),
			},
		},
	}
}

type serverProbeAuthStore struct {
	session authdomain.SessionWithUser
}

func (s *serverProbeAuthStore) FindByAccessTokenHash(context.Context, string, time.Time) (authdomain.SessionWithUser, error) {
	return s.session, nil
}
