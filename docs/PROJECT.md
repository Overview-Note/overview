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
- **MCP 服务端**：`POST /mcp`（JSON-RPC 2.0），暴露 `notes_list/search/read/write/delete/links`
- **AI 助手**：对话 / 整理（替换）/ 补全（追加），支持 OpenAI、DeepSeek、Ollama、vLLM 等
- 管理员可在**设置**中运行时配置，无需重启

### 7. 多端接入
- **WebDAV**：在 Obsidian / Finder / 移动端直接挂载读写
- **REST API**：`/api/v1`，OpenAPI 规范 + `/api/docs`
- **PWA**：可安装的应用壳

### 8. 现代而克制的界面
参考 Starlight 文档站的布局：全宽顶栏 + 居中搜索（`Ctrl/⌘ + K`）、左侧分组式层级导航、
内容区大标题 + 右侧「大纲」栏；全站字号派生自 `--base-size`，随设置整体缩放；明暗主题随系统。

---

## 功能清单（当前版本）

**内容与组织**：层级目录 · Markdown 存储 · 双链与反链 · 每篇私有/公开 + 独立公开页
**编辑**：富文本（Tiptap）· 图片粘贴/拖拽 + 压缩 · 表格 + 就近浮动工具条 · 斜杠命令 · 大纲
**检索**：CJK 全文检索 · 全局搜索
**AI**：AI 助手（对话/整理/补全）· MCP 服务端
**访问**：多用户认证（bcrypt/会话/角色）· WebDAV · REST + OpenAPI · PWA
**运维**：单二进制 · SQLite 迁移 · 原子写 + 乐观并发 · 版本历史 · 回收站 · 增量索引 + 文件监视 · S3 附件后端 · 设置中心

---

## 技术栈

| 层 | 选型 |
| --- | --- |
| 后端 | Go 1.26，标准库 `net/http`（方法路由）+ 手写中间件 |
| 存储 | Markdown 文件 + YAML frontmatter；SQLite（纯 Go `modernc.org/sqlite`） |
| 检索 | SQLite FTS5 + 自研 CJK 分词 |
| 前端 | Vue 3 + TypeScript + Vite 6 + Pinia + vue-router |
| 编辑器 | Tiptap 2（ProseMirror） |
| AI/MCP | OpenAI 兼容客户端（无 SDK）+ 自研 JSON-RPC MCP 服务端 |
| 集成 | WebDAV（`x/net/webdav`）、S3（`minio-go`）、fsnotify |
| 发布 | 前端 `go:embed` → 单二进制；Docker 三阶段构建 |

---

## 项目状态

- **版本**：v0.9.0（架构稳定，功能覆盖路线图前四阶段）
- **测试**：`go test ./...` 覆盖核心包（store/index/textproc/service/server/mcp 等）+
  `httptest` 集成测试；前端 `vue-tsc` 类型检查 + `vite build`
- **CI**：GitHub Actions（后端 race 测试、前端类型检查构建、golangci-lint）
- **规模**：后端 ~6k 行 Go / 17 个 internal 包；前端 ~3k 行 TS/Vue

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

**接下来的方向**：移动端原生 App · 实时协作编辑 · 标签管理 UI · 知识图谱视图。

---

## 许可

[MIT](../LICENSE) © Overview contributors
