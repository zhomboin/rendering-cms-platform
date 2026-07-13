# 可观测性

本文档记录 Rendering CMS Platform 后端日志约定。命令默认在 WSL Ubuntu 24.04 或 Linux 服务器中执行。

## 请求日志

后端使用 JSON 结构化日志记录每次 HTTP 请求，日志文件按自然日切分。Docker 后端容器内默认写入：

```text
/var/log/rendering-cms-platform/backend-YYYY-MM-DD.log
```

本地 Docker 开发环境通过 bind mount 将该容器目录挂载到宿主机目录，默认宿主机目录为：

```text
logs/backend/
```

生产 Docker 环境使用 Compose volume `rendering_cms_backend_logs` 挂载该目录；如需接入宿主机 logrotate 或外部采集系统，可在 Compose 层调整为宿主机目录挂载。

可通过环境变量调整容器内日志目录和宿主机挂载目录：

```bash
export LOG_DIR=/var/log/rendering-cms-platform
export BACKEND_LOG_HOST_DIR=/var/log/rendering-cms-platform
```

请求日志事件名为 `http_request`，当前记录字段包括：

- `method`：HTTP 方法。
- `path`：请求路径，不包含原始 query string。
- `status`：响应状态码。
- `bytes`：响应体字节数。
- `duration_ms`：请求处理耗时，单位为毫秒。
- `remote_ip_hash`：只从可信代理重写的 `X-Real-IP` 或 `RemoteAddr` 提取客户端地址后计算 SHA-256 哈希；不信任客户端可伪造的 `X-Forwarded-For` 链。
- `user_agent`：请求 `User-Agent`。

请求日志不记录原始 IP 地址、不记录请求体、不记录原始 query string，不在日志中写入密码、token、Cookie 或上传文件内容。

## 抗滥用监控

生产告警至少覆盖：

- Nginx `429`、应用 `429/503` 和后端 `5xx` 的速率与突增。
- Nginx `$upstream_cache_status` 中 `MISS`、`BYPASS`、`EXPIRED` 的比例；热点文章持续 MISS 时检查 cache key 和权限。
- Go goroutine、RSS、请求耗时和 PostgreSQL 连接池使用率。
- 搜索接口 P95、数据库慢查询和超时数量。
- CMS 发布成功但 Rendering Pagefind 构建触发失败的告警。

不得在日志或指标标签中记录原始 IP、Authorization、Cookie 或完整搜索词。

## 日志失败策略

日志写入不参与主业务链路控制。日志目录创建失败、日志文件打开失败或日志文件写入失败时，后端不会中断当前请求，也不会改变接口响应。

生产环境应保证运行用户对 `LOG_DIR` 目录具有写入权限。使用 Docker 部署时，应将宿主机系统盘上的日志目录挂载到容器内 `LOG_DIR`，并由 systemd、Docker、logrotate 或日志平台负责归档和采集。

## 验证

本地验证后端日志：

```bash
cd backend
go test ./...
```

启动后端后请求健康检查：

```bash
curl -fsS http://127.0.0.1:8080/api/v1/health
```

确认当天日志文件出现 `http_request` 记录：

```bash
tail -n 20 logs/backend/backend-$(date +%F).log
```
