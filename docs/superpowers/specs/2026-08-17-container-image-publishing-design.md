# 双仓库容器镜像发布设计

## 目标

为 StarPay 增加可重复、可审计的容器镜像发布流程。一次构建同时发布到 GitHub Container Registry（GHCR）和 Docker Hub，使默认部署脚本能够拉取当前 `main` 对应的 `latest` 镜像，同时保留可回溯的提交及版本标签。

## 范围

本次新增一个独立的 GitHub Actions 工作流，并配置 Docker Hub 发布凭据。部署脚本继续默认使用 `ghcr.io/aurosk-star/starpay:latest`；Docker Hub 的 `zxabugx/payment-gateway` 作为公开镜像站，可通过 `--image` 显式选择。

本次不修改应用代码、数据库结构、生产环境变量或现有 Release，也不把当前 `main` 错误标记成已有的 `v0.2.0`。版本镜像只从对应的 Git 标签构建。

## 发布入口

工作流支持以下事件：

- 针对工作流文件的 Pull Request：只构建、不登录仓库、不推送，用于提前验证镜像构建。
- 推送到 `main`：构建并发布 `latest` 与提交 SHA 标签。
- 推送符合 `v*.*.*` 的 Git 标签：发布原始 Git 标签、完整语义版本和主次版本标签。
- `workflow_dispatch`：允许从 GitHub Actions 手动发布所选分支；当前 `main` 可用此入口立即补发镜像。

并发控制按 Git 引用分组，同一引用的新运行取消旧运行，避免重复发布过期构建。

## 镜像与标签

同一个 BuildKit 构建结果推送到：

- `ghcr.io/aurosk-star/starpay`
- `docker.io/zxabugx/payment-gateway`

标签规则如下：

| 触发来源 | 示例标签 |
| --- | --- |
| `main` 或在 `main` 上手动运行 | `latest`、`sha-35f611a` |
| Git 标签 `v0.3.0` | `v0.3.0`、`0.3.0`、`0.3`、`sha-...` |
| Pull Request | `sha-...`，但不推送 |

`latest` 仅表示默认分支最新构建。生产部署仍可通过 `--image` 使用版本标签或 digest，以获得不可变发布。

## 构建与供应链安全

- 使用 Docker 官方 Actions，并将所有第三方 Actions 固定到完整 commit SHA。
- 使用 Buildx 构建 `linux/amd64` 与 `linux/arm64` 镜像。
- Bun 和 Go 构建阶段固定在 Runner 原生平台执行，Go 使用 BuildKit 的 `TARGETOS`/`TARGETARCH` 交叉编译，避免在 QEMU 中执行完整工具链。
- 启用 GitHub Actions 缓存以减少重复构建时间。
- 生成 OCI provenance 和 SBOM attestation。
- 工作流只授予 `contents: read`、`packages: write`、`attestations: write` 和 `id-token: write`。
- PR 构建不接触发布凭据，也不执行 registry login 或 push。

现有 Security 工作流继续负责 Trivy 漏洞门禁；发布工作流不重复运行另一套扫描。

## 凭据与可见性

Docker Hub 凭据保存为 GitHub Actions repository secrets：

- `DOCKERHUB_USERNAME`
- `DOCKERHUB_TOKEN`

令牌只通过标准输入提交给 GitHub，不写入命令参数、仓库文件、提交、部署记录或 Actions 日志。GitHub 登录使用当前仓库的 `GITHUB_TOKEN`，无需额外 PAT。

首次发布后，工作流使用仅限当前仓库、具有 `packages: write` 权限的 `GITHUB_TOKEN` 将 GHCR 的 `starpay` 包可见性设置为 public，并保留与 `aurosk-star/starpay` 仓库的关联。Docker Hub 仓库保持 public，不依赖个人 GitHub PAT 管理 GHCR 可见性。

由于本次令牌曾通过聊天提供，发布验证完成后应在 Docker Hub 轮换令牌，并同步更新 `DOCKERHUB_TOKEN` Secret。

## 失败处理

- 任一仓库登录、构建或推送失败会使工作流失败，避免把部分成功误报为完整发布。
- 双仓库使用同一组标签和同一构建输出，防止两个仓库内容漂移。
- 发布后分别检查两个 registry 的远端 manifest；只有两个 `latest` digest 均存在且平台完整时才视为成功。
- 发布失败不会影响已经存在的旧镜像；修复后可通过手动触发重新发布当前 `main`。

## 验证与完成标准

1. 工作流 YAML 通过静态检查，所有 Actions 均固定 SHA。
2. Pull Request 中双平台镜像构建成功且没有推送。
3. 合并后手动发布当前 `main` 成功。
4. GHCR 与 Docker Hub 的 `latest` 均包含 `linux/amd64` 和 `linux/arm64`。
5. 两个仓库的 `latest` 指向同一构建内容，并带有提交 SHA 标签。
6. 匿名执行 `docker pull ghcr.io/aurosk-star/starpay:latest` 成功。
7. 匿名执行 `docker pull zxabugx/payment-gateway:latest` 成功。
8. 当前部署脚本使用默认设置时能够拉取 GHCR 的最新镜像。
9. GitHub 与 CodeUP 的 `main` 最终保持同步。
