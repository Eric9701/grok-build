# Models cache 对托管密钥使用 At-Rest ENC

客户端 `models_cache.json` 与 **Managed Config Segment**（`config.toml` 的 `[model.*]`）一样，对托管条目的 `api_key` 和 `info.model`（路由名）落盘 **At-Rest ENC**；catalog id 仍作 JSON map key / `info.id` 明文。一次拉取要么全是 Managed Catalog Mode、要么全是 fallback，因此用文件级 `managed: true` 标记；托管 cache 里这两字段不是 `ENC(...)` 则整文件作废再拉网，不为旧明文做迁移改写。Fallback catalog 保持明文。Unix 写入 `0o600`。不整文件加密：要和 toml 同一套 `ENC(...)`，并保住 JSON watcher / 跨进程 hot-reload。
