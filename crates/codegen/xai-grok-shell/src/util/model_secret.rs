//! ENC(...) AES-256-GCM helpers for managed model fields (api_key, model id).
//!
//! Wire format matches atlas-server `internal/crypto`: `ENC(<base64(nonce||ciphertext||tag)>)`.
//! Key material is SHA-256 of the shared secret string.

use base64::Engine;
use ring::aead::{Aad, LessSafeKey, Nonce, UnboundKey, AES_256_GCM, NONCE_LEN};
use sha2::{Digest, Sha256};

/// Baked-in default shared with atlas-server (`DefaultModelSecret`).
pub const DEFAULT_MODEL_SECRET: &str = "atlas-managed-model-secret-v1";

const ENC_PREFIX: &str = "ENC(";
const ENC_SUFFIX: &str = ")";

/// Resolve decrypt secret: `GROK_MODEL_SECRET_KEY` > optional config > baked-in default.
pub fn resolve_model_secret(config_secret: Option<&str>) -> String {
    if let Ok(v) = std::env::var("GROK_MODEL_SECRET_KEY") {
        let t = v.trim();
        if !t.is_empty() {
            return t.to_owned();
        }
    }
    if let Some(v) = config_secret.map(str::trim).filter(|s| !s.is_empty()) {
        return v.to_owned();
    }
    DEFAULT_MODEL_SECRET.to_owned()
}

pub fn is_enc(s: &str) -> bool {
    let s = s.trim();
    s.starts_with(ENC_PREFIX) && s.ends_with(ENC_SUFFIX)
}

fn derive_key(secret: &str) -> [u8; 32] {
    let mut hasher = Sha256::new();
    hasher.update(secret.as_bytes());
    hasher.finalize().into()
}

/// Decrypt `ENC(...)` or return plaintext unchanged.
pub fn decrypt_field(enc_or_plain: &str, secret: &str) -> Result<String, String> {
    let s = enc_or_plain.trim();
    if s.is_empty() {
        return Ok(String::new());
    }
    if !is_enc(s) {
        return Ok(s.to_owned());
    }
    let payload = &s[ENC_PREFIX.len()..s.len() - ENC_SUFFIX.len()];
    let raw = base64::engine::general_purpose::STANDARD
        .decode(payload)
        .map_err(|e| format!("invalid ENC payload: {e}"))?;
    if raw.len() < NONCE_LEN {
        return Err("ENC payload too short".into());
    }
    let key = LessSafeKey::new(
        UnboundKey::new(&AES_256_GCM, &derive_key(secret))
            .map_err(|_| "invalid AES key".to_string())?,
    );
    let mut nonce_bytes = [0u8; NONCE_LEN];
    nonce_bytes.copy_from_slice(&raw[..NONCE_LEN]);
    let nonce = Nonce::assume_unique_for_key(nonce_bytes);
    let mut in_out = raw[NONCE_LEN..].to_vec();
    let plain = key
        .open_in_place(nonce, Aad::empty(), &mut in_out)
        .map_err(|_| "decrypt failed (wrong key?)".to_string())?;
    String::from_utf8(plain.to_vec()).map_err(|_| "decrypted value is not utf-8".into())
}

/// Alias kept for call sites that decrypt API keys.
pub fn decrypt_api_key(enc_or_plain: &str, secret: &str) -> Result<String, String> {
    decrypt_field(enc_or_plain, secret)
}

/// Decrypt if needed; on failure log and return None.
pub fn maybe_decrypt_api_key(api_key: Option<String>) -> Option<String> {
    maybe_decrypt_field(api_key, "api_key")
}

/// Decrypt optional field; plaintext passthrough; ENC failure → None.
pub fn maybe_decrypt_field(value: Option<String>, field: &str) -> Option<String> {
    let Some(raw) = value.filter(|s| !s.trim().is_empty()) else {
        return None;
    };
    if !is_enc(&raw) {
        return Some(raw);
    }
    let secret = resolve_model_secret(None);
    match decrypt_field(&raw, &secret) {
        Ok(plain) => Some(plain),
        Err(e) => {
            tracing::warn!(error = %e, field, "failed to decrypt managed model field");
            None
        }
    }
}

/// Managed catalog fields must be `ENC(...)` and decrypt successfully.
pub fn require_decrypt_managed(enc: &str, field: &str) -> Result<String, String> {
    let s = enc.trim();
    if s.is_empty() {
        return Err(format!("managed {field} is empty"));
    }
    if !is_enc(s) {
        return Err(format!(
            "managed {field} must be ENC(...); plaintext rejected (possible tamper)"
        ));
    }
    let secret = resolve_model_secret(None);
    decrypt_field(s, &secret).map_err(|e| format!("managed {field}: {e}"))
}

/// Decrypt a string in place for catalog use.
/// - `require_enc`: managed entries — plaintext rejected
/// - otherwise: ENC decrypts, plaintext passthrough
pub fn resolve_catalog_string(raw: &str, require_enc: bool, field: &str) -> Result<String, String> {
    if require_enc {
        require_decrypt_managed(raw, field)
    } else if is_enc(raw) {
        let secret = resolve_model_secret(None);
        decrypt_field(raw, &secret)
    } else {
        Ok(raw.to_owned())
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use ring::aead::{Aad, LessSafeKey, Nonce, UnboundKey, AES_256_GCM, NONCE_LEN};
    use ring::rand::{SecureRandom, SystemRandom};

    fn encrypt_like_server(plaintext: &str, secret: &str) -> String {
        let key = LessSafeKey::new(UnboundKey::new(&AES_256_GCM, &derive_key(secret)).unwrap());
        let rng = SystemRandom::new();
        let mut nonce_bytes = [0u8; NONCE_LEN];
        rng.fill(&mut nonce_bytes).unwrap();
        let nonce = Nonce::assume_unique_for_key(nonce_bytes);
        let mut in_out = plaintext.as_bytes().to_vec();
        key.seal_in_place_append_tag(nonce, Aad::empty(), &mut in_out)
            .unwrap();
        let mut packed = nonce_bytes.to_vec();
        packed.extend_from_slice(&in_out);
        format!(
            "ENC({})",
            base64::engine::general_purpose::STANDARD.encode(packed)
        )
    }

    #[test]
    fn decrypt_roundtrip_matches_server_format() {
        let enc = encrypt_like_server("sk-test", DEFAULT_MODEL_SECRET);
        assert!(is_enc(&enc));
        let plain = decrypt_api_key(&enc, DEFAULT_MODEL_SECRET).unwrap();
        assert_eq!(plain, "sk-test");
    }

    #[test]
    fn plaintext_passthrough() {
        assert_eq!(
            decrypt_api_key("sk-plain", DEFAULT_MODEL_SECRET).unwrap(),
            "sk-plain"
        );
    }

    #[test]
    fn managed_model_id_requires_enc() {
        assert!(require_decrypt_managed("kimi-for-coding", "model").is_err());
        let enc = encrypt_like_server("kimi-for-coding", DEFAULT_MODEL_SECRET);
        assert_eq!(
            require_decrypt_managed(&enc, "model").unwrap(),
            "kimi-for-coding"
        );
    }
}
