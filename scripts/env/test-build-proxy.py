#!/usr/bin/env python3
"""验证构建代理只作用于构建进程，正常与回退共用同一入口。"""

import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[2]
PROXY_KEYS = (
    "HTTP_PROXY",
    "HTTPS_PROXY",
    "http_proxy",
    "https_proxy",
    "NO_PROXY",
    "no_proxy",
    "CMS_BUILD_PROXY",
    "CMS_BUILD_NO_PROXY",
    "COMPOSE_PARALLEL_LIMIT",
    "BUILDX_NO_DEFAULT_ATTESTATIONS",
    "COMPOSE_BAKE",
    "DOCKER_BUILDKIT",
)


class BuildProxyTest(unittest.TestCase):
    def invoke(self, *, proxy="", overlay=True, failure=False):
        (ROOT / "temp").mkdir(exist_ok=True)
        with tempfile.TemporaryDirectory(dir=ROOT / "temp") as directory:
            base = Path(directory)
            trace = base / "trace.py"
            trace.write_text(
                "import json, os, sys\n"
                + "keys = "
                + repr(PROXY_KEYS)
                + "\n"
                + "print(json.dumps({'args': sys.argv[1:], "
                + "'env': {k: os.environ.get(k) for k in keys}}))\n"
                + "sys.exit(19 if os.environ.get('FAIL') == '1' and "
                + "sys.argv[1:] != ['after'] else 0)\n",
                encoding="utf-8",
            )
            model = base / "proxy.yml"
            if overlay:
                model.write_text("services: {}\n", encoding="utf-8")
            env = {k: v for k, v in os.environ.items() if k not in PROXY_KEYS}
            env.update(
                TRACE=str(trace),
                HELPER=str(ROOT / "deploy/build-images.sh"),
                BUILD_PROXY_URL=proxy,
                BUILD_PROXY_COMPOSE_FILE=str(model),
                PRODUCTION_ENV_FILE=str(base / "production.env"),
                COMPOSE_FILE=str(base / "production.yml"),
                FAIL="1" if failure else "0",
            )
            script = """
compose() { python3 "$TRACE" "$@"; }
log() { printf '%s\n' "$*" >&2; }
source "$HELPER"
compose_build
status=$?
python3 "$TRACE" after
exit "$status"
"""
            result = subprocess.run(
                ["bash", "-c", script],
                env=env,
                text=True,
                capture_output=True,
                timeout=10,
            )
            return result, [json.loads(line) for line in result.stdout.splitlines()]

    def test_direct_build_and_no_environment_leak(self):
        result, records = self.invoke()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(records[0]["args"], ["build"])
        self.assertEqual(records[0]["env"]["COMPOSE_PARALLEL_LIMIT"], "1")
        self.assertTrue(all(value is None for value in records[-1]["env"].values()))

    def test_proxy_covers_client_and_run_without_false_attestation_flags(self):
        result, records = self.invoke(proxy="http://127.0.0.1:12345")
        self.assertEqual(result.returncode, 0, result.stderr)
        build = records[0]
        self.assertEqual(build["args"][0], "-f")
        self.assertEqual(build["args"][-1], "build")
        self.assertEqual(build["env"]["HTTP_PROXY"], "http://127.0.0.1:12345")
        self.assertEqual(build["env"]["http_proxy"], build["env"]["HTTP_PROXY"])
        self.assertEqual(build["env"]["CMS_BUILD_PROXY"], build["env"]["HTTP_PROXY"])
        self.assertEqual(build["env"]["BUILDX_NO_DEFAULT_ATTESTATIONS"], "1")
        self.assertEqual(build["env"]["COMPOSE_BAKE"], "false")
        self.assertTrue(all(value is None for value in records[-1]["env"].values()))
        self.assertNotIn("--sbom=false", build["args"])
        self.assertNotIn("--provenance=false", build["args"])

    def test_missing_overlay_blocks_build(self):
        result, records = self.invoke(proxy="http://127.0.0.1:12345", overlay=False)
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual([item["args"] for item in records], [["after"]])

    def test_invalid_proxy_blocks_build(self):
        result, records = self.invoke(proxy="socks5://127.0.0.1:12345")
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual([item["args"] for item in records], [["after"]])

    def test_failure_exit_is_preserved(self):
        result, _ = self.invoke(proxy="http://127.0.0.1:12345", failure=True)
        self.assertEqual(result.returncode, 19)

    def test_both_publish_and_rollback_use_helper(self):
        for name in ("auto-deploy.sh", "rollback.sh"):
            source = (ROOT / "deploy" / name).read_text()
            self.assertIn('/build-images.sh"', source)
            self.assertIn("compose_build", source)
            self.assertNotIn("\ncompose build", source)


if __name__ == "__main__":
    unittest.main(verbosity=2)
