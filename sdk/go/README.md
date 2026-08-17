# StarPay Go SDK

## 安装

第三方 Go 服务直接引入：

```bash
go get github.com/zmoyi/starpay-go
```

## 创建客户端

```go
client, err := paygateway.NewClient(paygateway.Config{
    BaseURL:   "https://pay.example.com",
    AppID:     "snsgo",
    AppSecret: "your_app_secret",
})
```

## 创建订单

```go
result, err := client.CreateOrder(ctx, paygateway.CreateOrderRequest{
    MerchantOrderNo: "snsgo_order_123",
    Amount:          9900,
    Currency:        "CNY",
    Subject:         "Pro 会员",
    Channel:         "alipay",
    PayMethod:       "alipay",
    ReturnURL:       "https://snsgo.example.com/payment/result",
    Metadata: map[string]any{
        "user_id": "123",
    },
})
```

创建成功后，将用户跳转到网关收银台：

```go
http.Redirect(w, r, result.Payment.PayURL, http.StatusFound)
```

业务方不需要直接发起支付。真实支付宝、PayPal、微信等通道支付由网关收银台内部完成。

## 查询和关闭订单

```go
order, err := client.GetOrder(ctx, "pay_20260713_001")

order, err = client.GetOrderByMerchant(ctx, "snsgo_order_123")

order, err = client.CloseOrder(ctx, "pay_20260713_001")
```

## 创建和查询退款

```go
result, err := client.CreateRefund(ctx, paygateway.CreateRefundRequest{
    GatewayOrderNo:   "pay_20260713_001",
    MerchantRefundNo: "snsgo_refund_123",
    Amount:           9900,
    Currency:         "CNY",
    Reason:           "duplicate purchase",
})

refund, err := client.GetRefund(ctx, result.Refund.RefundNo)
refund, err = client.GetRefundByMerchant(ctx, "snsgo_refund_123")
```

相同 `merchant_refund_no` 和相同参数会返回原退款单；参数不一致时返回 `IDEMPOTENCY_CONFLICT`。

## 错误处理

SDK 会把网关标准错误响应解析为 `*paygateway.APIError`，其中包含 HTTP 状态、错误码和结构化详情：

```go
result, err := client.CreateOrder(ctx, request)
if err != nil {
    var apiErr *paygateway.APIError
    if errors.As(err, &apiErr) {
        if apiErr.Code == paygateway.CodeIdempotencyConflict {
            // 请求与已存在订单冲突
        }
        if retryable, ok := apiErr.Details["retryable"].(bool); ok && retryable {
            // 按业务策略重试
        }
    }
    return err
}
```

当网关或中间代理返回非标准 JSON 时，错误码为 `paygateway.CodeInvalidResponse`，原始响应摘要保存在 `ResponseBody`。

## Webhook 验签

```go
event, err := paygateway.ParseWebhookRequest(r, "your_app_secret")
if err != nil {
    http.Error(w, "invalid signature", http.StatusUnauthorized)
    return
}

switch event.EventType {
case "payment.succeeded":
    // 发放权益
case "payment.failed":
    // 标记支付失败
case "refund.succeeded":
    // 确认退款完成
case "refund.failed":
    // 记录退款失败
case "order.closed":
    // 根据 event.CloseSource 区分管理员或商户主动关闭
case "order.expired":
    // 标记本地订单超时
}

w.WriteHeader(http.StatusOK)
```

`ParseWebhookRequest` 默认拒绝与当前时间相差超过 5 分钟的请求。业务系统还必须使用 `event.EventID` 做事件幂等；`event.DeliveryNo` 可用于排查单次投递，`event.Timestamp` 是签名时间戳。

### Webhook 事件说明

网关当前支持以下业务事件：

| 事件类型 | 触发时机 |
| --- | --- |
| `payment.succeeded` | 通道确认支付成功，订单变为 `paid` |
| `payment.failed` | 通道确认支付失败 |
| `refund.succeeded` | 已支付订单退款成功 |
| `refund.failed` | 已支付订单退款失败 |
| `order.expired` | 待支付订单超时关闭 |
| `order.closed` | 管理员或商户主动关闭待支付/失败订单 |

支付成功事件的 JSON body 示例：

```json
{
  "event_type": "payment.succeeded",
  "app_id": "app_E68DH89pKtPCyyznPcY7MA",
  "gateway_order_no": "pay_20260818_xxxxx",
  "merchant_order_no": "aurox_12345",
  "amount": 9900,
  "currency": "CNY",
  "channel": "alipay",
  "channel_trade_no": "channel_xxxxx",
  "paid_at": "2026-08-18T12:00:00Z",
  "metadata": {},
  "resource_type": "payment_order",
  "resource_id": "pay_20260818_xxxxx"
}
```

`payment.succeeded` 当前不会包含 `status: "paid"`；`status` 主要用于 `order.expired` 和 `order.closed` 等订单状态事件。金额使用货币最小单位，例如 `9900` 表示 `99.00 CNY`。

事件资源字段遵循以下规则：支付订单事件使用 `resource_type=payment_order`，`resource_id` 为网关订单号；退款事件使用 `resource_type=refund`，`resource_id` 为 `refund_no`。

事件 ID 不要求出现在 JSON body 中，网关通过请求头传递：

```text
X-Pay-Gateway-Event-Id
X-Pay-Gateway-Timestamp
X-Pay-Gateway-Signature
X-Pay-Gateway-Event-Type
X-Pay-Gateway-Delivery-No
```

签名为 `HMAC-SHA256(app_secret, timestamp + "." + raw_body)`。业务方应先验签，再按 `event.EventID` 幂等处理；同一事件的自动重试不会生成新的事件 ID。网关收到 HTTP `2xx` 才认为投递成功，失败时最多自动尝试 3 次。

需要自定义时间窗口或测试时钟时，可以使用：

```go
event, err := paygateway.ParseWebhookRequestWithOptions(
    r,
    "your_app_secret",
    paygateway.WebhookVerificationOptions{
        Tolerance: 10 * time.Minute,
    },
)
```
