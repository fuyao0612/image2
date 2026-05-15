# API 路由说明

统一前缀：

```text
/api/v1
```

统一响应：

```json
{
  "code": 0,
  "message": "success",
  "data": {}
}
```

鉴权接口使用：

```text
Authorization: Bearer <accessToken>
```

## 认证

### 发送邮箱验证码

`POST /api/v1/auth/email-code`

Request:

```json
{
  "email": "test@example.com",
  "scene": "register"
}
```

Response:

```json
{
  "sent": true,
  "expiresIn": 300
}
```

说明：

- `APP_ENV=dev` 且 `EMAIL_DELIVERY=debug` 时会额外返回 `debugCode`，便于本地调试。
- `APP_ENV=prod` 时必须配置 SMTP，接口只返回 `sent` 和 `expiresIn`，验证码会发送到用户邮箱。

### 注册

`POST /api/v1/auth/register`

Request:

```json
{
  "email": "test@example.com",
  "code": "123456",
  "username": "snow",
  "password": "password123"
}
```

Response:

```json
{
  "accessToken": "jwt-token",
  "tokenType": "Bearer",
  "expiresIn": 86400,
  "user": {
    "id": "1",
    "username": "snow",
    "email": "test@example.com",
    "pointsBalance": 0,
    "freeCredits": 0,
    "paidCredits": 0,
    "totalCredits": 0,
    "checkinStreak": 0
  }
}
```

### 登录

`POST /api/v1/auth/login`

Request:

```json
{
  "account": "snow@example.com",
  "password": "password123"
}
```

`account` 支持用户名或邮箱。

## 用户和积分

### 当前用户

`GET /api/v1/user/me`

Response:

```json
{
  "id": "1",
  "username": "snow",
  "email": "test@example.com",
  "role": "user",
  "pointsBalance": 128,
  "freeCredits": 8,
  "paidCredits": 120,
  "totalCredits": 128,
  "checkinStreak": 3
}
```

### 每日签到

`POST /api/v1/user/checkin`

Response:

```json
{
  "checkedIn": true,
  "gainedPoints": 3,
  "streak": 1,
  "pointsBalance": 3,
  "freeCredits": 0,
  "paidCredits": 0,
  "totalCredits": 0,
  "lastCheckinAt": "2026-05-03T08:00:00+08:00"
}
```

如果当天已经签到，`checkedIn` 返回 `false`，不会重复加积分。

### 获取积分余额

`GET /api/v1/user/points`

Response:

```json
{
  "balance": 128,
  "freeCredits": 8,
  "paidCredits": 120,
  "totalCredits": 128,
  "streak": 3,
  "lastCheckinAt": "2026-05-03T08:00:00+08:00",
  "recentLogs": [],
  "recentCreditLogs": []
}
```

## 生图任务

### 提交生图任务

`POST /api/v1/image-tasks`

Request:

```json
{
  "prompt": "一只站在城市屋顶上的机械狐狸",
  "size": "1024x1024",
  "style": "cinematic",
  "count": 1,
  "quality": "auto",
  "outputFormat": "default"
}
```

Response:

```json
{
  "taskId": "1",
  "status": "generating",
  "balance": 127,
  "pointsBalance": 127,
  "freeCreditsBalance": 0,
  "paidCreditsBalance": 0,
  "totalCreditsBalance": 0
}
```

说明：

- 提交任务前会先进行内容安全规则审核，命中 `PROMPT_BLOCK_KEYWORDS` 时会直接拒绝，不扣积分，也不扣生图福利次数。
- 生图时优先消耗福利次数。
- 后端会异步调用第三方服务，前端通过任务详情接口轮询状态。
- 失败、超时、长期卡住时，任务会标记为 `failed` 并退还本次扣除的福利次数。
- 生成成功后，后端会根据 `IMAGE_STORAGE_DRIVER` 尝试把第三方图片转存到自有存储，并把自有图片 URL 写入任务。

### 查询任务详情

`GET /api/v1/image-tasks/:id`

Response:

```json
{
  "taskId": "1",
  "prompt": "一只站在城市屋顶上的机械狐狸",
  "imageUrl": "http://localhost:8080/uploads/images/2026/05/03/user-1/task-1-1.png",
  "imageUrls": ["http://localhost:8080/uploads/images/2026/05/03/user-1/task-1-1.png"],
  "costCredits": 1,
  "freeCreditsCost": 1,
  "paidCreditsCost": 0,
  "creditSource": "free",
  "status": "success",
  "createdAt": "2026-05-03T08:00:00+08:00"
}
```

失败时：

```json
{
  "taskId": "1",
  "prompt": "一只站在城市屋顶上的机械狐狸",
  "costCredits": 1,
  "freeCreditsCost": 1,
  "paidCreditsCost": 0,
  "creditSource": "free",
  "status": "failed",
  "errorMessage": "图片生成失败",
  "createdAt": "2026-05-03T08:00:00+08:00"
}
```

### 查询任务历史

`GET /api/v1/image-tasks?page=1&pageSize=10`

Response:

```json
{
  "items": [],
  "page": 1,
  "pageSize": 10,
  "total": 0
}
```

### 重试历史任务

`POST /api/v1/image-tasks/:id/retry`

说明：

- 只能重试非 `generating` 状态的任务。
- 后端会读取历史任务的请求快照，创建一条新任务。
- 重试会重新扣生图福利次数。

Response:

```json
{
  "taskId": "2",
  "status": "generating",
  "pointsBalance": 126,
  "freeCreditsBalance": 0,
  "paidCreditsBalance": 0,
  "totalCreditsBalance": 0
}
```

## 套餐

### 套餐列表

`GET /api/v1/packages?type=recharge|redeem`

Response:

```json
{
  "items": [
    {
      "code": "redeem-11-5",
      "name": "均衡兑换包",
      "packageType": "redeem",
      "creditsAmount": 5,
      "pointsCost": 11,
      "amountCents": 0,
      "bonusPoints": 0,
      "enabled": true,
      "sortOrder": 20,
      "note": "11 积分兑换 5 福利次数"
    }
  ]
}
```

## 后台管理

后台接口统一前缀：

```text
/api/v1/admin
```

权限说明：

- 必须携带普通登录的 `Bearer Token`。
- 只有 `role=admin` 的用户可以访问。
- 本地或生产可通过 `ADMIN_EMAILS=admin@example.com` 指定管理员邮箱，这些邮箱注册或登录后会自动拥有后台权限。

### 后台统计

`GET /api/v1/admin/stats`

Response:

```json
{
  "usersTotal": 10,
  "usersDisabled": 1,
  "tasksTotal": 30,
  "tasksGenerating": 1,
  "tasksSuccess": 25,
  "tasksFailed": 4,
  "auditRejected": 2,
  "pointsBalanceTotal": 1000,
  "freeCreditsTotal": 200,
  "paidCreditsTotal": 800
}
```

### 用户管理

`GET /api/v1/admin/users?page=1&pageSize=20&keyword=test`

`PATCH /api/v1/admin/users/:id/status`

```json
{
  "status": 2
}
```

`status=1` 表示正常，`status=2` 表示禁用。

`POST /api/v1/admin/users/:id/points-adjustments`

```json
{
  "delta": 10,
  "remark": "后台人工补偿"
}
```

正数加积分，负数扣积分；调整会写入 `PointsLog`，类型为 `adjust`。

### 全站任务

`GET /api/v1/admin/image-tasks?page=1&pageSize=20&status=success`

### 内容审核日志

`GET /api/v1/admin/content-audits?page=1&pageSize=20&status=rejected`

返回用户 prompt 的规则审核结果，包括命中的关键词、处理动作和原因。

### 套餐配置和审计

```text
GET /api/v1/admin/packages?page=1&pageSize=50&type=recharge
POST /api/v1/admin/packages
PATCH /api/v1/admin/packages/:id
GET /api/v1/admin/action-logs?page=1&pageSize=20&targetType=billing_package
```

说明：
- 后台可以配置商品档位和积分兑换档位。
- 创建或更新套餐会写入 `admin_action_logs`。
- 后端会保护每个套餐类型至少保留一个启用档位，避免前台无可用套餐。

## 限流防刷

当前已在这些入口启用限流：

- `POST /api/v1/auth/email-code`
- `POST /api/v1/auth/register`
- `POST /api/v1/auth/login`
- `POST /api/v1/user/checkin`
- `POST /api/v1/image-tasks`
- `POST /api/v1/image-tasks/:id/retry`

Redis 可用时使用 Redis 计数；本地开发没有 Redis 或 Redis 短暂异常时使用进程内存计数。限流命中时返回 HTTP `429`。
