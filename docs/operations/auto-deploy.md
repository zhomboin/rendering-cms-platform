# 自动部署（systemd timer）

本文档描述 Rendering CMS Platform 在生产服务器上的自动部署机制：定时拉取代码、对比远端提交、有更新时自动执行完整发布流程，失败时自动回滚代码。手动发布基线流程见 `docs/operations/deployment.md`，数据库恢复细节见 `docs/operations/restore.md`。

## 触发机制

```text
cms-rendering-deploy.timer（每小时整点，OnCalendar=hourly）
  -> cms-rendering-deploy.service（systemd oneshot）
    -> deploy/auto-deploy.sh
      1. flock 抢 /var/lock/rendering-cms/deploy.lock，抢不到直接退出
      2. git fetch origin main，比较 HEAD 与 origin/main
      3. 无新提交 -> 记录日志退出，不做任何变更
      4. 有新提交 -> 按固定 SOP 发布：
         git pull --ff-only -> docker compose build -> PostgreSQL 备份
         -> migration -> up -d backend frontend -> 健康检查
      5. 任一阶段失败 -> 自动回滚代码到上一成功提交并重建旧版本
```

发布顺序与 `docs/operations/deployment.md` 的「发布更新」完全一致，自动化只是替代了手工执行。

## 目录布局

| 路径 | 用途 | 权限 |
| --- | --- | --- |
| `/opt/rendering-cms-platform` | 应用代码（Git 仓库） | 仓库属主 |
| `/etc/rendering-cms/production.env` | 生产环境变量（含密钥） | `0600` |
| `/etc/rendering-cms/deploy.env` | 自动部署配置（可选） | `0644` |
| `/var/log/rendering-cms/` | 部署日志 `deploy.log`、回滚日志 `rollback.log`（追加写入，logrotate 轮转） | `0750` |
| `/var/lib/rendering-cms/backups/` | PostgreSQL 备份（migration 前自动生成） | `0700` |
| `/var/lib/rendering-cms/deploy-state.env` | 最近两次成功部署的提交记录 | - |
| `/var/lock/rendering-cms/deploy.lock` | 部署互斥锁（flock） | `0755` |

`/var/lock` 在 Ubuntu 上是 `/run/lock` 的符号链接（tmpfs），锁目录随重启消失，脚本每次运行会自动重建。

## 首次启用

前置条件：

- 已按 `docs/operations/deployment.md` 完成首次部署（容器可正常启动，健康检查通过）。
- 服务器 root（或仓库属主）能访问 Git 远端：私有仓库需配置 SSH deploy key。

在服务器上执行：

```bash
cd /opt/rendering-cms-platform
git pull
sudo bash deploy/setup.sh
```

`setup.sh` 会创建目录、迁移 `deploy/production.env` 到 `/etc/rendering-cms/production.env`、生成 systemd 单元（直接写入服务器 `/etc/systemd/system/`，仓库内不保存单元文件）与 logrotate，并在 `production.env` 不含占位值时自动启用 timer。

`production.env` 仍是占位值时，setup 不会启用 timer；填好真实配置后手动启用：

```bash
sudo systemctl enable --now cms-rendering-deploy.timer
```

## 配置

`/etc/rendering-cms/deploy.env` 可覆盖 `deploy/auto-deploy.env.example` 中列出的全部默认值，常用项：

- `GIT_BRANCH`：跟踪分支，默认 `main`。
- `HEALTH_URL`：健康检查地址，默认 `http://127.0.0.1:3001/api/v1/health`；修改 `PUBLIC_HTTP_BIND` 时需同步调整。
- `BACKUP_KEEP`：备份保留个数，默认 `14`；自动生成的 `.sql.gz` 与手动放入的 `.sql` 按修改时间一起排序，超出部分在成功部署后清理，非法值回退为 `14`。
- `AUTO_ROLLBACK`：部署失败时是否自动回滚代码，默认 `true`。
- `NOTIFY_WEBHOOK`：可选 webhook（接收 `{"text": "..."}`），部署成功/失败时通知。

修改后无需重启 timer，下次触发自动生效。

## 日常运维

```bash
# 手动触发一次部署（与 timer 共用锁，不会并发）
sudo systemctl start cms-rendering-deploy.service

# 强制重新部署当前提交（改完 /etc/rendering-cms 配置后常用）
sudo /opt/rendering-cms-platform/deploy/auto-deploy.sh --force

# 查看部署输出
journalctl -u cms-rendering-deploy.service -f

# 查看部署日志文件
tail -f /var/log/rendering-cms/deploy.log

# 查看 timer 计划
systemctl list-timers | grep cms-rendering-deploy

# 发布窗口暂停/恢复自动部署
sudo systemctl stop cms-rendering-deploy.timer
sudo systemctl start cms-rendering-deploy.timer

# 查看最近成功部署的提交
cat /var/lib/rendering-cms/deploy-state.env
```

## 日志完整性验证

部署脚本在退出前关闭日志管道写端，并等待日志同时写入标准输出和 `deploy.log`，随后按原始退出码结束。无新提交、锁冲突跳过和意外错误路径都使用相同的退出处理；systemd 的进程清理策略保持不变。

本地验证入口为 `scripts/env/test-auto-deploy-logging.py`，使用 Python 3 运行。测试在 `temp/` 内创建隔离 Git 仓库，模拟日志消费者延迟和服务进程组清理，检查末行、双路日志一致性、退出码及子进程回收；不连接生产，也不启动容器。

## 备份失败保护

PostgreSQL 运行时，导出和压缩命令必须成功，备份须通过 gzip 校验、解压内容非空且文件权限设置成功，才会继续目标提交的 migration 和容器更新。导出失败、部分导出后失败、压缩失败或空输出均会阻断目标发布，不写入成功部署状态。

备份失败仍遵循原有 `AUTO_ROLLBACK` 配置：关闭时直接结束；开启时回退代码、重建并启动旧提交，再检查健康状态。备份时机、目录、文件命名和保留数量不变。

本地验证入口为 `scripts/env/test-auto-deploy-backup.py`，使用 Python 3 运行。测试在 `temp/` 内创建隔离 Git 仓库，用外部工具替身执行真实部署主流程，覆盖导出、压缩、空内容、损坏文件、权限失败、正常备份与代码回滚，检查目标发布阻断、成功状态保护和子进程回收；不连接数据库或网络。生产已应用修复，普通自动检查通过，日志文件与 journal 均保留完整结束信息。导出失败保护由隔离测试覆盖，未在生产注入导出故障或执行恢复演练。

## 失败处理

`auto-deploy.sh` 对不同失败阶段的处理策略：

| 失败阶段 | 自动处理 | 原因 |
| --- | --- | --- |
| 预检查 / `git fetch` / 工作区脏 | 直接退出，不做任何变更 | 未触碰运行中的服务 |
| `git pull` / `build` / 备份失败 | 回滚代码到上一成功提交，重建旧镜像并重启 | 数据库未变更，回滚安全 |
| migration 失败 | 只回退代码，不重启容器 | 数据库可能 dirty，盲目重启会放大故障；按 `docs/operations/restore.md` 人工处理 |
| `up -d` / 健康检查失败 | 回滚代码到上一成功提交，重建旧镜像并重启 | 应用级问题，按 SOP 回退 |

注意：

- 自动回滚只回退应用代码与镜像，绝不自动回滚数据库；migration 后的数据库问题必须人工评估。
- 本地工作区有未提交改动时部署会中止，这是保护机制；生产服务器不要直接改仓库内文件。
- 若回滚后健康检查仍失败，服务可能处于不可用状态，配置了 `NOTIFY_WEBHOOK` 会收到告警，需立即人工介入。

## 手动回滚

```bash
# 回滚代码到上一次成功部署的提交
sudo /opt/rendering-cms-platform/deploy/rollback.sh

# 回滚到指定提交
sudo /opt/rendering-cms-platform/deploy/rollback.sh --commit <sha>

# 同时恢复最近一个数据库备份（DROP + CREATE + 导入，会覆盖当前数据）
sudo /opt/rendering-cms-platform/deploy/rollback.sh --backup latest

# 跳过确认（脚本化场景）
sudo /opt/rendering-cms-platform/deploy/rollback.sh --yes
```

回滚脚本与自动部署共用锁文件，回滚期间定时触发会自动跳过。数据库恢复遵循 `docs/operations/restore.md`：先停 backend、终止残留连接、重建空库再导入。

回滚完成后建议暂停 timer，确认服务稳定、远端问题修复后再恢复，避免新提交立刻再次部署。

## 服务运行用户

默认 `cms-rendering-deploy.service` 以 root 运行：Docker、`/etc`、`/var` 写入开箱即用；脚本中的 Git 操作会自动降权为 `/opt/rendering-cms-platform` 属主执行（`runuser`），不会在仓库留下 root 属主文件。

如需最小权限运行：

```bash
sudo useradd -m -s /usr/sbin/nologin rendering-deploy 2>/dev/null || true
sudo usermod -aG docker rendering-deploy
sudo chown -R rendering-deploy:rendering-deploy /opt/rendering-cms-platform
sudo bash deploy/setup.sh --user rendering-deploy
```

注意：`docker` 组等价于 root 权限，该方案主要价值是隔离文件属主与 Git 凭据，不是安全边界。

## 与手动发布的关系

- 自动部署覆盖「发布更新」章节的全部步骤，是生产默认发布方式。
- 手动流程（`docs/operations/deployment.md`）仍是基线 SOP：排查故障、首次部署、执行特殊操作（如更换对象存储、恢复数据库）时使用手动流程，并先 `sudo systemctl stop cms-rendering-deploy.timer` 暂停自动部署。
- migration 相关安全规则不变：生产 migration 前自动备份 PostgreSQL，恢复与回滚决策见 `docs/operations/restore.md`。
