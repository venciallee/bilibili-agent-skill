// Package textx holds the small amount of Chinese-text handling every tool
// needs: how long a line is when spoken, how wide it is on a feed card, and
// a dependency-free stand-in for word segmentation.
//
// There is no tokenizer here on purpose. Character bigrams are crude, but
// they are honest about what they are, they need no dictionary, and for the
// jobs these tools do (does the cover repeat the title, did the topic change
// across this pause) they are good enough.
package textx

import (
	"regexp"
	"strings"
	"unicode"
)

// IsHan reports whether r is a CJK ideograph.
func IsHan(r rune) bool { return unicode.Is(unicode.Han, r) }

// isWide reports whether r occupies a full-width cell: ideographs, kana,
// hangul, full-width forms and CJK punctuation.
func isWide(r rune) bool {
	switch {
	case IsHan(r):
		return true
	case unicode.Is(unicode.Hiragana, r), unicode.Is(unicode.Katakana, r), unicode.Is(unicode.Hangul, r):
		return true
	case r >= 0x3000 && r <= 0x303F: // CJK symbols and punctuation
		return true
	case r >= 0xFF00 && r <= 0xFF60: // full-width forms
		return true
	case r >= 0xFE30 && r <= 0xFE4F: // CJK compatibility forms
		return true
	case r == '【' || r == '】' || r == '“' || r == '”' || r == '‘' || r == '’' || r == '…' || r == '—':
		return true
	}
	return false
}

// Width is the display width in "字": a full-width character is 1, a
// half-width one (ASCII letters, digits, spaces) is 0.5. This is how a title
// is measured against a feed card, which wraps on pixels, not on runes.
func Width(s string) float64 {
	w := 0.0
	for _, r := range s {
		if isWide(r) {
			w++
		} else if unicode.IsPrint(r) {
			w += 0.5
		}
	}
	return w
}

// RuneLen is the plain character count, which is what an upload form counts
// against a hard limit.
func RuneLen(s string) int { return len([]rune(s)) }

// HeadByWidth returns the longest prefix of s whose Width is at most w.
func HeadByWidth(s string, w float64) string {
	acc := 0.0
	var b strings.Builder
	for _, r := range s {
		d := 0.5
		if isWide(r) {
			d = 1
		}
		if acc+d > w {
			break
		}
		acc += d
		b.WriteRune(r)
	}
	return b.String()
}

var (
	latinWord = regexp.MustCompile(`[A-Za-z][A-Za-z'+#.-]*`)
	digitRun  = regexp.MustCompile(`[0-9０-９][0-9０-９,.，]*`)
)

// SpokenLen approximates how many syllables a line takes to say: one per
// ideograph, one per Latin word, and roughly one per two digits of a number
// (a figure like 200000 is said as 二十万, three syllables, not six).
func SpokenLen(s string) int {
	n := 0
	for _, r := range s {
		if IsHan(r) {
			n++
		}
	}
	n += len(latinWord.FindAllString(s, -1))
	for _, d := range digitRun.FindAllString(s, -1) {
		n += (len([]rune(d)) + 1) / 2
	}
	return n
}

// Compact strips everything except letters and digits and lowercases Latin,
// so two takes of the same sentence compare equal regardless of punctuation.
func Compact(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// stopBigrams are pairs too common to carry meaning on their own.
var stopBigrams = toSet(strings.Fields(`
一个 这个 那个 我们 你们 他们 自己 就是 然后 其实 什么 怎么 可以 没有 不是 还是 但是 因为 所以
如果 这样 那样 这么 那么 一下 一些 一点 时候 现在 今天 大家 视频 这期 已经 还有 而且 或者 的话
的时 是一 了一 我的 你的 他的 的是 是不 不会 也是 都是 就会 会有 有一 一种 这种 那种 东西 感觉
觉得 知道 看看 然后 之后 之前 之类 出来 起来 下来 上来 过来 一样 不要 需要 应该 能够 这里 那里
`))

// particles are function characters; a bigram containing one ("的弹",
// "是一") straddles a word boundary far more often than it is a word.
const particles = "的了是在和就也都还这那个们吗呢吧啊着把被很又才而与及或"

var stopLatin = toSet(strings.Fields(`the a an of for to in on and or is are with your you my i this that it how what why vs`))

func toSet(xs []string) map[string]bool {
	m := make(map[string]bool, len(xs))
	for _, x := range xs {
		m[x] = true
	}
	return m
}

// Terms is the bag of comparable units in s: overlapping bigrams of every
// run of ideographs, plus lowercased Latin words. Stop units are dropped.
func Terms(s string) map[string]int {
	out := map[string]int{}
	var run []rune
	flush := func() {
		for i := 0; i+1 < len(run); i++ {
			bg := string(run[i : i+2])
			if !stopBigrams[bg] && !strings.ContainsAny(bg, particles) {
				out[bg]++
			}
		}
		run = run[:0]
	}
	for _, r := range s {
		if IsHan(r) {
			run = append(run, r)
		} else {
			flush()
		}
	}
	flush()
	for _, w := range latinWord.FindAllString(s, -1) {
		lw := strings.ToLower(strings.Trim(w, ".-"))
		if len(lw) > 1 && !stopLatin[lw] {
			out[lw]++
		}
	}
	return out
}

// Shared returns the terms present in both a and b.
func Shared(a, b string) []string {
	ta, tb := Terms(a), Terms(b)
	var out []string
	for k := range ta {
		if tb[k] > 0 {
			out = append(out, k)
		}
	}
	return out
}

// Jaccard is |A∩B| / |A∪B| over term sets; 0 when both are empty.
func Jaccard(a, b map[string]int) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 0
	}
	inter, union := 0, 0
	for k := range a {
		union++
		if b[k] > 0 {
			inter++
		}
	}
	for k := range b {
		if a[k] == 0 {
			union++
		}
	}
	return float64(inter) / float64(union)
}

// concreteNumber matches a figure that carries information: Arabic digits,
// or a Chinese numeral with a unit. A bare 一 (一个, 一下) is grammar, not a
// number, so it only counts in front of 十/百/千/万/亿.
var concreteNumber = regexp.MustCompile(
	`[0-9０-９]+(?:[.,][0-9]+)*\s*(?:%|％|w|W|k|K|x|X|倍|万|亿|千|百)?` +
		`|[二三四五六七八九十两][零一二三四五六七八九十百千万亿两]*[个天年月周时分秒倍块元次期集款种步招条名位章节岁斤页行]` +
		`|一[十百千万亿][零一二三四五六七八九十百千万亿两]*`)

// Numbers returns every concrete figure in s.
func Numbers(s string) []string {
	m := concreteNumber.FindAllString(s, -1)
	for i := range m {
		m[i] = strings.TrimSpace(m[i])
	}
	return m
}

// CountAny counts non-overlapping occurrences of each needle in s, longest
// needles first so 一键三连 is not also counted as 三连.
func CountAny(s string, needles []string) (int, []string) {
	ls := strings.ToLower(s)
	sorted := append([]string(nil), needles...)
	for i := 1; i < len(sorted); i++ { // insertion sort by length, desc
		for j := i; j > 0 && len([]rune(sorted[j])) > len([]rune(sorted[j-1])); j-- {
			sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
		}
	}
	n := 0
	var hit []string
	for _, nd := range sorted {
		c := strings.Count(ls, strings.ToLower(nd))
		if c > 0 {
			n += c
			hit = append(hit, nd)
			ls = strings.ReplaceAll(ls, strings.ToLower(nd), " ")
		}
	}
	return n, hit
}
