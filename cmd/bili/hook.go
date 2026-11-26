package main

import (
	"flag"
	"fmt"

	"github.com/venciallee/bilibili-agent-skill/internal/formula"
	"github.com/venciallee/bilibili-agent-skill/internal/hook"
)

func cmdHook(args []string) error {
	fs := flag.NewFlagSet("hook", flag.ContinueOnError)
	one := fs.String("hook", "", "给单独一句钩子打分")
	asJSON := fs.Bool("json", false, "输出 JSON")
	pos, err := parse(fs, args)
	if err != nil {
		return err
	}
	var in []string
	if *one != "" {
		in = []string{*one}
	} else {
		p, err := needFile(pos, "钩子文件（每行一个）或 --hook \"一句话\"")
		if err != nil {
			return err
		}
		if in, err = lines(p); err != nil {
			return err
		}
	}
	set, err := formula.Builtin()
	if err != nil {
		return err
	}
	res := hook.Rank(in, set)
	if *asJSON {
		return emitJSON(res)
	}
	for _, r := range res {
		fmt.Printf("\n  %s\n  %s\n", r.Hook, "────────────────────")
		for _, k := range hook.Order {
			fmt.Printf("    %s  %3d  %s\n", k, r.Properties[k], bar(r.Properties[k]))
		}
		fmt.Printf("    总分  %3d  %s\n", r.Verdict, r.Band)
		if r.Matched > 0 {
			fmt.Printf("    公式  %s（命中 %d 条）\n", r.Formula, r.Matched)
		} else {
			fmt.Printf("    公式  未归类——通常说明这是一句摘要，不是钩子\n")
		}
		for _, rl := range r.RedLines {
			fmt.Printf("    红线  %s（-15）\n", rl)
		}
		fmt.Printf("    最弱  %s —— %s\n", r.Weakest, hook.Fix[r.Weakest])
	}
	if len(res) > 1 {
		fmt.Printf("\n  胜出：%s（%d，%s）\n", res[0].Hook, res[0].Verdict, res[0].Band)
	}
	fmt.Println("\n  启发式打分，未在 B站真实数据上校准。低分是回头再看的理由，高分什么也不保证。")
	return nil
}

func cmdFormulas(args []string) error {
	fs := flag.NewFlagSet("formulas", flag.ContinueOnError)
	asJSON := fs.Bool("json", false, "输出 JSON")
	if _, err := parse(fs, args); err != nil {
		return err
	}
	set, err := formula.Builtin()
	if err != nil {
		return err
	}
	if *asJSON {
		return emitJSON(set)
	}
	fmt.Println()
	for i, f := range set.Hooks {
		fmt.Printf("  %2d. %s  [%s]\n      结构：%s\n      示例：%s\n      失效：%s\n\n", i+1, f.Name, f.ID, f.Shape, f.Example, f.FailsWhen)
	}
	fmt.Println("  通用规则")
	for _, r := range set.Rules {
		fmt.Println("    -", r)
	}
	fmt.Println()
	return nil
}
