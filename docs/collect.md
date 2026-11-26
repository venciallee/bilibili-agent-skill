# 数据从哪来

`bili` 的每个子命令都只读你本地的文件。它不登录、不调 B站接口、不做请求签名。这一页说清楚每种输入怎么拿到手——拿数据的方式由你选，工具不替你选。

为什么这么克制：2026 年 1 月，收集 B站接口文档的开源项目 bilibili-API-collect 收到 B站委托律所的律师函，指其系统性整理、传播非公开接口及其鉴权机制，项目随后关停（见该仓库 README）。所以这个包不封装任何 B站接口，也不在 skill 里教人绕风控。

## 字幕（`bili cut` / `bili chapters`，以及 `--transcript`）

支持 SRT、WebVTT、whisper JSON（`{"segments": [...]}`）和 B站 BCC 字幕 JSON（`{"body": [{"from", "to", "content"}]}`）。

- 剪映 / 必剪：识别字幕后导出 SRT。最省事。
- `whisper` / `faster-whisper`：对本地成片跑一遍，输出 SRT 或 JSON。
- 已发布视频的字幕轨：`yt-dlp --skip-download --write-subs --sub-langs all "https://www.bilibili.com/video/BV..."`。AI 字幕有时要求登录才拿得到，这时直接在剪辑软件里重新识别一份更省事。

## 弹幕（`bili danmaku`，以及 `--danmaku`）

```bash
yt-dlp --skip-download --write-subs --sub-langs danmaku "https://www.bilibili.com/video/BV..."
```

会得到一个 `.danmaku.xml`。多 P 视频在链接后加 `?p=N`，每个分 P 的弹幕是分开的。

注意这份 XML 只包含弹幕池里最新的一批，数量上限写在文件开头的 `<maxlimit>` 里。弹幕数顶到上限时，早期弹幕已经被挤掉了，`bili danmaku` 会在报告里提示。

## 留存曲线（`bili retention`，以及 `--retention`）

在创作中心的数据中心 / 稿件分析里，找到这个稿件的观众留存（播放完成率）曲线。入口和叫法会随版本变化，以你看到的为准。

如果没有导出按钮，就截图发给 Claude，让它逐点读出 15 到 20 个点，写成两列 CSV：

```csv
时间,留存
0:00,100
0:05,93
0:30,71
...
```

第一列可以是秒、`M:SS`，或者进度百分比（用百分比时加 `--duration` 告诉工具视频多长）。估读的曲线有一个采样间隔的误差，报告会提醒。

## 同领域的视频数据（`bili viral`）

```bash
yt-dlp -j --skip-download --playlist-end 30 "https://space.bilibili.com/<UID>/video" > collected.jsonl
```

逐行输出每个视频的标题、播放、点赞、评论数，`bili viral` 直接读这个格式。每个 UP 主至少要 4 个视频。

投币、收藏、弹幕数在视频页上公开可见，想知道一个爆款“赢在哪”（收藏型 / 投币型 / 讨论型），把它们补进去。手工整理成 JSON 也行：

```json
[{"up": "某UP主", "title": "...", "view": 412000, "like": 31000, "coin": 18000, "favorite": 26000, "danmaku": 5200, "reply": 2100, "bvid": "BV...", "pubdate": 1760000000}]
```

B站有风控。被拦（HTTP 412、错误码 -352）就停下来等一会儿，别换着法子硬抓，更别拿你的账号 cookie 去批量请求。
