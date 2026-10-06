package index

import (
	"context"
	"time"

	"github.com/Overview-Note/overview/internal/core"
)

// DefaultChangeLimit bounds an incremental change query when the caller does
// not provide a limit.
const DefaultChangeLimit = 500

var _ core.ChangeLog = (*Index)(nil)

// LatestSeq returns the highest change sequence across notes and folders, or 0
// when empty. Folder changes share the note sequence space so a folder added
// without any note still advances the cursor a sync client compares against.
func (ix *Index) LatestSeq(ctx context.Context) (int64, error) {
	var seq int64
	err := ix.db.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(seq), 0) FROM (
			SELECT COALESCE(MAX(changed_seq), 0) AS seq FROM notes
			UNION ALL
			SELECT COALESCE(MAX(changed_seq), 0) AS seq FROM folder_changes
		)`).Scan(&seq)
	return seq, err
}

// ChangesSince returns the notes whose changed_seq is greater than seq, oldest
// first. It returns at most limit entries and reports whether more remain. A
// limit <= 0 falls back to DefaultChangeLimit.
func (ix *Index) ChangesSince(ctx context.Context, seq int64, limit int) ([]core.SyncChange, bool, error) {
	if limit <= 0 {
		limit = DefaultChangeLimit
	}
	rows, err := ix.db.QueryContext(ctx,
		`SELECT id, path, version, changed_seq FROM notes
		 WHERE changed_seq > ? ORDER BY changed_seq, id LIMIT ?`,
		seq, limit+1)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()

	changes := make([]core.SyncChange, 0, limit)
	for rows.Next() {
		var c core.SyncChange
		if err := rows.Scan(&c.ID, &c.Path, &c.Version, &c.Seq); err != nil {
			return nil, false, err
		}
		c.Op = "upsert"
		changes = append(changes, c)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}

	hasMore := len(changes) > limit
	if hasMore {
		changes = changes[:limit]
	}
	return changes, hasMore, nil
}

// TombstonesSince returns note deletion markers newer than since, oldest first.
func (ix *Index) TombstonesSince(ctx context.Context, since time.Time) ([]core.Tombstone, error) {
	rows, err := ix.db.QueryContext(ctx,
		`SELECT id, path, deleted_at, device FROM note_tombstones
		 WHERE deleted_at > ? ORDER BY deleted_at, id`,
		stamp(since))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]core.Tombstone, 0, 16)
	for rows.Next() {
		var (
			t  core.Tombstone
			at string
		)
		if err := rows.Scan(&t.ID, &t.Path, &at, &t.Device); err != nil {
			return nil, err
		}
		t.DeletedAt = parseTime(at)
		out = append(out, t)
	}
	return out, rows.Err()
}

// RecordFolderTombstone upserts a folder deletion marker. A folder deleted
// again (or moved) overwrites its earlier marker.
func (ix *Index) RecordFolderTombstone(ctx context.Context, t core.FolderTombstone) error {
	_, err := ix.db.ExecContext(ctx, `
		INSERT INTO folder_tombstones (path, deleted_at, device) VALUES (?, ?, ?)
		ON CONFLICT(path) DO UPDATE SET
			deleted_at = excluded.deleted_at, device = excluded.device`,
		t.Path, stamp(t.DeletedAt), t.Device)
	return err
}

// FolderTombstonesSince returns folder deletion markers newer than since,
// oldest first.
func (ix *Index) FolderTombstonesSince(ctx context.Context, since time.Time) ([]core.FolderTombstone, error) {
	rows, err := ix.db.QueryContext(ctx,
		`SELECT path, deleted_at, device FROM folder_tombstones
		 WHERE deleted_at > ? ORDER BY deleted_at, path`,
		stamp(since))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]core.FolderTombstone, 0, 16)
	for rows.Next() {
		var (
			t  core.FolderTombstone
			at string
		)
		if err := rows.Scan(&t.Path, &at, &t.Device); err != nil {
			return nil, err
		}
		t.DeletedAt = parseTime(at)
		out = append(out, t)
	}
	return out, rows.Err()
}

// LatestTombstoneTime returns the newest deletion marker timestamp across note
// and folder tombstones, or the zero time when there are none.
func (ix *Index) LatestTombstoneTime(ctx context.Context) (time.Time, error) {
	var at string
	err := ix.db.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(deleted_at), '') FROM (
			SELECT deleted_at FROM note_tombstones
			UNION ALL
			SELECT deleted_at FROM folder_tombstones
		)`).Scan(&at)
	if err != nil {
		return time.Time{}, err
	}
	return parseTime(at), nil
}

// RecordFolderChange upserts a folder creation or relocation marker. It
// allocates a change sequence from the shared counter so an empty folder is
// observable through the same cursor as a note change. A folder changed again
// overwrites its earlier marker.
func (ix *Index) RecordFolderChange(ctx context.Context, path string) error {
	tx, err := ix.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	seq, err := nextChangeSeq(ctx, tx)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO folder_changes (path, changed_seq) VALUES (?, ?)
		ON CONFLICT(path) DO UPDATE SET changed_seq = excluded.changed_seq`,
		path, seq); err != nil {
		return err
	}
	return tx.Commit()
}

// stamp formats a time for lexicographic comparison against stored RFC3339
// timestamps. UTC values with the same layout sort chronologically.
func stamp(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}
