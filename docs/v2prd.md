# 支付网关 V2 缺口补齐 PRD

## 1. 背景

当前支付网关已经完成 V1 核心支付链路：多应用接入、签名开放 API、统一下单、收银台支付、支付宝/微信/PayPal 通道、通道路由、支付回调、订单过期、商户 webhook、后台管理台和 Go SDK。

V2 的目标不是重新设计支付网关，而是在现有 V1 基础上补齐生产化缺口，让系统从“可联调、可试运行”推进到“主支付闭环完整、异常可恢复、退款可管理、事件可追踪”的阶段。

## 2. 当前基线

### 2.1 已具备能力

- 应用管理：`app_id`、密钥、通知地址、默认回跳地址、IP 白名单、启停、密钥重置。
- 开放 API：应用签名鉴权、时间戳窗口、nonce 防重放、Redis replay store、Open API 限流。
- 订单：创建、查询、按商户订单号查询、关闭、幂等创建、checkout token、过期关闭。
- 支付：支付宝、微信、PayPal provider；支付启动；统一支付回调；PayPal capture/return。
- 路由：按应用、支付方式、通道、币种、金额区间、终端、优先级和目标通道做基础路由。
- Webhook：`payment.succeeded`、`order.expired` 事件，投递记录，HMAC 签名，自动重试，手动重试。
- 后台：应用、用户、通道、路由、订单、webhook、配置、监控、测试支付、公共收银台。
- 运行时：订单过期 worker、webhook worker、webhook retry scanner、基础监控概览。
- SDK：Go SDK 支持签名下单、查单、关单和 webhook 验签。

### 2.2 明确缺口

- 没有支付失败状态闭环和 `payment.failed` 事件。
- 没有主动查单补偿，回调丢失时 pending 订单无法自动修复。
- 没有退款域：无退款表、退款 API、退款后台、退款事件和退款状态流转。
- 没有订阅域：只有目录占位，没有计划、订阅、账单、续费和订阅事件。
- 没有 Stripe provider，当前 PRD 中的 Stripe 能力未实现。
- 路由还不是完整智能路由：无动态健康、成功率、成本、A/B、国家或业务类型路由。
- 没有财务账务模型：无 transaction、ledger、settlement 或对账明细。
- RBAC 只有基础保护和角色列表，没有完整角色/权限后台管理。
- 前端没有单元测试或 E2E 测试体系。
- SDK 测试是独立 Go module，根目录 `make test` 不覆盖。

## 3. V2 目标

### 3.1 必须完成

- 补齐支付失败状态流转和 `payment.failed` webhook。
- 补齐主动查单补偿，自动修复回调丢失、回调延迟、用户完成支付但本地仍 pending 的订单。
- 补齐退款核心能力：退款创建、查询、状态流转、后台管理和退款 webhook。
- 扩展 webhook 事件模型，支持订单、支付、退款等多资源类型幂等事件。
- 提升后台异常处理能力：待补偿订单、失败 webhook、退款失败记录可见可处理。
- 补齐测试和验证命令，让 backend、frontend、SDK 的关键检查一键覆盖。

### 3.2 应该完成

- 增强路由规则，补上业务类型、国家/地区、权重选择、通道健康禁用。
- 增强监控页面，展示 pending 积压、补偿结果、通道失败率、webhook 失败率。
- 完善 RBAC 管理，支持角色、权限和策略维护。
- 统一文档中的 SDK module 路径、Open API 路径和回调路径。

### 3.3 可以后置

- Stripe provider。
- 订阅计划、订阅实例、账单和周期扣款。
- 完整账务、清结算、渠道对账文件导入。
- 成功率/成本/A-B 智能路由。
- 前端 E2E 全链路自动化。

## 4. 非目标

V2 仍不做以下能力：

- 外部商户入驻。
- 商户余额、提现、资金池。
- 自动分账。
- 自动换汇报价。
- 复杂风控评分。
- 资金清结算平台。
- 通用商业 SaaS 计费平台。

## 5. 功能范围

### 5.1 支付失败闭环

当前支付回调只对 `paid` 和 `closed` 做明确处理，失败状态没有落库和事件。

V2 需要：

- provider 将渠道明确失败状态归一为 `failed`。
- 订单服务提供 `MarkFailed` 能力。
- 订单失败后记录失败时间和失败原因。
- 仅从非失败状态首次进入 `failed` 时产生 webhook。
- 新增 `payment.failed` 事件。

建议订单状态：

```text
pending   待支付
paid      已支付
failed    支付失败
closed    已关闭
```

过期订单可以继续存储为 `closed`，通过 `order.expired` 事件表达过期原因。

### 5.2 主动查单补偿

主动查单用于修复支付通道回调丢失或延迟导致的本地状态不一致。

V2 需要：

- provider interface 增加查单能力。
- 定时扫描达到补偿条件的 pending 订单。
- 根据订单通道调用对应 provider 查单。
- 通道已支付：本地标记 `paid`，触发 `payment.succeeded`。
- 通道已关闭：本地标记 `closed`。
- 通道明确失败：本地标记 `failed`，触发 `payment.failed`。
- 通道仍处理中：记录补偿次数和下次补偿时间。
- 达到最大补偿次数仍无结果：标记为异常待人工处理。

建议新增字段或模型：

```text
reconcile_status       pending / processing / resolved / manual_required
reconcile_count        补偿次数
last_reconcile_at      最近补偿时间
next_reconcile_at      下次补偿时间
last_reconcile_error   最近错误
```

字段可以先放在 `payment_orders`，也可以独立建 `payment_reconciliations`。如果后续要做完整对账，优先独立建模。

### 5.3 退款核心能力

退款是 V2 的核心缺口，需要独立领域建模，不能只在订单 metadata 中记录。

V2 需要新增 `refunds` domain：

- `ent/schema/refund.go`
- `internal/domain/refunds/repository`
- `internal/domain/refunds/service`
- `internal/domain/refunds/handler`
- `internal/domain/refunds/router`
- `internal/domain/refunds/test`
- `web/src/features/refunds`

退款核心字段：

```text
refund_no             网关退款号
app_id                应用 ID
payment_order_id      支付订单 ID
gateway_order_no      网关订单号
merchant_order_no     商户订单号
merchant_refund_no    商户退款号
channel               通道
channel_trade_no      通道交易号
channel_refund_no     通道退款号
amount                退款金额，最小货币单位
currency              退款币种
reason                退款原因
status                pending / succeeded / failed / closed
failure_reason        失败原因
metadata              扩展数据
succeeded_at          成功时间
failed_at             失败时间
created_at            创建时间
updated_at            更新时间
```

退款规则：

- 只允许对 `paid` 订单发起退款。
- 同一应用内 `merchant_refund_no` 唯一。
- 同一支付订单可多次部分退款，但累计退款金额不能超过可退金额。
- 重复请求参数一致返回原退款单。
- 重复请求参数冲突返回幂等冲突。
- 退款成功后订单可保持 `paid`，并通过退款金额判断是否部分退款或全额退款。
- 如需要订单层展示，可派生 `refund_status`，不建议直接把订单状态改成 `refunded` 替代退款单。

### 5.4 退款 API

开放 API：

```text
POST /v1/open/refunds
GET  /v1/open/refunds/:refund_no
GET  /v1/open/refunds/by-merchant/:merchant_refund_no
```

后台 API：

```text
GET  /v1/admin/refunds
GET  /v1/admin/refunds/:id
POST /v1/admin/refunds
POST /v1/admin/refunds/:id/retry
```

第一版退款可以不支持商户主动取消退款；如果通道支持撤销退款，后续再扩展。

### 5.5 退款事件

新增事件：

```text
refund.succeeded
refund.failed
```

`refund.succeeded` payload：

```json
{
  "event_type": "refund.succeeded",
  "app_id": "snsgo",
  "gateway_order_no": "gw_...",
  "merchant_order_no": "biz_...",
  "refund_no": "rf_...",
  "merchant_refund_no": "mrf_...",
  "amount": 9900,
  "currency": "CNY",
  "channel": "alipay",
  "channel_trade_no": "...",
  "channel_refund_no": "...",
  "succeeded_at": "2026-07-13T10:00:00Z",
  "metadata": {}
}
```

`refund.failed` payload 需要额外包含：

```json
{
  "failure_reason": "channel rejected refund"
}
```

### 5.6 Webhook 事件模型扩展

当前 `webhook_events` 使用 `(event_type, gateway_order_no)` 唯一约束，适合支付成功和订单过期，但不适合一个订单多次退款。

V2 需要把事件幂等键改成资源维度：

```text
event_type
resource_type   payment_order / refund / subscription / invoice
resource_id     gateway_order_no / refund_no / subscription_no / invoice_no
```

唯一键：

```text
event_type + resource_type + resource_id
```

兼容字段：

- 保留 `gateway_order_no`，便于后台筛选。
- 新增 `refund_no`，便于退款事件筛选。
- 新增 `resource_type` 和 `resource_id`，作为通用幂等键。

### 5.7 后台补齐

V2 后台需要新增或增强：

- 退款列表、退款详情、退款发起、退款重试。
- 订单详情展示退款记录和累计退款金额。
- Dashboard 展示 pending 补偿积压、退款失败数、webhook 失败数。
- Webhook 页面支持按资源类型、退款号筛选。
- 路由页面支持权重目标展示和通道健康状态展示。
- 角色权限页面支持角色 CRUD 和权限策略维护。

### 5.8 路由增强

V2 路由增强分两层。

第一层必须补齐：

- `business_type` 匹配。
- `country` 或 `region` 匹配。
- 多目标按权重选择。
- 通道健康状态为不可用时自动跳过。

第二层可以后置：

- 成功率动态路由。
- 成本最低路由。
- A/B 流量分配。
- 风控评分路由。

### 5.9 Stripe

Stripe 属于独立 provider 能力。若 V2 排期有限，Stripe 可以放到 V2.1。

Stripe 最小范围：

- card 一次性支付。
- webhook 验签。
- payment intent 状态映射。
- refund 创建和 webhook 状态映射。
- USD/EUR/HKD/JPY/GBP 币种支持。

订阅相关 Stripe 能力不纳入 Stripe 一次性支付最小范围。

### 5.10 订阅

订阅是 V1.5/后续阶段能力，V2 只要求文档和数据模型预研，不要求实现。

后续订阅域至少需要：

- Plan
- Subscription
- Invoice
- SubscriptionPayment
- SubscriptionEvent

订阅 webhook 至少包括：

```text
subscription.created
subscription.renewed
subscription.cancelled
subscription.expired
invoice.paid
invoice.payment_failed
```

## 6. 优先级

### P0：主链路闭环

- `payment.failed` 状态和事件。
- 主动查单补偿。
- webhook 事件模型扩展。
- 根目录验证命令覆盖 SDK 测试。

### P1：退款闭环

- Refund schema 和领域代码。
- 开放 API 创建/查询退款。
- 后台退款列表/详情。
- 退款 provider interface。
- `refund.succeeded` / `refund.failed`。

### P2：运营和后台增强

- 异常订单工作台。
- 路由增强。
- 监控增强。
- RBAC 管理后台。

### P3：后续扩展

- Stripe。
- 订阅。
- 账务/结算/对账文件。
- 高级智能路由。

## 7. 验收标准

### 7.1 支付失败

- 渠道返回明确失败状态时，订单进入 `failed`。
- 同一订单重复失败回调不会重复创建 webhook event。
- 商户能收到 `payment.failed` webhook。
- 后台订单详情能看到失败状态和失败原因。

### 7.2 主动查单补偿

- pending 订单达到补偿条件后会进入补偿队列。
- 通道已支付时，本地订单能自动变为 `paid` 并投递 `payment.succeeded`。
- 通道已关闭或失败时，本地状态能自动修正。
- 补偿失败会记录错误和重试时间。
- 达到最大补偿次数后进入人工处理状态。

### 7.3 退款

- paid 订单可以创建退款。
- pending、closed、failed 订单不能创建退款。
- 部分退款和多次退款不超过可退金额。
- 同一 `app_id + merchant_refund_no` 幂等。
- 退款成功投递 `refund.succeeded`。
- 退款失败投递 `refund.failed`。
- 后台能查询退款单和对应订单。

### 7.4 Webhook

- 支付和退款事件使用统一资源幂等键。
- 一个订单多次退款不会因为 `gateway_order_no` 相同而错误去重。
- webhook delivery 仍支持自动重试、手动重试、签名和状态查询。

### 7.5 验证命令

以下命令必须通过：

```bash
make test
make web-typecheck
make web-build
cd web && bun run lint
cd sdk/go && go test ./...
go build ./cmd/server
```

建议新增聚合命令覆盖 SDK：

```bash
make test-all
```

## 8. 风险和约束

- 退款会改变 webhook 事件幂等模型，需要谨慎处理历史数据迁移。
- 主动查单依赖各通道查询接口，provider 抽象需要先稳定。
- 支付失败不是所有通道都会异步通知，不能假设所有失败都能实时落库。
- 多次部分退款必须以数据库约束或事务保证累计金额不超退。
- Stripe 和订阅都可能扩大范围，不能和退款闭环混在同一批强行完成。
- 支付通道回调响应可能要求特殊格式，不能强行套后台 API 的统一响应 envelope。

## 9. 建议实施顺序

```text
1. 扩展 webhook event 资源幂等模型
2. 补 payment.failed 状态流转和事件
3. 增加 provider 查单接口和主动补偿 worker
4. 建 Refund schema 和 refunds domain
5. 接退款 provider interface 和退款事件
6. 补退款后台和订单详情退款记录
7. 增强监控和异常工作台
8. 再评估 Stripe、订阅和账务能力
```

## 10. 文档同步项

V2 实施前后需要同步更新：

- `docs/PAYMENT_GATEWAY_PRD.md`
- `docs/PAYMENT_GATEWAY_INTEGRATION.md`
- `sdk/go/README.md`
- 前端 i18n 文案
- Open API 示例和签名示例

当前已知文档漂移：

- PRD 提到 `/v1/payment/orders`，当前实现和集成文档使用 `/v1/open/orders`。
- PRD 提到分通道回调路径，当前实现使用统一 `/v1/channel/notify`。
- PRD 提到 Stripe，当前实现是 Alipay、WeChat、PayPal。
