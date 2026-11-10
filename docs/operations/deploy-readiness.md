# 部署收尾与启用条件

迁移及自动部署只维护一份收尾清单。源码、隔离测试、CI、真实生产验收分别记录，生产修改、发布及定时器启用按用户确认的完整命令执行。具体机器与固定哈希只写入被忽略的本地审批材料。

## 必须逐项关闭

| 项目 | 完成条件 |
| --- | --- |
| 代码交付 | 各仓库独立 PR 完成检视与 CI；不夹带未批准功能或 migration |
| 部署互斥 | 抢锁失败和持有者退出都不删除锁路径；并发回归通过 |
| 构建出口 | daemon、Compose 客户端和 RUN 分别验证；正常构建和回退共用出口 |
| 数据保护 | 数据库停止、缺失、检查失败或备份失败均阻断发布；生产 migration 前有最新校验备份 |
| 镜像标识 | 按提交使用独立标签，成功状态记录标签；上一可用镜像和初始迁移镜像保留 |
| 完整发布 | 两套正式入口依次完成依赖、构建、启动、健康与发布状态检查 |
| 回退准备 | 保存代码、配置、数据库快照及可用产物；核对回退命令和数据兼容性 |
| 定时器 | 记录实际状态；明确要求关闭时保持关闭，启用须另获明确授权 |
| 收尾 | 临时进程和检查容器归零，证据完整，文档与当前部署一致 |

## 可选构建代理

配置入口是 deploy/auto-deploy.env.example，实际值位于服务器外部配置文件，不提交凭据。代理启用时，BUILD_PROXY_URL 与 BUILD_PROXY_COMPOSE_FILE 必须同时有效；后者安装自 deploy/docker-compose.build-proxy.yml，放在仓库外，确保回退到不包含此文件的提交时仍可使用。

deploy/build-images.sh 为一次构建设置客户端大小写 HTTP/HTTPS 代理、NO_PROXY、RUN 的预定义代理参数和 host 构建网络。运行容器不加载该覆盖文件，不注入代理，也不改为 host 网络。没有配置代理时使用原生产 Compose。

构建采用 COMPOSE_PARALLEL_LIMIT=1，使用现有 Compose 内置 BuildKit，不传两个 false attestation 参数；BUILDX_NO_DEFAULT_ATTESTATIONS=1 仅控制默认 provenance，不代表全部证明审计。主机资源和网络条件在真实发布前检查。

## 标签与回退

自动发布及手动源码回退的应用镜像标签为 commit-完整提交号，成功时写入 deploy-state.env 的 LAST_IMAGE_TAG。手动执行 Compose 时应显式传入已批准的 APP_IMAGE_TAG，不能依赖与实际运行不一致的旧环境值。

默认不执行全局 docker image prune。启用 PRUNE_IMAGES 或调整备份保留数量属于独立策略修改，必须获得授权。数据库备份策略本身不在此变更中调整。

自动源码回退仍需可用构建出口；首次受控发布额外保留旧容器镜像 ID，并准备使用 --no-build 与 --pull never 的固定镜像回退命令。应用回退不会恢复数据库，migration 失败应停止并按 [恢复说明](restore.md) 处理，禁止自动执行 down 或覆盖当前数据。

## 本地检查与环境证据

在 WSL Ubuntu 的仓库根目录执行以下隔离测试，不访问生产或网络：

```bash
bash -n deploy/auto-deploy.sh deploy/rollback.sh deploy/build-images.sh
python3 scripts/env/test-auto-deploy-backup.py
python3 scripts/env/test-auto-deploy-logging.py
python3 scripts/env/test-build-proxy.py
```

全量测试由 GitHub CI 执行。生产验收必须使用真实部署提交，记录健康、备份哈希、镜像 ID、状态文件、定时器和临时资源清理结果。被用户明确排除的对象备份、图片读写等不纳入本批执行，也不得虚报通过。
