# Starpay Web

Starpay 的 React 管理后台和收银台，使用 Bun 1.3.13、Rsbuild、Tailwind CSS 和 shadcn/ui 源码组件。

管理后台采用 shadcn-admin 风格的响应式 shell；业务 API、路由和认证状态仍由本仓库维护。上游项目的 MIT 归属和完整许可文本见 [`THIRD_PARTY_NOTICES.md`](THIRD_PARTY_NOTICES.md)。

## 安装

```bash
bun install --frozen-lockfile
```

锁文件是构建输入的一部分，不要在 CI 或发布构建中省略 `--frozen-lockfile`。

## 开发

```bash
bun run dev
```

默认开发地址由 Rsbuild 输出。后端需单独运行，开发代理配置见 `rsbuild.config.ts`。

## 验证

```bash
bun test
bun run typecheck
bun run lint
bun run build
bun audit
```

界面变更还需人工检查管理后台、收银台和深浅色切换。

## shadcn/ui 约束

`src/components/ui/` 中的 shadcn/ui 组件和 `src/styles/shadcn.css` 都是仓库源码，运行和构建不依赖 `shadcn` CLI。

确需新增或更新组件时，只能在独立分支中一次性使用经过审计且固定精确版本的 CLI。生成后应审查差异、运行全部验证，并确保 CLI 没有写入 `package.json` 或锁文件。

## 管理后台架构约定

- **Shell ownership**：`src/App.tsx` 负责主题、Toast 和认证会话启动；`src/routes/root.tsx` 根据路由和会话选择登录边界、公开收银台或 `AuthenticatedLayout`。认证后的侧边栏、顶栏、跳过链接和主内容容器由 `src/components/layout/` 统一维护。
- **Navigation metadata**：`src/config/navigation.ts` 是导航的单一元数据源。侧边栏和命令菜单都从这里读取分组、标题 key、图标和 URL；新增后台入口先更新元数据，再在 i18n 资源中补齐标题。
- **Page states and headings**：业务页使用共享的 `Main`、`PageHeader`，并可使用 `PageState`（加载、错误、空态）保持标题、描述和操作区的响应式布局一致。不要在业务页重复实现 shell 或页面标题结构。
- **Data tables**：列表页必须通过 `src/components/data-table/` 的 `createDataTable` 工厂和共享 `DataTable` 组件渲染。筛选工具栏、列显隐、排序、分页、空态和行操作应使用已有扩展点，不要手写重复的 `<table>` 标记。
- **Authentication boundary**：后台 API 页面要求管理员会话；`App.tsx` 会先恢复 refresh cookie 或校验 access token，未认证时仅渲染登录/初始化界面。`/checkout/*` 是公开收银台路径，不挂载管理员 sidebar 或用户菜单，但仍使用共享主题 token。

样式语义 token（颜色、圆角、字体、focus ring、sidebar 和 chart）只定义在 `src/styles/index.css`；`src/styles/shadcn.css` 保留上游动画、variant 和 utility recipe，避免出现两套 token 覆盖顺序。键盘导航、skip link、sidebar drawer、深浅色模式和 reduced-motion 需要在界面变更后人工检查。

第三方归属和许可证清单维护在 [`THIRD_PARTY_NOTICES.md`](THIRD_PARTY_NOTICES.md)，其中包含 [satnaing/shadcn-admin](https://github.com/satnaing/shadcn-admin) 的 MIT 声明。
