# Overview 设计文档

> 版本：v0.13.0（AI 智能体 · 工具调用 · 危险操作两段式确认 · 工具审计）
> 更新日期：2026-10-05
> 定位：可自部署、支持层级目录、AI 原生、以文件为真相的 Markdown 知识库

---

## 0. 版本变更

| 版本 | 主题 | 关键变化 |
| --- | --- | --- |
| v0.1.0 | Phase 1 MVP | 文件存储 + SQLite 索引 + 基础 API + Tiptap 编辑器 |
| v0.2.0 | 架构加固 | 分层重构、原子写、乐观并发、迁移机制、中文检索、服务端快照、测试与 CI、前端 router/pinia |
| v0.3.0 | 双链 | `[[wiki-link]]` 解析与索引、反向链接面板、斜杠命令菜单、按文件名解析 |
| v0.4.0 | 多用户 | bcrypt 认证、会话、首启管理员、用户管理、WebDAV 挂载、可移植附件 |
| v0.5.0 | i18n | 中英文案抽取与语言切换 |
| v0.6.0 | 协作与智能 | 编辑器 TOC、公开分享（匿名只读）、MCP 服务端、AI 助手 |
| v0.7.0 | 规模化与运维 | 增量索引、文件监视器、版本历史、回收站、附件孤儿清理、S3 后端、PWA、OpenAPI |
| v0.8.0 | 设置与设计系统 | 设置中心（外观/编辑器/AI）、运行时 AI 配置、顶栏搜索、Starlight 风格布局与统一排版 |
| v0.9.0 | 可见性与开源 | 每篇私有/公开可见性选择器、独立公开页（globe）、开源文档与协作基建 |
| v0.10.0 | 对标 memos 补齐 | 代码高亮 / 任务列表 / KaTeX / Mermaid / 脚注、拖拽移动、ZIP 导入导出、应用内快速捕获、专注模式与快捷键面板、MCP 工具由 OpenAPI 生成、sitemap/robots、i18n 增繁中/日/德 |
| v0.10.2 | 已知问题修复 | render 模式白名单、历史保留策略、导出站搜索、WebDAV 增量重建、OpenAPI 补全、表格/wiki 往返修复 |
| v0.11.0 | CLI 化 · MCP 升级 · 性能与品牌 | 离线 CLI（笔记/检索/历史/回收站/附件/归档）、`build` 静态站生成、MCP 升级到 `2026-07-28`（无状态 `_meta` + `server/discover` + `resultType`）并补齐到 22 个工具、前端路由切分 + 静态资源 gzip/immutable 缓存（Lighthouse 99）、可拖拽侧栏与对比度回归、笔记风格新 Logo 与暖色柔和主题、GHCR 多架构镜像 |
| v0.11.1 | 图片性能与静态站主题 | 笔记图片 `loading=lazy`/`decoding=async` + 切换时取消过期请求与在飞图片（修复图片密集页切换卡顿）；静态站可选主题 `--theme auto\|light\|dark`（`OVERVIEW_SITE_THEME`）并与 App 设计令牌对齐配色 |
| v0.11.2 | API/MCP 令牌管理 | 管理员界面 + `/auth/tokens` API 生成/吊销持久令牌（仅存哈希，明文一次性）；令牌同时用于 REST 与 MCP，`OVERVIEW_MCP_TOKEN` 作为回退 |
| v0.11.3 | 令牌 UI 修正 | 令牌管理入口从顶栏移入「设置」（较少用）；修复弹框层级（DialogHost `z-index:300` 恒在最上）；令牌对话框加宽、生成后常驻可复制密钥框 |
| v0.12.0 | 配色 · 设置页 · 邮箱用户 · 附件 · 捕获 | 整体改为 Gridea 风格（琥珀主色 `#D4870E`，暖白/深色两套）+ 主题色自定义（预设 + 取色器，明暗自适应）；设置从弹窗改为独立路由页（外观/编辑器/AI 助手/邮件服务器/站点/数据/用户管理/API 令牌），删除旧 `SettingsDialog/TokensDialog/UsersDialog`；邮箱用户管理（邀请/邮箱验证/密码重置）、可开关自注册、登录页内容注入；文件附件（任意类型，下载链接 + RFC5987）；应用内快速捕获（顶栏弹窗 + SSRF 防护）；静态站样式与应用调色板同步 |
| v0.13.0 | AI 智能体（工具调用） | 内置助手从纯文本聊天升级为**可执行应用内操作的智能体**：抽取 `internal/tools` 为唯一能力源（原 `internal/mcp/generate.go`+`tools.go` 的 `toolBindings`/`runTool`/schema 迁入，MCP 变薄适配器），新增 `Defs()/DefsOpenAI()/Exec()/Risk()/Allows()/Preview()`；`internal/ai` 支持 OpenAI 兼容 function/tool calling（`ChatTools`/`AgentMessage`/`ToolCall`/`ErrToolsUnsupported` + 能力探测）；新增 `internal/agent` 有界循环（默认 8 步）、进程内会话 registry（TTL/容量）、危险操作「预览→确认→执行」（`/ai/agent/confirm`）、`Stop` 与审计；迁移 `0009_ai_tool_audit` + `internal/index/audit.go`（args 脱敏）；HTTP 新增 `/ai/agent`、`/ai/agent/confirm`、`/ai/agent/stop`，`/ai/status` 与 `/settings/ai` 扩展 agent 字段；前端新增 `AgentPanel/ToolCallCard/ConfirmBar` + `stores/agent.ts` + 编辑器「助手 / 智能体」分段；安全模型含按角色裁剪工具、危险必确认、防提示注入、按用户限流与能力探测降级 |

v0.2.0 的目标不是加功能，而是**建立可持续演进的地基**，避免后续加双链/多用户/WebDAV 时返工。

---

## 1. 背景与目标

### 1.1 背景

memos 等轻量工具部署简单但不支持层级目录，内容锁定数据库。Overview 旨在提供**支持无限层级、以纯 Markdown 存盘、可 Docker 自部署**的知识库。

### 1.2 目标

| 维度 | 目标 |
| --- | --- |
| 部署 | Docker 自部署，单容器、单二进制 |
| 访问 | 网页端 + REST API（移动端/第三方） |
| 组织 | 无限层级目录 + 文件夹管理 |
| 存储 | 纯 Markdown 存盘，Git 友好、可迁移 |
| 编辑 | 快捷插入图片、表格；所见即所得 |
| 视觉 | 参考 gridea docs 的干净文档风格 |

---

## 2. 架构总览

### 2.1 分层

```
┌────────────────────────────────────────────────────────────┐
│ 客户端：浏览器 (Vue3 SPA) │ 移动端/第三方 (REST)             │
└───────────────┬────────────────────────────────────────────┘
                │ HTTP
┌───────────────▼────────────────────────────────────────────┐
│ server (HTTP 适配层)                                        │
│   路由 /api/v1 · 中间件(request-id/recovery/log) · ETag      │
├────────────────────────────────────────────────────────────┤
│ agent (AI 智能体)  ← 有界工具循环 · 会话 registry · 两段式确认 · 审计 │
├────────────────────────────────────────────────────────────┤
│ tools (能力源)  ← OpenAPI 生成 schema · Defs/Exec/Risk/Allows/Preview │
│   MCP 与 agent 共用同一工具面（MCP 仅为薄 JSON-RPC 适配器）    │
├────────────────────────────────────────────────────────────┤
│ service (应用服务层)  ← 用例、事务边界、Tree 组装、认证/邮件/站点/捕获 │
├────────────────────────────────────────────────────────────┤
│ core (领域层)                                              │
│   模型 Note/TreeNode · 端口接口 · 哨兵错误                   │
├──────────────┬───────────────┬─────────────────────────────┤
│ store        │ index         │ textproc                    │
│ (文件系统)    │ (SQLite+FTS5) │ (CJK 分词/快照)              │
└──────────────┴───────────────┴─────────────────────────────┘

数据流（一次智能体回合）：
Browser → POST /api/v1/ai/agent → server/agent.go → internal/agent.Run
  → internal/tools.Defs()（按角色裁剪）→ internal/ai.ChatTools
  → 若模型请求工具：internal/tools.Exec / Preview → service（落盘 + 索引）
  → 危险操作暂停返回 needs_confirmation → /ai/agent/confirm → 续跑
  → 每步写 internal/index 的 ai_tool_audit（args 脱敏）
                │
        data/ (notes/, assets/, overview.db)
```

### 2.2 设计原则

1. **文件即真相**：内容以 Markdown 文件为准，数据库仅是可重建的索引。
2. **索引可抛弃**：删除 `overview.db` 后启动会自动全量重建。
3. **依赖倒置**：`service` 只依赖 `core` 定义的接口，`store`/`index` 是接口实现，未来可替换后端（Postgres/S3）。
4. **适配层薄**：HTTP handler 只做协议转换，业务逻辑在 `service`，便于 WebDAV/CLI 复用。
5. **写入安全**：所有落盘为原子写；并发编辑用版本检测避免覆盖。

---

## 3. 架构决策记录（ADR）

| 编号 | 决策 | 理由 | 影响 |
| --- | --- | --- | --- |
| ADR-001 | 后端维持 Go 单二进制 + 前端 embed | 部署最简，运维成本低 | 无外部运行时依赖 |
| ADR-002 | SQLite 使用纯 Go 驱动 `modernc.org/sqlite` | 免 CGO，交叉编译与 Docker 简洁 | 写入性能约为 cgo 版 2–3×，已用接口隔离 |
| ADR-003 | 内容存 Markdown 文件，索引存 SQLite | 可迁移、Git 友好 | 需处理索引与文件的最终一致性 |
| ADR-004 | 引入 `core`(端口) + `service` 分层 | 避免业务逻辑散落 handler，支持多适配器 | 初期样板代码增多 |
| ADR-005 | 中文检索自研 CJK 单字+bigram 分词 | FTS5 `trigram` 对 2 字中文查询失效；`unicode61` 无法子串匹配 | 索引增大；换来无依赖的中文子串检索 |
| ADR-006 | 快照（snippet）由服务端生成并 HTML 转义 | FTS 存储的是分词后文本，无法直接出可读快照；同时修掉 XSS | 前端可安全 `v-html` |
| ADR-007 | 乐观并发：版本 = 文件内容 SHA-256 | 简单、确定、跨进程有效 | 冲突需前端处理（提示 + 不覆盖） |
| ADR-008 | API 版本化 `/api/v1` | 为未来破坏性变更留出空间 | 前端同步更新 |
| ADR-009 | 目录树由「文件系统结构 + 索引标题」合成 | 避免读取每个文件正文（旧实现的性能瓶颈），同时保留空文件夹 | 依赖索引；索引可重建 |
| ADR-010 | 引入 vue-router + Pinia | 支持深链接/刷新保持、集中状态 | 前端结构规范化 |
| ADR-011 | 认证默认 `multi`，保留 `none` 开关 | 公网默认安全；单机可通过环境变量关闭 | 无用户时强制首启向导 |
| ADR-012 | 会话用 HttpOnly Cookie，另支持 `Bearer` | 浏览器安全（防 JS 读取），同时便于 API 客户端 | 前后端同源最简 |
| ADR-013 | WebDAV 由 x/net/webdav 提供，Basic 认证 + 写后去抖重索引 | 复用认证与文件存储，外部编辑可检索 | WebDAV 与 SPA 并发编辑靠版本/时间区分 |
| ADR-014 | 附件在 Markdown 中存相对路径 `assets/<...>` | 整个 `data/` 可被其它 Markdown 工具直接打开 | 渲染时重写为 `/assets/` |
| ADR-015 | 自研轻量 i18n（无 vue-i18n 依赖） | 体积/复杂度可控，响应式 + 持久化 | 文案集中管理 |
| ADR-016 | 公开分享基于 frontmatter `public` 标记 | 与文件即真相一致，随笔记文件迁移 | 公开页面不泄漏私有笔记与 wiki 目标 |
| ADR-017 | MCP 用 JSON-RPC over HTTP（POST /mcp），Bearer 认证 | 复用 HTTP 栈，无需额外 stdio 进程管理 | 与 REST 并存，工具复用 service |
| ADR-018 | AI 走 OpenAI 兼容 `/chat/completions`，无 SDK 依赖 | 兼容 OpenAI/DeepSeek/Ollama/vLLM 等 | 未配置时功能整体禁用 |
| ADR-019 | 索引增量更新（`ReplacePrefix`），文件监视器驱动 | 避免大库全量重建；外部编辑自动入库 | 复杂前缀删除需 LIKE 转义 |
| ADR-020 | 版本历史与回收站均落盘（`.history` / `.trash`） | 坚持"文件即真相"，可随数据目录迁移 | 需定期清理；隐藏目录被遍历忽略 |
| ADR-021 | 附件后端抽象为 `AssetStore`，S3 为可选实现 | 本地优先，按需切换到对象存储 | 切换后旧本地附件需迁移 |
| ADR-022 | PWA 仅缓存应用壳与构建产物 | 离线可启动，同时避免缓存隐私数据 | API/上传/DAV 永不缓存 |
| ADR-023 | OpenAPI 由 YAML 内嵌并在启动时转 JSON | 单一可读来源，工具可直接消费 | 手写维护 |
| ADR-024 | 设置中心集中管理偏好（主题/字号/语言/压缩/AI） | 避免功能入口散落在工具栏，降低认知负担 | 偏好持久化到 localStorage；AI 配置持久化到 DB |
| ADR-025 | AI 配置支持运行时覆盖（settings 表 + 环境变量默认） | 管理员可在界面改 BaseURL/Key/Model 并即时生效 | 密钥仅在服务端保存，接口永不回传 |
| ADR-026 | UI 采用派生自 `--base-size` 的字号阶梯 | 编辑器与周边 UI 字号统一，随字号设置整体缩放 | 新增 `--text-xs/sm/ui/body` 令牌 |
| ADR-027 | 编辑操作与非编辑操作分离（格式化工具栏 vs 笔记栏） | 语义清晰：工具栏只做排版，页面级动作独立 | 笔记栏承载大纲/历史/AI/分享 |
| ADR-028 | 新增离线 CLI（`internal/cli`），复用 `service` 层 | 让 Overview 可被脚本/编辑器/工具调用，无需启动服务 | CLI 与 REST/WebDAV/MCP 共享同一业务层 |
| ADR-029 | `overview export` 合并为 `overview build` 的别名 | 消除两套静态站导出实现 | 旧命令与 Makefile 目标保持可用 |
| ADR-030 | MCP 升级到 `2026-07-28`，实现为 dual-era 服务端 | 兼容最新无状态协议，同时不破坏旧握手客户端 | 每请求 `_meta` 版本协商、`server/discover`、`resultType` |
| ADR-031 | MCP 工具面拉平到 CLI/REST（22 个工具） | 让 AI 能执行与人类同等的操作 | 工具 schema 由 OpenAPI 生成，个别手写覆盖 |
| ADR-032 | 前端路由全部懒加载并按需注入编辑器重依赖 | 首屏不再加载 TipTap/KaTeX/lowlight/mermaid | 初始 JS 从 ~1.1MB 降至 ~154KB |
| ADR-033 | 静态资源在 Go 侧 gzip 压缩 + 哈希资源 immutable 缓存 | Lighthouse 从 80+ 提升到 99 | 前端资源不再走 `http.FileServer` 裸服务 |
| ADR-034 | 品牌改为笔记/文档图形 + 暖色柔和调色板 | 定位是笔记软件；降低刺眼对比但保留层级 | 设计令牌整体调整，符号色改为变量 |
| ADR-035 | 新增持久 API/MCP 令牌（管理界面 + `/auth/tokens` API，仅存 SHA-256 哈希） | 环境变量无法在界面管理，会话令牌无 UI 且短期 | 令牌与登录会话共用 Bearer 认证，`OVERVIEW_MCP_TOKEN` 作为回退 |
| ADR-036 | 整体配色改为 Gridea 风格（琥珀主色 `#D4870E`，暖白/深色两套） | 文档化知识库需要克制、偏纸张感的视觉，同时保留清晰层级 | 设计令牌整体替换；静态站调色板同步 |
| ADR-037 | 主题色可在设置中自定义（预设 + 取色器，单值 hex 派生 `--accent/-hover/-soft/-ring`，明暗自适应） | 单一固定主色无法满足品牌/偏好；直接改 4 个变量易出现对比度问题 | 由前端根据明暗模式计算色阶；持久化到 `localStorage` |
| ADR-038 | 设置从弹窗改为独立路由页 `/settings`（左侧分类导航 + 内容面板） | 设置项增多后弹窗难以承载，深链接/刷新保持也需要路由 | 删除 `SettingsDialog/TokensDialog/UsersDialog`，改为 `settings/*` 子路由；仅管理员可见管理分区 |
| ADR-039 | 邮箱用户管理（邀请/验证/密码重置/自注册）与 SMTP 运行时配置 | 需要一个不依赖外部 IdP 的多用户自助流程；SMTP 也应像 AI 一样可运行时配置 | 新增 `user_tokens` 表（sha256 哈希、一次性、可过期）；SMTP 存 `settings` 表；邮件未配置时优雅降级 |
| ADR-040 | 公开认证端点限流 + 防枚举 | 注册/找回密码是匿名可写的，易被滥用或用于探测账号 | 进程内固定窗口限流（按 IP 与邮箱），未知账号一律静默成功 |
| ADR-041 | 附件支持任意文件类型，Markdown 保存相对路径 | 仅图片不够用，用户需要上传 PDF/ZIP 等并以下载链接引用 | 非内联类型强制 `Content-Disposition: attachment`（含 RFC5987 文件名）；前端与静态站重写 `href` |
| ADR-042 | 应用内快速捕获走服务端抓取，内置 SSRF 防护 | 浏览器受 CORS 限制无法跨域抓取，服务端抓取又可能被用于探测内网 | 连接时校验解析后的 IP、限制重定向与响应体、仅 http/https |
| ADR-043 | 匿名只读放行 `GET/HEAD /assets/` | 公开笔记的图片/附件托管在 `/assets/`，匿名读者没有会话 | 仅放行读取；上传/维护/`/api/v1/assets` 仍需认证；文件名带随机 ULID 前缀，不可枚举 |
| ADR-044 | 抽取 `internal/tools` 作为唯一能力源（原 `internal/mcp/generate.go`+`tools.go` 迁入） | 智能体与 MCP 需要同一套能力面，重复实现会漂移 | MCP 变为薄 JSON-RPC 适配器；工具 schema/风险/权限单点维护；新增 `Defs()/DefsOpenAI()/Exec()/Risk()/Allows()/Preview()` |
| ADR-045 | `internal/ai` 增加 OpenAI 兼容 function/tool calling（`ChatTools`/`AgentMessage`/`Tool`/`ToolCall`） | 智能体需模型返回结构化工具调用；保持无 SDK 依赖 | 复用同一 `/chat/completions`；`tool_choice:"auto"`；不支持时返回 `ErrToolsUnsupported` |
| ADR-046 | 新增 `internal/agent` 有界循环（默认 8 步，上限 32） | 无界的模型-工具往返可能失控或烧 token | 步数用尽返回 `max_steps`；超出上下文预算时裁剪最旧回合（保留 tool_calls→tool 配对） |
| ADR-047 | 危险操作「预览→确认→执行」两段式（`/ai/agent/confirm`） | 删除/清空等不可逆操作不能由模型单方面触发 | 命中确认策略时暂停并返回 `preview`；用户批准可覆盖 args，拒绝则回填 `rejected` 并续跑 |
| ADR-048 | 工具调用审计（迁移 `0009_ai_tool_audit`，args 脱敏，可撤销操作记录版本/回收站/修订 id） | 智能体执行了写操作，需要事后追责与撤销依据 | 审计只存脱敏摘要（密钥 `[redacted]`、正文/上传存 sha256+长度），无明文正文 |
| ADR-049 | 工具调用能力探测与降级（`tool_calling_unsupported`） | 部分 OpenAI 兼容后端不支持 tools，硬失败体验差 | 首次被 4xx 拒绝即缓存「不支持」，`/ai/status` 暴露 `toolCalling`，前端禁用智能体 |
| ADR-050 | 提示注入防护：笔记正文/检索/工具输出一律视为不可信数据 | 笔记内容可能包含针对模型的恶意指令 | `SystemPrompt` 明确「数据非指令」；工具集按角色裁剪；危险操作强制人工确认 |

---

## 4. 数据模型

### 4.1 磁盘布局

```
data/
├── notes/                         # 目录树 = 物理文件夹
│   ├── 欢迎.md
│   └── 技术/Go/并发模型.md
├── assets/2026/10/<ulid>-<name>   # 附件（图片与任意文件）
└── overview.db                    # SQLite 索引（可删除重建）
```

### 4.2 笔记文件格式

```markdown
---
id: 01M3W66GM72J1CYWE3NYR33CDE
title: 欢迎使用 Overview
created: "2026-10-01T16:54:09Z"
updated: "2026-10-01T16:54:09Z"
---

正文……
```

- `id`：ULID，创建时生成，重命名保持不变（供未来双链使用）
- API 返回的 `updated` 取文件 `ModTime`；`size` 取文件大小
- 标题策略：frontmatter `title` 一旦存在即保留，不随正文 H1 变化（见 §12 限制）

### 4.3 SQLite Schema 与迁移

- 迁移文件位于 `internal/index/migrations/*.sql`，通过 `go:embed` 内嵌
- 迁移记录表 `schema_migrations(version, applied_at)`，启动时按序应用未执行项
- 迁移清单（0001–0009）：

| 版本 | 内容 |
| --- | --- |
| `0001_init` | `notes`（元数据）+ `notes_fts`（FTS5，unicode61，存**分词后**文本） |
| `0002_links` | `links(source_id, source_path, target_raw, target_key)` 双链 |
| `0003_notes_name` | `notes.name`（按文件名解析 wiki 链接） |
| `0004_users` | `users`（bcrypt 哈希 + 角色）、`sessions` |
| `0005_public` | `notes.public`（公开可见性） |
| `0006_settings` | `settings(key, value)`（运行时 AI / 邮件 / 站点配置） |
| `0007_api_tokens` | `api_tokens`（仅存 sha256 哈希 + 短前缀，明文一次性） |
| `0008_email_users` | `users` 增加 `email/status/email_verified`；新增 `user_tokens`（一次性、带用途、可过期） |
| `0009_ai_tool_audit` | `ai_tool_audit`（智能体工具调用审计：run/user/role/step/tool/args/result_summary/status/destructive + note_version/trash_id/revision_id） |

- `users`（迁移 0004 + 0008）：`id`、`username`（唯一）、`password_hash`、`role`（`admin`/`member`）、`created`、`updated`、`email`（唯一，非空时）、`status`（`active`/`invited`）、`email_verified`。受邀账号 `password_hash` 为空串（bcrypt 永不匹配），必须先接受邀请设置密码。
- `user_tokens`（迁移 0008）：`id`、`user_id`、`purpose`（`invite`/`reset`/`verify`）、`token_hash`（sha256，唯一）、`created`、`expires`、`used`。令牌单次消费：读取-校验-写入在同一事务内完成（连接池限单连接以串行化）。
- `settings`（迁移 0006）：`key`/`value` 字符串键值表，存 `ai_*`、`mail_*`、`registration_enabled`、`login_*` 等运行时配置。
- `api_tokens`（迁移 0007）：`id`、`name`、`prefix`、`token_hash`、`created`、`last_used`、`expires`。
- `sessions`（迁移 0004）：`token`、`user_id`、`created`、`expires`。
- `ai_tool_audit`（迁移 0009）：`id`、`run_id`、`user_id`、`username`、`role`、`step`、`tool`、`args`（脱敏后 JSON）、`result_summary`、`status`（`ok`/`error`/`denied`/`rejected`）、`destructive`、`note_version`、`trash_id`、`revision_id`、`created_at`；按 `run_id` 与 `(user_id, created_at)` 建索引。写入前由 `sanitizeAuditArgs` 脱敏：`key/token/password/secret/authorization` → `[redacted]`，`body/content` → `sha256:<hash> (len N)`，截断至 2 KB。

> 从旧版（v0.1，无迁移表且 `notes` 缺列）升级：索引可直接删除重建，内容文件不受影响。

---

## 5. 后端分层

### 5.1 包职责

| 包 | 职责 | 关键类型/方法 |
| --- | --- | --- |
| `internal/core` | 领域模型、端口接口、哨兵错误 | `Note` `TreeNode` `User` `UserToken` `APIToken` `NoteRepository` `Index` `UserStore` `AssetStore` `TokenStore` `ErrNotFound/ErrConflict/ErrInvalid/ErrForbidden` |
| `internal/service` | 应用用例编排 | `Tree` `GetNote` `SaveNote` `Delete` `Move` `Mkdir` `Search` `Upload` `Reindex`、`AuthService` `MailService` `SiteService` `TokenService`、`FetchPreview`（捕获） |
| `internal/store` | 文件系统实现 | 原子写、版本校验、`List`(结构)、`Walk`(全量) |
| `internal/index` | SQLite 实现 | 迁移、`Upsert` `Sync` `Tree` `Search` |
| `internal/textproc` | 文本处理 | `Tokens` `Segment` `Snippet` |
| `internal/server` | HTTP 适配 | 路由、中间件、错误映射、ETag、`agent.go`（`/ai/agent*`） |
| `internal/markdown` | frontmatter 解析/序列化 | `Parse` `Document.String` |
| `internal/tools` | **唯一能力源**（原 MCP 工具实现迁入） | `Defs` `DefsOpenAI` `Exec` `Risk` `Allows` `Preview` `Known`、`Set` |
| `internal/agent` | AI 智能体：有界工具循环、会话、确认、审计 | `Agent` `Run` `Confirm` `Stop` `SystemPrompt`、`Session`/`Result`/`Step` |
| `internal/ai` | OpenAI 兼容客户端（chat + tool calling） | `Chat` `ChatTools` `AgentMessage` `Tool` `ToolCall` `ErrToolsUnsupported` |
| `internal/mcp` | MCP JSON-RPC 适配器（薄） | `Server.ServeHTTP`（工具面委托 `internal/tools`） |

### 5.2 关键读写路径

**保存笔记**
```
PUT /api/v1/note
  → server 解析 baseVersion / If-Match
  → service.SaveNote
      → store.Write   (版本校验 + 原子写 temp→fsync→rename)
      → index.Upsert  (元数据 + 分词 FTS)
  → 返回 Note（含新 version），并设置 ETag
```

**获取目录树**
```
GET /api/v1/tree
  → service.Tree
      → store.List   (仅 ReadDir，不读文件内容)
      → index.Tree   (一次查询取全部标题/ID)
      → 组装嵌套树（文件夹优先排序）
```

**全文检索**
```
GET /api/v1/search?q=
  → index.Search
      → textproc.Tokens(q) → FTS5 MATCH
      → 命中 id 列表
      → 从 notes 表取原文 → textproc.Snippet 生成并转义快照
```

**智能体回合（工具调用）**
```
POST /api/v1/ai/agent {runId?, input, context{path,selection}}
  → server 鉴权/限流 + 配置门禁（AI 已配置 + agentEnabled + 支持 tool calling）
  → agent.Session（无 runId 则 Start，登记进程内 registry）
  → agent.Run → 追加 system 提示 + user 输入（含笔记/选区上下文）
      → loop（≤ maxSteps）:
          tools.Defs() → 按角色 + allowedTools 过滤 → ai.ChatTools
          无 tool_calls → done；有则逐条：
              工具不允许 → denied；需确认 → 暂停返回 needs_confirmation
              否则 tools.Exec → service 落盘/索引 → 记录 step + 审计
  → 危险操作: /ai/agent/confirm {runId, toolCallId, decision, args?} 续跑
  → /ai/agent/stop 丢弃会话
```

### 5.3 版本与并发（ADR-007）

| 客户端 `baseVersion` / `If-Match` | 语义 |
| --- | --- |
| `"*"` | 仅新建（已存在则 409） |
| 具体版本串 | 必须与当前一致（否则 409） |
| 省略 | 无条件写（内部/运维用途） |

`version = sha256(文件原始字节)`。GET 返回 `ETag: "<version>"`。

---

## 6. REST API（v1）

Base：`/api/v1`

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/health` | 状态 + 版本 |
| GET | `/tree` | 目录树 |
| GET | `/note?path=` | 读取笔记（带 ETag） |
| PUT | `/note` | 保存，body `{path, body, baseVersion}`，或 `If-Match` |
| DELETE | `/note?path=` | 删除笔记/文件夹 |
| POST | `/folder` | 新建文件夹 `{path}` |
| POST | `/rename` | 重命名 `{from, to}` |
| GET | `/search?q=&limit=&offset=` | 全文检索 |
| POST | `/assets` | 上传附件（multipart `file`，任意类型） |
| GET | `/assets/{path...}` | 访问附件（nosniff，非图片强制下载） |
| POST | `/capture/preview` | 抓取网页并返回 `{title, text, url}`（SSRF 防护） |
| POST | `/reindex` | 重建索引 |
| GET | `/links?path=` | 出链与反链 |
| GET | `/resolve?target=` | 解析 wiki 链接目标 |
| GET | `/auth/state` | 认证模式 / 是否需要初始化 / 当前用户 / 站点与邮件状态 |
| POST | `/auth/setup` | 创建首个管理员 |
| POST | `/auth/login` | 登录（未验证邮箱返回 403 `email_not_verified`） |
| POST | `/auth/logout` | 登出 |
| GET | `/auth/me` | 当前用户 |
| POST | `/auth/password` | 修改本人密码 |
| GET | `/auth/users` | 用户列表（仅管理员） |
| POST | `/auth/users` | 创建用户（仅管理员） |
| DELETE | `/auth/users/{id}` | 删除用户（仅管理员） |
| POST | `/auth/users/invite` | 邀请用户（仅管理员，发邀请邮件） |
| POST | `/auth/users/{id}/resend-invite` | 重发邀请（仅管理员） |
| PUT | `/auth/users/{id}/email` | 设置/更换邮箱，返回 `verificationSent`（仅管理员） |
| POST | `/auth/users/{id}/send-verification` | 重发验证邮件（仅管理员） |
| GET | `/auth/tokens` | API/MCP 令牌列表（仅管理员） |
| POST | `/auth/tokens` | 生成令牌，明文仅返回一次（仅管理员） |
| DELETE | `/auth/tokens/{id}` | 吊销令牌（仅管理员） |
| POST | `/auth/password/request` | 请求密码重置（匿名，限流，防枚举） |
| POST | `/auth/password/reset` | 用一次性令牌重置密码（匿名，限流） |
| POST | `/auth/accept-invite` | 接受邀请并设置密码（匿名，限流） |
| POST | `/auth/verify-email` | 用一次性令牌验证邮箱（匿名，限流） |
| POST | `/auth/register` | 自注册（匿名，需开启，限流） |
| POST | `/auth/verify/resend` | 重发验证邮件（匿名，限流，防枚举） |
| GET | `/public/notes` | 公开笔记列表（匿名） |
| GET | `/public/note?path=` | 公开笔记正文（匿名，非公开返回 404） |
| GET | `/ai/status` | AI 是否可用、模型名，以及 `toolCalling`（`true`/`false`/`unknown`）`agentEnabled` `maxSteps` `confirmPolicy` |
| POST | `/ai/chat` | AI：`{mode: chat\|organize\|complete, messages, content}`（纯文本，无工具） |
| POST | `/ai/agent` | 运行一个智能体回合：`{runId?, input, context?}`，返回 `{runId,status,text,steps[],pending?}`；`status` ∈ `done`/`needs_confirmation`/`max_steps`（未配置/停用/不支持时分别 503/400 `agent_disabled`/400 `tool_calling_unsupported`） |
| POST | `/ai/agent/confirm` | 批准/拒绝待确认工具调用：`{runId, toolCallId, decision: approve\|reject, args?}`，批准可用 `args` 覆盖原参数 |
| POST | `/ai/agent/stop` | 丢弃会话（终止该 run）`{runId}` |
| GET | `/settings/ai` | 读取 AI + 智能体配置：`baseUrl` `model` `hasKey` `agentEnabled` `toolCalling`（`auto`/`off`）`confirmPolicy` `maxSteps` `allowedTools[]`（仅管理员，密钥永不回传） |
| PUT | `/settings/ai` | 运行时更新 AI/智能体配置（仅管理员，**部分更新**：省略字段保持不变） |
| GET | `/settings/mail` | 读取 SMTP 配置，密码永不回传（仅管理员） |
| PUT | `/settings/mail` | 运行时更新 SMTP 配置（仅管理员） |
| GET | `/settings/site` | 读取登录页/注册配置（仅管理员） |
| PUT | `/settings/site` | 更新注册开关、提示语、备案号、链接（仅管理员） |
| GET | `/assets/orphans` | 未被引用的附件列表 |
| POST | `/assets/orphans/purge` | 清理孤儿附件 |
| GET | `/history?path=` | 笔记历史版本列表 |
| GET | `/history/revision?path=&id=` | 某版本内容 |
| POST | `/history/restore` | 回滚到某版本 |
| GET | `/trash` | 回收站列表 |
| POST | `/trash/restore` | 从回收站恢复 |
| DELETE | `/trash?id=` | 彻底删除 |
| GET | `/openapi.json` | OpenAPI 规范（公开） |

**其他端点**

| 路径 | 说明 |
| --- | --- |
| `/assets/{path...}` | 附件访问（`GET`/`HEAD` 匿名只读以支持公开页；上传与 `/api/v1/assets` 仍需认证；内嵌前端资源优先，经静态处理器压缩/缓存） |
| `/dav/` | WebDAV 挂载（Basic 认证，需 auth=multi） |
| `/mcp` | MCP 服务端（JSON-RPC 2.0，Bearer 令牌，协议 `2026-07-28`） |
| `/api/docs` | 自包含的 OpenAPI 交互文档 |

**CLI（离线，复用同一 `service` 层）**

`overview [全局参数] <命令>`，全局参数 `-C/--data-dir DIR`、`--json`。命令覆盖：
`list/search/read/get/write/delete/move/rename/mkdir/links/resolve`、
`history/revision/restore`、`trash/trash-restore/trash-purge`、
`asset-upload/assets-orphans/assets-purge`、`import/export-zip`、
`build`（静态站，`export` 为其别名）。CLI 直接读写数据目录，**无需启动服务**，便于脚本与工具调用。

**错误响应**
```json
{ "error": { "code": "conflict", "message": "note was modified by another client" } }
```
`code` ∈ `invalid` | `not_found` | `conflict` | `rate_limited` | `unavailable` | `email_not_verified` | `internal`，
对应 HTTP 400/404/409/429/503/403/500。未验证邮箱登录返回 `email_not_verified`，邮件未配置/投递失败返回 503。

**中间件**：`X-Request-Id` 注入、panic 恢复、结构化请求日志（method/path/status/duration/request_id）。

---

## 7. 中文检索设计（ADR-005/006）

**问题**：FTS5 默认 `unicode61` 把连续中文当作一个整词，无法子串匹配（`发模` 无法命中 `并发模型`）；`trigram` 又要求查询 ≥3 字符，2 字中文（极常见）失效。

**方案**：索引前对文本做自研分词——对 CJK 连续串展开为**单字 + 相邻双字 bigram**，Latin 词保持原样，再交给 `unicode61` 索引。

- `并发模型` → `并 发 模 型 并发 发模 模型`
- 查询 `发模` 同样分词为 `发 模 发模`，AND 命中

**快照**：因 FTS 存的是分词文本，快照改由 Go 在原文上定位 + 截取 + `html.EscapeString` + `<mark>` 高亮，既保证可读，也消除存储型 XSS。

---

## 8. 前端架构

### 8.1 结构

```
main.ts → Pinia + Router
router.ts        全部路由懒加载（import()）：/ → EmptyState ； /note/:path(.*) → EditorPane
                 /settings/{appearance|editor|ai|mail|site|data|users|tokens} → SettingsView 分区
                 /public[/:path] → PublicHomeView / PublicNoteView ； /trash
                 /login /setup /register /forgot-password /reset-password /accept-invite /verify-email
                 /capture → 重定向首页（快速捕获改为顶栏弹窗）
stores/
  workspace.ts   目录树、当前笔记、增删改查、搜索
  settings.ts    主题/字号/语言/压缩/专注/侧栏宽度/主题色（localStorage）
  auth.ts site.ts dialog.ts
views/           Login/Setup/Register/ForgotPassword/ResetPassword/AcceptInvite/VerifyEmail
                 SettingsView + settings/{Appearance,Editor,AI,Mail,Site,Data,Users,Tokens}Section
components/
  App.vue        布局 + RouterView + DialogHost + CaptureDialog
  BrandMark.vue  品牌图形（笔记/文档 SVG）
  Sidebar.vue    操作 + 搜索 + 目录树 + 可拖拽宽度（紧凑字号）
  TreeNodeItem.vue 递归节点（行内操作浮层，不改变行高）
  EditorPane.vue Tiptap 编辑器 + 工具栏 + 笔记栏（图片 + 文件附件）
  CaptureDialog.vue 顶栏快速捕获弹窗（URL → 抓取预览 → 选目录保存）
  PublicShell.vue 公开页外壳（文档站风格）
  TokensPanel.vue EmptyState.vue SlashMenu.vue TocPanel.vue HistoryPanel.vue LinksPanel.vue AiPanel.vue
editor/          extensions.ts / nodes.ts / lowlight.ts / slash.ts / link.ts
markdown/        pipeline.ts / tasks.ts / math.ts / diagrams.ts / doc.ts / rules.ts / footnotes.ts / assets.ts
api.ts           类型化客户端（/api/v1，ApiError）
styles.css       Gridea 风格设计令牌（暖白/深色 + 主题色变量）+ 组件样式
```

**构建分包**：路由懒加载使编辑器重依赖（TipTap / KaTeX / lowlight / Mermaid）不进首屏；
Vite 自动按需拆分（不使用 `manualChunks`，避免把预加载 helper 分进巨块）。

**设置页**（ADR-038）：`/settings` 为独立路由，左侧分类导航由 `SettingsView` 渲染（管理员多出邮件/站点/数据/用户/令牌分区），右侧 `<RouterView>` 内容面板；离开时经 `sessionStorage` 记忆返回路径。

### 8.2 编辑与自动保存

- 内部为 Tiptap(HTML)，落盘为 Markdown：加载 `marked`(md→html)，保存 `turndown`+GFM(html→md)
- 防抖 700ms 自动保存，携带 `baseVersion`
- **保存 flush**：切换笔记、离开路由（`onBeforeRouteLeave`）、关闭页面（`beforeunload`）前强制落盘，避免防抖窗口丢数据
- **冲突处理**：409 时不清空内容，提示用户刷新，避免覆盖
- 图片粘贴/拖拽 → 上传 `/api/v1/assets` → 插入节点（可选客户端压缩）
- **文件附件**（ADR-041）：工具栏「附件」可选择任意类型；非图片以带 `title`（文件名 + 大小）的链接插入，服务端对非内联类型强制 `Content-Disposition: attachment`。Markdown 落盘为相对路径 `assets/...`，渲染/导出时重写为 `/assets/...`
- 表格：工具栏插入 3×3；光标在表内时显示增删行列工具条
- `/` 斜杠命令菜单（标题/列表/引用/代码块/表格/分割线）
- 笔记栏：大纲 / 历史版本 / AI 助手 / 分享（页面级动作，与排版工具栏分离）
- **快速捕获**（ADR-042）：顶栏「捕获」弹窗粘贴 URL → `POST /api/v1/capture/preview` 抓取标题/正文（SSRF 防护）→ 选择目标文件夹 → 以 `baseVersion:"*"` 新建笔记并跳转

### 8.3 路由

- 深链接：`/note/技术/Go/并发模型.md` 可直接打开、刷新保持、可分享
- 目录树高亮由路由参数驱动

---

## 9. 开发与测试环境

### 9.1 命令（Makefile）

```bash
make dev          # 后端 :5230（内嵌前端）
make build        # 前端 + 单二进制（bin/overview）
make test         # Go 测试
make test-web     # 前端类型检查
make lint         # golangci-lint + vue-tsc
make fmt          # gofmt + prettier
make vet
make docker
```

前端热更新开发：
```bash
go run ./cmd/overview      # 终端 1
cd web && npm run dev      # 终端 2（:5173，/api 代理到 :5230）
```

### 9.2 测试覆盖

| 包 | 测试内容 |
| --- | --- |
| `textproc` | CJK 分词、混合文本、快照高亮、HTML 转义 |
| `markdown` | frontmatter 有无/往返/CRLF |
| `store` | 路径安全、写读、版本契约、List/Walk、Move/Delete、附件 |
| `index` | 迁移、Sync/Tree、中文中缀检索、快照转义、Upsert/Delete、用户/会话/令牌（邮箱唯一、一次性令牌消费） |
| `service` | 树组装、保存+检索、删除笔记/文件夹、冲突传播、邀请/验证/重置、捕获抓取 |
| `server` | 完整生命周期、409、路径校验、附件上传/服务头（含 RFC5987）、静态资源 gzip/304/SPA 回退、render 白名单、邮箱认证流程、站点设置、限流 |
| `history`/`trash` | 版本快照与裁剪、回收站恢复/清理 |
| `archivex`/`sitegen` | ZIP 往返、静态站生成与搜索索引 |
| `config`/`logging`/`ai`/`openapi` | 配置解析、日志轮转、AI 客户端（含 `ChatTools`/`ErrToolsUnsupported` 探测与解析）、OpenAPI 规范 |
| `tools` | 能力源 schema、OpenAPI 生成/回退、`Risk`/`Allows` 分类、`Preview`、`Exec` 输出 |
| `agent` | 有界循环、`max_steps`、危险操作需确认（approve/reject/改参）、按角色拒绝、会话 TTL/淘汰、审计写入、上下文裁剪、能力探测降级 |
| `mcp` | 初始化/发现、工具调用、认证、**协议版本协商**、`resultType`、工具面平铺（≥20 工具）、与 `internal/tools` 的一致性（parity） |
| `cli` | 读写/检索、参数位置无关解析、list/move、归档往返、history/restore、`build`/`export` 别名、用法错误 |

前端：`vue-tsc` 类型检查、**Vitest** 单元测试（`npm test`，覆盖 `markdown/html`、`markdown/doc`、`markdown/roundtrip`）、`vite build`。

集成测试用 `httptest` 真实走 HTTP 栈；`store`/`service`/`cli` 用 `t.TempDir()`。

### 9.3 CI 与发布

- `.github/workflows/ci.yml`：后端 `go vet` + `go test -race -cover`；前端 `npm ci` + `vue-tsc` + `npm test` + `vite build`；`golangci-lint`。
- `.github/workflows/docker.yml`：推送 `v*` 标签时构建 **多架构（amd64/arm64）** 镜像并发布到 **GHCR**（`ghcr.io/<owner>/<repo>`），使用内置 `GITHUB_TOKEN`，无需额外密钥。
- `.github/workflows/pages.yml`：导出文档站（`overview export`）并部署到 GitHub Pages。

---

## 10. 安全

| 项 | 现状 |
| --- | --- |
| 路径穿越 | `store.resolve` 净化 + `filepath.Rel` 校验；有测试覆盖 |
| 存储型 XSS | 快照服务端转义；前端仅渲染已转义内容；登录页外链仅放行 `http(s)://` |
| 附件 | `X-Content-Type-Options: nosniff`；非内联类型强制 `Content-Disposition: attachment`（含 RFC5987 `filename*`，中文名不丢失）；SVG 不内联 |
| 上传体积 | `http.MaxBytesReader` + `ParseMultipartForm`（`OVERVIEW_MAX_UPLOAD_MB`） |
| 认证 | 多用户（bcrypt + 会话 + 角色），`auth=multi` 时对 `/api/v1/*` 与附件上传强制认证；`auth=none` 为单用户模式 |
| 会话 | HttpOnly + SameSite=Lax Cookie；另支持 `Authorization: Bearer`（会话令牌或持久 API 令牌） |
| 邮箱令牌 | `user_tokens` 仅存 **SHA-256 哈希**；一次性（消费即标记 `used`）、按用途（邀请/重置/验证）隔离、可过期（7d/2h/24h）；重置密码后注销该用户全部会话 |
| 防枚举 | 找回密码、重发验证对未知/未激活账号一律返回成功；令牌校验失败统一返回「无效或过期」 |
| 限流 | 公开认证端点按客户端 IP（10 次/15min）与邮箱（3 次/小时）固定窗口限流，超限 429 |
| SSRF | 捕获抓取仅允许 `http(s)`；连接时校验解析后的 IP，拒绝 loopback/私网/链路本地/CGNAT/组播等；限制重定向次数（5）与响应体（2 MiB） |
| 智能体工具权限 | `tools.Allows(role, name)`：`member` 仅可读/写工具，`admin`/no-auth `owner` 才可执行危险工具；可选 `allowedTools` 白名单再收窄；被拒调用记为 `denied` 并回填模型 |
| 危险操作确认 | 确认策略 `dangerous`（默认，写工具需确认）/`all`（读写都确认）/`none`；危险工具在 `none` 下仍执行，但默认与推荐为 `dangerous`；确认可改参或拒绝 |
| 提示注入 | 笔记正文/标题/检索结果/工具输出在 `SystemPrompt` 中明确定为**不可信数据**；模型只能调用受白名单约束的工具，危险动作必须人工确认 |
| 智能体限流 | 按用户（无身份时回退客户端 IP）20 次 / 5 分钟固定窗口，超限 429；会话有 TTL（30min）与容量（256）上限，超限淘汰最旧 |
| 工具调用能力探测 | 被 provider 4xx 明确拒绝 tools 时缓存「不支持」并降级（前端禁用智能体、返回 `tool_calling_unsupported`），避免反复失败 |
| 工具审计 | `ai_tool_audit` 仅存脱敏摘要：密钥字段 `[redacted]`，正文/上传存 `sha256 + 长度`，不落明文正文；记录可撤销操作对应的版本/回收站/修订 id |
| 匿名资产只读 | `GET/HEAD /assets/` 匿名放行以渲染公开页；上传/维护/`/api/v1/assets` 仍需认证；文件名含随机 ULID，不可枚举 |
| base URL / Host | 邮件链接优先用 `OVERVIEW_BASE_URL`，否则按请求推导（尊重 `X-Forwarded-Proto`）；生产应在反代后使用 HTTPS |
| 密钥 | AI/MCP/S3/SMTP 密钥仅存服务端，接口永不回传（`hasKey` / `hasPassword`） |
| 日志 | 结构化 JSON，无敏感内容；邮件收件人/正文/令牌从不记录 |

> ✅ render 模式已收紧为**白名单**：`renderGuard` 仅放行 `health`、`auth/state`、
> `tree`、`note`、`public/*`、`openapi.json`、`/api/docs` 与附件读取；其余 `/api/` 路径
> 一律 404（`history`、`trash`、`assets/orphans`、`search`、`settings` 等），写方法返回 405。
> 覆盖测试见 `internal/server/render_test.go`。

---

## 11. 已知限制与权衡

1. 文件夹级重命名/移动走后缀替换重建（`ReplacePrefix`），超大库仍有开销；单文件变更已增量。
2. 标题一旦写入 frontmatter 不随正文 H1 变化。
3. Tiptap 的 Markdown 往返：表格与 wiki 链接已修复为无损（见 §13.8），其他复杂结构仍可能被规范化。
4. 单实例：SQLite + 本地文件系统，无法水平扩展（如需多实例，替换 `core` 端口实现）。
5. i18n 覆盖中/英/繁中/日/德 5 种语言，非 memos 的 40+（按需扩展，暂缓）。

> 已修复（原「已知问题」）：render 模式只读端点越权、版本历史无清理、静态导出站无搜索、
> WebDAV 写入全量重建、OpenAPI spec 端点缺失、表格/wiki 链接往返损坏。
>
> v0.11.0 补充：侧栏悬浮抖动（行内操作改为绝对定位浮层）、长文件名不可见（新增可拖拽宽度 +
> 悬浮提示）、柔和配色导致层级不清（加深文字令牌、强化激活态）、连续切换卡顿（取消过期加载、
> 后台保存、Mermaid 渲染防抖）、首屏体积偏大（路由懒加载 + 静态资源 gzip/immutable）。
>
> v0.12.0 待权衡：限流与令牌表为**单进程内存/单库**实现，多实例部署下各自计数且不共享令牌状态；
> 邮件发送为同步阻塞（10s 拨号超时），高并发邀请/重置可能拖慢请求。
>
> v0.13.0 待权衡：智能体会话 registry 与限流同为**单进程内存**，多实例下会话不共享（确认请求须落到同一实例），
> 需要粘性会话或外部存储；审计 async 写入（失败仅忽略）且无清理策略；上下文裁剪按字节估算，
> 极端长会话仍可能触发 provider 上限；工具调用为**非流式**（一次回合返回完整结果）。

---

## 12. 路线图

**Phase 2 — 核心增强**
- [x] `[[wiki-link]]` 双链 + 反链面板（基于已有稳定 `id`）
- [x] 斜杠命令（`/`）悬浮菜单
- [x] 用户认证 / 多用户（bcrypt + 会话 + 角色）
- [x] 附件路径可移植（相对路径 + 渲染重写）

**Phase 3 — 集成与多端**
- [x] WebDAV 挂载（供 Obsidian 等直接读写）
- [x] i18n（前端文案抽取 + 语言切换）
- [x] 编辑器目录（TOC）
- [x] 公开分享 / 匿名只读访问
- [x] MCP 服务端（AI 可操作笔记）
- [x] AI 助手（对话 / 整理 / 补全）
- [ ] 移动端 PWA / 原生 App（基于 `/api/v1`）

**Phase 4 — 体验与规模**
- [x] 文件监视器，外部编辑自动入库
- [x] 增量索引（避免全量重建）
- [x] 版本历史 / 回收站
- [x] 附件引用计数与回收
- [x] 多存储后端（S3）
- [x] OpenAPI 文档
- [x] 移动端 PWA
- [x] 图片压缩（上传前客户端压缩）

**Phase 5 — 设置与设计系统（v0.8）**
- [x] 设置中心（外观 / 编辑器 / AI）
- [x] 运行时 AI 配置（管理员可编辑 Base URL / Key / Model）
- [x] 主题（跟随系统/浅/深）与字号偏好
- [x] 顶栏全局搜索（Ctrl+K）
- [x] Starlight 风格布局与统一排版阶梯
- [x] 表格就近浮动工具条（编辑体验优化）
- [x] 多级菜单第三级缩进微调

**Phase 6 — 对标 memos 的体验补齐（v0.10）**

> 来源：与 memos v0.31 的功能对标分析。优先级 P0（日常刚需）→ P2（加分项）。

P0 — 写作刚需
- [x] 代码块语法高亮（编辑与预览）
- [x] 任务列表 / checkbox（GFM task list）
- [x] 拖拽移动 / 排序笔记树

P1 — 表达与集成
- [x] 数学公式（KaTeX）
- [x] Mermaid 图表 / diagrams
- [x] ZIP 导入 / 导出（笔记 + 附件 + 元数据）
- [x] 应用内快速捕获（顶栏弹窗：服务端抓取网页标题/正文、选择目标文件夹、SSRF 防护，`POST /api/v1/capture/preview`）
- [x] MCP 工具由 OpenAPI 生成，并补齐 spec 缺失端点

P2 — 打磨与生态
- [x] GFM 脚注
- [x] 专注模式 / 快捷键面板
- [x] sitemap.xml / robots.txt（渲染与公开站）
- [x] i18n 补充 ja / zh-TW / de

**Phase 7 — 未完成 / 待办（下一阶段候选）**

> 与 memos v0.31 对比后仍缺失、或自身体验上的缺口，按主题归类，尚未排期。

组织与检索
- [ ] 标签系统：标签管理 UI、标签树、按标签过滤、编辑器 `#` 自动补全（现仅作为可搜索字段）
- [ ] 置顶（Pin）/ 收藏
- [ ] 保存的筛选视图（类似 memos CEL Shortcuts）
- [ ] 时间线视图（按更新时间聚合）

协作与实时
- [ ] 实时刷新 / 多人协作（SSE/WebSocket）
- [ ] 评论、反应、@提及
- [ ] 通知

编辑体验
- [ ] 版本历史 Diff 视图（现仅原文预览）
- [ ] 草稿本地持久化（防意外关闭）
- [x] 非图片附件的插入 UI（v0.12 已支持任意类型，见 Phase 9）

平台与运维
- [ ] 出站 Webhook（签名 + SSRF 防护）
- [ ] 内嵌 WebView2 的原生窗口（免依赖浏览器）
- [ ] OIDC / SSO
- [ ] Mobile 原生 App（PWA 已具备）
- [ ] REST 之外补 gRPC

**Phase 6.5 — 已知问题修复（v0.10.2）**
- [x] render 模式收紧为白名单，隐藏 history/trash/orphans/search/settings
- [x] 版本历史保留策略（`OVERVIEW_HISTORY_KEEP`，默认 50，按笔记裁剪）
- [x] 静态导出站内置搜索（`search-index.json` + `search.js`）
- [x] WebDAV 写入按受影响路径增量重建索引
- [x] OpenAPI spec 补齐 `/settings/ai`、`/assets/{path}`、`/export`、`/import`，并说明 `/dav`、`/mcp`
- [x] 编辑器表格 / wiki 链接 Markdown 往返修复（`sanitizeEditorHtml`、保留 `data-wiki`）

**Phase 8 — CLI 化、MCP 升级与性能（v0.11.0）**

可编程与集成
- [x] 离线 CLI：`list/search/read/get/write/delete/move/rename/mkdir/links/resolve`
- [x] CLI：`history/revision/restore`、`trash/trash-restore/trash-purge`
- [x] CLI：`asset-upload/assets-orphans/assets-purge`、`import/export-zip`
- [x] CLI：`build` 静态站生成（`--all` 可导出全部笔记），`export` 保留为别名
- [x] MCP 升级到 `2026-07-28`：无状态 `_meta`、`server/discover`、`resultType`、版本协商（dual-era）
- [x] MCP 工具面拉平到 CLI/REST（22 个工具）

体验与性能
- [x] 前端路由懒加载 + 重依赖按需加载（首屏 JS ~154KB）
- [x] 静态资源 gzip + 哈希资源 immutable 缓存 + ETag 协商（Lighthouse 99）
- [x] 侧栏可拖拽宽度、行内操作浮层消除悬浮抖动、长文件名提示
- [x] 连续切换笔记性能：过期加载取消、后台保存、Mermaid 渲染防抖
- [x] 笔记/文档风格新 Logo；暖色柔和调色板（保留层级对比）

交付
- [x] GHCR 多架构（amd64/arm64）镜像自动发布（`.github/workflows/docker.yml`）

**Phase 9 — 配色 · 设置页 · 邮箱用户 · 附件 · 捕获（v0.12.0）**

外观与设置
- [x] Gridea 风格整体配色（琥珀主色 `#D4870E`，暖白/深色两套），静态站调色板同步
- [x] 主题色自定义：预设 + 取色器，前端派生明暗自适应色阶
- [x] 设置改为独立路由页 `/settings`，分区导航（外观/编辑器/AI/邮件/站点/数据/用户/令牌）

账户与邮件
- [x] 邮箱用户管理：邀请、邮箱验证、密码重置（一次性哈希令牌）
- [x] 可开关自注册（`settings` 表），登录页内容注入（提示语/备案号/链接）
- [x] SMTP 运行时配置（`settings` 表 + `/settings/mail`），密钥仅存服务端
- [x] 公开认证端点限流 + 防枚举
- [x] `OVERVIEW_BASE_URL` 生成邮件链接

编辑与捕获
- [x] 文件附件：任意类型上传 + 下载链接（非内联强制 attachment，RFC5987 文件名）
- [x] 匿名只读 `/assets/`（公开页附件）
- [x] 应用内快速捕获（顶栏弹窗 + SSRF 防护）

**Phase 10 — AI 智能体与工具调用（v0.13.0）**

能力与循环
- [x] 抽取 `internal/tools` 为唯一能力源（`Defs/DefsOpenAI/Exec/Risk/Allows/Preview`），MCP 变薄适配器
- [x] `internal/ai` 支持 OpenAI 兼容 function/tool calling（`ChatTools` + 能力探测降级）
- [x] `internal/agent` 有界循环（默认 8 步 / 上限 32）、进程内会话 registry（TTL + 容量）、`Stop`
- [x] 危险操作「预览→确认→执行」两段式（`/ai/agent/confirm`），批准可改参、拒绝回填

安全与审计
- [x] 按角色裁剪工具集（member 只读/写，危险仅 admin/owner）+ 可选 `allowedTools` 白名单
- [x] 提示注入防护（内容视为不可信数据）+ 危险必确认（策略 `dangerous`/`all`/`none`）
- [x] 按用户智能体限流（20 次 / 5 分钟）
- [x] 工具审计 `0009_ai_tool_audit` + `internal/index/audit.go`（args 脱敏、无明文正文）

HTTP 与前端
- [x] `POST /ai/agent`、`/ai/agent/confirm`、`/ai/agent/stop`；`/ai/status`、`/settings/ai` 扩展 agent 字段
- [x] 前端「智能体面板」：`AgentPanel/ToolCallCard/ConfirmBar` + `stores/agent.ts` + `api.ts`
- [x] 编辑器右侧「助手 / 智能体」分段；受影响对象可跳转、写类操作后刷新目录

---

## 13. 附录

### 13.1 环境变量

| 变量 | 默认 | 说明 |
| --- | --- | --- |
| `OVERVIEW_ADDR` | `:5230` | 监听地址 |
| `OVERVIEW_DATA_DIR` | `./data` | 数据根目录 |
| `OVERVIEW_DB` | `<data>/overview.db` | SQLite 路径 |
| `OVERVIEW_MAX_UPLOAD_MB` | `32` | 上传上限 |
| `OVERVIEW_HISTORY_KEEP` | `50` | 每篇笔记保留的历史版本数（`0` 不裁剪） |
| `OVERVIEW_LOG_LEVEL` | `info` | 日志级别：`debug`/`info`/`warn`/`error` |
| `OVERVIEW_LOG_FORMAT` | `json` | 日志格式：`json` 或 `text` |
| `OVERVIEW_LOG_FILE` | 空 | 日志文件路径；设置后同时写入文件（空则仅 stdout） |
| `OVERVIEW_LOG_MAX_MB` | `10` | 单个日志文件大小上限，超过后轮转 |
| `OVERVIEW_LOG_BACKUPS` | `3` | 轮转保留的历史文件数（`0` 表示截断不保留） |
| `OVERVIEW_AUTH` | `multi` | 认证模式：`multi` 或 `none` |
| `OVERVIEW_SITE_TITLE` | `Overview` | 站点标题（UI 与 render 模式） |
| `OVERVIEW_SITE_THEME` | `auto` | 静态站主题：`auto`（跟随系统）/`light`/`dark` |
| `OVERVIEW_RENDER` | `false` | 设为 `true` 变为公开只读文档站 |
| `OVERVIEW_EXPORT_DIR` | `_site` | `overview export` 输出目录 |
| `OVERVIEW_EXPORT_BASE` | `/` | `overview export` URL 前缀 |
| `OVERVIEW_BASE_URL` | 空 | 生成邮件链接的外部可达基址；空则按请求推导（尊重 `X-Forwarded-Proto`） |
| `OVERVIEW_MCP_TOKEN` | 空 | MCP 静态 Bearer 令牌；空且 auth=multi 时用会话令牌或 UI 生成的 API 令牌 |
| `OVERVIEW_AI_BASE_URL` | 空 | OpenAI 兼容基址（如 `https://api.openai.com/v1`）；空则禁用 AI |
| `OVERVIEW_AI_API_KEY` | 空 | AI 密钥 |
| `OVERVIEW_AI_MODEL` | `gpt-4o-mini` | 模型名 |
| `OVERVIEW_MAIL_HOST` | 空 | SMTP 主机；与 `OVERVIEW_MAIL_FROM` 同时设置才启用邮件 |
| `OVERVIEW_MAIL_PORT` | `587` | SMTP 端口 |
| `OVERVIEW_MAIL_USERNAME` | 空 | SMTP 用户名（空则不认证） |
| `OVERVIEW_MAIL_PASSWORD` | 空 | SMTP 密码 |
| `OVERVIEW_MAIL_FROM` | 空 | 发件地址 |
| `OVERVIEW_MAIL_STARTTLS` | `true` | 是否使用 STARTTLS |
| `OVERVIEW_S3_BUCKET` | 空 | 设置后附件改存 S3 兼容存储 |
| `OVERVIEW_S3_ENDPOINT` | 空 | S3 端点（如 `s3.amazonaws.com`） |
| `OVERVIEW_S3_REGION` | `us-east-1` | 区域 |
| `OVERVIEW_S3_ACCESS_KEY` / `_SECRET_KEY` | 空 | 凭据 |
| `OVERVIEW_S3_USE_SSL` | `true` | 是否使用 HTTPS |
| `OVERVIEW_S3_PUBLIC_URL` | 空 | 可选 CDN/公开前缀 |

### 13.3 双链设计（v0.3）

- 语法：`[[Target]]` / `[[Target|Display]]`
- 解析：`textproc.ExtractWikiLinks` 抽取目标（忽略代码块/行内代码），存入 `links(source_id, source_path, target_raw, target_key)` 表
- 目标解析优先级：完整路径 → 路径+.md → 标题 → **文件名（basename）** → id；因此 `[[并发模型]]` 可命中 `技术/Go/并发模型.md`
- 反链：以目标笔记的 {路径, 标题, 文件名, id} 为键，反查 `links` 指向它的来源笔记
- API：`GET /api/v1/links?path=`、`GET /api/v1/resolve?target=`
- 前端：编辑器内 `[[...]]` 渲染为可点击链接（点击解析跳转，未创建则询问创建）；右侧面板展示反向链接与外部链接
- 斜杠命令：`/` 触发 `@tiptap/suggestion` 菜单（标题/列表/引用/代码块/表格/分割线）

### 13.4 认证、WebDAV、i18n（v0.4/v0.5）

- **认证**（ADR-011/012）：`users`/`sessions` 表（迁移 0004，0008 扩展），bcrypt 哈希；`AuthService` 负责 setup/login/logout/用户管理；HTTP 中间件对 `/api/v1/*` 与附件上传强制认证（Cookie 或 `Bearer`）；无用户时前端跳转 `/setup` 创建管理员。角色 `admin`/`member`，仅管理员可管理用户，且禁止删除最后一个**活跃**管理员。详见 §13.14（邮箱用户管理）。
- **WebDAV**（ADR-013）：`/dav/` 映射 `data/notes`，`auth=multi` 时启用 Basic 认证；成功写入去抖 1s 后全量重建索引，外部编辑器保存的内容即可被搜索。
- **可移植附件**（ADR-014）：Markdown 中保存 `assets/2026/10/xxx.png`，SPA 渲染为重写为 `/assets/...`；`/assets/` 既是附件路由也是前端静态资源前缀，服务端优先命中内嵌资源。
- **i18n**（ADR-015）：`web/src/i18n` 提供 `t()` 与响应式 `locale`，持久化到 `localStorage`，顶栏下拉切换中/英；斜杠菜单、弹窗、工具栏等全部接入。

### 13.5 协作与智能（v0.6）

- **TOC**：`@tiptap/extension-table-of-contents` 生成标题锚点并跟踪滚动高亮；`TocPanel` 支持层级缩进、平滑跳转，工具栏可开关。
- **公开分享**（ADR-016）：frontmatter `public`（迁移 0005）；`/api/v1/public/*` 匿名可读，仅暴露公开笔记；前端 `/public` 列表与只读页，编辑器工具栏「分享」一键复制链接。公开页把 wiki 链接降级为纯文本，避免泄漏私有目标。
- **MCP**（ADR-017）：`POST /mcp` JSON-RPC，方法 `initialize`/`tools/list`/`tools/call`；工具面由 `internal/tools` 统一提供（v0.13 起，见 §13.19）；令牌认证（`OVERVIEW_MCP_TOKEN`，否则回退会话令牌）；工具错误按规范 in-band 返回。
- **AI**（ADR-018）：`internal/ai` 无依赖 OpenAI 兼容客户端；`AIService` 提供 `organize`/`complete`/`chat`；`/api/v1/ai/*` 未配置返回 503；前端 AI 面板支持对话、整理（替换）、补全（追加）、插入。v0.13 起 `internal/ai` 另支持 tool calling，`/ai/agent*` 提供可执行应用内操作的智能体（见 §13.19）。

### 13.6 规模化与运维（v0.7）

- **增量索引**（ADR-019）：`Index.ReplacePrefix` 只重建某子树；`Delete`/`Move`/`ReindexPath` 均只影响相关路径。
- **文件监视器**：`internal/watcher` 用 fsnotify 递归监听 `data/notes`，去抖后对顶层子树增量重建——外部编辑器/Git/DAV 的改动即时可搜。
- **版本历史**（ADR-020）：每次保存前把旧文件快照到 `data/.history/<path>/`；提供列表、查看、回滚。
- **回收站**（ADR-020）：删除改为移动到 `data/.trash/<id>/`（含 `meta.json`），支持恢复与彻底删除。
- **附件孤儿清理**：扫描所有正文中的 `assets/...` 引用，列出并清理未被引用的附件。
- **S3 后端**（ADR-021）：`internal/s3store`（minio-go）实现 `AssetStore`，配置 `OVERVIEW_S3_BUCKET` 即启用。
- **PWA**（ADR-022）：manifest + service worker，仅缓存应用壳与哈希构建产物，离线可启动。
- **OpenAPI**（ADR-023）：`/api/v1/openapi.json` 与 `/api/docs`（无外部依赖）。

### 13.7 设置中心与设计系统（v0.8，v0.12 改为独立页）

> v0.12.0 起设置由弹窗改为独立路由页 `/settings`，分区扩展为 8 个；旧的 `SettingsDialog` 已删除。详见 §13.15。

- **设置中心**（ADR-024/038）：独立页左侧分类导航：
  - 外观：主题（跟随系统/浅色/深色）、字号（紧凑/默认/舒适/大）、语言、主题色
  - 编辑器：上传前压缩图片（默认开，持久化）
  - AI：状态 + 当前模型；管理员可编辑 Base URL / API Key / Model
- **运行时 AI 配置**（ADR-025）：`settings` 表（迁移 0006）存 `ai_base_url`/`ai_api_key`/`ai_model`；`AIService.Load` 合成默认值与环境变量，`SaveConfig` 持久化并即时重载；接口 `GET/PUT /api/v1/settings/ai` 仅管理员，**密钥永不回传**（只返回 `hasKey`）。
- **图片压缩**：`web/src/media/compress.ts` 用 Canvas 降采样（默认最长边 1920）+ 优先 WebP 重编码 + 2MB 预算自动降质；GIF/SVG 与非图片不动，失败回退原图。
- **排版系统**（ADR-026）：所有 UI 文本基于 `--base-size` 派生的 `--text-xs/sm/ui/body`；编辑器正文 `--text-body` 与周边一致。字号设置切换时**整站同步缩放**。
- **布局重做**：全宽顶栏（品牌左、居中搜索 `Ctrl K`、右侧导航）、左侧分组式多级导航（顶层大写分组、按深度区分字重/缩进/配色）、内容区大标题 + 右侧「大纲」栏，参考 Starlight 文档站。
- **操作分层**（ADR-027）：编辑器上方新增**笔记栏**承载大纲/历史版本/AI/分享等页面级动作；格式化工具栏只保留排版类操作。
- **表格密度与动效微调**（参考 apple-design）：表格使用更紧的行距（1.4）、0.95em 字号与更小内边距，
  并折叠 ProseMirror 单元格内 `<p>` 的默认外边距，使行高贴合内容；公开页顶栏采用半透明材质
  （`backdrop-filter`）；按钮提供「按下即反馈」；新增 `prefers-reduced-motion` 与
  `prefers-reduced-transparency` 支持。

### 13.8 已知问题修复与交付（v0.10.2）

- **render 模式白名单**：`renderGuard` 增加 `renderAllowed(path)`，仅放行
  `health`、`auth/state`、`tree`、`note`、`public/notes`、`public/note`、`openapi.json`、
  `/api/docs` 与附件；`history`/`trash`/`assets/orphans`/`search`/`settings` 返回 404，写方法 405。
  回归测试：`internal/server/render_test.go`。
- **版本历史保留**：`history.Store` 支持 `keep`，`Snapshot` 后按修改时间裁剪旧快照；
  环境变量 `OVERVIEW_HISTORY_KEEP`（默认 50，0 表示不裁剪）。
- **导出站搜索**：`sitegen` 生成 `search-index.json`（标题 + 正文纯文本）与 `search.js`，
  顶栏提供无依赖的即时搜索（方向键 + Enter + Esc）。
- **WebDAV 增量重建**：`recordingFS` 包住 `webdav.FileSystem`，记录写入/删除/重命名路径，
  去抖后对每个路径调用 `service.ReindexPath`（替代原先的全量 `Reindex`）。
- **OpenAPI 补全**：新增 `/settings/ai`、`/assets/{path}`、`/export`、`/import`，并在文档描述中
  说明 `/dav` 与 `/mcp` 两个非 REST 面。
- **表格 / wiki 链接往返**：前端 `sanitizeEditorHtml` 移除 Tiptap 顶层表格的 `colgroup`/`col`、
  归一化 `colspan/rowspan` 与单元格 `<p>`，使 turndown-plugin-gfm 输出标准 Markdown 表格；
  自定义 `WikiLink` 扩展保留 `data-wiki`，使 `[[Note]]` 往返无损。

### 13.9 日志与测试（v0.10.2）

- **日志系统**：`internal/logging` 构建 `slog` 记录器——stdout 始终输出；设置
  `OVERVIEW_LOG_FILE` 后同时写入文件，并按大小轮转（`OVERVIEW_LOG_MAX_MB`、`OVERVIEW_LOG_BACKUPS`）；
  格式 `json`（默认）或 `text`，级别 `debug/info/warn/error`。HTTP 访问日志由 `server` 中间件结构化记录。
- **测试环境**：`go test ./...` 覆盖 `config / logging / history / archivex / sitegen / trash / ai /
  openapi / index / markdown / mcp / server / service / store / textproc` 等包；前端用 **Vitest**
  （`npm test`）对 `markdown/html`、`markdown/doc` 等纯函数做单元测试。`make test` 同时运行两端。
- **顺带修复**：回收站恢复后残留空条目（`trash.Restore` 未清理元数据目录）——由新增测试发现并修复。

### 13.2 v0.2 / v0.3 验证记录

- `go vet ./...` / `go test ./...` 通过（覆盖 6 个包）
- `vue-tsc --noEmit` / `vite build` 通过
- 端到端：中文中缀检索（`发模`/`识库`）命中；`baseVersion` 过期写入返回 409；附件返回 `nosniff`；路径穿越返回 400；`/health` 返回版本
- v0.4：未认证访问 `/api/v1/tree` 返回 401；首启 `/setup` 创建管理员后可访问；登出后再次 401；WebDAV PUT/GET 成功且写入被索引
- v0.5：未认证可加载 SPA 与内嵌资源（200），数据接口仍需认证（401）；语言切换即时生效
- v0.6：公开笔记匿名可读、私有笔记 404；MCP `initialize`/`tools/list` 正常、无令牌 401；AI 未配置返回 503、配置 mock 后 `organize` 返回重写内容
- v0.7：磁盘新建文件 2s 内可被搜索（文件监视器）；软删除进入回收站并可恢复；`/api/v1/openapi.json`、`/api/docs`、`/manifest.webmanifest`、`/sw.js`、`/icon.svg` 均 200
- v0.8：管理员保存 AI 配置后 `ai/status` 由 `enabled:false` 变为 `enabled:true`，`settings/ai` 回传 `hasKey:true` 且不含密钥；主题/字号切换即时生效；`vue-tsc` 与 `go test ./...` 全绿

### 13.10 离线 CLI（v0.11.0，ADR-028/029）

`internal/cli` 把 `service` 用例映射为命令行，直接操作数据目录，**不启动 HTTP 服务**。
它让 Overview 能作为「纯 Markdown 编辑器/索引」被脚本与工具调用，也方便在无服务器场景批量处理。

- **全局参数**：`-C/--data-dir DIR`（`config.WithDataDir` 重算派生路径）、`--json`（机器可读）。
- **参数顺序无关**：自实现 `parseArgs`，允许 `write <path> --body …`（Go `flag` 默认遇到位置参数即停止）。
- **命令**：`list/search/read/get/write/delete/move/rename/mkdir/links/resolve`、
  `history/revision/restore`、`trash/trash-restore/trash-purge`、
  `asset-upload/assets-orphans/assets-purge`、`import/export-zip`、`build`（`export` 别名）。
- **`write`**：`--file/--stdin/--body` 三选一，`--public`、`--create`、`--if-version`（乐观并发）。
- **`build`**（ADR-029）：调用 `sitegen.Generate` 生成可部署静态站（HTML/CSS/`search.js`/`search-index.json`）；
  `--all` 导出全部笔记（默认仅 `public`），`--base`/`--title`/`--out` 可配；`--theme auto|light|dark`
  （或 `OVERVIEW_SITE_THEME`）指定主题，默认 `auto` 跟随访客系统偏好。

### 13.11 MCP 协议升级与工具拉平（v0.11.0，ADR-030/031）

MCP 服务端升级到 **`2026-07-28`**，并实现为 **dual-era**（同时支持新协议与旧握手）：

- **Modern（2026-07-28）**：请求在 `_meta` 中携带 `io.modelcontextprotocol/protocolVersion`；
  服务端无状态处理；`server/discover` 返回 `supportedVersions`、`capabilities`、`serverInfo`
  （`resultType: "complete"`）；所有结果带 `resultType`。
- **版本协商**：不支持的版本返回 `UnsupportedProtocolVersionError`（`-32022`），`data` 列出支持版本；
  同时接受 `MCP-Protocol-Version` 头。
- **Legacy 兼容**：保留 `initialize` 握手与 `ping`，旧客户端无缝可用。
- **工具面（22 个，ADR-031）**：`notes_list/search/read/write/delete/move/rename/links/resolve`、
  `folder_create`、`notes_history/revision/restore`、`trash_list/restore/purge`、
  `assets_upload/orphans/purge`、`notes_reindex`、`public_notes/public_note`。
  Schema 由 OpenAPI 生成，`notes_rename`/`assets_upload` 等用手写覆盖。
  > v0.13.0 起该工具面迁至 `internal/tools`（`Defs()`/`Exec()`），MCP 仅保留薄 JSON-RPC 适配器，
  > 与 AI 智能体共用（见 §13.19、ADR-044）。
- **令牌管理（ADR-035）**：管理员在**设置 → API 令牌**生成/吊销长期令牌（`GET/POST/DELETE
  /api/v1/auth/tokens`），仅存哈希、明文只显示一次；令牌同时可用于 REST 的 `Bearer`
  认证；未配置时回退到服务端 `OVERVIEW_MCP_TOKEN` 或登录会话令牌。

### 13.12 性能与品牌（v0.11.0，ADR-032/033/034）

- **首屏性能**（ADR-032）：所有路由 `import()` 懒加载，编辑器重依赖不再进首屏；初始 JS 由 ~1.1MB
  降至 ~154KB（gzip ~57KB），页面总重 ~67KiB。
- **传输优化**（ADR-033）：`internal/server/static.go` 对静态资源 **gzip**（压缩结果内存缓存）、
  哈希资源 `Cache-Control: immutable`、壳文件 `no-cache` + ETag 304、SPA 回退；前端 `/assets/*`
  统一经该处理器（此前被上传路由绕过）。Lighthouse（移动端）由 80+ 提升到 **99**。
- **交互修复**：侧栏行内操作改为绝对定位浮层（消除悬浮引起的行高/截断抖动）；新增可拖拽宽度
  （持久化，双击复位）与文件名提示；加深文字令牌、强化激活态以恢复层级。
- **图片密集页卡顿修复**：笔记内图片统一加 `loading="lazy"` 与 `decoding="async"`（编辑器的
  `Image` 扩展 HTMLAttributes + 公开页 `resolveAssetSrc` 注入），离屏图片不再立即拉取/解码；
  切换笔记时用 `AbortController` 取消上一次请求，并在替换 DOM 前清理未完成图片，避免解码争用。
- **品牌**（ADR-034）：Logo 改为「页面 + 折角 + 文字行」的笔记/文档图形；调色板改为暖色柔和
  纸感灰 + 长春花靛蓝主色，成功/危险与代码高亮去饱和（`--hl-*` 变量）。
- **静态站主题**（ADR-029）：`sitegen` 支持 `auto/light/dark`——导出时在 `<html>` 写 `data-theme`，
  CSS 用 `:root[data-theme="dark"]` 固定深色、`@media (prefers-color-scheme:dark)` 处理 `auto`；
  站点调色板与 App 当前设计令牌保持一致。

### 13.13 v0.11.0 验证记录

- `go build ./...`、`go vet ./...`、`golangci-lint run`（0 issues）、`go test ./...` 全绿
- 前端 `vue-tsc`、`npm test`（Vitest）、`vite build` 全绿
- CLI 端到端：`write/read/search/links/history/revision/restore/move/delete/trash-*`、
  `asset-upload/assets-orphans/assets-purge`、`export-zip`+`import` 往返、`build`（9 页）均通过
- MCP：`server/discover` 返回支持版本；现代请求带 `resultType`；不支持版本返回 `-32022`；
  `tools/list` ≥20 工具；`folder_create`/`move`/`history`/`trash_list` 实测通过
- Lighthouse（移动端模拟，登录页）：Performance **99**，FCP 1.3s / LCP 1.9s / TBT 30ms / CLS 0
- Docker：推送 `v*` 触发 GHCR 多架构镜像构建成功

### 13.14 邮箱用户管理（v0.12.0，ADR-039/040）

- **账户模型**（迁移 0008）：`users` 增加 `email`（唯一，非空时）、`status`（`active`/`invited`）、`email_verified`；`user_tokens` 存一次性哈希令牌。
- **邀请**：管理员 `POST /auth/users/invite` 创建 `status=invited` 且 `password_hash=''` 的账号（bcrypt 永不匹配），邮件发送 `/accept-invite#token=…`（令牌放 URL fragment，不进访问日志）。接受邀请（`POST /auth/accept-invite`）设置密码并置为 active + verified。重发邀请会轮换令牌（`DeleteUserTokens` 再签发）。
- **邮箱验证**：配置 SMTP 后注册/改邮箱的账号需先验证；未验证登录返回 403 `email_not_verified`，登录页可重发（`POST /auth/verify/resend`）。无邮箱的旧账号跳过验证。
- **密码重置**：`POST /auth/password/request` 对未知/未激活账号静默成功（防枚举），发送 `/reset-password#token=…`（2h）；`POST /auth/password/reset` 消费令牌、更新密码并注销该用户全部会话。
- **自注册**：`POST /auth/register` 仅在 `settings.registration_enabled=on` 且 `auth=multi` 时可用；用户名即规范化邮箱、角色恒为 `member`（永不成 admin）。
- **邮件配置**：`MailService`（`settings` 表，SMTP host/port/user/pass/from/starttls）优先于环境变量；`from` 与 `host` 同时存在才 `Enabled`；`hasPassword` 回传布尔，密码永不回传。未配置时邀请/重置返回 503 `ErrMailDisabled`，落地账号仍可稍后重发。
- **限流**：`newRateLimiter(10, 15min, 4096)`（按 IP）+ `(3, 1h, 4096)`（按邮箱）；固定窗口、惰性清理、满时淘汰最旧键以限定内存。超限 429 `rate_limited`。
- **base URL**：邮件链接优先 `OVERVIEW_BASE_URL`，否则 `scheme://Host`（`scheme` 取 `r.TLS` 或 `X-Forwarded-Proto`）。生产应置于 HTTPS 反代之后。

### 13.15 设置页与 Gridea 配色（v0.12.0，ADR-036/037/038）

- **独立设置页**（ADR-038）：`/settings` 重定向到 `settings-appearance`，子路由 `appearance/editor/ai/mail/site/data/users/tokens`；`SettingsView` 左侧导航按角色过滤（`admin` 才有邮件/站点/数据/用户/令牌）；返回路径记忆在 `sessionStorage`。删除 `SettingsDialog/TokensDialog/UsersDialog` 三个旧组件。
- **配色**（ADR-036）：主色琥珀 `#D4870E`，浅色暖白纸感、深色近黑暖灰；静态站 `sitegen` 调色板与 App 令牌同步（`--accent:#d4870e`、深色 `#e9a23b`）。
- **主题色自定义**（ADR-037）：`AppearanceSection` 提供 8 个预设 + 原生取色器 + 复位。单值 hex 在前端派生 `--accent`、`--accent-hover`、`--accent-soft`、`--accent-ring`（浅色向黑/白混合，深色整体提亮），持久化到 `localStorage`。
- **侧栏紧凑**：笔记 15px / 文件夹 13px 的字号层级。

### 13.16 文件附件（v0.12.0，ADR-041）

- **上传**：`POST /api/v1/assets` 接受任意类型（仍受 `OVERVIEW_MAX_UPLOAD_MB` 限制）。图片沿用客户端压缩 + `setImage`；非图片以带 `title`（`文件名 (大小)`）的链接插入正文。
- **落盘/渲染**：Markdown 使用相对路径 `assets/…`；前端 `resolveAssetSrc` 与静态站生成会把 `src=`/`href="assets/…"` 重写为 `/assets/…`；编辑器 `WikiLink` 的 `isAllowedUri` 特批 `assets/` 前缀以免往返丢失。
- **下载头**：`assetMIMEType` 在平台 MIME 库外补充 pdf/zip/office/md/json 等；非内联类型返回 `Content-Disposition: attachment; filename="…"; filename*=UTF-8''…`（RFC 5987），中文文件名不丢。
- **匿名只读**：`GET/HEAD /assets/…` 匿名放行（公开页图片/附件），上传与维护端点仍需认证（ADR-043）。

### 13.17 应用内快速捕获（v0.12.0，ADR-042）

- **流程**：顶栏「捕获」打开 `CaptureDialog`，粘贴 URL → `POST /api/v1/capture/preview` 返回 `{title, text, url}` → 选择目标文件夹（来自目录树）→ 以 `baseVersion:"*"` 新建 `# 标题 + 正文 + > 来源` 的笔记并跳转。
- **抓取**：`service.FetchPreview` 仅允许 `http(s)`；正则抽取 `og:title`/`<title>` 与去标签正文（失败回退 meta description），正文截断 5000 字符；UA `OverviewBot/1.0`，超时 10s，响应体上限 2 MiB。
- **SSRF 防护**：`isPrivateHost`/`isPrivateIP` 拒绝 loopback、私网、链路本地、组播、CGNAT、benchmarking、"this network" 与 IPv6 ULA；`net.Dialer.Control` 在**连接时**复核解析后的 IP（防 DNS rebinding）；重定向限 5 次且逐跳校验 scheme/地址。
- **前端**：`/capture` 独立页已移除（路由重定向首页），改为顶栏弹窗；书签（bookmarklet）方案移除。

### 13.18 v0.12.0 验证记录

- `go build ./...`、`go test ./...` 全绿（新增 `index/users`、`service/auth_mail`、`service/capture`、`server/email_auth`、`server/site`、`server/ratelimit` 覆盖）
- 前端 `vue-tsc`、`npm test`（Vitest）、`vite build` 全绿
- 端到端：未验证邮箱登录 403 `email_not_verified`；邀请→接受→登录、重置→新密码→旧会话失效；自注册开关生效；公开认证端点超限 429
- 附件：非图片返回 `Content-Disposition`（含 RFC5987 中文名）；`GET /assets/…` 匿名 200，`POST /api/v1/assets` 未认证 401
- 捕获：内网/loopback URL 被拒（502/400），公网页面返回标题与正文

### 13.19 AI 智能体与工具调用（v0.13.0，ADR-044…050）

内置助手从「纯文本聊天」升级为**可执行应用内操作的智能体**。

**能力源 `internal/tools`（ADR-044）**

- 原 `internal/mcp/generate.go`（OpenAPI → schema 生成）与 `internal/mcp/tools.go`（`toolBindings` + `runTool`）整体迁入 `internal/tools`，删除 `generate.go`。
- 对外 API：`Defs()`（MCP 形状工具列表，schema 由 OpenAPI 生成、可回退手写）、`DefsOpenAI(allow)`（OpenAI `tools[].function` 形状）、`Exec()`（进程内执行，输出与旧 MCP 逐字节一致）、`Risk()`/`Allows()`（风险与角色判定）、`Preview()`（危险操作的人类可读预览）、`Known()`。
- `Set` 仅依赖 `service.Service`；MCP 的 `Server` 持有一个 `tools.Set`，`tools/list`/`tools/call` 直接委托，适配器变薄。

**工具调用客户端 `internal/ai`（ADR-045）**

- 新增 `ChatTools(ctx, messages, offered, temperature)`，请求体带 `tools` + `tool_choice:"auto"`，解析 `choices[0].message` 的 `content` 与 `tool_calls`，返回 `ChatResult{Content, ToolCalls, FinishReason}`。
- 新增类型 `AgentMessage`（`role`/`content`/`tool_calls`/`tool_call_id`）、`Tool`、`ToolCall`。
- provider 以 4xx 明确拒绝 tools（错误体含 tool/function）时返回 `ErrToolsUnsupported`，由 `AIService` 缓存该事实。

**智能体循环 `internal/agent`（ADR-046/047）**

- `Agent.Run`：首轮注入 `SystemPrompt`，追加用户输入（可带当前笔记 `path` 与选区 `selection`），进入有界循环；`maxSteps` 默认 8、上限 32，用尽返回 `StatusMaxSteps`。
- 每轮 `tools.Defs()` 按角色 + `allowedTools` 过滤后交给模型；无 `tool_calls` 即 `done`；否则逐条处理，工具不允许记为 `denied` 并回填。
- 需确认时保存 `PendingCall` 并返回 `needs_confirmation`（含 `preview`）；`Confirm` 批准可覆盖 args，拒绝回填 `rejected`，随后从停点续跑。
- 进程内会话 registry：`Start`/`Session`/`Stop`，TTL 30 分钟、容量 256（超限淘汰最旧）；上下文超预算时按「完整回合」裁剪并保留 `tool_calls`→`tool` 配对。
- `RunID` 为 ULID；`Stop` 丢弃会话。

**审计（ADR-048）**：每步写 `ai_tool_audit`（见 §4.3），`sanitizeAuditArgs` 脱敏密钥与正文、截断，另记录危险操作对应的 `note_version`/`trash_id`/`revision_id` 作为撤销线索。

**HTTP（`internal/server/agent.go`）**

- `POST /ai/agent`：无 `runId` 则 `Start`，否则复用；返回 `{runId,status,text,steps[],pending?}`；步进项含 `toolCallId/name/args/risk/status/summary`。
- `POST /ai/agent/confirm`、`POST /ai/agent/stop`；错误码 `agent_disabled`、`tool_calling_unsupported`、`run_not_found`。
- 门禁：AI 未配置 503；agent 停用 400 `agent_disabled`；探测到不支持 400 `tool_calling_unsupported`；按用户限流 20 次/5 分钟。

**前端「智能体面板」**

- `AgentPanel.vue`（可用性判定/上下文 chip/步骤/文本/确认/停止/重试）、`ToolCallCard.vue`（可折叠、风险与状态徽章、受影响 `.md` 路径可跳转）、`ConfirmBar.vue`（预览 + 可编辑 args + 批准/拒绝）；`stores/agent.ts` 管理 `idle/running/awaiting/done/max_steps/error`；`api.ts` 新增 `aiAgent/aiAgentConfirm/aiAgentStop` 与 `AIStatus`。
- 编辑器右栏改为「助手 / 智能体」分段（`EditorPane.vue`）；写类操作完成后 `flush` + 刷新目录，命中当前笔记则重载。

### 13.20 v0.13.0 验证记录

- `go build ./...`、`go test ./...` 全绿（新增 `internal/tools`、`internal/agent`、`internal/ai` 工具调用、`server/agent`、MCP↔tools parity 覆盖）
- 前端 `vue-tsc`、`npm test`（Vitest）、`vite build` 全绿
- 端到端：读写工具可自动执行；删除/清空触发 `needs_confirmation` 且需 `approve`；`reject` 回填并续跑；`member` 调用危险工具被拒；不支持 tool calling 的 provider 返回 `tool_calling_unsupported`；限流超限 429；审计表仅含脱敏 args
