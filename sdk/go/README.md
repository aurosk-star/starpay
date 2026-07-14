# StarPay Go SDK

## 安装

第三方 Go 服务直接引入：

```bash
go get github.com/zmoyi/starpay-go
```

如果仓库是私有的，需要先配置 Go 私有模块：

```bash
go env -w GOPRIVATE=github.com/zmoyi/*
```

然后确保当前机器可以通过 SSH 或 HTTPS 访问 GitHub 仓库。

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
case "refund.succeeded":
    // 确认退款完成
case "order.closed":
    // 根据 event.CloseSource 区分管理员或商户主动关闭
case "order.expired":
    // 标记本地订单超时
}

w.WriteHeader(http.StatusOK)
```

`ParseWebhookRequest` 默认拒绝与当前时间相差超过 5 分钟的请求。业务系统还必须使用 `event.EventID` 做事件幂等；`event.DeliveryNo` 可用于排查单次投递，`event.Timestamp` 是签名时间戳。

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
