# 支付网关接入文档

## 1. 接入概览

本支付网关面向内部业务应用，提供统一下单、查单、关单、通道支付、回调验签、业务 webhook 和过期订单通知。

流程分两层：

1. 业务应用调用网关创建订单并发起支付。
2. 网关完成通道支付后，通过 webhook 通知业务应用。

## 2. 接入前准备

业务应用需先在网关后台配置：

- `app_id`
- `app_secret`
- `notify_url`
- IP 白名单（可选）

业务侧请求必须携带以下鉴权参数。当前实现从 URL query 和表单参数读取鉴权字段，JSON body 不参与签名。

- `app_id`
- `request_id`
- `timestamp`
- `nonce`
- `sign`

签名算法：

```text
1. 收集 URL query 参数。
2. 如果请求是 form 或 multipart，再合并表单参数。
3. 排除 sign。
4. 按 key 字典序排序；同 key 多值按 value 字典序排序。
5. 拼成 key=value&key=value。
6. 使用 app_secret 做 HMAC-SHA256。
7. 输出十六进制小写字符串作为 sign。
```

示例待签名字符串：

```text
app_id=snsgo&nonce=n_001&request_id=req_20260702_001&timestamp=1782921600
```

伪代码：

```text
params = query + form
delete params["sign"]
signing_string = sort_by_key_and_value(params).join("&")
sign = hex(hmac_sha256(app_secret, signing_string))
```

时间戳支持 Unix 秒或 RFC3339，允许 5 分钟窗口。`request_id` 和 `nonce` 在窗口期内不可重复。

开放 API 默认按 `app_id + HTTP method + route` 限流。默认每个应用每个接口每分钟 120 次，命中限流时返回 `RATE_LIMITED`。

### 2.1 密钥轮换和泄露处理

`app_secret` 只会在应用创建或后台重置密钥时展示一次。业务方应存入自己的密钥管理系统，不应写入代码仓库或前端包。

V1 重置密钥会立即使旧密钥失效。建议流程：

1. 在低峰期由网关管理员重置应用密钥。
2. 业务方更新服务端配置。
3. 业务方调用 `/v1/open/ping` 验证新密钥签名。
4. 验证通过后恢复正常支付请求。

如果怀疑密钥泄露，应先禁用应用或重置密钥，再排查泄露来源。

统一响应格式：

```json
{
  "code": "ok",
  "message": "ok",
  "data": {},
  "error": null
}
```

失败时：

```json
{
  "code": "INVALID_SIGNATURE",
  "message": "invalid signature",
  "data": null,
  "error": {
    "code": "INVALID_SIGNATURE",
    "message": "invalid signature",
    "details": {}
  }
}
```

错误响应约定：

- `code` 与 `error.code` 保持一致。
- 错误码使用稳定的大写蛇形命名，业务逻辑应按 `error.code` 判断，不依赖 `message`。
- `error.details` 始终是对象；没有结构化信息时为 `{}`。
- 支付通道错误只会暴露脱敏字段，例如 `provider_error_code`、`provider_error_message`、`provider_request_id`、`retryable`。

## 3. 订单生命周期

订单状态：

- `pending`
- `paid`
- `failed`
- `closed`

网关会自动为未传 `expires_at` 的订单生成默认过期时间，当前默认 15 分钟。超时未支付的订单会被自动关闭，并投递 `order.expired` webhook。

## 4. 创建支付订单

```text
POST /v1/open/orders?app_id={app_id}&request_id={request_id}&timestamp={timestamp}&nonce={nonce}&sign={sign}
```

请求示例：

```json
{
  "merchant_order_no": "snsgo_membership_123",
  "amount": 9900,
  "currency": "CNY",
  "subject": "Pro 会员",
  "description": "snsgo Pro membership monthly",
  "business_type": "membership",
  "channel": "alipay",
  "pay_method": "alipay",
  "return_url": "https://snsgo.example.com/payment/result",
  "metadata": {
    "user_id": "123",
    "tier": "pro"
  }
}
```

说明：

- `merchant_order_no` 在同一 `app_id` 下必须唯一。
- `amount` 使用最小货币单位。
- 业务方不传 `expires_at`，由网关统一处理。
- `return_url` 可不传；未传时使用应用默认返回地址。

响应中会返回：

- `data.created`
- `data.order.gateway_order_no`
- `data.order.status`
- `data.order.expires_at`
- `data.order.channel`
- `data.order.pay_method`
- `data.payment.status`
- `data.payment.pay_url`：网关收银台地址，业务方应将用户跳转到该地址完成支付。

响应示例：

```json
{
  "code": "ok",
  "message": "ok",
  "data": {
    "created": true,
    "order": {
      "gateway_order_no": "pay_20260702_xxx",
      "merchant_order_no": "snsgo_membership_123",
      "amount": 9900,
      "currency": "CNY",
      "status": "pending"
    },
    "payment": {
      "status": "pending",
      "pay_url": "https://pay.example.com/checkout/pay_20260702_xxx?token=checkout_token"
    }
  },
  "error": null
}
```

## 5. 查询订单

### 按网关单号查询

```text
GET /v1/open/orders/{gateway_order_no}?app_id={app_id}&request_id={request_id}&timestamp={timestamp}&nonce={nonce}&sign={sign}
```

### 按业务单号查询

```text
GET /v1/open/orders/by-merchant/{merchant_order_no}?app_id={app_id}&request_id={request_id}&timestamp={timestamp}&nonce={nonce}&sign={sign}
```

## 6. 关闭订单

```text
POST /v1/open/orders/{gateway_order_no}/close?app_id={app_id}&request_id={request_id}&timestamp={timestamp}&nonce={nonce}&sign={sign}
```

仅 `pending` 和 `failed` 订单可关闭。

## 7. 收银台支付

业务方标准接入不需要直接调用本节接口。业务方只需要把用户跳转到创建订单返回的 `data.payment.pay_url`。

以下接口由网关收银台页面内部使用，必须携带收银台令牌：

```text
X-Checkout-Token: checkout_token
```

### 读取收银台订单

```text
GET /v1/checkout/orders/{gateway_order_no}
```

### 获取支付方式

```text
GET /v1/checkout/orders/{gateway_order_no}/methods
```

### 发起支付

```text
POST /v1/checkout/orders/{gateway_order_no}/pay
```

请求示例：

```json
{
  "channel": "alipay",
  "pay_method": "alipay",
  "client_ip": "127.0.0.1",
  "return_url": "https://snsgo.example.com/payment/result"
}
```

说明：

- 这些接口不是业务方服务端接口。
- 收银台令牌来自创建订单返回的 `payment.pay_url` 查询参数。
- 若订单已锁定渠道，前端不应再显示渠道选择。
- 若订单已过期，支付会被拒绝。
- 若未传 `return_url`，网关按订单或应用默认地址处理。
- `paypal` 会走网关内部的返回跳转逻辑。

## 8. 通道回调

支付通道回调由网关统一接收：

```text
POST /v1/channel/notify
```

支付宝可不传 `channel`，默认按 `alipay` 处理。支付发起时，网关会把实际通道账号写入订单，并在支付宝、微信的回调地址中追加 `channel` 和 `channel_account_id`。

PayPal webhook 地址由 PayPal 控制台配置。每个 PayPal 通道账号必须使用包含自身账号 ID 的回调地址：

```text
POST /v1/channel/notify?channel=paypal&channel_account_id={channel_account_id}
```

同一通道只有一个启用账号时，旧的不带 `channel_account_id` 的地址仍可兼容；存在多个启用账号时，缺少账号 ID 的回调会被拒绝。

网关会：

- 验签
- 幂等更新订单
- 生成业务 webhook

## 9. 业务 Webhook

所有新事件都包含资源幂等字段：

```json
{
  "resource_type": "payment_order",
  "resource_id": "gw_..."
}
```

退款事件使用 `resource_type=refund`、`resource_id=refund_no`，同一支付订单的多次退款不会互相去重。

网关向业务方 `notify_url` 投递事件。

### 9.1 支付成功

事件类型：`payment.succeeded`

示例：

```json
{
  "event_type": "payment.succeeded",
  "app_id": "snsgo",
  "gateway_order_no": "pay_20260702_xxx",
  "merchant_order_no": "snsgo_membership_123",
  "amount": 9900,
  "currency": "CNY",
  "channel": "alipay",
  "channel_trade_no": "202607022200...",
  "paid_at": "2026-07-02T12:03:21Z",
  "metadata": {
    "user_id": "123",
    "tier": "pro"
  }
}
```

### 9.2 支付失败

事件类型：`payment.failed`

```json
{
  "event_type": "payment.failed",
  "app_id": "snsgo",
  "gateway_order_no": "pay_20260702_xxx",
  "merchant_order_no": "snsgo_membership_123",
  "amount": 9900,
  "currency": "CNY",
  "channel": "wechat",
  "pay_method": "wechat",
  "failure_reason": "PAYERROR",
  "failed_at": "2026-07-02T12:03:21Z",
  "metadata": {}
}
```

### 9.3 订单过期

事件类型：`order.expired`

示例：

```json
{
  "event_type": "order.expired",
  "app_id": "snsgo",
  "gateway_order_no": "pay_20260702_xxx",
  "merchant_order_no": "snsgo_membership_123",
  "amount": 9900,
  "currency": "CNY",
  "status": "closed",
  "channel": "alipay",
  "pay_method": "alipay",
  "expires_at": "2026-07-02T12:15:00Z",
  "closed_at": "2026-07-02T12:15:30Z",
  "metadata": {
    "user_id": "123",
    "tier": "pro"
  }
}
```

### 9.4 退款事件

退款成功投递 `refund.succeeded`，退款失败投递 `refund.failed`。关键字段包括 `refund_no`、`merchant_refund_no`、`gateway_order_no`、整数最小单位金额、币种、渠道交易号和渠道退款号。

### 9.5 请求头

```text
X-Pay-Gateway-Event-Id
X-Pay-Gateway-Timestamp
X-Pay-Gateway-Signature
X-Pay-Gateway-Event-Type
X-Pay-Gateway-Delivery-No
```

### 9.6 幂等要求

业务方必须按 `event_id` 做幂等处理。

验签方式：

```text
signature = HMAC-SHA256(app_secret, timestamp + "." + raw_body)
```

业务方应使用 `X-Pay-Gateway-Timestamp` 和原始请求体计算签名，并与 `X-Pay-Gateway-Signature` 做常量时间比较。

业务方成功处理后返回任意 2xx 状态码即可。非 2xx 或网络错误会进入重试。

## 10. 退款 API

```text
POST /v1/open/refunds
GET  /v1/open/refunds/:refund_no
GET  /v1/open/refunds/by-merchant/:merchant_refund_no
```

创建参数示例：

```json
{
  "gateway_order_no": "gw_...",
  "merchant_refund_no": "merchant_refund_001",
  "amount": 9900,
  "currency": "CNY",
  "reason": "duplicate purchase"
}
```

只允许已支付订单退款。`pending` 和 `succeeded` 退款都会占用可退额度；同一 `merchant_refund_no` 参数一致时返回原退款单，参数冲突返回 `IDEMPOTENCY_CONFLICT`。

## 11. 主动查单补偿

支付发起成功后网关创建补偿记录。Worker 在回调缺失时主动查询支付宝、微信或 PayPal，并修正本地 `paid`、`failed`、`closed` 状态。连续八次无法确认后进入 `manual_required`，管理员可在支付补偿页面立即重试。

## 12. 支持的渠道与币种

当前内置校验：

- `alipay`：仅支持 `CNY`
- `wechat`：仅支持 `CNY`
- `paypal`：支持 `USD`、`EUR`、`HKD`、`JPY`、`GBP`

如果订单创建时指定 `channel` 或 `pay_method`，网关会校验币种是否匹配。不指定时，收银台只展示可用且匹配币种的渠道。

## 13. 推荐对接流程

1. 业务方创建订单。
2. 业务方调用收银台发起支付。
3. 用户完成支付。
4. 通道回调网关。
5. 网关更新订单为 `paid`。
6. 网关投递 `payment.succeeded` webhook。
7. 业务方确认后发放权益。

## 14. 过期订单

订单未支付且超过有效期后：

- 网关自动关闭订单
- 网关投递 `order.expired` webhook

业务方可用它做：

- 超时关单
- 前端状态刷新
- 资源释放

## 15. 常见错误

| 错误码 | HTTP | 场景 | 建议处理 |
| --- | --- | --- | --- |
| `INVALID_SIGNATURE` | 401 | 签名错误或缺少签名参数 | 重新按签名规则生成请求 |
| `TIMESTAMP_EXPIRED` | 401 | 请求时间戳超过允许窗口 | 使用当前时间重新签名 |
| `REPLAYED_REQUEST` | 401 | `request_id` 或 `nonce` 重复 | 生成新的幂等请求参数 |
| `APP_NOT_FOUND` | 401 | 应用不存在或凭据无效 | 检查 `app_id` 和密钥 |
| `APP_DISABLED` | 403 | 应用被禁用 | 联系网关管理员启用应用 |
| `IP_NOT_ALLOWED` | 403 | 请求来源 IP 不在白名单 | 调整应用 IP 白名单 |
| `INVALID_REQUEST` | 400 | 请求体格式错误或 JSON 无法解析 | 修正请求体 |
| `MISSING_REQUIRED_FIELD` | 400 | 缺少必填字段 | 查看 `error.details.field` |
| `INVALID_AMOUNT` | 400 | 金额非法 | 金额必须为最小货币单位的正整数 |
| `INVALID_CURRENCY` | 400 | 币种格式非法 | 使用 ISO 币种代码，例如 `CNY` |
| `CURRENCY_NOT_SUPPORTED` | 400 | 当前通道不支持该币种 | 更换支付通道或币种 |
| `ORDER_NOT_FOUND` | 404 | 支付订单不存在 | 检查 `gateway_order_no` |
| `ORDER_EXPIRED` | 409 | 订单已过期 | 创建新订单 |
| `ORDER_STATUS_NOT_ALLOWED` | 409 | 当前订单状态不允许操作 | 查询订单状态后再处理 |
| `IDEMPOTENCY_CONFLICT` | 409 | 同一业务单号重复请求参数不一致 | 保持同一 `merchant_order_no` 参数一致 |
| `CHANNEL_UNAVAILABLE` | 503 | 没有可用支付通道或模式 | 稍后重试或切换通道 |
| `CHANNEL_CONFIG_INVALID` | 500 | 通道配置缺失或非法 | 检查后台通道账号配置 |
| `CHANNEL_RESPONSE_ERROR` | 502 | 支付通道返回失败 | 查看 `error.details`，按 `retryable` 判断 |
| `CHANNEL_TIMEOUT` | 504 | 调用支付通道超时 | 可按幂等逻辑重试 |
| `RATE_LIMITED` | 429 | 请求频率超限 | 降低请求频率后重试 |
| `INTERNAL_ERROR` | 500 | 网关内部错误 | 记录请求信息并联系网关管理员 |

命中开放 API 限流时返回：

```json
{
  "code": "RATE_LIMITED",
  "message": "rate limit exceeded",
  "data": null,
  "error": {
    "code": "RATE_LIMITED",
    "message": "rate limit exceeded",
    "details": {}
  }
}
```

## 16. 接入验收清单

- 能成功创建订单。
- 重复创建相同业务单号时返回原订单。
- 重复创建相同业务单号但金额不同会失败。
- 能跳转或打开通道支付页面。
- 支付成功后能收到 `payment.succeeded`。
- 超时未支付后能收到 `order.expired`。
- webhook 能按事件 ID 幂等。
- webhook 签名校验失败时拒绝处理。

## 17. 测试建议

推荐先跑最小链路：

1. 创建应用并配置 `notify_url`
2. 创建一笔 `CNY` 订单
3. 在收银台发起支付宝支付
4. 验证通道回调
5. 验证业务 webhook

测试支付页面可用于联调，生产环境请改为真实业务返回页。

## 18. Go SDK

业务服务可以直接引入 Go SDK：

```bash
go get github.com/zmoyi/starpay-go
```

私有仓库环境需要配置：

```bash
go env -w GOPRIVATE=github.com/zmoyi/*
```

SDK 文档见：

```text
sdk/go/README.md
```
