package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"beeba.org/internal/domain/tags"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TagRepository struct {
	db *pgxpool.Pool
}

var ErrTagConflict = errors.New("tag slug already exists")

const publicTagHavingClause = "HAVING count(ci.id) > 0"

func (r TagRepository) ListAdminTags(ctx context.Context) ([]tags.Tag, error) {
	rows, err := r.db.Query(ctx, `
SELECT
  t.id::text,
  t.slug,
  t.name,
  t.is_system,
  count(ci.id)::int AS published_count,
  t.created_at,
  t.updated_at
FROM tags t
LEFT JOIN content_tags ct ON ct.tag_id = t.id
LEFT JOIN content_items ci ON ci.id = ct.content_id
  AND ci.status = 'published'
  AND ci.visibility = 'public'
  AND ci.deleted_at IS NULL
  AND ci.hidden_at IS NULL
  AND ci.published_at IS NOT NULL
GROUP BY t.id
ORDER BY t.is_system DESC, published_count DESC, t.name ASC
LIMIT 500`)
	if err != nil {
		return nil, fmt.Errorf("query admin tags: %w", err)
	}
	defer rows.Close()
	return scanTags(rows)
}

func (r TagRepository) CreateAdminTag(ctx context.Context, actorUserID string, input tags.AdminCreateInput, ipAddress string, userAgent string) (tags.Tag, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return tags.Tag{}, fmt.Errorf("begin tag create: %w", err)
	}
	defer tx.Rollback(ctx)

	var tag tags.Tag
	err = tx.QueryRow(ctx, `
INSERT INTO tags (slug, name, is_system)
VALUES ($1, $2, $3)
RETURNING id::text, slug, name, is_system, 0::int, created_at, updated_at`,
		input.Slug, input.Name, input.IsSystem,
	).Scan(&tag.ID, &tag.Slug, &tag.Name, &tag.IsSystem, &tag.PublishedCount, &tag.CreatedAt, &tag.UpdatedAt)
	if err != nil {
		if isTagUniqueViolation(err) {
			return tags.Tag{}, ErrTagConflict
		}
		return tags.Tag{}, fmt.Errorf("insert tag: %w", err)
	}
	if err := insertTagAudit(ctx, tx, actorUserID, tag.ID, "tag.create", nil, tag, ipAddress, userAgent); err != nil {
		return tags.Tag{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return tags.Tag{}, fmt.Errorf("commit tag create: %w", err)
	}
	return tag, nil
}

func (r TagRepository) UpdateAdminTag(ctx context.Context, actorUserID string, tagID string, input tags.AdminUpdateInput, ipAddress string, userAgent string) (tags.Tag, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return tags.Tag{}, fmt.Errorf("begin tag update: %w", err)
	}
	defer tx.Rollback(ctx)

	before, err := loadTagForUpdate(ctx, tx, tagID)
	if err != nil {
		return tags.Tag{}, err
	}
	name := before.Name
	if input.Name != nil {
		name = *input.Name
	}
	isSystem := before.IsSystem
	if input.IsSystem != nil {
		isSystem = *input.IsSystem
	}

	var after tags.Tag
	err = tx.QueryRow(ctx, `
UPDATE tags
SET name = $2, is_system = $3, updated_at = now()
WHERE id = $1::uuid
RETURNING id::text, slug, name, is_system, (
  SELECT count(ci.id)::int
  FROM content_tags ct
  JOIN content_items ci ON ci.id = ct.content_id
  WHERE ct.tag_id = tags.id
    AND ci.status = 'published'
    AND ci.visibility = 'public'
    AND ci.deleted_at IS NULL
    AND ci.hidden_at IS NULL
    AND ci.published_at IS NOT NULL
), created_at, updated_at`,
		tagID, name, isSystem,
	).Scan(&after.ID, &after.Slug, &after.Name, &after.IsSystem, &after.PublishedCount, &after.CreatedAt, &after.UpdatedAt)
	if err != nil {
		return tags.Tag{}, fmt.Errorf("update tag: %w", err)
	}
	if err := insertTagAudit(ctx, tx, actorUserID, tagID, "tag.update", before, after, ipAddress, userAgent); err != nil {
		return tags.Tag{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return tags.Tag{}, fmt.Errorf("commit tag update: %w", err)
	}
	return after, nil
}

func NewTagRepository(db *pgxpool.Pool) TagRepository {
	return TagRepository{db: db}
}

func (r TagRepository) ListPublic(ctx context.Context) ([]tags.Tag, error) {
	rows, err := r.db.Query(ctx, `
SELECT
  t.id::text,
  t.slug,
  t.name,
  t.is_system,
  count(ci.id)::int AS published_count,
  t.created_at,
  t.updated_at
FROM tags t
LEFT JOIN content_tags ct ON ct.tag_id = t.id
LEFT JOIN content_items ci ON ci.id = ct.content_id
  AND ci.status = 'published'
  AND ci.visibility = 'public'
  AND ci.deleted_at IS NULL
  AND ci.hidden_at IS NULL
  AND ci.published_at IS NOT NULL
GROUP BY t.id
`+publicTagHavingClause+`
ORDER BY published_count DESC, t.name ASC
LIMIT 100`)
	if err != nil {
		return nil, fmt.Errorf("query tags: %w", err)
	}
	defer rows.Close()

	return scanTags(rows)
}

type tagRows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}

func scanTags(rows tagRows) ([]tags.Tag, error) {
	items := make([]tags.Tag, 0, 32)
	for rows.Next() {
		var item tags.Tag
		if err := rows.Scan(&item.ID, &item.Slug, &item.Name, &item.IsSystem, &item.PublishedCount, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan tag: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read tags: %w", err)
	}
	return items, nil
}

func loadTagForUpdate(ctx context.Context, tx pgx.Tx, tagID string) (tags.Tag, error) {
	var tag tags.Tag
	err := tx.QueryRow(ctx, `
SELECT id::text, slug, name, is_system, 0::int, created_at, updated_at
FROM tags
WHERE id = $1::uuid
FOR UPDATE`, tagID).Scan(&tag.ID, &tag.Slug, &tag.Name, &tag.IsSystem, &tag.PublishedCount, &tag.CreatedAt, &tag.UpdatedAt)
	return tag, err
}

func insertTagAudit(ctx context.Context, tx pgx.Tx, actorUserID string, tagID string, action string, before any, after any, ipAddress string, userAgent string) error {
	beforeJSON, err := json.Marshal(before)
	if err != nil {
		return err
	}
	afterJSON, err := json.Marshal(after)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO audit_logs (actor_user_id, action, entity_type, entity_id, before_json, after_json, ip_address, user_agent)
VALUES ($1::uuid, $2, 'tag', $3::uuid, COALESCE($4::jsonb, '{}'::jsonb), $5::jsonb, NULLIF($6, '')::inet, NULLIF($7, ''))`,
		actorUserID, action, tagID, string(beforeJSON), string(afterJSON), ipAddress, userAgent,
	); err != nil {
		return fmt.Errorf("insert tag audit: %w", err)
	}
	return nil
}

func isTagUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return err != nil && fmt.Sprint(err) != "" && errors.As(err, &pgErr) && pgErr.Code == "23505"
}
