# Overview 设计文档

> 版本：v0.11.0（CLI 化 · MCP 2026-07-28 · 性能与品牌）
> 更新日期：2026-10-02
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
| v0.10.0 | 对标 memos 补齐 | 代码高亮 / 任务列表 / KaTeX / Mermaid / 脚注、拖拽移动、ZIP 导入导出、浏览器快速捕获、专注模式与快捷键面板、MCP 工具由 OpenAPI 生成、sitemap/robots、i18n 增繁中/日/德 |
| v0.10.2 | 已知问题修复 | render 模式白名单、历史保留策略、导出站搜索、WebDAV 增量重建、OpenAPI 补全、表格/wiki 往返修复 |
| v0.11.0 | CLI 化 · MCP 升级 · 性能与品牌 | 离线 CLI（笔记/检索/历史/回收站/附件/归档）、`build` 静态站生成、MCP 升级到 `2026-07-28`（无状态 `_meta` + `server/discover` + `resultType`）并补齐到 22 个工具、前端路由切分 + 静态资源 gzip/immutable 缓存（Lighthouse 99）、可拖拽侧栏与对比度回归、笔记风格新 Logo 与暖色柔和主题、GHCR 多架构镜像 |

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
│ service (应用服务层)  ← 业务用例、事务边界、Tree 组装         │
├────────────────────────────────────────────────────────────┤
│ core (领域层)                                              │
│   模型 Note/TreeNode · 端口接口 · 哨兵错误                   │
├──────────────┬───────────────┬─────────────────────────────┤
│ store        │ index         │ textproc                    │
│ (文件系统)    │ (SQLite+FTS5) │ (CJK 分词/快照)              │
└──────────────┴───────────────┴─────────────────────────────┘
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

---

## 4. 数据模型

### 4.1 磁盘布局

```
data/
├── notes/                         # 目录树 = 物理文件夹
│   ├── 欢迎.md
│   └── 技术/Go/并发模型.md
├── assets/2026/10/<ulid>-<name>.# attachments
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
- `0001_init.sql`：`notes`（元数据）+ `notes_fts`（FTS5，unicode61，存**分词后**文本）

> 从旧版（v0.1，无迁移表且 `notes` 缺列）升级：索引可直接删除重建，内容文件不受影响。

---

## 5. 后端分层

### 5.1 包职责

| 包 | 职责 | 关键类型/方法 |
| --- | --- | --- |
| `internal/core` | 领域模型、端口接口、哨兵错误 | `Note` `TreeNode` `NoteRepository` `Index` `AssetStore` `ErrNotFound/ErrConflict/ErrInvalid` |
| `internal/service` | 应用用例编排 | `Tree` `GetNote` `SaveNote` `Delete` `Move` `Mkdir` `Search` `Upload` `Reindex` |
| `internal/store` | 文件系统实现 | 原子写、版本校验、`List`(结构)、`Walk`(全量) |
| `internal/index` | SQLite 实现 | 迁移、`Upsert` `Sync` `Tree` `Search` |
| `internal/textproc` | 文本处理 | `Tokens` `Segment` `Snippet` |
| `internal/server` | HTTP 适配 | 路由、中间件、错误映射、ETag |
| `internal/markdown` | frontmatter 解析/序列化 | `Parse` `Document.String` |

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
| POST | `/assets` | 上传附件（multipart `file`） |
| GET | `/assets/{path...}` | 访问附件（nosniff，非图片强制下载） |
| POST | `/reindex` | 重建索引 |
| GET | `/links?path=` | 出链与反链 |
| GET | `/resolve?target=` | 解析 wiki 链接目标 |
| GET | `/auth/state` | 认证模式 / 是否需要初始化 / 当前用户 |
| POST | `/auth/setup` | 创建首个管理员 |
| POST | `/auth/login` | 登录 |
| POST | `/auth/logout` | 登出 |
| GET | `/auth/me` | 当前用户 |
| POST | `/auth/password` | 修改本人密码 |
| GET | `/auth/users` | 用户列表（仅管理员） |
| POST | `/auth/users` | 创建用户（仅管理员） |
| DELETE | `/auth/users/{id}` | 删除用户（仅管理员） |
| GET | `/public/notes` | 公开笔记列表（匿名） |
| GET | `/public/note?path=` | 公开笔记正文（匿名，非公开返回 404） |
| GET | `/ai/status` | AI 是否可用及模型名 |
| POST | `/ai/chat` | AI：`{mode: chat\|organize\|complete, messages, content}` |
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
| `/assets/{path...}` | 附件访问（需认证；内嵌前端资源优先，经静态处理器压缩/缓存） |
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
`code` ∈ `invalid` | `not_found` | `conflict` | `internal`，对应 HTTP 400/404/409/500。

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
                 /public[/:path] → PublicHomeView / PublicNoteView ； /login /setup /trash /capture
stores/
  workspace.ts   目录树、当前笔记、增删改查、搜索
  settings.ts    主题/字号/语言/压缩/专注/侧栏宽度（localStorage）
  auth.ts site.ts dialog.ts
components/
  App.vue        布局 + RouterView + DialogHost
  BrandMark.vue  品牌图形（笔记/文档 SVG）
  Sidebar.vue    操作 + 搜索 + 目录树 + 可拖拽宽度
  TreeNodeItem.vue 递归节点（行内操作浮层，不改变行高）
  EditorPane.vue Tiptap 编辑器 + 工具栏 + 笔记栏
  PublicShell.vue 公开页外壳（文档站风格）
  DialogHost.vue EmptyState.vue SlashMenu.vue TocPanel.vue HistoryPanel.vue LinksPanel.vue AiPanel.vue
editor/          extensions.ts / nodes.ts / lowlight.ts / slash.ts
markdown/        pipeline.ts / tasks.ts / math.ts / diagrams.ts / doc.ts / rules.ts / footnotes.ts
api.ts           类型化客户端（/api/v1，ApiError）
styles.css       暖色柔和设计令牌（明/暗）+ 组件样式
```

**构建分包**：路由懒加载使编辑器重依赖（TipTap / KaTeX / lowlight / Mermaid）不进首屏；
Vite 自动按需拆分（不使用 `manualChunks`，避免把预加载 helper 分进巨块）。

### 8.2 编辑与自动保存

- 内部为 Tiptap(HTML)，落盘为 Markdown：加载 `marked`(md→html)，保存 `turndown`+GFM(html→md)
- 防抖 700ms 自动保存，携带 `baseVersion`
- **保存 flush**：切换笔记、离开路由（`onBeforeRouteLeave`）、关闭页面（`beforeunload`）前强制落盘，避免防抖窗口丢数据
- **冲突处理**：409 时不清空内容，提示用户刷新，避免覆盖
- 图片粘贴/拖拽 → 上传 `/api/v1/assets` → 插入节点
- 表格：工具栏插入 3×3；光标在表内时显示增删行列工具条
- `/` 斜杠命令菜单（标题/列表/引用/代码块/表格/分割线）
- 笔记栏：大纲 / 历史版本 / AI 助手 / 分享（页面级动作，与排版工具栏分离）

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
| `index` | 迁移、Sync/Tree、中文中缀检索、快照转义、Upsert/Delete |
| `service` | 树组装、保存+检索、删除笔记/文件夹、冲突传播 |
| `server` | 完整生命周期、409、路径校验、附件上传/服务头、静态资源 gzip/304/SPA 回退、render 白名单 |
| `history`/`trash` | 版本快照与裁剪、回收站恢复/清理 |
| `archivex`/`sitegen` | ZIP 往返、静态站生成与搜索索引 |
| `config`/`logging`/`ai`/`openapi` | 配置解析、日志轮转、AI 客户端、OpenAPI 规范 |
| `mcp` | 初始化/发现、工具调用、认证、**协议版本协商**、`resultType`、工具面平铺（≥20 工具） |
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
| 存储型 XSS | 快照服务端转义；前端仅渲染已转义内容 |
| 附件 | `X-Content-Type-Options: nosniff`；非图片强制 `Content-Disposition: attachment`；SVG 不内联 |
| 上传体积 | `http.MaxBytesReader` + `ParseMultipartForm` |
| 认证 | 多用户（bcrypt + 会话 + 角色），`auth=multi` 时对 `/api/v1/*` 与附件强制认证；`auth=none` 为单用户模式 |
| 密钥 | AI/MCP/S3 密钥仅存服务端，接口永不回传 |
| 日志 | 结构化 JSON，无敏感内容 |

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
- [x] 浏览器快速捕获（Bookmarklet / 扩展）
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
- [ ] 非图片附件的插入 UI（现文件选择器仅 `image/*`）

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
| `OVERVIEW_RENDER` | `false` | 设为 `true` 变为公开只读文档站 |
| `OVERVIEW_EXPORT_DIR` | `_site` | `overview export` 输出目录 |
| `OVERVIEW_EXPORT_BASE` | `/` | `overview export` URL 前缀 |
| `OVERVIEW_MCP_TOKEN` | 空 | MCP Bearer 令牌；空且 auth=multi 时用会话令牌（协议 `2026-07-28`） |
| `OVERVIEW_AI_BASE_URL` | 空 | OpenAI 兼容基址（如 `https://api.openai.com/v1`）；空则禁用 AI |
| `OVERVIEW_AI_API_KEY` | 空 | AI 密钥 |
| `OVERVIEW_AI_MODEL` | `gpt-4o-mini` | 模型名 |
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

- **认证**（ADR-011/012）：`users`/`sessions` 表（迁移 0004），bcrypt 哈希；`AuthService` 负责 setup/login/logout/用户管理；HTTP 中间件对 `/api/v1/*` 与附件强制会话（Cookie 或 `Bearer`）；无用户时前端跳转 `/setup` 创建管理员。角色 `admin`/`member`，仅管理员可管理用户，且禁止删除最后一个管理员。
- **WebDAV**（ADR-013）：`/dav/` 映射 `data/notes`，`auth=multi` 时启用 Basic 认证；成功写入去抖 1s 后全量重建索引，外部编辑器保存的内容即可被搜索。
- **可移植附件**（ADR-014）：Markdown 中保存 `assets/2026/10/xxx.png`，SPA 渲染为重写为 `/assets/...`；`/assets/` 既是附件路由也是前端静态资源前缀，服务端优先命中内嵌资源。
- **i18n**（ADR-015）：`web/src/i18n` 提供 `t()` 与响应式 `locale`，持久化到 `localStorage`，顶栏下拉切换中/英；斜杠菜单、弹窗、工具栏等全部接入。

### 13.5 协作与智能（v0.6）

- **TOC**：`@tiptap/extension-table-of-contents` 生成标题锚点并跟踪滚动高亮；`TocPanel` 支持层级缩进、平滑跳转，工具栏可开关。
- **公开分享**（ADR-016）：frontmatter `public`（迁移 0005）；`/api/v1/public/*` 匿名可读，仅暴露公开笔记；前端 `/public` 列表与只读页，编辑器工具栏「分享」一键复制链接。公开页把 wiki 链接降级为纯文本，避免泄漏私有目标。
- **MCP**（ADR-017）：`POST /mcp` JSON-RPC，方法 `initialize`/`tools/list`/`tools/call`；工具 `notes_list/search/read/write/delete/links`；令牌认证（`OVERVIEW_MCP_TOKEN`，否则回退会话令牌）；工具错误按规范 in-band 返回。
- **AI**（ADR-018）：`internal/ai` 无依赖 OpenAI 兼容客户端；`AIService` 提供 `organize`/`complete`/`chat`；`/api/v1/ai/*` 未配置返回 503；前端 AI 面板支持对话、整理（替换）、补全（追加）、插入。

### 13.6 规模化与运维（v0.7）

- **增量索引**（ADR-019）：`Index.ReplacePrefix` 只重建某子树；`Delete`/`Move`/`ReindexPath` 均只影响相关路径。
- **文件监视器**：`internal/watcher` 用 fsnotify 递归监听 `data/notes`，去抖后对顶层子树增量重建——外部编辑器/Git/DAV 的改动即时可搜。
- **版本历史**（ADR-020）：每次保存前把旧文件快照到 `data/.history/<path>/`；提供列表、查看、回滚。
- **回收站**（ADR-020）：删除改为移动到 `data/.trash/<id>/`（含 `meta.json`），支持恢复与彻底删除。
- **附件孤儿清理**：扫描所有正文中的 `assets/...` 引用，列出并清理未被引用的附件。
- **S3 后端**（ADR-021）：`internal/s3store`（minio-go）实现 `AssetStore`，配置 `OVERVIEW_S3_BUCKET` 即启用。
- **PWA**（ADR-022）：manifest + service worker，仅缓存应用壳与哈希构建产物，离线可启动。
- **OpenAPI**（ADR-023）：`/api/v1/openapi.json` 与 `/api/docs`（无外部依赖）。

### 13.7 设置中心与设计系统（v0.8）

- **设置中心**（ADR-024）：顶栏齿轮打开，分三区：
  - 外观：主题（跟随系统/浅色/深色）、字号（紧凑/默认/舒适/大）、语言
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
  `--all` 导出全部笔记（默认仅 `public`），`--base`/`--title`/`--out` 可配。

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

### 13.12 性能与品牌（v0.11.0，ADR-032/033/034）

- **首屏性能**（ADR-032）：所有路由 `import()` 懒加载，编辑器重依赖不再进首屏；初始 JS 由 ~1.1MB
  降至 ~154KB（gzip ~57KB），页面总重 ~67KiB。
- **传输优化**（ADR-033）：`internal/server/static.go` 对静态资源 **gzip**（压缩结果内存缓存）、
  哈希资源 `Cache-Control: immutable`、壳文件 `no-cache` + ETag 304、SPA 回退；前端 `/assets/*`
  统一经该处理器（此前被上传路由绕过）。Lighthouse（移动端）由 80+ 提升到 **99**。
- **交互修复**：侧栏行内操作改为绝对定位浮层（消除悬浮引起的行高/截断抖动）；新增可拖拽宽度
  （持久化，双击复位）与文件名提示；加深文字令牌、强化激活态以恢复层级。
- **品牌**（ADR-034）：Logo 改为「页面 + 折角 + 文字行」的笔记/文档图形；调色板改为暖色柔和
  纸感灰 + 长春花靛蓝主色，成功/危险与代码高亮去饱和（`--hl-*` 变量）。

### 13.13 v0.11.0 验证记录

- `go build ./...`、`go vet ./...`、`golangci-lint run`（0 issues）、`go test ./...` 全绿
- 前端 `vue-tsc`、`npm test`（Vitest）、`vite build` 全绿
- CLI 端到端：`write/read/search/links/history/revision/restore/move/delete/trash-*`、
  `asset-upload/assets-orphans/assets-purge`、`export-zip`+`import` 往返、`build`（9 页）均通过
- MCP：`server/discover` 返回支持版本；现代请求带 `resultType`；不支持版本返回 `-32022`；
  `tools/list` ≥20 工具；`folder_create`/`move`/`history`/`trash_list` 实测通过
- Lighthouse（移动端模拟，登录页）：Performance **99**，FCP 1.3s / LCP 1.9s / TBT 30ms / CLS 0
- Docker：推送 `v*` 触发 GHCR 多架构镜像构建成功
