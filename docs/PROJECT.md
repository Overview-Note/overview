# Overview — 项目介绍

> 一个可自部署、以文件为真相、AI 原生的层级化 Markdown 知识库。

## 一句话

**像 memos 一样好部署，像文件一样好迁移，像现代工具一样可被 AI 操作。**

Overview 取了 [memos](https://github.com/usememos/memos) 的轻量自部署体验，补上它缺失的
**层级目录**，并把内容还原为**纯 Markdown 文件**。它以一个内嵌前端的 Go 单二进制发布，
用 SQLite FTS5 做检索，同时通过 REST、WebDAV 与 MCP 三种方式对外开放。

---

## 它解决什么问题

| 痛点 | Overview 的答案 |
| --- | --- |
| memos 等工具不支持目录层级，内容一多就乱 | 磁盘文件夹即导航树，无限层级 |
| 内容锁在数据库里，难迁移、难版本管理 | 每篇是带 frontmatter 的 Markdown，`data/` 可被 Git / 任意编辑器直接使用 |
| 中文搜索只能整词、无法子串匹配 | 自研 CJK 单字 + bigram 分词，`发模` 命中 `并发模型` |
| 自部署工具往往「要么简单但封闭，要么强大但难运维」 | 单二进制、单进程、默认零外部依赖，同时提供 WebDAV/MCP/REST |
| AI 功能要么缺席、要么绑定单一厂商 | 内置 MCP 服务端 + AI 助手，可接任意 OpenAI 兼容后端，也可完全关闭 |

---

## 核心亮点

### 1. 文件即真相
每篇笔记 = 一个 Markdown 文件 + YAML frontmatter（`id` / `title` / `tags` / `public` …）。
数据库只是**可抛弃的索引**：删掉 `overview.db`，启动即全量重建。这带来 Git 版本管理、
跨工具互操作、零锁定。

### 2. 层级而非扁平
`data/notes/技术/Go/并发模型.md` 就是一个三级目录树。重命名不破坏稳定 `id`，为双链提供基础。

### 3. 中文子串检索
FTS5 默认分词把中文当整词，`trigram` 又要 ≥3 字。Overview 在入库前把 CJK 展开为
**单字 + 相邻双字 bigram**，Latin 词保持原样，再由 `unicode61` 索引——无需外部搜索引擎，
也能做中文中缀匹配。

### 4. 分层 + 依赖倒置的架构
```
core (领域模型 + 端口接口)
  ↑ 被依赖
service (业务用例：笔记/搜索/认证/AI …)
  ↑ 实现/消费
store · index · s3store · watcher · mcp · server
```
`service` 只依赖 `core` 中的接口，后端可整体替换（S3 附件已是示例）。同一套 `service`
驱动 REST、WebDAV、MCP 三个适配器。

### 5. 写入安全与实时
- 原子写（temp → fsync → rename），杜绝半截文件
- 乐观并发：版本 = 文件内容 SHA-256，`If-Match` 冲突返回 409
- 增量索引（`ReplacePrefix`）+ fsnotify 文件监视器，外部编辑即时可搜

### 6. AI 原生
- **MCP 服务端**：`POST /mcp`（JSON-RPC 2.0，协议 `2026-07-28`，向下兼容旧握手），暴露 22 个工具
  （笔记/目录/历史/回收站/附件/索引/公开笔记），与 CLI、REST 能力拉平
- **AI 助手**：对话 / 整理（替换）/ 补全（追加），支持 OpenAI、DeepSeek、Ollama、vLLM 等
- **AI 智能体（工具调用）**：内置助手可**执行应用内操作**——用 OpenAI 兼容 function calling 读取、
  搜索、写入、移动、删除笔记与附件。能力源统一在 `internal/tools`（MCP 与智能体共用）；
  循环有界（默认 8 步）、**危险操作必须预览后人工确认**、按角色裁剪工具、提示注入防护、
  工具调用审计（`ai_tool_audit`，参数脱敏）
- 管理员可在**设置**中运行时配置，无需重启

### 7. 多端接入与自动化
- **离线 CLI**：`overview <命令>` 直接操作数据目录，无需服务；覆盖读写/检索/历史/回收站/附件/归档，
  并提供 `build` 一键生成可部署静态站
- **WebDAV**：在 Obsidian / Finder / 移动端直接挂载读写
- **REST API**：`/api/v1`，OpenAPI 规范 + `/api/docs`
- **PWA**：可安装的应用壳

### 8. 现代而克制的界面
参考 Starlight 文档站的布局：全宽顶栏 + 居中搜索（`Ctrl/⌘ + K`）、左侧分组式层级导航、
内容区大标题 + 右侧「大纲」栏；全站字号派生自 `--base-size`，随设置整体缩放；
Gridea 风格暖色配色（琥珀主色 `#D4870E`，明暗两套），主题色可在设置中自定义；
设置从弹窗升级为**独立路由页**（外观 / 编辑器 / AI / 邮件 / 站点 / 数据 / 用户 / 令牌）。

---

## 功能清单（当前版本）

**内容与组织**：层级目录 · Markdown 存储 · 双链与反链 · 每篇私有/公开 + 独立公开页 · 拖拽移动
**编辑**：富文本（Tiptap）· 代码高亮 · 任务列表 · 数学公式（KaTeX）· Mermaid 图表 · GFM 脚注 · 图片粘贴/拖拽 + 压缩 · **文件附件（任意类型，下载链接）** · 表格 + 就近浮动工具条 · 斜杠命令 · 大纲 · 专注模式 + 快捷键面板
**检索**：CJK 全文检索 · 全局搜索
**AI**：AI 助手（对话/整理/补全）· **AI 智能体（工具调用，可执行应用内操作 + 危险操作确认）** · MCP 服务端（工具与智能体同源于 `internal/tools`）
**账户**：多用户认证（bcrypt/会话/角色）· **邮箱邀请/验证/密码重置** · **可开关自注册** · 持久 API/MCP 令牌 · 登录页内容注入（提示语/备案号/链接）
**访问**：WebDAV · REST + OpenAPI · PWA · 应用内快速捕获（顶栏弹窗 · 服务端抓取标题/正文 · SSRF 防护）
**移动端**：**响应式手机端**（抽屉式侧栏 · 顶栏手机化 · 编辑器全宽 · 工具栏横向滚动 · 右侧面板底部抽屉 · 搜索浮层 · 触屏基础 · 可安装 PWA）
**外观**：Gridea 风格配色（琥珀主色 `#D4870E`）· 明暗主题 · **主题色自定义（预设 + 取色器）** · **独立设置页**（外观/编辑器/AI/邮件/站点/对象存储/数据/用户/令牌）
**运维**：单二进制 · Docker · 结构化日志（文件 + 轮转）· SQLite 迁移 · 原子写 + 乐观并发 · 版本历史 · 回收站 · 增量索引 + 文件监视 · ZIP 导入/导出 · **S3 附件后端（设置页可运行时切换）** · 静态站导出 + sitemap/robots · 多语言（中/英/繁中/日/德）

---

## 技术栈

| 层 | 选型 |
| --- | --- |
| 后端 | Go 1.26，标准库 `net/http`（方法路由）+ 手写中间件 |
| 存储 | Markdown 文件 + YAML frontmatter；SQLite（纯 Go `modernc.org/sqlite`） |
| 检索 | SQLite FTS5 + 自研 CJK 分词 |
| 前端 | Vue 3 + TypeScript + Vite 6 + Pinia + vue-router |
| 编辑器 | Tiptap 2（ProseMirror） |
| AI/MCP | OpenAI 兼容客户端（无 SDK，含 function/tool calling）+ 自研 JSON-RPC MCP 服务端；`internal/tools` 统一能力源 + `internal/agent` 有界工具循环 |
| 集成 | WebDAV（`x/net/webdav`）、S3（`minio-go`）、SMTP（标准库 `net/smtp`）、fsnotify |
| 发布 | 前端 `go:embed` → 单二进制；Docker 三阶段构建 |

---

## 项目状态

- **版本**：v0.13.2（手机端适配 · 对象存储设置页 · AI 面板与用户管理修复；Phase 7 部分待排期）
- **测试**：`go test ./...` 覆盖 config/logging/history/archivex/sitegen/trash/ai/openapi/cli/agent/tools
  以及 store/index/textproc/service/server/mcp（含 `httptest` 集成测试与 MCP↔tools parity）；前端 `vue-tsc` 类型检查、
  **Vitest** 单元测试（`npm test`）与 `vite build`；`make test` 一键运行 Go + 前端
- **CI**：GitHub Actions（后端 race 测试、前端类型检查+测试+构建、golangci-lint）；
  推送 `v*` 标签自动构建 **多架构镜像** 发布到 GHCR
- **交付**：Docker / 单二进制（内嵌前端）/ GHCR 镜像，浏览器访问
- **规模**：后端 ~9k 行 Go / 20 个 internal 包；前端 ~5k 行 TS/Vue
- **已知限制**：见 [`DESIGN.md`](DESIGN.md) §11。v0.13.0 待权衡的是智能体会话/限流的
  单实例内存假设（确认需同实例）、审计无清理策略与工具调用为非流式；v0.13.2 待权衡的是运行时切换资产后端**不迁移历史附件**、移动端为响应式适配而非原生体验；仍待办的是标签/置顶/实时协作/评论等功能（Phase 7）与 i18n 语言扩充（暂缓）

---

## 配置与端点

配置全部通过环境变量（完整表见 [`DESIGN.md`](DESIGN.md) §13.1 与 [`README.md`](../README.md)）。
常用项：`OVERVIEW_ADDR`、`OVERVIEW_DATA_DIR`、`OVERVIEW_AUTH`、`OVERVIEW_MAX_UPLOAD_MB`、
`OVERVIEW_SITE_TITLE`、`OVERVIEW_SITE_THEME`、`OVERVIEW_RENDER`。v0.12 新增/相关：

- `OVERVIEW_BASE_URL`：生成邮件链接的外部基址（空则按请求推导）
- `OVERVIEW_MAIL_HOST` / `OVERVIEW_MAIL_PORT` / `OVERVIEW_MAIL_USERNAME` / `OVERVIEW_MAIL_PASSWORD` /
  `OVERVIEW_MAIL_FROM` / `OVERVIEW_MAIL_STARTTLS`：SMTP 默认值（也可在「设置 → 邮件服务器」运行时配置）
- `OVERVIEW_AI_BASE_URL` / `OVERVIEW_AI_API_KEY` / `OVERVIEW_AI_MODEL`：AI（也可运行时配置）
- `OVERVIEW_MCP_TOKEN`：MCP 静态令牌（推荐改用「设置 → API 令牌」生成）
- `OVERVIEW_S3_*`（`BUCKET`/`ENDPOINT`/`REGION`/`ACCESS_KEY`/`SECRET_KEY`/`USE_SSL`/`PUBLIC_URL`）：
  对象存储**初始值**；设置页「对象存储」保存后以其为准（运行时切换本地 ↔ S3，历史附件不迁移）

主要端点：REST 在 `/api/v1`（笔记/检索/历史/回收站/附件/公开笔记/AI/设置/认证），
OpenAPI 文档 `/api/docs`、规范 `/api/v1/openapi.json`，MCP `POST /mcp`，WebDAV `/dav/`。
v0.12 新增认证流程端点：`/auth/password/request`、`/auth/password/reset`、`/auth/accept-invite`、
`/auth/verify-email`、`/auth/register`、`/auth/verify/resend`，以及管理员
`/auth/users/invite`、`/auth/users/{id}/resend-invite`、`/auth/users/{id}/email`、
`/auth/users/{id}/send-verification`；设置 `/settings/{ai,mail,site}`；捕获 `/capture/preview`。
v0.13 新增智能体端点：`POST /ai/agent`（运行一回合）、`POST /ai/agent/confirm`（批准/拒绝危险操作）、
`POST /ai/agent/stop`（终止会话）；`GET /ai/status` 与 `GET/PUT /settings/ai` 扩展
`toolCalling`、`agentEnabled`、`confirmPolicy`、`maxSteps`、`allowedTools` 字段。
v0.13.2 新增对象存储设置端点：`GET/PUT /settings/storage`（读取/保存并切换后端，仅管理员，
`secretKey` 不回传）、`POST /settings/storage/test`（用表单当前值探测连接，不改动活动后端）。

---

## 快速开始

```bash
git clone https://github.com/Overview-Note/overview.git
cd overview
docker compose up -d --build
# 打开 http://localhost:5230，完成首次管理员设置
```

本地开发、构建、配置、集成方式详见 [`README.md`](../README.md)。

---

## 参与贡献

我们欢迎各种形式的贡献：缺陷报告、功能建议、文档改进与代码。

- 阅读 [`CONTRIBUTING.md`](../CONTRIBUTING.md) 了解开发环境与规范
- 寻找 `good first issue` 标签的入门任务
- 架构约定请参考 [`DESIGN.md`](DESIGN.md)（含架构决策记录 ADR）

**接下来的方向**（Phase 7，未排期）：标签系统（管理 UI / 过滤 / 补全）· 置顶与保存视图 ·
实时协作与评论 · 版本 Diff 与草稿持久化 · Webhook / OIDC / gRPC ·
移动端原生 App。详见 [`DESIGN.md`](DESIGN.md) §12。

---

## 许可

[MIT](../LICENSE) © Overview contributors
