//! Enterprise password grant for machines with no browser.
//!
//! The TUI collects email, password, and the account machine code, then this
//! module posts `grant_type=password` to the configured OAuth issuer. A success
//! response is the same token payload as device-code approval and is stored
//! with [`crate::device_code::build_auth`].

use std::sync::Arc;

use crate::device_code::{TokenErr, TokenOk, build_auth};
use crate::oidc::with_alpha_test_key;
use crate::{AuthManager, GrokAuth};

/// Credentials typed on the TUI login form. Debug output redacts the secrets.
pub struct PasswordCredentials {
    pub email: String,
    pub password: String,
    pub machine_code: String,
}

impl std::fmt::Debug for PasswordCredentials {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("PasswordCredentials")
            .field("email", &self.email)
            .field("password", &"<redacted>")
            .field("machine_code", &"<redacted>")
            .finish()
    }
}

/// Exchange the form for a session and write `auth.json`.
///
/// Customer OIDC configs have no enterprise password endpoint; that case
/// returns an error instead of opening a browser.
pub async fn run_password_login(
    auth_manager: &Arc<AuthManager>,
    creds: PasswordCredentials,
) -> anyhow::Result<GrokAuth> {
    let Some(oauth2) = auth_manager.grok_com_config().oauth2.clone() else {
        anyhow::bail!("密码登录需要企业 OAuth issuer，当前配置走的是客户 OIDC");
    };
    let issuer = oauth2.issuer.trim_end_matches('/').to_string();
    let client_id = oauth2.client_id;
    let scope = oauth2.scopes.join(" ");
    let token_url = format!("{issuer}/oauth2/token");
    let email = creds.email.trim().to_string();
    let machine_code = creds.machine_code.trim().to_string();
    let password = creds.password;

    let client = xai_grok_http::shared_client();
    let resp = with_alpha_test_key(
        client
            .post(&token_url)
            .header("x-grok-client-version", xai_grok_version::VERSION)
            .header("x-grok-client-surface", "ui")
            .form(&[
                ("grant_type", "password"),
                ("client_id", client_id.as_str()),
                ("username", email.as_str()),
                ("password", password.as_str()),
                ("machine_code", machine_code.as_str()),
                ("scope", scope.as_str()),
            ]),
        &token_url,
    )
    .send()
    .await?;
    drop(password);

    if resp.status().is_success() {
        let tokens: TokenOk = resp.json().await?;
        return build_auth(&tokens, &issuer, &client_id, auth_manager).await;
    }

    let status = resp.status();
    let err = resp.json::<TokenErr>().await.unwrap_or(TokenErr {
        error: "invalid_grant".to_string(),
        error_description: None,
    });
    let detail = err
        .error_description
        .filter(|text| !text.is_empty())
        .unwrap_or(err.error);
    tracing::warn!(status = %status, "password grant rejected");
    anyhow::bail!("{detail}")
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::GrokComConfig;

    fn unsigned_id_token() -> String {
        "eyJhbGciOiJub25lIiwidHlwIjoiSldUIn0.eyJzdWIiOiJ1c2VyLTEyMyIsImVtYWlsIjoiZGV2aWNlLWF1dGhAbG9jYWwudGVzdCJ9.sig".to_string()
    }

    #[tokio::test]
    async fn password_login_persists_oidc_session() {
        let server = wiremock::MockServer::start().await;
        wiremock::Mock::given(wiremock::matchers::method("POST"))
            .and(wiremock::matchers::path("/oauth2/token"))
            .respond_with(
                wiremock::ResponseTemplate::new(200).set_body_json(serde_json::json!({
                    "access_token": "access-token",
                    "refresh_token": "refresh-token",
                    "expires_in": 3600,
                    "token_type": "Bearer",
                    "id_token": unsigned_id_token(),
                })),
            )
            .mount(&server)
            .await;

        let temp_dir = tempfile::tempdir().unwrap();
        let grok_home = temp_dir.path().join(".atlas");
        std::fs::create_dir_all(&grok_home).unwrap();
        let mut cfg = GrokComConfig::default();
        cfg.oauth2.as_mut().expect("oauth2").issuer = server.uri();
        let auth_manager =
            Arc::new(AuthManager::new(&grok_home, cfg).with_proxy_base_url("http://127.0.0.1:9"));

        let auth = run_password_login(
            &auth_manager,
            PasswordCredentials {
                email: "dev@atlas.local".into(),
                password: "atlas123".into(),
                machine_code: "ABCD-EFGH".into(),
            },
        )
        .await
        .expect("password login");

        assert_eq!(auth.auth_mode, crate::AuthMode::Oidc);
        assert_eq!(auth.refresh_token.as_deref(), Some("refresh-token"));
        assert!(auth_manager.current().is_some());
        let raw = std::fs::read_to_string(grok_home.join("auth.json")).unwrap();
        assert!(raw.contains("refresh-token"));
        assert!(!raw.contains("atlas123"));
    }

    #[tokio::test]
    async fn password_login_surfaces_server_message() {
        let server = wiremock::MockServer::start().await;
        wiremock::Mock::given(wiremock::matchers::method("POST"))
            .and(wiremock::matchers::path("/oauth2/token"))
            .respond_with(
                wiremock::ResponseTemplate::new(400).set_body_json(serde_json::json!({
                    "error": "invalid_grant",
                    "error_description": "邮箱、密码或机器码不正确",
                })),
            )
            .mount(&server)
            .await;

        let temp_dir = tempfile::tempdir().unwrap();
        let mut cfg = GrokComConfig::default();
        cfg.oauth2.as_mut().expect("oauth2").issuer = server.uri();
        let auth_manager = Arc::new(
            AuthManager::new(temp_dir.path(), cfg).with_proxy_base_url("http://127.0.0.1:9"),
        );
        let err = run_password_login(
            &auth_manager,
            PasswordCredentials {
                email: "dev@atlas.local".into(),
                password: "nope".into(),
                machine_code: "ABCD-EFGH".into(),
            },
        )
        .await
        .expect_err("rejected grant");
        assert!(err.to_string().contains("邮箱、密码或机器码不正确"));
        let rendered = format!(
            "{:?}",
            PasswordCredentials {
                email: "dev@atlas.local".into(),
                password: "nope".into(),
                machine_code: "ABCD-EFGH".into(),
            }
        );
        assert!(!rendered.contains("nope"));
        assert!(!rendered.contains("ABCD-EFGH"));
    }
}
