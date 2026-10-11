#!/usr/bin/env python3
"""Contract smoke tests for one live Experiment 001 service."""

import http.client
import json
import sys
import time
from concurrent.futures import ThreadPoolExecutor
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


def main(base_url: str, scenario: str):
    status, headers, body = get(base_url, "/health")
    status, payload = json_body(status, headers, body)
    assert status == 200, (status, payload)
    assert payload == {"status": "ok"}, payload

    for delay in (0, 50):
        status, headers, body = get(base_url, f"/io?delay={delay}")
        status, payload = json_body(status, headers, body)
        assert status == 200, (status, payload)
        assert payload == {"status": "ok", "delay_ms": delay}, payload

    if scenario == "success":
        for delay in (0, 50):
            status, headers, body = get(base_url, f"/downstream?delay={delay}")
            status, payload = json_body(status, headers, body)
            assert status == 200, (status, payload)
            assert payload == {"status": "ok", "delay_ms": delay}, payload
        status, headers, body = get(base_url, "/fanout?delay=0&branches=4")
        status, payload = json_body(status, headers, body)
        assert status == 200, (status, payload)
        assert payload == {"status": "ok", "delay_ms": 0}, payload

    if scenario in ("error", "slow"):
        delay = 50 if scenario == "error" else 0
        status, headers, body = get(base_url, f"/downstream?delay={delay}", timeout=2.0)
        status, payload = json_body(status, headers, body)
        assert status == 502, (status, payload)
        assert payload == {"status": "bad_gateway", "delay_ms": delay}, payload
        status, headers, body = get(base_url, "/fanout?delay=0&branches=4", timeout=2.0)
        status, payload = json_body(status, headers, body)
        assert status == 502, (status, payload)
        assert payload == {"status": "bad_gateway", "delay_ms": 0}, payload

    if scenario == "capacity":
        with ThreadPoolExecutor(max_workers=2) as executor:
            responses = list(executor.map(lambda _: get(base_url, "/downstream?delay=100"), range(2)))
        statuses = []
        for status, headers, body in responses:
            status, payload = json_body(status, headers, body)
            statuses.append(status)
            assert status in (200, 502), (status, payload)
        assert 200 in statuses and 502 in statuses, statuses

    for invalid in ("", "not-a-number", "-1", "5001"):
        suffix = "" if invalid == "" else f"?delay={invalid}"
        status, headers, body = get(base_url, "/io" + suffix)
        status, payload = json_body(status, headers, body)
        assert status == 400, (invalid, status, payload)
        assert "error" in payload, payload

    for branches in ("", "not-a-number", "0", "17"):
        suffix = "" if branches == "" else f"&branches={branches}"
        status, headers, body = get(base_url, "/fanout?delay=0" + suffix)
        status, payload = json_body(status, headers, body)
        assert status == 400, (branches, status, payload)
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
    if len(sys.argv) not in (2, 3):
        raise SystemExit(f"usage: {sys.argv[0]} http://host:port [scenario]")
    main(sys.argv[1], sys.argv[2] if len(sys.argv) == 3 else "success")
