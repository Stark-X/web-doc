# Web-Doc — 自托管 HTML 文档站

**Language / 语言**: [English](./README.md) · **中文**

> 像管理 Markdown 笔记一样，管理 AI 生成的 HTML 文档。
> 树形工作区 · 沙箱化实时预览 · 文件变更热更新 · 一键分享 · 内置 AI 创作 · 面向 Agent 的 MCP 服务。

Web-Doc 是一个自托管的单二进制应用，让你可以组织、编辑、预览并分享单文件或多文件的 HTML 文档。每个文档在磁盘上独占一个目录，可通过 Monaco 编辑器编辑，也可由大模型流式生成，并在受沙箱保护的 iframe 中渲染。

---

## ✨ 功能特性

### 文档管理
- **树形工作区**：支持文件夹与文档（创建 / 重命名 / 删除 / 移动）。
- **拖拽**：同级排序、跨文件夹移动（基于 `dnd-kit`）。
- **单文件文档**：直接粘贴 HTML 源码或上传 `.html` 文件。
- **多文件文档**：上传包含 `index.html` 与静态资源（js/css/图片/字体/...）的 `.zip`。
- **文档内文件浏览**：在工具栏切换并编辑文档下的任意文件。

### 编辑器与预览
- **Monaco 代码编辑器**：`预览` · `分屏` · `代码` 三模式切换，`⌘S` / `Ctrl+S` 保存。
- **沙箱化 iframe 预览**：通过独立路径 `/d/{docId}/` 隔离，并启用 `sandbox` 属性。
- **实时热更新**：文件系统监听（`fsnotify`）通过 WebSocket 推送变更，文档目录下任意文件变化（包括 AI 流式写入）都会自动刷新预览。

### AI 创作
- **流式生成**：兼容 OpenAI Chat Completions 协议，支持 OpenAI、DeepSeek、Kimi、智谱（GLM）、通义、OpenRouter 以及任何兼容网关。
- 每位用户可独立配置 `Base URL`、`API Key`、`Model`、`System Prompt`、`Temperature`、`MaxTokens`。
- 两种模式：**生成新文档** 或 **改写当前文档**。
- **边生成边落盘**：内容实时写入文档目录，预览以约 300ms 节流刷新。
- **AI 对话面板**：编辑过程中与模型多轮交互。
- **可复用 Prompt 模板**：增删改查个人 Prompt 预设。
- **AI 辅助文档树整理**：同时也提供手动批量移动接口（`/api/nodes/reorder/batch`）。

### 分享
- 为任意文档生成公开 **分享短链**。
- 在 `/s/{shareToken}` 提供干净、无干扰的纯净预览页。
- **纯静态分享** `/p/{shareToken}/`：直接输出文档文件本身，无主站外壳、无 iframe；响应携带 CSP `sandbox`，文档脚本运行在 opaque origin，无法读取访问者在主站的登录态。

### 认证与多用户
- 基于 **JWT** 的用户名/密码注册与登录。
- AI 设置、Prompt、MCP Token 与文档归属均按用户隔离存储。
- 可通过 `WEBDOC_DISABLE_REGISTER=1` 关闭注册，仅允许已存在用户登录。

### MCP 服务（面向 AI Agent）
Web-Doc 内置 **Model Context Protocol** 服务端点，使 Agent 可以编程式管理文档：

| 工具 | 用途 |
|---|---|
| `list_documents` | 列出完整文档树（带 `parentId`/`type` 的扁平列表）。 |
| `get_document` | 获取文档元信息及文件清单。 |
| `create_document` | 创建文档或文件夹（可选附带初始 HTML）。 |
| `delete_document` | 递归删除节点。 |
| `read_document_file` | 读取文档内任意文本文件（默认 `index.html`）。 |
| `upload_html` | 写入/覆盖文档下单个文件。 |
| `upload_zip_base64` | 用 base64 编码的 zip 整体替换文档。 |

端点为 **JSON-RPC 2.0 over Streamable HTTP**：`POST /mcp`，使用用户在 UI 中签发的 Bearer Token 鉴权。

### UI / UX
- 现代 UI：**TailwindCSS + shadcn/ui**。
- 左侧浮动抽屉，默认收起，悬停展开。
- 轻量、快速、响应式。

---

## 🧱 项目结构

```
web-doc/
├── apps/
│   ├── api/                    # Go 后端（Gin）— REST + 静态资源 + 文件监听 + MCP
│   │   ├── cmd/server/         # 主入口
│   │   └── internal/
│   │       ├── ai/             # OpenAI 兼容的流式客户端
│   │       ├── auth/           # JWT 工具
│   │       ├── config/         # 基于环境变量的配置
│   │       ├── db/             # GORM + SQLite（可选 Postgres）
│   │       ├── handler/        # HTTP 处理器（REST、MCP、认证、AI 重排）
│   │       ├── model/          # GORM 模型 + 初始数据
│   │       ├── storage/        # 文件系统抽象（每个文档一个目录）
│   │       └── watcher/        # fsnotify → WebSocket 推送中心
│   └── web/                    # React 19 + Vite + TypeScript SPA
│       └── src/
│           ├── components/     # AIChatPanel、AISettingsDialog、DocTree、DocViewer 等
│           ├── pages/          # HomePage、SharePage
│           └── store/          # Zustand 状态（auth、docs、aiChat）
├── storage/docs/               # 默认文档存储根目录（每个文档一个子目录）
├── docker-compose.yml          # server + SQLite
├── Dockerfile                  # 多阶段构建（web + api → 单镜像）
└── package.json                # 根工作区脚本（concurrently）
```

---

## 🚀 快速开始

### 方式 A — 本地开发（一条命令）

需要 **Node ≥ 18** 与 **Go ≥ 1.21**。默认使用 SQLite，不需要额外准备数据库服务。

```bash
# 安装根目录与 web 子项目依赖
npm run install:all

# 同时启动 API（:8787）与 Web（:5173）
npm run dev
```

然后访问：

- 应用：<http://localhost:5173>
- 文档静态资源路径：`http://localhost:8787/d/{docId}/index.html`
- 公开分享：<http://localhost:5173/s/{shareToken}>
- 纯静态分享：`http://localhost:5173/p/{shareToken}/index.html`

### 方式 B — 生产构建（单二进制）

```bash
npm run build           # 先构建 web → apps/web/dist，再构建 api → dist/webdoc-server
WEBDOC_WEB_ROOT=$(pwd)/apps/web/dist npm start
```

Go 服务会在同一个端口（默认 `:8787`）上同时提供 SPA、REST API、文档静态资源以及 MCP 端点。

### 方式 C — Docker Compose（推荐自托管使用）

```bash
docker compose up -d
```

这会基于 `Dockerfile` 在本地构建 Web-Doc 镜像，并在 <http://localhost:8787> 启动 Go 服务。文档文件和 SQLite 数据库会持久化到名为 `data` 的 volume。

---

## ⚙️ 配置项

所有配置均通过环境变量提供。

| 变量名 | 默认值 | 说明 |
|---|---|---|
| `WEBDOC_ADDR` | `:8787` | 监听地址。 |
| `WEBDOC_STORAGE` | `../../storage/docs` | 文档目录的根路径。 |
| `WEBDOC_WEB_ROOT` | _(空)_ | 设置后，服务也会从该路径托管 SPA（含 SPA fallback）。 |
| `WEBDOC_ORIGIN` | `*` | CORS 白名单（逗号分隔；`*` 表示全部允许）。 |
| `WEBDOC_JWT_SECRET` | _(默认值不安全)_ | **生产环境必须修改。** 用于签发 JWT 的 HMAC 密钥。 |
| `WEBDOC_DISABLE_REGISTER` | _(未设置)_ | 设为 `1` 时关闭公开注册接口。 |
| `WEBDOC_SHARE_BASE_URL` | _(未设置)_ | 可选的纯静态分享公开 URL 前缀。例如反代暴露 `/p/` 时设为 `https://share.example.com/p`；若分享域根路径重写到后端 `/p/`，设为 `https://share.example.com`。 |
| `WEBDOC_DB_DRIVER` | `sqlite` | 数据库驱动：`sqlite` 或 `postgres`。 |
| `WEBDOC_DB_PATH` | `../../storage/webdoc.db` | SQLite 数据库文件路径。 |
| `WEBDOC_DSN` | _(未设置)_ | 完整数据库 DSN；设置后会自动推断驱动，除非同时设置 `WEBDOC_DB_DRIVER`。 |
| `WEBDOC_PG_HOST` | `127.0.0.1` | Postgres 主机，仅在 `WEBDOC_DB_DRIVER=postgres` 且未设置 `WEBDOC_DSN` 时使用。 |
| `WEBDOC_PG_PORT` | `5432` | Postgres 端口。 |
| `WEBDOC_PG_USER` | `webdoc` | Postgres 用户。 |
| `WEBDOC_PG_PASSWORD` | `webdoc` | Postgres 密码。 |
| `WEBDOC_PG_DB` | `webdoc` | Postgres 数据库名。 |
| `WEBDOC_PG_SSLMODE` | `disable` | Postgres SSL 模式。 |
| `WEBDOC_PG_TZ` | `UTC` | Postgres 时区。 |

单文档上传体积默认上限为 **50 MB**（详见 `config.go` 中的 `MaxUploadMB`）。

---

## 🌐 HTTP API 一览

### 认证
- `GET  /api/auth/public-info` — 站点公开信息（如是否开放注册）。
- `POST /api/auth/register` · `POST /api/auth/login`
- `GET  /api/auth/me` — 获取当前用户（需 `Authorization: Bearer <jwt>`）。

### 节点（文件夹与文档）
- `GET    /api/nodes` · `POST /api/nodes` · `GET /api/nodes/:id` · `PATCH /api/nodes/:id` · `DELETE /api/nodes/:id`
- `PATCH  /api/nodes/reorder/batch` — 拖拽批量移动。

### 文档
- `POST /api/docs/:id/html` — 上传单个 `.html`。
- `POST /api/docs/:id/zip` — 上传多文件 `.zip`。
- `GET  /api/docs/:id/file?path=...` · `POST /api/docs/:id/file` — 读取/保存单个文件。

### 分享
- `POST /api/docs/:id/share` — 签发公开分享 token。
- `GET  /api/shares/:token` — 解析分享 token（供公开页使用）。

### AI
- `GET   /api/ai/settings` · `PATCH /api/ai/settings`
- `POST  /api/ai/generate` — 流式生成 / 改写。
- `GET   /api/ai/prompts` · `POST /api/ai/prompts` · `PATCH /api/ai/prompts/:id` · `DELETE /api/ai/prompts/:id`

### MCP
- `GET   /api/mcp/tokens` · `POST /api/mcp/tokens` · `DELETE /api/mcp/tokens/:id`
- `POST  /mcp` — JSON-RPC 2.0 端点（Bearer 鉴权），供 Agent 调用。

### 静态资源 / 实时
- `GET   /d/:id/*path` — 文档静态资源（沙箱化独立路径）。
- `GET   /p/:token/*path` — 纯静态分享（通过分享 token 直接访问文档文件，始终携带 CSP `sandbox`）。
- `GET   /ws/docs/:id` — WebSocket：推送变更事件用于热更新。
- `GET   /healthz` — 健康检查。

---

## 🧰 技术栈

| 层 | 技术 |
|---|---|
| 前端 | React 19 · Vite · TypeScript · TailwindCSS · shadcn/ui · Zustand · React Router · **Monaco Editor** · **dnd-kit** |
| 后端 | Go · **Gin** · **GORM** · **SQLite**（可选 Postgres）· `fsnotify` · `gorilla/websocket` · JWT |
| 存储 | 本地文件系统（每个文档一个目录） + 默认 SQLite（元数据、用户、AI 设置、Prompt、MCP Token） |
| 部署 | 单 Go 二进制 · Docker · Docker Compose · 可选外部反向代理 |

---

## 🔒 安全设计

- iframe `sandbox` 属性强隔离用户编写的 HTML / JS。
- `/d/`、`/p/` 响应默认携带 CSP `sandbox` 头（无 `allow-same-origin`）：文档以 opaque origin 运行，无法读取主站 localStorage / Cookie（防止上传文档中的脚本窃取访问者 JWT）。仅主站编辑器的同源 iframe 加载（按 `Sec-Fetch-Dest` / `Sec-Fetch-Site` 判定）豁免 sandbox，改为 `frame-ancestors 'self'` 防外站嵌套。
- `WEBDOC_SHARE_BASE_URL` 可将纯静态分享链接指向独立 origin。生产环境建议配合反向代理，让该 Host 只暴露静态分享路径。
- 文档静态资源固定挂载在 `/d/` 前缀，禁用目录列表。
- 路径穿越防护（拒绝 `..` 与以 `/` 开头的绝对路径）。
- ZIP 上传扩展名白名单（html / js / css / png / jpg / svg / woff2 / ...）。
- 单文档上传体积限制（默认 50 MB）。
- 所有写操作均需 JWT 鉴权，用户间数据隔离。
- 任何非开发部署都 **必须** 覆盖默认的 `WEBDOC_JWT_SECRET`。

---

## 📄 License

详见仓库中的 License 信息。
