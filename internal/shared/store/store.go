// Package store defines the persistence contract used by feature services for
// idempotency, audit logging, metadata caching and resumable screenshot
// uploads. The contract is small and cross-cutting; concrete implementations
// live in internal/shared/db (Postgres) and Noop (DB disabled).
package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"time"
)

// Operation is one row of the audit / idempotency log.
type Operation struct {
	ID        int64     `db:"id"`
	AppID     string    `db:"app_id"`
	VersionID string    `db:"version_id"`
	Tool      string    `db:"tool"`
	InputHash string    `db:"input_hash"`
	Status    string    `db:"status"`
	Error     string    `db:"error"`
	Result    string    `db:"result"`
	CreatedAt time.Time `db:"created_at"`
}

// Operation statuses.
const (
	StatusSuccess = "success"
	StatusError   = "error"
	StatusDryRun  = "dry_run"
)

// CacheEntry is a cached snapshot of metadata for dry-run/diff.
type CacheEntry struct {
	AppID     string    `db:"app_id"`
	VersionID string    `db:"version_id"`
	Locale    string    `db:"locale"`
	Payload   string    `db:"payload"`
	FetchedAt time.Time `db:"fetched_at"`
}

// Store is the persistence interface consumed by services.
type Store interface {
	// LookupOperation returns the most recent successful operation matching
	// inputHash, if any (used for idempotency).
	LookupOperation(ctx context.Context, inputHash string) (Operation, bool, error)
	// SaveOperation appends an audit-log row.
	SaveOperation(ctx context.Context, op Operation) error
	// PutCache upserts a metadata cache snapshot.
	PutCache(ctx context.Context, e CacheEntry) error
	// GetCache returns a cached snapshot if present.
	GetCache(ctx context.Context, appID, versionID, locale string) (CacheEntry, bool, error)
	// Enabled reports whether persistence is active.
	Enabled() bool
}

// HashInput produces a stable hash of (tool + normalized input) for
// idempotency. Map keys are sorted so logically-identical inputs hash equally.
func HashInput(tool string, input any) string {
	h := sha256.New()
	h.Write([]byte(tool))
	h.Write([]byte{0})
	h.Write(canonical(input))
	return hex.EncodeToString(h.Sum(nil))
}

// canonical renders a value to deterministic JSON (sorted object keys).
func canonical(v any) []byte {
	raw, err := json.Marshal(v)
	if err != nil {
		return []byte("null")
	}
	var generic any
	if err := json.Unmarshal(raw, &generic); err != nil {
		return raw
	}
	out, _ := json.Marshal(sortValue(generic))
	return out
}

func sortValue(v any) any {
	switch t := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		ordered := make([][2]any, 0, len(keys))
		for _, k := range keys {
			ordered = append(ordered, [2]any{k, sortValue(t[k])})
		}
		return ordered
	case []any:
		out := make([]any, len(t))
		for i := range t {
			out[i] = sortValue(t[i])
		}
		return out
	default:
		return v
	}
}

// Noop is a Store that persists nothing — used when the DB is disabled.
type Noop struct{}

func (Noop) LookupOperation(context.Context, string) (Operation, bool, error) {
	return Operation{}, false, nil
}
func (Noop) SaveOperation(context.Context, Operation) error { return nil }
func (Noop) PutCache(context.Context, CacheEntry) error     { return nil }
func (Noop) GetCache(context.Context, string, string, string) (CacheEntry, bool, error) {
	return CacheEntry{}, false, nil
}
func (Noop) Enabled() bool { return false }

// compile-time check
var _ Store = Noop{}
