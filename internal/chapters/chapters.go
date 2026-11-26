// Package chapters drafts Bilibili 分段章节 from a timestamped transcript and
// validates the draft against the platform's rules, so a list that would be
// rejected is caught here rather than in the 创作中心 editor.
//
// The rules, as of 2026-11 (override from the CLI when they move):
//   - the 稿件 must be past review, longer than 5 minutes, and not 互动视频
//   - 2 to 10 segments, none shorter than 5 seconds, none overlapping
//   - a segment title under 16 汉字 (or 32 Latin letters)
//
// Boundaries come from the pauses actually taken between sections, scored
// by how long the pause was and how much the vocabulary changes across it.
// The titles are topic words: a placeholder to be rewritten, not a summary.
package chapters

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/venciallee/bilibili-agent-skill/internal/textx"
	"github.com/venciallee/bilibili-agent-skill/internal/transcript"
)

// Rules are the platform constraints the draft is validated against.
type Rules struct {
	MinVideo   float64 // seconds; the official feature needs a longer video
	MinCount   int
	MaxCount   int
	MinSegment float64 // seconds, platform minimum
	TitleWidth float64 // 字; a Latin letter counts half
}

// Bilibili is the rule set as of 2026-11.
var Bilibili = Rules{MinVideo: 300, MinCount: 2, MaxCount: 10, MinSegment: 5, TitleWidth: 16}

// Chapter is one drafted segment.
type Chapter struct {
	Start      float64 `json:"start"`
	End        float64 `json:"end"`
	Label      string  `json:"label"`
	DraftTitle string  `json:"draft_title"`
	Seconds    float64 `json:"seconds"`
}

// Draft is the result, with every rule violation spelled out.
type Draft struct {
	Duration float64   `json:"duration"`
	Chapters []Chapter `json:"chapters"`
	Valid    bool      `json:"valid"`
	Problems []string  `json:"problems"`
}

type candidate struct {
	score float64
	at    float64
}

// Make drafts up to target chapters, keeping boundaries at least spacing
// seconds apart (a chapter shorter than half a minute is rarely worth a
// line in the list, even though the platform allows five seconds).
func Make(cues []transcript.Cue, target int, spacing float64, rules Rules) Draft {
	dur := transcript.Duration(cues)
	if target > rules.MaxCount {
		target = rules.MaxCount
	}
	var cand []candidate
	for i := 1; i < len(cues); i++ {
		gap := math.Max(0, cues[i].Start-cues[i-1].End)
		before := joinText(cues[max(0, i-12):i])
		after := joinText(cues[i:min(len(cues), i+12)])
		shift := 1 - textx.Jaccard(textx.Terms(before), textx.Terms(after))
		cand = append(cand, candidate{gap*1.6 + shift*3.2, cues[i].Start})
	}
	sort.SliceStable(cand, func(i, j int) bool { return cand[i].score > cand[j].score })
	picked := []float64{0}
	for _, c := range cand {
		if len(picked) >= target {
			break
		}
		ok := dur-c.at >= spacing
		for _, p := range picked {
			if math.Abs(c.at-p) < spacing {
				ok = false
				break
			}
		}
		if ok {
			picked = append(picked, c.at)
		}
	}
	sort.Float64s(picked)

	d := Draft{Duration: dur}
	for n, start := range picked {
		end := dur
		if n+1 < len(picked) {
			end = picked[n+1]
		}
		var seg []transcript.Cue
		for _, c := range cues {
			if c.Start >= start && c.Start < end {
				seg = append(seg, c)
			}
		}
		d.Chapters = append(d.Chapters, Chapter{
			Start: round2(start), End: round2(end), Label: transcript.Clock(start),
			DraftTitle: draftTitle(seg, cues), Seconds: round2(end - start),
		})
	}
	d.Problems = Validate(d, rules)
	d.Valid = len(d.Problems) == 0
	return d
}

// Validate lists every rule the draft breaks.
func Validate(d Draft, r Rules) []string {
	var p []string
	if d.Duration <= r.MinVideo {
		p = append(p, fmt.Sprintf("视频只有 %s，官方分段章节要求时长超过 %s——改用评论区时间戳（会变成可点击的跳转）",
			transcript.Clock(d.Duration), transcript.Clock(r.MinVideo)))
	}
	if n := len(d.Chapters); n < r.MinCount || n > r.MaxCount {
		p = append(p, fmt.Sprintf("%d 段，规则要求 %d 到 %d 段", n, r.MinCount, r.MaxCount))
	}
	if len(d.Chapters) > 0 && d.Chapters[0].Start != 0 {
		p = append(p, "第一段没有从 0:00 开始")
	}
	for i, c := range d.Chapters {
		if c.Seconds < r.MinSegment {
			p = append(p, fmt.Sprintf("第 %d 段只有 %.1f 秒，短于 %.0f 秒", i+1, c.Seconds, r.MinSegment))
		}
		if i > 0 && c.Start < d.Chapters[i-1].End {
			p = append(p, fmt.Sprintf("第 %d 段和上一段重叠", i+1))
		}
		if w := textx.Width(c.DraftTitle); w >= r.TitleWidth {
			p = append(p, fmt.Sprintf("第 %d 段标题约 %.0f 字，要少于 %.0f 个汉字", i+1, w, r.TitleWidth))
		}
	}
	return p
}

func joinText(cs []transcript.Cue) string {
	var b strings.Builder
	for _, c := range cs {
		b.WriteString(c.Text)
		b.WriteString(" ")
	}
	return b.String()
}

// draftTitle picks the two terms most distinctive to this segment: frequent
// here, rare elsewhere. It is a placeholder for the human to rewrite.
func draftTitle(seg, all []transcript.Cue) string {
	local := textx.Terms(joinText(seg))
	global := textx.Terms(joinText(all))
	type kv struct {
		k string
		v float64
	}
	var ranked []kv
	for k, n := range local {
		if n < 2 {
			continue
		}
		ranked = append(ranked, kv{k, float64(n) * float64(n) / float64(global[k])})
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].v == ranked[j].v {
			return ranked[i].k < ranked[j].k
		}
		return ranked[i].v > ranked[j].v
	})
	var picks []string
	for _, r := range ranked {
		dup := false
		for _, p := range picks {
			if strings.Contains(p, r.k) || strings.Contains(r.k, p) || overlaps(p, r.k) {
				dup = true
				break
			}
		}
		if !dup {
			picks = append(picks, r.k)
		}
		if len(picks) == 2 {
			break
		}
	}
	if len(picks) == 0 {
		return "（待命名）"
	}
	return strings.Join(picks, "·")
}

// overlaps reports whether two bigrams share a character (话题 / 题目), which
// would make the draft read as a stutter.
func overlaps(a, b string) bool {
	for _, r := range a {
		if strings.ContainsRune(b, r) && textx.IsHan(r) {
			return true
		}
	}
	return false
}

func round2(v float64) float64 { return math.Round(v*100) / 100 }
