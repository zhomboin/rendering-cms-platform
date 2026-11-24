"""向 CMS 发送标准 MCP 工具发现请求。无参数时使用当前 Codex 配置里的 cms-mcp。"""

import argparse
import getpass
import json
import os
from pathlib import Path
import sys
import tomllib
from urllib.error import HTTPError, URLError
from urllib.parse import urlsplit
from urllib.request import build_opener, HTTPRedirectHandler, Request


class NoRedirect(HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        return None


def connection(endpoint):
    if endpoint:
        token = os.getenv("CMS_MCP_TOKEN")
    else:
        root = Path(os.environ.get("CODEX_HOME") or Path.home() / ".codex")
        with (root / "config.toml").open("rb") as config:
            server = tomllib.load(config)["mcp_servers"]["cms-mcp"]
        if not server.get("enabled", True):
            raise ValueError("Codex 配置中的 cms-mcp 已关闭")
        endpoint = server["url"]
        token = os.environ.get(server.get("bearer_token_env_var", "")) or os.getenv("CMS_MCP_TOKEN")
    url = urlsplit(endpoint)
    local = url.hostname in ("localhost", "127.0.0.1", "::1")
    if not url.hostname or url.username is not None or url.fragment or not (
        url.scheme == "https" or local and url.scheme == "http"
    ):
        raise ValueError("请使用 CMS 的 HTTPS 接入地址，本机开发可使用 HTTP")
    if not token:
        token = getpass.getpass(f"{url.netloc} 的连接令牌：").strip()
    if not token:
        raise ValueError("需要提供连接令牌")
    if not token.isascii() or not token.isprintable() or any(ch.isspace() for ch in token):
        raise ValueError("连接令牌包含无效字符")
    return endpoint, token


def list_tools(endpoint, token):
    http = build_opener(NoRedirect)
    headers = {"Authorization": "Bearer " + token, "Content-Type": "application/json",
               "Accept": "application/json, text/event-stream"}

    def send(method, params, request_id=None):
        payload = {"jsonrpc": "2.0", "method": method, "params": params}
        if request_id is not None:
            payload["id"] = request_id
        request = Request(endpoint, data=json.dumps(payload).encode(), headers=headers)
        with http.open(request, timeout=15) as response:
            if method == "initialize" and response.headers.get("Mcp-Session-Id"):
                headers["Mcp-Session-Id"] = response.headers["Mcp-Session-Id"]
            if request_id is None:
                return
            reply = json.load(response)
        if reply.get("id") != request_id or "result" not in reply:
            raise ValueError(f"{method} 调用失败")
        return reply["result"]

    result = send("initialize", {"protocolVersion": "2025-11-25", "capabilities": {},
                                 "clientInfo": {"name": "cms-tools-list", "version": "1"}}, 1)
    headers["MCP-Protocol-Version"] = result["protocolVersion"]
    send("notifications/initialized", {})
    names = []
    params = {}
    seen = set()
    while True:
        page = send("tools/list", params, 2 + len(seen))
        names.extend(tool["name"] for tool in page["tools"])
        cursor = page.get("nextCursor")
        if not cursor:
            return sorted(names)
        if cursor in seen:
            raise ValueError("服务器重复返回同一分页位置")
        seen.add(cursor)
        params = {"cursor": cursor}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("endpoint", nargs="?", help="CMS 的 MCP 接入地址")
    args = parser.parse_args()
    try:
        names = list_tools(*connection(args.endpoint))
    except HTTPError as err:
        sys.exit(f"CMS 请求失败（HTTP {err.code}）")
    except (OSError, URLError, KeyError, ValueError, EOFError) as err:
        sys.exit(f"无法查询工具：{err}")
    print(f"当前可用工具（{len(names)} 项）")
    for name in names:
        print(name)


if __name__ == "__main__":
    main()
