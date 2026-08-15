# 贡献指南

感谢你为 Starpay 做出贡献。提交代码即表示你同意遵守 [行为准则](CODE_OF_CONDUCT.md) 和本指南。

## 开发环境

需要 Go 1.26.6、Bun 1.3.13、Docker 和 Docker Compose v2。

```bash
go mod download
cd web && bun install --frozen-lockfile && cd ..
make db-up
make dev
```

前端开发使用 `make web-dev`。SDK 位于 `sdk/go`，保留独立 MIT 许可证。

## 架构与实现约束

请先阅读 [AGENTS.md](AGENTS.md)，并遵守以下现有约定：

- 后端按 `internal/domain/<module>/` 分域，业务行为放在 service、持久化放在 repository、HTTP 解析和序列化放在 handler、路由注册放在 router；
- 所有 API 响应继续使用 `{ code, message, data, error }`，不要在 handler 中手写其他响应结构；
- 金额只使用整数最小货币单位存储和传输；
- 前端以仓库配置的 shadcn/ui 和 Tailwind 语义 token 为主，新增文本必须进入 i18n 资源；
- 业务数据列表必须复用 `web/src/components/data-table/`，不得手写重复表格；
- 不得提交支付密钥、应用密钥、证书、私钥、签名密钥或真实凭据。

行为变化必须包含测试。后端测试放在对应 domain 的 `test/` 目录，并优先从外部测试包验证导出行为。

## Required verification

```bash
make verify
docker build -t starpay:contributor-check .
```

涉及界面时，请同时启动 `make web-dev`，检查管理后台、收银台以及深浅色切换，并在 PR 中记录结果或附截图。

## 提交 Pull Request

1. 从最新 `main` 创建分支。
2. 保持提交聚焦，使用简洁的祈使句提交标题。
3. 更新测试、配置示例和相关文档。
4. 完成上述验证并填写 PR 模板。
5. 安全漏洞请按 [SECURITY.md](SECURITY.md) 私密报告，不要直接提交公开修复细节。

## Developer Certificate of Origin

All commits must include a `Signed-off-by` trailer created with `git commit -s`. By contributing, the author certifies the Developer Certificate of Origin 1.1: https://developercertificate.org/
