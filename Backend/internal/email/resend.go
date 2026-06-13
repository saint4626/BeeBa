package email

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"strings"
	"time"

	"beeba.org/internal/config"
)

const emailBrandLogoPath = "/favicon-48.png"

type Sender interface {
	SendEmailVerification(ctx context.Context, input VerificationEmail) (string, error)
	SendPasswordChangeConfirmation(ctx context.Context, input PasswordChangeEmail) (string, error)
	SendEmailChangeConfirmation(ctx context.Context, input EmailChangeEmail) (string, error)
}

type VerificationEmail struct {
	To            string
	Username      string
	TokenID       string
	Token         string
	PublicBaseURL string
	ExpiresAt     time.Time
}

type PasswordChangeEmail struct {
	To            string
	Username      string
	TokenID       string
	Token         string
	PublicBaseURL string
	ExpiresAt     time.Time
}

type EmailChangeEmail struct {
	To            string
	Username      string
	TokenID       string
	Token         string
	PublicBaseURL string
	ExpiresAt     time.Time
}

type ResendSender struct {
	apiKey  string
	baseURL string
	from    string
	replyTo string
	client  *http.Client
}

func NewResendSender(cfg config.Config) (*ResendSender, error) {
	return &ResendSender{
		apiKey:  strings.TrimSpace(cfg.ResendAPIKey),
		baseURL: strings.TrimRight(strings.TrimSpace(cfg.ResendAPIBaseURL), "/"),
		from:    strings.TrimSpace(cfg.EmailFrom),
		replyTo: strings.TrimSpace(cfg.EmailReplyTo),
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}, nil
}

func (s *ResendSender) SendEmailVerification(ctx context.Context, input VerificationEmail) (string, error) {
	if s == nil || s.client == nil {
		return "", fmt.Errorf("email sender is not configured")
	}
	if strings.TrimSpace(input.To) == "" || strings.TrimSpace(input.Token) == "" || strings.TrimSpace(input.TokenID) == "" {
		return "", fmt.Errorf("email verification input is incomplete")
	}

	link, err := verificationLink(input.PublicBaseURL, input.Token)
	if err != nil {
		return "", err
	}

	payload := resendEmailRequest{
		From:    s.from,
		To:      []string{input.To},
		Subject: "Verify your BeeBa email",
		HTML:    verificationHTML(input.Username, link, input.ExpiresAt),
		Text:    verificationText(input.Username, link, input.ExpiresAt),
		Tags: []resendTag{
			{Name: "type", Value: "email_verification"},
			{Name: "token_id", Value: safeTagValue(input.TokenID)},
		},
	}
	if s.replyTo != "" {
		payload.ReplyTo = []string{s.replyTo}
	}

	return s.send(ctx, payload, "email-verification-"+input.TokenID)
}

func (s *ResendSender) SendPasswordChangeConfirmation(ctx context.Context, input PasswordChangeEmail) (string, error) {
	if s == nil || s.client == nil {
		return "", fmt.Errorf("email sender is not configured")
	}
	if strings.TrimSpace(input.To) == "" || strings.TrimSpace(input.Token) == "" || strings.TrimSpace(input.TokenID) == "" {
		return "", fmt.Errorf("password change email input is incomplete")
	}

	link, err := accountActionLink(input.PublicBaseURL, "/confirm-password-change", input.Token)
	if err != nil {
		return "", err
	}

	payload := s.accountActionRequest(input.To, input.Username, input.TokenID, "password_change", actionTemplate{
		Subject:   "Confirm your BeeBa password change",
		Title:     "Confirm your BeeBa password change",
		Preheader: "Confirm your request to change your BeeBa password.",
		Heading:   "Confirm password change",
		Body:      "Use this link to change your BeeBa password. If you did not request this, you can ignore this email and your current password will stay active.",
		Button:    "Confirm password change",
		Footer:    "You received this email because someone requested a BeeBa password change. You can reply to this message if something looks wrong.",
	}, link, input.ExpiresAt)

	return s.send(ctx, payload, "password-change-"+input.TokenID)
}

func (s *ResendSender) SendEmailChangeConfirmation(ctx context.Context, input EmailChangeEmail) (string, error) {
	if s == nil || s.client == nil {
		return "", fmt.Errorf("email sender is not configured")
	}
	if strings.TrimSpace(input.To) == "" || strings.TrimSpace(input.Token) == "" || strings.TrimSpace(input.TokenID) == "" {
		return "", fmt.Errorf("email change input is incomplete")
	}

	link, err := accountActionLink(input.PublicBaseURL, "/confirm-email-change", input.Token)
	if err != nil {
		return "", err
	}

	payload := s.accountActionRequest(input.To, input.Username, input.TokenID, "email_change", actionTemplate{
		Subject:   "Confirm your BeeBa email change",
		Title:     "Confirm your BeeBa email change",
		Preheader: "Confirm this email address for your BeeBa account.",
		Heading:   "Confirm new email",
		Body:      "Confirm that you want to use this address for BeeBa. Your account email will not change until this confirmation is complete.",
		Button:    "Confirm email change",
		Footer:    "You received this email because someone requested to use this address for a BeeBa account. You can reply to this message if something looks wrong.",
	}, link, input.ExpiresAt)

	return s.send(ctx, payload, "email-change-"+input.TokenID)
}

func (s *ResendSender) send(ctx context.Context, payload resendEmailRequest, idempotencyKey string) (string, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal resend email request: %w", err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+"/emails", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create resend email request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+s.apiKey)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", "beeba-api/0.1")
	request.Header.Set("Idempotency-Key", idempotencyKey)

	response, err := s.client.Do(request)
	if err != nil {
		return "", fmt.Errorf("send resend email request: %w", err)
	}
	defer response.Body.Close()

	var result resendEmailResponse
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode resend email response: %w", err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		message := strings.TrimSpace(result.Message)
		if message == "" {
			message = http.StatusText(response.StatusCode)
		}
		return "", fmt.Errorf("resend email failed with status %d: %s", response.StatusCode, message)
	}
	if strings.TrimSpace(result.ID) == "" {
		return "", fmt.Errorf("resend email response missing id")
	}
	return result.ID, nil
}

type actionTemplate struct {
	Subject   string
	Title     string
	Preheader string
	Heading   string
	Body      string
	Button    string
	Footer    string
}

func (s *ResendSender) accountActionRequest(to string, username string, tokenID string, kind string, template actionTemplate, link string, expiresAt time.Time) resendEmailRequest {
	payload := resendEmailRequest{
		From:    s.from,
		To:      []string{to},
		Subject: template.Subject,
		HTML:    accountActionHTML(username, link, expiresAt, template),
		Text:    accountActionText(username, link, expiresAt, template),
		Tags: []resendTag{
			{Name: "type", Value: kind},
			{Name: "token_id", Value: safeTagValue(tokenID)},
		},
	}
	if s.replyTo != "" {
		payload.ReplyTo = []string{s.replyTo}
	}
	return payload
}

type resendEmailRequest struct {
	From    string      `json:"from"`
	To      []string    `json:"to"`
	ReplyTo []string    `json:"reply_to,omitempty"`
	Subject string      `json:"subject"`
	HTML    string      `json:"html"`
	Text    string      `json:"text"`
	Tags    []resendTag `json:"tags,omitempty"`
}

type resendTag struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type resendEmailResponse struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Message string `json:"message"`
}

func verificationLink(publicBaseURL string, token string) (string, error) {
	return accountActionLink(publicBaseURL, "/verify-email", token)
}

func accountActionLink(publicBaseURL string, path string, token string) (string, error) {
	base, err := url.Parse(strings.TrimRight(strings.TrimSpace(publicBaseURL), "/"))
	if err != nil {
		return "", fmt.Errorf("parse public base url: %w", err)
	}
	if base.Scheme == "" || base.Host == "" {
		return "", fmt.Errorf("public base url must be absolute")
	}
	base.Path = path
	base.RawQuery = url.Values{"token": []string{token}}.Encode()
	return base.String(), nil
}

func accountActionHTML(username string, link string, expiresAt time.Time, template actionTemplate) string {
	name := html.EscapeString(displayName(username))
	escapedLink := html.EscapeString(link)
	expiry := html.EscapeString(expiryText(expiresAt))
	title := html.EscapeString(template.Title)
	preheader := html.EscapeString(template.Preheader)
	heading := html.EscapeString(template.Heading)
	body := html.EscapeString(template.Body)
	button := html.EscapeString(template.Button)
	footer := html.EscapeString(template.Footer)
	brandHeader := emailBrandHeaderHTML(link)
	return fmt.Sprintf(`<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>%s</title>
  </head>
  <body style="margin:0;background:#0f0f0f;color:#ffffff;font-family:Arial,'Helvetica Neue',Helvetica,sans-serif;">
    <div style="display:none;max-height:0;overflow:hidden;color:transparent;opacity:0;">
      %s
    </div>
    <table role="presentation" width="100%%" cellspacing="0" cellpadding="0" style="background:#0f0f0f;margin:0;padding:0;">
      <tr>
        <td align="center" style="padding:32px 16px;">
          <table role="presentation" width="100%%" cellspacing="0" cellpadding="0" style="max-width:520px;border:1px solid rgba(255,215,0,.28);border-radius:8px;background:#161616;overflow:hidden;">
            <tr>
              <td style="height:8px;background:#ffd700;font-size:0;line-height:0;">&nbsp;</td>
            </tr>
            <tr>
              <td style="padding:28px 28px 12px;">
                %s
                <h1 style="margin:14px 0 0;color:#ffffff;font-size:28px;line-height:1.18;font-weight:700;">%s</h1>
              </td>
            </tr>
            <tr>
              <td style="padding:8px 28px 0;color:#d8d8d8;font-size:15px;line-height:1.6;">
                <p style="margin:0 0 14px;">Hello %s,</p>
                <p style="margin:0 0 20px;">%s</p>
              </td>
            </tr>
            <tr>
              <td align="left" style="padding:4px 28px 24px;">
                <a href="%s" style="display:inline-block;border-radius:6px;background:#ffd700;color:#000000;font-size:15px;font-weight:700;line-height:1;text-decoration:none;padding:14px 18px;">%s</a>
              </td>
            </tr>
            <tr>
              <td style="padding:0 28px 24px;color:#a8a8a8;font-size:13px;line-height:1.55;">
                <p style="margin:0 0 10px;">This link expires %s.</p>
                <p style="margin:0;">If the button does not work, paste this link into your browser:</p>
                <p style="margin:8px 0 0;word-break:break-all;"><a href="%s" style="color:#ffa800;text-decoration:underline;">%s</a></p>
              </td>
            </tr>
            <tr>
              <td style="border-top:1px solid rgba(255,255,255,.08);padding:18px 28px 24px;color:#7f7f7f;font-size:12px;line-height:1.5;">
                %s
              </td>
            </tr>
          </table>
        </td>
      </tr>
    </table>
  </body>
</html>`, title, preheader, brandHeader, heading, name, body, escapedLink, button, expiry, escapedLink, escapedLink, footer)
}

func accountActionText(username string, link string, expiresAt time.Time, template actionTemplate) string {
	return fmt.Sprintf("Hello %s,\n\n%s\n%s\n\nThis link expires %s.\n\n%s\n", displayName(username), template.Body, link, expiryText(expiresAt), template.Footer)
}

func verificationHTML(username string, link string, expiresAt time.Time) string {
	name := html.EscapeString(displayName(username))
	escapedLink := html.EscapeString(link)
	expiry := html.EscapeString(expiryText(expiresAt))
	brandHeader := emailBrandHeaderHTML(link)
	return fmt.Sprintf(`<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>Verify your BeeBa email</title>
  </head>
  <body style="margin:0;background:#0f0f0f;color:#ffffff;font-family:Arial,'Helvetica Neue',Helvetica,sans-serif;">
    <div style="display:none;max-height:0;overflow:hidden;color:transparent;opacity:0;">
      Confirm your email address to finish securing your BeeBa account.
    </div>
    <table role="presentation" width="100%%" cellspacing="0" cellpadding="0" style="background:#0f0f0f;margin:0;padding:0;">
      <tr>
        <td align="center" style="padding:32px 16px;">
          <table role="presentation" width="100%%" cellspacing="0" cellpadding="0" style="max-width:520px;border:1px solid rgba(255,215,0,.28);border-radius:8px;background:#161616;overflow:hidden;">
            <tr>
              <td style="height:8px;background:#ffd700;font-size:0;line-height:0;">&nbsp;</td>
            </tr>
            <tr>
              <td style="padding:28px 28px 12px;">
                %s
                <h1 style="margin:14px 0 0;color:#ffffff;font-size:28px;line-height:1.18;font-weight:700;">Verify your email</h1>
              </td>
            </tr>
            <tr>
              <td style="padding:8px 28px 0;color:#d8d8d8;font-size:15px;line-height:1.6;">
                <p style="margin:0 0 14px;">Hello %s,</p>
                <p style="margin:0 0 20px;">Confirm this email address to finish securing your BeeBa account and keep your Basis / VR uploads tied to the right profile.</p>
              </td>
            </tr>
            <tr>
              <td align="left" style="padding:4px 28px 24px;">
                <a href="%s" style="display:inline-block;border-radius:6px;background:#ffd700;color:#000000;font-size:15px;font-weight:700;line-height:1;text-decoration:none;padding:14px 18px;">Verify email</a>
              </td>
            </tr>
            <tr>
              <td style="padding:0 28px 24px;color:#a8a8a8;font-size:13px;line-height:1.55;">
                <p style="margin:0 0 10px;">This link expires %s.</p>
                <p style="margin:0;">If the button does not work, paste this link into your browser:</p>
                <p style="margin:8px 0 0;word-break:break-all;"><a href="%s" style="color:#ffa800;text-decoration:underline;">%s</a></p>
              </td>
            </tr>
            <tr>
              <td style="border-top:1px solid rgba(255,255,255,.08);padding:18px 28px 24px;color:#7f7f7f;font-size:12px;line-height:1.5;">
                You received this email because someone created or updated a BeeBa account with this address. You can reply to this message if something looks wrong.
              </td>
            </tr>
          </table>
        </td>
      </tr>
    </table>
  </body>
</html>`, brandHeader, name, escapedLink, expiry, escapedLink, escapedLink)
}

func verificationText(username string, link string, expiresAt time.Time) string {
	return fmt.Sprintf("Hello %s,\n\nConfirm this email address to finish securing your BeeBa account and keep your Basis / VR uploads tied to the right profile:\n%s\n\nThis link expires %s.\n\nYou can reply to this message if something looks wrong.\n", displayName(username), link, expiryText(expiresAt))
}

func emailBrandHeaderHTML(actionLink string) string {
	logoURL := html.EscapeString(emailAssetURL(actionLink, emailBrandLogoPath))
	return fmt.Sprintf(`<table role="presentation" cellspacing="0" cellpadding="0" style="border-collapse:collapse;">
                  <tr>
                    <td style="padding:0 8px 0 0;vertical-align:middle;">
                      <img src="%s" width="24" height="24" alt="" style="display:block;width:24px;height:24px;border:0;outline:none;text-decoration:none;">
                    </td>
                    <td style="vertical-align:middle;font-size:13px;line-height:1.2;font-weight:700;letter-spacing:.08em;text-transform:uppercase;color:#ffd700;">.BEEBA</td>
                  </tr>
                </table>`, logoURL)
}

func emailAssetURL(actionLink string, assetPath string) string {
	base, err := url.Parse(actionLink)
	if err != nil || base.Scheme == "" || base.Host == "" {
		return assetPath
	}
	base.Path = assetPath
	base.RawQuery = ""
	base.Fragment = ""
	return base.String()
}

func displayName(username string) string {
	value := strings.TrimSpace(username)
	if value == "" {
		return "there"
	}
	return value
}

func expiryText(expiresAt time.Time) string {
	if expiresAt.IsZero() {
		return "in 24 hours"
	}
	return "at " + expiresAt.UTC().Format(time.RFC3339)
}

func safeTagValue(value string) string {
	replacer := strings.NewReplacer("_", "_", "-", "-", " ", "-")
	value = replacer.Replace(strings.TrimSpace(value))
	var builder strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			builder.WriteRune(r)
		}
	}
	result := builder.String()
	if len(result) > 256 {
		return result[:256]
	}
	return result
}
