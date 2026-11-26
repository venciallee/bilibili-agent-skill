// Package hook scores the spoken opening of a Bilibili video.
//
// Five properties, 0-100 each, and a verdict that is 60% the mean and 40%
// the weakest one. The weakest-link weighting is deliberate: a hook with
// four strong properties and one dead one leaks at the dead one, and an
// average hides that. On top of that, B站-specific red lines (opening with
// 大家好我是…, asking for 三连 before anything has been earned) each take
// 15 points off, because they are the single most common reason a viewer
// swipes away in the first five seconds.
//
// WHAT THIS IS NOT. It has not been calibrated against real Bilibili
// retention data. The word lists and bands are hand-built for spoken
// Chinese. Treat a low score as a reason to look again and a high score as
// nothing at all.
package hook

import (
	"math"
	"regexp"
	"sort"
	"strings"

	"github.com/venciallee/bilibili-agent-skill/internal/formula"
	"github.com/venciallee/bilibili-agent-skill/internal/textx"
)

// Property names, in display order.
const (
	Specificity = "具体"
	Address     = "对话"
	Stakes      = "代价"
	Curiosity   = "悬念"
	Brevity     = "长度"
)

// Order is the display order of the five properties.
var Order = []string{Specificity, Address, Stakes, Curiosity, Brevity}

// Fix is the one-line repair for each property when it is the weakest.
var Fix = map[string]string{
	Specificity: "把一个形容词换成数字、名字或日期",
	Address:     "前 12 个字里说出“你”",
	Stakes:      "说出继续用老办法要付出什么",
	Curiosity:   "砍掉自己回答了自己的那半句",
	Brevity:     "15 到 40 个字。读出来，一口气读不完就是太长",
}

var (
	vague = []string{"震惊", "绝了", "逆天", "炸裂", "神器", "天花板", "封神", "王炸", "史诗级", "颠覆",
		"革命性", "无敌", "牛逼", "yyds", "绝绝子", "宝藏", "干货满满", "必看", "全网最", "史上最",
		"最强", "超级", "巨好", "离谱", "疯狂", "爆炸", "惊人", "神级", "顶级", "完美"}
	filler = []string{"那个", "就是说", "然后", "其实吧", "嗯", "啊", "呃", "大家好", "欢迎", "今天",
		"这期视频", "本期", "视频", "关注", "一键三连", "三连", "点赞", "投币", "收藏", "话不多说", "废话不多说"}
	stakes = []string{"亏", "浪费", "白干", "白花", "踩坑", "坑", "翻车", "别再", "千万别", "后悔", "错过",
		"代价", "淘汰", "风险", "损失", "封号", "被骗", "智商税", "学费", "来不及", "失败", "放弃",
		"劝退", "毁", "掉粉", "限流", "打回"}
	curious = []string{"为什么", "怎么", "到底", "竟然", "居然", "结果", "但是", "却", "没想到", "只有",
		"真相", "原来", "直到", "除了", "除非", "唯一", "哪", "什么", "谁", "原因", "没人", "几乎",
		"之前", "其实", "同一", "一件事", "那一", "出在"}
	closers = regexp.MustCompile(`因为|所以|也就是说|答案是|总之|简单来说|总结一下`)
	you     = regexp.MustCompile(`你们|您|你`)
	crowd   = regexp.MustCompile(`大家|各位|家人们|兄弟们|宝子们`)
	named   = regexp.MustCompile(`《[^》]+》|「[^」]+」|“[^”]+”|@\S+|\b[A-Z][A-Za-z0-9+#.-]+`)
)

// RedLine is a B站-specific opening sin, checked separately from the five
// properties because no amount of specificity rescues it.
type RedLine struct {
	Pattern *regexp.Regexp
	Message string
}

// RedLines are applied to every hook.
var RedLines = []RedLine{
	{regexp.MustCompile(`(?i)大家好|哈喽|hello\s*大家|欢迎(来到|收看|回到)|我是(本|你们的)?(UP主|up主|阿婆主)`),
		"开场自我介绍：观众还不认识你，也还不在乎你是谁"},
	{regexp.MustCompile(`(先|记得|别忘了)?(一键三连|三连|点个?关注|点赞投币|投币收藏)`),
		"开头求三连：这时候你还什么都没给"},
	{regexp.MustCompile(`话不多说|废话不多说|直接开始|我们直接进入正题`),
		"“话不多说”：说这句话本身就是多说"},
}

// Result is one scored hook.
type Result struct {
	Hook       string         `json:"hook"`
	Properties map[string]int `json:"properties"`
	RedLines   []string       `json:"red_lines,omitempty"`
	Verdict    int            `json:"verdict"`
	Band       string         `json:"band"`
	Formula    string         `json:"formula"`
	Matched    int            `json:"-"`
	Weakest    string         `json:"weakest"`
}

func clamp(v float64) int { return int(math.Max(0, math.Min(100, math.Round(v)))) }

func specificity(t string) int {
	nums := len(textx.Numbers(t))
	v, _ := textx.CountAny(t, vague)
	f, _ := textx.CountAny(t, filler)
	n := len(named.FindAllString(t, -1))
	s := 34 + float64(nums)*22 - float64(v)*16 - float64(f)*5 + math.Min(18, float64(6*n))
	return clamp(s)
}

func address(t string) int {
	n := len(you.FindAllString(t, -1))
	if n == 0 && crowd.MatchString(t) {
		// 大家 / 各位 talks to a room, not to the one person watching
		return 16
	}
	first := 0.0
	if you.MatchString(string(headRunes(t, 12))) {
		first = 30
	}
	return clamp(26 + float64(n)*20 + first)
}

func stakesScore(t string) int {
	n, _ := textx.CountAny(t, stakes)
	c := 0.0
	if len(textx.Numbers(t)) > 0 {
		c = 14
	}
	return clamp(22 + float64(n)*26 + c)
}

func curiosity(t string) int {
	n, _ := textx.CountAny(t, curious)
	q := 0.0
	tt := strings.TrimSpace(t)
	if strings.HasSuffix(tt, "？") || strings.HasSuffix(tt, "?") {
		q = 18
	}
	closed := 0.0
	if closers.MatchString(t) {
		closed = -18
	}
	return clamp(24 + float64(n)*17 + q + closed)
}

// brevity: 15-40 spoken units is what lands inside ten seconds at the
// 4 字/秒 a Chinese voiceover runs at.
func brevity(t string) int {
	n := textx.SpokenLen(t)
	switch {
	case n == 0:
		return 0
	case n >= 15 && n <= 40:
		return 100
	case n < 15:
		return clamp(math.Max(30, 100-float64(15-n)*6))
	default:
		return clamp(math.Max(10, 100-float64(n-40)*4))
	}
}

func headRunes(s string, n int) []rune {
	r := []rune(s)
	if len(r) > n {
		return r[:n]
	}
	return r
}

// Band names a verdict.
//
// The cut lines (55 / 35) were set against two hand-written reference sets,
// not against viewing data: the 21 formula examples in hooks.json and the
// deliberately bad openings in testdata_bad.txt. Every bad opening must land
// in 弱 and at least 18 of the 21 examples in 可用 or better; the tests hold
// both. That makes the bands agree with the formulas. It does not make them
// a prediction.
func Band(v int) string {
	switch {
	case v >= 55:
		return "强"
	case v >= 35:
		return "可用"
	default:
		return "弱"
	}
}

// Score scores one hook against the formula set.
func Score(t string, fs *formula.Set) Result {
	t = strings.TrimSpace(t)
	p := map[string]int{
		Specificity: specificity(t),
		Address:     address(t),
		Stakes:      stakesScore(t),
		Curiosity:   curiosity(t),
		Brevity:     brevity(t),
	}
	sum, lo, weakest := 0, 101, ""
	for _, k := range Order {
		sum += p[k]
		if p[k] < lo {
			lo, weakest = p[k], k
		}
	}
	verdict := 0.6*float64(sum)/float64(len(Order)) + 0.4*float64(lo)
	var reds []string
	for _, rl := range RedLines {
		if rl.Pattern.MatchString(t) {
			reds = append(reds, rl.Message)
			verdict -= 15
		}
	}
	r := Result{Hook: t, Properties: p, RedLines: reds, Verdict: clamp(verdict), Weakest: weakest, Formula: "未归类"}
	r.Band = Band(r.Verdict)
	if fs != nil {
		if f, hits, ok := fs.Classify(t); ok {
			r.Formula, r.Matched = f.Name, hits
		}
	}
	return r
}

// Rank scores and sorts hooks, best first. The sort is stable so equal
// scores keep the order the writer gave them.
func Rank(lines []string, fs *formula.Set) []Result {
	var out []Result
	for _, l := range lines {
		if strings.TrimSpace(l) != "" {
			out = append(out, Score(l, fs))
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Verdict > out[j].Verdict })
	return out
}
