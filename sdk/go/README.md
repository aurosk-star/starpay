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
case "order.expired":
    // 标记本地订单超时
}

w.WriteHeader(http.StatusOK)
```
