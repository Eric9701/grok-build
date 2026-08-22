//! Best-effort task/agent/artifact reporting to atlas-server.
//!
//! Used for both subagent completion and primary-session turn completion.

use std::sync::Arc;

use crate::auth::{AuthManager, GrokAuth};
use crate::remote::{TaskReport, post_task_report};

/// Fill Report User and Client Version on a Task Report from the live session.
///
/// Does not overwrite fields the caller already set. Missing auth leaves
/// user fields empty; the server stores `anonymous`.
pub(crate) fn attach_report_attribution(report: &mut TaskReport, auth: Option<&GrokAuth>) {
    if report.client_version.is_none() {
        report.client_version = Some(xai_grok_version::VERSION.to_string());
    }
    let Some(auth) = auth else {
        return;
    };
    if report.user_id.is_none() && !auth.user_id.is_empty() {
        report.user_id = Some(auth.user_id.clone());
    }
    if report.email.is_none() {
        if let Some(email) = auth
            .email
            .as_deref()
            .map(str::trim)
            .filter(|e| !e.is_empty())
        {
            report.email = Some(email.to_string());
        }
    }
}

/// Opt-out via `GROK_DISABLE_TASK_REPORT=1|true|on|yes`.
pub(crate) fn task_reporting_enabled() -> bool {
    !matches!(
        std::env::var("GROK_DISABLE_TASK_REPORT").ok().as_deref(),
        Some("1") | Some("true") | Some("on") | Some("yes")
    )
}

/// Truncate `text` to at most `cap` bytes on a char boundary.
pub(crate) fn truncate_on_boundary(mut text: String, cap: usize) -> String {
    if text.len() > cap {
        let mut end = cap;
        while end > 0 && !text.is_char_boundary(end) {
            end -= 1;
        }
        text.truncate(end);
    }
    text
}

/// Fire-and-forget POST of a [`TaskReport`]. Never fails the caller.
pub(crate) fn spawn_task_report(
    base_url: String,
    auth_manager: Option<Arc<AuthManager>>,
    deployment_key: Option<String>,
    alpha_test_key: Option<String>,
    report: TaskReport,
) {
    if base_url.is_empty() {
        xai_grok_telemetry::unified_log::warn(
            "skip task report: empty proxy base url",
            Some(report.parent_session_id.as_str()),
            Some(serde_json::json!({ "subagent_id": &report.subagent_id })),
        );
        return;
    }
    let parent_sid = report.parent_session_id.clone();
    let subagent_id = report.subagent_id.clone();
    let subagent_type = report.subagent_type.clone();
    tokio::spawn(async move {
        let mut report = report;
        attach_report_attribution(
            &mut report,
            auth_manager
                .as_ref()
                .and_then(|am| am.current_or_expired())
                .as_ref(),
        );
        let url = format!("{}/task-reports", base_url.trim_end_matches('/'));
        let prompt_len = report.prompt.as_ref().map(|p| p.len()).unwrap_or(0);
        xai_grok_telemetry::unified_log::info(
            "posting task report",
            Some(parent_sid.as_str()),
            Some(serde_json::json!({
                "subagent_id": &subagent_id,
                "subagent_type": &subagent_type,
                "url": &url,
                "artifacts": report.artifact_count,
                "status": &report.status,
                "tokens_used": report.tokens_used,
                "prompt_len": prompt_len,
            })),
        );
        match post_task_report(
            &base_url,
            auth_manager.as_ref(),
            deployment_key.as_deref(),
            alpha_test_key.as_deref(),
            &report,
        )
        .await
        {
            Ok(()) => {
                xai_grok_telemetry::unified_log::info(
                    "task report accepted",
                    Some(parent_sid.as_str()),
                    Some(serde_json::json!({
                        "subagent_id": &subagent_id,
                        "url": &url,
                    })),
                );
            }
            Err(e) => {
                xai_grok_telemetry::unified_log::warn(
                    "task report post failed",
                    Some(parent_sid.as_str()),
                    Some(serde_json::json!({
                        "subagent_id": &subagent_id,
                        "url": &url,
                        "error": e.to_string(),
                    })),
                );
            }
        }
    });
}

#[cfg(test)]
mod tests {
    use super::*;

    fn blank_report() -> TaskReport {
        TaskReport {
            subagent_id: "sa".into(),
            parent_session_id: "p".into(),
            child_session_id: "c".into(),
            subagent_type: "explore".into(),
            model: None,
            model_routing: None,
            description: "d".into(),
            prompt: None,
            status: "completed".into(),
            success: true,
            duration_ms: 1,
            tool_calls: 0,
            turns: 1,
            tokens_used: 0,
            artifacts: vec![],
            artifact_count: 0,
            cwd: None,
            worktree_path: None,
            error: None,
            started_at: "t0".into(),
            completed_at: "t1".into(),
            user_id: None,
            email: None,
            client_version: None,
        }
    }

    #[test]
    fn attach_fills_report_user_and_client_version() {
        let auth = GrokAuth {
            user_id: "u-42".into(),
            email: Some("dev@atlas.local".into()),
            ..GrokAuth::default()
        };
        let mut report = blank_report();
        attach_report_attribution(&mut report, Some(&auth));
        assert_eq!(report.user_id.as_deref(), Some("u-42"));
        assert_eq!(report.email.as_deref(), Some("dev@atlas.local"));
        assert_eq!(
            report.client_version.as_deref(),
            Some(xai_grok_version::VERSION)
        );
    }

    #[test]
    fn attach_without_auth_still_sets_client_version() {
        let mut report = blank_report();
        attach_report_attribution(&mut report, None);
        assert!(report.user_id.is_none());
        assert!(report.email.is_none());
        assert_eq!(
            report.client_version.as_deref(),
            Some(xai_grok_version::VERSION)
        );
    }

    #[test]
    fn task_report_json_uses_camel_case_identity_fields() {
        let mut report = blank_report();
        report.user_id = Some("u-42".into());
        report.email = Some("dev@atlas.local".into());
        report.client_version = Some("0.2.121".into());
        let v = serde_json::to_value(&report).unwrap();
        assert_eq!(v["userId"], "u-42");
        assert_eq!(v["email"], "dev@atlas.local");
        assert_eq!(v["clientVersion"], "0.2.121");
    }

    #[test]
    fn task_report_json_uses_camel_case_model_routing() {
        let mut report = blank_report();
        report.model = Some("arch-qwen3.8-max".into());
        report.model_routing = Some("qwen3.8-max".into());
        let v = serde_json::to_value(&report).unwrap();
        assert_eq!(v["model"], "arch-qwen3.8-max");
        assert_eq!(v["modelRouting"], "qwen3.8-max");
        assert!(v.get("model_routing").is_none());
    }
}

