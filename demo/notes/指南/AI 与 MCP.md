---
id: 01M3XSEASFWKER0KX1GXAV7KPM
title: AI 与 MCP
tags:
  - 指南
  - AI
public: true
created: "2026-10-02T08:00:00Z"
updated: "2026-10-02T08:00:00Z"
---

# AI 与 MCP

## AI 助手

右侧「AI」面板基于任意 OpenAI 兼容接口（OpenAI / DeepSeek / Ollama / vLLM）。
管理员可在「设置 → AI」中填写 Base URL、API Key 与模型，保存后即时生效。

- **对话**：就当前笔记提问
- **整理**：重写并结构化当前内容
- **补全**：在当前内容后继续写作

> 未配置时 AI 相关入口保持禁用，密钥仅在服务端保存、永不回传。

## MCP

Overview 内置 MCP 服务端（`POST /mcp`，JSON-RPC）。工具列表**由 OpenAPI 规范生成**，
与 REST API 保持同步，覆盖：

| 工具 | 作用 |
| --- | --- |
| `notes_list` | 列出目录树 |
| `notes_search` | 全文搜索 |
| `notes_read` | 读取笔记 |
| `notes_write` | 新建 / 覆盖 |
| `notes_delete` | 删除 |
| `notes_links` | 双链与反链 |

令牌认证使用 `OVERVIEW_MCP_TOKEN`（未设置且 `auth=multi` 时回退到会话令牌）。

返回 [[功能演示]]。
