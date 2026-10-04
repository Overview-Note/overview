package agent_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/Overview-Note/overview/internal/agent"
	"github.com/Overview-Note/overview/internal/ai"
	"github.com/Overview-Note/overview/internal/history"
	"github.com/Overview-Note/overview/internal/index"
	"github.com/Overview-Note/overview/internal/service"
	"github.com/Overview-Note/overview/internal/store"
	"github.com/Overview-Note/overview/internal/trash"
)

// scriptedLLM replays a fixed list of responses and records the offered tools.
type scriptedLLM struct {
	mu       sync.Mutex
	results  []ai.ChatResult
	i        int
	offered  [][]string
	messages [][]ai.AgentMessage
}

func (f *scriptedLLM) ChatTools(_ context.Context, msgs []ai.AgentMessage, offered []ai.Tool, _ float64) (ai.ChatResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.messages = append(f.messages, append([]ai.AgentMessage(nil), msgs...))
	names := make([]string, 0, len(offered))
	for _, o := range offered {
		names = append(names, o.Function.Name)
	}
	f.offered = append(f.offered, names)
	if f.i >= len(f.results) {
		return ai.ChatResult{Content: "done", FinishReason: "stop"}, nil
	}
	res := f.results[f.i]
	f.i++
	return res, nil
}

func toolCall(id, name, args string) ai.ChatResult {
	var tc ai.ToolCall
	tc.ID = id
	tc.Type = "function"
	tc.Function.Name = name
	tc.Function.Arguments = args
	return ai.ChatResult{FinishReason: "tool_calls", ToolCalls: []ai.ToolCall{tc}}
}

func text(s string) ai.ChatResult {
	return ai.ChatResult{Content: s, FinishReason: "stop"}
}

func newService(t *testing.T) (*service.Service, *index.Index) {
	t.Helper()
	root := t.TempDir()
	notes := filepath.Join(root, "notes")
	assets := filepath.Join(root, "assets")
	if err := os.MkdirAll(notes, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(assets, 0o755); err != nil {
		t.Fatal(err)
	}
	ix, err := index.Open(filepath.Join(root, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ix.Close() })
	st := store.New(notes, assets)
	hist := history.New(filepath.Join(root, ".history"), 50)
	tr := trash.New(filepath.Join(root, ".trash"))
	return service.New(st, ix, st, hist, tr), ix
}

func TestRunReadWriteDone(t *testing.T) {
	svc, ix := newService(t)
	ctx := context.Background()
	if _, err := svc.SaveNote(ctx, "a.md", "hello", "*", false); err != nil {
		t.Fatal(err)
	}

	llm := &scriptedLLM{results: []ai.ChatResult{
		toolCall("c1", "notes_read", `{"path":"a.md"}`),
		toolCall("c2", "notes_write", `{"path":"b.md","body":"world","baseVersion":"*"}`),
		text("finished"),
	}}
	ag := agent.New(svc, llm, ix)
	s := ag.Start("u1", "alice", "admin")

	res, err := ag.Run(ctx, s, "read a and write b", nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != agent.StatusDone || res.Text != "finished" {
		t.Fatalf("result = %+v", res)
	}
	if len(res.Steps) != 2 {
		t.Fatalf("steps = %d, want 2", len(res.Steps))
	}
	if res.Steps[0].Status != "ok" || res.Steps[1].Status != "ok" {
		t.Errorf("step statuses = %+v", res.Steps)
	}
	if _, err := svc.GetNote(ctx, "b.md"); err != nil {
		t.Fatalf("b.md was not written: %v", err)
	}
}

func TestDangerousRequiresConfirmation(t *testing.T) {
	svc, ix := newService(t)
	ctx := context.Background()
	if _, err := svc.SaveNote(ctx, "del.md", "content", "*", false); err != nil {
		t.Fatal(err)
	}

	newAgent := func() (*agent.Agent, *scriptedLLM) {
		llm := &scriptedLLM{results: []ai.ChatResult{
			toolCall("c1", "notes_delete", `{"path":"del.md"}`),
			text("deleted"),
		}}
		return agent.New(svc, llm, ix), llm
	}

	// Approve path.
	ag, _ := newAgent()
	s := ag.Start("u1", "alice", "admin")
	res, err := ag.Run(ctx, s, "delete del.md", nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != agent.StatusNeedsConfirmation {
		t.Fatalf("status = %s, want needs_confirmation", res.Status)
	}
	if res.Pending == nil || res.Pending.Name != "notes_delete" {
		t.Fatalf("pending = %+v", res.Pending)
	}
	if !strings.Contains(res.Pending.Preview, "del.md") {
		t.Errorf("preview = %q", res.Pending.Preview)
	}
	// No side effect before confirmation.
	if _, err := svc.GetNote(ctx, "del.md"); err != nil {
		t.Fatal("note was deleted before confirmation")
	}

	res, err = ag.Confirm(ctx, s.ID, res.Pending.ToolCallID, agent.DecisionApprove, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != agent.StatusDone {
		t.Fatalf("status after approve = %s", res.Status)
	}
	if _, err := svc.GetNote(ctx, "del.md"); err == nil {
		t.Fatal("note should be deleted after approval")
	}

	// Reject path.
	if _, err := svc.SaveNote(ctx, "del.md", "content", "*", false); err != nil {
		t.Fatal(err)
	}
	ag2, _ := newAgent()
	s2 := ag2.Start("u1", "alice", "admin")
	res, err = ag2.Run(ctx, s2, "delete del.md", nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != agent.StatusNeedsConfirmation {
		t.Fatalf("status = %s", res.Status)
	}
	res, err = ag2.Confirm(ctx, s2.ID, res.Pending.ToolCallID, agent.DecisionReject, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != agent.StatusDone {
		t.Fatalf("status after reject = %s", res.Status)
	}
	if _, err := svc.GetNote(ctx, "del.md"); err != nil {
		t.Fatal("note should survive rejection")
	}
}

func TestMemberCannotRunDangerousTool(t *testing.T) {
	svc, ix := newService(t)
	ctx := context.Background()
	if _, err := svc.SaveNote(ctx, "keep.md", "safe", "*", false); err != nil {
		t.Fatal(err)
	}

	llm := &scriptedLLM{results: []ai.ChatResult{
		toolCall("c1", "notes_delete", `{"path":"keep.md"}`),
		text("ok"),
	}}
	ag := agent.New(svc, llm, ix)
	s := ag.Start("u2", "bob", "member")

	res, err := ag.Run(ctx, s, "delete keep.md", nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != agent.StatusDone {
		t.Fatalf("status = %s", res.Status)
	}
	if len(res.Steps) != 1 || res.Steps[0].Status != "denied" {
		t.Fatalf("steps = %+v", res.Steps)
	}
	if _, err := svc.GetNote(ctx, "keep.md"); err != nil {
		t.Fatal("member must not be able to delete")
	}
	// The dangerous tool must not even be offered to the model.
	for _, name := range llm.offered[0] {
		if name == "notes_delete" {
			t.Fatal("notes_delete was offered to a member")
		}
	}
}

func TestMaxSteps(t *testing.T) {
	svc, ix := newService(t)
	ctx := context.Background()

	results := make([]ai.ChatResult, 0, 5)
	for i := 0; i < 5; i++ {
		results = append(results, toolCall("c", "notes_list", `{}`))
	}
	llm := &scriptedLLM{results: results}
	ag := agent.New(svc, llm, ix)
	ag.SetPolicy(agent.PolicyDangerous, 2, nil)
	s := ag.Start("u1", "alice", "admin")

	res, err := ag.Run(ctx, s, "loop", nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != agent.StatusMaxSteps {
		t.Fatalf("status = %s, want max_steps", res.Status)
	}
	if s.Steps != 2 {
		t.Errorf("steps = %d, want 2", s.Steps)
	}
}

func TestAuditTrail(t *testing.T) {
	svc, ix := newService(t)
	ctx := context.Background()
	if _, err := svc.SaveNote(ctx, "a.md", "hello", "*", false); err != nil {
		t.Fatal(err)
	}

	llm := &scriptedLLM{results: []ai.ChatResult{
		toolCall("c1", "notes_read", `{"path":"a.md"}`),
		toolCall("c2", "notes_write", `{"path":"b.md","body":"SECRETBODY","baseVersion":"*"}`),
		text("done"),
	}}
	ag := agent.New(svc, llm, ix)
	s := ag.Start("u1", "alice", "admin")
	if _, err := ag.Run(ctx, s, "go", nil); err != nil {
		t.Fatal(err)
	}

	rows, err := ix.ListAIToolAudit(ctx, s.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("audit rows = %d, want 2", len(rows))
	}
	if rows[0].Tool != "notes_read" || rows[1].Tool != "notes_write" {
		t.Errorf("audit tools = %s, %s", rows[0].Tool, rows[1].Tool)
	}
	if rows[0].Status != "ok" || rows[1].Status != "ok" {
		t.Errorf("audit statuses = %s, %s", rows[0].Status, rows[1].Status)
	}
	if !rows[1].Destructive {
		t.Error("notes_write should be marked destructive")
	}
	if strings.Contains(rows[1].Args, "SECRETBODY") {
		t.Errorf("audit leaked note body: %s", rows[1].Args)
	}
	if !strings.Contains(rows[1].Args, "sha256:") {
		t.Errorf("audit args should hash content: %s", rows[1].Args)
	}
}

func TestConflictIsReportedNotOverwritten(t *testing.T) {
	svc, ix := newService(t)
	ctx := context.Background()
	if _, err := svc.SaveNote(ctx, "c.md", "v1", "*", false); err != nil {
		t.Fatal(err)
	}

	llm := &scriptedLLM{results: []ai.ChatResult{
		toolCall("c1", "notes_write", `{"path":"c.md","body":"changed","baseVersion":"stale"}`),
		text("done"),
	}}
	ag := agent.New(svc, llm, ix)
	s := ag.Start("u1", "alice", "admin")
	res, err := ag.Run(ctx, s, "update c", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Steps) != 1 || res.Steps[0].Status != "error" {
		t.Fatalf("steps = %+v", res.Steps)
	}
	note, err := svc.GetNote(ctx, "c.md")
	if err != nil {
		t.Fatal(err)
	}
	if note.Body != "v1" {
		t.Fatalf("note was overwritten on conflict: %q", note.Body)
	}
}

func TestConfirmRejectsUnknownDecision(t *testing.T) {
	svc, ix := newService(t)
	ctx := context.Background()
	if _, err := svc.SaveNote(ctx, "del.md", "x", "*", false); err != nil {
		t.Fatal(err)
	}
	llm := &scriptedLLM{results: []ai.ChatResult{toolCall("c1", "notes_delete", `{"path":"del.md"}`)}}
	ag := agent.New(svc, llm, ix)
	s := ag.Start("u1", "alice", "admin")
	res, err := ag.Run(ctx, s, "delete", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ag.Confirm(ctx, s.ID, res.Pending.ToolCallID, "maybe", nil); err == nil {
		t.Fatal("expected invalid decision error")
	}
	if _, err := svc.GetNote(ctx, "del.md"); err != nil {
		t.Fatal("note must survive an invalid decision")
	}
}

func TestStopRemovesSession(t *testing.T) {
	svc, ix := newService(t)
	ag := agent.New(svc, &scriptedLLM{}, ix)
	s := ag.Start("u1", "alice", "admin")
	ag.Stop(s.ID)
	if _, ok := ag.Session(s.ID); ok {
		t.Fatal("session should be gone after Stop")
	}
}
