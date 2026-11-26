package cut

import (
	"testing"

	"github.com/venciallee/bilibili-agent-skill/internal/transcript"
)

const srt = `1
00:00:00,000 --> 00:00:02,000
今天我们来讲

2
00:00:02,100 --> 00:00:02,600
嗯

3
00:00:02,700 --> 00:00:06,000
今天我们来讲一下 Go 的并发模型

4
00:00:08,500 --> 00:00:10,000
那个，就是

5
00:00:10,100 --> 00:00:13,000
先看一个最简单的例子
`

func TestBuild(t *testing.T) {
	cues, err := transcript.Parse([]byte(srt))
	if err != nil || len(cues) != 5 {
		t.Fatalf("parse: %v %d", err, len(cues))
	}
	rep := Build(cues, 0.45)
	want := map[string]int{}
	for _, c := range rep.Cuts {
		want[c.Kind]++
	}
	if want["REPEAT"] != 1 || want["FILLER"] != 2 || want["DEAD"] != 1 {
		t.Fatalf("got %+v", rep.Cuts)
	}
	if rep.Out <= 0 || rep.Out >= rep.Duration {
		t.Fatalf("bad runtime %v of %v", rep.Out, rep.Duration)
	}
	// the dead-air cut keeps half the floor on each side
	for _, c := range rep.Cuts {
		if c.Kind == "DEAD" && (c.Start != 6.225 || c.End != 8.275) {
			t.Fatalf("dead cut should be trimmed from the middle: %+v", c)
		}
	}
}

func TestFillerOnly(t *testing.T) {
	for _, s := range []string{"嗯", "啊啊", "那个，就是", "然后呢……", "um, so"} {
		if !FillerOnly.MatchString(s) {
			t.Errorf("%q should be filler", s)
		}
	}
	for _, s := range []string{"然后我们打开设置", "那个按钮在右上角", "好看"} {
		if FillerOnly.MatchString(s) {
			t.Errorf("%q has words in it", s)
		}
	}
}
