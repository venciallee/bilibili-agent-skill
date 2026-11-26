package chapters

import (
	"math"
	"testing"

	"github.com/venciallee/bilibili-agent-skill/internal/sample"

	"github.com/venciallee/bilibili-agent-skill/internal/transcript"
)

func TestDraftFindsSectionBreaks(t *testing.T) {
	cues, err := transcript.Load(sample.Path("transcript.srt"))
	if err != nil {
		t.Fatal(err)
	}
	exp, err := sample.Load()
	if err != nil {
		t.Fatal(err)
	}
	d := Make(cues, 6, 30, Bilibili)
	if !d.Valid {
		t.Fatalf("sample should validate: %v", d.Problems)
	}
	// every section the generator wrote must start a chapter, except the
	// ~20 s outro, which is shorter than the 30 s spacing
	var want []float64
	for name, at := range exp.Sections {
		if name != "outro" {
			want = append(want, at)
		}
	}
	for _, w := range want {
		found := false
		for _, c := range d.Chapters {
			if math.Abs(c.Start-w) <= 1 {
				found = true
			}
		}
		if !found {
			t.Errorf("no chapter near %.1f: %+v", w, d.Chapters)
		}
	}
	if len(d.Chapters) > 6 {
		t.Errorf("asked for 6, got %d", len(d.Chapters))
	}
}

func TestShortVideoIsRejected(t *testing.T) {
	cues := []transcript.Cue{{Start: 0, End: 60, Text: "第一段内容"}, {Start: 62, End: 120, Text: "第二段内容"}, {Start: 122, End: 200, Text: "第三段内容"}}
	d := Make(cues, 3, 30, Bilibili)
	if d.Valid {
		t.Fatal("a 3:20 video cannot use 分段章节")
	}
}
