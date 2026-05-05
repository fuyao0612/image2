# 生产部署说明

这套部署方案采用单仓库、前后端分目录的方式：

- `web` 容器负责对外提供前端页面
- `backend` 容器负责 API、鉴权、积分、任务和图片下载
- `mysql` 和 `redis` 作为独立服务运行

## 部署前准备

1. 复制生产环境模板：

```powershell
Copy-Item .env.production.example .env.production
```

2. 打开 `.env.production`，重点确认这些值：

- `BACKEND_PUBLIC_URL`
- `VITE_API_BASE_URL`
- `JWT_SECRET`
- `CORS_ALLOW_ORIGINS`
- `ADMIN_EMAILS`
- `SMTP_*`
- `IMAGE_PROVIDER_API_KEY`
- `MYSQL_ROOT_PASSWORD`
- `MYSQL_PASSWORD`
- `REDIS_PASSWORD`
- `REGISTRATION_ENABLED=false` 可以临时关闭前后端注册入口

3. 如果使用本地图片存储，请确认：

- `IMAGE_STORAGE_DRIVER=local`
- `IMAGE_STORAGE_LOCAL_DIR=/app/storage/uploads`
- 服务器会持久化挂载 `backend_storage` 卷

4. 如果后续要切到对象存储，把 `IMAGE_STORAGE_DRIVER` 改成 `s3`，再补齐对应的 S3/R2/MinIO/COS 配置。

## 启动命令

在服务器上执行：

```powershell
docker compose -f deploy/docker-compose.prod.yml --env-file deploy/.env.production up -d --build
```

## 常用检查命令

```powershell
docker compose -f deploy/docker-compose.prod.yml ps
docker compose -f deploy/docker-compose.prod.yml logs -f backend
docker compose -f deploy/docker-compose.prod.yml logs -f web
```

健康检查：

```powershell
curl http://127.0.0.1/healthz
```

## 目录说明

- MySQL 数据卷：`mysql_data`
- Redis 数据卷：`redis_data`
- 图片存储卷：`backend_storage`

上传后的图片会保存在后端容器挂载卷里，再通过 `/uploads` 暴露给前端。
