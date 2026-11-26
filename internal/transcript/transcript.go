// Package transcript loads timestamped speech from the formats a Bilibili
// creator actually has lying around: SRT (剪映 / 必剪 / whisper exports),
// WebVTT, whisper JSON, and Bilibili's own BCC subtitle JSON
// ({"body":[{"from":..,"to":..,"content":..}]}), which is what the AI
// subtitle track is stored as.
package transcript

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// Cue is one timed line of speech. Times are seconds from the start.
type Cue struct {
	Start, End float64
	Text       string
}

// ParseTime accepts HH:MM:SS,mmm, HH:MM:SS.mmm, MM:SS(.mmm) and plain seconds.
func ParseTime(s string) (float64, error) {
	s = strings.TrimSpace(strings.ReplaceAll(s, ",", "."))
	s = strings.ReplaceAll(s, "：", ":")
	parts := strings.Split(s, ":")
	var h, m float64
	var sec string
	switch len(parts) {
	case 3:
		hh, err := strconv.Atoi(parts[0])
		if err != nil {
			return 0, err
		}
		mm, err := strconv.Atoi(parts[1])
		if err != nil {
			return 0, err
		}
		h, m, sec = float64(hh), float64(mm), parts[2]
	case 2:
		mm, err := strconv.Atoi(parts[0])
		if err != nil {
			return 0, err
		}
		m, sec = float64(mm), parts[1]
	case 1:
		sec = parts[0]
	default:
		return 0, fmt.Errorf("bad timestamp %q", s)
	}
	f, err := strconv.ParseFloat(sec, 64)
	if err != nil {
		return 0, err
	}
	return h*3600 + m*60 + f, nil
}

// Clock formats seconds as M:SS, or H:MM:SS past the hour.
func Clock(t float64) string {
	if t < 0 {
		t = 0
	}
	n := int(t)
	h, m, s := n/3600, (n%3600)/60, n%60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}

var (
	cueLine = regexp.MustCompile(`^\s*(\d[\d:.,]*)\s*-->\s*(\d[\d:.,]*)`)
	vttTag  = regexp.MustCompile(`<[^>]+>`)
)

// Load reads a transcript from path, choosing the parser by content rather
// than trusting the extension.
func Load(path string) ([]Cue, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Parse(raw)
}

// Parse parses transcript bytes in any supported format.
func Parse(raw []byte) ([]Cue, error) {
	trim := strings.TrimSpace(strings.TrimPrefix(string(raw), "\ufeff"))
	if strings.HasPrefix(trim, "{") || strings.HasPrefix(trim, "[") {
		return parseJSON([]byte(trim))
	}
	return parseTimed(trim), nil
}

func parseTimed(s string) []Cue {
	var cues []Cue
	var cur *Cue
	var text []string
	flush := func() {
		if cur != nil {
			cur.Text = strings.TrimSpace(strings.Join(text, " "))
			if cur.Text != "" {
				cues = append(cues, *cur)
			}
		}
		cur, text = nil, nil
	}
	for _, line := range strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n") {
		if m := cueLine.FindStringSubmatch(line); m != nil {
			flush()
			a, errA := ParseTime(m[1])
			b, errB := ParseTime(m[2])
			if errA != nil || errB != nil {
				continue
			}
			cur = &Cue{Start: a, End: b}
			continue
		}
		l := strings.TrimSpace(line)
		if cur == nil || l == "" {
			if l == "" && cur != nil && len(text) > 0 {
				flush()
			}
			continue
		}
		if _, err := strconv.Atoi(l); err == nil && len(text) == 0 {
			continue // an SRT index line that slipped through
		}
		text = append(text, strings.TrimSpace(vttTag.ReplaceAllString(l, "")))
	}
	flush()
	return cues
}

type bccLine struct {
	From    float64 `json:"from"`
	To      float64 `json:"to"`
	Content string  `json:"content"`
}

type whisperSeg struct {
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	Text  string  `json:"text"`
}

func parseJSON(raw []byte) ([]Cue, error) {
	var obj struct {
		Body     []bccLine    `json:"body"`
		Segments []whisperSeg `json:"segments"`
	}
	if raw[0] == '{' {
		if err := json.Unmarshal(raw, &obj); err != nil {
			return nil, fmt.Errorf("transcript json: %w", err)
		}
	} else {
		if err := json.Unmarshal(raw, &obj.Segments); err != nil {
			// a bare BCC body array
			if err2 := json.Unmarshal(raw, &obj.Body); err2 != nil {
				return nil, fmt.Errorf("transcript json: %w", err)
			}
		}
	}
	var cues []Cue
	for _, b := range obj.Body {
		if t := strings.TrimSpace(b.Content); t != "" {
			cues = append(cues, Cue{b.From, b.To, t})
		}
	}
	for _, s := range obj.Segments {
		if t := strings.TrimSpace(s.Text); t != "" {
			cues = append(cues, Cue{s.Start, s.End, t})
		}
	}
	return cues, nil
}

// Around returns the text spoken within pad seconds of t, capped at max runes.
func Around(cues []Cue, t, pad float64, max int) string {
	var parts []string
	for _, c := range cues {
		if c.Start <= t+pad && c.End >= t-pad {
			parts = append(parts, c.Text)
		}
	}
	s := strings.Join(parts, " ")
	r := []rune(s)
	if len(r) > max {
		return string(r[:max]) + "…"
	}
	return s
}

// Duration is the end of the last cue.
func Duration(cues []Cue) float64 {
	d := 0.0
	for _, c := range cues {
		if c.End > d {
			d = c.End
		}
	}
	return d
}
