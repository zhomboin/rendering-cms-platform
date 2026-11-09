#!/usr/bin/env python3
"""隔离验证恢复失败会阻止应用启动和成功状态写入，不连接真实数据库。"""

import gzip
import json
import os
from pathlib import Path
import signal
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]


class RollbackSafetyTest(unittest.TestCase):
    def run_restore(self, compressed, fail):
        with tempfile.TemporaryDirectory(prefix="restore-test-", dir=ROOT / "temp") as temp:
            base = Path(temp)
            repo, tools = base / "repo", base / "bin"
            repo.mkdir()
            tools.mkdir()
            subprocess.run(["git", "init", "-q", str(repo)], check=True, capture_output=True)
            (base / "production.env").touch()
            (base / "state").mkdir()
            state = base / "state/deploy-state.env"
            initial = "LAST_SUCCESS_COMMIT=" + "a" * 40 + "\n"
            state.write_text(initial)
            backup = base / ("backup.sql.gz" if compressed else "backup.sql")
            data = b"select 1;\nselect invalid_sql;\n" if fail else b"select 1;\n"
            backup.write_bytes(gzip.compress(data) if compressed else data)
            scripts = {
                "git": """#!/usr/bin/env python3
import sys
print("b" * 40 if "HEAD" not in sys.argv else "a" * 40)
""",
                "docker": """#!/usr/bin/env python3
import json
import os
from pathlib import Path
import sys
args = sys.argv[1:]
with (Path(os.environ["TEST_BASE"]) / "calls").open("a") as log:
    log.write(json.dumps(args) + "\\n")
if "exec" in args and "psql" in args[-1] and " -c " not in args[-1]:
    sql = sys.stdin.buffer.read()
    if b"invalid_sql" in sql and "ON_ERROR_STOP=1" in args[-1]:
        sys.exit(3)
sys.exit(0)
""",
                "systemctl": "#!/bin/sh\nexit 1\n",
                "curl": "#!/bin/sh\nexit 0\n",
            }
            for name, content in scripts.items():
                path = tools / name
                path.write_text(content)
                path.chmod(0o755)
            env = os.environ.copy()
            env.update(
                {
                    "PATH": str(tools) + os.pathsep + env["PATH"],
                    "DEPLOY_ENV_FILE": str(base / "absent.env"),
                    "APP_DIR": str(repo),
                    "LOG_DIR": str(base / "logs"),
                    "STATE_DIR": str(base / "state"),
                    "BACKUP_DIR": str(base / "backups"),
                    "LOCK_DIR": str(base / "lock"),
                    "PRODUCTION_ENV_FILE": str(base / "production.env"),
                    "TEST_BASE": str(base),
                    "HEALTH_RETRIES": "1",
                    "HEALTH_INTERVAL": "0",
                }
            )
            process = subprocess.Popen(
                [
                    "bash",
                    str(ROOT / "deploy/rollback.sh"),
                    "--commit",
                    "b" * 40,
                    "--backup",
                    str(backup),
                    "--yes",
                ],
                env=env,
                stdout=subprocess.PIPE,
                stderr=subprocess.STDOUT,
                start_new_session=True,
            )
            try:
                output, _ = process.communicate(timeout=10)
            finally:
                try:
                    os.killpg(process.pid, signal.SIGTERM)
                except ProcessLookupError:
                    pass
                process.wait(timeout=10)
            calls = [json.loads(line) for line in (base / "calls").read_text().splitlines()]
            commands = [args[5:] for args in calls]
            if fail:
                self.assertNotEqual(process.returncode, 0, output.decode())
                self.assertNotIn(["up", "-d", "backend", "frontend"], commands)
                self.assertNotIn(["build"], commands)
                self.assertEqual(state.read_text(), initial)
                self.assertNotIn("数据库恢复完成", output.decode())
            else:
                self.assertEqual(process.returncode, 0, output.decode())
                self.assertIn(["up", "-d", "backend", "frontend"], commands)
                self.assertIn("LAST_SUCCESS_COMMIT=" + "b" * 40, state.read_text())
                restores = [
                    args[-1] for args in calls if "psql" in args[-1] and " -c " not in args[-1]
                ]
                self.assertEqual(len(restores), 1)
                self.assertIn("ON_ERROR_STOP=1", restores[0])
                self.assertIn("--single-transaction", restores[0])

    def test_sql_error_blocks_start_and_success_state(self):
        for compressed in (False, True):
            with self.subTest(compressed=compressed):
                self.run_restore(compressed, True)

    def test_valid_restore_uses_atomic_fail_fast_import(self):
        for compressed in (False, True):
            with self.subTest(compressed=compressed):
                self.run_restore(compressed, False)


if __name__ == "__main__":
    unittest.main(verbosity=2)
