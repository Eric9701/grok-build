#!/usr/bin/env python3
"""xAI 官方 Device Login，给 probe_xai_proxy.py 准备 cli-chat-proxy 凭证。

走 RFC 8628，打 https://auth.x.ai（不是企业 atlas-server）。
auth.json 写在本脚本同目录，避免覆盖 ~/.atlas 企业会话。

用法（在 services/atlas-server 下，需要能访问 auth.x.ai）：

    $env:HTTPS_PROXY="http://127.0.0.1:7890"
    python scripts/xai_login.py              # 浏览器登录
    python scripts/xai_login.py --refresh    # 用 refresh_token 续期
    python scripts/xai_login.py --status     # 只看摘要，不打印完整 token
    python scripts/probe_xai_proxy.py        # 随后拉 probe_*
"""

from __future__ import annotations

import argparse
import base64
import json
import os
import stat
import sys
import time
import urllib.error
import urllib.parse
import urllib.request
import webbrowser
from datetime import datetime, timedelta, timezone
from pathlib import Path
from typing import Any

# 与 grok-cli 内置 OAuth2 client_id 一致（公开 client，不是密钥）。
DEFAULT_ISSUER = "https://auth.x.ai"
DEFAULT_CLIENT_ID = "b1a00492-073a-47ea-816f-4c329264a828"
DEFAULT_SCOPES = (
    "openid profile email offline_access grok-cli:access api:access "
    "conversations:read conversations:write workspaces:read workspaces:write"
)
DEVICE_GRANT = "urn:ietf:params:oauth:grant-type:device_code"
LEGACY_SCOPE = "https://accounts.x.ai/sign-in"
EARLY_REFRESH_SECS = 300
POLL_FALLBACK_SECS = 5
SLOW_DOWN_SECS = 5


def script_dir() -> Path:
    return Path(__file__).resolve().parent


def auth_file() -> Path:
    return script_dir() / "auth.json"


def issuer() -> str:
    return (
        os.environ.get("XAI_OAUTH2_ISSUER")
        or os.environ.get("GROK_OAUTH2_ISSUER")
        or DEFAULT_ISSUER
    ).rstrip("/")


def client_id() -> str:
    return os.environ.get("GROK_OAUTH2_CLIENT_ID") or DEFAULT_CLIENT_ID


def scope_key(iss: str | None = None, cid: str | None = None) -> str:
    return f"{iss or issuer()}::{cid or client_id()}"


def http_opener() -> urllib.request.OpenerDirector:
    proxy = os.environ.get("https_proxy") or os.environ.get("HTTPS_PROXY")
    handlers: list[Any] = []
    if proxy:
        handlers.append(urllib.request.ProxyHandler({"http": proxy, "https": proxy}))
    return urllib.request.build_opener(*handlers)


def http_json(
    opener: urllib.request.OpenerDirector,
    method: str,
    url: str,
    *,
    form: dict[str, str] | None = None,
    timeout: int = 30,
) -> tuple[int, dict[str, Any] | None, bytes]:
    data = None
    headers = {
        "User-Agent": "atlas-server-xai-login",
        "Accept": "application/json",
        "x-grok-client-surface": "cli",
    }
    if form is not None:
        data = urllib.parse.urlencode(form).encode("utf-8")
        headers["Content-Type"] = "application/x-www-form-urlencoded"
    req = urllib.request.Request(url, data=data, headers=headers, method=method)
    try:
        with opener.open(req, timeout=timeout) as resp:
            raw = resp.read()
            parsed = _try_json(raw)
            return resp.status, parsed, raw
    except urllib.error.HTTPError as e:
        raw = e.read() if hasattr(e, "read") else b""
        return e.code, _try_json(raw), raw


def _try_json(raw: bytes) -> dict[str, Any] | None:
    if not raw:
        return None
    try:
        val = json.loads(raw.decode("utf-8", "replace"))
    except json.JSONDecodeError:
        return None
    return val if isinstance(val, dict) else None


def jwt_payload(token: str) -> dict[str, Any]:
    parts = token.split(".")
    if len(parts) < 2:
        return {}
    pad = "=" * (-len(parts[1]) % 4)
    try:
        raw = base64.urlsafe_b64decode(parts[1] + pad)
        val = json.loads(raw.decode("utf-8"))
    except (ValueError, json.JSONDecodeError):
        return {}
    return val if isinstance(val, dict) else {}


def token_suffix(token: str) -> str:
    if len(token) <= 8:
        return "****"
    return f"...{token[-6:]}"


def load_store(path: Path | None = None) -> dict[str, Any]:
    p = path or auth_file()
    if not p.is_file():
        return {}
    raw = p.read_text(encoding="utf-8").strip()
    if not raw:
        return {}
    data = json.loads(raw)
    if not isinstance(data, dict):
        raise SystemExit(f"auth.json 不是对象: {p}")
    return data


def pick_entry(store: dict[str, Any]) -> tuple[str, dict[str, Any]] | None:
    preferred = [
        scope_key(),
        LEGACY_SCOPE,
    ]
    for key in preferred:
        entry = store.get(key)
        if isinstance(entry, dict) and (entry.get("key") or entry.get("access_token")):
            return key, entry
    for key, entry in store.items():
        if isinstance(entry, dict) and (entry.get("key") or entry.get("access_token")):
            return str(key), entry
    return None


def access_token_of(entry: dict[str, Any]) -> str:
    return str(entry.get("key") or entry.get("access_token") or "")


def parse_expires_at(entry: dict[str, Any]) -> datetime | None:
    raw = entry.get("expires_at")
    if isinstance(raw, str) and raw:
        text = raw.replace("Z", "+00:00")
        try:
            dt = datetime.fromisoformat(text)
        except ValueError:
            dt = None
        if dt is not None:
            if dt.tzinfo is None:
                dt = dt.replace(tzinfo=timezone.utc)
            return dt
    token = access_token_of(entry)
    exp = jwt_payload(token).get("exp")
    if isinstance(exp, (int, float)):
        return datetime.fromtimestamp(exp, tz=timezone.utc)
    return None


def needs_refresh(entry: dict[str, Any], skew: int = EARLY_REFRESH_SECS) -> bool:
    exp = parse_expires_at(entry)
    if exp is None:
        return False
    return datetime.now(timezone.utc) + timedelta(seconds=skew) >= exp


def looks_enterprise(store: dict[str, Any]) -> bool:
    markers = ("22255", "/atlas", "localhost")
    for key, entry in store.items():
        blob = key.lower()
        if isinstance(entry, dict):
            blob += " " + str(entry.get("oidc_issuer") or "").lower()
        if any(m in blob for m in markers) and "auth.x.ai" not in blob:
            return True
    return False


def save_entry(scope: str, entry: dict[str, Any], path: Path | None = None) -> Path:
    p = path or auth_file()
    p.parent.mkdir(parents=True, exist_ok=True)
    store = load_store(p)
    store[scope] = entry
    tmp = p.with_suffix(p.suffix + f".{os.getpid()}.tmp")
    tmp.write_text(json.dumps(store, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")
    os.replace(tmp, p)
    try:
        os.chmod(p, stat.S_IRUSR | stat.S_IWUSR)
    except OSError:
        pass
    return p


def build_entry(tokens: dict[str, Any], iss: str, cid: str) -> dict[str, Any]:
    access = str(tokens.get("access_token") or "")
    if not access:
        raise SystemExit("token 响应缺少 access_token")
    claims = jwt_payload(str(tokens.get("id_token") or "")) or jwt_payload(access)
    expires_in = tokens.get("expires_in")
    expires_at = None
    if isinstance(expires_in, (int, float)):
        expires_at = datetime.now(timezone.utc) + timedelta(seconds=int(expires_in))
    elif isinstance(claims.get("exp"), (int, float)):
        expires_at = datetime.fromtimestamp(int(claims["exp"]), tz=timezone.utc)
    entry: dict[str, Any] = {
        "key": access,
        "auth_mode": "oidc",
        "create_time": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
        "user_id": str(claims.get("sub") or "unknown"),
        "email": claims.get("email"),
        "refresh_token": tokens.get("refresh_token"),
        "oidc_issuer": iss,
        "oidc_client_id": cid,
    }
    if expires_at is not None:
        entry["expires_at"] = expires_at.strftime("%Y-%m-%dT%H:%M:%SZ")
    if claims.get("principal_type"):
        entry["principal_type"] = claims["principal_type"]
    if claims.get("principal_id"):
        entry["principal_id"] = claims["principal_id"]
    if claims.get("team_id"):
        entry["team_id"] = claims["team_id"]
    return {k: v for k, v in entry.items() if v is not None}


def print_summary(scope: str, entry: dict[str, Any], path: Path) -> None:
    exp = parse_expires_at(entry)
    print(f"auth_file   {path}")
    print(f"scope       {scope}")
    print(f"issuer      {entry.get('oidc_issuer') or '-'}")
    print(f"user_id     {entry.get('user_id') or '-'}")
    print(f"email       {entry.get('email') or '-'}")
    print(f"expires_at  {exp.isoformat() if exp else '-'}")
    print(f"access      {token_suffix(access_token_of(entry))}")
    print(f"refresh     {'yes' if entry.get('refresh_token') else 'no'}")


def load_access_token(*, refresh: bool = True) -> str:
    """供 probe_xai_proxy.py 调用：读脚本同目录 auth.json，必要时续期。"""
    store = load_store()
    picked = pick_entry(store)
    if not picked:
        raise SystemExit(
            "NO_TOKEN: 未找到 xAI 凭证。先跑 python scripts/xai_login.py "
            f"（写入 {auth_file()}）"
        )
    scope, entry = picked
    if refresh and entry.get("refresh_token") and needs_refresh(entry):
        print("access token 即将过期，正在 refresh …")
        entry = do_refresh(entry)
        save_entry(scope, entry)
    token = access_token_of(entry)
    if not token:
        raise SystemExit("NO_TOKEN")
    return token


def do_refresh(entry: dict[str, Any]) -> dict[str, Any]:
    rt = entry.get("refresh_token")
    if not rt:
        raise SystemExit("没有 refresh_token，请重新 python scripts/xai_login.py")
    iss = str(entry.get("oidc_issuer") or issuer())
    cid = str(entry.get("oidc_client_id") or client_id())
    opener = http_opener()
    status, body, raw = http_json(
        opener,
        "POST",
        f"{iss}/oauth2/token",
        form={
            "grant_type": "refresh_token",
            "refresh_token": str(rt),
            "client_id": cid,
        },
    )
    if status < 200 or status >= 300 or not body:
        err = (body or {}).get("error") if body else raw[:200]
        raise SystemExit(f"refresh 失败 HTTP {status}: {err}")
    if not body.get("refresh_token"):
        body["refresh_token"] = rt
    return build_entry(body, iss, cid)


def request_device_code(opener: urllib.request.OpenerDirector, iss: str, cid: str) -> dict[str, Any]:
    status, body, raw = http_json(
        opener,
        "POST",
        f"{iss}/oauth2/device/code",
        form={
            "client_id": cid,
            "scope": DEFAULT_SCOPES,
            "referrer": "grok-build",
        },
    )
    if status < 200 or status >= 300 or not body:
        err = (body or {}).get("error") if body else raw[:300]
        raise SystemExit(f"申请 device code 失败 HTTP {status}: {err}")
    user_code = str(body.get("user_code") or "")
    if not user_code or not all(c.isalnum() or c == "-" for c in user_code):
        raise SystemExit("服务端返回的 user_code 非法")
    uri = str(body.get("verification_uri") or "")
    if not uri.startswith("https://"):
        raise SystemExit(f"verification_uri 必须是 https: {uri}")
    return body


def poll_token(
    opener: urllib.request.OpenerDirector,
    iss: str,
    cid: str,
    device: dict[str, Any],
) -> dict[str, Any]:
    interval = max(int(device.get("interval") or POLL_FALLBACK_SECS), 1)
    expires_in = int(device.get("expires_in") or 900)
    deadline = time.time() + max(expires_in, 600)
    token_url = f"{iss}/oauth2/token"
    device_code = str(device["device_code"])
    while True:
        time.sleep(interval)
        if time.time() > deadline:
            raise SystemExit("Device code 已过期，请重新运行 xai_login.py")
        status, body, raw = http_json(
            opener,
            "POST",
            token_url,
            form={
                "grant_type": DEVICE_GRANT,
                "device_code": device_code,
                "client_id": cid,
            },
        )
        if status >= 200 and status < 300 and body and body.get("access_token"):
            return body
        err = str((body or {}).get("error") or "")
        if err == "authorization_pending":
            continue
        if err == "slow_down":
            interval += SLOW_DOWN_SECS
            continue
        if err == "access_denied":
            raise SystemExit("授权被拒绝")
        if err == "expired_token":
            raise SystemExit("Device code 已过期，请重新运行 xai_login.py")
        detail = (body or {}).get("error_description") or raw[:200]
        raise SystemExit(f"换 token 失败 HTTP {status}: {detail}")


def cmd_login(*, open_browser: bool, force: bool) -> int:
    path = auth_file()
    store = load_store(path) if path.is_file() else {}
    if looks_enterprise(store) and not force:
        print(
            f"拒绝写入 {path}：已有企业 atlas-server 凭证。\n"
            "xAI probe 登录写脚本同目录 auth.json。若确实要覆盖，加 --force。",
            file=sys.stderr,
        )
        return 2
    iss = issuer()
    cid = client_id()
    opener = http_opener()
    print(f"issuer     {iss}")
    print(f"client_id  {cid}")
    print(f"auth_file  {path}")
    device = request_device_code(opener, iss, cid)
    uri = str(device.get("verification_uri_complete") or device["verification_uri"])
    print()
    print("在浏览器打开并确认：")
    print(f"  {uri}")
    print(f"  机器码: {device['user_code']}")
    print()
    if open_browser:
        try:
            webbrowser.open(uri)
        except Exception as e:
            print(f"（未能自动打开浏览器: {e}）")
    tokens = poll_token(opener, iss, cid, device)
    entry = build_entry(tokens, iss, cid)
    saved = save_entry(scope_key(iss, cid), entry, path)
    print("登录成功")
    print_summary(scope_key(iss, cid), entry, saved)
    return 0


def cmd_refresh() -> int:
    store = load_store()
    picked = pick_entry(store)
    if not picked:
        raise SystemExit("没有可 refresh 的凭证，先跑 python scripts/xai_login.py")
    scope, entry = picked
    entry = do_refresh(entry)
    saved = save_entry(scope, entry)
    print("refresh 成功")
    print_summary(scope, entry, saved)
    return 0


def cmd_status() -> int:
    path = auth_file()
    store = load_store(path)
    picked = pick_entry(store)
    if not picked:
        print(f"NO_TOKEN  {path}")
        return 1
    scope, entry = picked
    print_summary(scope, entry, path)
    exp = parse_expires_at(entry)
    if exp and datetime.now(timezone.utc) >= exp:
        print("state      expired")
        return 1
    if needs_refresh(entry):
        print("state      refresh_soon")
    else:
        print("state      ok")
    return 0


def _utf8_stdio() -> None:
    for stream in (sys.stdout, sys.stderr):
        try:
            stream.reconfigure(encoding="utf-8")
        except Exception:
            pass


def main(argv: list[str] | None = None) -> int:
    _utf8_stdio()
    parser = argparse.ArgumentParser(
        description="通过 xAI Device Login 获取 cli-chat-proxy token，供 probe_xai_proxy.py 使用"
    )
    parser.add_argument("--status", action="store_true", help="查看现有凭证摘要")
    parser.add_argument("--refresh", action="store_true", help="用 refresh_token 续期")
    parser.add_argument("--no-browser", action="store_true", help="不自动打开浏览器")
    parser.add_argument(
        "--force",
        action="store_true",
        help="允许写入已含企业 atlas-server 凭证的 auth.json",
    )
    args = parser.parse_args(argv)
    if args.status:
        return cmd_status()
    if args.refresh:
        return cmd_refresh()
    return cmd_login(open_browser=not args.no_browser, force=args.force)


if __name__ == "__main__":
    raise SystemExit(main())
