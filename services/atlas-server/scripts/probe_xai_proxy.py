import json
import os
import sys
import urllib.request

auth_path = os.environ.get("ATLAS_GROK_HOME", os.path.join(os.path.expanduser("~"), ".grok"))
auth_path = os.path.join(auth_path, "auth.json")
auth = json.load(open(auth_path, encoding="utf-8"))
entry = next(iter(auth.values()))
token = entry.get("key") or entry.get("access_token")
if not token:
    print("NO_TOKEN")
    sys.exit(1)

base = "https://cli-chat-proxy.grok.com/v1"
headers = {
    "Authorization": f"Bearer {token}",
    "X-XAI-Token-Auth": "xai-grok-cli",
    "User-Agent": "atlas-server-probe",
    "Accept": "*/*",
}
proxy = os.environ.get("https_proxy") or os.environ.get("HTTPS_PROXY")
handlers = []
if proxy:
    handlers.append(urllib.request.ProxyHandler({"http": proxy, "https": proxy}))
opener = urllib.request.build_opener(*handlers)
out_dir = os.environ.get("ATLAS_DOWNLOAD_DIR") or os.path.join(os.getcwd(), "download")
os.makedirs(out_dir, exist_ok=True)
print("writing probes to", out_dir)


def get(path: str, binary: bool = False, timeout: int = 90):
    req = urllib.request.Request(base + path, headers=headers)
    try:
        with opener.open(req, timeout=timeout) as resp:
            data = resp.read()
            print(
                f"OK {path} status={resp.status} bytes={len(data)} "
                f"ctype={resp.headers.get('Content-Type')}"
            )
            safe_name = path.strip("/").replace("/", "_").replace("?", "_")
            out = os.path.join(out_dir, f"probe_{safe_name}")
            if binary:
                open(out, "wb").write(data)
                print("  saved", out)
                return data
            text = data.decode("utf-8", "replace")
            open(out + ".json", "w", encoding="utf-8").write(text[:500000])
            try:
                j = json.loads(text)
            except Exception as e:
                print("  non-json", e, "head=", text[:200])
                return data
            if path.startswith("/models"):
                ids = [m.get("id") for m in j.get("data", [])[:30]]
                print("  model_count=", len(j.get("data", [])))
                print("  model_ids_sample=", ids)
                if j.get("data"):
                    print("  first_model_keys=", sorted(j["data"][0].keys()))
            elif path.startswith("/settings"):
                print("  settings_keys=", sorted(j.keys()))
                for k in [
                    "allow_access",
                    "default_model",
                    "subscription_tier",
                    "subscription_tier_display",
                    "cursor_skills_enabled",
                    "claude_skills_enabled",
                ]:
                    if k in j:
                        print(f"  {k}=", j[k])
            elif path.startswith("/user"):
                print("  user_keys=", sorted(j.keys()))
                print("  subscriptionTier=", j.get("subscriptionTier"))
            elif "mcp" in path:
                print("  top_keys=", sorted(j.keys()) if isinstance(j, dict) else type(j))
                if isinstance(j, dict) and "mcp_servers" in j:
                    print("  mcp_servers_count=", len(j["mcp_servers"]))
            elif "billing" in path:
                print("  billing_keys=", sorted(j.keys()) if isinstance(j, dict) else type(j))
            elif "subagents" in path:
                print("  bundle_keys=", sorted(j.keys()) if isinstance(j, dict) else type(j))
                if isinstance(j, dict):
                    for k in ["version", "skills", "agents", "personas", "roles"]:
                        v = j.get(k)
                        if isinstance(v, dict):
                            print(f"  {k}_count=", len(v))
                        elif v is not None:
                            print(f"  {k}=", v)
            return data
    except Exception as e:
        print(f"ERR {path}: {e}")
        if hasattr(e, "read"):
            try:
                print("  body=", e.read()[:400])
            except Exception:
                pass
        return None


get("/user?include=subscription")
get("/settings")
get("/models")
get("/mcp/configs")
get("/billing?format=credits")
get("/feedback/config")
get("/subagents/bundle")
get("/bundle/archive", binary=True)
