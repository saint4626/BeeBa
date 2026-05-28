package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"beeba.org/internal/domain/audit"

	"github.com/jackc/pgx/v5/pgxpool"
)

type AuditRepository struct {
	db *pgxpool.Pool
}

func NewAuditRepository(db *pgxpool.Pool) AuditRepository {
	return AuditRepository{db: db}
}

func (r AuditRepository) ListAuditLogs(ctx context.Context, filter audit.ListFilter) (audit.Page, error) {
	args := []any{}
	conditions := []string{"1 = 1"}

	if filter.Action != "" {
		args = append(args, filter.Action)
		conditions = append(conditions, fmt.Sprintf("al.action = $%d", len(args)))
	}
	if filter.EntityType != "" {
		args = append(args, filter.EntityType)
		conditions = append(conditions, fmt.Sprintf("al.entity_type = $%d", len(args)))
	}
	if filter.ActorUserID != "" {
		args = append(args, filter.ActorUserID)
		conditions = append(conditions, fmt.Sprintf("al.actor_user_id = $%d::uuid", len(args)))
	}
	if filter.EntityID != "" {
		args = append(args, filter.EntityID)
		conditions = append(conditions, fmt.Sprintf("al.entity_id = $%d::uuid", len(args)))
	}
	if !filter.Cursor.CreatedAt.IsZero() && filter.Cursor.ID != "" {
		args = append(args, filter.Cursor.CreatedAt, filter.Cursor.ID)
		conditions = append(conditions, fmt.Sprintf("(al.created_at, al.id) < ($%d::timestamptz, $%d::uuid)", len(args)-1, len(args)))
	}

	args = append(args, filter.Limit+1)
	limitArg := len(args)

	query := fmt.Sprintf(`
SELECT
  al.id::text,
  al.actor_user_id::text,
  u.username,
  u.email,
  al.action,
  al.entity_type,
  al.entity_id::text,
  al.before_json::text,
  al.after_json::text,
  host(al.ip_address),
  al.user_agent,
  al.created_at
FROM audit_logs al
LEFT JOIN users u ON u.id = al.actor_user_id
WHERE %s
ORDER BY al.created_at DESC, al.id DESC
LIMIT $%d`, joinConditions(conditions), limitArg)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return audit.Page{}, fmt.Errorf("query audit logs: %w", err)
	}
	defer rows.Close()

	items := make([]audit.LogEntry, 0, filter.Limit)
	for rows.Next() {
		var item audit.LogEntry
		var beforeJSON string
		var afterJSON string
		if err := rows.Scan(
			&item.ID,
			&item.ActorUserID,
			&item.ActorUsername,
			&item.ActorEmail,
			&item.Action,
			&item.EntityType,
			&item.EntityID,
			&beforeJSON,
			&afterJSON,
			&item.IPAddress,
			&item.UserAgent,
			&item.CreatedAt,
		); err != nil {
			return audit.Page{}, fmt.Errorf("scan audit log: %w", err)
		}
		item.Before = normalizeRawJSON(beforeJSON)
		item.After = normalizeRawJSON(afterJSON)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return audit.Page{}, fmt.Errorf("iterate audit logs: %w", err)
	}

	var next audit.Cursor
	if len(items) > filter.Limit {
		items = items[:filter.Limit]
		last := items[len(items)-1]
		next = audit.Cursor{CreatedAt: last.CreatedAt, ID: last.ID}
	}

	return audit.Page{Items: items, NextCursor: next}, nil
}

func normalizeRawJSON(value string) json.RawMessage {
	if value == "" {
		return json.RawMessage(`{}`)
	}
	return json.RawMessage(value)
}
