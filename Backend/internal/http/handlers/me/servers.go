package me

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/url"
	"strconv"
	"strings"
	"time"

	domain "beeba.org/internal/domain/servers"
	"beeba.org/internal/http/middleware/authz"
	"beeba.org/internal/servercheck"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type ServerStore interface {
	ListOwned(ctx context.Context, ownerID string, filter domain.OwnerListFilter) (domain.OwnerPage, error)
	Create(ctx context.Context, input domain.CreateInput) (domain.OwnerItem, error)
	UpdateOwned(ctx context.Context, ownerID string, serverID string, input domain.UpdateInput) (domain.OwnerItem, error)
	DeleteOwned(ctx context.Context, ownerID string, serverID string) error
	RequestVerification(ctx context.Context, ownerID string, serverID string) (domain.OwnerItem, error)
}

type ServerHandler struct {
	store ServerStore
}

func NewServerHandler(store ServerStore) ServerHandler {
	return ServerHandler{store: store}
}

const serverProbeTimeout = 3 * time.Second

type serverCreateRequest struct {
	Name             string   `json:"name"`
	Description      string   `json:"description"`
	ConnectionString string   `json:"connection_string"`
	Host             string   `json:"host"`
	Port             *int     `json:"port"`
	Password         *string  `json:"password"`
	Visibility       string   `json:"visibility"`
	Region           string   `json:"region"`
	Language         string   `json:"language"`
	NSFW             bool     `json:"nsfw"`
	Tags             []string `json:"tags"`
	Rules            string   `json:"rules"`
	DiscordURL       *string  `json:"discord_url"`
	WebsiteURL       *string  `json:"website_url"`
}

type serverUpdateRequest struct {
	Name             *string  `json:"name"`
	Description      *string  `json:"description"`
	ConnectionString *string  `json:"connection_string"`
	Host             *string  `json:"host"`
	Port             *int     `json:"port"`
	Password         *string  `json:"password"`
	Visibility       *string  `json:"visibility"`
	Region           *string  `json:"region"`
	Language         *string  `json:"language"`
	NSFW             *bool    `json:"nsfw"`
	Tags             []string `json:"tags"`
	Rules            *string  `json:"rules"`
	DiscordURL       *string  `json:"discord_url"`
	WebsiteURL       *string  `json:"website_url"`
}

type serverProbeRequest struct {
	ConnectionString string `json:"connection_string"`
}

type serverProbeData struct {
	Host            string  `json:"host"`
	Port            uint16  `json:"port"`
	HasPassword     bool    `json:"has_password"`
	OnlinePlayers   int     `json:"online_players"`
	MaxPlayers      int     `json:"max_players"`
	ProtocolVersion int     `json:"protocol_version"`
	ServerName      *string `json:"server_name,omitempty"`
	Motd            *string `json:"motd,omitempty"`
	RoundTripMS     int     `json:"round_trip_ms"`
}

func (h ServerHandler) List(c fiber.Ctx) error {
	if h.store == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "server repository unavailable")
	}
	session, ok := authz.CurrentSession(c)
	if !ok {
		return fiber.NewError(fiber.StatusUnauthorized, "Missing authenticated session")
	}
	limit, err := parseServerLimit(c.Query("limit"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}
	cursor, err := decodeServerCursor(c.Query("cursor"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}
	filter := domain.OwnerListFilter{
		Query:  strings.TrimSpace(c.Query("q")),
		Status: strings.TrimSpace(c.Query("status")),
		Limit:  limit,
		Cursor: cursor,
	}
	if filter.Status != "" && !allowedOwnerServerStatus(filter.Status) {
		return fiber.NewError(fiber.StatusBadRequest, "status is invalid")
	}
	ctx, cancel := context.WithTimeout(c.Context(), 2*time.Second)
	defer cancel()
	page, err := h.store.ListOwned(ctx, session.User.ID, filter)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "Failed to load owned servers")
	}
	return c.JSON(fiber.Map{
		"data": page.Items,
		"pagination": fiber.Map{
			"next_cursor": encodeServerCursor(page.NextCursor),
			"limit":       limit,
		},
	})
}

func (h ServerHandler) Create(c fiber.Ctx) error {
	if h.store == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "server repository unavailable")
	}
	session, ok := authz.CurrentSession(c)
	if !ok {
		return fiber.NewError(fiber.StatusUnauthorized, "Missing authenticated session")
	}
	var request serverCreateRequest
	if err := c.Bind().Body(&request); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid JSON body")
	}
	input, err := normalizeCreateServerRequest(session.User.ID, request)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}
	ctx, cancel := context.WithTimeout(c.Context(), 5*time.Second)
	defer cancel()
	item, err := h.store.Create(ctx, input)
	if err != nil {
		return serverMutationError(err)
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"data": item})
}

func (h ServerHandler) Update(c fiber.Ctx) error {
	if h.store == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "server repository unavailable")
	}
	session, ok := authz.CurrentSession(c)
	if !ok {
		return fiber.NewError(fiber.StatusUnauthorized, "Missing authenticated session")
	}
	serverID := strings.TrimSpace(c.Params("serverID"))
	if _, err := uuid.Parse(serverID); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "server id is invalid")
	}
	var request serverUpdateRequest
	if err := c.Bind().Body(&request); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid JSON body")
	}
	input, err := normalizeUpdateServerRequest(request)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}
	ctx, cancel := context.WithTimeout(c.Context(), 5*time.Second)
	defer cancel()
	item, err := h.store.UpdateOwned(ctx, session.User.ID, serverID, input)
	if err != nil {
		return serverMutationError(err)
	}
	return c.JSON(fiber.Map{"data": item})
}

func (h ServerHandler) Delete(c fiber.Ctx) error {
	if h.store == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "server repository unavailable")
	}
	session, ok := authz.CurrentSession(c)
	if !ok {
		return fiber.NewError(fiber.StatusUnauthorized, "Missing authenticated session")
	}
	serverID := strings.TrimSpace(c.Params("serverID"))
	if _, err := uuid.Parse(serverID); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "server id is invalid")
	}
	ctx, cancel := context.WithTimeout(c.Context(), 3*time.Second)
	defer cancel()
	if err := h.store.DeleteOwned(ctx, session.User.ID, serverID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fiber.NewError(fiber.StatusNotFound, "owned server not found")
		}
		return fiber.NewError(fiber.StatusInternalServerError, "Failed to delete server")
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h ServerHandler) Verify(c fiber.Ctx) error {
	if h.store == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "server repository unavailable")
	}
	session, ok := authz.CurrentSession(c)
	if !ok {
		return fiber.NewError(fiber.StatusUnauthorized, "Missing authenticated session")
	}
	serverID := strings.TrimSpace(c.Params("serverID"))
	if _, err := uuid.Parse(serverID); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "server id is invalid")
	}
	ctx, cancel := context.WithTimeout(c.Context(), 3*time.Second)
	defer cancel()
	item, err := h.store.RequestVerification(ctx, session.User.ID, serverID)
	if err != nil {
		return serverMutationError(err)
	}
	return c.JSON(fiber.Map{"data": item})
}

func (h ServerHandler) Probe(c fiber.Ctx) error {
	if _, ok := authz.CurrentSession(c); !ok {
		return fiber.NewError(fiber.StatusUnauthorized, "Missing authenticated session")
	}
	var request serverProbeRequest
	if err := c.Bind().Body(&request); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid JSON body")
	}
	endpoint, err := domain.ParseConnectionString(request.ConnectionString)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	ctx, cancel := context.WithTimeout(c.Context(), serverProbeTimeout+500*time.Millisecond)
	defer cancel()
	info, err := servercheck.Probe(ctx, endpoint.Host, endpoint.Port, serverProbeTimeout)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidEndpoint) || errors.Is(err, domain.ErrUnsafeEndpoint) {
			return fiber.NewError(fiber.StatusBadRequest, err.Error())
		}
		return fiber.NewError(fiber.StatusServiceUnavailable, "Server did not respond to the Basis server-info query")
	}

	return c.JSON(fiber.Map{
		"data": serverProbeData{
			Host:            endpoint.Host,
			Port:            endpoint.Port,
			HasPassword:     endpoint.Password != nil,
			OnlinePlayers:   info.OnlinePlayers,
			MaxPlayers:      info.MaxPlayers,
			ProtocolVersion: info.ProtocolVersion,
			ServerName:      optionalString(truncateTrimmed(info.ServerName, 80)),
			Motd:            optionalString(truncateTrimmed(info.Motd, 1200)),
			RoundTripMS:     info.RoundTripMS,
		},
	})
}

func normalizeCreateServerRequest(ownerID string, request serverCreateRequest) (domain.CreateInput, error) {
	name, err := normalizeServerName(request.Name)
	if err != nil {
		return domain.CreateInput{}, err
	}
	endpoint, err := endpointFromCreateRequest(request)
	if err != nil {
		return domain.CreateInput{}, err
	}
	visibility := normalizeServerVisibility(request.Visibility)
	tags, err := normalizeServerTags(request.Tags)
	if err != nil {
		return domain.CreateInput{}, err
	}
	discordURL, err := normalizeOptionalHTTPURL(request.DiscordURL)
	if err != nil {
		return domain.CreateInput{}, err
	}
	websiteURL, err := normalizeOptionalHTTPURL(request.WebsiteURL)
	if err != nil {
		return domain.CreateInput{}, err
	}
	language, err := domain.NormalizeLanguage(request.Language)
	if err != nil {
		return domain.CreateInput{}, err
	}
	return domain.CreateInput{
		OwnerID:     ownerID,
		Name:        name,
		Description: truncateTrimmed(request.Description, 1200),
		Host:        endpoint.Host,
		Port:        endpoint.Port,
		Password:    endpoint.Password,
		Visibility:  visibility,
		Region:      truncateTrimmed(request.Region, 64),
		Language:    language,
		NSFW:        request.NSFW,
		Tags:        tags,
		Rules:       truncateTrimmed(request.Rules, 3000),
		DiscordURL:  discordURL,
		WebsiteURL:  websiteURL,
	}, nil
}

func normalizeUpdateServerRequest(request serverUpdateRequest) (domain.UpdateInput, error) {
	input := domain.UpdateInput{}
	hasField := false
	if request.Name != nil {
		name, err := normalizeServerName(*request.Name)
		if err != nil {
			return domain.UpdateInput{}, err
		}
		input.Name = &name
		hasField = true
	}
	if request.Description != nil {
		description := truncateTrimmed(*request.Description, 1200)
		input.Description = &description
		hasField = true
	}
	if request.ConnectionString != nil && strings.TrimSpace(*request.ConnectionString) != "" {
		endpoint, err := domain.ParseConnectionString(*request.ConnectionString)
		if err != nil {
			return domain.UpdateInput{}, err
		}
		input.Host = &endpoint.Host
		input.Port = &endpoint.Port
		input.Password = &endpoint.Password
		hasField = true
	}
	if request.Host != nil {
		host := strings.ToLower(strings.TrimSpace(*request.Host))
		input.Host = &host
		hasField = true
	}
	if request.Port != nil {
		port, err := normalizePort(*request.Port)
		if err != nil {
			return domain.UpdateInput{}, err
		}
		input.Port = &port
		hasField = true
	}
	if request.Password != nil {
		password := normalizePassword(request.Password)
		input.Password = &password
		hasField = true
	}
	if request.Visibility != nil {
		visibility := normalizeServerVisibility(*request.Visibility)
		input.Visibility = &visibility
		hasField = true
	}
	if request.Region != nil {
		region := truncateTrimmed(*request.Region, 64)
		input.Region = &region
		hasField = true
	}
	if request.Language != nil {
		language, err := domain.NormalizeLanguage(*request.Language)
		if err != nil {
			return domain.UpdateInput{}, err
		}
		input.Language = &language
		hasField = true
	}
	if request.NSFW != nil {
		input.NSFW = request.NSFW
		hasField = true
	}
	if request.Tags != nil {
		tags, err := normalizeServerTags(request.Tags)
		if err != nil {
			return domain.UpdateInput{}, err
		}
		input.Tags = &tags
		hasField = true
	}
	if request.Rules != nil {
		rules := truncateTrimmed(*request.Rules, 3000)
		input.Rules = &rules
		hasField = true
	}
	if request.DiscordURL != nil {
		discordURL, err := normalizeOptionalHTTPURL(request.DiscordURL)
		if err != nil {
			return domain.UpdateInput{}, err
		}
		input.DiscordURL = &discordURL
		hasField = true
	}
	if request.WebsiteURL != nil {
		websiteURL, err := normalizeOptionalHTTPURL(request.WebsiteURL)
		if err != nil {
			return domain.UpdateInput{}, err
		}
		input.WebsiteURL = &websiteURL
		hasField = true
	}
	if !hasField {
		return domain.UpdateInput{}, errors.New("at least one editable field is required")
	}
	return input, nil
}

func endpointFromCreateRequest(request serverCreateRequest) (domain.Endpoint, error) {
	if strings.TrimSpace(request.ConnectionString) != "" {
		return domain.ParseConnectionString(request.ConnectionString)
	}
	port := domain.DefaultPort
	if request.Port != nil {
		parsed, err := normalizePort(*request.Port)
		if err != nil {
			return domain.Endpoint{}, err
		}
		port = parsed
	}
	return domain.Endpoint{
		Host:     strings.ToLower(strings.TrimSpace(request.Host)),
		Port:     port,
		Password: normalizePassword(request.Password),
	}, nil
}

func normalizeServerName(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", errors.New("name is required")
	}
	if len([]rune(value)) > 80 {
		return "", errors.New("name must be 80 characters or fewer")
	}
	return value, nil
}

func normalizePort(raw int) (uint16, error) {
	if raw <= 0 || raw > 65535 {
		return 0, errors.New("port must be 1-65535")
	}
	return uint16(raw), nil
}

func normalizePassword(raw *string) *string {
	if raw == nil {
		return nil
	}
	value := strings.TrimSpace(*raw)
	if value == "" {
		return nil
	}
	return &value
}

func normalizeServerVisibility(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	if value == domain.VisibilityUnlisted {
		return domain.VisibilityUnlisted
	}
	return domain.VisibilityPublic
}

func normalizeServerTags(rawTags []string) ([]domain.TagInput, error) {
	if len(rawTags) > 16 {
		return nil, errors.New("tags must contain 16 items or fewer")
	}
	tags := make([]domain.TagInput, 0, len(rawTags))
	seen := map[string]struct{}{}
	for _, raw := range rawTags {
		name := strings.TrimSpace(raw)
		if name == "" {
			continue
		}
		if len([]rune(name)) > 40 {
			return nil, errors.New("each tag must be 40 characters or fewer")
		}
		slug := tagSlug(name)
		if slug == "" {
			return nil, errors.New("each tag must include a letter or number")
		}
		if _, ok := seen[slug]; ok {
			continue
		}
		seen[slug] = struct{}{}
		tags = append(tags, domain.TagInput{Slug: slug, Name: name})
	}
	return tags, nil
}

func normalizeOptionalHTTPURL(raw *string) (*string, error) {
	if raw == nil {
		return nil, nil
	}
	value := strings.TrimSpace(*raw)
	if value == "" {
		return nil, nil
	}
	if len(value) > 500 {
		return nil, errors.New("url must be 500 characters or fewer")
	}
	parsed, err := url.ParseRequestURI(value)
	if err != nil {
		return nil, errors.New("url is invalid")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, errors.New("url must start with http or https")
	}
	return &value, nil
}

func truncateTrimmed(value string, limit int) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) <= limit {
		return string(runes)
	}
	return string(runes[:limit])
}

func optionalString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func parseServerLimit(raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return defaultOwnedLimit, nil
	}
	limit, err := strconv.Atoi(raw)
	if err != nil || limit <= 0 {
		return 0, errors.New("limit must be a positive integer")
	}
	if limit > maxOwnedLimit {
		return maxOwnedLimit, nil
	}
	return limit, nil
}

func encodeServerCursor(cursor *domain.Cursor) *string {
	if cursor == nil || cursor.ID == "" || cursor.SortValue == "" {
		return nil
	}
	payload, err := json.Marshal(cursor)
	if err != nil {
		return nil
	}
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	return &encoded
}

func decodeServerCursor(raw string) (domain.Cursor, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return domain.Cursor{}, nil
	}
	payload, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return domain.Cursor{}, errors.New("cursor is invalid")
	}
	var cursor domain.Cursor
	if err := json.Unmarshal(payload, &cursor); err != nil {
		return domain.Cursor{}, errors.New("cursor is invalid")
	}
	if cursor.ID == "" || cursor.SortValue == "" {
		return domain.Cursor{}, errors.New("cursor is invalid")
	}
	return cursor, nil
}

func allowedOwnerServerStatus(status string) bool {
	switch status {
	case domain.StatusDraft,
		domain.StatusPendingVerification,
		domain.StatusPendingModeration,
		domain.StatusPublished,
		domain.StatusOffline,
		domain.StatusFailed,
		domain.StatusHidden,
		domain.StatusDeleted:
		return true
	default:
		return false
	}
}

func serverMutationError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return fiber.NewError(fiber.StatusNotFound, "owned server not found")
	}
	if errors.Is(err, domain.ErrInvalidEndpoint) || errors.Is(err, domain.ErrUnsafeEndpoint) {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}
	return fiber.NewError(fiber.StatusInternalServerError, "Failed to update server catalog")
}
