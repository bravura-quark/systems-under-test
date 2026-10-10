#!/usr/bin/env python3

import argparse
import json
import platform
import socket
import subprocess
from pathlib import Path


def command_output(*command: str, cwd: Path | None = None) -> str | None:
    try:
        completed = subprocess.run(
            command,
            cwd=cwd,
            check=True,
            capture_output=True,
            text=True,
        )
    except (OSError, subprocess.CalledProcessError):
        return None
    return completed.stdout.strip() or None


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Write reproducibility metadata for one benchmark run.")
    parser.add_argument("--output", required=True)
    parser.add_argument("--service", required=True, choices=("java", "go"))
    parser.add_argument("--workload", required=True)
    parser.add_argument("--target", required=True)
    parser.add_argument("--rate", required=True)
    parser.add_argument("--warmup", required=True)
    parser.add_argument("--duration", required=True)
    parser.add_argument("--timeout", required=True)
    parser.add_argument("--connections", required=True, type=int)
    parser.add_argument("--container-id", required=True)
    parser.add_argument("--loadgen-version", required=True)
    parser.add_argument("--downstream-url", required=True)
    parser.add_argument("--downstream-timeout", required=True)
    parser.add_argument("--downstream-mode", required=True)
    parser.add_argument("--downstream-max-in-flight", required=True, type=int)
    parser.add_argument("--started-at", required=True)
    parser.add_argument("--ended-at", required=True)
    return parser.parse_args()


def main() -> None:
    args = parse_args()
    experiment_dir = Path(__file__).resolve().parents[1]
    output_path = Path(args.output)
    output_path.parent.mkdir(parents=True, exist_ok=True)

    metadata = {
        "service": args.service,
        "workload": args.workload,
        "target": args.target,
        "rate": args.rate,
        "warmup_duration": args.warmup,
        "measurement_duration": args.duration,
        "request_timeout": args.timeout,
        "connections": args.connections,
        "started_at": args.started_at,
        "ended_at": args.ended_at,
        "git_commit": command_output("git", "rev-parse", "HEAD", cwd=experiment_dir),
        "container_image_id": command_output("docker", "inspect", "-f", "{{.Image}}", args.container_id),
        "docker_server_version": command_output("docker", "version", "--format", "{{.Server.Version}}"),
        "compose_version": command_output("docker", "compose", "version", "--short"),
        "loadgen_version": args.loadgen_version,
        "downstream": {
            "url": args.downstream_url,
            "timeout": args.downstream_timeout,
            "mode": args.downstream_mode,
            "max_in_flight": args.downstream_max_in_flight,
        },
        "host": {
            "hostname": socket.gethostname(),
            "platform": platform.platform(),
            "architecture": platform.machine(),
            "python": platform.python_version(),
        },
    }

    output_path.write_text(json.dumps(metadata, indent=2) + "\n", encoding="utf-8")


if __name__ == "__main__":
    main()
