package servers

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"time"
	"unicode"
)

const (
	DefaultPort uint16 = 4296
)

const (
	StatusDraft               = "draft"
	StatusPendingVerification = "pending_verification"
	StatusPendingModeration   = "pending_moderation"
	StatusPublished           = "published"
	StatusOffline             = "offline"
	StatusFailed              = "failed"
	StatusHidden              = "hidden"
	StatusDeleted             = "deleted"
)

const (
	CheckStatusPending = "pending"
	CheckStatusRunning = "running"
	CheckStatusOnline  = "online"
	CheckStatusOffline = "offline"
	CheckStatusFailed  = "failed"
)

const (
	VisibilityPublic   = "public"
	VisibilityUnlisted = "unlisted"
)

var (
	ErrInvalidEndpoint = errors.New("invalid server endpoint")
	ErrUnsafeEndpoint  = errors.New("server endpoint points to a private or reserved address")
	ErrInvalidLanguage = errors.New("invalid server language")
)

type Endpoint struct {
	Host     string
	Port     uint16
	Password *string
}

func (e Endpoint) ConnectionString() string {
	host := strings.TrimSpace(e.Host)
	if strings.Contains(host, ":") && !strings.HasPrefix(host, "[") && !strings.HasSuffix(host, "]") {
		host = "[" + host + "]"
	}
	value := fmt.Sprintf("%s:%d", host, e.Port)
	if e.Password != nil && strings.TrimSpace(*e.Password) != "" {
		value += "#" + *e.Password
	}
	return value
}

func ParseConnectionString(raw string) (Endpoint, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return Endpoint{}, fmt.Errorf("%w: address is required", ErrInvalidEndpoint)
	}

	left := value
	var password *string
	if hash := strings.IndexByte(value, '#'); hash >= 0 {
		rawPassword := value[hash+1:]
		left = value[:hash]
		if strings.TrimSpace(rawPassword) != "" {
			password = &rawPassword
		}
	}

	host := strings.TrimSpace(left)
	port := DefaultPort
	if strings.HasPrefix(host, "[") {
		endBracket := strings.IndexByte(host, ']')
		if endBracket < 0 {
			return Endpoint{}, fmt.Errorf("%w: invalid IPv6 host", ErrInvalidEndpoint)
		}
		withoutBrackets := strings.TrimSpace(host[1:endBracket])
		rest := strings.TrimSpace(host[endBracket+1:])
		host = withoutBrackets
		if strings.HasPrefix(rest, ":") {
			parsed, err := parsePort(rest[1:])
			if err != nil {
				return Endpoint{}, err
			}
			port = parsed
		} else if rest != "" {
			return Endpoint{}, fmt.Errorf("%w: invalid host suffix", ErrInvalidEndpoint)
		}
	} else if colon := strings.LastIndexByte(host, ':'); colon > 0 && colon < len(host)-1 {
		if parsed, err := parsePort(host[colon+1:]); err == nil {
			host = strings.TrimSpace(host[:colon])
			port = parsed
		}
	}

	endpoint := Endpoint{Host: normalizeHost(host), Port: port, Password: password}
	if err := ValidateEndpointShape(endpoint); err != nil {
		return Endpoint{}, err
	}
	return endpoint, nil
}

func parsePort(raw string) (uint16, error) {
	parsed, err := strconv.ParseUint(strings.TrimSpace(raw), 10, 16)
	if err != nil || parsed == 0 {
		return 0, fmt.Errorf("%w: port must be 1-65535", ErrInvalidEndpoint)
	}
	return uint16(parsed), nil
}

func ValidateEndpointShape(endpoint Endpoint) error {
	host := strings.TrimSpace(endpoint.Host)
	if host == "" {
		return fmt.Errorf("%w: host is required", ErrInvalidEndpoint)
	}
	if endpoint.Port == 0 {
		return fmt.Errorf("%w: port is required", ErrInvalidEndpoint)
	}
	if len([]rune(host)) > 253 {
		return fmt.Errorf("%w: host is too long", ErrInvalidEndpoint)
	}
	if strings.ContainsAny(host, "/?#\\") {
		return fmt.Errorf("%w: host must not contain URL syntax", ErrInvalidEndpoint)
	}
	return nil
}

func ValidateEndpointForPublicCheck(endpoint Endpoint) error {
	if err := ValidateEndpointShape(endpoint); err != nil {
		return err
	}
	host := strings.ToLower(strings.TrimSpace(endpoint.Host))
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return ErrUnsafeEndpoint
	}
	if addr, err := netip.ParseAddr(host); err == nil {
		return validatePublicIP(addr)
	}
	if ip := net.ParseIP(host); ip != nil {
		addr, ok := netip.AddrFromSlice(ip)
		if !ok {
			return ErrUnsafeEndpoint
		}
		return validatePublicIP(addr)
	}
	return nil
}

func validatePublicIP(addr netip.Addr) error {
	if addr.Is4In6() {
		addr = addr.Unmap()
	}
	if addr.IsLoopback() ||
		addr.IsPrivate() ||
		addr.IsLinkLocalUnicast() ||
		addr.IsLinkLocalMulticast() ||
		addr.IsMulticast() ||
		addr.IsUnspecified() {
		return ErrUnsafeEndpoint
	}
	if addr.Is6() && addr.IsInterfaceLocalMulticast() {
		return ErrUnsafeEndpoint
	}
	return nil
}

func normalizeHost(host string) string {
	host = strings.TrimSpace(host)
	if strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") && len(host) > 1 {
		host = strings.TrimSpace(host[1 : len(host)-1])
	}
	return strings.ToLower(host)
}

func Slug(value string) string {
	var builder strings.Builder
	lastHyphen := false
	for _, r := range strings.ToLower(strings.TrimSpace(value)) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			builder.WriteRune(r)
			lastHyphen = false
			continue
		}
		if !lastHyphen && builder.Len() > 0 {
			builder.WriteByte('-')
			lastHyphen = true
		}
	}
	slug := strings.Trim(builder.String(), "-")
	if slug == "" {
		return "basis-server"
	}
	return slug
}

func NormalizeLanguage(raw string) (string, error) {
	value := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(raw), "_", "-"))
	if value == "" {
		return "", nil
	}
	if alias, ok := languageAliases[value]; ok {
		return alias, nil
	}
	if len([]rune(value)) > 32 {
		return "", fmt.Errorf("%w: language is too long", ErrInvalidLanguage)
	}
	if value == "multi" {
		return value, nil
	}
	parts := strings.Split(value, "-")
	if len(parts) == 0 || !asciiAlpha(parts[0], 2, 3) {
		return "", fmt.Errorf("%w: use a language tag such as en, ru, pt-br, or multi", ErrInvalidLanguage)
	}
	for _, part := range parts[1:] {
		if !asciiAlnum(part, 2, 8) {
			return "", fmt.Errorf("%w: use a language tag such as en, ru, pt-br, or multi", ErrInvalidLanguage)
		}
	}
	return value, nil
}

var languageAliases = map[string]string{
	"english":       "en",
	"английский":    "en",
	"russian":       "ru",
	"русский":       "ru",
	"multilingual":  "multi",
	"мультиязычный": "multi",
}

func asciiAlpha(value string, minLength int, maxLength int) bool {
	if len(value) < minLength || len(value) > maxLength {
		return false
	}
	for _, r := range value {
		if r < 'a' || r > 'z' {
			return false
		}
	}
	return true
}

func asciiAlnum(value string, minLength int, maxLength int) bool {
	if len(value) < minLength || len(value) > maxLength {
		return false
	}
	for _, r := range value {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') {
			return false
		}
	}
	return true
}

type PublicItem struct {
	ID               string     `json:"id"`
	Slug             string     `json:"slug"`
	Name             string     `json:"name"`
	Description      string     `json:"description"`
	Visibility       string     `json:"visibility"`
	Status           string     `json:"status"`
	Host             string     `json:"host"`
	Port             uint16     `json:"port"`
	HasPassword      bool       `json:"has_password"`
	ConnectionString string     `json:"connection_string"`
	Region           string     `json:"region"`
	Language         string     `json:"language"`
	NSFW             bool       `json:"nsfw"`
	Tags             []Tag      `json:"tags"`
	Owner            Owner      `json:"owner"`
	Check            CheckState `json:"check"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	PublishedAt      *time.Time `json:"published_at,omitempty"`
}

type PublicDetail struct {
	PublicItem
	Rules      string  `json:"rules"`
	DiscordURL *string `json:"discord_url,omitempty"`
	WebsiteURL *string `json:"website_url,omitempty"`
}

type OwnerItem struct {
	PublicDetail
	Password *string `json:"password,omitempty"`
}

type Owner struct {
	ID            string  `json:"id"`
	Username      string  `json:"username"`
	DisplayName   *string `json:"display_name,omitempty"`
	AvatarImageID *string `json:"avatar_image_id,omitempty"`
}

type Tag struct {
	Slug string `json:"slug"`
	Name string `json:"name"`
}

type CheckState struct {
	Status          string     `json:"status"`
	OnlinePlayers   *int       `json:"online_players,omitempty"`
	MaxPlayers      *int       `json:"max_players,omitempty"`
	ProtocolVersion *int       `json:"protocol_version,omitempty"`
	ServerName      *string    `json:"server_name,omitempty"`
	Motd            *string    `json:"motd,omitempty"`
	RoundTripMS     *int       `json:"round_trip_ms,omitempty"`
	LastCheckedAt   *time.Time `json:"last_checked_at,omitempty"`
	LastError       *string    `json:"last_error,omitempty"`
}

type ListFilter struct {
	Query       string
	Region      string
	Language    string
	Tags        []string
	OnlineOnly  bool
	IncludeNSFW bool
	Sort        string
	Limit       int
	Cursor      Cursor
}

type OwnerListFilter struct {
	Query  string
	Status string
	Limit  int
	Cursor Cursor
}

type Cursor struct {
	SortValue string `json:"sort_value"`
	ID        string `json:"id"`
}

type Page struct {
	Items      []PublicItem
	NextCursor *Cursor
}

type OwnerPage struct {
	Items      []OwnerItem
	NextCursor *Cursor
}

type CreateInput struct {
	OwnerID     string
	Name        string
	Description string
	Host        string
	Port        uint16
	Password    *string
	Visibility  string
	Region      string
	Language    string
	NSFW        bool
	Tags        []TagInput
	Rules       string
	DiscordURL  *string
	WebsiteURL  *string
}

type UpdateInput struct {
	Name        *string
	Description *string
	Host        *string
	Port        *uint16
	Password    **string
	Visibility  *string
	Region      *string
	Language    *string
	NSFW        *bool
	Tags        *[]TagInput
	Rules       *string
	DiscordURL  **string
	WebsiteURL  **string
}

type TagInput struct {
	Slug string
	Name string
}

type CheckPayload struct {
	ServerID string `json:"server_id"`
}

type CheckResult struct {
	Status          string
	OnlinePlayers   *int
	MaxPlayers      *int
	ProtocolVersion *int
	ServerName      *string
	Motd            *string
	RoundTripMS     *int
	LastError       *string
	CheckedAt       time.Time
}
