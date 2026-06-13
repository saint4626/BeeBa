package turnstile

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestVerifierVerify(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method %s", r.Method)
		}
		if got := r.Header.Get("Content-Type"); got != "application/x-www-form-urlencoded" {
			t.Fatalf("unexpected content type %q", got)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		if got := r.Form.Get("secret"); got != "secret-key" {
			t.Fatalf("unexpected secret %q", got)
		}
		if got := r.Form.Get("response"); got != "token" {
			t.Fatalf("unexpected response %q", got)
		}
		if got := r.Form.Get("remoteip"); got != "203.0.113.10" {
			t.Fatalf("unexpected remoteip %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(Result{Success: true, Hostname: "beeba.org", Action: "register"})
	}))
	defer server.Close()

	verifier := &Verifier{
		secret:   "secret-key",
		endpoint: server.URL,
		client:   server.Client(),
	}

	result, err := verifier.Verify(context.Background(), "token", "203.0.113.10")
	if err != nil {
		t.Fatalf("verify turnstile: %v", err)
	}
	if !result.Success || result.Hostname != "beeba.org" || result.Action != "register" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestVerifierRequiresToken(t *testing.T) {
	verifier := New("secret-key", "https://example.com")
	if _, err := verifier.Verify(context.Background(), " ", ""); err == nil {
		t.Fatal("expected empty token to fail")
	}
}
