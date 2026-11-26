package main

import (
	"flag"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/venciallee/bilibili-agent-skill/internal/formula"
	"github.com/venciallee/bilibili-agent-skill/internal/viral"
)

func cmdViral(args []string) error {
	fs := flag.NewFlagSet("viral", flag.ContinueOnError)
	min := fs.Float64("min", 1.5, "只显示超过 UP 主自己中位数这么多倍的视频")
	asJSON := fs.Bool("json", false, "输出 JSON")
	pos, err := parse(fs, args)
	if err != nil {
		return err
	}
	p, err := needFile(pos, "收集好的视频数据（JSON 数组或 yt-dlp -j 的逐行 JSON）")
	if err != nil {
		return err
	}
	vs, err := viral.Load(p)
	if err != nil {
		return err
	}
	set, err := formula.Builtin()
	if err != nil {
		return err
	}
	rep := viral.Rank(vs, *min, set, time.Now())
	if *asJSON {
		return emitJSON(rep)
	}
	fmt.Printf("\n  %d 个视频，%d 个 UP 主，只看 %.1f 倍以上\n\n", rep.Videos, rep.UPs, rep.Min)
	shown := rep.Outliers
	if len(shown) > 25 {
		shown = shown[:25]
	}
	for _, o := range shown {
		young := ""
		if o.Young {
			young = "  [发布不到 7 天，还在涨]"
		}
		fmt.Printf("    %5.2fx  %9s  vs 中位数 %8s   %-10s %s%s\n", o.Multiple, wan(o.View), wan(o.Median), clip(o.UP, 10), clip(o.Title, 30), young)
		line := "            公式：" + o.Formula
		if o.Type != "" {
			line += "   赢在：" + o.Type
		}
		if len(o.Ratios) > 0 {
			line += "   " + ratios(o.Ratios)
		}
		fmt.Println(line)
	}
	if len(rep.Outliers) == 0 {
		fmt.Println("    没有视频过线——每个 UP 主多收集几个，或者调低 --min")
	}
	if len(rep.Thin) > 0 {
		var t []string
		for _, th := range rep.Thin {
			t = append(t, fmt.Sprintf("%s（%d）", th.UP, th.Videos))
		}
		fmt.Printf("\n  跳过 %d 个只收集了不到 4 个视频的 UP 主——一两个视频算不出中位数：%s\n", len(rep.Thin), strings.Join(t, "、"))
	}
	if len(rep.Formulas) > 0 {
		fmt.Println("\n  爆款里的标题公式")
		printCounts(rep.Formulas)
	}
	if len(rep.Types) > 0 {
		fmt.Println("\n  爆款赢在哪（互动率相对 UP 主自己中位数的倍数，≥1.5 才算）")
		printCounts(rep.Types)
	}
	fmt.Println("\n  公式是对标题字面的判断，不是对它为什么火的解释。")
	fmt.Println()
	return nil
}

func wan(v float64) string {
	if v >= 1e4 {
		return fmt.Sprintf("%.1f万", v/1e4)
	}
	return fmt.Sprintf("%.0f", v)
}

func ratios(m map[string]float64) string {
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return m[keys[i]] > m[keys[j]] })
	var p []string
	for _, k := range keys {
		p = append(p, fmt.Sprintf("%s×%.1f", k, m[k]))
	}
	return strings.Join(p, " ")
}

func printCounts(m map[string]int) {
	type kv struct {
		k string
		v int
	}
	var s []kv
	for k, v := range m {
		s = append(s, kv{k, v})
	}
	sort.Slice(s, func(i, j int) bool {
		if s[i].v == s[j].v {
			return s[i].k < s[j].k
		}
		return s[i].v > s[j].v
	})
	for _, x := range s {
		fmt.Printf("    %2d  %s\n", x.v, x.k)
	}
}
