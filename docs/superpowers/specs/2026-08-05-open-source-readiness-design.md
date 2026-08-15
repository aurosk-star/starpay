# 开源就绪设计

## 目标

以 Apache License 2.0 协议完成支付网关的首次源码公开发布准备。首次公开发布必须具备干净的安全基线、自动化持续集成、清晰的贡献与漏洞报告渠道，并确保不会在缺少兼容授权条款的情况下重新分发已知第三方材料。

公开的支付网关仓库与现有公开的 `zmoyi/starpay-go` SDK 仓库保持独立。支付网关采用 Apache-2.0 协议，SDK 继续使用现有 MIT 协议。

## 当前基线

仓库当前工作区干净，后端测试、`go vet`、SDK 测试、前端测试、TypeScript 检查、前端 lint 和构建，以及部署脚本测试均可通过。

开源就绪审计发现以下阻断项：

- 仓库根目录没有许可证和社区健康文件。
- 仓库没有 GitHub Actions 工作流。
- `govulncheck` 从 Go 工具链和依赖图中报告 9 个可达漏洞。
- `bun audit` 报告 16 个前端依赖漏洞，其中 7 个为高危漏洞。
- 仓库在 `.agents/skills/` 下跟踪了 111 个第三方文件，但没有附带对应许可证文件。
- 面向公众的文档仍包含私有 Codeup 克隆地址、个人容器命名空间、针对已公开 SDK 的 `GOPRIVATE` 配置，以及通用的 Rsbuild 前端 README。
- 两个测试文件包含仅供测试使用的 PEM 私钥夹具，预计会触发通用密钥扫描器。

## 选定方案

先收缩攻击面，再升级依赖。删除不参与应用运行或可重复构建的工具，升级其余直接依赖和传递依赖到已修复版本，并通过自动化安全门禁防止漏洞回归。

不得仅为获得绿色检查结果而压制真实漏洞。只有经确认不属于真实密钥的测试夹具，或已证实的误报，才允许配置扫描例外。每条例外必须精确限定到文件路径，在版本控制中写明原因，并且不能忽略生产凭据或可达漏洞。

## 许可证与第三方材料

在根目录添加未经修改的 Apache License 2.0 正文，文件名为 `LICENSE`。添加 `NOTICE`，项目名称使用 Starpay，版权主体写为“Starpay contributors”。添加 `THIRD_PARTY_NOTICES.md`，列出许可证要求署名的依赖或随附资源。

不对 Ent 生成代码添加 SPDX 文件头，也不机械修改所有源文件。仓库级许可证已经足够，可以避免生成代码产生无意义变更，同时保留现有上游声明。

从公开源码树中删除 vendored `.agents/skills/` 目录和 `skills-lock.json`。这些内容属于开发环境材料，不属于支付网关源代码。保留 `AGENTS.md`，因为它记录的是仓库自身工程规范，没有打包第三方 skill 实现。

保留 `docs/superpowers/specs/` 和 `docs/superpowers/plans/`，因为它们属于项目自身的工程记录。发布前扫描其中的私有地址、凭据、个人数据和过时运维说明。

## 后端漏洞修复

在根模块和 SDK 模块中添加 `toolchain go1.26.5`，将受支持工具链固定为 Go 1.26.5。Docker 构建阶段使用 `golang:1.26.5`，不再使用 `golang:latest`。

将 `golang.org/x/text` 升级到至少 v0.39.0，将 `github.com/quic-go/quic-go` 升级到至少 v0.59.1。如果升级直接父依赖可以获得已修复的传递依赖版本，应优先升级父依赖。升级完成后运行 `go mod tidy`，只保留构建和工具链实际需要的依赖。

后端安全门禁为 `govulncheck ./...`，要求可达漏洞数量为 0。已导入但不可达的符号漏洞仍需保留在日志中并进行审查，但只有可达漏洞会阻断发布。

## 前端漏洞修复

从 `web/package.json` 删除 `shadcn` CLI 包。已生成的 shadcn/ui 组件本身就是源码文件，应用运行和常规构建都不依赖该 CLI。维护者以后需要生成新组件时，文档应要求使用当时已通过安全审计的固定版本一次性 CLI。

升级 Tailwind/PostCSS 以及其他剩余直接依赖，使其传递依赖图不再包含已知安全通告。使用仓库规定的 Bun 版本重新生成 `web/bun.lock`。优先采用上游已修复版本；当父依赖已有修复版本时，不得使用 override。只有当 override 选择的是语义化版本兼容的安全修复版本、包格式支持写入解释性注释，并且已记录明确移除条件时，才允许临时使用 override。

首次公开发布要求 `bun audit` 检查结果为 0，并且包含开发依赖。这同时保护交付的前端产物和维护者的构建环境。

## 密钥与配置安全

继续确保 `.env` 和 `.env.production` 同时被 Git 和 Docker 构建上下文排除。将 `.env.production.example` 中固定的应用密钥加密 Key 改为不可直接运行的替换标记。生产环境启动时必须拒绝文档中的开发 JWT 密钥、开发应用密钥加密 Key，以及尚未替换的 `CHANGE_ME` 值。

只有在支付渠道测试确实需要可解析密钥时才保留 PEM 夹具。将其明确标记为仅供测试使用，并为密钥扫描器添加精确到文件和规则的例外。例外不能匹配其他私钥或其他路径。

首次公开推送前，使用 gitleaks 扫描完整 Git 历史。任何提交中发现真实凭据时，必须先轮换凭据，再从历史中移除。如果不希望现有作者邮箱公开，应在首次推送支付网关前将其重写为 GitHub noreply 邮箱。

## 持续集成设计

创建 `.github/workflows/ci.yml`，在 Pull Request 和推送到 `main` 时运行。工作流采用最小权限的只读权限，分别运行后端、SDK、前端和仓库安全任务：

- 后端：使用 Go 1.26.5，下载模块，运行 `go test ./...`、`go vet ./...` 和 `govulncheck ./...`。
- SDK：在 `sdk/go` 中运行 `go test -count=1 ./...` 和 `go vet ./...`。
- 前端：使用 Bun 1.3.13，冻结安装依赖，运行 Node 测试、TypeScript 检查、oxlint、生产构建和 `bun audit`。
- 部署工具：运行 `bash scripts/deploy_test.sh`。
- 仓库安全：扫描完整历史的 gitleaks，只允许经过审查的测试密钥例外。

创建 `.github/workflows/security.yml`，支持每周定时运行、手动触发、推送到 `main`，并在扫描器支持时对 Pull Request 运行。工作流使用 CodeQL 分析 Go 和 JavaScript/TypeScript，构建应用镜像，使用 Trivy 扫描镜像，并生成 SPDX 或 CycloneDX SBOM。容器存在高危或严重漏洞时阻断工作流。所有扫描 Action 固定到不可变提交 SHA，由 Dependabot 负责提出升级。

创建 `.github/dependabot.yml`，为 Go 模块、前端包生态、GitHub Actions 和 Docker 基础镜像配置每周分组更新。按照项目现有决定，Compose 中的 PostgreSQL 和 Redis 继续使用 `latest`；公开部署文档必须明确说明该策略及其运维取舍。

CI 工作流不发布镜像、不创建 Release，也不修改仓库设置。发布自动化和 GitHub 分支保护配置属于开源就绪变更合并后的独立后续工作。

## 社区与公开文档

新增：

- `SECURITY.md`：包含受支持版本、私密报告渠道、预期确认和修复时限，并要求不要为尚未修复的漏洞创建公开 Issue。
- `CONTRIBUTING.md`：包含本地环境配置、必需验证命令、架构边界、提交规范，以及 Developer Certificate of Origin 签署要求。
- `CODE_OF_CONDUCT.md`：采用 Contributor Covenant 2.1。
- `.github/PULL_REQUEST_TEMPLATE.md`，以及聚焦明确的缺陷、功能建议和安全报告 Issue 配置。
- 根目录变更日志，记录首次公开发布的基线。

更新根 README，增加 Apache-2.0 许可证、构建和安全状态徽章、成熟度声明、支持的 Go/Bun 版本、五分钟快速启动、截图或视觉验证说明、安全警告，以及指向安全政策和贡献指南的链接。

将通用的 `web/README.md` 替换为前端专用开发和验证说明。把生产部署文档中的私有 Codeup 地址替换为 `https://github.com/aurosk-star/starpay`，删除公开 SDK 的 `GOPRIVATE` 配置说明，并将个人容器命名空间默认值替换为 `ghcr.io/aurosk-star/starpay`，同时保留现有环境变量覆盖能力。

## 验证与发布门禁

只有在全新检出环境中满足以下所有条件，开源就绪变更才算完成：

```text
go test ./...                          通过
go vet ./...                           通过
govulncheck ./...                      报告 0 个可达漏洞
cd sdk/go && go test -count=1 ./...    通过
cd sdk/go && go vet ./...              通过
cd web && node --test test/*.test.mts  通过
cd web && bun run typecheck            通过
cd web && bun run lint                 通过
cd web && bun run build                通过
cd web && bun audit                    报告 0 个漏洞
bash scripts/deploy_test.sh             通过
gitleaks 完整历史扫描                   报告 0 个未放行密钥
Trivy 应用镜像扫描                      报告 0 个高危或严重漏洞
git status --short                     无输出
```

相同命令必须在 GitHub Actions 中通过。本次实施不负责将仓库设为公开，也不创建发布标签；这些操作由仓库所有者在审查干净的 CI 结果和最终 Git 历史后明确执行。
