package main

import (
	"flag"
	"fmt"

	"github.com/venciallee/bilibili-agent-skill/internal/title"
)

func cmdTitle(args []string) error {
	fs := flag.NewFlagSet("title", flag.ContinueOnError)
	one := fs.String("title", "", "检查单个标题")
	cover := fs.String("cover", "", "封面上的大字（会检查和标题是否重复）")
	l := title.Default
	fs.IntVar(&l.Hard, "hard", l.Hard, "投稿页标题字数上限")
	fs.Float64Var(&l.Feed, "feed", l.Feed, "手机首页卡片折叠前能显示的字宽")
	fs.Float64Var(&l.CoverMax, "cover-max", l.CoverMax, "封面字的字宽上限")
	asJSON := fs.Bool("json", false, "输出 JSON")
	pos, err := parse(fs, args)
	if err != nil {
		return err
	}
	var in []string
	if *one != "" {
		in = []string{*one}
	} else {
		p, err := needFile(pos, "标题文件（每行一个）或 --title \"...\"")
		if err != nil {
			return err
		}
		if in, err = lines(p); err != nil {
			return err
		}
	}
	res := title.Rank(in, *cover, l)
	if *asJSON {
		return emitJSON(res)
	}
	for _, r := range res {
		fmt.Printf("\n  《%s》\n  %d 字符 · 字宽约 %.0f   %d/100\n", r.Title, r.Chars, r.Width, r.Score)
		for _, i := range r.Issues {
			fmt.Printf("    ✗  %-4s %s\n", i.Kind, i.Message)
		}
		for _, g := range r.Good {
			fmt.Printf("    ✓       %s\n", g)
		}
	}
	if *cover != "" {
		fmt.Printf("\n  封面字：%s\n", *cover)
	}
	fmt.Println("\n  阈值是 2026-11 的经验值，平台改了就用 --hard / --feed / --cover-max 覆盖。")
	return nil
}
