package retention

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/venciallee/bilibili-agent-skill/internal/sample"
)

func TestSampleCurve(t *testing.T) {
	exp, err := sample.Load()
	if err != nil {
		t.Fatal(err)
	}
	c, err := LoadCSV(sample.Path("retention.csv"))
	if err != nil {
		t.Fatal(err)
	}
	if c.PercentX {
		t.Fatal("sample is in seconds")
	}
	rep := Analyze(c)
	if rep.HookLeak <= 0 || rep.HookBand == "" {
		t.Fatalf("bad hook leak %+v", rep)
	}
	if len(rep.Cliffs) == 0 {
		t.Fatal("sample has a built-in cliff")
	}
	// the generator's biggest drop is the confusing paragraph
	top := rep.Cliffs[0]
	if top.AtSeconds == nil || *top.AtSeconds < exp.ConfusionAt-10 || *top.AtSeconds > exp.ConfusionAt+5 {
		t.Fatalf("biggest cliff should be near %.0f s, got %+v", exp.ConfusionAt, top)
	}
}

func TestClockColumnAndPercentAxis(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "r.csv")
	os.WriteFile(p, []byte("0:00,100\n0:10,90\n0:20,85\n0:30,80\n1:00,70\n1:30,68\n2:00,50\n2:30,48\n3:00,46\n"), 0o644)
	c, err := LoadCSV(p)
	if err != nil {
		t.Fatal(err)
	}
	if c.PercentX {
		t.Fatal("M:SS column must be read as seconds")
	}
	if c.Points[4].X != 60 {
		t.Fatalf("1:00 should be 60 s, got %v", c.Points[4].X)
	}
	rep := Analyze(c)
	if rep.Cliffs[0].From != 90 {
		t.Fatalf("the 1:30 -> 2:00 drop is the cliff: %+v", rep.Cliffs)
	}
}
