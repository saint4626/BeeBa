package turnstile

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Verifier struct {
	secret   string
	endpoint string
	client   *http.Client
}

type Result struct {
	Success     bool     `json:"success"`
	ErrorCodes  []string `json:"error-codes"`
	ChallengeTS string   `json:"challenge_ts"`
	Hostname    string   `json:"hostname"`
	Action      string   `json:"action"`
	CData       string   `json:"cdata"`
}

func New(secret string, endpoint string) *Verifier {
	return &Verifier{
		secret:   strings.TrimSpace(secret),
		endpoint: strings.TrimSpace(endpoint),
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

func (v *Verifier) Verify(ctx context.Context, token string, remoteIP string) (Result, error) {
	if v == nil || v.client == nil || v.secret == "" || v.endpoint == "" {
		return Result{}, fmt.Errorf("turnstile verifier is not configured")
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return Result{}, fmt.Errorf("turnstile token is empty")
	}

	form := url.Values{}
	form.Set("secret", v.secret)
	form.Set("response", token)
	if strings.TrimSpace(remoteIP) != "" {
		form.Set("remoteip", strings.TrimSpace(remoteIP))
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, v.endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return Result{}, fmt.Errorf("create turnstile verification request: %w", err)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("User-Agent", "beeba-api/0.1")

	response, err := v.client.Do(request)
	if err != nil {
		return Result{}, fmt.Errorf("send turnstile verification request: %w", err)
	}
	defer response.Body.Close()

	var result Result
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return Result{}, fmt.Errorf("decode turnstile verification response: %w", err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return result, fmt.Errorf("turnstile verification failed with status %d", response.StatusCode)
	}
	return result, nil
}
