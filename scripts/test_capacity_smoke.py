import json
import http.server
import socket
import subprocess
import sys
import threading


def test_capacity_smoke_rejects_invalid_arguments():
    result = subprocess.run(
        [sys.executable, "scripts/capacity_smoke.py", "--clients", "0"],
        capture_output=True,
        text=True,
        check=False,
    )
    assert result.returncode == 2


def test_capacity_smoke_emits_machine_readable_result():
    result = subprocess.run(
        [sys.executable, "scripts/capacity_smoke.py", "--url", "http://127.0.0.1:1", "--requests", "1"],
        capture_output=True,
        text=True,
        check=False,
    )
    assert result.returncode == 1
    report = json.loads(result.stdout)
    assert report["errors"] == 1
    assert report["error_rate"] == 1.0


def test_capacity_smoke_fails_when_history_surface_exceeds_error_budget():
    class Handler(http.server.BaseHTTPRequestHandler):
        def do_GET(self):
            self.send_response(500 if self.path == "/history" else 200)
            self.end_headers()

        def log_message(self, *_args):
            return

    with socket.socket() as probe:
        probe.bind(("127.0.0.1", 0))
        port = probe.getsockname()[1]
    server = http.server.ThreadingHTTPServer(("127.0.0.1", port), Handler)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    try:
        result = subprocess.run(
            [
                sys.executable,
                "scripts/capacity_smoke.py",
                "--url",
                f"http://127.0.0.1:{port}/health",
                "--history-url",
                f"http://127.0.0.1:{port}/history",
                "--latest-url",
                f"http://127.0.0.1:{port}/latest",
                "--requests",
                "2",
            ],
            capture_output=True,
            text=True,
            check=False,
        )
    finally:
        server.shutdown()
        thread.join()
    assert result.returncode == 1
    report = json.loads(result.stdout)
    assert report["surface"]["health"]["error_rate"] == 0.0
    assert report["surface"]["history"]["error_rate"] == 1.0
