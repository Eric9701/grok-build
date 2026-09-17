//! Best-effort task/agent/artifact reporting to atlas-server.
//!
//! Used for both subagent completion and primary-session turn completion.

use std::collections::HashMap;
use std::sync::Arc;

use crate::auth::{AuthManager, GrokAuth};
use crate::remote::{ArtifactLineAdd, TaskReport, post_task_report};
use xai_grok_tools::types::output::{ApplyPatchOutput, SearchReplaceOutput, ToolOutput};

/// Convert a per-path inserted-line map into the Task Report payload.
pub(crate) fn artifact_line_adds_payload(map: &HashMap<String, u64>) -> Vec<ArtifactLineAdd> {
    let mut out: Vec<ArtifactLineAdd> = map
        .iter()
        .filter(|(_, n)| **n > 0)
        .map(|(path, lines_added)| ArtifactLineAdd {
            path: path.clone(),
            lines_added: *lines_added,
        })
        .collect();
    out.sort_by(|a, b| a.path.cmp(&b.path));
    out
}

/// Inserted-line counts from a successful write/edit/apply_patch tool output.
///
/// Prefers the `edit.lines` counts stored on the tool output (computed once
/// at apply time). Falls back to `line_diff` only when the producer omitted them.
pub(crate) fn artifact_line_adds_from_output(output: &ToolOutput) -> Vec<(String, u64)> {
    match output {
        ToolOutput::SearchReplace(SearchReplaceOutput::EditsApplied(applied)) => {
            vec![(
                applied.absolute_path.to_string_lossy().into_owned(),
                applied.inserted_line_count(),
            )]
        }
        ToolOutput::ApplyPatch(ApplyPatchOutput::Success { files, .. }) => files
            .iter()
            .map(|f| {
                let dest = f.move_to.as_ref().unwrap_or(&f.path);
                (dest.to_string_lossy().into_owned(), f.inserted_line_count())
            })
            .collect(),
        _ => Vec::new(),
    }
}

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
    use crate::remote::ArtifactLineAdd;

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
            artifact_lines_added: vec![],
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

    #[test]
    fn artifact_line_adds_accumulate_and_omit_zeros() {
        let mut map = HashMap::new();
        map.insert("src/lib.rs".into(), 10);
        map.insert("src/lib.rs".into(), 10); // HashMap last write, not accumulate
        map.insert("docs/note.md".into(), 0);
        map.insert("src/main.rs".into(), 3);
        let got = artifact_line_adds_payload(&map);
        assert_eq!(
            got,
            vec![
                ArtifactLineAdd {
                    path: "src/lib.rs".into(),
                    lines_added: 10,
                },
                ArtifactLineAdd {
                    path: "src/main.rs".into(),
                    lines_added: 3,
                },
            ]
        );
    }

    #[test]
    fn search_replace_output_counts_inserted_lines() {
        use xai_grok_tools::types::output::{
            SearchReplaceEditContextInformation, SearchReplaceEditDetail, SearchReplaceEditsApplied,
        };
        let output = ToolOutput::SearchReplace(SearchReplaceOutput::EditsApplied(
            SearchReplaceEditsApplied {
                old_string: "a\n".into(),
                new_string: "a\nb\nc\n".into(),
                tool_output_for_prompt: String::new(),
                tool_output_for_prompt_concise: None,
                absolute_path: std::path::PathBuf::from("src/lib.rs"),
                edits: SearchReplaceEditContextInformation {
                    details: vec![SearchReplaceEditDetail {
                        old_string: "a\n".into(),
                        old_line: 1,
                        new_string: "a\nb\nc\n".into(),
                        new_line: 1,
                        context_before: String::new(),
                        context_after: String::new(),
                        line_prefix: String::new(),
                    }],
                },
                patch: None,
                unicode_normalized: false,
                lines_added: None,
                lines_removed: None,
            },
        ));
        let got = artifact_line_adds_from_output(&output);
        assert_eq!(got, vec![("src/lib.rs".into(), 2)]);
    }

    #[test]
    fn search_replace_reuses_stored_edit_lines_without_details() {
        use xai_grok_tools::types::output::{
            SearchReplaceEditContextInformation, SearchReplaceEditsApplied,
        };
        let output = ToolOutput::SearchReplace(SearchReplaceOutput::EditsApplied(
            SearchReplaceEditsApplied {
                old_string: String::new(),
                new_string: String::new(),
                tool_output_for_prompt: String::new(),
                tool_output_for_prompt_concise: None,
                absolute_path: std::path::PathBuf::from("src/lib.rs"),
                edits: SearchReplaceEditContextInformation { details: vec![] },
                patch: None,
                unicode_normalized: false,
                lines_added: Some(7),
                lines_removed: Some(1),
            },
        ));
        let got = artifact_line_adds_from_output(&output);
        assert_eq!(got, vec![("src/lib.rs".into(), 7)]);
    }

    #[test]
    fn apply_patch_new_file_counts_all_lines() {
        use xai_grok_tools::types::output::ApplyPatchFileResult;
        let output = ToolOutput::ApplyPatch(ApplyPatchOutput::Success {
            files: vec![ApplyPatchFileResult {
                path: std::path::PathBuf::from("src/new.rs"),
                action: "added".into(),
                old_text: None,
                new_text: "fn a() {}\nfn b() {}\n".into(),
                move_to: None,
                lines_added: None,
                lines_removed: None,
            }],
            tool_output_for_prompt: String::new(),
        });
        let got = artifact_line_adds_from_output(&output);
        assert_eq!(got, vec![("src/new.rs".into(), 2)]);
    }

    #[test]
    fn apply_patch_reuses_stored_edit_lines() {
        use xai_grok_tools::types::output::ApplyPatchFileResult;
        let output = ToolOutput::ApplyPatch(ApplyPatchOutput::Success {
            files: vec![ApplyPatchFileResult {
                path: std::path::PathBuf::from("src/new.rs"),
                action: "added".into(),
                old_text: None,
                new_text: String::new(),
                move_to: None,
                lines_added: Some(4),
                lines_removed: Some(0),
            }],
            tool_output_for_prompt: String::new(),
        });
        let got = artifact_line_adds_from_output(&output);
        assert_eq!(got, vec![("src/new.rs".into(), 4)]);
    }
}
