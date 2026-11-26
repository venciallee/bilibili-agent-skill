package danmaku

import (
	"math"
	"testing"

	"github.com/venciallee/bilibili-agent-skill/internal/sample"
)

func TestClassify(t *testing.T) {
	cases := map[string]string{
		"空降 2:33":     Skip,
		"空降2分33":      Skip,
		"？？？":         Confused,
		"没听懂":         Confused,
		"应该是cid不是aid": Correction,
		"哈哈哈哈":        Laugh,
		"2333":        Laugh,
		"卧槽":          Hype,
		"前排":          CheckIn,
		"来了来了":        CheckIn,
		"第一次看到这种写法":   Other,
		"下次一定":        Support,
		"学到了":         Other,
	}
	for in, want := range cases {
		if got := Classify(in); got != want {
			t.Errorf("%q: got %s want %s", in, got, want)
		}
	}
	if v, ok := SkipTarget("空降 2:33"); !ok || v != 153 {
		t.Errorf("skip target %v %v", v, ok)
	}
	if _, ok := SkipTarget("空降 2:75"); ok {
		t.Error("75 seconds is not a time")
	}
}

func TestSample(t *testing.T) {
	exp, err := sample.Load()
	if err != nil {
		t.Fatal(err)
	}
	f, err := Load(sample.Path("danmaku.xml"))
	if err != nil {
		t.Fatal(err)
	}
	if f.MaxLimit != 1000 || f.Skipped != 3 {
		t.Fatalf("maxlimit %d skipped %d", f.MaxLimit, f.Skipped)
	}
	rep := Analyze(f, Options{Duration: exp.Duration})
	if rep.PoolFull {
		t.Fatal("482 of 1000 is not full")
	}
	if len(rep.Skips) == 0 || rep.Skips[0].Target != exp.SkipTarget || rep.Skips[0].Count < 10 {
		t.Fatalf("skips: %+v", rep.Skips)
	}
	near := func(at, want, tol float64) bool { return math.Abs(at-want) <= tol }
	var laugh, hype bool
	for _, p := range rep.Peaks {
		if p.Dominant == Laugh && near(p.At, exp.LaughAt, rep.Bucket) {
			laugh = true
		}
		if p.Dominant == Hype && near(p.At, exp.HypeAt, rep.Bucket) {
			hype = true
		}
	}
	if !laugh || !hype {
		t.Fatalf("peaks: %+v", rep.Peaks)
	}
	if len(rep.Confusion) == 0 || !near(rep.Confusion[0].Start, exp.ConfusionAt, 2*rep.Bucket) {
		t.Fatalf("confusion: %+v", rep.Confusion)
	}
	if len(rep.Fixes) == 0 || !near(rep.Fixes[0].Start, exp.CorrectionsAt, 2*rep.Bucket) {
		t.Fatalf("corrections: %+v", rep.Fixes)
	}
	if rep.Kinds[CheckIn] == 0 || rep.Kinds[Support] == 0 {
		t.Fatalf("kinds: %+v", rep.Kinds)
	}
}

func TestNormText(t *testing.T) {
	if normText("哈哈哈哈哈哈") != "哈哈哈" || normText("？？？？") != "？？？" || normText("好耶") != "好耶" {
		t.Fatal("normText")
	}
}
