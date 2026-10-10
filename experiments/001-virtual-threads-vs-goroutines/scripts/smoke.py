#!/usr/bin/env python3
"""Contract smoke tests for one live Experiment 001 service."""

import http.client
import json
import sys
import time
from urllib.error import HTTPError
from urllib.request import Request, urlopen


def get(base_url: str, path: str, timeout: float = 4.0):
    request = Request(base_url + path, method="GET")
    try:
        with urlopen(request, timeout=timeout) as response:
            return response.status, response.headers, response.read()
    except HTTPError as error:
        return error.code, error.headers, error.read()


def json_body(status, headers, body):
    assert headers.get_content_type() == "application/json", headers.get("Content-Type")
    return status, json.loads(body.decode("utf-8"))


def main(base_url: str):
    status, headers, body = get(base_url, "/health")
    status, payload = json_body(status, headers, body)
    assert status == 200, (status, payload)
    assert payload == {"status": "ok"}, payload

    for delay in (0, 50):
        status, headers, body = get(base_url, f"/io?delay={delay}")
        status, payload = json_body(status, headers, body)
        assert status == 200, (status, payload)
        assert payload == {"status": "ok", "delay_ms": delay}, payload

    for invalid in ("", "not-a-number", "-1", "5001"):
        suffix = "" if invalid == "" else f"?delay={invalid}"
        status, headers, body = get(base_url, "/io" + suffix)
        status, payload = json_body(status, headers, body)
        assert status == 400, (invalid, status, payload)
        assert "error" in payload, payload

    status, headers, body = get(base_url, "/io?delay=5000", timeout=4.0)
    status, payload = json_body(status, headers, body)
    assert status == 504, (status, payload)
    assert payload == {"status": "timeout", "delay_ms": 5000}, payload

    host = base_url.removeprefix("http://").split(":", 1)[0]
    port = int(base_url.rsplit(":", 1)[1])
    connection = http.client.HTTPConnection(host, port, timeout=1.0)
    connection.request("GET", "/io?delay=5000")
    connection.close()
    time.sleep(0.1)

    status, headers, body = get(base_url, "/health")
    status, payload = json_body(status, headers, body)
    assert status == 200 and payload == {"status": "ok"}, (status, payload)
    print(f"smoke test passed for {base_url}")


if __name__ == "__main__":
    if len(sys.argv) != 2:
        raise SystemExit(f"usage: {sys.argv[0]} http://host:port")
    main(sys.argv[1])
