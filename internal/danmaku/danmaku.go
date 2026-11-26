// Package danmaku reads a Bilibili danmaku file and turns it into the one
// piece of feedback no other platform gives a creator: viewers typing, at
// the exact second, what they think of the video.
//
// It looks for five signals:
//
//	高峰    reaction density far above the video's own mean
//	疑惑    clusters of ？/没懂 - the paragraph that lost them
//	空降    "空降 2:33": a viewer telling others to skip to 2:33, which makes
//	        everything before it a section the audience judged skippable
//	纠错    corrections worth answering in the pinned comment
//	沉默区  long stretches nobody reacted to
//
// Known biases, which the report states rather than hides: the XML holds
// only the newest danmaku up to the pool limit (maxlimit), not all of them;
// the opening is inflated by 来了/前排 and by the fact that most viewers are
// still there; the ending is inflated by 三连 replies. Check-ins, 三连 and
// 空降 are therefore excluded from reaction density, and a retention curve,
// when given, divides density by the share of viewers still watching.
package danmaku

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Item is one danmaku.
type Item struct {
	T    float64 `json:"t"`
	Text string  `json:"text"`
	Mode int     `json:"mode"`
	Pool int     `json:"pool"`
	Kind string  `json:"kind"`
}

// File is a loaded danmaku file.
type File struct {
	Items    []Item
	MaxLimit int
	Skipped  int // advanced, code and BAS danmaku: not viewers talking
}

// Categories, in classification order: the first that matches wins.
const (
	Skip       = "空降"
	Confused   = "疑惑"
	Correction = "纠错"
	Laugh      = "笑"
	Hype       = "高能"
	CheckIn    = "打卡"
	Support    = "三连"
	Other      = "其他"
)

// Kinds lists the categories in display order.
var Kinds = []string{Laugh, Hype, Confused, Correction, Skip, CheckIn, Support, Other}

var (
	skipRe = regexp.MustCompile(`空降\s*(?:到|至)?\s*(\d{1,2})\s*(?:[:：.]|分钟?)\s*(\d{1,2})?`)
	rules  = []struct {
		kind string
		re   *regexp.Regexp
	}{
		{Confused, regexp.MustCompile(`^[\s?？¿]+$|[?？]{2,}|没懂|没听懂|啥意思|什么意思|看不懂|听不懂|懵了|不明白|没看懂|跟不上|没跟上`)},
		{Correction, regexp.MustCompile(`说错|口误|错了|不对吧|应该是|不是.{0,8}而是|字幕错|有误|纠正|笔误`)},
		{Laugh, regexp.MustCompile(`(?i)哈{2,}|h{3,}|233|笑死|xswl|绷不住|蚌埠住|^草+$|草草|乐死|笑出|好笑|笑了`)},
		{Hype, regexp.MustCompile(`(?i)高能|名场面|卧槽|我靠|^wc|牛[逼啊bB批]|太强|好强|强啊|好家伙|泪目|燃起来|绝了|神了|tql|yyds|awsl|帅`)},
		{CheckIn, regexp.MustCompile(`^(来了+|来啦|来辣|前排|第一|报道|报到|签到|爷青回|围观|前排围观|来了来了)[!！~～。.\s]*$|打卡|^[0-9]+\s*(楼|L|l)$`)},
		{Support, regexp.MustCompile(`(?i)三连|下次一定|投币|硬币|已投|收藏了|已收藏|码住|mark|马克|充电`)},
	}
)

// Classify names the category of one danmaku.
func Classify(text string) string {
	t := strings.TrimSpace(text)
	if skipRe.MatchString(t) {
		return Skip
	}
	for _, r := range rules {
		if r.re.MatchString(t) {
			return r.kind
		}
	}
	return Other
}

// SkipTarget returns the second a 空降 danmaku points at.
func SkipTarget(text string) (float64, bool) {
	m := skipRe.FindStringSubmatch(text)
	if m == nil {
		return 0, false
	}
	mm, _ := strconv.Atoi(m[1])
	ss := 0
	if m[2] != "" {
		ss, _ = strconv.Atoi(m[2])
	}
	if ss >= 60 {
		return 0, false
	}
	return float64(mm*60 + ss), true
}

// Load reads Bilibili XML (<i><d p="...">text</d></i>, which is also what
// yt-dlp saves as the "danmaku" subtitle) or a JSON list of
// {"time"|"progress"(ms), "text"|"content"} objects.
func Load(path string) (*File, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	trim := bytes.TrimSpace(bytes.TrimPrefix(raw, []byte("\xef\xbb\xbf")))
	if len(trim) > 0 && trim[0] == '[' {
		return parseJSON(trim)
	}
	return parseXML(trim)
}

func parseXML(raw []byte) (*File, error) {
	if len(raw) == 0 || raw[0] != '<' {
		return nil, errors.New("不像弹幕 XML。如果是用 curl 直接下的，加上 --compressed（接口返回的是压缩过的内容）")
	}
	dec := xml.NewDecoder(bytes.NewReader(raw))
	dec.Strict = false
	f := &File{}
	var cur *Item
	var inMax bool
	var text strings.Builder
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("弹幕 XML: %w", err)
		}
		switch el := tok.(type) {
		case xml.StartElement:
			switch el.Name.Local {
			case "maxlimit":
				inMax = true
				text.Reset()
			case "d":
				text.Reset()
				for _, a := range el.Attr {
					if a.Name.Local == "p" {
						if it, ok := parseP(a.Value); ok {
							cur = &it
						}
					}
				}
			}
		case xml.CharData:
			text.Write(el)
		case xml.EndElement:
			switch el.Name.Local {
			case "maxlimit":
				if inMax {
					f.MaxLimit, _ = strconv.Atoi(strings.TrimSpace(text.String()))
				}
				inMax = false
			case "d":
				if cur != nil {
					cur.Text = strings.TrimSpace(text.String())
					f.add(*cur)
				}
				cur = nil
			}
		}
	}
	return f, nil
}

// parseP reads the p attribute: time,mode,size,color,sent,pool,userhash,dmid.
func parseP(p string) (Item, bool) {
	parts := strings.Split(p, ",")
	if len(parts) < 2 {
		return Item{}, false
	}
	t, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return Item{}, false
	}
	mode, _ := strconv.Atoi(parts[1])
	pool := 0
	if len(parts) > 5 {
		pool, _ = strconv.Atoi(parts[5])
	}
	return Item{T: t, Mode: mode, Pool: pool}, true
}

func (f *File) add(it Item) {
	// modes 1-6 are scrolling/bottom/top/reverse text; 7 advanced, 8 code,
	// 9 BAS are authored effects, not a viewer reacting
	if it.Mode >= 7 || it.Text == "" {
		f.Skipped++
		return
	}
	it.Kind = Classify(it.Text)
	f.Items = append(f.Items, it)
}

func parseJSON(raw []byte) (*File, error) {
	var rows []map[string]any
	if err := json.Unmarshal(raw, &rows); err != nil {
		return nil, fmt.Errorf("弹幕 JSON: %w", err)
	}
	f := &File{}
	for _, r := range rows {
		it := Item{Mode: 1}
		switch {
		case r["time"] != nil:
			it.T = num(r["time"])
		case r["t"] != nil:
			it.T = num(r["t"])
		case r["progress"] != nil: // protobuf-derived exports use milliseconds
			it.T = num(r["progress"]) / 1000
		}
		for _, k := range []string{"text", "content", "msg"} {
			if s, ok := r[k].(string); ok {
				it.Text = strings.TrimSpace(s)
				break
			}
		}
		if r["mode"] != nil {
			it.Mode = int(num(r["mode"]))
		}
		f.add(it)
	}
	return f, nil
}

func num(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case string:
		f, _ := strconv.ParseFloat(x, 64)
		return f
	}
	return 0
}

// Options tune the analysis.
type Options struct {
	Duration float64                         // seconds; 0 means "last danmaku + 1 s"
	Bucket   float64                         // seconds; 0 means auto
	Retained func(t float64) (float64, bool) // share still watching at t, 0-100
}

// Peak is one bucket of unusually dense reaction.
type Peak struct {
	At       float64        `json:"at"`
	End      float64        `json:"end"`
	Count    int            `json:"count"`
	Rate     float64        `json:"rate"`
	Dominant string         `json:"dominant"`
	Share    float64        `json:"share"`
	Top      []TextCount    `json:"top"`
	Kinds    map[string]int `json:"kinds"`
}

// TextCount is a normalized danmaku text and how often it appeared.
type TextCount struct {
	Text  string `json:"text"`
	Count int    `json:"count"`
}

// SkipGroup is a set of 空降 danmaku pointing at the same moment.
type SkipGroup struct {
	Target     float64 `json:"target"`
	Count      int     `json:"count"`
	FromMedian float64 `json:"from_median"`
	FromMin    float64 `json:"from_min"`
	FromMax    float64 `json:"from_max"`
}

// Spot is a stretch of time with a count of one kind.
type Spot struct {
	Start float64     `json:"start"`
	End   float64     `json:"end"`
	Count int         `json:"count"`
	Top   []TextCount `json:"top,omitempty"`
}

// Report is the full analysis.
type Report struct {
	Total      int            `json:"total"`
	Skipped    int            `json:"skipped_effects"`
	MaxLimit   int            `json:"maxlimit"`
	PoolFull   bool           `json:"pool_full"`
	Duration   float64        `json:"duration"`
	Bucket     float64        `json:"bucket"`
	Normalized bool           `json:"normalized_by_retention"`
	Kinds      map[string]int `json:"kinds"`
	Series     []int          `json:"reaction_series"`
	Peaks      []Peak         `json:"peaks"`
	Confusion  []Spot         `json:"confusion"`
	Skips      []SkipGroup    `json:"skips"`
	Fixes      []Spot         `json:"corrections"`
	Silences   []Spot         `json:"silences"`
	TooFew     bool           `json:"too_few"`
}

func reaction(kind string) bool { return kind != CheckIn && kind != Support && kind != Skip }

// normText collapses runs of one character longer than three (哈哈哈哈哈,
// ？？？？) down to three, so repeats group together. RE2 has no
// backreferences, hence the loop.
func normText(s string) string {
	var b strings.Builder
	var prev rune
	run := 0
	for _, r := range strings.TrimSpace(s) {
		if r == prev {
			run++
		} else {
			prev, run = r, 1
		}
		if run <= 3 {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func topTexts(items []Item, n int) []TextCount {
	c := map[string]int{}
	for _, it := range items {
		c[normText(it.Text)]++
	}
	var out []TextCount
	for k, v := range c {
		out = append(out, TextCount{k, v})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count == out[j].Count {
			return out[i].Text < out[j].Text
		}
		return out[i].Count > out[j].Count
	})
	if len(out) > n {
		out = out[:n]
	}
	return out
}

// Analyze runs every signal over f.
func Analyze(f *File, o Options) Report {
	rep := Report{Total: len(f.Items), Skipped: f.Skipped, MaxLimit: f.MaxLimit, Kinds: map[string]int{}}
	if f.MaxLimit > 0 && float64(len(f.Items)+f.Skipped) >= 0.95*float64(f.MaxLimit) {
		rep.PoolFull = true
	}
	dur := o.Duration
	for _, it := range f.Items {
		rep.Kinds[it.Kind]++
		if o.Duration == 0 && it.T+1 > dur {
			dur = it.T + 1
		}
	}
	rep.Duration = dur
	if dur <= 0 {
		return rep
	}
	b := o.Bucket
	if b <= 0 {
		b = math.Max(5, math.Ceil(dur/60/5)*5)
	}
	rep.Bucket = b
	n := int(math.Ceil(dur / b))
	buckets := make([][]Item, n)
	for _, it := range f.Items {
		i := int(it.T / b)
		if i >= 0 && i < n {
			buckets[i] = append(buckets[i], it)
		}
	}

	// reaction density, optionally per viewer still watching
	rep.Series = make([]int, n)
	rate := make([]float64, n)
	rep.Normalized = o.Retained != nil
	for i, bk := range buckets {
		c := 0
		for _, it := range bk {
			if reaction(it.Kind) {
				c++
			}
		}
		rep.Series[i] = c
		rate[i] = float64(c)
		if o.Retained != nil {
			if r, ok := o.Retained((float64(i) + 0.5) * b); ok && r > 1 {
				rate[i] = float64(c) / (r / 100)
			} else {
				rep.Normalized = false
			}
		}
	}
	if !rep.Normalized {
		for i := range rate {
			rate[i] = float64(rep.Series[i])
		}
	}
	reactions := 0
	for _, c := range rep.Series {
		reactions += c
	}
	rep.TooFew = reactions < 100

	mean, sd := meanSD(rate)
	type pk struct {
		i int
		r float64
	}
	var pks []pk
	for i, r := range rate {
		if r >= mean+2*sd && rep.Series[i] >= 5 {
			pks = append(pks, pk{i, r})
		}
	}
	// Adjacent hot buckets are one moment, not two: a laugh that straddles
	// 3:20 and 3:30 is reported once, spanning both.
	sort.Slice(pks, func(a, c int) bool { return pks[a].i < pks[c].i })
	var runs [][]pk
	for _, p := range pks {
		if n := len(runs); n > 0 && runs[n-1][len(runs[n-1])-1].i == p.i-1 {
			runs[n-1] = append(runs[n-1], p)
		} else {
			runs = append(runs, []pk{p})
		}
	}
	for _, run := range runs {
		var re []Item
		kinds := map[string]int{}
		top := 0.0
		for _, p := range run {
			top = math.Max(top, p.r)
			for _, it := range buckets[p.i] {
				if reaction(it.Kind) {
					re = append(re, it)
					kinds[it.Kind]++
				}
			}
		}
		dom, dc := Other, 0
		for _, k := range Kinds {
			if kinds[k] > dc {
				dom, dc = k, kinds[k]
			}
		}
		if dom == Confused { // reported in its own section, with more context
			continue
		}
		rep.Peaks = append(rep.Peaks, Peak{
			At: float64(run[0].i) * b, End: float64(run[len(run)-1].i+1) * b, Count: len(re),
			Rate: math.Round(top*10) / 10, Dominant: dom,
			Share: math.Round(float64(dc)/float64(len(re))*100) / 100, Top: topTexts(re, 3), Kinds: kinds,
		})
	}
	sort.SliceStable(rep.Peaks, func(a, c int) bool { return rep.Peaks[a].Rate > rep.Peaks[c].Rate })
	if len(rep.Peaks) > 5 {
		rep.Peaks = rep.Peaks[:5]
	}

	rep.Confusion = spots(buckets, b, Confused, 3, 0.3)
	rep.Fixes = spots(buckets, b, Correction, 2, 0)
	rep.Skips = skipGroups(f.Items, dur)
	if !rep.TooFew {
		rep.Silences = silences(rep.Series, b, dur)
	}
	return rep
}

func meanSD(xs []float64) (float64, float64) {
	if len(xs) == 0 {
		return 0, 0
	}
	m := 0.0
	for _, x := range xs {
		m += x
	}
	m /= float64(len(xs))
	v := 0.0
	for _, x := range xs {
		v += (x - m) * (x - m)
	}
	return m, math.Sqrt(v / float64(len(xs)))
}

// spots merges adjacent buckets where kind has at least min danmaku and at
// least share of the bucket, then keeps the three largest.
func spots(buckets [][]Item, b float64, kind string, min int, share float64) []Spot {
	var out []Spot
	var cur *Spot
	var curItems []Item
	for i, bk := range buckets {
		var hit []Item
		for _, it := range bk {
			if it.Kind == kind {
				hit = append(hit, it)
			}
		}
		ok := len(hit) >= min && (share == 0 || float64(len(hit)) >= share*float64(len(bk)))
		if ok {
			if cur != nil && cur.End == float64(i)*b {
				cur.End += b
				cur.Count += len(hit)
				curItems = append(curItems, hit...)
				continue
			}
			if cur != nil {
				cur.Top = topTexts(curItems, 3)
				out = append(out, *cur)
			}
			cur = &Spot{Start: float64(i) * b, End: float64(i+1) * b, Count: len(hit)}
			curItems = append([]Item(nil), hit...)
		}
	}
	if cur != nil {
		cur.Top = topTexts(curItems, 3)
		out = append(out, *cur)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Count > out[j].Count })
	if len(out) > 3 {
		out = out[:3]
	}
	return out
}

// skipGroups pairs each 空降 with where it was sent and groups targets that
// land within ten seconds of each other.
func skipGroups(items []Item, dur float64) []SkipGroup {
	type pair struct{ from, to float64 }
	var ps []pair
	for _, it := range items {
		if it.Kind != Skip {
			continue
		}
		if to, ok := SkipTarget(it.Text); ok && to > it.T+5 && to <= dur+1 {
			ps = append(ps, pair{it.T, to})
		}
	}
	sort.Slice(ps, func(i, j int) bool { return ps[i].to < ps[j].to })
	var out []SkipGroup
	for i := 0; i < len(ps); {
		j := i
		for j < len(ps) && ps[j].to-ps[i].to <= 10 {
			j++
		}
		g := ps[i:j]
		if len(g) >= 2 {
			froms := make([]float64, len(g))
			tos := map[float64]int{}
			for k, p := range g {
				froms[k] = p.from
				tos[p.to]++
			}
			sort.Float64s(froms)
			target, tc := 0.0, 0
			for t, c := range tos {
				if c > tc || (c == tc && t < target) {
					target, tc = t, c
				}
			}
			out = append(out, SkipGroup{Target: target, Count: len(g), FromMedian: math.Round(froms[len(froms)/2]*10) / 10,
				FromMin: math.Round(froms[0]*10) / 10, FromMax: math.Round(froms[len(froms)-1]*10) / 10})
		}
		i = j
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Count > out[j].Count })
	return out
}

// silences finds runs of at least three buckets, away from the first and
// last 30 seconds, where reaction stayed under a quarter of the median.
func silences(series []int, b, dur float64) []Spot {
	xs := append([]int(nil), series...)
	sort.Ints(xs)
	med := float64(xs[len(xs)/2])
	floor := math.Max(1, 0.25*med)
	var out []Spot
	start := -1
	for i := 0; i <= len(series); i++ {
		t := float64(i) * b
		quiet := i < len(series) && t >= 30 && t+b <= dur-30 && float64(series[i]) <= floor
		if quiet && start < 0 {
			start = i
		}
		if !quiet && start >= 0 {
			if i-start >= 3 {
				c := 0
				for _, v := range series[start:i] {
					c += v
				}
				out = append(out, Spot{Start: float64(start) * b, End: float64(i) * b, Count: c})
			}
			start = -1
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].End-out[i].Start > out[j].End-out[j].Start })
	if len(out) > 3 {
		out = out[:3]
	}
	return out
}

// Sparkline renders the reaction series as block characters.
func Sparkline(series []int) string {
	blocks := []rune("▁▂▃▄▅▆▇█")
	hi := 0
	for _, v := range series {
		if v > hi {
			hi = v
		}
	}
	var sb strings.Builder
	for _, v := range series {
		if hi == 0 {
			sb.WriteRune(blocks[0])
			continue
		}
		sb.WriteRune(blocks[int(math.Round(float64(v)/float64(hi)*float64(len(blocks)-1)))])
	}
	return sb.String()
}
