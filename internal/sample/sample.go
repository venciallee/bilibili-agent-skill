// Package sample exposes where the synthetic files in testdata/ planted
// their signals, so tests assert against the generator instead of against
// numbers copied by hand.
package sample

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
)

// Expected mirrors testdata/expected.json.
type Expected struct {
	Duration      float64            `json:"duration"`
	Sections      map[string]float64 `json:"sections"`
	SkipTarget    float64            `json:"skip_target"`
	LaughAt       float64            `json:"laugh_at"`
	ConfusionAt   float64            `json:"confusion_at"`
	HypeAt        float64            `json:"hype_at"`
	CorrectionsAt float64            `json:"corrections_at"`
}

// Dir is the absolute path of testdata/.
func Dir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "testdata")
}

// Path joins name onto testdata/.
func Path(name string) string { return filepath.Join(Dir(), name) }

// Load reads testdata/expected.json.
func Load() (Expected, error) {
	var e Expected
	b, err := os.ReadFile(Path("expected.json"))
	if err != nil {
		return e, err
	}
	return e, json.Unmarshal(b, &e)
}
