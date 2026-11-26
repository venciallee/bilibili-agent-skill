// Package cut turns a timestamped transcript into an edit decision list.
//
// It finds three things:
//
//	DEAD    gaps between spoken cues longer than the floor
//	FILLER  cues that are nothing but filler (嗯、啊、那个、就是、然后…)
//	REPEAT  a sentence restarted - the abandoned first take of a line
//
// It never touches media. It prints the cuts, the total it would remove and
// the runtime you would land on, and you apply them in 剪映, 必剪, Premiere,
// DaVinci or whatever you edit in.
package cut

import (
	"fmt"
	"math"
	"regexp"
	"sort"

	"github.com/venciallee/bilibili-agent-skill/internal/textx"
	"github.com/venciallee/bilibili-agent-skill/internal/transcript"
)

// FillerOnly matches a cue that carries no words at all.
var FillerOnly = regexp.MustCompile(`(?i)^[\s，。、,.!?！？…~～\-]*((嗯+|啊+|呃+|额+|哦+|噢+|唔+|诶+|欸+|呐|那个+|就是说?|然后呢?|对吧|是吧|对对对|对|好的|好吧|好|行|嗯哼|其实|反正|这个|怎么说呢|我想想|等一下|稍等|um+|uh+|er+|ah+|ok|okay|so)[\s，。、,.!?！？…~～\-]*)+$`)

// Cut is one entry of the decision list.
type Cut struct {
	Kind  string  `json:"kind"`
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	Why   string  `json:"why"`
}

// Report is the whole decision list.
type Report struct {
	Source   string  `json:"source"`
	Duration float64 `json:"duration"`
	Cues     int     `json:"cues"`
	Floor    float64 `json:"floor"`
	Cuts     []Cut   `json:"cuts"`
	Removed  float64 `json:"removed"`
	Out      float64 `json:"out"`
}

func round3(v float64) float64 { return math.Round(v*1000) / 1000 }

func isSpeech(t string) bool { return t != "" && !FillerOnly.MatchString(t) }

func clip(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}

// restart reports whether later is a second attempt at earlier: both start
// with the same few characters, or earlier is an abandoned prefix of later.
// Comparison is on Compact text so punctuation and spacing do not matter.
func restart(earlier, later string) (bool, string) {
	a, b := []rune(textx.Compact(earlier)), []rune(textx.Compact(later))
	const probe = 6
	if len(a) >= probe && len(b) >= probe && string(a[:probe]) == string(b[:probe]) {
		return true, string(a[:probe])
	}
	// "今天我们来讲" … "今天我们来讲一下 Go 的并发": the first take was cut short
	if len(a) >= 4 && len(a) < len(b) && string(b[:len(a)]) == string(a) {
		return true, string(a)
	}
	return false, ""
}

// Build makes the decision list. floor is the longest pause kept intact;
// a longer gap is trimmed from the middle so both sides keep a breath.
func Build(cues []transcript.Cue, floor float64) Report {
	rep := Report{Duration: transcript.Duration(cues), Cues: len(cues), Floor: floor}
	var cuts []Cut
	for i, c := range cues {
		if FillerOnly.MatchString(c.Text) {
			cuts = append(cuts, Cut{"FILLER", c.Start, c.End, clip(c.Text, 24)})
		}
		if i > 0 {
			gap := c.Start - cues[i-1].End
			if gap > floor {
				keep := floor / 2
				cuts = append(cuts, Cut{"DEAD", round3(cues[i-1].End + keep), round3(c.Start - keep),
					formatGap(gap)})
			}
		}
		// A restart is compared with the last cue that was actually speech.
		// Comparing with the literal previous cue misses every retake with
		// an 嗯 between the two attempts, which is most of them.
		if isSpeech(c.Text) {
			j := i - 1
			for j >= 0 && !isSpeech(cues[j].Text) {
				j--
			}
			if j >= 0 {
				if ok, head := restart(cues[j].Text, c.Text); ok {
					cuts = append(cuts, Cut{"REPEAT", cues[j].Start, cues[j].End, "重说了“" + head + "”"})
				}
			}
		}
	}
	for _, c := range cuts {
		if c.End > c.Start {
			rep.Cuts = append(rep.Cuts, c)
		}
	}
	sort.SliceStable(rep.Cuts, func(i, j int) bool { return rep.Cuts[i].Start < rep.Cuts[j].Start })
	rep.Removed = round3(mergedLength(rep.Cuts))
	rep.Out = round3(rep.Duration - rep.Removed)
	return rep
}

// mergedLength sums the cut ranges without double-counting overlaps (a
// FILLER cue inside a REPEAT, say).
func mergedLength(cuts []Cut) float64 {
	if len(cuts) == 0 {
		return 0
	}
	s := append([]Cut(nil), cuts...)
	sort.Slice(s, func(i, j int) bool { return s[i].Start < s[j].Start })
	total, cs, ce := 0.0, s[0].Start, s[0].End
	for _, c := range s[1:] {
		if c.Start <= ce {
			if c.End > ce {
				ce = c.End
			}
			continue
		}
		total += ce - cs
		cs, ce = c.Start, c.End
	}
	return total + ce - cs
}

func formatGap(g float64) string { return fmt.Sprintf("停顿 %.2f 秒", g) }
