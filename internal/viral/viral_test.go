package viral

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/venciallee/bilibili-agent-skill/internal/formula"
	"github.com/venciallee/bilibili-agent-skill/internal/sample"
)

func TestSample(t *testing.T) {
	vs, err := Load(sample.Path("collected.json"))
	if err != nil {
		t.Fatal(err)
	}
	fs, _ := formula.Builtin()
	rep := Rank(vs, 1.5, fs, time.Unix(1764000000, 0))
	if len(rep.Thin) != 1 || rep.Thin[0].Videos != 3 {
		t.Fatalf("the three-video UP主 should be skipped: %+v", rep.Thin)
	}
	if len(rep.Outliers) == 0 {
		t.Fatal("no outliers")
	}
	top := rep.Outliers[0]
	if top.UP != "示例UP·甲" || top.Multiple < 8 {
		t.Fatalf("甲's 30-day video is ~8x its median (~8x): %+v", top)
	}
	var fav bool
	for _, o := range rep.Outliers {
		if o.UP == "示例UP·乙" && o.Type == "收藏型" {
			fav = true
		}
	}
	if !fav {
		t.Fatalf("乙's shortcut list is a favorites win: %+v", rep.Outliers)
	}
}

func TestYtDlpLinesAndViewAPI(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "x.jsonl")
	os.WriteFile(p, []byte(`{"uploader":"A","title":"t1","view_count":100,"like_count":5,"comment_count":1}
{"uploader":"A","title":"t2","view_count":200}
{"code":0,"data":{"owner":{"name":"B"},"title":"t3","bvid":"BV1x","stat":{"view":"1.2万","coin":30}}}
`), 0o644)
	vs, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(vs) != 3 || vs[0].UP != "A" || vs[0].View != 100 || vs[2].UP != "B" || vs[2].View != 12000 || vs[2].Coin != 30 {
		t.Fatalf("%+v", vs)
	}
}
