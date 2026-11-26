// Package viral ranks collected videos by how far each beat its OWN UP主's
// median, then says what kind of win it was.
//
// Raw play counts rank account size, not ideas. A 40万 video on a 300万-fan
// account is a normal Tuesday; a 40万 video on an account whose median is
// 3万 is the thing worth studying. So every video is scored as a multiple of
// its UP主's median, which needs at least four videos per UP主 to mean
// anything - the tool says so rather than ranking noise.
//
// Bilibili exposes more than views, and the ratios say what kind of win it
// was. Each ratio is also compared with the UP主's own median ratio:
//
//	收藏型  favorites/view high: reference material people keep for later
//	投币型  coins/view high: coins are scarce, so this is an endorsement
//	讨论型  (danmaku+replies)/view high: it started an argument or a meme
//	点赞型  likes/view high and nothing else: liked, not kept
package viral

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/venciallee/bilibili-agent-skill/internal/formula"
)

// Video is one normalized record.
type Video struct {
	UP       string  `json:"up"`
	Title    string  `json:"title"`
	View     float64 `json:"view"`
	Like     float64 `json:"like,omitempty"`
	Coin     float64 `json:"coin,omitempty"`
	Favorite float64 `json:"favorite,omitempty"`
	Danmaku  float64 `json:"danmaku,omitempty"`
	Reply    float64 `json:"reply,omitempty"`
	Share    float64 `json:"share,omitempty"`
	ID       string  `json:"id,omitempty"`
	Pubdate  int64   `json:"pubdate,omitempty"`
}

func get(m map[string]any, keys ...string) any {
	for _, k := range keys {
		cur := any(m)
		for _, part := range strings.Split(k, ".") {
			mm, ok := cur.(map[string]any)
			if !ok {
				cur = nil
				break
			}
			cur = mm[part]
		}
		if cur != nil {
			return cur
		}
	}
	return nil
}

func f64(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case string:
		s := strings.TrimSpace(x)
		mult := 1.0
		switch {
		case strings.HasSuffix(s, "万"):
			mult, s = 1e4, strings.TrimSuffix(s, "万")
		case strings.HasSuffix(s, "亿"):
			mult, s = 1e8, strings.TrimSuffix(s, "亿")
		}
		f, _ := strconv.ParseFloat(strings.ReplaceAll(s, ",", ""), 64)
		return f * mult
	}
	return 0
}

func str(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	}
	return ""
}

// normalize accepts the shapes a creator is likely to have: this tool's own
// format, a web-interface "view" response ({"data":{"owner":..,"stat":..}}),
// and yt-dlp's info JSON (uploader, view_count, like_count, comment_count).
func normalize(m map[string]any) Video {
	if d, ok := m["data"].(map[string]any); ok {
		m = d
	}
	return Video{
		UP:       str(get(m, "up", "owner.name", "uploader", "author", "channel")),
		Title:    str(get(m, "title")),
		View:     f64(get(m, "view", "views", "play", "stat.view", "view_count")),
		Like:     f64(get(m, "like", "stat.like", "like_count")),
		Coin:     f64(get(m, "coin", "coins", "stat.coin")),
		Favorite: f64(get(m, "favorite", "fav", "favorites", "stat.favorite")),
		Danmaku:  f64(get(m, "danmaku", "video_review", "stat.danmaku")),
		Reply:    f64(get(m, "reply", "comment", "stat.reply", "comment_count")),
		Share:    f64(get(m, "share", "stat.share")),
		ID:       str(get(m, "bvid", "id", "url", "webpage_url")),
		Pubdate:  int64(f64(get(m, "pubdate", "timestamp", "created"))),
	}
}

// Load reads a JSON array, an object with a "videos" list, or JSON Lines
// (one object per line, which is what `yt-dlp -j` prints).
func Load(path string) ([]Video, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	raw = bytes.TrimSpace(bytes.TrimPrefix(raw, []byte("\xef\xbb\xbf")))
	var rows []map[string]any
	if len(raw) > 0 && raw[0] == '[' {
		if err := json.Unmarshal(raw, &rows); err != nil {
			return nil, err
		}
	} else {
		var obj map[string]any
		if json.Unmarshal(raw, &obj) == nil {
			if vs, ok := obj["videos"].([]any); ok {
				for _, v := range vs {
					if m, ok := v.(map[string]any); ok {
						rows = append(rows, m)
					}
				}
			} else {
				rows = append(rows, obj)
			}
		} else {
			sc := bufio.NewScanner(bytes.NewReader(raw))
			sc.Buffer(make([]byte, 1024*1024), 64*1024*1024)
			for n := 1; sc.Scan(); n++ {
				line := bytes.TrimSpace(sc.Bytes())
				if len(line) == 0 {
					continue
				}
				var m map[string]any
				if err := json.Unmarshal(line, &m); err != nil {
					return nil, fmt.Errorf("第 %d 行不是 JSON: %w", n, err)
				}
				rows = append(rows, m)
			}
		}
	}
	var out []Video
	for _, r := range rows {
		if v := normalize(r); v.Title != "" {
			if v.UP == "" {
				v.UP = "?"
			}
			out = append(out, v)
		}
	}
	return out, nil
}

// Outlier is one ranked video.
type Outlier struct {
	Video
	Median   float64            `json:"median"`
	Multiple float64            `json:"multiple"`
	Ratios   map[string]float64 `json:"ratio_multiples,omitempty"`
	Type     string             `json:"type,omitempty"`
	Formula  string             `json:"formula"`
	Young    bool               `json:"young"`
}

// Thin is an UP主 skipped for having too few videos.
type Thin struct {
	UP     string `json:"up"`
	Videos int    `json:"videos"`
}

// Report is the ranking.
type Report struct {
	Videos   int            `json:"videos"`
	UPs      int            `json:"ups"`
	Min      float64        `json:"min"`
	Outliers []Outlier      `json:"outliers"`
	Thin     []Thin         `json:"skipped_thin"`
	Formulas map[string]int `json:"formulas"`
	Types    map[string]int `json:"types"`
}

func median(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	s := append([]float64(nil), xs...)
	sort.Float64s(s)
	n := len(s)
	if n%2 == 1 {
		return s[n/2]
	}
	return (s[n/2-1] + s[n/2]) / 2
}

var ratioDefs = []struct {
	name string
	f    func(Video) (float64, bool)
}{
	{"收藏", func(v Video) (float64, bool) { return v.Favorite / v.View, v.Favorite > 0 }},
	{"投币", func(v Video) (float64, bool) { return v.Coin / v.View, v.Coin > 0 }},
	{"讨论", func(v Video) (float64, bool) { return (v.Danmaku + v.Reply) / v.View, v.Danmaku+v.Reply > 0 }},
	{"点赞", func(v Video) (float64, bool) { return v.Like / v.View, v.Like > 0 }},
}

// Rank scores every video against its UP主's median. now is used to flag
// videos younger than seven days, whose counts are still climbing.
func Rank(videos []Video, min float64, fs *formula.Set, now time.Time) Report {
	by := map[string][]Video{}
	var order []string
	for _, v := range videos {
		if _, ok := by[v.UP]; !ok {
			order = append(order, v.UP)
		}
		by[v.UP] = append(by[v.UP], v)
	}
	rep := Report{Videos: len(videos), UPs: len(by), Min: min, Formulas: map[string]int{}, Types: map[string]int{}}
	for _, up := range order {
		vs := by[up]
		if len(vs) < 4 {
			rep.Thin = append(rep.Thin, Thin{up, len(vs)})
			continue
		}
		views := make([]float64, len(vs))
		for i, v := range vs {
			views[i] = v.View
		}
		med := median(views)
		ratioMed := map[string]float64{}
		for _, rd := range ratioDefs {
			var rs []float64
			for _, v := range vs {
				if r, ok := rd.f(v); ok && v.View > 0 {
					rs = append(rs, r)
				}
			}
			if len(rs) >= 4 {
				ratioMed[rd.name] = median(rs)
			}
		}
		for _, v := range vs {
			if med <= 0 {
				continue
			}
			o := Outlier{Video: v, Median: med, Multiple: math.Round(v.View/med*100) / 100, Formula: "未归类"}
			if fs != nil {
				if f, _, ok := fs.Classify(v.Title); ok {
					o.Formula = f.Name
				}
			}
			if v.Pubdate > 0 && now.Sub(time.Unix(v.Pubdate, 0)) < 7*24*time.Hour {
				o.Young = true
			}
			best, bestM := "", 0.0
			for _, rd := range ratioDefs {
				m, ok := ratioMed[rd.name]
				r, has := rd.f(v)
				if !ok || !has || m <= 0 || v.View <= 0 {
					continue
				}
				if o.Ratios == nil {
					o.Ratios = map[string]float64{}
				}
				x := math.Round(r/m*100) / 100
				o.Ratios[rd.name] = x
				if x > bestM {
					best, bestM = rd.name, x
				}
			}
			if bestM >= 1.5 {
				o.Type = best + "型"
			}
			if o.Multiple >= min {
				rep.Outliers = append(rep.Outliers, o)
			}
		}
	}
	sort.SliceStable(rep.Outliers, func(i, j int) bool { return rep.Outliers[i].Multiple > rep.Outliers[j].Multiple })
	for _, o := range rep.Outliers {
		rep.Formulas[o.Formula]++
		if o.Type != "" {
			rep.Types[o.Type]++
		}
	}
	return rep
}
