#!/usr/bin/env python3

import json
import subprocess
import sys
import tempfile
from pathlib import Path


def main() -> None:
    script = Path(__file__).with_name("write_run_metadata.py")
    with tempfile.TemporaryDirectory() as temporary:
        output = Path(temporary) / "run.json"
        command = [
            sys.executable,
            str(script),
            "--output",
            str(output),
            "--service",
            "java",
            "--workload",
            "/downstream?delay=50",
            "--target",
            "http://java:8080/downstream?delay=50",
            "--rate",
            "10/s",
            "--warmup",
            "1s",
            "--duration",
            "2s",
            "--timeout",
            "3s",
            "--connections",
            "20",
            "--container-id",
            "missing-container",
            "--loadgen-version",
            "12.13.0",
            "--downstream-url",
            "http://downstream:8080/io",
            "--downstream-timeout",
            "500ms",
            "--started-at",
            "start",
            "--ended-at",
            "end",
        ]
        subprocess.run(command, check=True)
        metadata = json.loads(output.read_text(encoding="utf-8"))
        assert metadata["workload"] == "/downstream?delay=50"
        assert metadata["downstream"] == {
            "url": "http://downstream:8080/io",
            "timeout": "500ms",
        }


if __name__ == "__main__":
    main()
