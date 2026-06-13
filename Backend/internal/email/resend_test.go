package email

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestResendSenderSendEmailVerification(t *testing.T) {
	var request resendEmailRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/emails" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("unexpected authorization header %q", got)
		}
		if got := r.Header.Get("User-Agent"); got != "beeba-api/0.1" {
			t.Fatalf("unexpected user agent %q", got)
		}
		if got := r.Header.Get("Idempotency-Key"); got != "email-verification-token-id" {
			t.Fatalf("unexpected idempotency key %q", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"email-id"}`))
	}))
	defer server.Close()

	sender := &ResendSender{
		apiKey:  "test-key",
		baseURL: server.URL,
		from:    "BeeBa <hello@beeba.org>",
		client:  server.Client(),
	}

	id, err := sender.SendEmailVerification(context.Background(), VerificationEmail{
		To:            "user@example.com",
		Username:      "kikfiz",
		TokenID:       "token-id",
		Token:         "bb_ev_secret",
		PublicBaseURL: "https://beeba.org",
		ExpiresAt:     time.Date(2026, 6, 13, 10, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("send email verification: %v", err)
	}
	if id != "email-id" {
		t.Fatalf("unexpected id %q", id)
	}
	if request.From != "BeeBa <hello@beeba.org>" || len(request.To) != 1 || request.To[0] != "user@example.com" {
		t.Fatalf("unexpected recipients: %+v", request)
	}
	if strings.Contains(request.From, "no-reply") || strings.Contains(request.HTML, "no-reply") || strings.Contains(request.Text, "no-reply") {
		t.Fatal("email should not use no-reply wording")
	}
	if !strings.Contains(request.HTML, "https://beeba.org/verify-email?token=bb_ev_secret") {
		t.Fatalf("verification link missing from html: %s", request.HTML)
	}
	if !strings.Contains(request.HTML, "#ffd700") || !strings.Contains(request.HTML, "Verify email") {
		t.Fatalf("branded cta missing from html: %s", request.HTML)
	}
	assertEmailBrandHeader(t, request.HTML)
	if !strings.Contains(request.Text, "kikfiz") {
		t.Fatalf("username missing from text: %s", request.Text)
	}
}

func TestVerificationLinkRequiresAbsoluteBaseURL(t *testing.T) {
	if _, err := verificationLink("/relative", "bb_ev_secret"); err == nil {
		t.Fatal("expected relative public base url to fail")
	}
}

func TestResendSenderSendPasswordChangeConfirmation(t *testing.T) {
	var request resendEmailRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Idempotency-Key"); got != "password-change-token-id" {
			t.Fatalf("unexpected idempotency key %q", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"email-id"}`))
	}))
	defer server.Close()

	sender := &ResendSender{
		apiKey:  "test-key",
		baseURL: server.URL,
		from:    "BeeBa <hello@beeba.org>",
		client:  server.Client(),
	}

	id, err := sender.SendPasswordChangeConfirmation(context.Background(), PasswordChangeEmail{
		To:            "user@example.com",
		Username:      "kikfiz",
		TokenID:       "token-id",
		Token:         "bb_pc_secret",
		PublicBaseURL: "https://beeba.org",
		ExpiresAt:     time.Date(2026, 6, 13, 10, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("send password change confirmation: %v", err)
	}
	if id != "email-id" {
		t.Fatalf("unexpected id %q", id)
	}
	if request.Subject != "Confirm your BeeBa password change" {
		t.Fatalf("unexpected subject %q", request.Subject)
	}
	if strings.Contains(request.From, "no-reply") || strings.Contains(request.HTML, "no-reply") || strings.Contains(request.Text, "no-reply") {
		t.Fatal("email should not use no-reply wording")
	}
	if !strings.Contains(request.HTML, "https://beeba.org/confirm-password-change?token=bb_pc_secret") {
		t.Fatalf("password confirmation link missing from html: %s", request.HTML)
	}
	if !strings.Contains(request.HTML, "Confirm password change") || !strings.Contains(request.Text, "change your BeeBa password") {
		t.Fatalf("password confirmation copy missing: %s", request.HTML)
	}
	assertEmailBrandHeader(t, request.HTML)
}

func TestResendSenderSendEmailChangeConfirmation(t *testing.T) {
	var request resendEmailRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Idempotency-Key"); got != "email-change-token-id" {
			t.Fatalf("unexpected idempotency key %q", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"email-id"}`))
	}))
	defer server.Close()

	sender := &ResendSender{
		apiKey:  "test-key",
		baseURL: server.URL,
		from:    "BeeBa <hello@beeba.org>",
		client:  server.Client(),
	}

	id, err := sender.SendEmailChangeConfirmation(context.Background(), EmailChangeEmail{
		To:            "new@example.com",
		Username:      "kikfiz",
		TokenID:       "token-id",
		Token:         "bb_ec_secret",
		PublicBaseURL: "https://beeba.org",
		ExpiresAt:     time.Date(2026, 6, 13, 10, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("send email change confirmation: %v", err)
	}
	if id != "email-id" {
		t.Fatalf("unexpected id %q", id)
	}
	if request.Subject != "Confirm your BeeBa email change" {
		t.Fatalf("unexpected subject %q", request.Subject)
	}
	if len(request.To) != 1 || request.To[0] != "new@example.com" {
		t.Fatalf("unexpected recipient: %+v", request.To)
	}
	if !strings.Contains(request.HTML, "https://beeba.org/confirm-email-change?token=bb_ec_secret") {
		t.Fatalf("email change confirmation link missing from html: %s", request.HTML)
	}
	if !strings.Contains(request.HTML, "Confirm new email") || !strings.Contains(request.Text, "use this address for BeeBa") {
		t.Fatalf("email change copy missing: %s", request.HTML)
	}
	assertEmailBrandHeader(t, request.HTML)
}

func assertEmailBrandHeader(t *testing.T, html string) {
	t.Helper()

	if !strings.Contains(html, `src="https://beeba.org/favicon-48.png"`) {
		t.Fatalf("brand logo missing from email header: %s", html)
	}
	if !strings.Contains(html, ">.BEEBA<") {
		t.Fatalf("brand name should render as .BEEBA: %s", html)
	}
	if strings.Contains(html, ">BeeBa</div>") {
		t.Fatalf("old brand header should not remain: %s", html)
	}
}
