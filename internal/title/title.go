// Package title lints a Bilibili title and its cover text as one pairing.
//
// The pairing is the unit, not the title. A cover that repeats the title
// wastes half the click surface, and that is the most common mistake this
// checks for.
//
// Limits are parameters because the platform changes them. The defaults:
//   - 80 characters is the upload form's hard limit for a video title.
//   - About 24 字 of display width is what the two-line title on a phone's
//     two-column home feed shows before it folds. This varies by device and
//     font size; treat it as an estimate.
//   - The first 12 字 are what a thumb-scrolling viewer actually reads, so
//     the subject has to be in there.
//   - Cover text beyond about 12 字 (two lines of six) turns into a smear at
//     feed-card size.
package title

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/venciallee/bilibili-agent-skill/internal/textx"
)

// Limits are the thresholds the linter checks against.
type Limits struct {
	Hard     int     // upload-form maximum, in characters
	Feed     float64 // display width before the feed card folds, in 字
	Glance   float64 // display width a scrolling viewer reads, in 字
	CoverMax float64 // display width of cover text before it smears, in 字
	Bracket  float64 // width a leading 【…】 may take of the glance zone
}

// Default are the limits as of 2026-11. Override them from the CLI when the
// platform moves.
var Default = Limits{Hard: 80, Feed: 24, Glance: 12, CoverMax: 12, Bracket: 6}

// Issue is one problem found.
type Issue struct {
	Kind    string `json:"kind"`
	Message string `json:"message"`
}

// Result is the lint of one title (and optional cover text).
type Result struct {
	Title  string   `json:"title"`
	Cover  string   `json:"cover,omitempty"`
	Chars  int      `json:"chars"`
	Width  float64  `json:"width"`
	Score  int      `json:"score"`
	Issues []Issue  `json:"issues"`
	Good   []string `json:"good"`
}

var (
	vague = []string{"震惊", "绝了", "逆天", "炸裂", "神器", "天花板", "封神", "王炸", "史诗级", "颠覆",
		"革命性", "无敌", "yyds", "绝绝子", "宝藏", "干货满满", "必看", "建议收藏", "神级", "顶级",
		"完美", "终极", "保姆级", "超详细", "超级", "疯狂", "惊呆"}
	absolute    = []string{"全网最", "史上最", "全网第一", "第一人", "唯一", "最强", "最好", "最佳", "顶级"}
	weakOpeners = regexp.MustCompile(`^(【[^】]*】)?\s*(记录一下|记录|分享一下|分享|今天|关于|聊聊|一个|我的|vlog|Vlog|VLOG|日常)`)
	leadBracket = regexp.MustCompile(`^\s*(【[^】]*】|\[[^\]]*\])`)
	repeatPunct = regexp.MustCompile(`[!！?？]{2,}|[.。…]{4,}`)
)

func latinCaps(s string) []string {
	var out []string
	for _, w := range strings.FieldsFunc(s, func(r rune) bool { return !(r < 128 && (unicode.IsLetter(r) || unicode.IsDigit(r))) }) {
		if len(w) > 2 && strings.ToUpper(w) == w && strings.IndexFunc(w, unicode.IsLetter) >= 0 {
			out = append(out, w)
		}
	}
	return out
}

// Check lints one title against l. cover may be empty.
func Check(title, cover string, l Limits) Result {
	t := strings.TrimSpace(title)
	r := Result{Title: t, Cover: strings.TrimSpace(cover), Chars: textx.RuneLen(t), Width: textx.Width(t)}
	add := func(kind, format string, a ...any) {
		r.Issues = append(r.Issues, Issue{kind, fmt.Sprintf(format, a...)})
	}
	good := func(format string, a ...any) { r.Good = append(r.Good, fmt.Sprintf(format, a...)) }

	switch {
	case r.Chars > l.Hard:
		add("长度", "%d 个字符，超过投稿页 %d 字的上限，发不出去", r.Chars, l.Hard)
	case r.Width > l.Feed:
		add("折叠", "宽度约 %.0f 字，手机首页双列卡片大约在 %.0f 字处折叠，只显示“%s…”", r.Width, l.Feed, textx.HeadByWidth(t, l.Feed))
	default:
		good("宽度约 %.0f 字，在手机首页卡片里能完整显示", r.Width)
	}

	glance := textx.HeadByWidth(t, l.Glance)
	if m := leadBracket.FindString(t); m != "" {
		if w := textx.Width(m); w > l.Bracket {
			add("前缀", "开头的 %s 占了 %.0f 字，扫一眼的前 %.0f 字只剩“%s”", strings.TrimSpace(m), w, l.Glance,
				strings.TrimPrefix(glance, strings.TrimSpace(m)))
		}
	}
	if weakOpeners.MatchString(t) {
		add("前置", "开头是“%s”这类铺垫词，把主体往前挪", weakOpeners.FindString(t))
	}

	if n, hit := textx.CountAny(t, vague); n > 0 {
		add("空话", "%s——换成一个数字、名字或日期", strings.Join(hit, "、"))
	}
	if _, hit := textx.CountAny(t, absolute); len(hit) > 0 {
		add("绝对化", "%s 是绝对化用语：商单里踩广告法，非商单也最容易招来弹幕挑刺。说出范围（“我测过的五款里”）", strings.Join(hit, "、"))
	}
	if caps := latinCaps(t); len(caps) > 2 {
		add("大写", "%d 个全大写英文词，超过两个就像垃圾广告", len(caps))
	}
	if m := repeatPunct.FindString(t); m != "" {
		add("标点", "“%s”连用，读起来像在喊", m)
	}

	if nums := textx.Numbers(t); len(nums) > 0 {
		if len(nums) > 3 {
			nums = nums[:3]
		}
		good("带了具体数字（%s）", strings.Join(nums, "、"))
	} else {
		add("无数字", "没有数字、日期或名字——这是最稳的单项修改")
	}
	if strings.HasSuffix(t, "？") || strings.HasSuffix(t, "?") {
		good("标题以问题结尾，留了口子")
	}

	if r.Cover != "" {
		if shared := covering(textx.Shared(t, r.Cover)); len(shared) > 0 {
			add("重复", "封面和标题重复了“%s”——封面应该说标题没说的", strings.Join(shared, "”“"))
		} else {
			good("封面字和标题没有重复")
		}
		if w := textx.Width(r.Cover); w > l.CoverMax {
			add("封面字", "封面字约 %.0f 字，超过 %.0f 字在信息流卡片上会糊成一片（两行、每行六个字左右）", w, l.CoverMax)
		}
	}

	score := 100 - 14*len(r.Issues) + 4*len(r.Good)
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}
	r.Score = score
	return r
}

// covering drops a term whose characters are all shown by other shared
// terms, so 剪辑/神器 is reported without the straddling 辑神. Terms with a
// character of their own are kept first; the rest only if they add one.
func covering(terms []string) []string {
	var kept, rest []string
	for i, t := range terms {
		unique := false
		for _, r := range t {
			seen := false
			for j, o := range terms {
				if j != i && strings.ContainsRune(o, r) {
					seen = true
					break
				}
			}
			if !seen {
				unique = true
				break
			}
		}
		if unique {
			kept = append(kept, t)
		} else {
			rest = append(rest, t)
		}
	}
	for _, t := range rest {
		for _, r := range t {
			if !strings.ContainsRune(strings.Join(kept, ""), r) {
				kept = append(kept, t)
				break
			}
		}
	}
	sort.Strings(kept)
	return kept
}

// Rank lints many titles against the same cover and sorts best first.
func Rank(titles []string, cover string, l Limits) []Result {
	var out []Result
	for _, t := range titles {
		if strings.TrimSpace(t) != "" {
			out = append(out, Check(t, cover, l))
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	return out
}
