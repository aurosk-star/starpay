# Changelog

本项目的所有重要变更都会记录在此文件中。

格式基于 [Keep a Changelog](https://keepachangelog.com/en/1.1.0/)，版本遵循 [Semantic Versioning](https://semver.org/spec/v2.0.0.html)。

## [Unreleased]

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
