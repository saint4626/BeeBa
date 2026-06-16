package auth

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"beeba.org/internal/config"
	authdomain "beeba.org/internal/domain/auth"
	"beeba.org/internal/domain/users"
	"beeba.org/internal/http/middleware/authz"
	"beeba.org/internal/security/password"

	"github.com/gofiber/fiber/v3"
)

func TestLoginSetsCookiesWithoutReturningTokens(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	passwordHash, err := password.Hash("correct horse battery staple")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	store := &loginStore{
		user: authdomain.UserWithPassword{
			User: users.PublicUser{
				ID:        "00000000-0000-0000-0000-000000000001",
				Email:     "user@example.com",
				Username:  "tester",
				CreatedAt: now,
				UpdatedAt: now,
			},
			PasswordHash: passwordHash,
		},
	}
	app := fiber.New()
	handler := New(config.Config{}, store)
	app.Post("/login", handler.Login)

	body := bytes.NewBufferString(`{"email":"user@example.com","password":"correct horse battery staple"}`)
	req := httptest.NewRequest("POST", "/login", body)
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("test request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != fiber.StatusOK {
		payload, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected status 200, got %d: %s", resp.StatusCode, string(payload))
	}

	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	responseText := string(payload)
	if strings.Contains(responseText, "access_token") || strings.Contains(responseText, "refresh_token") {
		t.Fatalf("login response leaked token fields: %s", responseText)
	}
	if !strings.Contains(responseText, `"username":"tester"`) {
		t.Fatalf("login response missing public user: %s", responseText)
	}

	cookies := resp.Cookies()
	if !hasCookie(cookies, authz.AccessCookieName, true) {
		t.Fatalf("missing HttpOnly access cookie in %#v", cookies)
	}
	if !hasCookie(cookies, refreshCookieName, true) {
		t.Fatalf("missing HttpOnly refresh cookie in %#v", cookies)
	}
	if !hasCookie(cookies, authz.CSRFCookieName, false) {
		t.Fatalf("missing readable csrf cookie in %#v", cookies)
	}
	if store.createdSession.AccessTokenHash == "" || store.createdSession.RefreshTokenHash == "" {
		t.Fatal("session token hashes were not persisted")
	}
}

func TestCookieRefreshRotatesCookiesWithoutReturningTokens(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	store := &loginStore{
		refreshSession: authdomain.SessionWithUser{
			SessionID: "00000000-0000-0000-0000-000000000010",
			User: users.PublicUser{
				ID:        "00000000-0000-0000-0000-000000000001",
				Email:     "user@example.com",
				Username:  "tester",
				CreatedAt: now,
				UpdatedAt: now,
			},
		},
	}
	app := fiber.New()
	handler := New(config.Config{}, store)
	app.Post("/refresh", handler.Refresh)

	req := httptest.NewRequest("POST", "/refresh", nil)
	req.AddCookie(&http.Cookie{Name: refreshCookieName, Value: "bb_rt_existing"})
	req.AddCookie(&http.Cookie{Name: authz.CSRFCookieName, Value: "bb_csrf_existing"})
	req.Header.Set("X-CSRF-Token", "bb_csrf_existing")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("test request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != fiber.StatusOK {
		payload, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected status 200, got %d: %s", resp.StatusCode, string(payload))
	}

	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	responseText := string(payload)
	if strings.Contains(responseText, "access_token") || strings.Contains(responseText, "refresh_token") {
		t.Fatalf("cookie refresh response leaked token fields: %s", responseText)
	}
	if !strings.Contains(responseText, `"username":"tester"`) {
		t.Fatalf("cookie refresh response missing public user: %s", responseText)
	}

	cookies := resp.Cookies()
	if !hasCookie(cookies, authz.AccessCookieName, true) {
		t.Fatalf("missing rotated HttpOnly access cookie in %#v", cookies)
	}
	if !hasCookie(cookies, refreshCookieName, true) {
		t.Fatalf("missing rotated HttpOnly refresh cookie in %#v", cookies)
	}
	if store.createdSession.AccessTokenHash == "" || store.createdSession.RefreshTokenHash == "" {
		t.Fatal("rotated session token hashes were not persisted")
	}
}

type loginStore struct {
	user           authdomain.UserWithPassword
	refreshSession authdomain.SessionWithUser
	createdSession authdomain.SessionInput
}

func (s *loginStore) Create(context.Context, users.CreateInput) (users.PublicUser, error) {
	return users.PublicUser{}, nil
}

func (s *loginStore) VerifyEmail(context.Context, string, string, string) (users.EmailVerificationResult, error) {
	return users.EmailVerificationResult{}, nil
}

func (s *loginStore) RequestEmailVerification(context.Context, users.EmailVerificationRequestInput) (users.EmailVerificationResult, error) {
	return users.EmailVerificationResult{}, nil
}

func (s *loginStore) ConfirmPasswordChange(context.Context, string, string, string) (users.PasswordChangeConfirmResult, error) {
	return users.PasswordChangeConfirmResult{}, nil
}

func (s *loginStore) ConfirmEmailChange(context.Context, string, string, string) (users.EmailChangeConfirmResult, error) {
	return users.EmailChangeConfirmResult{}, nil
}

func (s *loginStore) FindByEmailForLogin(context.Context, string) (authdomain.UserWithPassword, error) {
	return s.user, nil
}

func (s *loginStore) CreateSession(_ context.Context, input authdomain.SessionInput) error {
	s.createdSession = input
	return nil
}

func (s *loginStore) FindByAccessTokenHash(context.Context, string, time.Time) (authdomain.SessionWithUser, error) {
	return authdomain.SessionWithUser{}, nil
}

func (s *loginStore) FindByRefreshTokenHash(context.Context, string, time.Time) (authdomain.SessionWithUser, error) {
	return s.refreshSession, nil
}

func (s *loginStore) RotateSession(_ context.Context, _ string, _ string, input authdomain.SessionInput) error {
	s.createdSession = input
	return nil
}

func (s *loginStore) RevokeByRefreshTokenHash(context.Context, string) error {
	return nil
}

func hasCookie(cookies []*http.Cookie, name string, httpOnly bool) bool {
	for _, cookie := range cookies {
		if cookie.Name == name && cookie.HttpOnly == httpOnly {
			return true
		}
	}
	return false
}
