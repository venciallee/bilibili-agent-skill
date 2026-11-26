package hook

import (
	"os"
	"strings"
	"testing"

	"github.com/venciallee/bilibili-agent-skill/internal/formula"
)

func set(t *testing.T) *formula.Set {
	s, err := formula.Builtin()
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestBadOpeningIsWeak(t *testing.T) {
	r := Score("大家好，欢迎来到我的频道，今天给大家分享一个超级好用的神器，记得一键三连", set(t))
	if r.Band != "弱" {
		t.Fatalf("a self-intro plus a 三连 ask should be weak, got %d %s %+v", r.Verdict, r.Band, r)
	}
	if len(r.RedLines) < 2 {
		t.Fatalf("want both red lines, got %v", r.RedLines)
	}
}

func TestConcreteHookBeatsVagueHook(t *testing.T) {
	good := Score("你视频的前 15 秒，就是没人看完的原因。我数了自己 48 期的后台，问题都出在同一句话上。", set(t))
	bad := Score("这个方法真的太绝了，效果炸裂，简直是天花板级别的神器", set(t))
	if good.Verdict <= bad.Verdict {
		t.Fatalf("good %d should beat bad %d", good.Verdict, bad.Verdict)
	}
	if good.Band == "弱" {
		t.Fatalf("concrete hook scored weak: %+v", good)
	}
	if good.Formula == "未归类" {
		t.Fatalf("concrete hook should match a formula")
	}
}

func TestSelfReferenceIsNotAnIntro(t *testing.T) {
	r := Score("我是在第 8 秒发现问题的，你可能也在犯同一个错", set(t))
	if len(r.RedLines) != 0 {
		t.Fatalf("\"我是在…\" is not a self-introduction: %v", r.RedLines)
	}
}

func TestBrevityBand(t *testing.T) {
	if brevity("太短") >= 100 {
		t.Fatal("two characters is not a hook")
	}
	if brevity("你剪辑时最浪费时间的那一步，其实二十行脚本就能替你做完") != 100 {
		t.Fatal("a 27-unit line should be in band")
	}
}

// The bands are only meaningful if they agree with the reference sets they
// were drawn against; see Band.
func TestBandsAgreeWithReferences(t *testing.T) {
	fs := set(t)
	raw, err := os.ReadFile("testdata_bad.txt")
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range Rank(strings.Split(string(raw), "\n"), fs) {
		if r.Band != "弱" {
			t.Errorf("bad opening scored %d %s: %s", r.Verdict, r.Band, r.Hook)
		}
	}
	ok := 0
	for _, f := range fs.Hooks {
		if r := Score(f.Example, fs); r.Band != "弱" {
			ok++
		} else {
			t.Logf("example scored weak (%d, weakest %s): %s", r.Verdict, r.Weakest, f.Example)
		}
	}
	if ok < 18 {
		t.Errorf("only %d of %d formula examples scored 可用 or better", ok, len(fs.Hooks))
	}
}
