# 自研支付网关产品文档

## 1. 背景

当前项目已有会员支付能力，但支付逻辑仍偏业务内置：订单、通道、回调和会员权益发放耦合在同一个应用内。随着后续多个应用都需要支付能力，需要建设一个独立的支付网关服务，为内部应用提供统一、安全、可扩展的支付接入能力。

本支付网关不定位为资金清结算平台，不做商户余额、提现、资金池或代收代付。第一阶段服务内部应用，使用平台统一商户号收款，通过标准 API、签名鉴权和 webhook 与业务系统解耦。

## 2. 产品定位

自研支付网关是一个独立服务，面向多个内部业务应用提供统一支付能力。

核心定位：

- 多应用统一接入支付。
- 平台统一支付宝、微信、Stripe 等通道账号收款。
- 提供统一下单、查单、退款、回调、webhook、对账能力。
- 支持通道路由、多币种和订阅扣款。
- 支付网关只负责支付状态，不直接发放业务权益。

整体结构：

```text
业务应用
  ├─ snsgo
  ├─ app B
  └─ app C
      │
      │ app_id + signature
      ▼
自研支付网关
  ├─ 应用鉴权
  ├─ 统一订单
  ├─ 通道路由
  ├─ 多币种
  ├─ 一次性支付
  ├─ 订阅扣款
  ├─ 回调验签
  ├─ webhook 投递
  ├─ 退款
  └─ 对账补偿
      │
      ▼
支付通道
  ├─ 支付宝
  ├─ 微信支付
  ├─ Stripe
  └─ 后续其他通道
```

## 3. 目标

### 3.1 第一阶段目标

- 支持多个内部应用接入。
- 支持平台统一商户号收款。
- 支持应用级 `app_id`、`app_secret` 签名鉴权。
- 支持统一创建支付订单。
- 支持支付订单查询。
- 支持支付宝、微信支付、Stripe 等通道适配。
- 支持按币种、支付方式、应用、优先级进行通道路由。
- 支持 CNY、USD 等多币种订单。
- 支持支付通道异步回调验签和幂等处理。
- 支持支付成功后向业务应用投递 webhook。
- 支持 webhook 失败重试和手动补发。
- 支持退款创建和退款查询。
- 支持基础订单后台和通道配置后台。
- 支持基础查单补偿和异常订单标记。

### 3.2 第二阶段目标

- 支持订阅计划、订阅实例、账单和周期扣款。
- 支持 Stripe 自动订阅扣款。
- 支持支付宝、微信的手动续费型订阅。
- 支持订阅生命周期 webhook。
- 支持订阅后台管理。
- 支持更细的路由规则和通道健康监控。

### 3.3 暂不做

- 外部商户入驻。
- 商户余额。
- 商户提现。
- 资金池。
- 自动分账。
- 清结算系统。
- 个人收款码。
- 跑分或码商模式。
- 复杂风控评分系统。
- 自动换汇报价。

## 4. 用户与使用方

### 4.1 业务应用

业务应用是支付网关的主要使用方，例如：

- snsgo
- 其他内部应用
- 未来内部管理系统或 SaaS 产品

业务应用负责：

- 创建自己的业务订单。
- 调用支付网关创建支付订单。
- 接收支付网关 webhook。
- 根据自己的业务订单发放权益或服务。
- 主动查询支付订单，处理 webhook 丢失或延迟。

### 4.2 平台管理员

平台管理员负责：

- 创建和管理接入应用。
- 配置通道账号。
- 配置路由规则。
- 查看支付订单。
- 查看 webhook 投递记录。
- 手动查单。
- 手动补发 webhook。
- 处理异常订单和退款。

## 5. 核心原则

### 5.1 支付网关不发放业务权益

支付网关只产生标准支付事件，不直接修改业务应用数据。

例如：

```text
支付网关发出 payment.succeeded
  ↓
snsgo 收到 webhook
  ↓
snsgo 查询本地会员订单
  ↓
snsgo 确认金额、用户、套餐
  ↓
snsgo 发放会员权益
```

### 5.2 业务应用不能只信任 metadata

`metadata` 可以用于回传上下文，但业务应用必须通过 `merchant_order_no` 查询自己的业务订单，再决定是否发放权益。

### 5.3 订单金额使用最小货币单位

金额统一使用整数，禁止使用浮点数。

示例：

```text
CNY 9900 = 99.00 元
USD 1299 = 12.99 美元
JPY 1200 = 1200 日元
```

### 5.4 `app_id + merchant_order_no` 全局唯一

同一个应用内，业务订单号不能重复。

重复请求规则：

- 参数一致：返回原支付订单。
- 参数不一致：拒绝请求。

### 5.5 最终状态以服务端为准

客户端轮询只用于体验，最终支付状态以通道回调、主动查单和网关订单状态为准。

## 6. 核心业务流程

### 6.1 一次性支付流程

```text
业务应用创建业务订单
  ↓
业务应用调用支付网关统一下单
  ↓
支付网关鉴权和参数校验
  ↓
支付网关执行通道路由
  ↓
支付网关创建支付订单
  ↓
支付网关调用支付通道
  ↓
支付网关返回支付链接、二维码或支付参数
  ↓
用户完成支付
  ↓
支付通道异步通知支付网关
  ↓
支付网关验签、幂等更新订单
  ↓
支付网关投递 webhook 给业务应用
  ↓
业务应用发放权益或服务
```

### 6.2 Webhook 投递流程

```text
支付事件产生
  ↓
创建 webhook event
  ↓
创建 webhook delivery
  ↓
POST 到业务应用 notify_url
  ↓
业务应用返回 2xx
  ↓
标记投递成功
```

失败时进入重试队列。

建议重试策略：

```text
10s -> 30s -> 2m -> 10m -> 30m -> 2h
```

### 6.3 主动查单补偿

```text
定时扫描 pending 订单
  ↓
调用对应通道查单
  ↓
通道已支付，本地未支付：修正为 paid，发送 webhook
  ↓
通道已关闭，本地 pending：修正为 closed
  ↓
超时未支付：标记 expired 或 closed
```

### 6.4 退款流程

```text
业务应用申请退款
  ↓
支付网关校验订单状态和可退金额
  ↓
创建退款单
  ↓
调用支付通道退款接口
  ↓
更新退款状态
  ↓
投递 refund.succeeded 或 refund.failed webhook
```

## 7. 通道路由

通道路由用于在多个支付通道之间选择最合适的通道。

### 7.1 路由输入

- `app_id`
- `amount`
- `currency`
- `country`
- `pay_method`
- `business_type`
- `client_platform`
- `preferred_channel`

### 7.2 路由输出

- `selected_channel`
- `selected_channel_account`
- `route_rule_id`
- `route_reason`

### 7.3 第一阶段路由能力

- 按币种路由。
- 按支付方式路由。
- 按应用路由。
- 按业务类型路由。
- 通道启停。
- 通道优先级。
- 指定通道校验。

示例：

```text
currency = CNY, pay_method = alipay -> alipay
currency = CNY, pay_method = wechat -> wechat
currency = USD, pay_method = card -> stripe
business_type = subscription, currency = USD -> stripe
```

### 7.4 暂不做的路由能力

- 成功率动态路由。
- 成本最低路由。
- 风控评分路由。
- A/B 流量分配。

## 8. 多币种

### 8.1 第一批支持币种

建议第一批支持：

- CNY
- USD

后续可扩展：

- EUR
- HKD
- JPY
- GBP

### 8.2 多币种原则

- 业务应用传入明确的 `amount` 和 `currency`。
- 网关不在第一阶段做自动换汇。
- 通道必须支持对应币种，否则路由失败。
- 订单币种和通道结算币种需要分别记录。

### 8.3 字段建议

```text
amount              订单金额，最小货币单位
currency            订单币种
settlement_amount   结算金额，可为空
settlement_currency 结算币种，可为空
exchange_rate       汇率，可为空
```

## 9. 订阅扣款

订阅扣款需要独立建模，不能简单视为普通订单字段。

### 9.1 订阅类型

第一阶段支持两类订阅：

```text
自动续费订阅
- 适合 Stripe card
- 到期自动扣款

手动续费订阅
- 适合支付宝、微信
- 到期生成续费订单，用户主动支付
```

### 9.2 核心对象

```text
Plan
- 订阅计划
- 定义金额、币种、周期

Subscription
- 用户订阅实例
- 记录订阅状态和当前周期

Invoice
- 周期账单
- 每个账期生成一张账单

PaymentOrder
- 账单对应的支付订单
```

### 9.3 订阅状态

```text
incomplete
  ↓ 首次支付成功
active
  ├─ past_due
  ├─ canceled
  ├─ paused
  └─ expired
```

### 9.4 订阅事件

- `subscription.created`
- `subscription.activated`
- `subscription.renewed`
- `subscription.past_due`
- `subscription.canceled`
- `invoice.created`
- `invoice.payment_succeeded`
- `invoice.payment_failed`

## 10. 核心数据模型

### 10.1 App

业务应用。

字段：

```text
id
app_id
name
app_secret_hash
notify_url
allowed_ips
status
created_at
updated_at
```

### 10.2 ChannelAccount

平台通道账号。

第一阶段使用平台统一商户号，因此通道账号归属于平台，不归属于具体应用。

字段：

```text
id
channel
name
enabled
env
config
created_at
updated_at
```

### 10.3 RouteRule

通道路由规则。

字段：

```text
id
app_id nullable
currency nullable
country nullable
pay_method nullable
business_type nullable
channel
priority
enabled
created_at
updated_at
```

### 10.4 PaymentOrder

支付订单。

字段：

```text
id
gateway_order_no
app_id
merchant_order_no
business_type
subject
description
amount
currency
settlement_amount nullable
settlement_currency nullable
channel
pay_method
channel_trade_no nullable
status
expires_at
paid_at nullable
closed_at nullable
metadata
created_at
updated_at
```

### 10.5 RefundOrder

退款单。

字段：

```text
id
refund_no
gateway_order_no
app_id
amount
currency
reason
channel_refund_no nullable
status
succeeded_at nullable
failed_reason nullable
created_at
updated_at
```

### 10.6 WebhookEvent

业务事件。

字段：

```text
id
event_id
app_id
event_type
payload
created_at
```

### 10.7 WebhookDelivery

Webhook 投递记录。

字段：

```text
id
event_id
app_id
notify_url
status
retry_count
next_retry_at
last_error
response_status
response_body
created_at
updated_at
```

### 10.8 Plan

订阅计划。

字段：

```text
id
app_id
plan_code
name
amount
currency
interval
interval_count
trial_days
status
metadata
created_at
updated_at
```

### 10.9 Subscription

订阅实例。

字段：

```text
id
subscription_no
app_id
plan_id
user_ref
status
channel
channel_subscription_id nullable
current_period_start
current_period_end
cancel_at_period_end
canceled_at nullable
metadata
created_at
updated_at
```

### 10.10 Invoice

订阅账单。

字段：

```text
id
invoice_no
subscription_no
app_id
amount
currency
period_start
period_end
status
payment_order_no nullable
due_at
paid_at nullable
created_at
updated_at
```

## 11. API 草案

### 11.1 鉴权 Header

业务应用调用网关 API 时必须携带：

```text
X-App-Id
X-Timestamp
X-Nonce
X-Signature
```

签名内容：

```text
method + path + timestamp + nonce + body
```

### 11.2 创建支付订单

```text
POST /v1/payment/orders
```

请求：

```json
{
  "merchant_order_no": "snsgo_membership_123",
  "amount": 9900,
  "currency": "CNY",
  "pay_method": "alipay",
  "preferred_channel": "alipay",
  "subject": "Pro 会员",
  "description": "snsgo Pro membership monthly",
  "business_type": "membership",
  "client_ip": "127.0.0.1",
  "return_url": "https://snsgo.example.com/payment/result",
  "metadata": {
    "user_id": "123",
    "tier": "pro"
  }
}
```

响应：

```json
{
  "gateway_order_no": "pay_20260630_xxx",
  "merchant_order_no": "snsgo_membership_123",
  "status": "pending",
  "channel": "alipay",
  "pay_url": "https://...",
  "qr_code": "https://...",
  "expires_at": "2026-06-30T12:10:00Z"
}
```

### 11.3 查询支付订单

```text
GET /v1/payment/orders/{gateway_order_no}
GET /v1/payment/orders/by-merchant/{merchant_order_no}
```

### 11.4 关闭支付订单

```text
POST /v1/payment/orders/{gateway_order_no}/close
```

### 11.5 创建退款

```text
POST /v1/payment/refunds
```

### 11.6 查询退款

```text
GET /v1/payment/refunds/{refund_no}
```

### 11.7 订阅接口

```text
POST /v1/plans
GET  /v1/plans
POST /v1/subscriptions
GET  /v1/subscriptions/{subscription_no}
POST /v1/subscriptions/{subscription_no}/cancel
POST /v1/subscriptions/{subscription_no}/resume
GET  /v1/invoices
```

### 11.8 通道回调接口

```text
POST /v1/channel/alipay/notify
POST /v1/channel/wechat/notify
POST /v1/channel/stripe/webhook
```

## 12. Webhook 草案

### 12.1 支付成功事件

```json
{
  "event_id": "evt_20260630_xxx",
  "event_type": "payment.succeeded",
  "app_id": "snsgo",
  "gateway_order_no": "pay_20260630_xxx",
  "merchant_order_no": "snsgo_membership_123",
  "amount": 9900,
  "currency": "CNY",
  "channel": "alipay",
  "channel_trade_no": "202606302200...",
  "paid_at": "2026-06-30T12:03:21Z",
  "metadata": {
    "user_id": "123",
    "tier": "pro"
  }
}
```

### 12.2 退款成功事件

```json
{
  "event_id": "evt_20260630_yyy",
  "event_type": "refund.succeeded",
  "app_id": "snsgo",
  "gateway_order_no": "pay_20260630_xxx",
  "refund_no": "ref_20260630_xxx",
  "merchant_order_no": "snsgo_membership_123",
  "amount": 9900,
  "currency": "CNY",
  "refunded_at": "2026-06-30T13:03:21Z"
}
```

## 13. 后台管理

### 13.1 应用管理

- 创建应用。
- 查看应用。
- 启用/禁用应用。
- 配置 notify_url。
- 配置 IP 白名单。
- 重置 app_secret。

### 13.2 通道配置

- 支付宝配置。
- 微信支付配置。
- Stripe 配置。
- 沙箱/生产环境切换。
- 启用/禁用通道。
- 测试通道连通性。

### 13.3 路由规则

- 创建路由规则。
- 调整优先级。
- 启用/禁用规则。
- 查看命中记录。

### 13.4 订单中心

- 按应用、通道、币种、状态、时间查询订单。
- 查看通道流水号。
- 手动查单。
- 手动关闭订单。
- 查看订单 webhook 记录。

### 13.5 Webhook 中心

- 查看投递状态。
- 查看失败原因。
- 手动重试。
- 查看响应状态码和响应内容。

### 13.6 退款中心

- 查看退款单。
- 发起退款。
- 查询退款状态。
- 查看退款 webhook 记录。

### 13.7 订阅中心

- 查看订阅计划。
- 查看订阅实例。
- 查看账单。
- 取消订阅。
- 手动生成续费订单。

## 14. 风险与约束

### 14.1 合规边界

第一阶段仅服务内部应用，使用平台统一商户号收款。支付网关不提供外部商户入驻、余额、提现、代收代付、资金池、二清等能力。

### 14.2 多币种复杂度

第一阶段不做自动换汇。若未来支持自动换汇，需要设计汇率来源、报价锁定、退款汇差、结算币种等能力。

### 14.3 订阅复杂度

支付宝、微信不适合作为第一阶段自动无感扣款能力。第一阶段建议：

- Stripe 支持自动续费。
- 支付宝、微信支持手动续费。

### 14.4 Webhook 可靠性

业务应用必须支持 webhook 幂等。支付网关需要支持 webhook 重试和主动查单。

## 15. 版本规划

### V1.0 支付网关基础版

- 独立服务。
- 多应用接入。
- 平台统一商户号。
- 应用签名鉴权。
- 一次性支付。
- 支付宝、微信、Stripe 基础适配。
- CNY、USD 多币种字段与校验。
- 规则化通道路由。
- 支付回调验签。
- Webhook 投递和重试。
- 退款。
- 基础后台。
- 基础查单补偿。

### V1.5 订阅版

- Plan。
- Subscription。
- Invoice。
- Stripe 自动续费。
- 支付宝、微信手动续费。
- 订阅 webhook。
- 订阅后台。

### V2.0 增强版

- 更多币种。
- 通道健康监控。
- 通道成功率统计。
- 更细路由策略。
- 对账中心。
- 异常订单处理台。
- 操作审计。

## 16. 待确认问题

- 第一批业务应用有哪些？
- 第一批支付通道是否确定为支付宝、微信、Stripe？
- 第一批币种是否只支持 CNY 和 USD？
- Stripe 是否作为订阅自动扣款的首个通道？
- 支付宝、微信订阅是否接受手动续费模型？
- 是否需要独立部署数据库，还是复用现有基础设施？
- 管理后台是复用现有 web-console，还是独立后台？
- 是否需要提供 Go/TypeScript SDK？
- 是否需要从第一版开始支持 IP 白名单？
- 是否需要对接消息队列，还是先用数据库任务表处理 webhook 重试？
