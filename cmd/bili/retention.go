package main

import (
	"flag"
	"fmt"
	"strings"

	"github.com/venciallee/bilibili-agent-skill/internal/danmaku"
	"github.com/venciallee/bilibili-agent-skill/internal/retention"
	"github.com/venciallee/bilibili-agent-skill/internal/transcript"
)

func cmdRetention(args []string) error {
	fs := flag.NewFlagSet("retention", flag.ContinueOnError)
	dur := fs.Float64("duration", 0, "视频时长（秒）；横轴是百分比时用来换算成秒")
	tr := fs.String("transcript", "", "字幕文件：打印每个断崖处在说什么")
	dm := fs.String("danmaku", "", "弹幕 XML：打印每个断崖前后观众在说什么")
	asJSON := fs.Bool("json", false, "输出 JSON")
	pos, err := parse(fs, args)
	if err != nil {
		return err
	}
	p, err := needFile(pos, "留存曲线 CSV（两列：时间或进度百分比, 留存百分比）")
	if err != nil {
		return err
	}
	c, err := retention.LoadCSV(p)
	if err != nil {
		return err
	}
	var cues []transcript.Cue
	if *tr != "" {
		if cues, err = transcript.Load(*tr); err != nil {
			return err
		}
	}
	c.Duration = *dur
	if c.Duration == 0 && len(cues) > 0 {
		c.Duration = transcript.Duration(cues)
	}
	rep := retention.Analyze(c)

	said := map[int]string{}
	heard := map[int]string{}
	var dmItems []danmaku.Item
	if *dm != "" {
		f, err := danmaku.Load(*dm)
		if err != nil {
			return err
		}
		dmItems = f.Items
	}
	for i, cl := range rep.Cliffs {
		if cl.AtSeconds == nil {
			continue
		}
		t := *cl.AtSeconds
		if len(cues) > 0 {
			end, _ := c.Seconds(cl.To)
			said[i] = transcript.Around(cues, (t+end)/2, (end-t)/2+2, 70)
		}
		if len(dmItems) > 0 {
			end, _ := c.Seconds(cl.To)
			heard[i] = nearbyDanmaku(dmItems, t-10, end+5)
		}
	}
	if *asJSON {
		return emitJSON(map[string]any{"report": rep, "said": said, "danmaku": heard})
	}
	fmt.Printf("\n  %s   %d 个点   %.1f%% → %.1f%%\n\n", p, rep.Points, rep.Start, rep.End)
	fmt.Printf("  开头流失  前 30 秒流失 %.1f%%   [%s]\n", rep.HookLeak, rep.HookBand)
	if rep.Lost5s != nil {
		fmt.Printf("            其中前 5 秒流失 %.1f%%\n", *rep.Lost5s)
	}
	fmt.Println("            25% 以内算健康是经验阈值，不是 B站官方基准。先修第一句，再碰别的。")
	fmt.Println("\n  断崖      观众真正离开的时刻")
	for i, cl := range rep.Cliffs {
		where := fmt.Sprintf("%.1f%%–%.1f%%", cl.From, cl.To)
		if cl.AtSeconds != nil {
			end, _ := c.Seconds(cl.To)
			where = transcript.Clock(*cl.AtSeconds) + "–" + transcript.Clock(end)
		}
		fmt.Printf("    -%5.1f%%  在 %11s", cl.Lost, where)
		if s := said[i]; s != "" {
			fmt.Printf("   说的是：“%s”", s)
		}
		fmt.Println()
		if h := heard[i]; h != "" {
			fmt.Printf("                       弹幕：%s\n", h)
		}
	}
	if len(rep.Cliffs) == 0 {
		fmt.Println("    没有超过 0.8% 的单点下跌——流失全是滑坡，不是某个时刻")
	}
	fmt.Printf("\n  滑坡      中段平均 %s流失 %.3f%%\n", rep.SlideUnit, rep.SlidePer)
	fmt.Println("            平稳的滑坡是节奏问题，不是内容问题。剪中段，别重写。")
	if rep.Coarse {
		fmt.Println("\n  只有不到 20 个点：如果是从截图估读的，断崖位置有 ±一个采样间隔的误差。")
	}
	fmt.Println()
	return nil
}

func nearbyDanmaku(items []danmaku.Item, from, to float64) string {
	kinds := map[string]int{}
	texts := map[string]map[string]int{}
	for _, it := range items {
		if it.T >= from && it.T <= to {
			kinds[it.Kind]++
			if texts[it.Kind] == nil {
				texts[it.Kind] = map[string]int{}
			}
			texts[it.Kind][clip(it.Text, 12)]++
		}
	}
	if len(kinds) == 0 {
		return ""
	}
	var parts []string
	for _, k := range danmaku.Kinds {
		if kinds[k] > 0 {
			parts = append(parts, fmt.Sprintf("%s %d", k, kinds[k]))
		}
	}
	s := strings.Join(parts, " · ")
	// quote the most frequent text of the most telling kind present
	for _, k := range []string{danmaku.Confused, danmaku.Skip, danmaku.Correction, danmaku.Laugh, danmaku.Hype} {
		best, bc := "", 0
		for t, c := range texts[k] {
			if c > bc || (c == bc && t < best) {
				best, bc = t, c
			}
		}
		if bc > 0 {
			s += fmt.Sprintf("   最多的是“%s”×%d", best, bc)
			break
		}
	}
	return s
}
