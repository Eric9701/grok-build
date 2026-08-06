//! Best-effort task/agent/artifact reporting to atlas-server.
//!
//! Used for both subagent completion and primary-session turn completion.

use std::sync::Arc;

use crate::auth::AuthManager;
use crate::remote::{TaskReport, post_task_report};

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
