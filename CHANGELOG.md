# Changelog

本项目的所有重要变更都会记录在此文件中。

格式基于 [Keep a Changelog](https://keepachangelog.com/en/1.1.0/)，版本遵循 [Semantic Versioning](https://semver.org/spec/v2.0.0.html)。

## [Unreleased]

### Added

- 添加 tag 触发的自动发布 workflow，按 `CHANGELOG.md` 生成 GitHub Release。

## [0.3.0] - 2026-11-24

### Added

- 新增基于 shadcn-admin 的独立管理后台 `web-admin/`，生产镜像与默认 `make web-*` 目标均切换至该前端。
- 新增 `web-admin` CI job：锁定依赖安装、TypeScript 类型检查、构建与依赖审计。

### Changed

- 旧版前端 `web/` 保留为收银台和迁移期回退版本，新增 `make web-legacy-*` 命令。
- 升级 Go 工具链至 1.26.11，并同步升级 `golang.org/x/net`、`golang.org/x/crypto` 等依赖。
- 容器基础镜像更新为 `golang:1.26.11-alpine3.23` 与 `alpine:3.24.4`。
- 前端升级 axios、postcss、zod、`@tanstack/router-plugin`，并通过 `overrides` 收敛传递依赖。
- `frontend` CI 单元测试改用 `bun test test/*.test.mts`，与本地 `make web-test` 保持一致。
- 更新 README、CONTRIBUTING 与生产部署文档，说明默认前端入口和本次变更。

### Security

- 修复 `govulncheck`、`bun audit` 与镜像 Trivy 扫描报告的高危问题，当前扫描无未修复告警。
- 合并 Dependabot 的 go-modules 与 github-actions 更新，关闭已被取代的前端依赖与镜像基础镜像 PR。

## [0.2.0] - 2026-08-15

### Added

- 添加 Apache License 2.0、NOTICE 和第三方软件声明。
- 添加 GitHub Actions CI、CodeQL、依赖审查、gitleaks、Trivy 和 SPDX SBOM 门禁。
- 添加 Dependabot、贡献指南、安全政策、行为准则以及结构化 PR/Issue 模板。

### Changed

- 升级到 Go 1.26.6，并升级已知漏洞相关 Go 依赖。
- 移除前端 `shadcn` CLI 依赖，固定安全的 Tailwind/PostCSS 版本并将所需 MIT 样式纳入源码。
- 固定容器构建镜像版本，使用精简的 Alpine Go builder，并统一公开仓库与 GHCR 地址。
- 清理本地 agent skills、私有仓库说明和个人镜像命名空间。

### Security

- 生产环境拒绝开发默认密钥和未替换的占位值。
- 后端 `govulncheck`、前端 `bun audit`、完整历史密钥扫描和镜像高危漏洞扫描成为发布门禁。

[Unreleased]: https://github.com/aurosk-star/starpay/commits/main
[0.3.0]: https://github.com/aurosk-star/starpay/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/aurosk-star/starpay/releases/tag/v0.2.0
