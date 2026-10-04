CREATE TABLE IF NOT EXISTS ai_tool_audit (
  id TEXT PRIMARY KEY, run_id TEXT NOT NULL, user_id TEXT NOT NULL DEFAULT '',
  username TEXT NOT NULL DEFAULT '', role TEXT NOT NULL DEFAULT '', step INTEGER NOT NULL DEFAULT 0,
  tool TEXT NOT NULL, args TEXT NOT NULL DEFAULT '', result_summary TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL, destructive INTEGER NOT NULL DEFAULT 0,
  note_version TEXT NOT NULL DEFAULT '', trash_id TEXT NOT NULL DEFAULT '', revision_id TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_ai_audit_run ON ai_tool_audit(run_id);
CREATE INDEX IF NOT EXISTS idx_ai_audit_user ON ai_tool_audit(user_id, created_at);
