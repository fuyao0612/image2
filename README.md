# AI 图像 SaaS 平台 (AI Image SaaS Platform)

这是一个前后端分离的现代化 AI 生图平台（类 Midjourney/Stable Diffusion 聚合后台服务），提供完整的用户系统、计费与福利体系、生图异步任务调度以及功能详尽的响应式管理员控制台。

## 技术栈 (Tech Stack)

### 前端 (Frontend)
- **框架**: React 18 + TypeScript + Vite
- **路由**: React Router DOM v7
- **样式**: Tailwind CSS (含 PostCSS/Autoprefixer) + 原生 CSS Variables
- **图标**: Lucide React
- **特点**: 单文件聚合后台 (`AdminConsole.tsx`)，实现了全端响应式（移动端卡片式表格、折叠抽屉等适配）；用户端和管理后台逻辑隔离。

### 后端 (Backend)
- **语言框架**: Go 1.26 + Gin (HTTP Web 框架)
- **ORM 与数据库**: GORM + MySQL / SQLite (支持双引擎)
- **缓存与队列**: Redis (限流、验证码防刷、验证码存储)
- **认证**: JWT (JSON Web Token)
- **存储**: 本地文件存储 + AWS S3 兼容协议支持 (MinIO, 腾讯云 COS, 阿里云 OSS 等)
- **邮件服务**: 支持 SMTP / Resend API / 腾讯云 SES (多渠道投递，可配置日额度与防刷限流)

## 核心功能特性 (Core Features)

### 1. 认证与用户体系
- **验证码注册/登录**: 支持多渠道邮箱发信验证，内建冷却、单日上限、防刷机制。
- **活跃跟踪**: 用户打开网页验证 Token 时自动触发活跃时间更新 (`last_login_at` 1小时节流)，方便后台追踪。
- **账号隔离**: 普通用户与管理员权限严格区分。环境变量中配置 `ADMIN_EMAILS` 的账号登录自动赋予后台管理权限。

### 2. 积分与计费系统 (Billing & Points)
- **双轨货币系统**: `Points` (积分) 和 `Credits` (福利次数/付费次数)。
- **每日签到**: 用户可通过每日签到获取基础积分与福利次数奖励。
- **兑换码系统**: 管理员可以批量生成兑换码，用户输入兑换码即可充值福利次数或积分。
- **动态消耗**: 生图操作可配置优先消耗福利次数，不足时抵扣积分；支持因参考图过多、邮件通知等额外加扣积分。
- **失败退款**: 如果生图异步任务失败或部分失败，系统会自动回滚扣除的积分和次数，并记录精确的账单日志 (CreditLog / PointsLog)。

### 3. AI 生图任务引擎
- **异步调度**: 提交生图需求后进入异步等待，前端通过短轮询查阅状态 (`generating` -> `success` / `failed`)。
- **多图床支持**: 后台支持对接外部生成 API (如 Mentalout)，生成成功后根据配置自动转存回本地或远端 S3 存储桶，防止链接失效。
- **内容审核**: 提交 Prompt 时会触发后端的关键词审核器，违规词将被阻断并记录审计日志。

### 4. 响应式管理员后台 (Admin Console)
- **多端适配**: 桌面端使用数据表格，移动端自动转化为高可用性的卡片列表。顶部导航、弹窗与侧边栏均针对手机端操作体验深度优化。
- **全站大盘**: 直观展示用户总数、任务数、积分消耗池与消耗趋势。
- **数据管理**: 支持用户日志审计、调整积分、生图任务记录与异常回溯、兑换码打包配置、图片套餐配置、敏感词等。

---

## 本地开发与启动 (Local Development)

### 后端启动

```powershell
cd backend
# 复制示例配置文件
Copy-Item .env.example .env

# 按需修改 .env 中的数据库和 Redis 配置
# 如果想本地轻量启动，默认使用 SQLite 即可 (SQLITE_PATH=data/app.db)

go run ./cmd/api
```

**后端服务默认启动在 `http://localhost:8080`，你可以访问 `http://localhost:8080/healthz` 检查健康状态。**

### 前端启动

```powershell
cd frontend
npm.cmd install
npm.cmd run dev
```

**前端服务默认启动在 `http://localhost:5173`。**
*注意：本地调试后台功能需在后端 `.env` 的 `ADMIN_EMAILS` 中配置你的邮箱，之后在前端使用该邮箱注册/登录。*

### 一键 Smoke Test

前端提供了一个基于 Playwright 的冒烟测试脚本，在前后端均启动的情况下可用于验证核心链路（需先安装 playwright 依赖）：

```powershell
cd frontend
npm.cmd run smoke
```

---

## 生产部署 (Production Deployment)

生产环境的完整部署说明、Nginx 配置及 Docker 支持情况请参阅：
👉 [部署指南 (deploy/README.md)](./deploy/README.md)

---

## 接口文档 (API Reference)

后端接口设计的请求响应规范、权限说明、限流机制和主要业务端点说明请参阅：
👉 [后端 API 手册 (backend/docs/api.md)](./backend/docs/api.md)
