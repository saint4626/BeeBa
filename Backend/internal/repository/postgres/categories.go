package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"beeba.org/internal/domain/categories"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CategoryRepository struct {
	db *pgxpool.Pool
}

func (r CategoryRepository) ListAdminCategories(ctx context.Context) ([]categories.Category, error) {
	rows, err := r.db.Query(ctx, `
SELECT id::text, slug, name, description, icon_key, sort_order, is_active, published_count, created_at, updated_at
FROM categories
ORDER BY sort_order ASC, name ASC`)
	if err != nil {
		return nil, fmt.Errorf("query admin categories: %w", err)
	}
	defer rows.Close()

	items := make([]categories.Category, 0, 4)
	for rows.Next() {
		item, err := scanCategory(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read admin categories: %w", err)
	}
	return items, nil
}

func (r CategoryRepository) UpdateAdminCategory(ctx context.Context, actorUserID string, categoryID string, input categories.AdminUpdateInput, ipAddress string, userAgent string) (categories.Category, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return categories.Category{}, fmt.Errorf("begin category update: %w", err)
	}
	defer tx.Rollback(ctx)

	before, err := loadCategoryForUpdate(ctx, tx, categoryID)
	if err != nil {
		return categories.Category{}, err
	}

	name := before.Name
	if input.Name != nil {
		name = *input.Name
	}
	description := before.Description
	if input.Description != nil {
		description = *input.Description
	}
	iconKey := before.IconKey
	if input.IconKey != nil {
		value := *input.IconKey
		if value == "" {
			iconKey = nil
		} else {
			iconKey = &value
		}
	}
	sortOrder := before.SortOrder
	if input.SortOrder != nil {
		sortOrder = *input.SortOrder
	}
	isActive := before.IsActive
	if input.IsActive != nil {
		isActive = *input.IsActive
	}

	var after categories.Category
	err = tx.QueryRow(ctx, `
UPDATE categories
SET name = $2, description = $3, icon_key = $4, sort_order = $5, is_active = $6, updated_at = now()
WHERE id = $1::uuid
RETURNING id::text, slug, name, description, icon_key, sort_order, is_active, published_count, created_at, updated_at`,
		categoryID, name, description, iconKey, sortOrder, isActive,
	).Scan(&after.ID, &after.Slug, &after.Name, &after.Description, &after.IconKey, &after.SortOrder, &after.IsActive, &after.PublishedCount, &after.CreatedAt, &after.UpdatedAt)
	if err != nil {
		return categories.Category{}, fmt.Errorf("update category: %w", err)
	}

	beforeJSON, err := json.Marshal(before)
	if err != nil {
		return categories.Category{}, err
	}
	afterJSON, err := json.Marshal(after)
	if err != nil {
		return categories.Category{}, err
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO audit_logs (actor_user_id, action, entity_type, entity_id, before_json, after_json, ip_address, user_agent)
VALUES ($1::uuid, 'category.update', 'category', $2::uuid, $3::jsonb, $4::jsonb, NULLIF($5, '')::inet, NULLIF($6, ''))`,
		actorUserID, categoryID, string(beforeJSON), string(afterJSON), ipAddress, userAgent,
	); err != nil {
		return categories.Category{}, fmt.Errorf("insert category audit: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return categories.Category{}, fmt.Errorf("commit category update: %w", err)
	}
	return after, nil
}

func NewCategoryRepository(db *pgxpool.Pool) CategoryRepository {
	return CategoryRepository{db: db}
}

func (r CategoryRepository) ListActive(ctx context.Context) ([]categories.Category, error) {
	rows, err := r.db.Query(ctx, `
SELECT id::text, slug, name, description, icon_key, sort_order, is_active, published_count, created_at, updated_at
FROM categories
WHERE is_active = true
ORDER BY sort_order ASC, name ASC`)
	if err != nil {
		return nil, fmt.Errorf("query categories: %w", err)
	}
	defer rows.Close()

	items := make([]categories.Category, 0, 4)
	for rows.Next() {
		item, err := scanCategory(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read categories: %w", err)
	}

	return items, nil
}

func loadCategoryForUpdate(ctx context.Context, tx pgx.Tx, categoryID string) (categories.Category, error) {
	return scanCategory(tx.QueryRow(ctx, `
SELECT id::text, slug, name, description, icon_key, sort_order, is_active, published_count, created_at, updated_at
FROM categories
WHERE id = $1::uuid
FOR UPDATE`, categoryID))
}

type categoryScanner interface {
	Scan(dest ...any) error
}

func scanCategory(row categoryScanner) (categories.Category, error) {
	var item categories.Category
	if err := row.Scan(
		&item.ID,
		&item.Slug,
		&item.Name,
		&item.Description,
		&item.IconKey,
		&item.SortOrder,
		&item.IsActive,
		&item.PublishedCount,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return categories.Category{}, fmt.Errorf("scan category: %w", err)
	}
	return item, nil
}
