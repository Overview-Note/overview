package history

import "testing"

func TestPruneKeepsNewest(t *testing.T) {
	s := New(t.TempDir(), 3)
	for i := 0; i < 10; i++ {
		if _, err := s.Snapshot("a.md", "", []byte("content")); err != nil {
			t.Fatal(err)
		}
	}
	revs, err := s.List("a.md")
	if err != nil {
		t.Fatal(err)
	}
	if len(revs) != 3 {
		t.Fatalf("got %d revisions, want 3", len(revs))
	}
}

func TestKeepZeroDisablesPruning(t *testing.T) {
	s := New(t.TempDir(), 0)
	for i := 0; i < 5; i++ {
		if _, err := s.Snapshot("a.md", "", []byte("content")); err != nil {
			t.Fatal(err)
		}
	}
	revs, err := s.List("a.md")
	if err != nil {
		t.Fatal(err)
	}
	if len(revs) != 5 {
		t.Fatalf("got %d revisions, want 5", len(revs))
	}
}
