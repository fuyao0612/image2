# AI 生图 SaaS MVP

这是一个前后端分离的 AI 生图平台，当前仓库包含：

- `backend/`：Go + Gin + GORM
- `frontend/`：React + TypeScript + TailwindCSS
- `deploy/`：生产部署文件和 Docker Compose 配置

## 主要能力

- 邮箱验证码注册
- 用户名或邮箱登录
- 每日签到得积分
- 积分兑换次数
- 充值套餐购买次数
- 提交生图任务
- 历史任务查看
- 图片下载
- 管理员后台

## 本地开发

### 后端

```powershell
cd backend
Copy-Item .env.example .env
go run ./cmd/api
```

健康检查：

```text
http://localhost:8080/healthz
```

### 前端

```powershell
cd frontend
npm.cmd install
npm.cmd run dev
```

前端地址：

```text
http://localhost:5173
```

### 一键 smoke

前后端都启动后，在 `frontend/` 目录下执行：

```powershell
npm.cmd run smoke
```

## 生产部署

生产部署说明请直接看 [deploy/README.md](./deploy/README.md)。

## 接入第三方生图服务

后端通过第三方生图站点的批次接口提交任务，核心请求路径是：

- `POST https://image.mentalout.top/api/batches`
- `GET https://image.mentalout.top/api/batches/{batchId}`

本仓库已经封装了提交、轮询、失败回滚和图片下载链路。
