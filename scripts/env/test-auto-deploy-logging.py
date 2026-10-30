#!/usr/bin/env python3
"""在隔离仓库中验证部署脚本退出前排空双路日志，无需 Docker 或服务器。"""

import ctypes
import fcntl
import os
from pathlib import Path
import shutil
import signal
import subprocess
import tempfile
import time
import unittest


ROOT = Path(__file__).resolve().parents[2]
TIMEOUT = 10


class AutoDeployLoggingTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        # 接管被测进程提前退出后留下的日志子进程，清理时负责回收，避免僵尸。
        cls.libc = ctypes.CDLL(None, use_errno=True)
        cls.previous_subreaper = ctypes.c_int()
        if cls.libc.prctl(37, ctypes.byref(cls.previous_subreaper), 0, 0, 0):
            raise OSError(ctypes.get_errno(), "读取子进程回收配置失败")
        if cls.libc.prctl(36, 1, 0, 0, 0):
            raise OSError(ctypes.get_errno(), "设置子进程回收配置失败")

    @classmethod
    def tearDownClass(cls):
        cls.libc.prctl(36, cls.previous_subreaper.value, 0, 0, 0)

    def run_deploy(self, *, failure=False, locked=False):
        (ROOT / "temp").mkdir(exist_ok=True)
        with tempfile.TemporaryDirectory(prefix="deploy-log-", dir=ROOT / "temp") as tmp:
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
                "NOTIFY_WEBHOOK": "", "TEST_BASE": str(base),
                "TEST_FAIL_HEAD": "1" if failure else "0",
                "GIT_CONFIG_NOSYSTEM": "1", "GIT_CONFIG_GLOBAL": "/dev/null",
            })
            git = shutil.which("git")
            self.assertIsNotNone(git)
            env["TEST_REAL_GIT"] = git
            for args in (["init", "-q", "-b", "main"],
                         ["-c", "user.name=测试", "-c", "user.email=test@example.invalid",
                          "commit", "-q", "--allow-empty", "-m", "测试基线"],
                         ["remote", "add", "origin", str(repo)]):
                subprocess.run([git, "-C", str(repo), *args], env=env,
                               check=True, capture_output=True, timeout=TIMEOUT)
            (base / "production.env").touch()
            scripts = {
                "tee": '''#!/usr/bin/env python3
import os
from pathlib import Path
import sys
import time
base = Path(os.environ["TEST_BASE"])
data = sys.stdin.buffer.read()
(base / "ready").write_text(str(os.getpid()))
end = time.monotonic() + 8
while not (base / "release").exists():
    if time.monotonic() > end:
        sys.exit(98)
    time.sleep(0.01)
with open(sys.argv[-1], "ab") as log:
    log.write(data)
sys.stdout.buffer.write(data)
sys.stdout.buffer.flush()
''',
                "git": '''#!/usr/bin/env python3
import os
import sys
if os.environ["TEST_FAIL_HEAD"] == "1" and sys.argv[-2:] == ["rev-parse", "HEAD"]:
    sys.exit(37)
os.execv(os.environ["TEST_REAL_GIT"], ["git", *sys.argv[1:]])
''',
                "docker": '''#!/usr/bin/env python3
import os
from pathlib import Path
import sys
with (Path(os.environ["TEST_BASE"]) / "docker-calls").open("a") as calls:
    calls.write(" ".join(sys.argv[1:]) + "\\n")
sys.exit(0 if sys.argv[1:] == ["compose", "version"] else 97)
''',
                "curl": "#!/bin/sh\nexit 97\n",
            }
            for name, content in scripts.items():
                target = tools / name
                target.write_text(content, encoding="utf-8")
                target.chmod(0o755)
            env["PATH"] = str(tools) + os.pathsep + env["PATH"]
            (base / "lock").mkdir()
            with (base / "lock/deploy.lock").open("w") as lock, \
                    (base / "stdout").open("w+b") as output:
                if locked:
                    fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
                process = subprocess.Popen(
                    ["bash", str(ROOT / "deploy/auto-deploy.sh")], env=env,
                    stdout=output, stderr=subprocess.STDOUT, start_new_session=True)
                try:
                    deadline = time.monotonic() + TIMEOUT
                    while not (base / "ready").exists():
                        if time.monotonic() > deadline:
                            self.fail("日志消费者没有读到 EOF：可能未关闭日志写端")
                        time.sleep(0.01)
                    # 消费者已完整读取但尚未输出。主进程若提前退出，模拟 systemd 清理组。
                    try:
                        process.wait(timeout=0.3)
                    except subprocess.TimeoutExpired:
                        (base / "release").touch()
                    else:
                        os.killpg(process.pid, signal.SIGKILL)
                    result = process.wait(timeout=TIMEOUT)
                finally:
                    # 无论断言或超时，整个进程组都要结束，并回收已接管的孤儿。
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
                                self.fail("日志子进程未能回收")
                            time.sleep(0.01)
                output.seek(0)
                stdout = output.read().decode("utf-8")
            with self.assertRaises(ProcessLookupError, msg="被测进程组存在残留"):
                os.killpg(process.pid, 0)
            logfile = base / "logs/deploy.log"
            file_output = logfile.read_text(encoding="utf-8") if logfile.exists() else ""
            self.assertEqual(result, 37 if failure else 0, "必须保留部署主进程退出码")
            expected = "意外错误：脚本第" if failure else (
                "本次跳过。" if locked else "本次不部署。")
            self.assertIn(expected, stdout, "标准输出丢失最终日志")
            self.assertIn(expected, file_output, "日志文件丢失最终日志")
            self.assertEqual(stdout, file_output, "标准输出和日志文件应一致")
            calls = base / "docker-calls"
            self.assertEqual(calls.read_text() if calls.exists() else "",
                             "" if locked else "compose version\n")
            self.assertEqual(list((base / "backups").iterdir()), [])
            self.assertFalse((base / "state/deploy-state.env").exists())

    def test_no_change_drains_final_log(self):
        self.run_deploy()

    def test_unexpected_error_drains_log_and_preserves_status(self):
        self.run_deploy(failure=True)

    def test_lock_contention_drains_skip_log(self):
        self.run_deploy(locked=True)


if __name__ == "__main__":
    unittest.main(verbosity=2)
