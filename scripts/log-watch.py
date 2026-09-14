#!/usr/bin/env python3
"""Scans both apps' Docker container logs for new error/crash lines, records
them as JSONL for the admin dashboard's "Recent errors" panel
(internal/admin), and emails a digest when new ones show up.

Meant to run every few minutes from cron on the production VPS (see
docs/ops/production-deployment.md) — deliberately reads via `docker logs`
on the host, not from inside a container: this app's own container has no
Docker socket access (a real security tradeoff that wasn't worth taking
just for this), and gawean-management's logs are not this project's to
reach into except by reading what Docker already exposes on the shared
host, the same way `docker logs` works for any operator.

Usage:
  log-watch.py                normal scan (run from cron)
  log-watch.py --test-email   send a test digest email immediately

State: ~/cashflow-monitor/log-watch-state.json tracks the last-scanned
timestamp per container, so each run only looks at genuinely new lines.
Output: ~/cashflow-monitor/error-log.jsonl (append-only; read by
internal/admin/errorlog.go inside the api container via a read-only bind
mount — see docker-compose.prod.yml).
"""
import json
import re
import smtplib
import ssl
import subprocess
import sys
import time
from email.mime.text import MIMEText
from pathlib import Path

ALERT_TO = "ari.decentindonesia@gmail.com"
ENV_FILE = Path.home() / "cashflow" / ".env"
STATE_FILE = Path.home() / "cashflow-monitor" / "log-watch-state.json"
ERROR_LOG_FILE = Path.home() / "cashflow-monitor" / "error-log.jsonl"

# (container name, app label shown on the dashboard)
WATCHED_CONTAINERS = [
    ("cashflow-api-1", "cashflow"),
    ("deploy-web-1", "gawean"),
]

# Intentionally simple and a little over-inclusive — a false positive here
# just adds one noisy dashboard row; a false negative hides a real crash.
ERROR_PATTERN = re.compile(r"(error|exception|panic|fatal|unhandled)", re.IGNORECASE)

RETRIES = 3
RETRY_DELAY_SECONDS = 5
MAX_LINES_PER_RUN = 200  # guards against one run emailing a wall of text if something goes very wrong


def load_env(path: Path) -> dict:
    env = {}
    if not path.exists():
        return env
    for line in path.read_text().splitlines():
        line = line.strip()
        if not line or line.startswith("#") or "=" not in line:
            continue
        key, _, value = line.partition("=")
        env[key.strip()] = value.strip()
    return env


def load_state() -> dict:
    if STATE_FILE.exists():
        return json.loads(STATE_FILE.read_text())
    return {}


def save_state(state: dict) -> None:
    STATE_FILE.parent.mkdir(parents=True, exist_ok=True)
    STATE_FILE.write_text(json.dumps(state))


def scan_container(container: str, app: str, since: str | None) -> list[dict]:
    cmd = ["docker", "logs", "--timestamps"]
    if since:
        cmd += ["--since", since]
    else:
        cmd += ["--since", "10m"]  # first-ever run: only look back a short window, not all history
    cmd.append(container)

    try:
        result = subprocess.run(cmd, capture_output=True, text=True, timeout=30)
    except Exception as exc:
        return [{"app": app, "container": container, "timestamp": now_iso(), "message": f"log-watch.py could not read logs: {exc}"}]

    matches = []
    for line in (result.stdout + result.stderr).splitlines():
        if not ERROR_PATTERN.search(line):
            continue
        timestamp, _, message = line.partition(" ")
        matches.append({"app": app, "container": container, "timestamp": timestamp, "message": message.strip()[:2000]})
        if len(matches) >= MAX_LINES_PER_RUN:
            break
    return matches


def now_iso() -> str:
    return time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())


def send_email(env: dict, subject: str, body: str) -> None:
    msg = MIMEText(body)
    msg["Subject"] = subject
    msg["From"] = f'{env.get("SMTP_FROM_NAME", "Cashflow")} <{env.get("SMTP_FROM_ADDRESS", "")}>'
    msg["To"] = ALERT_TO

    last_error: Exception | None = None
    for attempt in range(RETRIES):
        try:
            context = ssl.create_default_context()
            with smtplib.SMTP_SSL(env["SMTP_HOST"], int(env["SMTP_PORT"]), context=context) as server:
                server.login(env["SMTP_USER"], env["SMTP_PASSWORD"])
                server.sendmail(env["SMTP_FROM_ADDRESS"], [ALERT_TO], msg.as_string())
            return
        except Exception as exc:  # noqa: BLE001 - genuinely want to retry any failure here
            last_error = exc
            if attempt < RETRIES - 1:
                time.sleep(RETRY_DELAY_SECONDS)
    raise RuntimeError(f"failed to send email after {RETRIES} attempts") from last_error


def main() -> None:
    env = load_env(ENV_FILE)

    if "--test-email" in sys.argv:
        send_email(env, "[Cashflow log-watch] test email",
                   "This is a test digest from log-watch.py — SMTP delivery works.")
        print("test email sent")
        return

    state = load_state()
    scan_started_at = now_iso()
    all_new: list[dict] = []

    for container, app in WATCHED_CONTAINERS:
        since = state.get(container)
        matches = scan_container(container, app, since)
        all_new.extend(matches)
        state[container] = scan_started_at

    save_state(state)

    if not all_new:
        print("no new errors")
        return

    ERROR_LOG_FILE.parent.mkdir(parents=True, exist_ok=True)
    with ERROR_LOG_FILE.open("a") as f:
        for entry in all_new:
            f.write(json.dumps(entry) + "\n")

    print(f"{len(all_new)} new error line(s) logged")

    if env.get("SMTP_HOST"):
        lines = "\n".join(f"[{e['app']}] {e['timestamp']}  {e['message']}" for e in all_new[:50])
        more = f"\n\n...and {len(all_new) - 50} more." if len(all_new) > 50 else ""
        try:
            send_email(
                env,
                f"[Cashflow log-watch] {len(all_new)} new error line(s)",
                f"New error/exception lines found in the last scan:\n\n{lines}{more}",
            )
            print("digest email sent")
        except Exception as exc:
            print(f"WARNING: could not send digest email: {exc}", file=sys.stderr)
    else:
        print("SMTP not configured — skipping digest email")


if __name__ == "__main__":
    main()
