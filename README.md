# Starpay

[![CI](https://github.com/zmoyi/starpay/actions/workflows/ci.yml/badge.svg)](https://github.com/zmoyi/starpay/actions/workflows/ci.yml)
[![Security](https://github.com/zmoyi/starpay/actions/workflows/security.yml/badge.svg)](https://github.com/zmoyi/starpay/actions/workflows/security.yml)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)

统一支付网关，提供接入应用管理、支付订单、收银台、支付通道适配、通道路由、平台异步通知入口、业务 Webhook 投递和后台管理控制台。

> [!WARNING]
> 项目处于 pre-1.0 阶段，接口和部署方式可能发生不兼容调整。支付系统上线前必须结合自身业务、监管要求和威胁模型完成独立安全评审；请勿把本项目的扫描结果视为生产安全保证。

## 功能概览

- 接入应用：自动生成 App ID，应用密钥只展示一次，支持业务通知地址、默认支付返回地址、IP 白名单、启停和密钥重置。
- 支付订单：支持后台创建/维护订单，也支持开放 API 由业务系统创建、查询和关闭订单。
- 收银台：提供订单详情、可用支付方式列表、发起支付、PayPal 返回处理和支付结果页。
- 支付通道：支持支付宝、微信支付、PayPal 通道账号配置，敏感配置脱敏展示。
- 支付模式：支付宝支持 `page`、`wap`、`qr`；微信支持 `native`、`h5`；PayPal 使用 `checkout`。
- 通道路由：按应用范围、币种、金额、终端、支付方式、支付模式和目标通道账号配置命中规则。
- 回调入口：支付平台异步通知路径由网关固定提供，默认 `/v1/channel/notify`。
- Webhook：支付成功、订单过期等事件会投递到应用配置的业务通知地址，支持查看详情和手动重试。
- 管理后台：React 控制台包含总览、应用、订单、Webhook、支付通道、通道路由、网关配置和用户管理。
- 认证权限：管理员登录、首个超级管理员初始化、JWT + Refresh Cookie、Casbin RBAC。

## 技术栈

- Go 1.26.6 + Gin：HTTP API
- Ent：数据库模型和查询
- PostgreSQL：主数据存储
- Redis：缓存、队列和异步协调
- Casbin：后台 RBAC
- Bun 1.3.13 + React + Rsbuild + TanStack Router：管理后台和收银台前端
- shadcn/ui + Tailwind CSS：前端组件和样式
- Docker Compose：本地依赖和容器化运行

## 项目结构

- `cmd/server/`：后端入口
- `internal/domain/`：业务域模块
  - `apps/`：接入应用和应用密钥
  - `channels/`：支付通道账号
  - `configs/`：网关配置和公开站点配置
  - `monitoring/`：运行概览
  - `orders/`：订单、开放 API、收银台
  - `payments/`：支付服务、通知处理、通道 provider
  - `routing/`：通道路由规则
  - `users/`：管理员、角色、登录认证
  - `webhooks/`：业务 Webhook 投递和重试
- `internal/platform/`：配置、数据库、Redis、HTTP、认证、RBAC 等基础设施
- `ent/schema/`：Ent schema
- `ent/`：Ent 生成代码
- `web/`：React 管理后台和收银台前端

每个业务域按 `handler/`、`router/`、`service/`、`repository/`、`test/` 分层组织。

## 五分钟启动

需要 Go 1.26.6、Bun 1.3.13、Docker 和 Docker Compose v2。

准备环境文件：

```bash
cp .env.example .env
```

启动 PostgreSQL 和 Redis：

```bash
make db-up
```

启动后端：

```bash
make dev
```

健康检查：

```bash
curl http://localhost:8080/healthz
```

启动前端：

```bash
cd web
bun install --frozen-lockfile
bun run dev
```

也可以通过 Makefile：

```bash
make web-dev
```

提交变更前运行完整本地门禁：

```bash
make verify
```

## 常用命令

```bash
make test          # go test ./...
make ent-up        # 重新生成 Ent 代码
make tidy          # go mod tidy
make db-up         # 启动 PostgreSQL 和 Redis
make db-down       # 停止 Docker Compose 服务
make compose-up    # 构建并启动完整容器服务
make compose-logs  # 查看 api 日志
make web-build     # 构建前端
make web-typecheck # TypeScript 类型检查
```

后端代码变更后，应构建最新二进制并重启正在运行的后端，避免旧进程继续提供服务。

## API 入口

所有 API 响应使用统一结构：

```json
{
  "code": "ok",
  "message": "ok",
  "data": {},
  "error": null
}
```

失败响应同样保留该信封，`data` 为 `null`，`error.details` 为结构化对象：

```json
{
  "code": "INVALID_AMOUNT",
  "message": "amount must be a positive integer in minor units",
  "data": null,
  "error": {
    "code": "INVALID_AMOUNT",
    "message": "amount must be a positive integer in minor units",
    "details": {
      "field": "amount"
    }
  }
}
```

开放 API 和收银台支付入口使用稳定大写错误码，例如 `INVALID_SIGNATURE`、`TIMESTAMP_EXPIRED`、`APP_DISABLED`、`INVALID_AMOUNT`、`CURRENCY_NOT_SUPPORTED`、`ORDER_NOT_FOUND`、`ORDER_STATUS_NOT_ALLOWED`、`IDEMPOTENCY_CONFLICT`、`CHANNEL_UNAVAILABLE`、`CHANNEL_RESPONSE_ERROR`。业务系统应优先按 `error.code` 做判断，不依赖 `message`。

开放 API 已启用应用级安全控制：签名鉴权、5 分钟时间窗口、`request_id`/`nonce` Redis 去重、IP 白名单、按 `app_id + method + route` 的限流。业务 Webhook 使用 `X-Pay-Gateway-Timestamp` 和 `X-Pay-Gateway-Signature` 签名，业务方应使用 `app_secret` 验签。

应用密钥由网关生成，只在创建和重置时展示一次。V1 重置密钥会立即使旧密钥失效，业务方应在维护窗口内完成配置更新和 `/v1/open/ping` 验证。

主要入口：

- `GET /healthz`：健康检查
- `GET /v1/ping`：基础连通性检查
- `POST /v1/admin/setup`：初始化超级管理员
- `POST /v1/admin/auth/login`：管理员登录
- `POST /v1/admin/auth/refresh`：刷新访问令牌
- `GET /v1/admin/auth/me`：当前管理员
- `GET /v1/admin/monitoring/overview`：运行概览
- `GET|POST|PUT /v1/admin/apps`：接入应用管理
- `GET|POST|PUT /v1/admin/channels`：支付通道账号管理
- `GET|POST|PUT /v1/admin/routing-rules`：通道路由规则管理
- `GET|PUT /v1/admin/config/gateway`：网关配置
- `GET /v1/public/site-config`：前端公开站点配置
- `GET|POST|PUT /v1/admin/orders`：后台订单管理
- `GET /v1/admin/webhook-deliveries`：Webhook 投递记录
- `POST /v1/open/orders`：业务系统创建订单
- `GET /v1/open/orders/:gateway_order_no`：业务系统查询订单
- `POST /v1/open/orders/:gateway_order_no/close`：业务系统关闭订单
- `GET /v1/checkout/orders/:gateway_order_no`：收银台订单详情
- `GET /v1/checkout/orders/:gateway_order_no/methods`：收银台可用支付方式
- `POST /v1/checkout/orders/:gateway_order_no/pay`：收银台发起支付
- `GET /v1/checkout/paypal/return`：PayPal 返回处理
- `POST /v1/channel/notify`：支付平台统一异步通知入口

开放 API 使用应用签名认证；后台 API 使用管理员访问令牌和 RBAC。

## 支付通道

### 支付宝

- `page`：电脑网站支付，默认 `product_code=FAST_INSTANT_TRADE_PAY`
- `wap`：手机网站支付，默认 `product_code=QUICK_WAP_WAY`
- `qr`：扫码预创建，不传 `product_code`
- 支持配置 `enable_page_pay`、`enable_wap_pay`、`enable_qr_pay`
- 支持沙箱和正式环境

### 微信支付

- `native`：桌面/扫码模式
- `h5`：移动端 H5 模式
- 支持配置 `enable_native_pay`、`enable_h5_pay`

### PayPal

- `checkout`：创建 PayPal Order 并返回 approval URL
- 支持返回 URL 拼接网关支付结果页
- 支持 capture 后更新订单状态

## 通道路由

路由规则用于决定收银台展示和发起支付时应命中的通道账号。匹配条件包括：

- 应用范围：全部应用或指定 App ID
- 支付方式：支付宝、微信支付、PayPal
- 支付模式：如 `page`、`wap`、`qr`、`native`、`h5`、`checkout`
- 币种：为空表示不限
- 金额区间：金额使用最小货币单位
- 终端：任意、桌面端、移动端、微信浏览器
- 目标通道账号：支持优先级、权重和启停

规则按优先级从高到低匹配；未配置规则时，收银台会回退到可用通道账号。

## 网关配置

网关配置包含：

- 站点名称
- 网关公网地址
- 默认币种
- 默认语言
- Request ID 开关
- 维护模式
- 扩展 JSON

支付平台异步通知路径固定为 `/v1/channel/notify`，前端只展示可复制的完整通知 URL，保存配置时不会修改该路径。

## 前端

前端位于 `web/`，包含：

- 管理后台：应用、订单、Webhook、通道、路由、配置、用户
- 收银台：订单展示、支付方式选择、支付发起、结果页
- i18n：翻译文件位于 `web/src/i18n/locales/`

新文本应写入 `zh-CN.json` 和 `en.json`。

## 配置

关键环境变量见 `.env.example`：

- `HTTP_ADDR`
- `DB_DRIVER`
- `DB_HOST` / `DB_PORT` / `DB_NAME` / `DB_USER` / `DB_PASSWORD`
- `DATABASE_URL`
- `REDIS_ADDR`
- `JWT_SECRET`
- `ACCESS_TOKEN_TTL`
- `REFRESH_TOKEN_TTL`
- `APP_SECRET_ENCRYPTION_KEY`
- `ORDER_DEFAULT_TTL`
- `ORDER_EXPIRE_SCAN_INTERVAL`
- `ORDER_EXPIRE_SCAN_LIMIT`
- `ORDER_EXPIRE_WORKER_CONCURRENCY`

生产环境必须替换 `JWT_SECRET` 和 `APP_SECRET_ENCRYPTION_KEY`，并妥善保管支付通道密钥、证书和应用密钥。

## 安全

不要在公开 Issue 或 PR 中提交未修复漏洞、真实凭据或支付数据。请按照 [SECURITY.md](SECURITY.md) 使用 GitHub Private Vulnerability Reporting 私密报告。

生产部署前请至少完成：

- 独立代码、架构和配置安全评审；
- `make verify`、容器构建和镜像漏洞扫描；
- 密钥管理、备份恢复、网络隔离和最小权限检查；
- 支付通道沙箱与故障恢复演练。

## 测试

后端：

```bash
make test
```

前端类型检查：

```bash
make web-typecheck
```

前端构建：

```bash
make web-build
```

重点覆盖范围包括：应用密钥、订单幂等、签名认证、支付通道回调、Webhook 重试、通道路由、货币限制和收银台支付模式选择。

界面变更还应运行 `make web-dev`，人工检查管理后台、收银台以及按 `d` 键进行的深浅色切换。

## 开源协作

- 许可证：[Apache License 2.0](LICENSE)
- 贡献：[CONTRIBUTING.md](CONTRIBUTING.md)
- 行为准则：[CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md)
- 安全政策：[SECURITY.md](SECURITY.md)
- 变更记录：[CHANGELOG.md](CHANGELOG.md)
- Go SDK：独立使用 [MIT License](sdk/go/LICENSE)
