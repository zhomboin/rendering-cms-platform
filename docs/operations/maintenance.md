# 运维速查（自动化部署）

本文档是日常运维的精简速查卡：该看什么、东西在哪、出问题先敲哪条命令。完整机制与流程见文末关联文档，本文不重复展开。

系统一句话：`cms-rendering-deploy.timer` 每小时整点触发 `deploy/auto-deploy.sh`，自动拉取代码、比较提交、按固定 SOP 发布（pull → build → 备份 → migration → 重启 → 健康检查），失败自动回滚代码。

## 每日检查（约 1 分钟）

```bash
curl -fsS http://127.0.0.1:3001/api/v1/health     # 期望 {"status":"ok"}
tail -20 /var/log/rendering-cms/latest.log        # 最近一次部署结果
cat /var/lib/rendering-cms/deploy-state.env       # 最近成功部署的提交
systemctl list-timers | grep cms-rendering-deploy # timer 在跑、下次触发在整点
df -h /var/lib/rendering-cms                      # 磁盘
```

判断标准：

- 健康检查返回 `ok`。
- `latest.log` 末尾是「部署成功」或「远端无新提交」，没有「部署失败」。
- `deploy-state.env` 里的 `LAST_SUCCESS_COMMIT` 落后 `origin/main` 最多一个检查周期（刚推送的新提交要等下一个整点，属正常）。
- 磁盘使用率明显低于 80%。

## 每周检查

```bash
ls -lht /var/lib/rendering-cms/backups/ | head -5                       # 备份在按次增长
gzip -t "$(ls -t /var/lib/rendering-cms/backups/rendering-cms-*.sql.gz | head -1)" && echo 备份完好
docker system df                                                        # 镜像/卷占用
grep -l "部署失败" /var/log/rendering-cms/deploy-*.log | wc -l          # 一周内的失败次数
```

- **把最新数据库备份复制到服务器之外**（scp/rsync 到本地电脑或对象存储）。自动备份只存在服务器本地，会随服务器一起丢失，异地保存是唯一需要人工坚持的备份环节。
- 若配置了 `NOTIFY_WEBHOOK`，确认通知渠道仍可达（改地址后发一条测试）。

## 每月 / 低频检查

- 导出 R2 对象清单，与数据库备份一起归档（命令见 `docs/operations/backup.md`）。
- 确认 Git deploy key 未失效：`journalctl -u cms-rendering-deploy.service | grep "git fetch 失败"` 无近期记录即正常。
- 登录后台抽查明细功能（文章、评论、文件、统计），验收清单见 `docs/operations/runbook.md`。
- TLS 证书与系统安全更新按服务器既有节奏处理，不在本系统范围内。

## 三个自动化脚本

| 脚本 | 作用 | 什么时候用 |
| --- | --- | --- |
| `deploy/auto-deploy.sh` | 部署主流程：锁 → fetch 比较 → pull → build → 备份 → migration → 重启 → 健康检查，失败自动回滚代码 | timer 每小时自动跑；改完 `/etc/rendering-cms` 配置后手动 `--force` 免提交重发 |
| `deploy/rollback.sh` | 手动回滚：代码退到上次成功提交，`--backup` 可连数据库一起恢复 | 发布出问题、需要退版本时 |
| `deploy/setup.sh` | 一次性安装：目录、配置迁移、systemd 单元、logrotate | 首次部署；目录/单元损坏后修复；`--user` 换运行用户 |

## 目录与文件地图

| 路径 | 内容 | 说明 |
| --- | --- | --- |
| `/opt/rendering-cms-platform` | 代码 + `deploy/` 脚本 | 不要在生产服务器上手改仓库内文件，脏工作区会阻止部署 |
| `/etc/rendering-cms/production.env` | 应用生产配置（数据库密码、JWT、R2 密钥） | 权限 `600`；修改后 `auto-deploy.sh --force` 生效 |
| `/etc/rendering-cms/deploy.env` | 部署脚本配置（分支、健康检查、备份保留数、webhook） | 全部可选；修改后下次触发自动生效 |
| `/var/log/rendering-cms/deploy-*.log`、`latest.log` | 每次部署与回滚的完整日志 | logrotate 每周轮转、保留 8 份；`latest.log` 始终指向最近一次 |
| `/var/lib/rendering-cms/backups/` | PostgreSQL 自动备份（`.sql.gz`） | 每次部署前生成，自动保留最近 14 份 |
| `/var/lib/rendering-cms/deploy-state.env` | 最近两次成功部署的提交号 | `rollback.sh` 的默认回滚目标 |
| `/var/lock/rendering-cms/deploy.lock` | 部署互斥锁 | 在 tmpfs 上，重启自动消失，无需清理 |
| `journalctl -u cms-rendering-deploy` | systemd 侧运行记录 | 内容与 `latest.log` 一致，适合 `--since` 查历史 |

应用自身的后端 JSON 日志在 Docker volume `rendering_cms_backend_logs` 内，查看方式见 `docs/operations/observability.md`。

## 异常第一反应

| 症状 | 先敲这条命令 |
| --- | --- |
| 收到部署失败通知 | `tail -50 /var/log/rendering-cms/latest.log`（失败阶段、是否已自动回滚都在里面） |
| 网站打不开 | `curl -fsS http://127.0.0.1:3001/api/v1/health`，不通则按 `runbook.md` 故障节查容器 |
| 远端有新提交但一直没部署 | `journalctl -u cms-rendering-deploy.service -n 50`（常见原因：fetch 凭据失效、工作区被改脏） |
| migration 后服务异常 | **不要盲目重启容器**，按 `docs/operations/restore.md` 评估；确认要回退时用 `rollback.sh --backup` |
| 磁盘持续上涨 | `docker system df`、`ls -lht /var/lib/rendering-cms/backups/`、确认 logrotate 在转日志 |

## 关联文档

- `docs/operations/auto-deploy.md`：自动部署机制、失败策略、回滚手册
- `docs/operations/runbook.md`：手动发布 SOP、应用故障排查
- `docs/operations/backup.md` / `docs/operations/restore.md`：备份要求与恢复流程
- `docs/operations/observability.md`：后端应用日志
