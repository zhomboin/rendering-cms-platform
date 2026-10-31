# 项目主进度记录

本文件是整个项目唯一的主进度记录文档，用于记录每次修改，由 Agent 在每次变更后定期维护。各专项进度不在此文档内展开，统一通过下方链接进入独立文档。

## 记录格式

每条记录追加在表格末尾，只描述最新状态和结果，不书写历史版本过程；被推翻或废弃的内容一律记录到 `docs/change-log.md`。

## 修改记录

| 日期 | 变更主题 | 范围 | 摘要与关联文档 |
| --- | --- | --- | --- |
| 2026-10-21 | 落地项目管理规范 | 文档 / 规范 | 明确所有项目文件 UTF-8 无 BOM；确立 `docs/`、`reports/`、`temp/` 目录约定，确认 `reports/` 与 `temp/` 均被 `.gitignore` 覆盖（本次新增 `reports/` 规则）；建立本主进度记录与 `docs/change-log.md` 变更记录机制；确立新功能分支开发 + PR 合入 + 代码检视流程。规范见 `AGENTS.md`。变更经分支 `docs/project-conventions` 提交，代码检视修复 4 项问题后经 PR #7 合入 `main`。 |
| 2026-10-21 | 登录邮箱验证码（两步登录） | 后端 / 前端 / 数据库 / 文档 | 密码验证成功后按风险判定（登录 IP 变更或连续失败 3 次）触发 6 位邮箱验证码，验证通过才发放登录令牌；新增 migration `000012_login_email_mfa`、`internal/mail` SMTP 发信、`/api/v1/auth/mfa/verify|resend` 接口、登录页两步表单；验证码 10 分钟有效、单码 5 次原子校验上限、60 秒发送间隔、每小时 5 封，猜错计入登录锁定；SMTP 未配置自动关闭。变更经分支 `feat/login-email-mfa` 提交，两轮代码检视（修复并发绕过校验上限等 7 项问题）后经 PR #8 合入 `main`。生产启用需配置 SMTP 授权码并执行 migration，详见 `docs/plans/2026-10-21-login-email-mfa.md` 与 `docs/apis/auth.md`。 |
| 2026-10-22 | Go 工具链安全升级与认证测试时钟修复 | 后端 / 部署 / 文档 | 修复 `govulncheck` 检出的 23 个可达漏洞（含 CVE-2026-111943 crypto/x509 信任链缺陷）：`backend/go.mod` 升级 `go 1.26.10`，`golang.org/x/text` v0.41.0、`golang.org/x/sync` v0.22.0，生产构建、本地开发与 MDX 导入脚本镜像固定 `golang:1.26.10-alpine`；同时修复 `internal/auth/mfa_test.go` 硬编码时钟导致的定时失败并补齐 `config`、`logging` 的 gofmt。升级后 `govulncheck` 可达漏洞为 0，`go test ./...` 全绿。变更经分支 `fix/go-1.26.10-security-upgrade` 提交，代码检视修复 2 项问题（`backend/README.md` 版本号同步、`scripts/ops/import-mdx.sh` 镜像固定）后经 PR #10 合入 `main`。 |
| 2026-10-23 | 相册功能（CMS 侧） | 后端 / 前端 / 数据库 / 文档 | 后台维护相册与照片，向 Rendering 博客 `/photos` 提供公开查询接口：新增 migration `000013_photo_albums`、`000014_album_view_analytics`（`photo_albums`/`album_photos` 表、`assets` 宽高列、相册浏览日聚合与每日 IP 哈希去重），`internal/imaging` 魔数嗅探、像素尺寸提取、EXIF/XMP 等元数据剥离与拍摄时间提取，`internal/albums` 公开三个分页查询接口（分页信封、越界空 data、未发布 404、ETag/304）与后台管理接口、照片后端代理上传（20MB 上限、UUID 不可变 key、immutable 长缓存头），相册浏览上报；前端后台相册列表页与编辑页（照片墙：上传、排序、设封面、删除）。变更经分支 `feature/photo-albums` 提交，代码检视（approve-with-comments）修复 2 项 major（元数据剥离扩展到 XMP 与厂商段、服务端 Read/WriteTimeout 放宽至 120s 匹配 20MB 代理上传）后经 PR #11 合入 `main`。生产部署需先备份 PostgreSQL 再执行 migration，配置 `S3_PHOTO_PREFIX=photos`，R2 桶保持公开读；契约见 `docs/apis/photos.md`，计划归档于 `docs/archive/plans/2026-10-23-photo-albums.md`。检视另遗留 5 项 minor 跟进（相册上报端到端测试、上传失败对象清理、相册删除事务化、`docs/operations/deployment.md` 补 `S3_PHOTO_PREFIX` 说明、批量上传 position 并发），未排期。 |
| 2026-10-30 | 自动部署日志退出排空 | 部署 / 测试 / 文档 | 脚本退出前等待日志消费者写完标准输出和日志文件，并保留原始退出码；无新提交、受控异常退出 37 和锁冲突跳过三个本地回归用例通过，Shell 语法与开发脚本检查通过，测试进程均已回收。已通过 [PR #12](https://github.com/zhomboin/rendering-cms-platform/pull/12) 合入并在生产生效；普通检查退出码为 0，日志文件与 journal 的结束信息完整，定时器正常；机制与验证入口见 `docs/operations/auto-deploy.md`。 |
| 2026-10-30 | 自动部署备份失败保护 | 部署 / 测试 / 文档 | 显式检查导出、压缩、gzip 完整性、解压内容非空与文件权限；备份失败阻断目标提交的 migration、容器更新和成功状态写入，原有代码回滚策略与备份参数不变。隔离主流程的 7 个测试覆盖 8 个场景，全部通过；日志回归、Shell 语法与开发脚本检查通过，测试进程均已回收。已合入并在生产生效，生产配置与备份参数未变化；失败分支由隔离测试覆盖，生产未注入故障；机制与验证入口见 `docs/operations/auto-deploy.md`。 |
| 2026-10-30 | 相册新建页面 | 前端 / 测试 / 文档 | 新建模式按路由是否存在相册 ID 判断，创建成功后进入真实 ID 的编辑页；三项真实页面回归覆盖新建、失败重试、编辑与刷新，接口使用隔离响应，已确认修复前失败与修复后通过。生产验收结果随修复 PR 记录；验证须包含真实草稿创建、读取与清理。测试入口见 [前端说明](../frontend/README.md)。Rendering 空摘要消费修复由博客项目处理，本仓库不修改或部署博客；备份策略保持不变。 |
| 2026-10-30 | 本地配置隔离与生产安全规范 | 文档 / 运维 | 服务器连接、网关、开发环境变量与本机配置统一记录在本地文档，并排除 Git 跟踪和 Docker 打包；公共文档只保留通用规范与本地入口。生产写操作须有任务授权，未定义高级操作先定义并获批。见 [生产访问](operations/production-access.md)。 |

| 2026-10-31 | 相册图片上传与独立管理 | 前端 / 测试 / 文档 | 上传清除 JSON 请求头默认值，由浏览器生成 multipart boundary；基本信息和图片管理使用独立页面，图片 URL 可展示和复制，保留照片编辑、排序、封面与删除。浏览器回归验证文件原始内容、拖拽失败重试、URL 复制及边界，生产构建通过；仅提交并合并 PR，生产实图与对象存储验收待部署后进行，备份策略不变。既有锁文件依赖审计报告 5 项 high，依赖升级单独跟进。 |

| 2026-10-31 | 首页数据驾驶舱 | 后端 / 前端 / 测试 / 文档 | 默认 30 天，可切换 7／30／90 天；展示六项指标、访问趋势、文章图文排行、相册、运营待办、近期内容与资源摘要。新增三个后台只读接口，按现有每日去重口径统计，不修改 schema；相关 Go 测试、真实 PostgreSQL 边界验证、sqlc 生成一致性、前端构建、8 项首页浏览器回归和真实服务联调通过，独立检视意见已修复并复验。GitHub CI 配置全量验证；生产数据与图片效果待部署后验收。临时服务均已关闭，说明见 [首页说明](guides/dashboard.md)。 |

| 2026-11-01 | 相册上传进度与预览 | 前端 / 测试 / 文档 | 逐文件展示真实上传百分比、处理中与结果；照片和列表封面可放大，照片支持切换与缩放；精简标题、字段和说明。14 项相册浏览器回归、生产构建与桌面及窄屏截图核对通过，独立代码检视问题已修复并复验，临时服务均已关闭。接口与数据库不变，生产图片及对象存储验收待部署后进行。行为约定见 [前端设计](guides/frontend-design.md)。 |

## 专项进度入口

- `docs/plans/2026-07-13-public-article-api-ddos-hardening.md`：公开文章接口防滥用加固计划。
- `docs/plans/2026-10-14-article-unpublish-republish.md`：文章下线与重新发布计划。
- `docs/plans/2026-10-18-article-private-status.md`：私有文章状态计划。
- `docs/plans/2026-10-21-login-email-mfa.md`：登录邮箱验证码（两步登录）计划，生产 SMTP 配置与 migration 验收未完成。
- 已完成并投产的计划归档在 `docs/archive/plans/`，仅作追溯，不作为当前进度入口。

## 维护要求

- Agent 每次完成修改后，必须在本文件追加或更新对应记录。
- 专项计划的复选框与状态在各自计划文档内维护，本文件只保留链接和整体结论。
- 本文件不记录被推翻或废弃的内容，相关内容统一写入 `docs/change-log.md`。
