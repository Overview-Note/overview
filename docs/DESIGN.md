# Overview 设计文档

> 版本：v0.16.3（SPA 回退修正 · 404 体验 · 首页空状态新建入口）
> 更新日期：2026-10-07
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
| v0.13.1 | 侧栏与标题栏排版 | 统一侧栏字号层级（笔记/文件夹）；放大顶栏与标题栏高度，改善可读性 |
| v0.13.2 | 手机端适配 · 对象存储设置 · 修复 | 响应式手机端（P0+P1）：抽屉式侧栏（汉堡 + 遮罩，次要入口收进抽屉）、顶栏手机化、编辑器全宽、工具栏/note-bar 横向滚动、右侧面板改底部抽屉（大纲/反链/历史/AI）、移动端搜索浮层、触屏基础（tap-highlight、`touch-action`、`:active`、hover 门槛、16px 输入、≥40px 目标、`100dvh`/safe-area）、明暗 `theme-color`、PWA `orientation` + 192/512 PNG 图标；S3 对象存储可在设置页配置并运行时切换（新增 `internal/service/storage.go` 的 `SwitchableAssetStore`/`StorageService` 与 `internal/server/storage.go` 的 `GET/PUT /settings/storage`、`POST /settings/storage/test`，前端 `StorageSection.vue`；env 为初始值、secret 不回传、历史附件不迁移、保存失败不切换）；用户管理窄屏三行布局；AI「整理/补全」打开笔记后按钮失效与空笔记无提示修复 |
| v0.13.3 | 静态文档站与应用阅读态一致 | 新增共享内容样式 `internal/sitegen/content.css`（设计令牌含 `--hl-*` + `.tiptap-content` 全部阅读态规则），App `web/src/styles.css` 以 `@import` 复用、静态站 `<link>` 同一文件（单一真源）；Go 侧 `internal/sitegen/render.go` 用 goldmark + `x/net/html` 后处理把静态站 DOM 对齐编辑器契约（`pre.code-block > code.language-*`、`ul[data-type=taskList]`/`li[data-type=taskItem][data-checked]`、`.fn-ref`/`.fn-defs`、`.math-inline`/`.math-block[data-tex]`、`.mermaid[data-source]`、表格 `th>p`/`td>p`、`a.wiki-link`（未命中 `wiki-missing`）、`img loading/decoding`、标题 slug 与 `doc.ts` 一致、搜索索引纳入代码/数学/图表源文本、`assets/` 递归拷贝）；运行期用 `internal/sitegen/vendor.go` + `readonly-enhance.js` **按需加载与应用同版本**的 highlight.js/KaTeX/Mermaid；公开页渲染对齐编辑器契约（`web/src/markdown/doc.ts`、`PublicNoteView.vue` 用 `public-doc tiptap-content`、删除 `.public-doc` 字号覆盖）；默认主题 accent 派生色直通（`accentRamp` 对默认色返回与 `content.css` 一致的值） |
| v0.14.0 | 桌面应用 | 新增 Wails v3（`v3.0.0-beta.26`）原生桌面壳 `cmd/overview-desktop`：窗口指向本地服务、系统托盘、单实例、优雅退出、首启目录选择器、`.md` 文件关联、`overview://` 深链、文件拖拽、开机自启、更新提醒；抽取 `internal/app` 统一生命周期（进程内实例锁 `.overview.lock` + 端口文件 `.overview-port` + 监听失败回退随机 loopback 端口 + 优雅退出），headless 与桌面共用；`internal/config` 增加 `Desktop`/`DefaultDataDir()`/`EnableMCP`/`EnableDAV`（桌面默认 `127.0.0.1:5230`、`auth=none`、MCP/WebDAV 关闭，均可被环境变量覆盖）；`internal/server/local.go` 本地安全边界（Host 必须为回环名以抵御 DNS rebinding + 写请求同源校验替代 SameSite CSRF），`internal/server/desktop.go` 暴露 `/api/v1/desktop/settings|update`（headless 下一律 404，与不存在路径不可区分）；`internal/update` 只检测 GitHub Releases 版本、不下载不静默安装；前端新增 `web/src/views/settings/DesktopSection.vue`（仅桌面壳显示）；打包配置 `build/config.yml`（应用元数据/文件关联/URL scheme）；新增 `.github/workflows/desktop.yml` 跨平台构建矩阵（Windows/macOS arm64+amd64/Linux GTK4+GTK3）并在 `v*` tag 发布 Release 产物；`make build-desktop` / `package-desktop` 本机构建与归档 |
| v0.15.0 | 桌面↔服务器同步 · 桌面打磨 | **Phase 1 增量同步（类 Trilium）**。服务端：迁移 `0010_sync`（`notes.changed_seq` + `note_tombstones` + `settings` 的 `vault_id`/`change_seq`）、`GET /api/v1/sync/manifest`（`vaultId` + 笔记版本 + 文件夹，**强 ETag + 304**）、`PUT /api/v1/note` 支持 `raw:true`（**字节保真写入**、保留 frontmatter id/created、`baseVersion` 乐观并发且禁止无条件写）、`PUT /api/v1/assets/{path}`（按路径写附件、防穿越、S3→501）、`GET /api/v1/note?raw=1`；`service` 新增 `Manifest`/`VaultID`/`SaveNoteRaw`/`RawNote`/`RestoreAsset`，删除/移动记录墓碑。客户端：新包 `internal/sync`（`RemoteClient` 仅 https 或 loopback http、令牌独立文件；`State` 持久化 `<DataDir>/sync.json` + `.sync-token`；`Engine` 三方比对、pull/push/delete/move、冲突副本 `<name> (conflict-<device>-<ts>).md`、首同步补 id、指数退避、方向 `both\|pull\|push`、附件按路径、watcher 近实时）。桌面：`internal/app` 以 `SyncFactory` 装配引擎生命周期；`internal/server/sync.go` 暴露 `/desktop/sync`、`/run`、`/conflicts`、`/conflicts/resolve`（仅桌面 LocalOnly，headless 404）；`cmd/overview-desktop` 注入。前端：`web/src/views/settings/SyncSection.vue`（服务器地址/令牌/开关/方向/间隔/立即同步/状态卡/冲突区，仅桌面显示）。**桌面隐藏控制台**：Windows 桌面构建加 `-H=windowsgui`（GUI PE 子系统）、桌面日志默认 `<DataDir>/logs/desktop.log`、`internal/logging` 改为容错 fanout（GUI 下无 stdout 也能写文件）；修复同步 `plan()` 的 `canPull`/`canPush` 守卫与显式「立即同步」绕过节流 |
| v0.16.0 | 桌面引导 · owner 权限 · 同步补齐 | **无鉴权 = owner/admin**：`authMiddleware` 在 `!Auth.Required()` 时向每个请求注入 `owner`（`core.RoleAdmin`）用户，桌面/本地单用户安装可访问邮件/站点/存储/数据/用户/令牌等全部管理设置；前端 `auth.isAdmin = mode==="none" || user.role==="admin"` 同步放开设置分区。**桌面初始化与 vault 管理**：新增 `internal/config/desktop.go`（配置持久化 `<UserConfigDir>/Overview/desktop.json`、`HasVault`、`PrepareDataDir` 校验并建目录/测可写）；桌面壳首启引导（`runOnboarding`：默认/打开已有/新建 + 原生目录对话框，`--data-dir`/`OVERVIEW_DATA_DIR` 跳过）；设置页「桌面」分区可**更改数据目录**（`PUT /api/v1/desktop/vault`）、**打开数据目录**（`POST /api/v1/desktop/open-folder`）、**重启应用**（`POST /api/v1/desktop/restart`，重启子进程等待前任退出以释放单实例/vault 锁）。**同步补齐（Phase 2/3 局部）**：增量 `GET /api/v1/sync/changes?since=&sinceTs=&limit=`（`changes/tombstones/folderTombstones/latestSeq/latestTs/hasMore`，迁移 `0011_sync_folders` 的 `folder_tombstones` 与 `0012_folder_changes` 的 `folder_changes`）；实时 `GET /api/v1/sync/events`（SSE，25s 心跳，`http.ResponseController` 清写超时、`statusRecorder` 透出 `Unwrap`/`Flush`、`http.Server.BaseContext` 在 Stop 时取消流）；进程内 `service.ChangeBus` 广播（覆盖 Save/SaveRaw/Delete/Move/Mkdir/ReindexPath/Reindex，即写入/删除/移动/reindex/watcher/Mkdir）。客户端：`RemoteClient.Changes/Events`（SSE 自动重连 + 指数退避，401/403 停止）；`Engine` 订阅 SSE（debounce 500ms 触发 `SyncOnce`，先查 `/sync/changes` 头再决定）+ 轮询兜底 + `execMu` 单飞；游标 `state.cursor` 持久化；**空文件夹双向同步**（父→子建、子→父删、`folder_sync`/`folder_forget`，`remoteFolderEmpty`/`localFolderEmpty` 守卫）；目录删除实时（`folder_tombstones`）；超大附件跳过并记 warning 而非中断。**S3 显式路径写**：`s3store.Restore` 按 vault 相对 key 写入，S3 下附件同步不再 501。**错误码分类**：401→`unauthorized`、403→`forbidden`、405→`method_not_allowed`、502→`bad_gateway` |
| v0.16.1 | 桌面升级可见性 · 专注出口 | **静态资源校验修正**：`weakETag` 改为对文件**内容**计算 FNV-1a 哈希（原实现只哈希路径 + 字节长度），Vite 重建后字节数几乎不变的 `index.html` 不再算出相同 ETag 而被 304 长期沿用；缺失的静态资源（`assets/` 前缀或带扩展名）返回 **404**，不再回退 `index.html`，避免把 HTML 当 JS/CSS 返回（`internal/server/static.go`）。**桌面壳按版本破缓存**：窗口 URL 由 `/?desktop=1` 改为 `/?desktop=1&v=<version>`（`desktopURL`，`net/url` 转义），每次发版即新的 WebView2 缓存条目，升级后自动加载新前端（`cmd/overview-desktop/main.go`）。**专注模式可退出**：顶栏/侧栏隐藏时新增右下角低调浮动按钮 `.focus-exit`（i18n `focus.exit`，zh「退出专注」/en「Exit focus」），仍保留 Esc/F9 出口（`web/src/App.vue` + `styles.css`） |
| v0.16.2 | 侧栏树文字对比度 | 左侧栏文件树「多级菜单」去掉「层级越深越淡」：`.tree-row.folder`（含 depth-0/1/2/3）由 `--text-soft`/`--text-faint` 统一改为 **`--text`**；`.tree-row.note.depth-3` 由 `--text-soft` 改为 **`--text-muted`**；层级仍由 `padding-left` 缩进 + 文件夹「大写 + 600 字重」区分，hover/active 不变。仅改 `web/src/styles.css`，未动 `internal/sitegen/content.css` 的共享 token（ADR-087） |
| v0.16.3 | SPA 回退修正 · 404 体验 · 空状态入口 | **静态资源判定收窄**：`isAssetRequest` 由「带扩展名」改为「`assets/` 前缀 + 固定顶层白名单（icon.svg/icon-192.png/icon-512.png/manifest.webmanifest/sw.js/favicon.ico/robots.txt/sitemap.xml）」，修正 v0.16.1 过宽判定导致 `/note/<path>.md`、`/public/<path>.md` 刷新 404 而非回退 `index.html` 的回归；`assets/` 缺失仍 404 且非 HTML（ADR-088）。**404 体验**：前端新增 catch-all 路由 `/:pathMatch(.*)*` 与 `views/NotFoundView.vue`（此前未知路径渲染空白），文档站 render 模式下 `not-found` 重定向 `public-home`；`EditorPane.vue` 笔记 404 时显示一致的覆盖卡片；服务端新增自包含、亮暗自适应的 HTML 404 页 `internal/server/notfound.go`（用于 `index.html` 缺失 / `Static==nil` 分支），`/api/*` 仍 JSON、`assets/` 仍非 HTML（ADR-089）。**首页空状态入口**：`EmptyState.vue` 新增「新建笔记 / 新建文件夹」，复用侧栏流程与 i18n，专注模式隐藏侧栏时也能新建 |

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

**桌面壳（v0.14.0）** 复用同一套 `internal/app` 与 `server`，只是把「浏览器 HTTP 客户端」
换成一个内嵌 WebView 的原生窗口；业务逻辑仍只有一份：

```
┌────────────────────────────────────────────────────────────┐
│ overview-desktop (cmd/overview-desktop, Wails v3)           │
│   窗口(WebView) · 托盘 · 单实例 · 深链/拖拽 · 自启 · 更新提醒   │
│        │ 创建并驱动                                     │
│        ▼                                                 │
│ internal/app.App：实例锁 + 端口文件 + 监听 + 索引 + 优雅退出 │
│        │ 监听 127.0.0.1:<port>（端口被占则回退随机 loopback）│
│        ▼                                                 │
│ server（LocalOnly）：Host 回环校验 + 同源/CSRF 守卫 + /desktop/* │
└────────────────────────────────────────────────────────────┘
```

- **同一套生命周期**：`cmd/overview`（headless）与 `cmd/overview-desktop` 都调用
  `app.New`/`app.Start`/`app.Stop`；桌面壳额外创建 Wails 应用、托盘与桌面端点，
  headless 进程 `LocalOnly=false`，不注册 `/desktop/*`。
- **构建标签**：桌面壳源码带 `//go:build windows || darwin || desktop`。Windows/macOS
  默认即可构建；Linux 需要 `-tags desktop`（GTK/WebKitGTK 依赖 CGO），因此
  `CGO_ENABLED=0 go build ./...` 在纯后端环境仍可用。

**桌面↔服务器同步（v0.15.0，Phase 1）** 把桌面本地 vault 与一个自建服务器上的 vault
做双向收敛。服务端是中心，桌面端是持有完整副本的客户端；两者用同名 Markdown 文件
（frontmatter `id` 为身份）与内容版本做三方比对：

```
┌──────────────────────────┐        HTTPS(或回环 http)        ┌──────────────────────────┐
│ overview-desktop vault   │  GET /sync/manifest (强 ETag)   │ 自建 Overview 服务端      │
│  notes/ + assets/        │ ───────────────────────────────▶│  notes/ + overview.db    │
│  internal/sync.Engine    │  PUT /note (raw, baseVersion)    │  changed_seq / tombstones │
│  sync.json + .sync-token │  PUT /assets/{path}              │  /desktop/sync* (仅桌面)  │
│  (本地 watcher 触发)      │ ◀─────────────────────────────── │  server.SyncHooks         │
└──────────────────────────┘   三方比对 → pull/push/delete    └──────────────────────────┘
```

- **身份**：frontmatter `id`（ULID）跨设备稳定；路径仅作位置，移动/重命名不改变身份。
- **变更检测**：Phase 1 拉取完整 manifest 并比对内容版本；服务端 `notes.changed_seq`
  单调递增，为 Phase 2 的增量/墓碑同步预留（见 ADR-068）。
- **写入**：pull/push 都走 `raw:true` 保真通道，PATCH 级别的 `baseVersion` 保证不会
  静默覆盖（见 ADR-069）；冲突产生同级「冲突副本」交由用户裁决（见 ADR-070）。
- **控制面**：引擎通过 `server.SyncHooks` 暴露给 HTTP；`/desktop/sync*` 仅在桌面
  （`LocalOnly`）且挂载了引擎时注册，headless 服务器一律 404。

**实时与增量（v0.16.0）** 在 Phase 1 完整 manifest 之上补齐事件与增量通道：

```
服务端写路径 ──▶ service.ChangeBus.Publish(cursor)
                                   │
GET /api/v1/sync/events (SSE) ◀────┘   25s 心跳 · ResponseController 清写超时
GET /api/v1/sync/changes?since=&sinceTs=&limit=   增量：notes.changed_seq + 墓碑
GET /api/v1/sync/manifest                          全量清单（强 ETag，正确性兜底）

客户端 Engine：SSE 事件 debounce 500ms → 先查 /sync/changes 头判断是否已推进
             → SyncOnce（execMu 单飞）→ 推进 state.cursor；轮询为断线兜底
```

- **目录游标**：`folder_changes` 的序号与 `notes.changed_seq` 共用同一计数器（迁移
  `0011`/`0012`），因此**只含空文件夹**的目录创建/移动也能被增量流观察到；目录删除写入
  `folder_tombstones`（ADR-081）。
- **空文件夹同步**：客户端三方比对把文件夹也纳入 base/local/remote，父→子创建、子→父删除，
  删除前用 `remoteFolderEmpty`/`localFolderEmpty` 确认无内容。
- **事件与正确性解耦**：SSE 事件只表示「游标可能动了」，真正的收敛仍由 `SyncOnce` 的
  manifest 三方比对完成；事件丢失或端点缺失时轮询保证最终一致（ADR-079/080）。

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
| ADR-051 | 移动端适配采用「能力查询 + 抽屉/底部面板」，不维护独立移动版 | 复用同一套组件、路由与状态，成本最低；窄屏按断点重排而非重写 | ≤768px 侧栏变抽屉（汉堡开合 + 遮罩，次要入口收进抽屉），右侧面板变底部抽屉（大纲/反链/历史/AI），顶栏手机化、编辑器全宽、工具栏横向滚动、移动端搜索浮层；触屏基础：`:active` 反馈、`:hover` gate 到 `(hover:hover) and (pointer:fine)`、`touch-action:manipulation`、输入 16px、可点目标 ≥40px、`100dvh` 与 `env(safe-area-inset-*)`；明暗 `theme-color` 与 PWA 图标/朝向 |
| ADR-052 | 资产存储运行时可切换（`SwitchableAssetStore` + `StorageService`，持久化到 `settings` 表） | S3 原先只能靠环境变量且需重启，运维门槛高；运行时切换让管理员可在设置页自助配置 | 新增 `internal/service/storage.go`（`SwitchableAssetStore` 代理当前后端、`StorageService` 校验/持久化/切换）与 `internal/server/storage.go`；`OVERVIEW_S3_*` 作为初始值，设置页保存后以其为准（`s3_configured`）；切换即时生效但**不迁移历史附件**；`secretKey` 不回传、构建失败不切换 |
| ADR-053 | AI 面板内容随编辑器状态显式同步（`syncAiContent`） | 打开笔记时 `setContent` 触发的 update 被 `suppress` 吞掉，`aiContent` 未更新，导致「整理/补全」静默无反应 | `load()` 完成、编辑器内容变化、插入 AI 结果后均同步 `aiContent`；空笔记点「整理/补全」给出明确提示（`ai.emptyContent`）而非无反应 |
| ADR-054 | 阅读态样式单一真源 `internal/sitegen/content.css`（令牌 + `.tiptap-content` 规则），App `@import`、静态站 `<link>` 同一文件 | 应用/公开页/静态站曾各自维护同一套排版，任何一处改动都要人工同步，必然漂移；把「静态站 = 应用阅读态」变成构建保证 | `web/src/styles.css` 只保留应用外壳/组件/编辑器交互样式；阅读态规则增删只改 `content.css`；静态站样式随二进制 `go:embed` 分发，不再单独维护 |
| ADR-055 | 静态站 Go 侧结构对齐编辑器 DOM（`internal/sitegen/render.go`：GFM + `x/net/html` 后处理） | 此前 goldmark 默认输出（`AutoHeadingID`、无任务/脚注/数学结构）与编辑器契约不同，共享 CSS 无法命中 | 代码 `pre.code-block > code.language-*`、任务 `data-type`/`data-checked`、`.fn-ref`/`.fn-defs`、`.math-inline`/`.math-block[data-tex]`、`.mermaid[data-source]`、表格 `<p>` 包裹、`a.wiki-link`（未命中 `wiki-missing`）、`img loading/decoding`；标题 id 按 `doc.ts` 的 slug 生成，TOC 锚点一致；搜索索引纳入代码/TeX/Mermaid 源文本、`assets/` 递归拷贝 |
| ADR-056 | 静态站运行期增强：vendored 同版本 highlight.js/KaTeX/Mermaid，按需加载（`vendor.go` + `readonly-enhance.js`） | Go 侧无法完整复刻裸 JS 的渲染器；直接 CDN 又会引入版本漂移与离线依赖 | `vendor/*` 从 `web/node_modules` 原样拷贝，导出时仅写入页面需要的库；浏览器端渲染高亮/数学/图表/脚注跳转，`<body data-base>` 支持子路径；无数学/图表/代码的站不增加体积 |
| ADR-057 | 公开页/编辑器/静态站三态统一：公开页渲染复用编辑器契约（`doc.ts` 的 `code-block`/表格 `<p>`/任务 `data-type`/`wikiToHtml`），`PublicNoteView` 用 `public-doc tiptap-content` | 公开页此前用独立后处理（wiki 降级纯文本、无 `code-block`）且 `.public-doc` 覆盖 h1/h2/h3 字号，与编辑器/静态站不一致 | 三处产出同一 `.tiptap-content` DOM 契约，由 `content.css` 统一渲染；删除 `.public-doc` 字号覆盖与旧的任务列表降级规则 |
| ADR-058 | 默认主题 accent 派生色直通（`accentRamp` 对默认色返回与 `content.css` 一致的四元组） | 默认琥珀色经通用混色算法派生，与 `content.css` 里手写的 `--accent/-hover/-soft/-ring` 存在细微偏差，三态不一致 | 默认色直接返回常量色阶，自定义色仍走派生；应用与静态站默认外观完全一致 |
| ADR-059 | 桌面壳选型 **Wails v3**（`v3.0.0-beta.26`），而非 Electron/Tauri 或 Wails v2 | 壳需要系统托盘、文件关联/深链、拖拽、自启、原生通知与单实例，且要复用现有 Go 服务；Electron 引入 Node/Chromium 运行时违背「单二进制」定位，Tauri 需 Rust 工具链与独立前端构建 | Wails v3 提供 Go 侧窗口/托盘/事件/单实例/自启/通知，复用 `internal/app`；**Wails v2 未采用**：其多窗口/托盘/单实例/事件 API 无法满足深链与并发窗口需求，且 v2 对 Go 1.26 与新版 WebKitGTK 支持有限。壳源码以构建标签隔离，不污染 headless 构建 |
| ADR-060 | 抽取 `internal/app` 统一应用生命周期，headless 与桌面共用 | 桌面壳需要「先起服务拿到端口、再建窗口」，而旧 `cmd/overview` 把装配写在 `main`；两份实现会漂移 | `App` 负责装配 `store/index/service/server`、实例锁、端口文件、监听、初始索引、文件监视与优雅退出；`cmd/overview` 与 `cmd/overview-desktop` 只负责驱动 |
| ADR-061 | 端口策略：桌面固定 `127.0.0.1:5230`，被占用则回退随机 loopback 端口；自部署仍失败即退出 | 桌面应用不能因为 5230 被占而无法启动；自部署需要端口确定的失败可诊断 | `App.listen` 仅在 `Desktop` 下回退 `127.0.0.1:0`；启动后把实际地址写入 `<data>/.overview-port` 供 MCP/WebDAV/工具发现 |
| ADR-062 | 本地安全边界：`localGuard`（Host 必须为回环名 + 写请求同源校验）在两处守卫 | 桌面默认 `auth=none`，SameSite Cookie 不再提供 CSRF 保护；若监听 loopback 但 Host 可伪造，会被 DNS rebinding 攻击 | `isLocalHost` 拒绝非 `127.0.0.1`/`localhost`/`::1` 的 Host；`isStateChanging` 的请求要求 Origin/Referer 与 Host 同源（无 Origin 的非浏览器客户端放行）；`/desktop/*` 仅在 `LocalOnly` 下存在，headless 一律 404 |
| ADR-063 | 平台数据目录：Windows/macOS `~/Documents/Overview`，Linux `~/Documents/Overview` 或 `$XDG_DATA_HOME/overview` / `~/.local/share/overview`，其他平台回退用户 config 目录 | 桌面用户期望数据落在「文档」而非当前工作目录；Linux 遵循 XDG 约定 | `config.DefaultDataDir()` 按 GOOS 实现；首启若目录缺失/为空则弹出原生目录选择器，取消则用默认值 |
| ADR-064 | 桌面默认关闭 MCP 与 WebDAV、`auth=none`、监听 loopback，可用 `OVERVIEW_ENABLE_MCP`/`OVERVIEW_ENABLE_DAV`/`OVERVIEW_AUTH`/`OVERVIEW_ADDR` 覆盖 | 桌面是单用户本地应用，默认暴露 MCP/WebDAV 会扩大攻击面；同时保留高级用户显式开启的能力 | `config.Load` 在 `OVERVIEW_DESKTOP=true` 时切换默认值；自部署默认值不变 |
| ADR-065 | 单实例与 SQLite 多进程：进程内实例锁（`.overview.lock`）保证每个数据目录只有一个服务进程；Wails 单实例（UniqueID `com.overview.app`）保证只有一个窗口 | SQLite 与本地文件不适合多进程并发写；重复启动应聚焦已有窗口而非再起一个服务 | `internal/app.AcquireInstanceLock` 用 OS 文件锁（Windows `LockFileEx` / Unix `flock`）；第二次启动经 Wails 转发 argv（文件/深链）给首实例并聚焦窗口 |
| ADR-066 | 更新只为「提醒」：`internal/update` 查 GitHub Releases 比较语义版本，发现新版本时发系统通知/托盘提示，并暴露 `/api/v1/desktop/update`；**不下载、不静默安装** | 静默自更新涉及签名、权限与失败回滚，风险高；桌面平台各自有安装器机制 | 启动后 5s 与每 24h 检查一次；只在版本变化时通知一次；用户点通知打开 Release 页面自行下载 |
| ADR-067 | 同步拓扑为「整个 vault 的镜像」：桌面客户端 ↔ 自建服务端，服务端为中心，客户端持一个 API 令牌 | Phase 1 目标是让桌面端拥有一份可离线编辑、随后收敛的完整副本；单机客户端↔自建服务端模型最贴合本地优先定位，避免引入每篇笔记的共享/权限模型 | 令牌等价于对该 vault 的完整读写；**多用户部署下所有用户共享同一 vault、桌面端同步全部笔记（无按用户 ACL）**；不支持多服务器/多主拓扑 |
| ADR-068 | 变更检测分两期：Phase 1 用完整 manifest（强 ETag + 304），`notes.changed_seq` 作为二期增量与删除追踪的地基 | 完整清单实现简单、正确性易验证；大库每次全量传输开销大，需要演进空间 | `notes.changed_seq` 单调递增（`settings.change_seq` 持久化计数器，全量重建时 id/path/version 未变的笔记沿用旧序号，子树替换/移动强制新序号）；`VaultManifest.ETag = sha256(vaultId:maxSeq:count)` 强校验；`note_tombstones` 记录删除供二期使用 |
| ADR-069 | 同步写入走保真 raw 通道：`PUT /note {raw:true, content}` 原样落盘，且强制 `baseVersion` | 普通保存会重新序列化 frontmatter，可能改写 id/created/字段顺序，破坏「同一文件字节哈希一致」的同步基础 | 写入字节 = 客户端字节，`version = sha256(bytes)`；保留客户端 id 与 created；`baseVersion` 必填（`*` 新建 / 指定版本更新，缺失即 400），绝不无条件覆盖；不支持 raw 的存储返回 501 |
| ADR-070 | 冲突不覆盖：本地版本另存为同级冲突副本，远端原件保留，交由用户裁决 | 自动合并 Markdown 易丢内容，静默覆盖更不可接受 | 冲突文件名 `<name> (conflict-<device8>-<ts>).md`、带新 ULID，作为独立笔记推送；`GET /desktop/sync/conflicts` 列出，`resolve` 可 `keep local`（本地版覆盖远端原件并删副本）或 `keep remote`（删本地副本） |
| ADR-071 | 以 frontmatter `id` 作为跨设备身份；首同步为缺 id 的笔记补 ULID | 路径随移动/重命名变化，内容哈希随编辑变化，都不能作稳定身份 | 同步扫描时就地回写缺失的 id；同 id 多文件仅保留其一并告警；冲突副本重新生成 id、实体笔记保留原 id |
| ADR-072 | 删除与移动记录墓碑（`note_tombstones`），三方比对区分「删除 vs 编辑」 | 仅凭「一方缺失」无法判断是删除还是尚未同步的新建；需要记录曾存在 | Delete/Move 对消失的笔记写墓碑（`id`/`path`/`deleted_at`/`device`）；本地未改而远端缺失 → 同步删除；远端缺失但本地已改 → 视为本地编辑并重建；本地删除而远端已改 → 拉回远端；本地删除且远端未改 → 推送删除 |
| ADR-073 | 同步方向 `both`/`pull`/`push`，在 `plan()` 中以 `canPull`/`canPush` 守卫 | 用户可能只想单向镜像（桌面只读服务端，或只把本地备份到服务端）；无守卫会在禁止方向误写 | 守卫覆盖文件夹创建、pull/push、移动与删除：禁止方向的一侧永不被写；显式「立即同步」与轮询共享同一计划；单向下的冲突由后写侧直接胜出（pull-only 远端优先、push-only 本地优先），不生成冲突副本 |
| ADR-074 | 令牌不落盘明文于状态文件/日志，且非回环必须 HTTPS | `sync.json` 可能被备份/导出；明文令牌一旦泄漏等于整库读写 | API 令牌单独存 `<DataDir>/.sync-token`（0600，原子写），`sync.json` 只存 serverURL/vaultId/deviceId 等；日志与错误不含令牌；`RemoteClient` 仅接受 `https://` 或回环（`localhost`/`127.0.0.1`/`::1`）`http://`，其余普通 HTTP 直接拒绝 |
| ADR-075 | 附件按 vault 相对路径同步（`PUT /assets/{path}` / `GET /assets/{path}`） | 普通上传会生成带 ULID 的新名，无法与 Markdown 中的相对引用对应 | 服务端写路径前校验（拒绝空、绝对路径、`..`、反斜杠/NUL/冒号）并受上传体积限制；后端不支持显式路径恢复时（S3）返回 501，客户端跳过并告警；push 按内容哈希去重，pull 仅补本地缺失 |
| ADR-076 | Windows 桌面壳以 GUI PE 子系统（`-H=windowsgui`）构建；桌面日志落 `<DataDir>/logs/desktop.log`；日志写出改为容错 fanout | GUI 子系统下进程没有控制台，stdout 不可写；旧 `io.MultiWriter` 遇到不可用 writer 会中断整条写出 | Makefile `DESKTOP_LDFLAGS` 与 CI 矩阵按平台附加 `-H windowsgui`；`OVERVIEW_LOG_FILE` 未设时桌面默认写 `logs/desktop.log`；`internal/logging` 的 `fanout` 逐个 writer 写出、某个失败不影响其它，保证无控制台时仍能落盘诊断 |
| ADR-077 | 无鉴权模式视调用者为 **owner（管理员）**：`authMiddleware` 在 `!Auth.Required()` 时注入 `core.User{ID:"owner", Role:admin}` | 桌面与单用户自部署默认 `auth=none`，但设置页的邮件/站点/存储/数据/用户/令牌分区与 `/settings/*` 端点均按管理员门禁；此前无会话会导致这些入口 401/隐藏 | 无鉴权安装等价于「唯一 owner」，管理端点全部可达；前端 `auth.isAdmin` 同步为 `mode==="none" || role==="admin"`，保持一致。多用户模式不受影响（仍走会话/令牌认证） |
| ADR-078 | 桌面配置独立于 vault：`<UserConfigDir>/Overview/desktop.json` 只存 `{dataDir}`，通过 `config.Load/SaveDesktopConfig` 原子读写；`HasVault(dir)` 以 `notes/` 或 `overview.db` 判定既有 vault，`PrepareDataDir(dir)` 校验类型/建目录/测可写 | 桌面壳需要**在 vault 存在之前**记住 vault 位置，存在「先有鸡还是先有蛋」；把配置放进 vault 又无法用它定位 vault | 首启引导（默认/打开已有/新建 + 原生目录对话框）结果落盘，后续启动直接复用；`--data-dir`/`OVERVIEW_DATA_DIR` 显式指定时跳过引导且不允许在界面更改；更改 vault 仅写配置，需重启（`POST /desktop/restart`）生效，重启子进程等待前任 PID 退出以释放实例锁与 vault 锁 |
| ADR-079 | 实时变更用进程内 `service.ChangeBus` + SSE `GET /api/v1/sync/events`：写路径（Save/SaveRaw/Delete/Move/Mkdir/ReindexPath/Reindex）广播当前游标；SSE 客户端 25s 心跳、`ResponseController.SetWriteDeadline(time.Time{})` 清服务端写超时、`statusRecorder` 实现 `Unwrap`/`Flush`、`http.Server.BaseContext` 在 Stop 时取消 | 轮询有延迟且大库开销高；长连接会被 `WriteTimeout: 120s` 与优雅退出卡住；`statusRecorder` 默认不暴露底层 writer，`ResponseController` 拿不到连接 | 事件驱动近实时（默认再叠加轮询兜底）；`ChangeBus` 非阻塞扇出（慢订阅者只丢中间事件、下次 drain 取最新游标）；headless 与桌面共用，SSE 走正常认证 |
| ADR-080 | 增量同步端点 `GET /api/v1/sync/changes?since=&sinceTs=&limit=`：`since` 为**独占**笔记序号游标、`sinceTs` 为**独占**墓碑时间戳、`limit` 默认 500/上限 5000；返回 `{latestSeq, latestTs, hasMore, changes[], tombstones[], folderTombstones[]}` | 完整 manifest 每次传输全部笔记元数据，大库开销大；客户端需要能分页追赶且能感知删除 | 游标推进由客户端负责（把最后一条 change 的 `seq` 作为下次 `since`，把 `latestTs` 作为 `sinceTs`，`hasMore` 为真则继续）；`changes[].op` 目前恒为 `upsert`；旧服务端无此端点时客户端回退 manifest |
| ADR-081 | 目录变更序号与空文件夹同步：迁移 `0011` 的 `folder_tombstones(path, deleted_at, device)` 与 `0012` 的 `folder_changes(path, changed_seq)`；`RecordFolderChange` 从**共享计数器**分配序号，`RecordFolderTombstone` upsert 删除标记；`LatestSeq` 取笔记与目录序号的最大值 | 只含空文件夹的目录此前无任何笔记，「新建空文件夹」不会推进任何游标，客户端观察不到；目录删除也需可传播 | 空文件夹与笔记共用同一游标空间，`/sync/changes` 能报告目录创建/移动/删除；客户端 `plan()` 据 `st.Folders`（base）、本地、远端三方做 `mkdir`（父先子后）/`folder_delete`（子先父后）/`folder_sync`/`folder_forget`，`remoteFolderEmpty`/`localFolderEmpty` 确保删除不误伤有内容的目录 |
| ADR-082 | S3 后端实现 `core.AssetRestorer`（`s3store.Restore` 按 vault 相对 key 直接 `PutObject`），不再对按路径写附件返回 501 | 附件同步与 ZIP 导入都需要「原样写回相对路径」的能力；此前 S3 缺失该能力导致附件整体跳过 | S3 下附件可正常双向同步与导入；`Restore` 不做日期分区命名，key 即 vault 相对路径（与 Markdown 引用一致） |
| ADR-083 | 错误码分类补齐：401→`unauthorized`、403→`forbidden`、405→`method_not_allowed`、502→`bad_gateway`（新增/细化 `statusCode`） | 客户端与测试需要按语义区分未认证、无权限、方法不允许与上游失败；原先这些状态只能落到 `internal` 泛化码 | 错误响应的 `error.code` 更精确；对既有 `invalid`/`not_found`/`conflict`/`too_large`/`not_supported`/`rate_limited`/`unavailable`/`email_not_verified` 映射保持不变 |
| ADR-084 | 静态资源弱 ETag 改为对文件**内容**计算 FNV-1a 哈希（原实现只哈希路径 + 字节长度）；缺失的静态资源（`assets/` 前缀或带扩展名）返回 404，不再回退 `index.html` | 内容哈希才能反映内容变化：Vite 重建的 `index.html` 字节长度几乎不变，旧的「路径 + 长度」校验器会算出相同 ETag，客户端 304 命中并长期沿用旧壳；旧壳引用的哈希资源又被 `immutable` 缓存一年，导致升级后前端 bundle 停留在旧版本。把 `index.html` 当作 JS/CSS 返回还会触发 MIME 解析错误 | 内容变化即新 ETag，`no-cache` 壳文件正确重取；只有无扩展名的客户端路由仍回退 `index.html`，缺失的资源明确 404，便于发现前后端版本不一致；`internal/server/static_test.go` 覆盖内容哈希与 404 行为 |
| ADR-085 | 桌面壳窗口 URL 追加构建版本 `?v=<version>`（`desktopURL`，`net/url` 转义） | WebView2 的 HTTP 缓存按 URL 区分条目；只用 `/?desktop=1` 时同一 URL 会复用旧缓存，配合被 `immutable` 缓存的旧资源，升级后可能一直加载旧前端 | 每个发行版本是全新缓存条目，升级后首次打开即拉取新 bundle；`?desktop=1` 标记保持不变，前端仍据此识别桌面壳并跳过 Service Worker 注册 |
| ADR-086 | 专注模式提供**可见出口**：右下角低调浮动按钮 `.focus-exit`（i18n `focus.exit`，zh「退出专注」/en「Exit focus」），点击调用 `settings.setFocus(false)`；Esc/F9 快捷键仍可用 | 专注模式隐藏顶栏/侧栏，此前只有键盘出口，触屏或鼠标用户无处退出，形成「无路可退」的死角 | 按钮 `z-index:90` 位于内容之上、全屏遮罩（`z-index:100`）之下；仅在专注模式且非 plain（render/公开）时渲染；`prefers-reduced-motion` 下不引入额外动画 |
| ADR-087 | 侧栏文件树的多级菜单**不再靠降低不透明度表达层级**：`.tree-row.folder`（含 depth-0/1/2/3）统一用 `--text`，`.tree-row.note.depth-3` 用 `--text-muted`；层级改由 `padding-left` 缩进与文件夹「大写 + 600 字重」承载，hover/active 不变 | 原实现「层级越深越淡」（`--text-soft` → `--text-faint`，浅色下 #9b9b9b/#b7b7b3）使深层文件夹/笔记与背景对比度不足，长目录树可读性差；用**对比度**编码深度还会与 hover/active 态、暗色主题相互干扰 | 所有层级的树文字都达到主/次文本对比度（浅色 `--text` #37352f、笔记 `--text-muted` #6b6b6b），缩进与字重仍清晰区分层级；仅改应用侧 `web/src/styles.css`，**未动** `internal/sitegen/content.css` 的共享 token，静态站与公开页不受影响 |
| ADR-088 | 静态资源判定收窄为「`assets/` 前缀 + 固定顶层文件白名单」，其余未命中路径一律回退 SPA；`assets/` 缺失仍返回非 HTML 404 | v0.16.1（ADR-084）以「带扩展名」判定资源，把 `/note/<path>.md`、`/public/<path>.md` 这类带扩展名的客户端路由误判为静态资源，刷新时返回 404 而非回退 `index.html`；但白名单仍须让缺失的真实资源（`assets/*`、`favicon.ico` 等）明确 404，不被 `index.html` 以 JS/CSS 内容类型掩盖 | 只有 `assets/*` 与固定顶层文件（icon.svg / icon-192.png / icon-512.png / manifest.webmanifest / sw.js / favicon.ico / robots.txt / sitemap.xml）视为资源；其余路径无论是否带扩展名都回退 SPA，`/note`、`/public` 下的 `.md` 深链接刷新恢复；`internal/server/static_test.go` 覆盖「带扩展名路由回退 SPA」与「白名单缺失 404」 |
| ADR-089 | 404 体验三端统一：前端 catch-all vue 路由 + 笔记缺失覆盖层 + 服务端自包含 HTML 404 页；内容协商按路径区分（`/api/*` JSON、`assets/` 非 HTML、文档路由 HTML） | 未知客户端路径此前无匹配路由而渲染空白页，笔记不存在时只有底部错误文本；未构建前端的服务端只回 `text/plain`，对访问者既不友好也不一致 | 新增 `web/src/views/NotFoundView.vue` 与置于路由末尾的 `/:pathMatch(.*)*`；文档站 render 模式把 `not-found` 重定向 `public-home` 而非应用 404；`EditorPane.vue` 在 `ApiError.status===404` 时显示同款卡片；`internal/server/notfound.go` 的 `writeNotFoundPage` 用于 `index.html` 缺失 / `Static==nil` 分支，内联 CSS、跟随系统明暗、无外部依赖 |

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
- 迁移清单（0001–0012）：

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
| `0010_sync` | `notes.changed_seq`（单调变更序号）+ `note_tombstones`（删除墓碑，含 `deleted_at` 索引） |
| `0011_sync_folders` | `folder_tombstones(path PK, deleted_at, device)` + `deleted_at` 索引（空文件夹删除/移动的墓碑） |
| `0012_folder_changes` | `folder_changes(path PK, changed_seq)` + `changed_seq` 索引（空文件夹创建/移动的序号，与笔记共计数器） |

- `users`（迁移 0004 + 0008）：`id`、`username`（唯一）、`password_hash`、`role`（`admin`/`member`）、`created`、`updated`、`email`（唯一，非空时）、`status`（`active`/`invited`）、`email_verified`。受邀账号 `password_hash` 为空串（bcrypt 永不匹配），必须先接受邀请设置密码。
- `user_tokens`（迁移 0008）：`id`、`user_id`、`purpose`（`invite`/`reset`/`verify`）、`token_hash`（sha256，唯一）、`created`、`expires`、`used`。令牌单次消费：读取-校验-写入在同一事务内完成（连接池限单连接以串行化）。
- `settings`（迁移 0006）：`key`/`value` 字符串键值表，存 `ai_*`、`mail_*`、`s3_*`、`registration_enabled`、`login_*` 等运行时配置。其中 `s3_endpoint/region/access_key/secret_key/bucket/use_ssl/public_url` 与 `s3_configured` 由设置页「对象存储」写入（见 ADR-052）；`s3_configured=true` 表示运行时配置已保存，重启时优先于环境变量初始值。
- `api_tokens`（迁移 0007）：`id`、`name`、`prefix`、`token_hash`、`created`、`last_used`、`expires`。
- `sessions`（迁移 0004）：`token`、`user_id`、`created`、`expires`。
- `ai_tool_audit`（迁移 0009）：`id`、`run_id`、`user_id`、`username`、`role`、`step`、`tool`、`args`（脱敏后 JSON）、`result_summary`、`status`（`ok`/`error`/`denied`/`rejected`）、`destructive`、`note_version`、`trash_id`、`revision_id`、`created_at`；按 `run_id` 与 `(user_id, created_at)` 建索引。写入前由 `sanitizeAuditArgs` 脱敏：`key/token/password/secret/authorization` → `[redacted]`，`body/content` → `sha256:<hash> (len N)`，截断至 2 KB。
- `notes.changed_seq`（迁移 0010）：索引内的单调变更序号。每次写入/新建分配递增序号；全量重建（`Sync`）时 id/path/version 未变的笔记沿用旧序号，子树替换（`ReplacePrefix`，含移动）强制分配新序号；计数器持久化在 `settings('change_seq')`，保证笔记删除后序号也不复用，manifest 的强 ETag 由 `vaultId + maxSeq + 笔记数` 派生（ADR-068）。
- `note_tombstones`（迁移 0010）：`id`(PK)、`path`、`deleted_at`、`device`；按 `deleted_at` 建索引。`service.Delete`/`Move` 对消失的笔记调用 `Index.RecordTombstone` 记录墓碑（`service.recordTombstones`），供同步客户端协调删除；v0.16.0 起由 `GET /sync/changes` 以 `sinceTs` 增量返回（ADR-072/080）。
- `folder_tombstones`（迁移 0011）：`path`(PK)、`deleted_at`、`device`；按 `deleted_at` 建索引。`service.Delete`/`Move` 对消失的目录（含空目录）调用 `Index.RecordFolderTombstone`（`service.recordFolderTombstones`）upsert 墓碑，使「目录删除」可随实时/增量流传播（ADR-081）。
- `folder_changes`（迁移 0012）：`path`(PK)、`changed_seq`；按 `changed_seq` 建索引。`service.Mkdir`/仍存在的目录移动调用 `Index.RecordFolderChange`，从 `settings('change_seq')` **共享计数器**分配序号并 upsert，使不含任何笔记的空文件夹也能推进客户端游标；`Index.LatestSeq` 取 `notes.changed_seq` 与 `folder_changes.changed_seq` 的最大值（ADR-081）。
- `settings` 另存同步库标识 `vault_id`（首次 `Index.VaultID` 生成并持久化的 ULID），随 `GET /sync/manifest` 返回，用于客户端校验连接的是同一个库（ADR-067）。
- 领域模型新增 `NoteMeta.Version`（索引里的内容哈希）、`ManifestNote`、`VaultManifest`（`vaultId`/`generatedAt`/`etag`/`notes`/`folders`）、`Tombstone`（见 `internal/core/model.go`）。
- v0.16.0 领域模型补充（`internal/core/model.go`）：`FolderTombstone{path, deletedAt, device}`、`SyncChange{id, path, version, seq, op}`（`op` 目前恒为 `upsert`）、`SyncChanges{latestSeq, latestTs, hasMore, changes[], tombstones[], folderTombstones[]}`；端口 `core.ChangeLog`（`internal/core/ports.go`）暴露 `LatestSeq`/`ChangesSince`/`TombstonesSince`/`FolderTombstonesSince`/`LatestTombstoneTime`/`RecordFolderTombstone`/`RecordFolderChange`，由 `internal/index` 实现（可选能力，未实现时增量端点返回 `not_supported`、目录标记为 no-op）。

> 从旧版（v0.1，无迁移表且 `notes` 缺列）升级：索引可直接删除重建，内容文件不受影响。

---

## 5. 后端分层

### 5.1 包职责

| 包 | 职责 | 关键类型/方法 |
| --- | --- | --- |
| `internal/core` | 领域模型、端口接口、哨兵错误 | `Note` `TreeNode` `User` `UserToken` `APIToken` `NoteRepository` `Index` `UserStore` `AssetStore` `AssetRestorer` `TokenStore` `ErrNotFound/ErrConflict/ErrInvalid/ErrForbidden/ErrNotSupported` |
| `internal/service` | 应用用例编排 | `Tree` `GetNote` `SaveNote` `Delete` `Move` `Mkdir` `Search` `Upload` `Reindex`、`AuthService` `MailService` `SiteService` `TokenService`、`FetchPreview`（捕获）、`StorageService`/`SwitchableAssetStore`（运行时资产后端）、`Changes`（增量变更流）、`ChangeBus`/`CurrentChangeCursor`（实时广播） |
| `internal/store` | 文件系统实现 | 原子写、版本校验、`List`(结构)、`Walk`(全量) |
| `internal/index` | SQLite 实现 | 迁移、`Upsert` `Sync` `Tree` `Search`、`ChangeLog`（`changes.go`：`LatestSeq`/`ChangesSince`/`TombstonesSince`/`FolderTombstonesSince`/`LatestTombstoneTime`/`RecordFolderChange`/`RecordFolderTombstone`） |
| `internal/textproc` | 文本处理 | `Tokens` `Segment` `Snippet` |
| `internal/server` | HTTP 适配 | 路由、中间件、错误映射、ETag、`agent.go`（`/ai/agent*`）、`storage.go`（`/settings/storage*`）、`sync.go`（`/sync/changes`、`/sync/events`、`/desktop/sync*`）、`desktop.go`（`/desktop/vault\|restart\|open-folder\|settings\|update`） |
| `internal/markdown` | frontmatter 解析/序列化 | `Parse` `Document.String` |
| `internal/sitegen` | 静态文档站生成（HTML + 客户端搜索） | `Generate`、`render.go`（结构对齐编辑器的 DOM）、`vendor.go`（按需内置运行期增强）、`content.css`（共享阅读样式） |
| `internal/tools` | **唯一能力源**（原 MCP 工具实现迁入） | `Defs` `DefsOpenAI` `Exec` `Risk` `Allows` `Preview` `Known`、`Set` |
| `internal/agent` | AI 智能体：有界工具循环、会话、确认、审计 | `Agent` `Run` `Confirm` `Stop` `SystemPrompt`、`Session`/`Result`/`Step` |
| `internal/ai` | OpenAI 兼容客户端（chat + tool calling） | `Chat` `ChatTools` `AgentMessage` `Tool` `ToolCall` `ErrToolsUnsupported` |
| `internal/mcp` | MCP JSON-RPC 适配器（薄） | `Server.ServeHTTP`（工具面委托 `internal/tools`） |
| `internal/app` | 应用装配与生命周期（headless 与桌面壳共用） | `App` `New`/`NewWithOptions`/`Start`/`Stop`、`AcquireInstanceLock`、端口文件、`ResolveOrImport` |
| `internal/update` | GitHub Releases 版本检测（只报告，不下载/安装） | `CheckLatest` `Compare` |
| `internal/config` | 环境配置 + 桌面配置 | `Load`、`WithDataDir`、`DefaultDataDir`、`EnsureDirs`、`DesktopConfig`/`LoadDesktopConfig`/`SaveDesktopConfig`（`<UserConfigDir>/Overview/desktop.json`）、`HasVault`/`PrepareDataDir`（vault 校验与建目录） |
| `internal/sync` | 桌面↔服务器同步引擎（Phase 1 + 实时/增量补齐） | `RemoteClient`（https/回环校验、Bearer、令牌不落日志、`Changes` 增量、`Events` SSE 重连）、`State`（`sync.json`(含 `cursor`) + `.sync-token`）、`Engine`（三方比对、pull/push/delete/move、**空文件夹双向同步**、冲突副本、方向、附件、指数退避、**SSE 事件 debounce + 轮询兜底 + 单飞**）、`Direction`、`SyncHooks` 实现 |

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

**同步一次（桌面 → 服务器）**
```
Engine.SyncOnce                       （execMu 单飞，轮询与 SSE 触发串行）
  → scanLocal (repo.Walk；缺 id 的笔记就地回写 ULID)
  → RemoteClient.Manifest("")          → 全量清单（强 ETag 不覆盖目录成员，故不条件请求）
  → RemoteClient.Changes(since=cursor.LatestSeq, sinceTs=cursor.LatestTs, 1)
                                        → 增量头（latestSeq/latestTs/folderTombstones）
  → plan(base=state / local / remote，叠加 folder tombstones)
      文件夹 creates（父先子后）→ folderSync → pulls → pushes → note deletes
        → folderDeletes（子先父后，remoteFolderEmpty/localFolderEmpty 守卫）→ forgets
      冲突 → actConflict；方向守卫 canPull/canPush
  → execute: pull=GetNoteRaw→WriteRaw；push=repo.Raw→PutNoteRaw(baseVersion)
      delete=repo.Delete / client.DeleteNote；move=repo.Move / client.Rename
      文件夹=repo.Mkdir / client.Mkdir / repo.Delete / client.DeleteNote
      附件 push 按 sha256 去重 → PUT /assets/{path}（S3 走 Restore；超大跳过记 warning）
  → state.Mutate(notes/folders/assets/lastSyncAt/cursor)；冲突副本写盘并登记
```

**实时事件（桌面 ← 服务器，v0.16.0）**
```
Engine.Run
  → go runSSE(ctx)  订阅 GET /api/v1/sync/events（SSE，Bearer）
      onEvent(latestSeq, latestTs)
        → debounce 500ms（合并突发）
        → remoteChanged? 查 GET /sync/changes?since=cursor&sinceTs=cursor&limit=1
            已推进 → SyncOnce；未推进 → 跳过
  轮询定时器（interval，默认 60s）始终保留为断线兜底
  本地 watcher → NotifyDirty → 触发同一条路径
```
错误处理：`Events` 内部自动重连（500ms 起指数退避至 30s），401/403 视为永久失败交回
`runSSE` 等重配置；服务端每 25s 发 `: ping` 心跳并由 `ResponseController` 清除写超时
（ADR-079）。

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
| GET | `/note?path=&raw=1` | 返回笔记完整 Markdown 字节（含 frontmatter），`Content-Type: text/markdown`，ETag = 内容版本（render 模式仅公开笔记） |
| PUT | `/note` | 保存，body `{path, body, baseVersion}`，或 `If-Match`；`{raw:true, content, baseVersion}` 走**保真写入**（字节原样落盘、保留 frontmatter id） |
| DELETE | `/note?path=` | 删除笔记/文件夹 |
| POST | `/folder` | 新建文件夹 `{path}` |
| POST | `/rename` | 重命名 `{from, to}` |
| GET | `/search?q=&limit=&offset=` | 全文检索 |
| POST | `/assets` | 上传附件（multipart `file`，任意类型） |
| PUT | `/assets/{path...}` | 按 vault 相对路径写附件（同步用：防穿越、受上传体积限制；后端不支持显式路径恢复时 501 `not_supported`） |
| GET | `/assets/{path...}` | 访问附件（nosniff，非图片强制下载） |
| GET | `/sync/manifest` | 同步清单：`vaultId` + 全部笔记（id/path/version/updated/size/public）+ 文件夹；强 `ETag`，`If-None-Match` 命中返回 304 |
| GET | `/sync/changes?since=&sinceTs=&limit=` | **增量**变更流：`since`（独占笔记序号，默认 0）、`sinceTs`（独占墓碑时间 RFC3339）、`limit`（默认 500，上限 5000）；返回 `{latestSeq, latestTs, hasMore, changes[], tombstones[], folderTombstones[]}`；`since`/`limit` 非整数返回 400 `invalid` |
| GET | `/sync/events` | **实时**变更流（SSE，`text/event-stream`）：连接即发当前游标，随后每个写事件发 `event: change`，空闲每 25s 发 `: ping` 心跳；`ResponseController` 清除写超时以支持长连接 |
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
| GET | `/settings/storage` | 读取对象存储配置：`endpoint` `region` `accessKey` `bucket` `useSSL` `publicURL` `hasSecret` `enabled`（仅管理员，`secretKey` 永不回传） |
| PUT | `/settings/storage` | 保存对象存储配置并切换后端（仅管理员；`bucket` 留空回退本地存储；校验/构建失败返回可诊断错误且**不切换**） |
| POST | `/settings/storage/test` | 用请求体（缺省则用已存配置）探测连接，**不改动当前后端**（仅管理员） |
| GET | `/assets/orphans` | 未被引用的附件列表 |
| POST | `/assets/orphans/purge` | 清理孤儿附件 |
| GET | `/history?path=` | 笔记历史版本列表 |
| GET | `/history/revision?path=&id=` | 某版本内容 |
| POST | `/history/restore` | 回滚到某版本 |
| GET | `/trash` | 回收站列表 |
| POST | `/trash/restore` | 从回收站恢复 |
| DELETE | `/trash?id=` | 彻底删除 |
| GET | `/openapi.json` | OpenAPI 规范（公开） |
| GET | `/desktop/settings` | 桌面壳设置 `{autostart, dataDir, version}`（仅桌面/LocalOnly；headless 返回 404） |
| PUT | `/desktop/settings` | 更新桌面壳设置（目前仅 `autostart`，省略字段保持不变；仅桌面） |
| PUT | `/desktop/vault` | 更改数据目录 `{dataDir}`：非空即校验并持久化；**空串**唤起原生目录选择器；返回设置体 + `changed`（+`changed` 时 `pendingDataDir`）；取消或 `--data-dir`/`OVERVIEW_DATA_DIR` 显式指定时 `changed:false`；目录非法返回 400（仅桌面；需重启生效） |
| POST | `/desktop/restart` | 重启桌面壳：启动一份等待当前进程退出的新实例后退出（仅桌面） |
| POST | `/desktop/open-folder` | 用系统文件管理器打开当前数据目录（仅桌面） |
| GET | `/desktop/update` | 检查最新发布版本 `{current, latest, hasUpdate, url}`（只检测不下载；仅桌面） |
| GET | `/desktop/sync` | 同步状态/配置/冲突 `{enabled, serverURL, vaultId, direction, intervalSec, lastSyncAt, status, progress, conflicts, lastError, connected}`（仅桌面且挂载引擎；headless 一律 404） |
| PUT | `/desktop/sync` | 更新同步配置（**部分更新**：`serverURL`/`token`/`enabled`/`direction`/`intervalSec`；令牌只写不回传，保存前校验连通性） |
| POST | `/desktop/sync/run` | 触发一次同步（后台执行，绕过节流） |
| GET | `/desktop/sync/conflicts` | 未解决冲突列表 |
| POST | `/desktop/sync/conflicts/resolve` | 解决冲突 `{id, keep: local\|remote}` |

**其他端点**

| 路径 | 说明 |
| --- | --- |
| `/assets/{path...}` | 附件访问（`GET`/`HEAD` 匿名只读以支持公开页；上传与 `/api/v1/assets` 仍需认证；内嵌前端资源优先，经静态处理器压缩/缓存） |
| `/dav/` | WebDAV 挂载（Basic 认证，需 auth=multi） |
| `/mcp` | MCP 服务端（JSON-RPC 2.0，Bearer 令牌，协议 `2026-07-28`） |
| `/api/docs` | 自包含的 OpenAPI 交互文档 |

**静态服务与 404 语义（v0.16.3，ADR-088/089）**

内嵌前端由 `internal/server/static.go` 的 `staticHandler` 服务，SPA 回退与资源 404 的判定顺序为：

1. `fs.Stat` 命中：按扩展名给 `Content-Type` 并加 `X-Content-Type-Options: nosniff`；`assets/` 下带内容哈希的路径 `Cache-Control: immutable`，其余（`index.html`/`sw.js`/manifest/图标）`no-cache` + 弱 ETag（内容 FNV-1a）。
2. 未命中且 `isAssetRequest(p)` 为真（`assets/` 前缀或顶层白名单）→ **404，非 HTML**（不被 `index.html` 掩盖）。
3. 未命中且非资源（如 `/note/技术/Go/并发模型.md`、`/public/<path>`、任意未知客户端路由）→ 回退 **`index.html`（200，`text/html`）**，交由前端 router 处理。
4. `index.html` 本身缺失（前端未构建或产物损坏）→ 自包含、亮暗自适应的 **HTML 404 页**（`internal/server/notfound.go`，`Static==nil` 分支同此）。

`/api/*` 的未命中仍为 `{error:{code:"not_found"}}` JSON；`assets/` 缺失为非 HTML 404。前端侧未知路径由末尾 catch-all
`/:pathMatch(.*)*` → `NotFoundView.vue` 呈现，render（文档站）模式则重定向 `public-home`。

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
`code` ∈ `invalid` | `unauthorized` | `forbidden` | `not_found` | `method_not_allowed` | `conflict` | `too_large` | `not_supported` | `rate_limited` | `bad_gateway` | `unavailable` | `email_not_verified` | `internal`，
对应 HTTP 400/401/403/404/405/409/413/501/429/502/503/403/500。未验证邮箱登录返回 `email_not_verified`（403），
邮件未配置/投递失败返回 503（`unavailable`），上游（如更新检查、同步配置校验）失败返回 502（`bad_gateway`）。
v0.16.0 起补全 401/403/405/502 的语义码（ADR-083）。

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
                 /settings/{appearance|editor|ai|mail|site|storage|data|users|tokens} → SettingsView 分区
                 /public[/:path] → PublicHomeView / PublicNoteView ； /trash
                 /login /setup /register /forgot-password /reset-password /accept-invite /verify-email
                 /capture → 重定向首页（快速捕获改为顶栏弹窗）
stores/
  workspace.ts   目录树、当前笔记、增删改查、搜索
  settings.ts    主题/字号/语言/压缩/专注/侧栏宽度/主题色（localStorage）
  auth.ts site.ts dialog.ts
views/           Login/Setup/Register/ForgotPassword/ResetPassword/AcceptInvite/VerifyEmail
                 SettingsView + settings/{Appearance,Editor,AI,Mail,Site,Storage,Data,Users,Tokens}Section
components/
  App.vue        布局 + RouterView + DialogHost + CaptureDialog
  BrandMark.vue  品牌图形（笔记/文档 SVG）
  Sidebar.vue    操作 + 搜索 + 目录树 + 可拖拽宽度（紧凑字号）；窄屏为抽屉（`open` + 遮罩 + 次要入口 slot）
  TreeNodeItem.vue 递归节点（行内操作浮层，不改变行高）
  EditorPane.vue Tiptap 编辑器 + 工具栏 + 笔记栏（图片 + 文件附件）；窄屏右侧面板变底部抽屉（大纲/反链/历史/AI）
  CaptureDialog.vue 顶栏快速捕获弹窗（URL → 抓取预览 → 选目录保存）
  PublicShell.vue 公开页外壳（文档站风格）
  TokensPanel.vue EmptyState.vue SlashMenu.vue TocPanel.vue HistoryPanel.vue LinksPanel.vue AiPanel.vue
editor/          extensions.ts / nodes.ts / lowlight.ts / slash.ts / link.ts
markdown/        pipeline.ts / tasks.ts / math.ts / diagrams.ts / doc.ts / rules.ts / footnotes.ts / assets.ts
api.ts           类型化客户端（/api/v1，ApiError）
styles.css       @import 共享阅读样式（internal/sitegen/content.css）；应用外壳/组件/编辑器交互样式
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

### 8.4 移动端布局与断点（ADR-051）

窄屏不维护独立版本，而是按断点重排同一套组件（`styles.css` 的 `@media`）：

| 断点 | 变化 |
| --- | --- |
| `≤1100px` | 内容区收窄，右侧面板宽度压缩 |
| `≤860px` | 设置页导航由左侧栏改为顶部横向滚动；用户管理行开始换行 |
| `≤768px` | **手机布局**：侧栏变抽屉（`position:fixed` + `translateX`，汉堡开合、遮罩关闭、次要入口收进抽屉）；顶栏手机化（隐藏居中搜索与次要链接、品牌名省略号）；编辑器全宽；工具栏与 note-bar 横向滚动；右侧面板变**底部抽屉**（`translateY` + 遮罩，顶部 tab 切换大纲/反链/历史/AI）；搜索改为浮层；`100dvh` + `env(safe-area-inset-*)` |
| `≤560px` | 用户管理行三行布局（名称 / 徽章 / 操作），长邮箱省略号 |

触屏基础（`@media (pointer:coarse)` / `(hover:none)`）：

- `-webkit-tap-highlight-color: transparent`、`touch-action: manipulation`、`overscroll-behavior` 抑制误触与橡皮筋
- 所有 `:hover` 规则 gate 到 `(hover:hover) and (pointer:fine)`，避免触屏「粘滞 hover」；行内操作在无 hover 设备常显
- 可点目标 ≥40px（`button`/`.icon-btn`），输入控件 `font-size:16px` 防止 iOS 聚焦缩放
- 列表/树/标签等提供 `:active` 按下反馈；`prefers-reduced-motion` 下关闭过渡

PWA：`manifest.webmanifest` 增加 `orientation:"any"` 与 192/512 PNG 图标（`purpose:any`/`maskable`）；`index.html` 提供明暗两套 `theme-color`、`apple-touch-icon` 与 `viewport-fit=cover`。

### 8.5 公开页与静态站的阅读渲染契约（v0.13.3，ADR-054…058）

应用内阅读、公开页、静态导出站三态共享同一份 `.tiptap-content` DOM 契约与同一份
`internal/sitegen/content.css`（由 `docs/DESIGN.md` 所指的 `web/src/styles.css` `@import` 复用），
保证「同一篇 Markdown 在三处排版/组件/配色/代码高亮/数学/图表/任务/脚注/表格/图片/链接一致」，
差异仅在可编辑性：

- **共享样式**：`content.css` = 设计令牌（明暗 + `--hl-*`）+ `.tiptap-content` 全部阅读态规则
  （标题/列表/引用/代码/表格/图片/链接/wiki 链接/任务列表/KaTeX/Mermaid/脚注）。新增阅读样式
  只改此文件，应用与静态站自动同步（ADR-054）。
- **公开页契约**：`web/src/markdown/doc.ts` 的 `renderPublicDoc` 复用编辑器契约——`wikiToHtml`
  （不再把 wiki 降级为纯文本）、`trimFencedCodeNewline`、`normalizeTaskLists`（checkbox `disabled`）、
  `addCodeBlockClass`、表格单元格包 `<p>`；`PublicNoteView.vue` 的 `<article>` 带 `tiptap-content`
  （`public-doc tiptap-content`），并删除 `styles.css` 中 `.public-doc` 对 h1/h2/h3 的字号覆盖（ADR-057）。
- **静态站契约**：Go 侧 `internal/sitegen/render.go` 产出与上述一致的结构（见 §13.23），
  运行期由 `readonly-enhance.js` 按需渲染高亮/数学/图表（ADR-055/056）。
- **默认外观一致**：`accentRamp` 对默认色直通 `content.css` 的色阶，避免派生偏差（ADR-058）。

---

## 9. 开发与测试环境

### 9.1 命令（Makefile）

```bash
make dev            # 后端 :5230（内嵌前端）
make build          # 前端 + 单二进制（bin/overview）
make build-desktop  # 前端 + 桌面壳（bin/overview-desktop，本机平台）
make package-desktop # build-desktop + 归档 tar.gz
make icon           # 重新生成桌面图标（icon.png / icon.ico / rsrc_windows_amd64.syso）
make test           # Go 测试
make test-web       # 前端类型检查
make lint           # golangci-lint + vue-tsc
make fmt            # gofmt + prettier
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
| `sync` | 三方比对、方向守卫（both/pull/push）、冲突副本生成与解决、首同步补 id、状态与令牌分离持久化、URL 校验（https/回环）、退避重试、watcher 触发 |
| `server`（同步补充） | `/sync/manifest` 强 ETag/304、`raw` 写入的 `baseVersion` 契约、`PUT /assets/{path}` 防穿越与 501、`/desktop/sync*` 仅 LocalOnly（headless 404） |
| `index`（同步补充） | `changed_seq` 单调与全量重建延续、`vault_id` 生成、墓碑记录/读取 |

前端：`vue-tsc` 类型检查、**Vitest** 单元测试（`npm test`，覆盖 `markdown/html`、`markdown/doc`、`markdown/roundtrip`）、`vite build`。

集成测试用 `httptest` 真实走 HTTP 栈；`store`/`service`/`cli` 用 `t.TempDir()`。

### 9.3 CI 与发布

- `.github/workflows/ci.yml`：后端 `go vet` + `go test -race -cover`；前端 `npm ci` + `vue-tsc` + `npm test` + `vite build`；`golangci-lint`；Windows 上 `go build ./cmd/overview-desktop` 编译冒烟。
- `.github/workflows/docker.yml`：推送 `v*` 标签时构建 **多架构（amd64/arm64）** 镜像并发布到 **GHCR**（`ghcr.io/<owner>/<repo>`），使用内置 `GITHUB_TOKEN`，无需额外密钥。
- `.github/workflows/pages.yml`：导出文档站（`overview export`）并部署到 GitHub Pages。
- `.github/workflows/desktop.yml`：推送 `v*` 标签或手动触发时，矩阵构建桌面壳（`windows-latest` 无 CGO；`macos-14` arm64 / `macos-13` amd64 CGO；`ubuntu-24.04` GTK4+WebKitGTK 6.0；`ubuntu-22.04` `-tags "desktop gtk3"`），产物归档为 `.zip`/`.tar.gz` 上传 artifact，并在 tag 触发时作为 Release assets 发布；`fail-fast: false` 使单个平台失败不影响其它平台。

---

## 10. 安全

| 项 | 现状 |
| --- | --- |
| 路径穿越 | `store.resolve` 净化 + `filepath.Rel` 校验；有测试覆盖 |
| 存储型 XSS | 快照服务端转义；前端仅渲染已转义内容；登录页外链仅放行 `http(s)://` |
| 附件 | `X-Content-Type-Options: nosniff`；非内联类型强制 `Content-Disposition: attachment`（含 RFC5987 `filename*`，中文名不丢失）；SVG 不内联 |
| 上传体积 | `http.MaxBytesReader` + `ParseMultipartForm`（`OVERVIEW_MAX_UPLOAD_MB`） |
| 认证 | 多用户（bcrypt + 会话 + 角色），`auth=multi` 时对 `/api/v1/*` 与附件上传强制认证；`auth=none` 为单用户模式 |
| 无鉴权 owner（ADR-077） | `auth=none` 时 `authMiddleware` 向每个请求注入隐式 `owner`（`core.RoleAdmin`），使管理员门禁的设置端点与前端管理分区在无会话下可达；前端 `auth.isAdmin = mode==="none" \|\| role==="admin"`。该模式等价于「唯一管理员」，安全边界由 `localGuard`（桌面）或网络边界（自部署）承担，不应在不可信网络暴露 |
| 桌面 vault 配置 | `desktop.json` 只存 `dataDir` 且位于用户配置目录（非 vault）；`PrepareDataDir` 仅把「不存在/空目录」或「已含 `notes/`/`overview.db` 的目录」接受为 vault，其它非空目录拒绝，避免把任意文件夹误当 vault；更改仅写配置，重启后生效 |
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
| 密钥 | AI/MCP/S3/SMTP 密钥仅存服务端，接口永不回传（`hasKey` / `hasPassword` / `hasSecret`）；对象存储 `secretKey` 留空表示沿用已存值 |
| 对象存储配置 | `/settings/storage*` 仅管理员；保存前先构建后端，**失败不切换**且错误信息只含 endpoint/bucket（不含密钥）；「测试连接」用表单当前值且不改动活动后端 |
| 日志 | 结构化 JSON，无敏感内容；邮件收件人/正文/令牌从不记录 |
| 桌面本地守卫 | `LocalOnly` 下 `localGuard` 要求 Host 为回环名（`127.0.0.1`/`localhost`/`::1`），拒绝 DNS rebinding；写请求（POST/PUT/PATCH/DELETE）要求 Origin/Referer 与 Host 同源，无 Origin 的非浏览器客户端（curl/CLI/MCP）放行；`auth=none` 时以此替代 SameSite Cookie 的 CSRF 保护 |
| 桌面端点隔离 | `/api/v1/desktop/*`（`settings`/`vault`/`restart`/`open-folder`/`update`）仅在 `LocalOnly` 注册，headless 由 `desktopGuard` 在鉴权/路由前直接 404，与不存在路径不可区分 |
| 实时变更流 | `GET /sync/events`（SSE）与 `/sync/changes` 与其它数据端点一样受正常认证；事件只携带游标（`latestSeq`/`latestTs`），不含笔记内容；长连接由 `BaseContext` 在 `Stop` 时取消以便优雅退出（ADR-079） |
| 更新检查 | `internal/update` 只读取 GitHub Releases 的 `tag_name`/`html_url` 并做语义版本比较；不下载、不执行、不静默安装；网络失败只记 debug 日志 |
| 同步传输 | `internal/sync.RemoteClient` 仅接受 `https://` 或回环（`localhost`/`127.0.0.1`/`::1`）`http://`；服务端 `/sync/manifest`、`/note?raw=1`、`PUT /assets/{path}` 均走正常认证（多用户下需登录/令牌），raw 写入强制 `baseVersion` |
| 同步令牌 | 令牌单独存 `<DataDir>/.sync-token`（0600、原子写），**不写入 `sync.json`（可导出的状态文件）**、日志与错误信息；`/desktop/sync` 只写不回传明文（ADR-074） |
| 同步路径穿越 | `PUT /assets/{path}` 在落盘前校验每一段路径：拒绝空段、`.`/`..`、绝对路径、反斜杠、NUL 与冒号，并用 `http.MaxBytesReader` 限制体积（超限 413 `too_large`） |
| 同步信任边界 | 同步令牌等价于对该 vault 的完整读写；多用户部署共享同一 vault（无按用户 ACL），桌面端会镜像全部笔记（ADR-067） |
| 桌面无控制台 | GUI 子系统进程无 stdout；日志走 `fanout`（单 writer 失败不影响其它）并默认落 `<DataDir>/logs/desktop.log`，日志内容同样不含令牌与密钥（ADR-076） |

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
>
> v0.13.2 待权衡：运行时切换资产后端**不迁移历史附件**（本地与 S3 各自持有已存文件，切换后旧附件需自行搬运）；
> 移动端为响应式适配而非独立原生体验，复杂表格/宽内容在窄屏仍需横向滚动。
>
> v0.13.3 待权衡：静态站的高亮/数学/图表在**浏览器端运行期渲染**（首屏需下载对应库，Mermaid 单文件较大，
> 无构建期预渲染）；共享 `content.css` 使应用与静态站强绑定，修改阅读样式需同时回归两侧；
> 静态站为只读，不提供编辑器交互（selection/toolbar 等）。
>
> v0.14.0 待权衡：桌面壳**未在 macOS/Linux 实机验证**（CI 只做编译与归档，未跑通真实签名/公证/打包）；
> Windows/macOS 安装器（NSIS/DMG）与 Linux `.desktop`/AppImage 尚未生成，桌面壳以 `go build` 产物 + 归档交付；
> `wails3` CLI 未接入仓库构建流水线（仓库缺少生成的 Taskfile/build-assets，见 ADR-059）；更新提醒依赖 GitHub Releases
> 且**只提醒不静默安装**；桌面壳复用本地 HTTP 服务，固定端口被占时会回退随机 loopback 端口（由 `.overview-port` 发现），
> 不保证端口恒定。
>
> v0.15.0 待权衡：同步 Phase 1 用**完整 manifest 轮询**（默认 60s，非 SSE/实时推送），大库每次比对要传输全部笔记元数据；
> 多用户部署下**整个 vault 共享**（无按用户 ACL，令牌即可读写全部笔记）；**S3 附件后端不支持按路径写入**（`PUT /assets/{path}` 返回 501，push/pull 都会跳过该附件）；
> 删除与编辑的判定依赖客户端状态（首次同步或状态丢失时，远端缺失可能被当作本地新建重建，反之亦然）；冲突副本会作为独立笔记出现在两侧（需用户裁决后清理）；
> 桌面无控制台时若日志文件也不可写，启动失败只能靠系统事件或手动设置 `OVERVIEW_LOG_FILE` 诊断。
>
> v0.16.0 已缓解：**同步实时化**（SSE `GET /sync/events` + `ChangeBus`，轮询降为兜底）、
> **增量同步**（`GET /sync/changes` 以笔记序号 + 墓碑时间戳游标追赶）、**远端新建空文件夹现已实时**、
> **S3 附件按路径同步不再 501**（`s3store.Restore`），并补齐 401/403/405/502 错误码。
> v0.16.0 待权衡：**大文件分片上传仍未做**（超过 `OVERVIEW_MAX_UPLOAD_MB` 的附件在同步中跳过并记
> warning，需人工处理）；**多用户部署仍无 per-user 工作区**（整库一个 vault，令牌即整库读写，
> 见 ADR-067）；SSE 为进程内 `ChangeBus`，多实例下事件不跨节点（与限流/会话同为单进程假设），
> 需要粘性路由或外部消息；`folder_changes` 使目录空置也占用共享序号空间，长期只增不减（未做
> 目录变更历史的清理/压缩）；增量游标位于客户端状态文件，状态丢失时仍回退一次完整 manifest。
>
> v0.16.1 已修复（原「已知问题」）：**桌面升级后仍加载旧前端**。根因是静态资源弱 ETag 只哈希
> 路径 + 字节长度，Vite 重建的 `index.html` 字节数几乎不变 → ETag 不变 → 客户端 304 沿用旧壳，
> 而旧壳引用的哈希资源又被 `immutable` 缓存一年。现改为内容哈希，并让桌面壳 URL 带版本号
> （`?v=<version>`）形成全新缓存条目（ADR-084/085）。同时补上专注模式的**可见出口**（ADR-086）：
> 顶栏/侧栏隐藏时不再只有 Esc/F9。
> v0.16.1 仍待权衡：`immutable` 缓存一年意味着**旧哈希资源**在升级后仍占据用户磁盘，直到浏览器
> 按容量淘汰（无主动清理）；桌面壳缓存条目按版本累积，长期频繁升级会新增多条缓存目录；版本号
> 通过 URL 查询串传递，若前端将来解析 `v` 需注意其仅用于破缓存、不代表后端 API 版本。

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
- [x] 移动端 PWA（响应式手机端适配，v0.13.2）
- [ ] 移动端原生 App（基于 `/api/v1`）

**Phase 4 — 体验与规模**
- [x] 文件监视器，外部编辑自动入库
- [x] 增量索引（避免全量重建）
- [x] 版本历史 / 回收站
- [x] 附件引用计数与回收
- [x] 多存储后端（S3，v0.13.2 起可在设置页运行时切换）
- [x] OpenAPI 文档
- [x] 移动端 PWA（v0.13.2 响应式手机端）
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

**Phase 11 — 手机端适配 · 对象存储设置 · 修复（v0.13.2）**

移动端（P0+P1）
- [x] 抽屉式侧栏（汉堡开合 + 遮罩 + 次要入口收进抽屉）、顶栏手机化、编辑器全宽
- [x] 工具栏 / note-bar 横向滚动；右侧面板改底部抽屉（大纲/反链/历史/AI）
- [x] 移动端搜索浮层；触屏基础（tap-highlight、`touch-action`、`:active`、hover 门槛、16px 输入、≥40px 目标、`100dvh`/safe-area）
- [x] 明暗 `theme-color`；PWA `orientation` + 192/512 PNG 图标

设置与修复
- [x] S3 对象存储可在设置页配置并运行时切换（`SwitchableAssetStore` + `StorageService` + `/settings/storage*`）
- [x] 用户管理窄屏三行布局（名称/徽章/操作）
- [x] AI「整理/补全」打开笔记后按钮失效与空笔记无提示修复

**Phase 12 — 静态文档站与应用阅读态一致（v0.13.3）**

渲染一致
- [x] 共享内容样式单一真源 `internal/sitegen/content.css`（App `@import` + 静态站 `<link>`）
- [x] Go 侧结构对齐（`render.go`：代码/任务/脚注/数学/图表/表格/wiki/图片/标题 slug/搜索索引）
- [x] 运行期增强**按需加载与应用同版本** highlight.js/KaTeX/Mermaid（`vendor.go` + `readonly-enhance.js`）
- [x] 公开页渲染复用编辑器契约（`doc.ts`），`PublicNoteView` 三态统一（`public-doc tiptap-content`）
- [x] 默认 accent 派生色直通（`accentRamp`），三态默认外观一致

**Phase 13 — 桌面应用（v0.14.0）**

桌面壳与生命周期
- [x] Wails v3 原生壳 `cmd/overview-desktop`：窗口、系统托盘、单实例、优雅退出
- [x] `internal/app` 统一生命周期（实例锁 + 端口文件 + 监听回退）供 headless/桌面共用
- [x] 首启原生目录选择器；平台数据目录 `DefaultDataDir()`（Windows/macOS/Linux）
- [x] 文件关联 `.md`、`overview://` 深链、文件拖拽；第二实例 argv 转发并聚焦窗口
- [x] 开机自启（Wails Autostart）与设置页 `DesktopSection.vue`
- [x] 更新提醒（`internal/update`，只检测不静默安装）

安全与打包
- [x] 本地安全边界 `localGuard`（Host 回环 + 同源/CSRF）；`/desktop/*` 仅 LocalOnly
- [x] 桌面默认 loopback/`auth=none`/MCP+WebDAV 关闭，环境变量可覆盖
- [x] `build/config.yml` 打包元数据（应用名/版本/标识/文件关联/协议）
- [x] `.github/workflows/desktop.yml` 跨平台构建矩阵 + tag 发布 Release 产物
- [ ] Windows NSIS / macOS DMG / Linux AppImage 安装器（待接入 wails3 打包资产）
- [ ] macOS/Linux 实机验证与签名/公证

**Phase 14 — 桌面↔服务器同步 · 桌面打磨（v0.15.0）**

服务端
- [x] 迁移 `0010_sync`：`notes.changed_seq` + `note_tombstones`；`settings` 存 `vault_id`/`change_seq`
- [x] `GET /api/v1/sync/manifest`（vaultId + 笔记版本 + 文件夹，强 ETag + 304）
- [x] `PUT /api/v1/note` 支持 `raw:true`（字节保真 + 保留 id + `baseVersion` 乐观并发）
- [x] `PUT /api/v1/assets/{path}`（按路径写附件、防穿越、体积限制、S3→501）
- [x] `GET /api/v1/note?raw=1`；删除/移动记录墓碑

客户端 `internal/sync`
- [x] `RemoteClient`：仅 https 或回环 http；令牌独立 `.sync-token`，不落 `sync.json`/日志
- [x] `Engine` 三方比对：pull/push/delete/move、首同步补 id、指数退避、幂等
- [x] 冲突副本 `<name> (conflict-<device>-<ts>).md` + `keep local|remote` 解决
- [x] 方向 `both|pull|push`（`plan()` 守卫）；显式「立即同步」绕过节流
- [x] 附件按路径同步（push 哈希去重 / pull 补缺失，S3 跳过）；watcher 近实时

桌面与前端
- [x] `internal/app` 以 `SyncFactory` 装配引擎生命周期；`internal/server/sync.go` 暴露 `/desktop/sync*`（仅 LocalOnly）
- [x] `SyncSection.vue`（服务器地址/令牌/开关/方向/间隔/立即同步/状态卡/冲突区，仅桌面）
- [x] Windows 桌面构建 `-H=windowsgui`（无控制台）；桌面日志默认 `<DataDir>/logs/desktop.log`；`internal/logging` 容错 fanout

**Phase 15 — 桌面引导 · owner 权限 · 同步补齐（v0.16.0）**

无鉴权与桌面生命周期
- [x] 无鉴权模式注入 `owner`（admin）：设置页邮件/站点/存储/数据/用户/令牌与 `/settings/*` 全部可达
- [x] 前端 `auth.isAdmin = mode==="none" || role==="admin"`，设置分区与各 Section 一致放开
- [x] `internal/config/desktop.go`：桌面配置 `<UserConfigDir>/Overview/desktop.json`、`HasVault`、`PrepareDataDir`
- [x] 桌面壳首启引导（默认/打开已有/新建 + 原生目录对话框），显式 `--data-dir`/env 跳过
- [x] 设置页「桌面」分区：更改数据目录、打开数据目录、重启应用（`/desktop/vault|open-folder|restart`）
- [x] 重启子进程等待前任退出，释放单实例与 vault 锁

同步实时与增量（Phase 2/3 局部）
- [x] 迁移 `0011_sync_folders`（`folder_tombstones`）、`0012_folder_changes`（`folder_changes`，共享序号）
- [x] `GET /api/v1/sync/changes`（`since`/`sinceTs`/`limit`：changes/tombstones/folderTombstones/游标/hasMore）
- [x] `GET /api/v1/sync/events`（SSE，25s 心跳，`ResponseController` 清写超时，`BaseContext` 随 Stop 取消）
- [x] 进程内 `service.ChangeBus` 广播（Save/SaveRaw/Delete/Move/Mkdir/ReindexPath/Reindex）
- [x] `RemoteClient.Changes/Events`；`Engine` SSE debounce + 轮询兜底 + `execMu` 单飞 + 游标持久化
- [x] **空文件夹双向同步**（父→子建、子→父删、`folder_sync`/`folder_forget`，空目录守卫）
- [x] 目录删除实时（`folder_tombstones`）；超出体积的附件跳过并记 warning
- [x] `s3store.Restore` 按路径写，S3 下附件同步不再 501
- [x] 错误码补全 401/403/405/502（`unauthorized`/`forbidden`/`method_not_allowed`/`bad_gateway`）

**Phase 16 — 桌面升级可见性与专注出口（v0.16.1）**

- [x] `weakETag` 改为内容哈希（FNV-1a），修正「字节数相近的重建 `index.html` 命中 304」的旧前端缓存（ADR-084）
- [x] 缺失的静态资源（`assets/` 前缀或带扩展名）返回 404，不再回退 `index.html`（ADR-084）
- [x] 桌面壳窗口 URL 带构建版本 `?v=<version>`，每个发版形成新 WebView2 缓存条目（ADR-085）
- [x] 专注模式新增右下角浮动「退出专注」按钮 `.focus-exit`（i18n `focus.exit`），保留 Esc/F9（ADR-086）

**Phase 17 — 侧栏树可读性（v0.16.2）**

- [x] 文件夹行（含 depth-0/1/2/3）统一 `--text`，去掉深层 `--text-soft`/`--text-faint` 的降对比度（ADR-087）
- [x] 三级笔记行由 `--text-soft` 改为 `--text-muted`；层级改由缩进 + 文件夹大写/600 字重表达（ADR-087）

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
| `OVERVIEW_LOG_FILE` | 空 | 日志文件路径；设置后同时写入文件（空则仅 stdout）；桌面壳未设时默认 `<DataDir>/logs/desktop.log`（GUI 无控制台） |
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
| `OVERVIEW_S3_BUCKET` | 空 | S3 兼容存储桶；**作为初始值**，设置页「对象存储」保存后以其为准 |
| `OVERVIEW_S3_ENDPOINT` | 空 | S3 端点（如 `s3.amazonaws.com`）；同上，可被设置页覆盖 |
| `OVERVIEW_S3_REGION` | `us-east-1` | 区域；同上 |
| `OVERVIEW_S3_ACCESS_KEY` / `_SECRET_KEY` | 空 | 凭据；同上 |
| `OVERVIEW_S3_USE_SSL` | `true` | 是否使用 HTTPS；同上 |
| `OVERVIEW_S3_PUBLIC_URL` | 空 | 可选 CDN/公开前缀；同上 |

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
- **S3 后端**（ADR-021/052）：`internal/s3store`（minio-go）实现 `AssetStore`；`OVERVIEW_S3_BUCKET` 作为初始值启用。设置页「对象存储」可在运行时切换本地 ↔ S3（`SwitchableAssetStore` 代理当前后端 + `StorageService` 校验/持久化到 `settings` 表）；保存后以设置页为准（`s3_configured`），切换即时生效但**不迁移历史附件**；`secretKey` 不回传、构建失败不切换。
- **PWA**（ADR-022）：manifest + service worker，仅缓存应用壳与哈希构建产物，离线可启动。
- **OpenAPI**（ADR-023）：`/api/v1/openapi.json` 与 `/api/docs`（无外部依赖）。

### 13.7 设置中心与设计系统（v0.8，v0.12 改为独立页）

> v0.12.0 起设置由弹窗改为独立路由页 `/settings`，分区扩展为 8 个；旧的 `SettingsDialog` 已删除。详见 §13.15。

- **设置中心**（ADR-024/038）：独立页左侧分类导航：
  - 外观：主题（跟随系统/浅色/深色）、字号（紧凑/默认/舒适/大）、语言、主题色
  - 编辑器：上传前压缩图片（默认开，持久化）
  - AI：状态 + 当前模型；管理员可编辑 Base URL / API Key / Model
  - 对象存储（管理员，ADR-052）：Endpoint / Region / Access Key / Secret Key / Bucket / HTTPS / 公开前缀 + 「测试连接」；留空 Bucket 回退本地存储
- **运行时 AI 配置**（ADR-025）：`settings` 表（迁移 0006）存 `ai_base_url`/`ai_api_key`/`ai_model`；`AIService.Load` 合成默认值与环境变量，`SaveConfig` 持久化并即时重载；接口 `GET/PUT /api/v1/settings/ai` 仅管理员，**密钥永不回传**（只返回 `hasKey`）。
- **图片压缩**：`web/src/media/compress.ts` 用 Canvas 降采样（默认最长边 1920）+ 优先 WebP 重编码 + 2MB 预算自动降质；GIF/SVG 与非图片不动，失败回退原图。
- **排版系统**（ADR-026）：所有 UI 文本基于 `--base-size` 派生的 `--text-xs/sm/ui/body`；编辑器正文 `--text-body` 与周边一致。字号设置切换时**整站同步缩放**。
- **布局重做**：全宽顶栏（品牌左、居中搜索 `Ctrl K`、右侧导航）、左侧分组式多级导航（顶层大写分组、按深度区分字重/缩进/配色）、内容区大标题 + 右侧「大纲」栏，参考 Starlight 文档站。
- **侧栏树文字对比度**（v0.16.2，ADR-087）：多级菜单不再「层级越深越淡」——`.tree-row.folder`（含 depth-0/1/2/3）
  统一 `--text`，`.tree-row.note.depth-3` 用 `--text-muted`；层级靠 `padding-left` 缩进与文件夹「大写 + 600 字重」
  表达，hover/active 不变。token 定义仍在共享 `internal/sitegen/content.css`，本次只调整应用侧选择器。
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
- **`build`**（ADR-029）：调用 `sitegen.Generate` 生成可部署静态站（每篇 HTML、`index.html`、
  `style.css`、共享阅读样式 `content.css`、`search.js`/`search-index.json`、运行期增强
  `readonly-enhance.js` 与按需的 `vendor/highlight|katex|mermaid`、可选 `assets/`）；
  `--all` 导出全部笔记（默认仅 `public`），`--base`/`--title`/`--out` 可配；`--theme auto|light|dark`
  （或 `OVERVIEW_SITE_THEME`）指定主题，默认 `auto` 跟随访客系统偏好。渲染与增强详见 §13.23。

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
- **缓存校验语义**（v0.16.1，ADR-084/085）：`weakETag` 用文件**内容**的 FNV-1a 哈希（而非路径 +
  字节长度），因此 `index.html` 内容变化必然更新 ETag，`no-cache` 下重取新壳；未命中的静态资源
  （`assets/` 前缀或带扩展名）直接 **404**，只有无扩展名的客户端路由才回退 `index.html`（避免把
  HTML 当 JS/CSS 返回）。桌面壳窗口 URL 带 `?v=<version>`（ADR-085），使每个发版成为独立的
  WebView2 缓存条目，升级后自动加载新前端，无需用户清缓存。
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

### 13.21 手机端适配 · 对象存储设置 · 修复（v0.13.2，ADR-051/052/053）

**手机端适配（P0+P1）**

- **抽屉式侧栏**：`Sidebar.vue` 新增 `open` prop 与 `close` 事件；`App.vue` 顶栏加汉堡按钮（`nav-toggle`），窄屏侧栏 `position:fixed` + `translateX(-100%)` 滑入，配遮罩关闭；搜索/专注/快捷键/设置/捕获/回收站/用户与登出等次要入口收进抽屉（`sidebar-drawer-nav`）。路由变化自动收起。
- **顶栏手机化**：隐藏居中搜索与次要链接，品牌名省略号；`height` 计入 `env(safe-area-inset-top)`。
- **编辑器全宽**：`.tiptap-content`/`.editor-crumb` 取消 `max-width`；工具栏与 note-bar 横向滚动（隐藏滚动条、`-webkit-overflow-scrolling:touch`）。
- **右侧面板 → 底部抽屉**：`EditorPane.vue` 用 `matchMedia("(max-width:768px)")` 判定 `isMobile`；窄屏时大纲/反链/历史/AI 合并为底部抽屉（`sheet-head` tab 切换 + `sheet-backdrop`），`Escape` 关闭，跳转标题后自动收起。
- **移动端搜索浮层**：`App.vue` 新增 `search-overlay`，`SearchBox` 支持 `autofocus`。
- **触屏基础**：见 §8.4（tap-highlight、`touch-action`、`:active`、hover 门槛、16px 输入、≥40px 目标、`100dvh`/safe-area）。
- **PWA**：`manifest.webmanifest` 加 `orientation:"any"` 与 192/512 PNG 图标；`index.html` 明暗 `theme-color`、`apple-touch-icon`、`viewport-fit=cover`。

**用户管理窄屏修复**

- `UsersSection.vue` 行结构改为语义分组 `.user-meta`（名称 + `.user-badges`）与 `.user-actions`；`≤560px` 三行布局（名称 / 徽章 / 操作），徽章 `white-space:nowrap`，长邮箱省略号，操作按钮 ≥40px。

**AI 助手「整理/补全」修复（ADR-053）**

- `EditorPane.vue` 新增 `syncAiContent()`，在 `load()` 完成后、编辑器内容变化、插入 AI 结果后同步 `aiContent`——修复打开笔记后「整理/补全」按钮静默无反应（`setContent` 的 update 被 `suppress` 吞掉）。
- `AiPanel.vue` 空笔记点「整理/补全」给出 `ai.emptyContent` 提示而非无反应。

**对象存储设置（ADR-052）**

- `internal/service/storage.go`：`SwitchableAssetStore`（`sync.RWMutex` 代理当前 `core.AssetStore`，`Set(nil)` 被忽略以保证始终有可用后端）；`StorageService`（`Load`/`SaveConfig`/`Test`/`TestConfig`/`Config`/`Enabled`，`StoreFactory` 为测试接缝）。`SaveConfig` 先构建后端再持久化，失败返回 `storageBuildError`（含 endpoint/bucket、不含密钥）且不切换；`secretKey` 留空沿用已存值。
- `internal/server/storage.go`：`GET/PUT /settings/storage`、`POST /settings/storage/test`（均仅管理员）；`test` 接受可选请求体（缺省用已存配置），返回 `{ok,message}`。
- `cmd/overview/main.go`：用 `service.NewSwitchableAssetStore(st)` 包裹本地存储，env 启用 S3 时 `Set(s3)`；构造 `StorageService` 并 `SetDefaults(env)` + `Load`。
- 前端 `views/settings/StorageSection.vue` + `api.ts`（`storageSettings`/`saveStorageSettings`/`testStorage`）+ 路由 `settings-storage` + 设置页导航项。
- `internal/core/errors.go` 新增 `ErrNotSupported`；`archivex` 导入遇到不支持显式路径恢复的后端（如 S3）时跳过该附件而非报错。

### 13.22 v0.13.2 验证记录

- `go build ./...`、`go test ./...` 全绿（新增 `internal/service/storage_test.go`、`internal/server/storage_test.go`）
- 前端 `vue-tsc`、`npm test`（Vitest）、`vite build` 全绿
- 端到端：设置页保存 S3 配置后新上传走对象存储、留空 Bucket 回退本地；「测试连接」用表单当前值且不改动活动后端；`secretKey` 不回传；构建失败返回可诊断错误且不切换；窄屏（≤768px）侧栏抽屉/底部面板/搜索浮层可用；用户管理 ≤560px 三行布局；打开笔记后 AI「整理/补全」正常，空笔记有提示

### 13.23 静态文档站与应用阅读态一致（v0.13.3，ADR-054…058）

**共享内容样式（ADR-054）**

- 新增 `internal/sitegen/content.css`：设计令牌（明暗 + `--hl-*` 语法高亮变量）+ `.tiptap-content`
  全部阅读态规则（标题/列表/引用/代码/表格/图片/链接/wiki 链接/任务列表/KaTeX 数学/Mermaid/脚注）。
- `web/src/styles.css` 删除这些令牌与规则，改为 `@import "../../internal/sitegen/content.css"` 复用；
  应用外壳/组件/编辑器交互样式（`.ProseMirror-*`、placeholder、选区、工具栏、面板）仍留在 `styles.css`。
- 静态站每页 `<link rel="stylesheet" href="…/content.css" />`（与 `style.css` 并列）；`go:embed content.css`
  保证样式随二进制分发，`content.css` 成为阅读样式的唯一真源（防漂移）。

**结构对齐（ADR-055，`internal/sitegen/render.go`）**

- goldmark（GFM）+ `golang.org/x/net/html` 解析 + 后处理，输出与编辑器阅读态一致的 DOM：
  - 代码：`pre.code-block > code.language-*`（去掉尾随换行）；`language-mermaid` 转 `.mermaid[data-source]`。
  - 任务列表：`ul[data-type=taskList]`、`li[data-type=taskItem][data-checked]`（结构为
    `label > input[disabled] + span` 与 `div > p`，嵌套列表保留在 `div` 内）。
  - 脚注：引用 `sup.fn-ref[data-id]`，定义集中到尾部 `.fn-defs > .fn-def[data-id]`（镜像 `footnotes.ts`）。
  - 数学：`.math-inline` / `.math-block` 均带 `data-tex`（保留原始 TeX，Go 侧不渲染）。
  - 表格：每个 `th`/`td` 包一层 `<p>`（与 ProseMirror 单元格一致）。
  - wiki 链接：按标题/basename 解析，命中输出 `a.wiki-link`，未命中降级 `span.wiki-link.wiki-missing`。
  - 图片：`img[loading=lazy][decoding=async]`；`src`/`href` 的 `assets/` 前缀按部署基址（`--base`）重写。
  - 标题：`id` 按应用 `doc.ts` 的 `slugify` 生成（去标签、小写、保留实体、去标点、空白转 `-`，重复加 `-N`），
    TOC 锚点与应用完全一致。
- 搜索索引：`searchText` 提取正文时纳入代码文本与 `data-tex`/`data-source` 原始源文本，
  使代码/公式/图表可被站内搜索命中。
- 资源：`assets/` 目录递归拷贝（`copyDir`）；启用 S3 后端时不拷贝本地镜像，改以 `S3PublicURL` 为
  资源前缀（`Options.AssetURLPrefix`）。

**运行期增强（ADR-056，`internal/sitegen/vendor.go` + `readonly-enhance.js`）**

- `vendor/highlight|katex|mermaid` 从 `web/node_modules` 原样拷贝（与应用**同版本**）；导出时**按需**
  只写入页面实际用到的库（`writeVendor` 依据 `HasCode`/`HasMath`/`HasMermaid`），无数学/图表/代码的站不增加体积。
- 页面按需注入 `<link>`（`katex.min.css`）与 `<script>`（highlight/katex/mermaid + `readonly-enhance.js`）。
- `readonly-enhance.js`：
  - 高亮 `pre.code-block > code.language-*`（highlight.js `common`，直接 `hljs.highlight`、不加 `hljs` 根类，
    贴合 `lowlight` 输出）。
  - 渲染 `[data-tex]`（`displayMode` 与 `math.ts` 一致，`throwOnError:false`、`output:"html"`）。
  - 渲染 `.mermaid[data-source]`（`securityLevel:"strict"`，按 `data-theme` 取 `dark`/`default`）。
  - 脚注引用点击平滑滚动到定义（不改变 DOM 形状）。
  - `<body data-base>` 供搜索索引与资源解析子路径。
- 体积：仅基础/代码的最小站为几十 KB 级；数学站额外 ~0.3 MB（KaTeX JS + CSS + woff2 字体）；
  Mermaid 站额外 ~3.5 MB（单文件）；库仅在页面确需时写入。

**三态统一（ADR-057）**

- `web/src/markdown/doc.ts` 的 `renderPublicDoc` 复用编辑器契约：`wikiToHtml`、`trimFencedCodeNewline`、
  `normalizeTaskLists`（checkbox `disabled`）、`addCodeBlockClass`、表格 `th/td` 包 `<p>`。
- `PublicNoteView.vue` 的 `<article>` 加 `tiptap-content`（`public-doc tiptap-content`）；
  `styles.css` 删除 `.public-doc` 对 h1/h2/h3 的字号覆盖，改由 `content.css` 统一。

**默认 accent 直通（ADR-058）**

- `web/src/stores/settings.ts` 的 `accentRamp` 对默认色直接返回与 `content.css` 一致的四元组
  （浅色 `#d4870e`/`#b87308`/`#fdf4e3`/`rgba(212,135,14,.24)`，深色 `#e9a23b`/`#f2b658`/`#3a2f16`/`rgba(233,162,59,.3)`）；
  自定义色仍走派生，消除默认外观在三态间的细微偏差。

### 13.24 v0.13.3 验证记录

- `go build ./...`、`go test ./...` 全绿（`internal/sitegen` 覆盖结构对齐、标题 slug、搜索索引、vendor 按需、assets 拷贝）
- 前端 `vue-tsc`、`npm test`（Vitest）、`vite build` 全绿（`markdown/doc` 覆盖公开页契约）
- 端到端：导出站打开代码/数学/Mermaid/任务/脚注/表格/wiki 链接与 App 阅读态一致；wiki 命中为链接、未命中降级；
  搜索可命中代码与公式源文本；`--base` 子路径下 vendor/资源/搜索索引路径正确；无数学/图表站点不产生多余 vendor 文件；S3 后端导出资源指向 `S3PublicURL`

### 13.25 桌面应用（v0.14.0，ADR-059…066）

**选型与生命周期（ADR-059/060/061/065）**

- 桌面壳 `cmd/overview-desktop` 采用 Wails v3 `v3.0.0-beta.26`：窗口、系统托盘、单实例、开机自启、原生通知由 Wails 提供，业务逻辑复用 `internal/app`。
- `internal/app` 统一 headless 与桌面的装配与生命周期：`AcquireInstanceLock`（`<data>/.overview.lock`）、端口文件（`<data>/.overview-port`）、监听（桌面固定端口被占回退随机 loopback）、初始索引、文件监视、优雅退出。
- `cmd/overview` 重构为调用 `app.New`/`app.Start`/`app.Stop`，对外命令行为不变。

**本地默认与安全（ADR-062/063/064）**

- `OVERVIEW_DESKTOP=true` 时 `config.Load` 默认 `127.0.0.1:5230`、`auth=none`、MCP/WebDAV 关闭、`DefaultDataDir()`；均可用环境变量覆盖。
- `server.Handler` 在 `LocalOnly` 下包一层 `localGuard`（Host 必须为回环名 + 写请求同源）；`/api/v1/desktop/*` 只在 LocalOnly 注册，headless 由 `desktopGuard` 在鉴权前直接 404。
- `internal/update.CheckLatest` 只读 GitHub Releases 的 `tag_name`/`html_url` 并做语义版本比较，**不下载、不静默安装**。

**打包与 CI（ADR-066）**

- `build/config.yml` 提供 Wails v3 打包元数据（`info`/`fileAssociations`/`protocols`）；`ext` 修正为无点 `md`（macOS `CFBundleTypeExtensions` 约定）。
- 新增 `.github/workflows/desktop.yml`：`fail-fast: false` 矩阵 `windows-latest`（CGO 0）/`macos-14`（arm64）/`macos-13`（amd64）/`ubuntu-24.04`（GTK4 + WebKitGTK 6.0）/`ubuntu-22.04`（`-tags "desktop gtk3"`）；归档 `.zip`/`.tar.gz` 上传 artifact，tag 触发时作为 Release assets 发布。
- `ci.yml` 新增 Windows 桌面壳编译冒烟（`go build ./cmd/overview-desktop`）。
- **Windows 图标**：exe/资源管理器/任务栏图标通过 rsrc 生成的资源对象 `cmd/overview-desktop/rsrc_windows_amd64.syso` 嵌入（`*_windows_amd64.syso` 命名使隐式构建约束只在 windows/amd64 生效，不影响 darwin/linux）。`icon.ico`（16/24/32/48/64/128/192/256，PNG 内嵌、保留透明）与 `cmd/overview-desktop/icon.png`（512 RGBA，`go:embed` 用于 Wails 窗口/托盘）由 `tools/icongen` 从品牌源 `web/public/icon-512.png` 生成；`make icon` 重新生成两者并重跑 rsrc。`.syso` 已提交，rsrc 不是普通构建/CI 的必需依赖。
- `wails3` CLI（`v3.0.0-beta.26`）已安装并确认可执行，但仓库未采用其生成的 Taskfile/build-assets，`wails3 build`/`package` 无法直接作用于当前布局；本轮以 `go build` 产物 + 归档交付，安装器（NSIS/DMG/AppImage）留待接入。

**验证记录**

- `gofmt -l .` 无输出；`go build ./...`、`go vet ./...`、`go test ./...` 全绿。
- `cd web && npm run build`（`overview-web@0.14.0`）通过。
- 本机（Windows amd64）`make build-desktop` / `make package-desktop` 成功：`bin/overview-desktop.exe`（~27.5 MB）与 `bin/overview-desktop-0.14.0-windows-amd64.tar.gz`（~10.9 MB）。
- `.github/workflows/desktop.yml`、`ci.yml` 等 YAML 可解析（pyyaml）。
- headless：`bin/overview.exe` 重编为 v0.14.0，以 `OVERVIEW_AUTH=multi`、`OVERVIEW_DATA_DIR=%TEMP%\opencode\uidemo`、`OVERVIEW_ADDR=0.0.0.0:5230` 启动；`/api/v1/health` 返回 `version: 0.14.0`，`/api/v1/auth/state` 返回 `mode: multi`。
- **本机未验证**：macOS/Linux 桌面构建、NSIS/DMG/AppImage 安装器、真实安装注册。

### 13.26 桌面↔服务器同步 · 桌面打磨（v0.15.0，ADR-067…076）

**服务端（`internal/service`、`internal/index`、`internal/server`）**

- 迁移 `0010_sync`：`notes.changed_seq`（`DEFAULT 0`）与 `note_tombstones(id, path, deleted_at, device)` + `deleted_at` 索引；`settings` 新增 `vault_id`、`change_seq` 键。
- `service.Manifest`：合成 `{vaultId, generatedAt, etag, notes[], folders[]}`（笔记含 id/path/version/updated/size/public，文件夹含空目录）；`manifestETag = sha256(vaultId:maxSeq:count)[:16]`。
- `handleSyncManifest`：返回强 `ETag` + `Cache-Control: no-cache`，`If-None-Match` 命中返回 304。
- `handleGetNoteRaw`（`GET /note?raw=1`）：返回完整 Markdown 字节，ETag = 版本；render 模式仅公开笔记。
- `Service.SaveNoteRaw`：`raw:true` 时字节原样写入（保留 frontmatter id/created），`baseVersion` 必填；`checkBaseVersion` 实现 `*` 新建 / 指定版本更新（缺失或不匹配 → 409）。`core.RawWriter` 未实现时 501。
- `handleRestoreAsset`（`PUT /assets/{path}`）：`validateAssetPath` 拒绝空段/`.`/`..`/绝对/反斜杠/NUL/冒号；`MaxBytesReader` 限体积（超限 413 `too_large`）；`Service.RestoreAsset` 经 `core.AssetRestorer`，S3 返回 501 `not_supported`。
- 删除/移动记录墓碑：`service.Delete`/`Move` 先收集 `deletionVictims`，索引更新后 `recordTombstones` 写入。
- `handleSync*`（`internal/server/sync.go`）：`SyncHooks` 接口 + `/desktop/sync`、`/run`、`/conflicts`、`/conflicts/resolve`；`syncHooksAvailable()` 要求 `LocalOnly && SyncHooks != nil`，否则 404。`statusCode` 增加 `too_large`/`not_supported` 映射。

**客户端（`internal/sync`）**

- `RemoteClient`：`normalizeServerURL` 校验 scheme/host，普通 HTTP 仅允许回环；Bearer 令牌；`HTTPError` 映射到哨兵错误（401/403/409/404/501/413/5xx）。
- `State`：`<DataDir>/sync.json`（0600，原子写）只存 serverURL/vaultId/deviceId/enabled/intervalSec/direction/lastSyncAt/notes/folders/assets/conflicts；令牌单独 `<DataDir>/.sync-token`（0600，`ReadToken`/`WriteToken`/`ClearToken`）。
- `Engine`：`SyncOnce` 三方比对；首同步 `assignID` 为缺 id 笔记回写 ULID；`plan` 产出文件夹/拉/推/删（按深度父先子后）并以 `canPull`/`canPush` 守卫方向；冲突 `execConflict` 写 `<name> (conflict-<device8>-<ts>).md`（新 id）并推送；`SyncResolveConflict` 支持 `local`/`remote`；`retry` 指数退避（5 次，1s→8s 上限）；`NotifyDirty`（3s 自写抑制）+ `Start`/`Stop`/`Run` 后台轮询；`SyncRun` 置 `forcePending` 绕过 1s 节流。
- 附件：`pushAssets` 按 sha256 去重后 `PutAsset`；`pullAssets` 仅补本地缺失，需 `core.AssetRestorer`，不支持则告警跳过。

**桌面与前端**

- `internal/app`：`SyncDeps`/`SyncEngine`/`SyncFactory`；`build()` 构造引擎并作为 `server.SyncHooks`；`Start`/`Stop` 管理引擎；watcher 用 `dirtyReindexer` 在增量重建前 `NotifyDirty`。
- `cmd/overview-desktop` 注入 `desktopSyncFactory`；`SYNC` 引擎与本地 vault 共用 store/index。
- 前端 `SyncSection.vue`：仅 `site.desktop` 渲染；服务器地址/令牌/开关/方向/间隔 + 保存（校验 https/回环）/立即同步（轮询状态）/状态卡/冲突区（keep local/remote）；`api.ts` 新增 `syncStatus`/`saveSync`/`runSync`/`resolveSyncConflict`。
- 桌面隐藏控制台：`cmd/overview-desktop` 日志默认 `desktopLogFile(cfg)`（`<DataDir>/logs/desktop.log`，`OVERVIEW_LOG_FILE` 优先）；`Makefile` 的 `DESKTOP_LDFLAGS` 与 CI 矩阵在 Windows 附加 `-H windowsgui`；`internal/logging` 用 `fanout` 替换 `io.MultiWriter`。

**验证记录**

- `go build ./...`、`go test ./...` 全绿（新增 `internal/sync`、`internal/server/sync_test.go`、`internal/app/sync_test.go` 及 index tombstone/`changed_seq` 覆盖）。
- `cd web && npm run build` 通过。
- headless：`bin/overview.exe` 重编为 v0.15.0 并以 `OVERVIEW_AUTH=multi` 重启 demo；`/api/v1/sync/manifest` 可用且带 ETag/304。
- **本轮未验证**：真实双机同步（同一 vault 的两端收敛）、S3 后端的附件同步（预期跳过）、macOS/Linux 桌面壳。

### 13.27 桌面引导 · owner 权限 · 同步补齐（v0.16.0，ADR-077…083）

**无鉴权 owner（ADR-077）**

- `internal/server/server.go` 的 `authMiddleware` 在 `!s.opts.Auth.Required()` 时用 `withUser` 注入
  `ownerUser = core.User{ID:"owner", Username:"owner", Role:core.RoleAdmin}`；`/auth/me` 在
  `auth=none` 下返回 `{username:"owner", role:"admin"}`（`internal/server/auth_none_test.go`）。
- 前端 `web/src/stores/auth.ts` 新增 `isAdmin = computed(() => mode.value === "none" || user.value?.role === "admin")`；
  `SettingsView.vue` 与各 `settings/*Section.vue` 改用 `auth.isAdmin`（此前直接比较 `user.role`）。

**桌面引导与 vault 生命周期（ADR-078）**

- `internal/config/desktop.go`：`DesktopConfig{DataDir}`、`DesktopConfigPath()`（`<UserConfigDir>/Overview/desktop.json`）、
  `LoadDesktopConfig`（缺文件返回零值、容忍 BOM）、`SaveDesktopConfig`（建目录 + 原子 temp→rename）；
  `HasVault(dir)`（`notes/` 或 `overview.db`）、`PrepareDataDir(dir)`（拒绝非目录/非空非 vault、`EnsureDirs`、写测试），
  `isDirEmpty`/`checkWritable` 辅助。
- `cmd/overview-desktop/main.go`：`resolveDataDir` 改为「显式 → 持久化配置 → 默认」；`chooseDataDir` 决定是否
  落盘；首启经 `runOnboarding`（`vault.go`：默认/打开已有/新建，原生 `Question`/`OpenFile` 对话框，
  非法目录重新提示）；`waitForPredecessor` 在启动时按 `OVERVIEW_RESTART_WAIT_PID` 等待前任退出。
- `cmd/overview-desktop/vault.go`：`changeVault`（显式目录或目录选择器、`PrepareDataDir`、写配置）、
  `restart`/`spawnRestart`/`restartEnv`（脱离进程重启，非显式时丢弃 `OVERVIEW_DATA_DIR` 以采用新配置）、
  `confirm`/`errorDialog`/`pickDirectory`。
- `internal/server/desktop.go`：`DesktopHooks` 增 `SetVault`/`Restart`/`OpenDataDir`；`PUT /desktop/vault`
  （空 body 视为空 `dataDir` → 目录选择器）、`POST /desktop/restart`、`POST /desktop/open-folder`。
- 前端 `DesktopSection.vue`：数据目录行 + 「打开数据目录」「更改数据目录」+ 重启提示与「立即重启」；
  `api.ts` 增 `changeVault`/`restartDesktop`/`openDesktopFolder` 与 `DesktopVaultResult`；i18n 增
  `desktop.dataDirHint`/`changeDataDir`/`choosing`/`openDataDir`/`restartNeeded`/`restartNow`/`restarting`/`restartManual`。

**实时与增量同步（ADR-079…082）**

- `internal/service/broadcast.go`：`ChangeBus`（`Subscribe`/`Unsubscribe`/`Publish`，容量 1 的非阻塞扇出）、
  `ChangeEvent{LatestSeq, LatestTs}`；`service.go` 增 `ChangeBus()`/`CurrentChangeCursor`/`Changes`，
  `notifyChange` 在 Save/SaveRaw/Delete/Move/Mkdir/ReindexPath/Reindex 后发布；Delete/Move 增
  `folderVictims`/`recordFolderTombstones`，Mkdir 增 `recordFolderChange`。
- `internal/core/model.go`/`ports.go`：`FolderTombstone`/`SyncChange`/`SyncChanges` 与 `core.ChangeLog` 端口。
- `internal/index/changes.go`：`LatestSeq`（笔记与目录序号取 max）、`ChangesSince`（`ORDER BY changed_seq, id`，
  `limit+1` 判 `hasMore`）、`TombstonesSince`、`FolderTombstonesSince`、`LatestTombstoneTime`、
  `RecordFolderChange`（事务内 `nextChangeSeq` + upsert）、`RecordFolderTombstone`；迁移 `0011`/`0012`。
- `internal/server/sync.go`：`handleSyncChanges`（参数校验、`svc.Changes`）、`handleSyncEvents`
  （`http.NewResponseController` 清写超时、SSE 头、`bus.Subscribe`、25s 心跳、`writeChangeEvent`）；
  `internal/server/server.go` 注册两条路由，`statusRecorder` 增 `Unwrap`/`Flush`。
- `internal/app/app.go`：`http.Server.BaseContext` 绑定 `baseCtx`，`Stop` 时 `baseCancel` 让 SSE 流返回。
- `internal/sync/client.go`：`RemoteClient.stream`（无整体超时的独立 client）、`Changes`、`Events`/`streamEvents`
  （解析 `event:/data:`、忽略 `:` 心跳、指数退避重连）。
- `internal/sync/engine.go`：`Run` 增 `go runSSE`；`runSSE`/`restartSSE`/`onRemoteEvent`（500ms debounce）/
  `sseTriggeredSync`/`remoteChanged`；`SyncOnce` 加 `execMu` 单飞、读 `fetchChanges` 头、写 `state.cursor`；
  `fetchManifest` 改为始终全量（目录成员不被 ETag 覆盖）；`plan` 增目录三方（`actFolderSync`/
  `actFolderDeleteLocal`/`actFolderDeleteRemote`/`actFolderForget`，`folderTombstones`、`remoteFolderEmpty`/
  `localFolderEmpty`、`sortByDepthDesc`）；`recordFolderBase`/`forgetFolder`；超大附件 `setWarning` 跳过。
- `internal/sync/state.go`：`cursorState{LatestSeq, LatestTs}` 与 `stateData.Cursor`。
- `internal/s3store/s3store.go`：`var _ core.AssetRestorer` + `Restore(ctx, rel, r)`（`PutObject` 至相对 key）。

**验证记录**

- `go build ./...`、`go test ./...` 全绿（新增 `internal/config/desktop_test.go`、`internal/index/changes_test.go`、
  `folder_changes_test.go`、`internal/server/auth_none_test.go`/`error_code_test.go`/`mkdir_changes_test.go`/
  `sync_changes_test.go`、`internal/s3store/s3store_test.go`、`internal/sync/client_test.go`/`engine_test.go`、
  `cmd/overview-desktop/main_test.go` 等覆盖）。
- `cd web && npm run build`（`overview-web@0.16.0`）通过。
- headless：`bin/overview.exe` 重编为 v0.16.0 并以 `OVERVIEW_AUTH=multi` 重启 demo；`/sync/changes` 返回增量、
  `/sync/events` 保持 SSE 流。
- **本轮未验证**：真实双机的事件驱动收敛、多实例下的 SSE（预期不跨节点）、S3 后端的按路径附件同步、
  桌面壳（macOS/Linux）与原生引导对话框的人工回归。

### 13.28 桌面升级可见性与专注出口（v0.16.1，ADR-084…086）

**静态资源缓存校验（`internal/server/static.go`，ADR-084）**

- `weakETag(p, data)` 由「路径 + 字节长度」的哈希改为对 `p` 与**完整内容** `data` 做 FNV-1a 哈希；
  内容变化必然换 ETag，重建后字节数几乎不变的 `index.html` 不再被 304 命中。
- `serve` 在 `fs.Stat` 未命中时调用新增的 `isAssetRequest(p)`：`assets/` 前缀或带扩展名的路径
  返回 `http.NotFound`（404），只有无扩展名的客户端路由才回退 `index.html`。避免把 HTML 作为
  JS/CSS 返回造成 MIME 解析失败，也让「资源缺失」这一版本不一致信号不被掩盖。
- `internal/server/static_test.go` 新增覆盖：同长度不同内容的 `index.html` 需返回不同 ETag；
  缺失资源 404、未知无扩展名路由回退 `index.html`。

**桌面壳缓存破化（`cmd/overview-desktop/main.go`，ADR-085）**

- 新增 `desktopURL(base, version)`：返回 `base + "/?desktop=1&v=" + url.QueryEscape(version)`；
  窗口 `URL` 由硬编码 `base + "/?desktop=1"` 改为 `desktopURL(base, version)`。每次发版是新缓存
  条目，升级后 WebView2 首次打开即拉取新前端；`?desktop=1` 语义不变（前端据此跳过 Service Worker）。
- `cmd/overview-desktop/main_test.go` 更新 `desktopURL` 用例（含版本转义与 origin 校验）。

**专注模式出口（`web/src/App.vue` + `styles.css` + i18n，ADR-086）**

- 非 plain 分支内新增 `<button v-if="settings.focusMode" class="focus-exit" @click="settings.setFocus(false)">`，
  文案取 i18n `focus.exit`（zh「退出专注」/en「Exit focus」），`title` 复用已有 `shortcuts.exitFocus`。
- `.focus-exit` 为右下角浮动胶囊：`position:fixed` + `env(safe-area-inset-*)`、`z-index:90`、
  `color-mix` 半透明背景 + `backdrop-filter`，默认 `opacity:0.75`，`hover` 提升；`prefers-reduced-motion`
  兼容。

**验证记录**

- `go build ./...`、`go test ./...` 全绿（含新增/更新的 `internal/server/static_test.go`、
  `cmd/overview-desktop/main_test.go`）。
- `cd web && npm run build`（`overview-web@0.16.1`）通过。
- headless：`bin/overview.exe` 重编为 v0.16.1 并以 `OVERVIEW_AUTH=multi` 重启 demo；`/api/v1/health`
  返回 `version: 0.16.1`。
- **本轮未验证**：真实 WebView2 升级路径（旧缓存 → 新 bundle）的人工回归、macOS/Linux 桌面壳。

### 13.29 侧栏树文字对比度（v0.16.2，ADR-087）

**改动（`web/src/styles.css`，仅应用侧）**

- `.tree-row.folder`（基础规则）与其 `depth-0`/`depth-1`/`depth-2`/`depth-3` 变体：颜色由
  `--text-soft`（depth-2/3 原为 `--text-faint`）统一改为 `--text`。
- `.tree-row.note.depth-3`：颜色由 `--text-soft` 改为 `--text-muted`（depth-0/1/2 笔记本就用 `--text-muted`）。
- 层级编码不变：`padding-left` 缩进 + 文件夹「`text-transform:uppercase` + `font-weight:600`」；
  hover/active 与行内操作浮层未改。**未修改** `internal/sitegen/content.css` 的共享 token，
  因此静态站/公开页配色不受影响。

**验证记录（computed color，浏览器 devtools，浅色主题）**

- `.tree-row.folder.depth-2`：改前 `rgb(183, 183, 179)`（`--text-faint` #b7b7b3）→ 改后 `rgb(55, 53, 47)`
  （`--text` #37352f）。
- `.tree-row.note.depth-3`：改前 `rgb(155, 155, 155)`（`--text-soft` #9b9b9b）→ 改后 `rgb(107, 107, 107)`
  （`--text-muted` #6b6b6b）。
- `go build ./...`、`go test ./...`、`cd web && npm run build` 全绿；headless demo 重启后
  `/api/v1/health` 返回 `version: 0.16.2`。
- **本轮未验证**：暗色主题的 computed color 人工比对（token 本身未改，预期 `--text` #e9e6e0 /
  `--text-muted` #b3afa8）。

### 13.30 SPA 回退修正 · 404 体验 · 空状态入口（v0.16.3，ADR-088/089）

**静态资源判定（`internal/server/static.go`，ADR-088）**

- `isAssetRequest(p)` 删除「带扩展名」判定，改为 `strings.HasPrefix(p, "assets/") || staticRoots[p]`；
  `staticRoots` 白名单为 `icon.svg` / `icon-192.png` / `icon-512.png` / `manifest.webmanifest` /
  `sw.js` / `favicon.ico` / `robots.txt` / `sitemap.xml`。
- 结果：`/note/guide/start.md`、`/public/notes/a.md` 等带扩展名的客户端路由刷新时回退
  `index.html`（200，`text/html`）；`/assets/missing.js`、缺失的 `/favicon.ico` 等仍 404。
- `internal/server/static_test.go` 新增 `TestStaticNoteRoutesFallBackToSPA`、`TestStaticTopLevelWhitelist`，
  并更新 `TestStaticMissingAssetReturns404`（改为断言白名单缺失文件 404）。

**404 体验（`internal/server/notfound.go` + 前端，ADR-089）**

- 服务端 `writeNotFoundPage`：内联 CSS、`color-scheme: light dark`、无外部资源的 HTML 404；
  `staticHandler.serve` 在 `index.html` 读取失败时调用，`server.mountStatic` 的 `Static==nil`
  分支由 `writeError(..., "frontend not built")` 改为 `writeNotFoundPage`。
- 前端 `web/src/router.ts` 新增末尾 catch-all `/:pathMatch(.*)*` → `views/NotFoundView.vue`；
  render 模式 `beforeEach` 把 `not-found` 重定向 `public-home`。`EditorPane.vue` 新增 `missing`
  状态，`load()` 捕获 `ApiError` 且 `status===404` 时显示与 NotFoundView 一致的 `.notfound-card`
  覆盖卡片。i18n 新增 `notFound.title`/`notFound.desc`/`notFound.home`（zh/en）。
- `internal/server/notfound_test.go`：状态 404、`Content-Type: text/html`、正文含 "404"；
  无 `index.html` 的 FS 对 `/`、`/note/hello.md` 均返回 HTML 404。

**首页空状态入口（`web/src/components/EmptyState.vue`）**

- 新增「新建笔记」（`store.createNote("", name)` 后跳转笔记）/「新建文件夹」（`store.createFolder("", name)`）
  按钮，复用 `sidebar.newNote*`/`sidebar.newFolder*`/`sidebar.createFailed` i18n 与侧栏流程；
  专注模式隐藏侧栏时仍可从首页新建。

**验证记录**

- `go build ./...`、`go test ./...` 全绿（含新增/更新的 `internal/server/static_test.go`、`notfound_test.go`）。
- `cd web && npm run build`（`overview-web@0.16.3`）通过。
- headless：重编 `bin/overview.exe`（v0.16.3）并以 `OVERVIEW_AUTH=multi` 重启 demo；`/api/v1/health`
  返回 `version: 0.16.3`。路由/资源语义：`/note/x.md` → 200（HTML）、`/public/x.md` → 200（HTML）、
  `/assets/missing.js` → 404（非 HTML）、`/api/v1/nope` → 404（JSON）；首页空状态「新建笔记 / 新建文件夹」
  按钮在专注模式（侧栏隐藏）下可见可用。
- **本轮未验证**：真实桌面 WebView2 升级路径与 macOS/Linux 桌面壳（同前）。
