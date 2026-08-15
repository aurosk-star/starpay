# Starpay Web

Starpay 的 React 管理后台和收银台，使用 Bun 1.3.13、Rsbuild、Tailwind CSS 和 shadcn/ui 源码组件。

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
node --test test/*.test.mts
bun run typecheck
bun run lint
bun run build
bun audit
```

界面变更还需人工检查管理后台、收银台和深浅色切换。

## shadcn/ui 约束

`src/components/ui/` 中的 shadcn/ui 组件和 `src/styles/shadcn.css` 都是仓库源码，运行和构建不依赖 `shadcn` CLI。

确需新增或更新组件时，只能在独立分支中一次性使用经过审计且固定精确版本的 CLI。生成后应审查差异、运行全部验证，并确保 CLI 没有写入 `package.json` 或锁文件。
