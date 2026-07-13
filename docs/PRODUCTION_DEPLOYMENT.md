# 线上部署教程

本文档说明 StarPay 支付网关的生产部署方式。推荐使用前后端分离部署：后端通过 Docker Compose 运行 API、PostgreSQL、Redis，前端构建为静态文件后由 Nginx 托管，并由同一个域名反向代理 `/v1/` 到后端。

## 1. 服务器准备

建议配置：

- Linux 服务器，2C4G 起步。
- Docker 与 Docker Compose。
- Nginx。
- 一个 HTTPS 域名，例如 `pay.example.com`。
- 已开放 `80`、`443` 端口。

安装依赖示例：

```bash
sudo apt update
sudo apt install -y nginx git
curl -fsSL https://get.docker.com | sudo sh
sudo usermod -aG docker $USER
```

重新登录服务器后确认：

```bash
docker version
docker compose version
nginx -v
```

## 2. 拉取代码

```bash
mkdir -p /opt/starpay
cd /opt/starpay
git clone git@codeup.aliyun.com:h-star/pay-gateway.git
cd pay-gateway
```

如果服务器不能直接访问私有仓库，先配置部署密钥或通过 CI/CD 上传代码包。

## 3. 配置生产环境变量

复制生产示例文件：

```bash
cp .env.production.example .env.production
```

编辑 `.env.production`，至少修改以下配置：

```dotenv
APP_ENV=production
HTTP_ADDR=:8080

HTTP_PORT=8080
POSTGRES_PORT=15434
REDIS_PORT=16380

POSTGRES_DB=payment_gateway
POSTGRES_USER=payment
POSTGRES_PASSWORD=替换为强密码

JWT_SECRET=替换为随机长密钥
REFRESH_COOKIE_SECURE=true
REFRESH_COOKIE_SAME_SITE=lax

APP_SECRET_ENCRYPTION_KEY=替换为16/24/32字节随机值

ORDER_DEFAULT_TTL=15m
ORDER_EXPIRE_SCAN_INTERVAL=30s
ORDER_EXPIRE_SCAN_LIMIT=100
ORDER_EXPIRE_WORKER_CONCURRENCY=2
```

注意：

- `JWT_SECRET` 必须是生产随机值。
- `APP_SECRET_ENCRYPTION_KEY` 必须是 16、24 或 32 字节。上线后不要随意更换，否则已有应用密钥需要重置。
- `POSTGRES_PASSWORD` 不要使用默认值。
- 生产建议不要把 PostgreSQL 和 Redis 端口暴露到公网，只允许本机或内网访问。

生产环境优先使用 `docker-compose.prod.yml`。该文件默认只把 API 端口绑定到 `127.0.0.1`，PostgreSQL 和 Redis 不映射宿主机端口，避免直接暴露到公网。

## 4. 启动后端服务

项目自带 `docker-compose.prod.yml`，会构建 API 镜像并启动 PostgreSQL、Redis：

```bash
docker compose --env-file .env.production -f docker-compose.prod.yml up -d --build
```

`postgres:latest` 当前可能是 PostgreSQL 18+。该版本官方镜像要求把数据卷挂载到 `/var/lib/postgresql`，项目的生产 Compose 已按该路径配置。不要改回旧路径 `/var/lib/postgresql/data`，否则会出现 `in 18+, these Docker images are configured to store database data...` 的启动错误。

查看状态：

```bash
docker compose --env-file .env.production -f docker-compose.prod.yml ps
docker compose --env-file .env.production -f docker-compose.prod.yml logs -f api
```

健康检查：

```bash
curl http://127.0.0.1:8080/healthz
curl http://127.0.0.1:8080/v1/ping
```

正常返回示例：

```json
{"code":"ok","data":{"status":"ok"},"error":null,"message":"ok"}
```

## 5. 构建前端

前端位于 `web/`，使用 Bun：

```bash
cd /opt/starpay/pay-gateway/web
bun install
bun run build
```

构建产物在：

```text
/opt/starpay/pay-gateway/web/dist
```

生产环境通过 Nginx 直接托管该目录。

## 6. 配置 Nginx

推荐同域部署：

- 前端页面：`https://pay.example.com/`
- 后端 API：`https://pay.example.com/v1/...`
- 健康检查：`https://pay.example.com/healthz`
- 支付平台异步通知：`https://pay.example.com/v1/channel/notify`
- 收银台：`https://pay.example.com/checkout/{gateway_order_no}`

创建 Nginx 配置：

```bash
sudo tee /etc/nginx/sites-available/starpay.conf >/dev/null <<'EOF'
server {
    listen 80;
    server_name pay.example.com;

    return 301 https://$host$request_uri;
}

server {
    listen 443 ssl http2;
    server_name pay.example.com;

    ssl_certificate     /etc/letsencrypt/live/pay.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/pay.example.com/privkey.pem;

    root /opt/starpay/pay-gateway/web/dist;
    index index.html;

    client_max_body_size 10m;

    location /healthz {
        proxy_pass http://127.0.0.1:8080/healthz;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    location /v1/ {
        proxy_pass http://127.0.0.1:8080/v1/;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    location / {
        try_files $uri $uri/ /index.html;
    }
}
EOF
```

启用配置：

```bash
sudo ln -sf /etc/nginx/sites-available/starpay.conf /etc/nginx/sites-enabled/starpay.conf
sudo nginx -t
sudo systemctl reload nginx
```

证书可用 Certbot 申请：

```bash
sudo apt install -y certbot python3-certbot-nginx
sudo certbot --nginx -d pay.example.com
```

## 7. 网关后台初始化配置

登录后台后进入“网关配置”，设置：

- 站点名称：例如 `starpay-支付网关`。
- 网关公网地址：`https://pay.example.com`。
- 统一异步通知路径：`/v1/channel/notify`。
- 默认币种：按业务设置，例如 `CNY` 或 `USD`。
- 默认语言：例如 `zh-CN` 或 `en`。

支付通道回调地址统一使用：

```text
https://pay.example.com/v1/channel/notify
```

PayPal、支付宝等支付完成后会先回到网关收银台结果页，再由收银台按订单配置跳转到商户返回地址。

## 8. 发布更新流程

后端更新：

```bash
cd /opt/starpay/pay-gateway
git pull
docker compose --env-file .env.production -f docker-compose.prod.yml up -d --build api
docker compose --env-file .env.production -f docker-compose.prod.yml logs -f api
```

前端更新：

```bash
cd /opt/starpay/pay-gateway
git pull
cd web
bun install
bun run build
sudo systemctl reload nginx
```

如果 Ent schema 有变更，后端启动时会自动执行当前项目内置的迁移逻辑。生产数据库仍建议在更新前做备份。

## 9. 数据备份

PostgreSQL 备份：

```bash
docker exec payment-gateway-postgres pg_dump -U payment payment_gateway > backup-$(date +%F).sql
```

恢复：

```bash
cat backup-2026-07-02.sql | docker exec -i payment-gateway-postgres psql -U payment payment_gateway
```

Redis 主要用于队列、缓存和幂等控制，生产建议保留 `redis_data` volume，并根据业务要求配置额外备份。

如果首次部署时 PostgreSQL 因旧 volume 路径初始化失败，且确认没有生产数据，可以清理 volume 后重启：

```bash
docker compose --env-file .env.production -f docker-compose.prod.yml down -v
docker compose --env-file .env.production -f docker-compose.prod.yml up -d --build
```

如果已经有生产数据，不要执行 `down -v`。应先备份旧库，再按 PostgreSQL 官方升级流程或 dump/restore 迁移。

## 10. 上线检查清单

- `curl https://pay.example.com/healthz` 返回 `200`。
- `curl https://pay.example.com/v1/ping` 返回 `200`。
- 后台可以登录。
- “网关配置”的公网地址是 HTTPS 正式域名。
- 支付通道密钥、证书、沙箱/正式环境配置正确。
- 支付平台异步通知地址是 `https://pay.example.com/v1/channel/notify`。
- 创建测试订单后能打开收银台。
- 支付成功后能进入网关结果页，并跳转到商户返回地址。
- Webhook 中心能看到业务通知投递记录。
- 支付补偿页面能看到 `pending`、`processing`、`resolved` 和 `manual_required` 任务。
- 退款中心能创建退款、查看渠道退款号并重试未完成退款。
- Redis 中存在 `payment:reconciliations` 和 `refund:processing` 消费组。

## 11. 常见问题

### 前端刷新 404

Nginx 必须配置：

```nginx
location / {
    try_files $uri $uri/ /index.html;
}
```

### API 404 或跨域问题

生产推荐同域部署，前端请求 `/v1/...`，Nginx 反代到 `127.0.0.1:8080`。不要让浏览器直接请求容器内网地址。

### 回调收不到

检查：

- 支付平台配置的通知地址是否为公网 HTTPS。
- Nginx 是否把 `/v1/` 正确反代到后端。
- 后端日志：`docker compose --env-file .env.production -f docker-compose.prod.yml logs -f api`。
- 通道配置中的证书、密钥和沙箱/正式环境是否匹配。

### 修改站点名称后 title 没变

后台“网关配置”保存后，新打开页面会读取公开配置接口：

```text
GET /v1/public/site-config
```

确认该接口返回的 `site_name` 已更新。
