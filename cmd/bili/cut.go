package main

import (
	"flag"
	"fmt"

	"github.com/venciallee/bilibili-agent-skill/internal/chapters"
	"github.com/venciallee/bilibili-agent-skill/internal/cut"
	"github.com/venciallee/bilibili-agent-skill/internal/transcript"
)

var kindCN = map[string]string{"DEAD": "停顿", "FILLER": "口癖", "REPEAT": "重说"}

func cmdCut(args []string) error {
	fs := flag.NewFlagSet("cut", flag.ContinueOnError)
	floor := fs.Float64("floor", 0.45, "保留的最长停顿（秒），更长的从中间剪掉")
	asJSON := fs.Bool("json", false, "输出 JSON")
	pos, err := parse(fs, args)
	if err != nil {
		return err
	}
	p, err := needFile(pos, "字幕文件（srt / vtt / whisper json / B站 BCC json）")
	if err != nil {
		return err
	}
	cues, err := transcript.Load(p)
	if err != nil {
		return err
	}
	if len(cues) == 0 {
		return fmt.Errorf("没读到任何字幕行——是 srt、vtt、whisper json 或 B站 BCC json 吗？")
	}
	rep := cut.Build(cues, *floor)
	rep.Source = p
	if *asJSON {
		return emitJSON(rep)
	}
	fmt.Printf("\n  %s   %s，%d 行字幕，停顿阈值 %.2f 秒\n\n", p, transcript.Clock(rep.Duration), rep.Cues, rep.Floor)
	for _, c := range rep.Cuts {
		fmt.Printf("    %s  %8.2f → %8.2f   %5.2f 秒   %s\n", kindCN[c.Kind], c.Start, c.End, c.End-c.Start, c.Why)
	}
	pct := 0.0
	if rep.Duration > 0 {
		pct = rep.Removed / rep.Duration * 100
	}
	fmt.Printf("\n  共 %d 处，剪掉 %.2f 秒，成片约 %s（短了 %.1f%%）\n", len(rep.Cuts), rep.Removed, transcript.Clock(rep.Out), pct)
	fmt.Println("  这是“说话部分”的剪法。视频里有长段无声演示的话，先对着素材核一遍再信这个时长。")
	fmt.Println()
	return nil
}

func cmdChapters(args []string) error {
	fs := flag.NewFlagSet("chapters", flag.ContinueOnError)
	target := fs.Int("target", 6, "想要几段（B站上限 10）")
	spacing := fs.Float64("spacing", 30, "两段之间至少隔多少秒")
	rules := chapters.Bilibili
	fs.Float64Var(&rules.MinVideo, "min-video", rules.MinVideo, "官方分段章节要求的最短视频时长（秒）")
	fs.IntVar(&rules.MaxCount, "max", rules.MaxCount, "最多段数")
	asJSON := fs.Bool("json", false, "输出 JSON")
	pos, err := parse(fs, args)
	if err != nil {
		return err
	}
	p, err := needFile(pos, "字幕文件")
	if err != nil {
		return err
	}
	cues, err := transcript.Load(p)
	if err != nil {
		return err
	}
	if len(cues) < 6 {
		return fmt.Errorf("字幕太短，分不了章节")
	}
	d := chapters.Make(cues, *target, *spacing, rules)
	if *asJSON {
		return emitJSON(d)
	}
	fmt.Printf("\n  %s   时长 %s\n\n", p, transcript.Clock(d.Duration))
	fmt.Println("  创作中心 → 稿件管理 → 个性化配置 → 分段章节（稿件过审后才能设置）：")
	for _, c := range d.Chapters {
		fmt.Printf("    %s – %s   %-12s  （%.0f 秒）\n", transcript.Clock(c.Start), transcript.Clock(c.End), c.DraftTitle, c.Seconds)
	}
	fmt.Println("\n  评论区 / 简介时间戳版（会变成可点击的跳转）：")
	for _, c := range d.Chapters {
		fmt.Printf("    %s %s\n", c.Label, c.DraftTitle)
	}
	if d.Valid {
		fmt.Printf("\n  %d 段，符合分段章节规则。\n", len(d.Chapters))
	} else {
		fmt.Printf("\n  %d 段，有问题：\n", len(d.Chapters))
		for _, pr := range d.Problems {
			fmt.Println("    ✗", pr)
		}
	}
	fmt.Println("  每一行都要重命名：这些是话题词，不是你的话。写成这一段的承诺，3 到 8 个字。")
	fmt.Println()
	return nil
}
