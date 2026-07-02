package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"beeba.org/internal/domain/servers"
	"beeba.org/internal/security/secretbox"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ServerRepository struct {
	db      *pgxpool.Pool
	secrets *secretbox.Box
}

func NewServerRepository(db *pgxpool.Pool, secrets secretbox.Box) ServerRepository {
	return ServerRepository{db: db, secrets: &secrets}
}

func (r ServerRepository) ListPublished(ctx context.Context, filter servers.ListFilter) (servers.Page, error) {
	sortExpr, cursorCondition, err := serverSort(filter.Sort)
	if err != nil {
		return servers.Page{}, err
	}

	args := []any{filter.Limit + 1}
	conditions := []string{
		"(sl.status = 'published' OR (sl.status = 'offline' AND sl.published_at IS NOT NULL))",
		"sl.visibility = 'public'",
		"sl.deleted_at IS NULL",
		"sl.hidden_at IS NULL",
	}
	if !filter.IncludeNSFW {
		conditions = append(conditions, "sl.nsfw = false")
	}
	if filter.OnlineOnly {
		conditions = append(conditions, "sl.check_status = 'online'")
	}
	if filter.Query != "" {
		args = append(args, "%"+strings.ToLower(filter.Query)+"%")
		conditions = append(conditions, fmt.Sprintf(`(
  lower(sl.name) LIKE $%d OR
  lower(sl.description) LIKE $%d OR
  lower(sl.check_server_name) LIKE $%d OR
  lower(sl.check_motd) LIKE $%d
)`, len(args), len(args), len(args), len(args)))
	}
	if filter.Region != "" {
		args = append(args, strings.ToLower(filter.Region))
		conditions = append(conditions, fmt.Sprintf("lower(sl.region) = $%d", len(args)))
	}
	if filter.Language != "" {
		args = append(args, strings.ToLower(filter.Language))
		conditions = append(conditions, fmt.Sprintf("lower(sl.language) = $%d", len(args)))
	}
	if len(filter.Tags) > 0 {
		args = append(args, filter.Tags)
		conditions = append(conditions, fmt.Sprintf(`EXISTS (
  SELECT 1
  FROM server_tags filter_st
  JOIN tags filter_t ON filter_t.id = filter_st.tag_id
  WHERE filter_st.server_id = sl.id AND filter_t.slug = ANY($%d::text[])
)`, len(args)))
	}
	if filter.Cursor.ID != "" && filter.Cursor.SortValue != "" {
		args = append(args, filter.Cursor.SortValue, filter.Cursor.ID)
		conditions = append(conditions, fmt.Sprintf(cursorCondition, len(args)-1, len(args)))
	}

	rows, err := r.db.Query(ctx, fmt.Sprintf(serverSelectSQL(`
WHERE %s
GROUP BY sl.id, u.id, u.username, u.display_name, avatar.id
ORDER BY %s DESC, sl.id DESC
LIMIT $1`), joinConditions(conditions), sortExpr), args...)
	if err != nil {
		return servers.Page{}, fmt.Errorf("query published servers: %w", err)
	}
	defer rows.Close()

	items := make([]servers.PublicItem, 0, filter.Limit)
	for rows.Next() {
		item, _, err := r.scanServerPublic(rows)
		if err != nil {
			return servers.Page{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return servers.Page{}, fmt.Errorf("read published servers: %w", err)
	}

	page := servers.Page{Items: items}
	if len(items) > filter.Limit {
		page.Items = items[:filter.Limit]
		last := page.Items[len(page.Items)-1]
		page.NextCursor = nextServerCursor(last, filter.Sort)
	}
	return page, nil
}

func (r ServerRepository) GetPublished(ctx context.Context, serverID string) (servers.PublicDetail, error) {
	row := r.db.QueryRow(ctx, serverSelectSQL(`
WHERE sl.id = $1::uuid
  AND (sl.status = 'published' OR (sl.status = 'offline' AND sl.published_at IS NOT NULL))
  AND sl.deleted_at IS NULL
  AND sl.hidden_at IS NULL
GROUP BY sl.id, u.id, u.username, u.display_name, avatar.id
LIMIT 1`), serverID)
	item, detail, err := r.scanServerPublic(row)
	if err != nil {
		return servers.PublicDetail{}, err
	}
	detail.PublicItem = item
	return detail, nil
}

func (r ServerRepository) ListOwned(ctx context.Context, ownerID string, filter servers.OwnerListFilter) (servers.OwnerPage, error) {
	args := []any{ownerID, filter.Limit + 1}
	conditions := []string{
		"sl.owner_id = $1::uuid",
		"sl.deleted_at IS NULL",
	}
	if filter.Query != "" {
		args = append(args, "%"+strings.ToLower(filter.Query)+"%")
		conditions = append(conditions, fmt.Sprintf("(lower(sl.name) LIKE $%d OR lower(sl.host) LIKE $%d)", len(args), len(args)))
	}
	if filter.Status != "" {
		args = append(args, filter.Status)
		conditions = append(conditions, fmt.Sprintf("sl.status = $%d", len(args)))
	}
	if filter.Cursor.ID != "" && filter.Cursor.SortValue != "" {
		args = append(args, filter.Cursor.SortValue, filter.Cursor.ID)
		conditions = append(conditions, fmt.Sprintf("(sl.updated_at, sl.id) < ($%d::timestamptz, $%d::uuid)", len(args)-1, len(args)))
	}

	rows, err := r.db.Query(ctx, fmt.Sprintf(serverSelectSQL(`
WHERE %s
GROUP BY sl.id, u.id, u.username, u.display_name, avatar.id
ORDER BY sl.updated_at DESC, sl.id DESC
LIMIT $2`), joinConditions(conditions)), args...)
	if err != nil {
		return servers.OwnerPage{}, fmt.Errorf("query owned servers: %w", err)
	}
	defer rows.Close()

	items := make([]servers.OwnerItem, 0, filter.Limit)
	for rows.Next() {
		item, detail, encryptedPassword, err := r.scanServerOwner(rows)
		if err != nil {
			return servers.OwnerPage{}, err
		}
		ownerItem := servers.OwnerItem{PublicDetail: detail}
		ownerItem.PublicItem = item
		if encryptedPassword != nil && r.secrets != nil {
			password, err := r.secrets.DecryptString(*encryptedPassword)
			if err != nil {
				return servers.OwnerPage{}, fmt.Errorf("decrypt owned server password: %w", err)
			}
			ownerItem.Password = &password
		}
		items = append(items, ownerItem)
	}
	if err := rows.Err(); err != nil {
		return servers.OwnerPage{}, fmt.Errorf("read owned servers: %w", err)
	}

	page := servers.OwnerPage{Items: items}
	if len(items) > filter.Limit {
		page.Items = items[:filter.Limit]
		last := page.Items[len(page.Items)-1]
		page.NextCursor = &servers.Cursor{ID: last.ID, SortValue: last.UpdatedAt.Format(timeFormatRFC3339Micro)}
	}
	return page, nil
}

func (r ServerRepository) Create(ctx context.Context, input servers.CreateInput) (servers.OwnerItem, error) {
	endpoint := servers.Endpoint{Host: input.Host, Port: input.Port, Password: input.Password}
	if err := servers.ValidateEndpointForPublicCheck(endpoint); err != nil {
		return servers.OwnerItem{}, err
	}

	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return servers.OwnerItem{}, fmt.Errorf("begin server create: %w", err)
	}
	defer tx.Rollback(ctx)

	serverID := uuid.NewString()
	slug := servers.Slug(input.Name)
	encryptedPassword, err := r.encryptOptional(input.Password)
	if err != nil {
		return servers.OwnerItem{}, err
	}

	if _, err := tx.Exec(ctx, `
INSERT INTO server_listings (
  id,
  owner_id,
  name,
  slug,
  description,
  host,
  port,
  password_ciphertext,
  visibility,
  status,
  region,
  language,
  nsfw,
  rules,
  discord_url,
  website_url
)
VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, NULLIF($8, ''), $9, 'pending_verification', $10, $11, $12, $13, NULLIF($14, ''), NULLIF($15, ''))`,
		serverID,
		input.OwnerID,
		input.Name,
		slug,
		input.Description,
		strings.ToLower(strings.TrimSpace(input.Host)),
		input.Port,
		encryptedPassword,
		input.Visibility,
		input.Region,
		input.Language,
		input.NSFW,
		input.Rules,
		optionalString(input.DiscordURL),
		optionalString(input.WebsiteURL),
	); err != nil {
		return servers.OwnerItem{}, fmt.Errorf("insert server listing: %w", err)
	}

	if err := replaceServerTagsTx(ctx, tx, serverID, input.Tags); err != nil {
		return servers.OwnerItem{}, err
	}
	if err := enqueueServerCheckTx(ctx, tx, serverID); err != nil {
		return servers.OwnerItem{}, err
	}

	item, err := getOwnedServerTx(ctx, tx, input.OwnerID, serverID, r.secrets)
	if err != nil {
		return servers.OwnerItem{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return servers.OwnerItem{}, fmt.Errorf("commit server create: %w", err)
	}
	return item, nil
}

func (r ServerRepository) UpdateOwned(ctx context.Context, ownerID string, serverID string, input servers.UpdateInput) (servers.OwnerItem, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return servers.OwnerItem{}, fmt.Errorf("begin server update: %w", err)
	}
	defer tx.Rollback(ctx)

	before, encryptedBefore, err := getOwnedServerForUpdateTx(ctx, tx, ownerID, serverID, r.secrets)
	if err != nil {
		return servers.OwnerItem{}, err
	}
	host := before.Host
	port := before.Port
	if input.Host != nil {
		host = strings.ToLower(strings.TrimSpace(*input.Host))
	}
	if input.Port != nil {
		port = *input.Port
	}
	passwordCiphertext := encryptedBefore
	if input.Password != nil {
		encrypted, err := r.encryptOptional(*input.Password)
		if err != nil {
			return servers.OwnerItem{}, err
		}
		if encrypted == "" {
			passwordCiphertext = nil
		} else {
			passwordCiphertext = &encrypted
		}
	}
	endpointPassword := input.Password
	if endpointPassword == nil {
		endpointPassword = &before.Password
	}
	if err := servers.ValidateEndpointForPublicCheck(servers.Endpoint{Host: host, Port: port, Password: *endpointPassword}); err != nil {
		return servers.OwnerItem{}, err
	}

	name := before.Name
	slug := before.Slug
	if input.Name != nil {
		name = strings.TrimSpace(*input.Name)
		slug = servers.Slug(name)
	}
	description := before.Description
	if input.Description != nil {
		description = strings.TrimSpace(*input.Description)
	}
	visibility := before.Visibility
	if input.Visibility != nil {
		visibility = strings.TrimSpace(*input.Visibility)
	}
	region := before.Region
	if input.Region != nil {
		region = strings.TrimSpace(*input.Region)
	}
	language := before.Language
	if input.Language != nil {
		language = strings.TrimSpace(*input.Language)
	}
	nsfw := before.NSFW
	if input.NSFW != nil {
		nsfw = *input.NSFW
	}
	rules := before.Rules
	if input.Rules != nil {
		rules = strings.TrimSpace(*input.Rules)
	}
	discordURL := before.DiscordURL
	if input.DiscordURL != nil {
		discordURL = *input.DiscordURL
	}
	websiteURL := before.WebsiteURL
	if input.WebsiteURL != nil {
		websiteURL = *input.WebsiteURL
	}

	endpointChanged := host != before.Host || port != before.Port || !sameOptionalString(passwordCiphertext, encryptedBefore)
	statusExpr := "status"
	checkStatusExpr := "check_status"
	if endpointChanged {
		statusExpr = "'pending_verification'"
		checkStatusExpr = "'pending'"
	}
	sql := fmt.Sprintf(`
UPDATE server_listings
SET
  name = $3,
  slug = $4,
  description = $5,
  host = $6,
  port = $7,
  password_ciphertext = $8,
  visibility = $9,
  region = $10,
  language = $11,
  nsfw = $12,
  rules = $13,
  discord_url = NULLIF($14, ''),
  website_url = NULLIF($15, ''),
  status = %s,
  check_status = %s,
  updated_at = now()
WHERE id = $1::uuid AND owner_id = $2::uuid AND deleted_at IS NULL`, statusExpr, checkStatusExpr)
	if _, err := tx.Exec(ctx, sql,
		serverID, ownerID, name, slug, description, host, port, passwordCiphertext, visibility,
		region, language, nsfw, rules, optionalString(discordURL), optionalString(websiteURL),
	); err != nil {
		return servers.OwnerItem{}, fmt.Errorf("update server listing: %w", err)
	}
	if input.Tags != nil {
		if err := replaceServerTagsTx(ctx, tx, serverID, *input.Tags); err != nil {
			return servers.OwnerItem{}, err
		}
	}
	if endpointChanged {
		if err := enqueueServerCheckTx(ctx, tx, serverID); err != nil {
			return servers.OwnerItem{}, err
		}
	}
	item, err := getOwnedServerTx(ctx, tx, ownerID, serverID, r.secrets)
	if err != nil {
		return servers.OwnerItem{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return servers.OwnerItem{}, fmt.Errorf("commit server update: %w", err)
	}
	return item, nil
}

func (r ServerRepository) DeleteOwned(ctx context.Context, ownerID string, serverID string) error {
	tag, err := r.db.Exec(ctx, `
UPDATE server_listings
SET status = 'deleted', deleted_at = now(), updated_at = now()
WHERE id = $1::uuid AND owner_id = $2::uuid AND deleted_at IS NULL`, serverID, ownerID)
	if err != nil {
		return fmt.Errorf("delete owned server: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (r ServerRepository) RequestVerification(ctx context.Context, ownerID string, serverID string) (servers.OwnerItem, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return servers.OwnerItem{}, fmt.Errorf("begin server verification request: %w", err)
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `
UPDATE server_listings
SET status = 'pending_verification', check_status = 'pending', check_error = NULL, updated_at = now()
WHERE id = $1::uuid AND owner_id = $2::uuid AND deleted_at IS NULL`, serverID, ownerID)
	if err != nil {
		return servers.OwnerItem{}, fmt.Errorf("mark server pending verification: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return servers.OwnerItem{}, pgx.ErrNoRows
	}
	if err := enqueueServerCheckTx(ctx, tx, serverID); err != nil {
		return servers.OwnerItem{}, err
	}
	item, err := getOwnedServerTx(ctx, tx, ownerID, serverID, r.secrets)
	if err != nil {
		return servers.OwnerItem{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return servers.OwnerItem{}, fmt.Errorf("commit server verification request: %w", err)
	}
	return item, nil
}

func (r ServerRepository) EnqueueDueChecks(ctx context.Context, pendingInterval time.Duration, onlineInterval time.Duration, offlineInterval time.Duration, limit int) (int64, error) {
	if limit <= 0 {
		return 0, nil
	}

	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return 0, fmt.Errorf("begin enqueue due server checks: %w", err)
	}
	defer tx.Rollback(ctx)

	var locked bool
	if err := tx.QueryRow(ctx, `SELECT pg_try_advisory_xact_lock(4296, 701)`).Scan(&locked); err != nil {
		return 0, fmt.Errorf("acquire server check scheduler lock: %w", err)
	}
	if !locked {
		return 0, nil
	}

	tag, err := tx.Exec(ctx, `
WITH candidates AS (
  SELECT sl.id
  FROM server_listings sl
  WHERE sl.deleted_at IS NULL
    AND sl.status IN ('pending_verification', 'published', 'offline', 'failed')
    AND (
      sl.last_checked_at IS NULL
      OR sl.last_checked_at <= now() - CASE
        WHEN sl.check_status = 'online' THEN $2::bigint * interval '1 millisecond'
        WHEN sl.check_status IN ('offline', 'failed') THEN $3::bigint * interval '1 millisecond'
        ELSE $1::bigint * interval '1 millisecond'
      END
    )
    AND NOT EXISTS (
      SELECT 1
      FROM worker_jobs job
      WHERE job.queue_name = 'server_check_queue'
        AND job.status IN ('pending', 'running')
        AND job.payload->>'server_id' = sl.id::text
    )
  ORDER BY sl.last_checked_at ASC NULLS FIRST, sl.created_at ASC, sl.id ASC
  LIMIT $4
)
INSERT INTO worker_jobs (queue_name, job_type, payload, max_attempts)
SELECT 'server_check_queue', 'check_basis_server', jsonb_build_object('server_id', id::text), 3
FROM candidates`,
		durationMilliseconds(pendingInterval),
		durationMilliseconds(onlineInterval),
		durationMilliseconds(offlineInterval),
		limit,
	)
	if err != nil {
		return 0, fmt.Errorf("insert due server check jobs: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit due server checks: %w", err)
	}
	return tag.RowsAffected(), nil
}

type ServerCheckTarget struct {
	ServerID string
	Host     string
	Port     uint16
}

func (r ServerRepository) GetCheckTarget(ctx context.Context, serverID string) (ServerCheckTarget, error) {
	var target ServerCheckTarget
	var port int
	err := r.db.QueryRow(ctx, `
SELECT id::text, host, port
FROM server_listings
WHERE id = $1::uuid AND deleted_at IS NULL AND status <> 'hidden'
LIMIT 1`, serverID).Scan(&target.ServerID, &target.Host, &port)
	if err != nil {
		return ServerCheckTarget{}, err
	}
	target.Port = uint16(port)
	return target, nil
}

func (r ServerRepository) CompleteCheck(ctx context.Context, serverID string, result servers.CheckResult) error {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin complete server check: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
INSERT INTO server_checks (
  server_id,
  status,
  online_players,
  max_players,
  protocol_version,
  server_name,
  motd,
  round_trip_ms,
  error,
  checked_at
)
VALUES ($1::uuid, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		serverID,
		result.Status,
		result.OnlinePlayers,
		result.MaxPlayers,
		result.ProtocolVersion,
		result.ServerName,
		result.Motd,
		result.RoundTripMS,
		result.LastError,
		result.CheckedAt,
	); err != nil {
		return fmt.Errorf("insert server check: %w", err)
	}

	newStatus := "status"
	if result.Status == servers.CheckStatusOnline {
		newStatus = "CASE WHEN status IN ('pending_verification', 'offline', 'failed') THEN 'published' ELSE status END"
	} else if result.Status == servers.CheckStatusOffline {
		newStatus = "CASE WHEN status IN ('pending_verification', 'published') THEN 'offline' ELSE status END"
	} else if result.Status == servers.CheckStatusFailed {
		newStatus = "CASE WHEN status IN ('pending_verification', 'published', 'offline') THEN 'failed' ELSE status END"
	}

	sql := fmt.Sprintf(`
UPDATE server_listings
SET
  check_status = $2,
  check_online_players = $3,
  check_max_players = $4,
  check_protocol_version = $5,
  check_server_name = $6,
  check_motd = $7,
  check_round_trip_ms = $8,
  check_error = $9,
  last_checked_at = $10,
  status = %s,
  published_at = CASE
    WHEN $2 = 'online' AND published_at IS NULL THEN $10
    ELSE published_at
  END,
  updated_at = now()
WHERE id = $1::uuid AND deleted_at IS NULL AND status <> 'hidden'`, newStatus)
	if _, err := tx.Exec(ctx, sql,
		serverID,
		result.Status,
		result.OnlinePlayers,
		result.MaxPlayers,
		result.ProtocolVersion,
		result.ServerName,
		result.Motd,
		result.RoundTripMS,
		result.LastError,
		result.CheckedAt,
	); err != nil {
		return fmt.Errorf("update server check state: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit complete server check: %w", err)
	}
	return nil
}

func (r ServerRepository) Hide(ctx context.Context, actorUserID string, serverID string, reason string, ipAddress string, userAgent string) (servers.PublicDetail, error) {
	return r.setAdminStatus(ctx, actorUserID, serverID, "hidden", reason, ipAddress, userAgent)
}

func (r ServerRepository) Restore(ctx context.Context, actorUserID string, serverID string, reason string, ipAddress string, userAgent string) (servers.PublicDetail, error) {
	return r.setAdminStatus(ctx, actorUserID, serverID, "published", reason, ipAddress, userAgent)
}

func (r ServerRepository) setAdminStatus(ctx context.Context, actorUserID string, serverID string, status string, reason string, ipAddress string, userAgent string) (servers.PublicDetail, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return servers.PublicDetail{}, fmt.Errorf("begin server admin status update: %w", err)
	}
	defer tx.Rollback(ctx)

	before, err := getServerForAuditTx(ctx, tx, serverID)
	if err != nil {
		return servers.PublicDetail{}, err
	}
	hiddenAtExpr := "hidden_at"
	if status == "hidden" {
		hiddenAtExpr = "now()"
	} else {
		hiddenAtExpr = "NULL"
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
UPDATE server_listings
SET status = $2, hidden_at = %s, updated_at = now()
WHERE id = $1::uuid AND deleted_at IS NULL`, hiddenAtExpr), serverID, status); err != nil {
		return servers.PublicDetail{}, fmt.Errorf("update server admin status: %w", err)
	}
	after, err := getServerForAuditTx(ctx, tx, serverID)
	if err != nil {
		return servers.PublicDetail{}, err
	}
	if err := insertServerAudit(ctx, tx, actorUserID, serverID, "server."+status, reason, before, after, ipAddress, userAgent); err != nil {
		return servers.PublicDetail{}, err
	}
	item, detail, err := r.scanServerPublic(tx.QueryRow(ctx, serverSelectSQL(`
WHERE sl.id = $1::uuid
GROUP BY sl.id, u.id, u.username, u.display_name, avatar.id
LIMIT 1`), serverID))
	if err != nil {
		return servers.PublicDetail{}, err
	}
	detail.PublicItem = item
	if err := tx.Commit(ctx); err != nil {
		return servers.PublicDetail{}, fmt.Errorf("commit server admin status update: %w", err)
	}
	return detail, nil
}

func (r ServerRepository) encryptOptional(password *string) (string, error) {
	if password == nil || strings.TrimSpace(*password) == "" {
		return "", nil
	}
	if r.secrets == nil {
		return "", fmt.Errorf("secret box unavailable")
	}
	encrypted, err := r.secrets.EncryptString(*password)
	if err != nil {
		return "", fmt.Errorf("encrypt server password: %w", err)
	}
	return encrypted, nil
}

type serverRows interface {
	Scan(dest ...any) error
}

func (r ServerRepository) scanServerPublic(row serverRows) (servers.PublicItem, servers.PublicDetail, error) {
	item, detail, encryptedPassword, err := scanServer(row)
	if err != nil {
		return servers.PublicItem{}, servers.PublicDetail{}, err
	}
	endpoint := servers.Endpoint{Host: item.Host, Port: item.Port}
	if encryptedPassword != nil && r.secrets != nil {
		password, err := r.secrets.DecryptString(*encryptedPassword)
		if err != nil {
			return servers.PublicItem{}, servers.PublicDetail{}, fmt.Errorf("decrypt server password: %w", err)
		}
		endpoint.Password = &password
		item.HasPassword = true
	}
	item.ConnectionString = endpoint.ConnectionString()
	detail.PublicItem = item
	return item, detail, nil
}

func (r ServerRepository) scanServerOwner(row serverRows) (servers.PublicItem, servers.PublicDetail, *string, error) {
	item, detail, encryptedPassword, err := scanServer(row)
	if err != nil {
		return servers.PublicItem{}, servers.PublicDetail{}, nil, err
	}
	endpoint := servers.Endpoint{Host: item.Host, Port: item.Port}
	if encryptedPassword != nil && r.secrets != nil {
		password, err := r.secrets.DecryptString(*encryptedPassword)
		if err != nil {
			return servers.PublicItem{}, servers.PublicDetail{}, nil, fmt.Errorf("decrypt server password: %w", err)
		}
		endpoint.Password = &password
		item.HasPassword = true
	}
	item.ConnectionString = endpoint.ConnectionString()
	detail.PublicItem = item
	return item, detail, encryptedPassword, nil
}

func scanServer(row serverRows) (servers.PublicItem, servers.PublicDetail, *string, error) {
	var item servers.PublicItem
	var detail servers.PublicDetail
	var tagsJSON string
	var port int
	var encryptedPassword *string
	var onlinePlayers *int
	var maxPlayers *int
	var protocolVersion *int
	var roundTripMS *int
	err := row.Scan(
		&item.ID,
		&item.Slug,
		&item.Name,
		&item.Description,
		&item.Visibility,
		&item.Status,
		&item.Host,
		&port,
		&encryptedPassword,
		&item.Region,
		&item.Language,
		&item.NSFW,
		&detail.Rules,
		&detail.DiscordURL,
		&detail.WebsiteURL,
		&item.Check.Status,
		&onlinePlayers,
		&maxPlayers,
		&protocolVersion,
		&item.Check.ServerName,
		&item.Check.Motd,
		&roundTripMS,
		&item.Check.LastError,
		&item.Check.LastCheckedAt,
		&item.Owner.ID,
		&item.Owner.Username,
		&item.Owner.DisplayName,
		&item.Owner.AvatarImageID,
		&tagsJSON,
		&item.PublishedAt,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err != nil {
		return servers.PublicItem{}, servers.PublicDetail{}, nil, err
	}
	item.Port = uint16(port)
	item.HasPassword = encryptedPassword != nil
	item.Check.OnlinePlayers = onlinePlayers
	item.Check.MaxPlayers = maxPlayers
	item.Check.ProtocolVersion = protocolVersion
	item.Check.RoundTripMS = roundTripMS
	if err := decodeJSON([]byte(tagsJSON), &item.Tags); err != nil {
		return servers.PublicItem{}, servers.PublicDetail{}, nil, fmt.Errorf("decode server tags: %w", err)
	}
	detail.PublicItem = item
	return item, detail, encryptedPassword, nil
}

func serverSelectSQL(suffix string) string {
	return `
SELECT
  sl.id::text,
  sl.slug,
  sl.name,
  sl.description,
  sl.visibility,
  sl.status,
  sl.host,
  sl.port,
  sl.password_ciphertext,
  sl.region,
  sl.language,
  sl.nsfw,
  sl.rules,
  sl.discord_url,
  sl.website_url,
  sl.check_status,
  sl.check_online_players,
  sl.check_max_players,
  sl.check_protocol_version,
  sl.check_server_name,
  sl.check_motd,
  sl.check_round_trip_ms,
  sl.check_error,
  sl.last_checked_at,
  u.id::text,
  u.username,
  u.display_name,
  avatar.id::text,
  COALESCE(
    jsonb_agg(DISTINCT jsonb_build_object('slug', t.slug, 'name', t.name))
      FILTER (WHERE t.id IS NOT NULL),
    '[]'::jsonb
  )::text AS tags_json,
  sl.published_at,
  sl.created_at,
  sl.updated_at
FROM server_listings sl
JOIN users u ON u.id = sl.owner_id
LEFT JOIN user_images avatar ON avatar.id = u.avatar_image_id AND avatar.processing_status = 'processed'
LEFT JOIN server_tags st ON st.server_id = sl.id
LEFT JOIN tags t ON t.id = st.tag_id
` + suffix
}

func getOwnedServerTx(ctx context.Context, tx pgx.Tx, ownerID string, serverID string, secrets *secretbox.Box) (servers.OwnerItem, error) {
	repo := ServerRepository{secrets: secrets}
	item, detail, encryptedPassword, err := repo.scanServerOwner(tx.QueryRow(ctx, serverSelectSQL(`
WHERE sl.id = $1::uuid AND sl.owner_id = $2::uuid AND sl.deleted_at IS NULL
GROUP BY sl.id, u.id, u.username, u.display_name, avatar.id
LIMIT 1`), serverID, ownerID))
	if err != nil {
		return servers.OwnerItem{}, err
	}
	ownerItem := servers.OwnerItem{PublicDetail: detail}
	ownerItem.PublicItem = item
	if encryptedPassword != nil && secrets != nil {
		password, err := secrets.DecryptString(*encryptedPassword)
		if err != nil {
			return servers.OwnerItem{}, fmt.Errorf("decrypt owned server password: %w", err)
		}
		ownerItem.Password = &password
	}
	return ownerItem, nil
}

func getOwnedServerForUpdateTx(ctx context.Context, tx pgx.Tx, ownerID string, serverID string, secrets *secretbox.Box) (servers.OwnerItem, *string, error) {
	var encryptedPassword *string
	if err := tx.QueryRow(ctx, `
SELECT password_ciphertext
FROM server_listings
WHERE id = $1::uuid AND owner_id = $2::uuid AND deleted_at IS NULL
FOR UPDATE`, serverID, ownerID).Scan(&encryptedPassword); err != nil {
		return servers.OwnerItem{}, nil, err
	}
	ownerItem, err := getOwnedServerTx(ctx, tx, ownerID, serverID, secrets)
	if err != nil {
		return servers.OwnerItem{}, nil, err
	}
	return ownerItem, encryptedPassword, nil
}

func replaceServerTagsTx(ctx context.Context, tx pgx.Tx, serverID string, tags []servers.TagInput) error {
	if _, err := tx.Exec(ctx, `DELETE FROM server_tags WHERE server_id = $1::uuid`, serverID); err != nil {
		return fmt.Errorf("delete existing server tags: %w", err)
	}
	for _, tag := range tags {
		var tagID string
		err := tx.QueryRow(ctx, `
INSERT INTO tags (slug, name)
VALUES ($1, $2)
ON CONFLICT (slug) DO UPDATE SET updated_at = tags.updated_at
RETURNING id::text`, tag.Slug, tag.Name).Scan(&tagID)
		if err != nil {
			return fmt.Errorf("upsert server tag %q: %w", tag.Slug, err)
		}
		if _, err := tx.Exec(ctx, `
INSERT INTO server_tags (server_id, tag_id)
VALUES ($1::uuid, $2::uuid)
ON CONFLICT DO NOTHING`, serverID, tagID); err != nil {
			return fmt.Errorf("link server tag %q: %w", tag.Slug, err)
		}
	}
	return nil
}

func enqueueServerCheckTx(ctx context.Context, tx pgx.Tx, serverID string) error {
	payload, err := json.Marshal(servers.CheckPayload{ServerID: serverID})
	if err != nil {
		return fmt.Errorf("marshal server check payload: %w", err)
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO worker_jobs (queue_name, job_type, payload, max_attempts)
SELECT 'server_check_queue', 'check_basis_server', $1::jsonb, 3
WHERE NOT EXISTS (
  SELECT 1
  FROM worker_jobs job
  WHERE job.queue_name = 'server_check_queue'
    AND job.status IN ('pending', 'running')
    AND job.payload->>'server_id' = $2
)`, string(payload), serverID); err != nil {
		return fmt.Errorf("insert server check job: %w", err)
	}
	return nil
}

func durationMilliseconds(value time.Duration) int64 {
	if value <= 0 {
		return 0
	}
	milliseconds := int64(value / time.Millisecond)
	if milliseconds < 1 {
		return 1
	}
	return milliseconds
}

func serverSort(sort string) (expr string, cursorCondition string, err error) {
	switch sort {
	case "", "newest":
		return "sl.published_at", "(sl.published_at, sl.id) < ($%d::timestamptz, $%d::uuid)", nil
	case "players":
		return "COALESCE(sl.check_online_players, -1)", "(COALESCE(sl.check_online_players, -1), sl.id) < ($%d::integer, $%d::uuid)", nil
	case "updated":
		return "sl.updated_at", "(sl.updated_at, sl.id) < ($%d::timestamptz, $%d::uuid)", nil
	default:
		return "", "", fmt.Errorf("unsupported server sort %q", sort)
	}
}

func nextServerCursor(item servers.PublicItem, sort string) *servers.Cursor {
	cursor := &servers.Cursor{ID: item.ID}
	switch sort {
	case "players":
		if item.Check.OnlinePlayers != nil {
			cursor.SortValue = strconv.Itoa(*item.Check.OnlinePlayers)
		} else {
			cursor.SortValue = "-1"
		}
	case "updated":
		cursor.SortValue = item.UpdatedAt.Format(timeFormatRFC3339Micro)
	default:
		if item.PublishedAt != nil {
			cursor.SortValue = item.PublishedAt.Format(timeFormatRFC3339Micro)
		}
	}
	return cursor
}

func optionalString(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func sameOptionalString(a *string, b *string) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func getServerForAuditTx(ctx context.Context, tx pgx.Tx, serverID string) (map[string]any, error) {
	var raw map[string]any
	err := tx.QueryRow(ctx, `
SELECT to_jsonb(server_listings)
FROM server_listings
WHERE id = $1::uuid
LIMIT 1`, serverID).Scan(&raw)
	return raw, err
}

func insertServerAudit(ctx context.Context, tx pgx.Tx, actorUserID string, serverID string, action string, reason string, before any, after any, ipAddress string, userAgent string) error {
	beforeJSON, err := json.Marshal(before)
	if err != nil {
		return err
	}
	afterJSON, err := json.Marshal(map[string]any{
		"reason": reason,
		"server": after,
	})
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO audit_logs (actor_user_id, action, entity_type, entity_id, before_json, after_json, ip_address, user_agent)
VALUES ($1::uuid, $2, 'server_listing', $3::uuid, COALESCE($4::jsonb, '{}'::jsonb), $5::jsonb, NULLIF($6, '')::inet, NULLIF($7, ''))`,
		actorUserID, action, serverID, string(beforeJSON), string(afterJSON), ipAddress, userAgent,
	); err != nil {
		return fmt.Errorf("insert server audit: %w", err)
	}
	return nil
}

func isNoRows(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}
