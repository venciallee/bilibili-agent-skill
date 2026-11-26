package formula

import "testing"

func TestBuiltinLoads(t *testing.T) {
	s, err := Builtin()
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Hooks) != 21 {
		t.Fatalf("want 21 formulas, got %d", len(s.Hooks))
	}
	seen := map[string]bool{}
	for _, f := range s.Hooks {
		if seen[f.ID] {
			t.Errorf("duplicate id %s", f.ID)
		}
		seen[f.ID] = true
		if f.Name == "" || f.Shape == "" || f.Example == "" || f.FailsWhen == "" || len(f.Match) == 0 {
			t.Errorf("%s: incomplete entry", f.ID)
		}
	}
}

// Every formula's own example must be classified as that formula. If this
// fails, either the example does not show the shape or the patterns do not
// describe it, and both are bugs in the data file.
func TestExamplesClassifyAsThemselves(t *testing.T) {
	s, err := Builtin()
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range s.Hooks {
		got, hits, ok := s.Classify(f.Example)
		if !ok {
			t.Errorf("%s: example matched nothing: %s", f.ID, f.Example)
			continue
		}
		if got.ID != f.ID {
			t.Errorf("%s: example classified as %s (%d hits, own %d): %s",
				f.ID, got.ID, hits, f.Hits(f.Example), f.Example)
		}
	}
}

func TestSummaryIsUnclassified(t *testing.T) {
	s, _ := Builtin()
	if f, _, ok := s.Classify("本期视频主要介绍一下剪辑软件的基本操作"); ok {
		t.Errorf("a plain summary should match nothing, got %s", f.ID)
	}
}
