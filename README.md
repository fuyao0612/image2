# AI 生成图平台

这是一个前后端分离的生成图项目，当前源码包含：

- `backend/`：Go + Gin + GORM 后端
- `frontend/`：React + TypeScript + Vite 前端
- `deploy/`：生产部署配置
- `scripts/`：辅助脚本

## 当前功能范围

- 邮箱验证码注册、登录
- 用户兑换码兑换
- 用户积分与福利次数资产
- 图片任务提交、轮询、失败回滚与图片下载
- 管理员概览、任务趋势、异常提醒
- 用户管理、用户日志、任务详情
- 兑换码管理、商品套餐、API Key 配置
- 公告、邀请、数据清理等后台能力

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

前后端都启动后，在 `frontend/` 目录执行：

```powershell
npm.cmd run smoke
```

## 生产部署

生产部署说明请直接看 [deploy/README.md](./deploy/README.md)。

## 说明

当前项目已移除旧的第三方订单 webhook 等历史流程，部署和验收都应以现有源码为准。
