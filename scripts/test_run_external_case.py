import json
import os
import stat
import subprocess
import sys


def test_runner_marks_failed_integration_and_returns_nonzero(tmp_path):
    fake_make = tmp_path / "make"
    fake_make.write_text("#!/bin/sh\n[ \"$1\" = integration ] && exit 7\nexit 0\n")
    fake_make.chmod(fake_make.stat().st_mode | stat.S_IXUSR)
    environment = os.environ.copy()
    environment["IOLINK_TEST_PG_DSN"] = "postgres://isolated"
    environment["PATH"] = f"{tmp_path}:{environment['PATH']}"
    result = subprocess.run(
        [sys.executable, "scripts/run_external_case.py", "--case", "R28-R33", "--provider", "reference", "--mode", "software"],
        capture_output=True,
        text=True,
        env=environment,
        check=False,
    )
    assert result.returncode == 7
    report = json.loads(result.stdout)
    assert report["software"] == "failed"
    assert report["integration"] == "failed"
