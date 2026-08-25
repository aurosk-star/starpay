# shadcn-admin 前端替换设计

## 目标

在不改变支付网关现有业务能力、API 合约和 URL 的前提下，将管理后台前端统一迁移为 [satnaing/shadcn-admin](https://github.com/satnaing/shadcn-admin) 的布局、交互和组件风格。

## 范围与不变项

- 保留现有 Rsbuild 构建入口，不切换到 Vite，避免影响 Docker 和部署工作流。
- 保留 TanStack Router 的现有 URL：`/`、`/apps`、`/orders`、`/webhooks`、`/refunds`、`/reconciliations`、`/channels`、`/routing`、`/config/gateway`、`/users`、`/test-pay`、`/checkout/:orderId` 及其详情/编辑子路由。
- 保留现有 API 客户端、TanStack Query、Zustand 鉴权状态、后端响应格式和登录/退出行为。
- 保留中英文 i18n，并将新增 shell、命令菜单、错误状态和无障碍文本加入 `en.json` 与 `zh-CN.json`。
- 收银台和支付结果页继续使用独立的无侧边栏布局；管理后台页面使用新的认证布局。
- 现有业务页面的数据查询、表单校验、支付状态和权限语义不因换肤而改变。

## 方案

采用“保留业务内核、替换应用壳层”的渐进式迁移：以 shadcn-admin 当前 `components/layout`、`context`、`components/data-table` 和 `styles` 的设计为参考，将其适配到本项目既有目录和 Rsbuild 配置中，不直接复制上游示例业务或 Clerk 集成。

### 应用结构

```text
App
├── ThemeProvider / QueryClientProvider
├── AuthScreen（未登录）
└── RouterProvider
    ├── PublicCheckoutLayout（/checkout/*）
    └── AuthenticatedLayout
        ├── AppSidebar
        ├── Header + Breadcrumb + CommandMenu
        └── Main
            └── 现有业务 Outlet
```

新的 shell 只负责导航、主题、用户菜单、命令菜单、页面标题、跳过主内容链接和统一页面容器；业务页面继续位于 `web/src/features/`，路由定义继续位于 `web/src/routes/`。

### 导航模型

在 `web/src/config/navigation.ts` 建立唯一导航元数据，包含分组、图标、翻译 key、URL、匹配规则和权限标识。侧边栏与命令菜单都消费这份数据，避免重复维护。现有导航分为“工作台”“支付”“平台”三组，订阅入口继续显示为禁用状态，直到后端能力存在。

### 页面视觉与组件

- 使用 shadcn-admin 风格的可折叠侧边栏、顶部栏、面包屑、页面标题区和响应式内容容器。
- 将上游数据表工具栏、列显示、分页、筛选和 URL 状态管理模式适配到现有 `web/src/components/data-table/`，不改变业务表格的数据类型。
- 增加统一的页面状态组件：加载骨架、空数据、错误重试、危险操作确认和 toast。
- 使用 `cmdk` 增加 `⌘K/Ctrl+K` 命令菜单，可跳转所有后台页面并切换主题/语言。
- 主题继续支持 light/dark/system；默认行为和 `d` 快捷键保持兼容。

### 认证与错误边界

不引入上游 Clerk。沿用当前 `useAuthStore` 和 `AuthScreen`，在路由根部增加认证布局、未知路由 404、未授权 401/403、服务器错误 500 和维护态页面；错误页面使用现有 i18n 和响应式 shadcn 组件。

### 许可证与第三方声明

上游 `shadcn-admin` 为 MIT License。本项目继续以 Apache-2.0 发布，并在 `web/THIRD_PARTY_NOTICES.md` 保留上游版权、MIT 条款和仓库地址；本地新增代码版权归本项目所有。

## 页面迁移顺序

1. 应用壳层、主题、导航、用户菜单、命令菜单和错误边界。
2. 工作台 `/`，确保统计卡片、图表和最近 Webhook 投递在新容器中正常显示。
3. 应用、订单、Webhook、退款、对账列表及详情页。
4. 支付通道、路由规则、网关配置和用户管理。
5. 收银台、支付结果和测试支付页的独立布局适配。
6. 删除旧的重复 shell 代码，统一组件导出、样式 token 和文档。

## 验收标准

- `bun run typecheck`、`bun run lint`、`bun run build` 和现有前端测试全部通过。
- 未登录访问管理后台仍显示登录页；登录后所有现有 URL 可直接访问，刷新不丢失路由。
- 侧边栏折叠、移动端抽屉、主题切换、语言切换、用户退出和命令菜单在桌面/移动视口可用。
- 所有列表页继续使用可复用 Data Table，不出现业务页面手写重复表格。
- API 请求、响应解析、金额显示、Webhook 重试、退款和对账筛选行为与迁移前一致。
- 收银台不显示管理后台侧边栏，支付流程和结果页视觉统一但功能不变。
- 新增组件具备键盘焦点、可读名称、`aria-current`/`aria-label` 和跳过主内容链接。
- 安全扫描不引入已知高危依赖，第三方声明完整。

## 回滚策略

所有改动集中在独立分支；每个迁移阶段单独提交。若某一阶段验证失败，可回退该阶段提交，不触碰后端和 API。合并前以完整前端构建、类型检查、Lint、单元测试和人工路由清单作为门禁。
