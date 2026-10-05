package sync

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpenStateDefaults(t *testing.T) {
	dir := t.TempDir()
	st, err := OpenState(dir)
	if err != nil {
		t.Fatal(err)
	}
	snap := st.Snapshot()
	if snap.Direction != string(DirectionBoth) {
		t.Errorf("direction = %q, want both", snap.Direction)
	}
	if snap.IntervalSec != DefaultIntervalSec {
		t.Errorf("interval = %d, want %d", snap.IntervalSec, DefaultIntervalSec)
	}
	if snap.DeviceID == "" {
		t.Error("device id is empty")
	}
	if _, err := os.Stat(filepath.Join(dir, "sync.json")); !os.IsNotExist(err) {
		t.Error("state file should not exist until first mutation")
	}
}

func TestStatePersists(t *testing.T) {
	dir := t.TempDir()
	st, err := OpenState(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Mutate(func(d *stateData) {
		d.ServerURL = "https://notes.example.com"
		d.Enabled = true
		d.Notes["id-1"] = noteState{Path: "a.md", BaseVersion: "v1"}
	}); err != nil {
		t.Fatal(err)
	}

	reopened, err := OpenState(dir)
	if err != nil {
		t.Fatal(err)
	}
	snap := reopened.Snapshot()
	if snap.ServerURL != "https://notes.example.com" || !snap.Enabled {
		t.Errorf("snapshot = %+v", snap)
	}
	if snap.Notes["id-1"].Path != "a.md" || snap.Notes["id-1"].BaseVersion != "v1" {
		t.Errorf("note state = %+v", snap.Notes["id-1"])
	}
	if snap.DeviceID != st.Snapshot().DeviceID {
		t.Error("device id changed across reload")
	}
}

func TestTokenRoundTrip(t *testing.T) {
	dir := t.TempDir()
	if err := WriteToken(dir, "abc"); err != nil {
		t.Fatal(err)
	}
	got, err := ReadToken(dir)
	if err != nil || got != "abc" {
		t.Fatalf("ReadToken = %q, %v", got, err)
	}
	if err := ClearToken(dir); err != nil {
		t.Fatal(err)
	}
	got, err = ReadToken(dir)
	if err != nil || got != "" {
		t.Fatalf("after clear = %q, %v", got, err)
	}
}

func TestRecentlyWrittenSuppression(t *testing.T) {
	dir := t.TempDir()
	vault := newTestVault(t)
	eng, err := NewEngine(Options{DataDir: dir, Repo: vault.store, Raw: vault.store, Assets: vault.store, Logger: discardLogger()})
	if err != nil {
		t.Fatal(err)
	}
	eng.markWritten("a.md")
	if !eng.recentlyWritten("a.md") {
		t.Error("path should be suppressed right after a write")
	}
	if eng.recentlyWritten("b.md") {
		t.Error("unrelated path should not be suppressed")
	}
}
