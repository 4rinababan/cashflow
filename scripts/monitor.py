#!/usr/bin/env python3
"""Health-check Cashflow production and email on state transitions.

Checks GET /healthz and /readyz on the public URL. Sends an email (via the
same SMTP account Cashflow itself uses for workspace invites) only when the
status actually CHANGES (up -> down, or down -> up) — never on every run —
so a sustained outage produces exactly one "down" email and one "recovered"
email, not one every cron tick.

Deployed as ~/cashflow/scripts/monitor.py on the production VPS (git-cloned,
see docs/ops/production-deployment.md), run every 5 minutes via cron. This
repo file is the source of truth — after changing it, `git pull` on the VPS
(scripts/deploy-vps.sh does this) and re-run once manually to confirm
before trusting the cron schedule again.

Usage:
  monitor.py                  normal check (run from cron)
  monitor.py --test-email     send a test email immediately, ignoring state

Reads SMTP_HOST/PORT/USER/PASSWORD/FROM_ADDRESS/FROM_NAME from the same
.env file Cashflow's own backend uses, so credentials live in exactly one
place on this server.
"""
import smtplib
import ssl
import sys
import time
import urllib.request
from email.mime.text import MIMEText
from pathlib import Path

BASE_URL = "https://api.laburaku.com"
CHECK_PATHS = ["/healthz", "/readyz"]
ALERT_TO = "ari.decentindonesia@gmail.com"
ENV_FILE = Path.home() / "cashflow" / ".env"
STATE_FILE = Path.home() / "cashflow-monitor" / "state"

# This VPS's resolver has shown transient DNS hiccups during development
# (a lookup fails once, then succeeds on immediate retry) — retry both the
# health check and the SMTP send a couple of times before concluding
# anything is actually down, so a one-off resolver blip never turns into a
# false "DOWN" email (or, worse, a failed attempt to send the real one).
RETRIES = 3
RETRY_DELAY_SECONDS = 5


def load_env(path: Path) -> dict:
    env = {}
    for line in path.read_text().splitlines():
        line = line.strip()
        if not line or line.startswith("#") or "=" not in line:
            continue
        key, _, value = line.partition("=")
        env[key.strip()] = value.strip()
    return env


def _one_check_pass() -> bool:
    for path in CHECK_PATHS:
        try:
            with urllib.request.urlopen(BASE_URL + path, timeout=10) as resp:
                if resp.status != 200:
                    return False
        except Exception:
            return False
    return True


def check_up() -> bool:
    for attempt in range(RETRIES):
        if _one_check_pass():
            return True
        if attempt < RETRIES - 1:
            time.sleep(RETRY_DELAY_SECONDS)
    return False


def send_email(env: dict, subject: str, body: str) -> None:
    msg = MIMEText(body)
    msg["Subject"] = subject
    msg["From"] = f'{env["SMTP_FROM_NAME"]} <{env["SMTP_FROM_ADDRESS"]}>'
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


def read_state() -> str | None:
    if STATE_FILE.exists():
        return STATE_FILE.read_text().strip()
    return None


def write_state(state: str) -> None:
    STATE_FILE.parent.mkdir(parents=True, exist_ok=True)
    STATE_FILE.write_text(state)


def main() -> None:
    env = load_env(ENV_FILE)

    if "--test-email" in sys.argv:
        send_email(env, "[Cashflow monitor] test email",
                   "This is a test alert from monitor.py — SMTP delivery works.")
        print("test email sent")
        return

    up_now = check_up()
    previous = read_state()

    if previous is None:
        # First run ever: establish a baseline, don't alert.
        write_state("up" if up_now else "down")
        print(f"baseline established: {'up' if up_now else 'down'}")
        return

    if up_now and previous == "down":
        send_email(env, "[Cashflow monitor] RECOVERED",
                   f"{BASE_URL} is responding again (both /healthz and /readyz OK).")
        write_state("up")
        print("recovered, email sent")
    elif not up_now and previous == "up":
        send_email(env, "[Cashflow monitor] DOWN",
                   f"{BASE_URL} failed a health check (/healthz or /readyz did not return 200).")
        write_state("down")
        print("down, email sent")
    else:
        print(f"no change ({previous})")


if __name__ == "__main__":
    main()
