//! Persist cloud-managed model catalog entries into `~/.grok|atlas/config.toml`.

use crate::agent::config::ModelEntryConfig;
use crate::sampling::ApiBackend;
use crate::util::model_secret::is_enc;

/// Sync managed remote models into local config.toml.
///
/// - Upserts entries with `managed = true`
/// - Does not overwrite user-authored `[model.<id>]` without `managed = true`
/// - Removes previously managed ids that are absent from this sync
/// - Empty `models` clears all managed entries (assignment revoked / probe fallback)
pub fn sync_managed_models_to_config(models: &[ModelEntryConfig]) -> Result<(), String> {
    let config_path = crate::util::grok_home::grok_home().join("config.toml");
    let content = std::fs::read_to_string(&config_path).unwrap_or_default();
    let mut root: toml::Value = if content.trim().is_empty() {
        toml::Value::Table(toml::map::Map::new())
    } else {
        toml::from_str(&content).map_err(|e| format!("parse config.toml: {e}"))?
    };
    let table = root
        .as_table_mut()
        .ok_or_else(|| "config.toml root is not a table".to_string())?;

    let has_model_table = table
        .get("model")
        .and_then(|v| v.as_table())
        .is_some_and(|t| !t.is_empty());
    if models.is_empty() && !has_model_table {
        return Ok(());
    }

    if !table.contains_key("model") {
        if models.is_empty() {
            return Ok(());
        }
        table.insert(
            "model".to_string(),
            toml::Value::Table(toml::map::Map::new()),
        );
    }
    let model_table = table
        .get_mut("model")
        .and_then(|v| v.as_table_mut())
        .ok_or_else(|| "[model] is not a table".to_string())?;

    let has_managed = model_table.iter().any(|(_, v)| {
        v.as_table()
            .and_then(|t| t.get("managed"))
            .and_then(|x| x.as_bool())
            .unwrap_or(false)
    });
    if models.is_empty() && !has_managed {
        return Ok(());
    }

    let incoming: std::collections::HashSet<String> = models
        .iter()
        .filter_map(|m| {
            let raw = m.id.clone().unwrap_or_else(|| m.model.clone());
            if raw.is_empty() {
                return None;
            }
            if is_enc(&raw) {
                crate::util::model_secret::require_decrypt_managed(&raw, "id").ok()
            } else {
                Some(raw)
            }
        })
        .collect();

    // Drop stale managed entries.
    let stale: Vec<String> = model_table
        .iter()
        .filter_map(|(id, v)| {
            let t = v.as_table()?;
            let managed = t.get("managed").and_then(|x| x.as_bool()).unwrap_or(false);
            if managed && !incoming.contains(id) {
                Some(id.clone())
            } else {
                None
            }
        })
        .collect();
    for id in stale {
        model_table.remove(&id);
    }

    for m in models {
        // Section key is plaintext catalog id (decrypt ENC for table key only).
        let raw_id = m.id.clone().unwrap_or_else(|| m.model.clone());
        if raw_id.is_empty() {
            continue;
        }
        let id = if is_enc(&raw_id) {
            match crate::util::model_secret::require_decrypt_managed(&raw_id, "id") {
                Ok(plain) => plain,
                Err(e) => {
                    tracing::warn!(error = %e, "skip persisting managed model (bad id)");
                    continue;
                }
            }
        } else {
            raw_id
        };
        // Routing model id must stay ENC on disk for managed entries.
        if m.managed == Some(true) || is_enc(&m.model) {
            if !is_enc(&m.model) {
                tracing::warn!(
                    id = %id,
                    "managed model routing id is not ENC(...); refusing to persist plaintext"
                );
                continue;
            }
        }
        if let Some(existing) = model_table.get(&id).and_then(|v| v.as_table()) {
            let managed = existing
                .get("managed")
                .and_then(|x| x.as_bool())
                .unwrap_or(false);
            if !managed {
                // User-authored — do not overwrite.
                continue;
            }
        }
        let mut entry = toml::map::Map::new();
        // Persist ENC form as received; never write decrypted plaintext.
        entry.insert("model".into(), toml::Value::String(m.model.clone()));
        if let Some(ref name) = m.name {
            entry.insert("name".into(), toml::Value::String(name.clone()));
        }
        if let Some(ref desc) = m.description {
            entry.insert("description".into(), toml::Value::String(desc.clone()));
        }
        entry.insert("base_url".into(), toml::Value::String(m.base_url.clone()));
        entry.insert(
            "api_backend".into(),
            toml::Value::String(api_backend_str(&m.api_backend).to_string()),
        );
        entry.insert(
            "context_window".into(),
            toml::Value::Integer(m.context_window.get() as i64),
        );
        if let Some(ref key) = m.api_key {
            if is_enc(key) || !key.is_empty() {
                entry.insert("api_key".into(), toml::Value::String(key.clone()));
            }
        }
        entry.insert("managed".into(), toml::Value::Boolean(true));
        model_table.insert(id, toml::Value::Table(entry));
    }

    if let Some(parent) = config_path.parent() {
        std::fs::create_dir_all(parent).map_err(|e| e.to_string())?;
    }
    let serialized = toml::to_string_pretty(&root).map_err(|e| e.to_string())?;
    std::fs::write(&config_path, serialized).map_err(|e| e.to_string())?;
    Ok(())
}

fn api_backend_str(b: &ApiBackend) -> &'static str {
    match b {
        ApiBackend::ChatCompletions => "chat_completions",
        ApiBackend::Responses => "responses",
        ApiBackend::Messages => "messages",
    }
}
