package index

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
)

// AIToolAudit is one recorded agent tool invocation. Args holds a sanitized,
// truncated rendering of the call arguments: note bodies and binary payloads
// are replaced by a content hash and secrets are never stored.
type AIToolAudit struct {
	ID            string
	RunID         string
	UserID        string
	Username      string
	Role          string
	Step          int
	Tool          string
	Args          string
	ResultSummary string
	Status        string
	Destructive   bool
	NoteVersion   string
	TrashID       string
	RevisionID    string
	Created       time.Time
}

// RecordAIToolAudit appends one audit row, sanitizing the arguments first.
func (ix *Index) RecordAIToolAudit(ctx context.Context, rec AIToolAudit) error {
	if rec.ID == "" {
		rec.ID = ulid.Make().String()
	}
	if rec.Created.IsZero() {
		rec.Created = time.Now().UTC()
	}
	_, err := ix.db.ExecContext(ctx, `
		INSERT INTO ai_tool_audit
			(id, run_id, user_id, username, role, step, tool, args, result_summary,
			 status, destructive, note_version, trash_id, revision_id, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rec.ID, rec.RunID, rec.UserID, rec.Username, rec.Role, rec.Step, rec.Tool,
		sanitizeAuditArgs(rec.Args), rec.ResultSummary, rec.Status, boolToInt(rec.Destructive),
		rec.NoteVersion, rec.TrashID, rec.RevisionID,
		rec.Created.UTC().Format(time.RFC3339))
	return err
}

// ListAIToolAudit returns the audit trail for a run, oldest first.
func (ix *Index) ListAIToolAudit(ctx context.Context, runID string) ([]AIToolAudit, error) {
	rows, err := ix.db.QueryContext(ctx, `
		SELECT id, run_id, user_id, username, role, step, tool, args, result_summary,
		       status, destructive, note_version, trash_id, revision_id, created_at
		FROM ai_tool_audit WHERE run_id = ? ORDER BY rowid`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]AIToolAudit, 0, 8)
	for rows.Next() {
		var (
			rec       AIToolAudit
			dest      int
			createdAt string
		)
		if err := rows.Scan(&rec.ID, &rec.RunID, &rec.UserID, &rec.Username, &rec.Role,
			&rec.Step, &rec.Tool, &rec.Args, &rec.ResultSummary, &rec.Status, &dest,
			&rec.NoteVersion, &rec.TrashID, &rec.RevisionID, &createdAt); err != nil {
			return nil, err
		}
		rec.Destructive = dest == 1
		rec.Created = parseTime(createdAt)
		out = append(out, rec)
	}
	return out, rows.Err()
}

const (
	auditArgsMax = 2048
	auditHashMax = 512
)

// sanitizeAuditArgs redacts secrets, hashes potentially large note bodies and
// uploads, and truncates the result so the audit trail never stores content.
func sanitizeAuditArgs(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "null" {
		return ""
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(raw), &obj); err != nil {
		return truncateString(raw, auditArgsMax)
	}
	for k, v := range obj {
		lk := strings.ToLower(k)
		switch {
		case strings.Contains(lk, "key"),
			strings.Contains(lk, "token"),
			strings.Contains(lk, "password"),
			strings.Contains(lk, "secret"),
			strings.Contains(lk, "authorization"):
			obj[k] = "[redacted]"
		case lk == "body", lk == "content":
			obj[k] = contentMarker(v)
		}
	}
	b, err := json.Marshal(obj)
	if err != nil {
		return truncateString(raw, auditArgsMax)
	}
	return truncateString(string(b), auditArgsMax)
}

func contentMarker(v any) string {
	s, ok := v.(string)
	if !ok {
		return "[redacted]"
	}
	sum := sha256.Sum256([]byte(s))
	return "sha256:" + hex.EncodeToString(sum[:]) + " (len " + strconv.Itoa(len(s)) + ")"
}

func truncateString(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
