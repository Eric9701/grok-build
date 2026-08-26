#!/usr/bin/env python3
"""Fetch Atlas admin task-report JSON for work-portrait analysis.

Stdlib only. Default strips prompt/error from report rows.
"""

from __future__ import annotations

import argparse
import json
import os
import sys
import urllib.error
import urllib.parse
import urllib.request
from typing import Any

DEFAULT_ORIGIN = "http://10.218.220.237:22255"
DEFAULT_BASE = DEFAULT_ORIGIN + "/atlas"


def normalize_base(raw: str) -> str:
    """Accept origin or origin+/atlas. Empty path gets /atlas (atlas-server prefix)."""
    s = (raw or "").strip()
    if not s:
        return DEFAULT_BASE
    parsed = urllib.parse.urlparse(s if "://" in s else "http://" + s)
    path = (parsed.path or "").rstrip("/")
    if path in ("", "/"):
        path = "/atlas"
    return urllib.parse.urlunparse((parsed.scheme, parsed.netloc, path, "", "", "")).rstrip("/")


def get_json(base: str, params: dict[str, str]) -> Any:
    q = urllib.parse.urlencode({k: v for k, v in params.items() if v != ""})
    url = base.rstrip("/") + "/admin/api/task-reports?" + q
    req = urllib.request.Request(url, headers={"Accept": "application/json"})
    try:
        with urllib.request.urlopen(req, timeout=60) as resp:
            body = resp.read().decode("utf-8")
    except urllib.error.HTTPError as e:
        detail = e.read().decode("utf-8", errors="replace")
        raise SystemExit(f"HTTP {e.code} {url}\n{detail}") from e
    except urllib.error.URLError as e:
        raise SystemExit(f"request failed {url}: {e}") from e
    return json.loads(body)


def strip_sensitive(reports: list[dict[str, Any]]) -> None:
    for row in reports:
        row.pop("prompt", None)
        row.pop("error", None)


def fetch_user(base: str, user_id: str, from_day: str, to_day: str, limit: int, sensitive: bool) -> dict[str, Any]:
    common = {"from": from_day, "to": to_day, "user_id": user_id}
    agg = get_json(base, {**common, "aggregate": "1"})
    listing = get_json(base, {**common, "limit": str(limit)})
    reports = listing.get("reports") or []
    if not sensitive:
        strip_sensitive(reports)
    return {
        "userId": agg.get("userId") or listing.get("userId") or user_id,
        "from": agg.get("from", from_day),
        "to": agg.get("to", to_day),
        "agents": agg.get("agents") or [],
        "models": agg.get("models") or [],
        "count": listing.get("count", len(reports)),
        "reports": reports,
    }


def main() -> None:
    p = argparse.ArgumentParser(description="Fetch Atlas task reports for work portraits")
    p.add_argument(
        "--base",
        default="",
        help=f"Atlas origin or origin+/atlas (default {DEFAULT_ORIGIN}/ or $ATLAS_BASE)",
    )
    p.add_argument("--from", dest="from_day", required=True, help="YYYY-MM-DD or all")
    p.add_argument("--to", dest="to_day", required=True, help="YYYY-MM-DD or all")
    p.add_argument("--user-id", action="append", default=[], dest="user_ids")
    p.add_argument("--email", action="append", default=[], dest="emails")
    p.add_argument("--all-users", action="store_true")
    p.add_argument("--max-users", type=int, default=15)
    p.add_argument("--limit", type=int, default=100, help="per-user report list limit (1-500)")
    p.add_argument("--include-sensitive", action="store_true")
    p.add_argument("--out", help="write JSON to this path instead of stdout")
    args = p.parse_args()
    args.base = normalize_base(args.base or os.environ.get("ATLAS_BASE", "") or DEFAULT_ORIGIN)

    limit = args.limit
    if limit <= 0 or limit > 500:
        raise SystemExit("--limit must be 1..500")
    max_users = args.max_users
    if max_users <= 0 or max_users > 100:
        raise SystemExit("--max-users must be 1..100")

    overall = get_json(
        args.base,
        {"from": args.from_day, "to": args.to_day, "limit": str(max(max_users, 50))},
    )
    out: dict[str, Any] = {
        "base": args.base.rstrip("/"),
        "from": overall.get("from", args.from_day),
        "to": overall.get("to", args.to_day),
        "overall": overall,
        "users": [],
    }

    wanted: list[str] = []
    seen: set[str] = set()
    for uid in args.user_ids:
        uid = uid.strip()
        if uid and uid not in seen:
            wanted.append(uid)
            seen.add(uid)
    for email in args.emails:
        email = email.strip()
        if not email:
            continue
        resolved = get_json(
            args.base,
            {"from": args.from_day, "to": args.to_day, "email": email, "aggregate": "1"},
        )
        uid = (resolved.get("userId") or "").strip()
        if uid and uid not in seen:
            wanted.append(uid)
            seen.add(uid)
        elif not uid:
            out.setdefault("unresolvedEmails", []).append(email)

    if args.all_users:
        for row in overall.get("users") or []:
            uid = (row.get("userId") or "").strip()
            if uid and uid not in seen:
                wanted.append(uid)
                seen.add(uid)
            if len(wanted) >= max_users:
                break
    elif wanted:
        wanted = wanted[:max_users]

    for uid in wanted:
        out["users"].append(
            fetch_user(args.base, uid, args.from_day, args.to_day, limit, args.include_sensitive)
        )

    text = json.dumps(out, ensure_ascii=False, indent=2)
    if args.out:
        with open(args.out, "w", encoding="utf-8") as f:
            f.write(text)
            f.write("\n")
    else:
        sys.stdout.reconfigure(encoding="utf-8") if hasattr(sys.stdout, "reconfigure") else None
        print(text)


if __name__ == "__main__":
    main()
