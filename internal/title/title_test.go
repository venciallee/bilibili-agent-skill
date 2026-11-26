package title

import (
	"strings"
	"testing"
)

func kinds(r Result) string {
	var k []string
	for _, i := range r.Issues {
		k = append(k, i.Kind)
	}
	return strings.Join(k, ",")
}

func has(r Result, kind string) bool { return strings.Contains(","+kinds(r)+",", ","+kind+",") }

func TestHardLimit(t *testing.T) {
	r := Check(strings.Repeat("长", 81), "", Default)
	if !has(r, "长度") {
		t.Fatalf("81 characters should break the hard limit: %s", kinds(r))
	}
}

func TestFeedFold(t *testing.T) {
	r := Check("我用三十天时间把自己的剪辑流程从头到尾重新做了一遍结果发现最慢的一步根本不是剪辑", "", Default)
	if !has(r, "折叠") {
		t.Fatalf("a 40-字 title should fold on the feed: %s", kinds(r))
	}
}

func TestCoverRepeatsTitle(t *testing.T) {
	r := Check("剪映和达芬奇剪同一段素材，我只留了一个", "剪映达芬奇", Default)
	if !has(r, "重复") {
		t.Fatalf("cover repeats 剪映/达芬奇: %s", kinds(r))
	}
	r2 := Check("剪映和达芬奇剪同一段素材，我只留了一个", "输的那个更贵", Default)
	if has(r2, "重复") {
		t.Fatalf("different cover words should pass: %s", kinds(r2))
	}
}

func TestClickbaitAndAbsolute(t *testing.T) {
	r := Check("全网最强剪辑神器！！效果炸裂", "", Default)
	for _, k := range []string{"空话", "绝对化", "标点", "无数字"} {
		if !has(r, k) {
			t.Errorf("want %s in %s", k, kinds(r))
		}
	}
	if r.Score >= Check("我测了 5 款剪辑软件，导出最快的竟然最便宜", "", Default).Score {
		t.Fatal("clickbait should score below a concrete title")
	}
}

func TestLeadingBracketEatsGlance(t *testing.T) {
	r := Check("【4K60帧·中文字幕·全程无剪辑】这台相机的对焦到底有多快", "", Default)
	if !has(r, "前缀") {
		t.Fatalf("a long 【】 prefix should be flagged: %s", kinds(r))
	}
}
