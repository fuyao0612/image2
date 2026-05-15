# 生产部署说明

本项目当前以最新源码为准，正式部署目录为：

- `backend`：后端 API、鉴权、积分、福利次数、任务、兑换码、商品套餐、API Key 配置、数据清理等能力
- `frontend`：用户端与管理员后台
- `deploy`：生产容器与 Nginx 配置
- `scripts`：辅助部署脚本

不要使用历史构建产物或临时目录作为部署来源，例如 `.artifacts`、`frontend/dist`、旧压缩包、`deploy-current`、`deploy-lite`、`recover-*` 等。它们如存在也只应视为临时产物，可以重新生成，不代表最新版本。

## 当前业务范围

当前系统以兑换码、商品套餐、API Key 和图片任务为核心：

- 用户注册、登录、邮箱验证码与图形验证码
- 用户兑换码兑换
- 用户积分与福利次数资产展示
- 图片生成任务提交、状态查询、结果展示、任务日志
- 管理员概览、任务趋势、异常提醒
- 用户管理、用户日志、任务详情
- 兑换码生成、复制、状态统计
- 商品套餐管理
- API Key 切换与配置
- 公告、邀请、数据清理等后台维护能力

已废弃的订单、第三方平台 webhook、历史类型套餐不应再作为部署或验收依据。

## 环境配置

部署前复制生产环境模板：

```powershell
Copy-Item deploy/.env.production.example deploy/.env.production
```

重点检查以下配置：

- `BACKEND_PUBLIC_URL`
- `VITE_API_BASE_URL`
- `JWT_SECRET`
- `CORS_ALLOW_ORIGINS`
- `ADMIN_EMAILS`
- `EMAIL_DELIVERY`
- `RESEND_API_KEY`
- `SMTP_FROM`
- `SMTP_FROM_NAME`
- `IMAGE_PROVIDER_API_KEY`
- `MYSQL_ROOT_PASSWORD`
- `MYSQL_PASSWORD`
- `REDIS_PASSWORD`
- `REGISTRATION_ENABLED`
- `VITE_REGISTRATION_ENABLED`

如使用本地图片存储，保持：

```env
IMAGE_STORAGE_DRIVER=local
IMAGE_STORAGE_LOCAL_DIR=/app/storage/uploads
```

如后续切换对象存储，将 `IMAGE_STORAGE_DRIVER` 改为 `s3` 并补齐对应 S3/R2/MinIO/COS 配置。

## 当前线上部署方式

当前线上服务器使用轻量替换流程：本地测试、本地构建、本地打包、上传产物，然后用 `docker cp`
替换运行中容器的文件。不要在服务器执行 `docker build`、`docker compose up --build`
或会重建 MySQL/Redis/volume 的命令，也不要覆盖服务器上的 `deploy/.env.production`。

本地需要准备的产物：

```powershell
cd backend
go test ./...
$env:CGO_ENABLED='0'
$env:GOOS='linux'
$env:GOARCH='amd64'
go build -o api ./cmd/api
Remove-Item Env:CGO_ENABLED,Env:GOOS,Env:GOARCH

cd ..\frontend
npm.cmd run build

cd ..
tar -czf frontend-dist.tgz -C frontend\dist .
```

上传后只替换 `deploy_backend_1:/app/api` 和 `deploy_web_1:/usr/share/nginx/html/`，
MySQL、Redis、上传图片目录和 Docker volume 均保持不动。

## 常用检查

```powershell
docker compose -f deploy/docker-compose.prod.yml ps
docker compose -f deploy/docker-compose.prod.yml logs -f backend
docker compose -f deploy/docker-compose.prod.yml logs -f web
curl http://127.0.0.1/healthz
```

## 运行时数据

这些目录是本地运行或容器运行产生的数据，不应当作为源码部署：

- `logs`
- `backend/logs`
- `backend/data`
- `backend/storage`
- `frontend/dist`
- `.artifacts`

数据库、Redis、上传图片在生产环境中由 Docker volume 或正式存储服务持久化。
