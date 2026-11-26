package main

import (
	"flag"
	"fmt"
	"math"
	"strings"

	"github.com/venciallee/bilibili-agent-skill/internal/danmaku"
	"github.com/venciallee/bilibili-agent-skill/internal/retention"
	"github.com/venciallee/bilibili-agent-skill/internal/transcript"
)

func cmdDanmaku(args []string) error {
	fs := flag.NewFlagSet("danmaku", flag.ContinueOnError)
	dur := fs.Float64("duration", 0, "视频时长（秒），默认取字幕或最后一条弹幕")
	bucket := fs.Float64("bucket", 0, "分桶秒数，默认按时长自动选")
	tr := fs.String("transcript", "", "字幕文件：打印每个信号处在说什么")
	ret := fs.String("retention", "", "留存曲线 CSV：按留下来的观众数归一化弹幕密度")
	asJSON := fs.Bool("json", false, "输出 JSON")
	pos, err := parse(fs, args)
	if err != nil {
		return err
	}
	p, err := needFile(pos, "弹幕文件（B站 XML，或 yt-dlp 下的 danmaku.xml）")
	if err != nil {
		return err
	}
	f, err := danmaku.Load(p)
	if err != nil {
		return err
	}
	if len(f.Items) == 0 {
		return fmt.Errorf("文件里没有可分析的弹幕")
	}
	var cues []transcript.Cue
	if *tr != "" {
		if cues, err = transcript.Load(*tr); err != nil {
			return err
		}
	}
	o := danmaku.Options{Duration: *dur, Bucket: *bucket}
	if o.Duration == 0 && len(cues) > 0 {
		o.Duration = transcript.Duration(cues)
	}
	if *ret != "" {
		c, err := retention.LoadCSV(*ret)
		if err != nil {
			return err
		}
		c.Duration = o.Duration
		o.Retained = c.At
	}
	rep := danmaku.Analyze(f, o)
	say := func(t float64) string {
		if len(cues) == 0 {
			return ""
		}
		return transcript.Around(cues, t, 3, 60)
	}
	if *asJSON {
		return emitJSON(rep)
	}

	fmt.Printf("\n  %s   %d 条弹幕", p, rep.Total)
	if rep.MaxLimit > 0 {
		fmt.Printf("（弹幕池上限 %d）", rep.MaxLimit)
	}
	fmt.Printf("   时长 %s   每格 %.0f 秒\n", transcript.Clock(rep.Duration), rep.Bucket)
	if rep.PoolFull {
		fmt.Println("  ⚠ 弹幕池已满：这是最新的一批，早期弹幕已被挤掉，分布会偏向最近的观众。")
	}
	if rep.Skipped > 0 {
		fmt.Printf("  跳过 %d 条高级/代码/BAS 弹幕（那是特效，不是观众在说话）。\n", rep.Skipped)
	}
	if rep.Normalized {
		fmt.Println("  已按留存归一化：密度 = 反应弹幕数 ÷ 当时还在看的观众比例。")
	} else {
		fmt.Println("  未归一化：开头人多，密度天然偏高。有留存曲线就加 --retention。")
	}

	fmt.Printf("\n  反应密度（不含打卡、三连、空降）\n    %s\n    %s\n", danmaku.Sparkline(rep.Series), ticks(len(rep.Series), rep.Bucket))

	fmt.Println("\n  空降（观众在帮别人跳过）")
	if len(rep.Skips) == 0 {
		fmt.Println("    没有成组的空降弹幕。")
	}
	for _, s := range rep.Skips {
		fmt.Printf("    %d 条在 %s–%s 之间发出，叫人空降到 %s\n", s.Count, transcript.Clock(s.FromMin), transcript.Clock(s.FromMax), transcript.Clock(s.Target))
		fmt.Printf("      → 观众认为大约 %s–%s 这一段可以跳过。", transcript.Clock(s.FromMedian), transcript.Clock(s.Target))
		if line := say(s.FromMedian); line != "" {
			fmt.Printf("这里在说：“%s”", line)
		}
		fmt.Println()
	}

	fmt.Println("\n  疑惑（？和“没懂”集中的地方）")
	if len(rep.Confusion) == 0 {
		fmt.Println("    没有明显的疑惑聚集。")
	}
	for _, s := range rep.Confusion {
		fmt.Printf("    %s–%s   %d 条   %s\n", transcript.Clock(s.Start), transcript.Clock(s.End), s.Count, tops(s.Top))
		if line := say(s.Start + (s.End-s.Start)/2); line != "" {
			fmt.Printf("      在说：“%s”\n", line)
		}
	}

	fmt.Println("\n  高峰（反应远高于这条视频自己的平均）")
	if len(rep.Peaks) == 0 {
		fmt.Println("    没有超过均值两个标准差的格子。")
	}
	for _, pk := range rep.Peaks {
		fmt.Printf("    %s–%s   %d 条   %s 占 %.0f%%   %s\n", transcript.Clock(pk.At), transcript.Clock(pk.End), pk.Count, pk.Dominant, pk.Share*100, tops(pk.Top))
		if line := say((pk.At + pk.End) / 2); line != "" {
			fmt.Printf("      在说：“%s”\n", line)
		}
	}

	if len(rep.Fixes) > 0 {
		fmt.Println("\n  纠错（值得在置顶评论里回应）")
		for _, s := range rep.Fixes {
			fmt.Printf("    %s   %d 条   %s\n", transcript.Clock(s.Start), s.Count, tops(s.Top))
		}
	}

	if rep.TooFew {
		fmt.Println("\n  沉默区   反应弹幕不到 100 条，沉默区没有统计意义，跳过。")
	} else if len(rep.Silences) > 0 {
		fmt.Println("\n  沉默区（很久没人开口）")
		for _, s := range rep.Silences {
			fmt.Printf("    %s–%s   %.0f 秒里只有 %d 条\n", transcript.Clock(s.Start), transcript.Clock(s.End), s.End-s.Start, s.Count)
		}
	}

	fmt.Println("\n  构成")
	var parts []string
	for _, k := range danmaku.Kinds {
		if n := rep.Kinds[k]; n > 0 {
			parts = append(parts, fmt.Sprintf("%s %d（%.0f%%）", k, n, float64(n)/float64(rep.Total)*100))
		}
	}
	fmt.Println("    " + strings.Join(parts, "  "))
	fmt.Println("\n  分类是按字面规则判的（“哈哈”算笑、“？”算疑惑），说的是观众打了什么，不是视频为什么好或不好。")
	fmt.Println()
	return nil
}

func tops(ts []danmaku.TextCount) string {
	var p []string
	for _, t := range ts {
		p = append(p, fmt.Sprintf("“%s”×%d", clip(t.Text, 10), t.Count))
	}
	return strings.Join(p, " ")
}

// ticks prints a time label under every sixth of the sparkline.
func ticks(n int, b float64) string {
	if n == 0 {
		return ""
	}
	line := []rune(strings.Repeat(" ", n+8))
	step := int(math.Max(1, math.Round(float64(n)/6)))
	for i := 0; i < n; i += step {
		lbl := []rune(transcript.Clock(float64(i) * b))
		if i+len(lbl) > len(line) {
			break
		}
		copy(line[i:], lbl)
	}
	return strings.TrimRight(string(line), " ")
}
