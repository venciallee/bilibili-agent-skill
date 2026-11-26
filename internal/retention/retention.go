// Package retention reads an audience-retention curve and finds the leaks.
//
// Input is a two-column CSV: a position (seconds, M:SS, or percent of the
// video) and the percentage still watching. Bilibili's 创作中心 shows this
// curve per 稿件 but may not offer a file export; reading 15-20 points off a
// screenshot into a CSV works, and the report says so when the curve is that
// coarse.
//
// It reports three things, because they are three different problems with
// three different fixes:
//
//	开头流失  what is lost in the first 30 seconds (and the first 5)
//	断崖      single steep drops: a moment people left at
//	滑坡      the steady bleed across the middle
package retention

import (
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/venciallee/bilibili-agent-skill/internal/transcript"
)

// Point is one sample of the curve.
type Point struct{ X, Y float64 }

// Curve is a loaded retention curve.
type Curve struct {
	Points     []Point
	PercentX   bool    // the x axis is percent of the video, not seconds
	Duration   float64 // seconds, when known
	SourceRows int
}

// LoadCSV reads a curve. Rows that do not hold two numbers (a header, a
// note) are skipped.
func LoadCSV(path string) (*Curve, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	c := &Curve{}
	clock := false
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		c.SourceRows++
		var nums []float64
		for _, cell := range rec {
			if v, ok := number(cell); ok {
				if len(nums) == 0 && strings.ContainsAny(cell, ":：") {
					clock = true
				}
				nums = append(nums, v)
			}
		}
		if len(nums) >= 2 {
			c.Points = append(c.Points, Point{nums[0], nums[1]})
		}
	}
	if len(c.Points) < 8 {
		return nil, fmt.Errorf("只读到 %d 个数据点，至少需要 8 个", len(c.Points))
	}
	sort.SliceStable(c.Points, func(i, j int) bool { return c.Points[i].X < c.Points[j].X })
	c.PercentX = c.Points[len(c.Points)-1].X <= 100.5 && !clock
	return c, nil
}

func number(cell string) (float64, bool) {
	s := strings.TrimSpace(strings.TrimPrefix(cell, "\ufeff"))
	s = strings.NewReplacer("%", "", "％", "", ",", "", "秒", "", "s", "").Replace(s)
	if s == "" {
		return 0, false
	}
	if strings.ContainsAny(s, ":：") {
		v, err := transcript.ParseTime(s)
		return v, err == nil
	}
	v, err := strconv.ParseFloat(s, 64)
	return v, err == nil
}

// Seconds converts an x value to seconds, when that is possible.
func (c *Curve) Seconds(x float64) (float64, bool) {
	if !c.PercentX {
		return x, true
	}
	if c.Duration > 0 {
		return x / 100 * c.Duration, true
	}
	return 0, false
}

// At interpolates the share still watching at second t. ok is false when
// the curve cannot be placed in seconds.
func (c *Curve) At(t float64) (float64, bool) {
	xs := make([]float64, len(c.Points))
	for i, p := range c.Points {
		s, ok := c.Seconds(p.X)
		if !ok {
			return 0, false
		}
		xs[i] = s
	}
	if t <= xs[0] {
		return c.Points[0].Y, true
	}
	for i := 1; i < len(xs); i++ {
		if t <= xs[i] {
			span := xs[i] - xs[i-1]
			if span <= 0 {
				return c.Points[i].Y, true
			}
			f := (t - xs[i-1]) / span
			return c.Points[i-1].Y + f*(c.Points[i].Y-c.Points[i-1].Y), true
		}
	}
	return c.Points[len(c.Points)-1].Y, true
}

// Cliff is one steep drop.
type Cliff struct {
	From      float64  `json:"from"`
	To        float64  `json:"to"`
	Lost      float64  `json:"lost"`
	AtSeconds *float64 `json:"at_seconds"`
}

// Report is the analysis.
type Report struct {
	Points    int      `json:"points"`
	Start     float64  `json:"start"`
	End       float64  `json:"end"`
	Lost5s    *float64 `json:"lost_5s,omitempty"`
	HookLeak  float64  `json:"hook_leak"`
	HookBand  string   `json:"hook_band"`
	Cliffs    []Cliff  `json:"cliffs"`
	SlidePer  float64  `json:"slide_per_unit"`
	SlideUnit string   `json:"slide_unit"`
	Coarse    bool     `json:"coarse"`
}

func r2(v float64) float64 { return math.Round(v*100) / 100 }

// Analyze finds the hook leak, the cliffs and the slide.
func Analyze(c *Curve) Report {
	ps := c.Points
	start := ps[0].Y
	if start == 0 {
		start = 100
	}
	// The opening window is the first 30 seconds, or the first 10% when the
	// axis is a percentage and there is no duration to convert with.
	cutoff := 30.0
	if c.PercentX {
		if c.Duration > 0 {
			cutoff = 30 / c.Duration * 100
		} else {
			cutoff = 10
		}
	}
	hookEnd := start
	for _, p := range ps {
		if p.X <= cutoff && p.Y < hookEnd {
			hookEnd = p.Y
		}
	}
	rep := Report{Points: len(ps), Start: start, End: ps[len(ps)-1].Y, HookLeak: r2(start - hookEnd)}
	switch {
	case rep.HookLeak < 25:
		rep.HookBand = "健康"
	case rep.HookLeak < 40:
		rep.HookBand = "漏"
	default:
		rep.HookBand = "严重"
	}
	if !c.PercentX || c.Duration > 0 {
		if y, ok := c.At(5); ok && ps[0].X < 5 {
			v := r2(start - y)
			rep.Lost5s = &v
		}
	}

	type drop struct{ rate, x0, x1, d float64 }
	var drops []drop
	for i := 1; i < len(ps); i++ {
		d := ps[i-1].Y - ps[i].Y
		span := ps[i].X - ps[i-1].X
		if span <= 0 {
			span = 1
		}
		drops = append(drops, drop{d / span, ps[i-1].X, ps[i].X, d})
	}
	var mid []float64
	for _, d := range drops {
		if d.x0 > cutoff {
			mid = append(mid, d.rate)
		}
	}
	sort.SliceStable(drops, func(i, j int) bool { return drops[i].rate > drops[j].rate })
	// Adjacent steep intervals are one cliff, not several: a drop sampled
	// every five seconds would otherwise be reported as 4:00, 4:05, 4:10.
	// Steep means more than 0.8 points lost AND at least twice the typical
	// rate of the middle; without the second test a coarse curve would merge
	// its ordinary slide into one long "cliff".
	typical := 0.0
	if len(mid) > 0 {
		ms := append([]float64(nil), mid...)
		sort.Float64s(ms)
		typical = ms[len(ms)/2]
	}
	var steep []drop
	for _, d := range drops {
		if d.d > 0.8 && d.x0 >= cutoff && d.rate >= 2*typical { // the opening is the hook leak, not a cliff
			steep = append(steep, d)
		}
	}
	sort.Slice(steep, func(i, j int) bool { return steep[i].x0 < steep[j].x0 })
	for _, d := range steep {
		if n := len(rep.Cliffs); n > 0 && rep.Cliffs[n-1].To == r2(d.x0) {
			rep.Cliffs[n-1].To = r2(d.x1)
			rep.Cliffs[n-1].Lost = r2(rep.Cliffs[n-1].Lost + d.d)
			continue
		}
		rep.Cliffs = append(rep.Cliffs, Cliff{From: r2(d.x0), To: r2(d.x1), Lost: r2(d.d)})
	}
	sort.SliceStable(rep.Cliffs, func(i, j int) bool { return rep.Cliffs[i].Lost > rep.Cliffs[j].Lost })
	if len(rep.Cliffs) > 5 {
		rep.Cliffs = rep.Cliffs[:5]
	}
	for i := range rep.Cliffs {
		if s, ok := c.Seconds(rep.Cliffs[i].From); ok {
			s = math.Round(s*10) / 10
			rep.Cliffs[i].AtSeconds = &s
		}
	}
	if len(mid) > 0 {
		sum := 0.0
		for _, m := range mid {
			sum += m
		}
		rep.SlidePer = math.Round(sum/float64(len(mid))*1000) / 1000
	}
	rep.SlideUnit = "每秒"
	if c.PercentX {
		rep.SlideUnit = "每 1% 进度"
	}
	rep.Coarse = len(ps) < 20
	return rep
}
