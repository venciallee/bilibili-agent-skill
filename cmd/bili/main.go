// Command bili is the toolbox behind the Bilibili agent skills. Every
// subcommand reads local files and prints a report; none of them uploads,
// logs in, or calls a Bilibili API.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

const version = "1.0.0"

const usage = `bili - B站 UP 主工具箱（只读本地文件，不登录、不调 B站接口、不发布）

用法:
  bili hook      hooks.txt | --hook "一句话"         给开头 15 秒的钩子打分
  bili title     titles.txt | --title "..." [--cover "封面字"]   标题 + 封面字联检
  bili cut       transcript.srt [--floor 0.45]       字幕 → 粗剪决策表（停顿/口癖/重说）
  bili chapters  transcript.srt [--target 6]         字幕 → 分段章节草稿（按 B站规则校验）
  bili retention retention.csv [--transcript t.srt] [--danmaku dm.xml]   留存曲线找漏点
  bili danmaku   danmaku.xml [--transcript t.srt] [--retention r.csv]   弹幕信号：高峰/疑惑/空降/纠错
  bili viral     collected.json [--min 1.5]          按 UP 主自己的中位数找爆款，并判断赢在哪
  bili formulas                                      列出 21 个钩子公式
  bili version

每个子命令都支持 --json。字幕支持 SRT、WebVTT、whisper JSON 和 B站 BCC 字幕 JSON。
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	cmds := map[string]func([]string) error{
		"hook": cmdHook, "title": cmdTitle, "cut": cmdCut, "chapters": cmdChapters,
		"retention": cmdRetention, "danmaku": cmdDanmaku, "viral": cmdViral, "formulas": cmdFormulas,
	}
	name, args := os.Args[1], os.Args[2:]
	switch name {
	case "version", "--version", "-v":
		fmt.Println("bili", version)
		return
	case "help", "--help", "-h":
		fmt.Print(usage)
		return
	}
	run, ok := cmds[name]
	if !ok {
		fmt.Fprintf(os.Stderr, "未知子命令 %q\n\n%s", name, usage)
		os.Exit(2)
	}
	if err := run(args); err != nil {
		var ue usageError
		if errors.As(err, &ue) {
			fmt.Fprintf(os.Stderr, "%s\n\n%s", ue.msg, usage)
			os.Exit(2)
		}
		fmt.Fprintln(os.Stderr, "bili "+name+":", err)
		os.Exit(1)
	}
}

type usageError struct{ msg string }

func (u usageError) Error() string { return u.msg }

// parse lets flags appear anywhere, before or after the file argument, the
// way people actually type them.
func parse(fs *flag.FlagSet, args []string) ([]string, error) {
	fs.SetOutput(io.Discard)
	var pos []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, usageError{err.Error()}
		}
		rest := fs.Args()
		if len(rest) == 0 {
			return pos, nil
		}
		pos = append(pos, rest[0])
		args = rest[1:]
	}
}

func needFile(pos []string, what string) (string, error) {
	if len(pos) == 0 {
		return "", usageError{"缺少" + what}
	}
	if _, err := os.Stat(pos[0]); err != nil {
		return "", fmt.Errorf("读不到 %s: %w", pos[0], err)
	}
	return pos[0], nil
}

func lines(path string) ([]string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, l := range strings.Split(strings.ReplaceAll(string(b), "\r\n", "\n"), "\n") {
		if strings.TrimSpace(l) != "" {
			out = append(out, strings.TrimSpace(l))
		}
	}
	return out, nil
}

func emitJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", " ")
	return enc.Encode(v)
}

func bar(v int) string { return strings.Repeat("#", v/5) }

func clip(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}
