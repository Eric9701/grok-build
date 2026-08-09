//! `/atlas-sdd` (alias `/sdd`) — pick an atlas-sdd role agent and inject a
//! Task-spawn instruction for the model.
//!
//! Requires the `atlas-sdd` plugin to be installed so the subagent type
//! resolves. Arg completion lists the eight role agents.

use agent_client_protocol as acp;

use crate::slash::command::{AppCtx, ArgItem, CommandExecCtx, CommandResult, SlashCommand};

/// Role agents shipped by the `atlas-sdd` plugin (`subagent_type` = `atlas-sdd:<id>`).
const SDD_AGENTS: &[(&str, &str)] = &[
    (
        "1-requirement-analyst-agent",
        "Requirements analysis and clarification",
    ),
    (
        "2-architect-design-agent",
        "IT / system architecture design",
    ),
    (
        "3-program-design-agent",
        "Detailed / program design documents",
    ),
    (
        "4-software-engineer-agent",
        "Implement code from design docs",
    ),
    (
        "5-hardware-engineer-agent",
        "Embedded / hardware implementation",
    ),
    ("6-qa-engineer-agent", "QA / test design and automation"),
    ("7-ops-engineer-agent", "Ops / deploy / incident response"),
    ("8-data-engineer-agent", "Data / SQL / warehouse work"),
];

pub struct AtlasSddCommand;

impl SlashCommand for AtlasSddCommand {
    fn name(&self) -> &str {
        "atlas-sdd"
    }

    fn aliases(&self) -> &[&str] {
        &["sdd"]
    }

    fn description(&self) -> &str {
        "Run an atlas-sdd role agent via Task"
    }

    fn usage(&self) -> &str {
        "/atlas-sdd <agent> <prompt>"
    }

    fn takes_args(&self) -> bool {
        true
    }

    fn args_required(&self) -> bool {
        true
    }

    fn arg_placeholder(&self) -> Option<&str> {
        Some("<agent> <prompt>")
    }

    fn session_scoped(&self) -> bool {
        true
    }

    fn suggest_args(&self, _ctx: &AppCtx, args_query: &str) -> Option<Vec<ArgItem>> {
        // Once an agent id is fully selected (followed by space), free-form prompt.
        if agent_selected(args_query).is_some() {
            return None;
        }
        let q = args_query.trim().to_ascii_lowercase();
        let items: Vec<ArgItem> = SDD_AGENTS
            .iter()
            .filter(|(id, desc)| {
                q.is_empty()
                    || id.to_ascii_lowercase().contains(&q)
                    || desc.to_ascii_lowercase().contains(&q)
            })
            .map(|(id, desc)| ArgItem {
                display: (*id).to_string(),
                match_text: format!("{id} {desc}"),
                insert_text: (*id).to_string(),
                description: (*desc).to_string(),
            })
            .collect();
        if items.is_empty() {
            None
        } else {
            Some(items)
        }
    }

    fn run(&self, _ctx: &mut CommandExecCtx, args: &str) -> CommandResult {
        let trimmed = args.trim();
        if trimmed.is_empty() {
            return CommandResult::Error(usage_help());
        }

        let Some((agent_id, prompt)) = split_agent_and_prompt(trimmed) else {
            return CommandResult::Error(usage_help());
        };

        let prompt = prompt.trim();
        if prompt.is_empty() {
            return CommandResult::Error(format!(
                "Missing task prompt.\nUsage: /atlas-sdd {agent_id} <prompt>"
            ));
        }

        let subagent_type = format!("atlas-sdd:{agent_id}");
        let instruction = format!(
            "The user selected an atlas-sdd role agent via /atlas-sdd.\n\
             \n\
             You MUST call the Task tool exactly once with:\n\
             - subagent_type: `{subagent_type}`\n\
             - description: a short 3-5 word summary of the task\n\
             - prompt: the user task below (do not rewrite goals)\n\
             - run_in_background: false (wait for the subagent unless the user asked otherwise)\n\
             \n\
             Do not use general-purpose or any other subagent_type for this turn.\n\
             \n\
             <user_task>\n{prompt}\n</user_task>"
        );

        CommandResult::InjectSkill {
            display_text: format!("/atlas-sdd {agent_id} {prompt}"),
            prompt_blocks: vec![acp::ContentBlock::Text(acp::TextContent::new(instruction))],
            display_as_skill: false,
            scheduled_task_preview: None,
        }
    }
}

fn usage_help() -> String {
    let mut s = String::from("Usage: /atlas-sdd <agent> <prompt>\nAgents:\n");
    for (id, desc) in SDD_AGENTS {
        s.push_str(&format!("  {id}  — {desc}\n"));
    }
    s.push_str("Requires the atlas-sdd plugin to be installed.");
    s
}

/// True when the first token fully matches a known agent and is followed by
/// whitespace (prompt phase).
fn agent_selected(args_query: &str) -> Option<&'static str> {
    let trimmed = args_query.trim_start();
    let (first, rest) = match trimmed.split_once(char::is_whitespace) {
        Some((a, b)) => (a, b),
        None => return None,
    };
    if rest.is_empty() && !args_query.ends_with(char::is_whitespace) {
        // Still typing the agent id (no trailing space yet).
        return None;
    }
    resolve_agent_id(first)
}

fn split_agent_and_prompt(args: &str) -> Option<(&'static str, &str)> {
    let trimmed = args.trim();
    let (first, rest) = trimmed
        .split_once(char::is_whitespace)
        .map(|(a, b)| (a, b.trim_start()))
        .unwrap_or((trimmed, ""));
    let id = resolve_agent_id(first)?;
    Some((id, rest))
}

fn resolve_agent_id(token: &str) -> Option<&'static str> {
    let t = token.trim();
    if t.is_empty() {
        return None;
    }
    // Accept bare id, numeric prefix ("1"), or plugin-qualified form.
    let bare = t
        .strip_prefix("atlas-sdd:")
        .unwrap_or(t);
    for (id, _) in SDD_AGENTS {
        if bare.eq_ignore_ascii_case(id) {
            return Some(*id);
        }
        // Allow "/atlas-sdd 1 …" → 1-requirement-analyst-agent
        if let Some(num) = id.split('-').next() {
            if bare == num {
                return Some(*id);
            }
        }
    }
    None
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn resolves_full_id_and_numeric_prefix() {
        assert_eq!(
            resolve_agent_id("1-requirement-analyst-agent"),
            Some("1-requirement-analyst-agent")
        );
        assert_eq!(
            resolve_agent_id("atlas-sdd:4-software-engineer-agent"),
            Some("4-software-engineer-agent")
        );
        assert_eq!(resolve_agent_id("1"), Some("1-requirement-analyst-agent"));
        assert_eq!(resolve_agent_id("nope"), None);
    }

    #[test]
    fn split_requires_known_agent() {
        let (id, prompt) = split_agent_and_prompt("1 analyze login").unwrap();
        assert_eq!(id, "1-requirement-analyst-agent");
        assert_eq!(prompt, "analyze login");
        assert!(split_agent_and_prompt("unknown do stuff").is_none());
    }

    #[test]
    fn agent_selected_only_after_trailing_space_or_prompt() {
        assert!(agent_selected("1-requirement").is_none());
        assert!(agent_selected("1-requirement-analyst-agent").is_none());
        assert_eq!(
            agent_selected("1-requirement-analyst-agent "),
            Some("1-requirement-analyst-agent")
        );
        assert_eq!(
            agent_selected("1 analyze"),
            Some("1-requirement-analyst-agent")
        );
    }
}
