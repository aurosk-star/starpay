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
  "code": "invalid_signature",
  "message": "invalid signature",
  "data": null,
  "error": {
    "code": "invalid_signature",
    "message": "invalid signature"
  }
}
```

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

- 若订单已锁定渠道，前端不应再显示渠道选择。
- 若订单已过期，支付会被拒绝。
- 若未传 `return_url`，网关按订单或应用默认地址处理。
- `paypal` 会走网关内部的返回跳转逻辑。

## 8. 通道回调

支付通道回调由网关统一接收：

```text
POST /v1/channel/notify
```

支付宝可不传 `channel`，默认按 `alipay` 处理。PayPal 等通道需要通过 query 或 form 传 `channel`：

```text
POST /v1/channel/notify?channel=paypal
```

网关会：

- 验签
- 幂等更新订单
- 生成业务 webhook

## 9. 业务 Webhook

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

### 9.2 订单过期

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

### 9.3 请求头

```text
X-Pay-Gateway-Event-Id
X-Pay-Gateway-Timestamp
X-Pay-Gateway-Signature
X-Pay-Gateway-Event-Type
X-Pay-Gateway-Delivery-No
```

### 9.4 幂等要求

业务方必须按 `event_id` 或 `gateway_order_no` 做幂等处理。

验签方式：

```text
signature = HMAC-SHA256(app_secret, timestamp + "." + raw_body)
```

业务方应使用 `X-Pay-Gateway-Timestamp` 和原始请求体计算签名，并与 `X-Pay-Gateway-Signature` 做常量时间比较。

业务方成功处理后返回任意 2xx 状态码即可。非 2xx 或网络错误会进入重试。

## 10. 支持的渠道与币种

当前内置校验：

- `alipay`：仅支持 `CNY`
- `wechat`：仅支持 `CNY`
- `paypal`：支持 `USD`、`EUR`、`HKD`、`JPY`、`GBP`

如果订单创建时指定 `channel` 或 `pay_method`，网关会校验币种是否匹配。不指定时，收银台只展示可用且匹配币种的渠道。

## 11. 推荐对接流程

1. 业务方创建订单。
2. 业务方调用收银台发起支付。
3. 用户完成支付。
4. 通道回调网关。
5. 网关更新订单为 `paid`。
6. 网关投递 `payment.succeeded` webhook。
7. 业务方确认后发放权益。

## 12. 过期订单

订单未支付且超过有效期后：

- 网关自动关闭订单
- 网关投递 `order.expired` webhook

业务方可用它做：

- 超时关单
- 前端状态刷新
- 资源释放

## 13. 常见错误

- 签名缺少 `request_id`
- `merchant_order_no` 重复但参数不一致
- 金额单位传成元而不是分
- 支付方式和币种不匹配
- 订单已过期仍继续支付

## 14. 接入验收清单

- 能成功创建订单。
- 重复创建相同业务单号时返回原订单。
- 重复创建相同业务单号但金额不同会失败。
- 能跳转或打开通道支付页面。
- 支付成功后能收到 `payment.succeeded`。
- 超时未支付后能收到 `order.expired`。
- webhook 能按事件 ID 幂等。
- webhook 签名校验失败时拒绝处理。

## 15. 测试建议

推荐先跑最小链路：

1. 创建应用并配置 `notify_url`
2. 创建一笔 `CNY` 订单
3. 在收银台发起支付宝支付
4. 验证通道回调
5. 验证业务 webhook

测试支付页面可用于联调，生产环境请改为真实业务返回页。

## 16. Go SDK

业务服务可以直接引入 Go SDK：

```bash
go get codeup.aliyun.com/h-star/pay-gateway.git/sdk/go
```

私有仓库环境需要配置：

```bash
go env -w GOPRIVATE=codeup.aliyun.com/h-star/*
```

SDK 文档见：

```text
sdk/go/README.md
```
