#!/usr/bin/env python3
"""隔离验证自动部署备份失败保护与原有代码回滚，不访问数据库或网络。"""

import ctypes
import gzip
import json
import os
from pathlib import Path
import shutil
import signal
import subprocess
import tempfile
import time
import unittest


ROOT = Path(__file__).resolve().parents[2]
TIMEOUT = 15
SQL = b"-- PostgreSQL dump\nSELECT 1;\n" * 10000


class AutoDeployBackupTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        # 接管异常退出时遗留的日志子进程，测试负责清理和回收。
        cls.libc = ctypes.CDLL(None, use_errno=True)
        cls.previous_subreaper = ctypes.c_int()
        if cls.libc.prctl(37, ctypes.byref(cls.previous_subreaper), 0, 0, 0):
            raise OSError(ctypes.get_errno(), "读取子进程回收配置失败")
        if cls.libc.prctl(36, 1, 0, 0, 0):
            raise OSError(ctypes.get_errno(), "设置子进程回收配置失败")

    @classmethod
    def tearDownClass(cls):
        cls.libc.prctl(36, cls.previous_subreaper.value, 0, 0, 0)

    def run_deploy(self, mode, *, rollback=False):
        (ROOT / "temp").mkdir(exist_ok=True)
        with tempfile.TemporaryDirectory(prefix="deploy-backup-", dir=ROOT / "temp") as tmp:
            base = Path(tmp)
            repo, tools = base / "repo", base / "bin"
            repo.mkdir()
            tools.mkdir()
            env = os.environ.copy()
            env.update({
                "DEPLOY_ENV_FILE": str(base / "absent.env"),
                "APP_DIR": str(repo), "GIT_REMOTE": "origin", "GIT_BRANCH": "main",
                "LOG_DIR": str(base / "logs"), "STATE_DIR": str(base / "state"),
                "BACKUP_DIR": str(base / "backups"), "LOCK_DIR": str(base / "lock"),
                "PRODUCTION_ENV_FILE": str(base / "production.env"),
                "COMPOSE_FILE": "deploy/isolated.yml", "NOTIFY_WEBHOOK": "",
                "AUTO_ROLLBACK": "true" if rollback else "false",
                "PRUNE_IMAGES": "false", "BACKUP_KEEP": "14",
                "HEALTH_RETRIES": "1", "HEALTH_INTERVAL": "0",
                "HEALTH_URL": "http://isolated.invalid/health",
                "TEST_BASE": str(base), "TEST_MODE": mode,
                "GIT_CONFIG_NOSYSTEM": "1", "GIT_CONFIG_GLOBAL": "/dev/null",
            })
            for tool in ("git", "gzip", "chmod"):
                real = shutil.which(tool)
                self.assertIsNotNone(real)
                env["TEST_REAL_" + tool.upper()] = real

            def git(*args):
                return subprocess.run(
                    [env["TEST_REAL_GIT"], "-C", str(repo), *args], env=env,
                    check=True, capture_output=True, text=True, timeout=TIMEOUT).stdout.strip()

            # 使用正常 Git 命令创建两个合法提交，不手写 .git 内容。
            git("init", "-q", "-b", "main")
            for message in ("运行中的提交", "待发布提交"):
                git("-c", "user.name=测试", "-c", "user.email=test@example.invalid",
                    "commit", "-q", "--allow-empty", "-m", message)
            target = git("rev-parse", "HEAD")
            previous = git("rev-parse", "HEAD^")
            git("remote", "add", "origin", str(repo))
            git("update-ref", "refs/remotes/origin/main", target)
            git("reset", "--hard", previous)
            env["TEST_TARGET"] = target
            (base / "production.env").touch()
            (base / "state").mkdir()
            state = base / "state/deploy-state.env"
            initial_state = "LAST_SUCCESS_COMMIT=" + previous + "\n"
            state.write_text(initial_state, encoding="utf-8")
            scripts = {
                "git": '''#!/usr/bin/env python3
import os
import sys
args = sys.argv[1:]
operation = args[2] if args[:1] == ["-C"] else args[0]
if operation == "fetch":
    sys.exit(0)
if operation == "pull":
    args = args[:2] + ["reset", "--hard", os.environ["TEST_TARGET"]]
os.execv(os.environ["TEST_REAL_GIT"], ["git", *args])
''',
                "docker": '''#!/usr/bin/env python3
import json
import os
from pathlib import Path
import subprocess
import sys
args = sys.argv[1:]
head = subprocess.run([os.environ["TEST_REAL_GIT"], "-C", os.environ["APP_DIR"],
                       "rev-parse", "HEAD"], check=True, capture_output=True,
                      text=True, timeout=5).stdout.strip()
with (Path(os.environ["TEST_BASE"]) / "docker-calls").open("a") as calls:
    calls.write(json.dumps([args, head]) + "\\n")
if args == ["compose", "version"]:
    sys.exit(0)
if args[:1] == ["inspect"]:
    print("true")
    sys.exit(0)
if args[:2] != ["compose", "--env-file"]:
    sys.exit(97)
operation = args[5:]
if operation == ["ps", "-q", "postgres"]:
    print("isolated-postgres")
elif operation[:1] == ["exec"]:
    mode = os.environ["TEST_MODE"]
    if mode == "export-failed":
        sys.exit(37)
    if mode != "empty":
        sys.stdout.buffer.write(b"-- PostgreSQL dump\\nSELECT 1;\\n" * 10000)
    if mode == "partial-export-failed":
        sys.exit(37)
elif operation not in (["build"], ["--profile", "migrate", "run", "--rm", "migrate"],
                        ["up", "-d", "backend", "frontend"], ["ps"]):
    sys.exit(97)
''',
                "gzip": '''#!/usr/bin/env python3
import os
import subprocess
import sys
mode = os.environ["TEST_MODE"]
if not sys.argv[1:]:
    if mode == "corrupt":
        sys.stdin.buffer.read()
        sys.stdout.buffer.write(b"corrupt backup")
        sys.exit(0)
    if mode == "compression-failed":
        # 即使压缩产物有效，压缩进程失败也必须阻止发布。
        subprocess.run([os.environ["TEST_REAL_GZIP"]], check=True, timeout=5)
        sys.exit(38)
os.execv(os.environ["TEST_REAL_GZIP"], ["gzip", *sys.argv[1:]])
''',
                "chmod": '''#!/usr/bin/env python3
import os
import sys
if os.environ["TEST_MODE"] == "chmod-failed":
    sys.exit(39)
os.execv(os.environ["TEST_REAL_CHMOD"], ["chmod", *sys.argv[1:]])
''',
                "curl": "#!/bin/sh\nexit 0\n",
            }
            for name, content in scripts.items():
                path = tools / name
                path.write_text(content, encoding="utf-8")
                path.chmod(0o755)
            env["PATH"] = str(tools) + os.pathsep + env["PATH"]
            with (base / "stdout").open("w+b") as output:
                process = subprocess.Popen(
                    ["bash", str(ROOT / "deploy/auto-deploy.sh")], env=env,
                    stdout=output, stderr=subprocess.STDOUT, start_new_session=True)
                try:
                    result = process.wait(timeout=TIMEOUT)
                finally:
                    try:
                        os.killpg(process.pid, signal.SIGKILL)
                    except ProcessLookupError:
                        pass
                    process.wait(timeout=TIMEOUT)
                    deadline = time.monotonic() + TIMEOUT
                    while True:
                        try:
                            child, _ = os.waitpid(-process.pid, os.WNOHANG)
                        except ChildProcessError:
                            break
                        if child == 0:
                            if time.monotonic() > deadline:
                                self.fail("测试子进程未能回收")
                            time.sleep(0.01)
                output.seek(0)
                stdout = output.read().decode("utf-8")
            with self.assertRaises(ProcessLookupError, msg="测试存在残留进程"):
                os.killpg(process.pid, 0)
            calls = [json.loads(line) for line in
                     (base / "docker-calls").read_text(encoding="utf-8").splitlines()]
            operations = [(args[5:], commit) for args, commit in calls
                          if args[:2] == ["compose", "--env-file"]]
            migration = [commit for args, commit in operations if args[:1] == ["--profile"]]
            starts = [commit for args, commit in operations if args[:1] == ["up"]]
            builds = [commit for args, commit in operations if args == ["build"]]
            backups = list((base / "backups").glob("rendering-cms-*.sql.gz"))
            self.assertEqual(len(backups), 1, stdout)
            if mode == "valid":
                self.assertEqual(result, 0, stdout)
                self.assertEqual(migration, [target], stdout)
                self.assertEqual(starts, [target], stdout)
                self.assertEqual(builds, [target], stdout)
                self.assertIn("LAST_SUCCESS_COMMIT=" + target, state.read_text())
                self.assertIn("PostgreSQL 备份完成", stdout)
                self.assertIn("==== 部署成功", stdout)
                self.assertEqual(gzip.decompress(backups[0].read_bytes()), SQL)
                self.assertEqual(backups[0].stat().st_mode & 0o777, 0o600)
            else:
                self.assertEqual(result, 1, stdout)
                self.assertIn("部署失败：数据库备份失败", stdout)
                self.assertNotIn("PostgreSQL 备份完成", stdout)
                self.assertNotIn("==== 部署成功", stdout)
                self.assertEqual(migration, [], stdout)
                self.assertEqual(state.read_text(), initial_state, stdout)
                self.assertEqual(starts, [previous] if rollback else [], stdout)
                self.assertEqual(builds, [target, previous] if rollback else [target], stdout)
                self.assertEqual(git("rev-parse", "HEAD"), previous if rollback else target)
                if rollback:
                    self.assertIn("代码回滚完成，服务已恢复到 " + previous, stdout)

    def test_export_failure_blocks_target_deployment(self):
        for mode in ("export-failed", "partial-export-failed"):
            with self.subTest(mode=mode):
                self.run_deploy(mode)

    def test_compression_failure_blocks_target_deployment(self):
        self.run_deploy("compression-failed")

    def test_empty_export_blocks_target_deployment(self):
        self.run_deploy("empty")

    def test_corrupt_gzip_blocks_target_deployment(self):
        self.run_deploy("corrupt")

    def test_permission_failure_blocks_target_deployment(self):
        self.run_deploy("chmod-failed")

    def test_nonempty_export_allows_target_deployment(self):
        self.run_deploy("valid")

    def test_backup_failure_preserves_existing_code_rollback(self):
        self.run_deploy("export-failed", rollback=True)


if __name__ == "__main__":
    unittest.main(verbosity=2)
