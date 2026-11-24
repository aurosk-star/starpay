# Starpay 管理后台

这是 Starpay 的新管理后台，使用 [satnaing/shadcn-admin](https://github.com/satnaing/shadcn-admin) 作为 UI 基线，并保留其 MIT 许可要求（见 [`THIRD_PARTY_NOTICES.md`](THIRD_PARTY_NOTICES.md)）。

## 本地开发

```bash
bun install --frozen-lockfile
bun run dev
```

默认地址为 `http://localhost:3001/`（具体端口以 Rsbuild 输出为准）。开发代理将 `/v1` 和 `/api` 请求转发到 `VITE_BACKEND_URL`，默认是 `http://localhost:8080`。

## 验证

```bash
bun run typecheck
bun run build
```

业务 API 和认证合同来自 Starpay 后端；上游 shadcn-admin 的演示数据、Clerk 集成和 mock API 不会进入生产路径。
