package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/jmoiron/sqlx"

	"github.com/tergeoo/asc-mcp/internal/shared/store"
)

// Store is the Postgres-backed store.Store implementation.
type Store struct {
	db *sqlx.DB
	sb sq.StatementBuilderType
}

// NewStore wraps a sqlx.DB.
func NewStore(db *sqlx.DB) *Store {
	return &Store{
		db: db,
		sb: sq.StatementBuilder.PlaceholderFormat(sq.Dollar),
	}
}

func (s *Store) Enabled() bool { return true }

// LookupOperation returns the most recent successful op for a given input hash.
func (s *Store) LookupOperation(ctx context.Context, inputHash string) (store.Operation, bool, error) {
	q, args, err := s.sb.
		Select("id", "app_id", "version_id", "tool", "input_hash", "status", "error", "result", "created_at").
		From("operation_log").
		Where(sq.Eq{"input_hash": inputHash, "status": store.StatusSuccess}).
		OrderBy("id DESC").
		Limit(1).
		ToSql()
	if err != nil {
		return store.Operation{}, false, fmt.Errorf("db: build lookup: %w", err)
	}
	var op store.Operation
	if err := s.db.GetContext(ctx, &op, q, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return store.Operation{}, false, nil
		}
		return store.Operation{}, false, fmt.Errorf("db: lookup operation: %w", err)
	}
	return op, true, nil
}

// SaveOperation appends an audit-log row.
func (s *Store) SaveOperation(ctx context.Context, op store.Operation) error {
	q, args, err := s.sb.
		Insert("operation_log").
		Columns("app_id", "version_id", "tool", "input_hash", "status", "error", "result").
		Values(op.AppID, op.VersionID, op.Tool, op.InputHash, op.Status, op.Error, op.Result).
		ToSql()
	if err != nil {
		return fmt.Errorf("db: build insert: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, q, args...); err != nil {
		return fmt.Errorf("db: save operation: %w", err)
	}
	return nil
}

// PutCache upserts a metadata snapshot.
func (s *Store) PutCache(ctx context.Context, e store.CacheEntry) error {
	const q = `
		INSERT INTO metadata_cache (app_id, version_id, locale, payload, fetched_at)
		VALUES ($1, $2, $3, $4::jsonb, now())
		ON CONFLICT (app_id, version_id, locale)
		DO UPDATE SET payload = EXCLUDED.payload, fetched_at = now()`
	if _, err := s.db.ExecContext(ctx, q, e.AppID, e.VersionID, e.Locale, e.Payload); err != nil {
		return fmt.Errorf("db: put cache: %w", err)
	}
	return nil
}

// GetCache returns a cached snapshot if present.
func (s *Store) GetCache(ctx context.Context, appID, versionID, locale string) (store.CacheEntry, bool, error) {
	q, args, err := s.sb.
		Select("app_id", "version_id", "locale", "payload", "fetched_at").
		From("metadata_cache").
		Where(sq.Eq{"app_id": appID, "version_id": versionID, "locale": locale}).
		ToSql()
	if err != nil {
		return store.CacheEntry{}, false, fmt.Errorf("db: build get cache: %w", err)
	}
	var e store.CacheEntry
	if err := s.db.GetContext(ctx, &e, q, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return store.CacheEntry{}, false, nil
		}
		return store.CacheEntry{}, false, fmt.Errorf("db: get cache: %w", err)
	}
	return e, true, nil
}

var _ store.Store = (*Store)(nil)
