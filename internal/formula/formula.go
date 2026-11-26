// Package formula loads the 21 hook formulas and classifies a line of text
// against them. The classification is a judgement about the words, never a
// claim about why a video did well.
package formula

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sync"

	bilibiliskill "github.com/venciallee/bilibili-agent-skill"
)

// Formula is one entry of hooks.json.
type Formula struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Shape     string   `json:"shape"`
	Example   string   `json:"example"`
	FailsWhen string   `json:"fails_when"`
	Match     []string `json:"match"`

	re []*regexp.Regexp
}

// Set is the parsed file.
type Set struct {
	Version string     `json:"version"`
	Note    string     `json:"note"`
	Rules   []string   `json:"rules"`
	Hooks   []*Formula `json:"hooks"`
}

var (
	once    sync.Once
	builtin *Set
	loadErr error
)

// Parse decodes and compiles a hooks.json document.
func Parse(raw []byte) (*Set, error) {
	var s Set
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("hooks.json: %w", err)
	}
	for _, f := range s.Hooks {
		for _, p := range f.Match {
			re, err := regexp.Compile("(?i)" + p)
			if err != nil {
				return nil, fmt.Errorf("hooks.json %s: pattern %q: %w", f.ID, p, err)
			}
			f.re = append(f.re, re)
		}
	}
	return &s, nil
}

// Builtin returns the embedded formula set.
func Builtin() (*Set, error) {
	once.Do(func() { builtin, loadErr = Parse(bilibiliskill.HooksJSON) })
	return builtin, loadErr
}

// Hits counts how many of f's patterns match text.
func (f *Formula) Hits(text string) int {
	n := 0
	for _, re := range f.re {
		if re.MatchString(text) {
			n++
		}
	}
	return n
}

// Classify returns the formula with the most matching patterns. Ties go to
// the earlier formula in the file. ok is false when nothing matched, which
// usually means the line is a summary rather than a hook.
func (s *Set) Classify(text string) (f *Formula, hits int, ok bool) {
	for _, cand := range s.Hooks {
		if n := cand.Hits(text); n > hits {
			f, hits = cand, n
		}
	}
	return f, hits, f != nil
}
