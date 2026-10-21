#!/usr/bin/env python3
"""Nagios plugin: check that the app is up AND that its real feature works.

Exit codes follow the Nagios plugin rules: 0 OK, 1 WARNING, 2 CRITICAL, 3 UNKNOWN.

Examples:
  check_app.py -H 10.0.1.5 -p 8000 --path /healthz
  check_app.py -H 10.0.1.5 -p 8000 --path /api/compress --upload /usr/local/share/sample.jpg \
               --field file --expect-type image/
"""
import argparse
import mimetypes
import os
import sys
import time
import urllib.error
import urllib.request
import uuid

OK, WARNING, CRITICAL, UNKNOWN = 0, 1, 2, 3
LABELS = {OK: "OK", WARNING: "WARNING", CRITICAL: "CRITICAL", UNKNOWN: "UNKNOWN"}


def finish(code, message, seconds=None, size=None):
    perf = []
    if seconds is not None:
        perf.append(f"time={seconds:.3f}s")
    if size is not None:
        perf.append(f"size={size}B")
    perf_text = (" | " + " ".join(perf)) if perf else ""
    print(f"APP {LABELS[code]} - {message}{perf_text}")
    sys.exit(code)


def multipart(field, path):
    boundary = uuid.uuid4().hex
    filename = os.path.basename(path)
    ctype = mimetypes.guess_type(filename)[0] or "application/octet-stream"
    with open(path, "rb") as f:
        data = f.read()
    body = (
        f"--{boundary}\r\n"
        f'Content-Disposition: form-data; name="{field}"; filename="{filename}"\r\n'
        f"Content-Type: {ctype}\r\n\r\n"
    ).encode() + data + f"\r\n--{boundary}--\r\n".encode()
    return body, f"multipart/form-data; boundary={boundary}"


def main():
    ap = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    ap.add_argument("-H", "--host", required=True)
    ap.add_argument("-p", "--port", type=int, default=80)
    ap.add_argument("--path", default="/healthz")
    ap.add_argument("--upload", help="file to POST as multipart form data")
    ap.add_argument("--field", default="file", help="form field name for --upload")
    ap.add_argument("--expect-text", help="text that must appear in the response body")
    ap.add_argument("--expect-type", help="Content-Type prefix the response must have")
    ap.add_argument("-w", "--warning", type=float, default=2.0, help="seconds")
    ap.add_argument("-c", "--critical", type=float, default=5.0, help="seconds")
    ap.add_argument("-t", "--timeout", type=float, default=15.0)
    args = ap.parse_args()

    url = f"http://{args.host}:{args.port}{args.path}"
    headers = {"User-Agent": "nagios-check_app"}
    data = None
    if args.upload:
        if not os.path.isfile(args.upload):
            finish(UNKNOWN, f"sample file not found: {args.upload}")
        data, headers["Content-Type"] = multipart(args.field, args.upload)

    start = time.monotonic()
    try:
        req = urllib.request.Request(url, data=data, headers=headers,
                                     method="POST" if data else "GET")
        with urllib.request.urlopen(req, timeout=args.timeout) as res:
            body = res.read()
            status = res.status
            ctype = res.headers.get("Content-Type", "")
    except urllib.error.HTTPError as e:
        finish(CRITICAL, f"{url} returned HTTP {e.code}", time.monotonic() - start)
    except Exception as e:  # connection refused, timeout, DNS ...
        finish(CRITICAL, f"{url} unreachable: {e}")
    elapsed = time.monotonic() - start

    if status != 200:
        finish(CRITICAL, f"{url} returned HTTP {status}", elapsed, len(body))
    if args.expect_type and not ctype.startswith(args.expect_type):
        finish(CRITICAL, f"unexpected Content-Type {ctype!r}", elapsed, len(body))
    if args.expect_text and args.expect_text.encode() not in body:
        finish(CRITICAL, f"response missing {args.expect_text!r}", elapsed, len(body))
    if elapsed >= args.critical:
        finish(CRITICAL, f"slow response {elapsed:.2f}s", elapsed, len(body))
    if elapsed >= args.warning:
        finish(WARNING, f"slow response {elapsed:.2f}s", elapsed, len(body))
    finish(OK, f"HTTP 200 in {elapsed:.2f}s", elapsed, len(body))


if __name__ == "__main__":
    main()
