# B站 Agent Skill

十二个帮你运营 B站账号的 Claude skill，外加一个 Go 写的命令行工具箱 `bili`。免费，MIT，不用注册，不用 API key，不用连任何账号。

其中一个按 21 个钩子公式写口播稿，并且在你开机录之前先给开头打分。一个把标题和封面大字当成一组来检查，因为分开写，正是一半点击面积在说重复话的原因。一个读你的留存曲线，告诉你观众在哪一秒走的、走的时候你在说什么。一个读弹幕，找出观众在哪里笑了、懵了、叫别人“空降”跳过你的开头——这是 YouTube 没有、只有 B站才有的反馈。一个把字幕变成粗剪决策表。一个去找你这个领域真正跑出来的视频，按它超出自己 UP 主中位数多少倍来排，而不是按账号多大来排，并且告诉你它赢在收藏、投币还是讨论。

**什么都不会替你发布。** skill 只负责写，投稿由你来点。

设计参考了 [Jakeschincariol/youtube-agent-skill](https://github.com/Jakeschincariol/youtube-agent-skill)，按 B站的规则、中文口播和弹幕文化重新做了一遍，工具用 Go 重写。区别见[下面](#和-youtube-版有什么不同)。

## 安装

把这段贴给 Claude：

```
https://github.com/venciallee/bilibili-agent-skill

安装这个 skill，然后确认 /bili-script 能用。
```

或者在 Claude Code 里自己装：

```bash
git clone https://github.com/venciallee/bilibili-agent-skill.git
cp -r bilibili-agent-skill/skills/bili-* ~/.claude/skills/
go install github.com/venciallee/bilibili-agent-skill/cmd/bili@latest   # 工具箱，需要 Go 1.22+
```

或者作为插件：

```
/plugin marketplace add venciallee/bilibili-agent-skill
/plugin install bilibili-agent
```

只想装在某个项目里：把同样的文件夹复制到项目的 `.claude/skills/`。用 Codex：复制到 `~/.codex/skills/`。完全不用 Claude Code：把任意一个 `SKILL.md` 贴在对话开头，它就会作为一个模式运行——只是没有 `bili` 工具，而 `/bili-script`、`/bili-danmaku`、`/bili-retention` 的大半价值都在工具里。

然后花十分钟填 [`templates/voice.md`](templates/voice.md)。复制到 `~/.claude/bilibili/voice.md` 填好，或者把三个你自己的视频发给 Claude，说“照这几个帮我写 voice.md”。每个 skill 都会先读它。在这里它比在任何地方都重要，因为你得对着镜头把这些字念出来。

## 十二个 skill

| 命令 | 做什么 |
| --- | --- |
| `/bili-script` | 一个想法变成口播稿。按 [21 个公式](skills/bili-script/hooks.json)写五个开头、打分，再写带画面标注的完整口播。 |
| `/bili-package` | 标题和封面大字作为一组：80 字上限、手机首页折叠、封面复述标题、空话和绝对化用语、4:3 封面安全区。 |
| `/bili-cut` | 字幕变成粗剪决策表：长停顿、口癖（嗯、啊、那个）、重说，带时间码。 |
| `/bili-chapters` | 字幕变成分段章节草稿，按 B站规则校验（过审、超过 5 分钟、2–10 段、每段 ≥5 秒、标题 <16 字）。 |
| `/bili-retention` | 读留存曲线：开头流失、断崖、滑坡，并对照字幕和弹幕说清楚每个断崖处发生了什么。 |
| `/bili-danmaku` | 读弹幕：空降区间、疑惑点、笑点和高能、纠错、沉默区，按留存归一化。 |
| `/bili-viral` | 你这个领域里什么在跑，按超出 UP 主自己中位数的倍数排，并判断赢在收藏、投币还是讨论。 |
| `/bili-clip` | 找出长视频里能独立成立的竖屏切片，为每条写新的第一句。 |
| `/bili-seo` | 简介、标签、分区、话题，以及这期应该赢下的三个搜索词。 |
| `/bili-comment` | 评论区分成四堆，用你的口吻回复，并说该置顶哪一条。 |
| `/bili-plan` | 一个塞得进你真实时间的一周：一个主视频、一个便宜的、两三条切片，给审核留时间。 |
| `/bili-audit` | 整个账号体检，最后只给**一个**修改，而不是二十个。 |

## `bili` 工具箱

一个二进制，七个子命令，只用 Go 标准库。它们是这些 skill 不只是提示词的原因。

```bash
bili hook --hook "一句话"                         # 钩子五项打分 + B站红线
bili title --title "..." --cover "封面大字"        # 标题 + 封面联检
bili cut transcript.srt                           # 粗剪决策表
bili chapters transcript.srt                      # 分段章节草稿 + 规则校验
bili retention retention.csv --transcript t.srt --danmaku dm.xml   # 观众在哪走的，走之前在说什么
bili danmaku danmaku.xml --transcript t.srt --retention r.csv      # 弹幕五信号
bili viral collected.json --min 2.0               # 按自己中位数倍数排的爆款
bili formulas                                     # 21 个钩子公式
```

字幕支持 SRT、WebVTT、whisper JSON 和 B站 BCC 字幕 JSON。每个子命令都有 `--json`。每样输入怎么拿到手，见 [docs/collect.md](docs/collect.md)。

用仓库自带的合成样例试一下：

```bash
cd testdata
bili danmaku danmaku.xml --transcript transcript.srt --retention retention.csv
```

```
  空降（观众在帮别人跳过）
    16 条在 0:19–0:37 之间发出，叫人空降到 2:39
      → 观众认为大约 0:30–2:39 这一段可以跳过。这里在说：“这里有一个空降，观众在帮别人跳过我的开头。…”

  疑惑（？和“没懂”集中的地方）
    4:10–4:30   32 条   “看不懂了”×8 “啥意思”×7 “？？？”×7

  高峰（反应远高于这条视频自己的平均）
    6:50–7:00   51 条   高能 占 86%   “太强了”×11 “好家伙”×8 “前方高能”×7
    3:20–3:40   79 条   笑 占 89%   “哈哈哈”×19 “2333”×15 “笑死”×15
```

## 细则

大多数技能包不写的部分。

**它不发布。** 每个 skill 都以一个可复制的块和一个问题结尾：就这么发，还是改？任何声称能无人值守替你运营账号的东西，都值得带着这个区别去看。

**它不登录，也不碰 B站接口。** 所有工具只读你本地的文件，数据怎么拿由你决定（创作中心、yt-dlp、手抄）。这是有意的：2026 年 1 月，收集 B站接口文档的 bilibili-API-collect 收到 B站委托律所的律师函后关停。这个包不封装任何接口、不做请求签名、不教人绕风控，也从不碰你的 cookie。

**`bili hook` 是启发式，不是预测器，而且没有用真实数据校准过。** YouTube 版的打分对照过 74 个真实开头；这个中文版没有——五项指标和词表是按中文口播手工设计的，再加上 B站特有的红线（开场自我介绍、开头求三连各扣 15 分）。分档线（55 / 35）只对着两组手写参照定过：21 个公式示例，和 12 个故意写坏的开头（`internal/hook/testdata_bad.txt`）。反例必须全部落在“弱”，示例至少 18 个落在“可用”以上，测试守着这两条。这只保证分档和公式自洽。低分是回头再看一眼的理由，高分什么也不保证。如果你用自己账号的数据校准了它，欢迎 PR。

**公式分类说的是字，不是结果。** `/bili-viral` 说某个标题用了“我亲自试了”，那是对你能看到的标题的判断，不是在解释它为什么火。

**弹幕分类按字面判。** “哈哈”算笑、“？”算疑惑。弹幕 XML 只有弹幕池里最新的一批，开头被“来了”“前排”抬高，结尾被“三连”抬高——报告会把这些偏差直接写出来，不替你藏。

**平台规则会变，阈值都能改。** 标题 80 字上限、手机首页约 24 字折叠、封面 4:3、分段章节的 5 分钟 / 2–10 段 / 5 秒 / 16 字，都是 2026 年 11 月核对过的，写在代码注释里，并且都有参数可以覆盖。“手机首页约 24 字折叠”“开头 30 秒流失 25% 以内算健康”是经验值，不是官方数字，输出里也是这么写的。

**标签是归类工具，不是流量开关。** 最多 10 个，用来消歧。这个包会直说，而不是卖给你一个标签生成器。

**什么都不编数字。** skill 想要一个它没有的数字时，会问你要，或者不用数字写。

**样例数据是合成的。** `testdata/` 里的字幕、弹幕、留存和视频数据都是为测试生成的，UP 主名字是“示例UP·甲”这样的占位符，数字不代表任何真实账号。

## 和 YouTube 版有什么不同

| | youtube-agent-skill | bilibili-agent-skill |
| --- | --- | --- |
| 工具 | 6 个 Python 脚本 | 1 个 Go 二进制、7 个子命令，标准库零依赖，带测试 |
| 钩子公式 | 21 个英文公式 | 21 个中文公式，正则按中文写；加入“从零手搓”这类 B站本土结构，“全网最”改成“限定范围的最” |
| 钩子打分 | 5 项，对照 74 个真实开头 | 5 项按中文重做（字数按音节算），加 B站红线；未校准 |
| 标题 | 桌面 60 / 手机 40 字符 | 80 字硬上限、手机双列约 24 字宽折叠、前 12 字、【】前缀、绝对化用语、标点刷屏 |
| 封面 | 16:9，最多 3 个词 | 4:3 上传、16:9 安全区、两行约 12 字；用中文二元组查和标题的重复 |
| 章节 | 描述里的时间戳规则 | 分段章节规则（过审、>5 分钟、2–10 段、≥5 秒、<16 字），不满足时给评论区时间戳版 |
| 留存 | Studio CSV 导出 | 创作中心曲线（可截图估读），相邻断崖合并，能和弹幕交叉看 |
| 弹幕 | — | 新增 `/bili-danmaku`：空降区间、疑惑、高峰、纠错、沉默区，按留存归一化 |
| 爆款 | 按自己频道中位数的倍数 | 同上，再加收藏 / 投币 / 讨论 / 点赞率相对自己中位数的倍数，判断赢在哪 |
| 短视频 | `/yt-shorts` | `/bili-clip` 竖屏切片，只切自己的或有授权的 |

## 开发

```bash
go vet ./...
go test ./...
go build -o bili ./cmd/bili
```

钩子公式只有一份：`skills/bili-script/hooks.json`。skill 读它，二进制通过 `go:embed` 把它编译进去，所以两边永远一致。`internal/formula` 的测试要求每个公式自己的示例必须被归到它自己名下。

```
cmd/bili/            命令行入口和各子命令的输出
internal/textx/      中文文本：字宽、音节数、二元组、数字
internal/transcript/ SRT / VTT / whisper / BCC 字幕解析
internal/formula/    21 个公式的加载与归类
internal/hook/       钩子打分
internal/title/      标题 + 封面联检
internal/cut/        粗剪决策表
internal/chapters/   分段章节草稿与规则校验
internal/retention/  留存曲线分析
internal/danmaku/    弹幕解析与五信号
internal/viral/      爆款倍数与互动类型
skills/              十二个 skill
testdata/            合成样例，expected.json 记录了每个信号埋在哪
```

## 许可

MIT。拿去用、改、发。结构和方法来自 Jake Schincariol 的 youtube-agent-skill（MIT），见 [NOTICE.md](NOTICE.md)。
