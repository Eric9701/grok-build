//! `/refresh-model` — fetch the remote model catalog immediately.

use crate::app::actions::Action;
use crate::slash::command::{CommandExecCtx, CommandResult, SlashCommand};

/// Force-refresh the model catalog from the remote server.
pub struct RefreshModelCommand;

impl SlashCommand for RefreshModelCommand {
    fn name(&self) -> &str {
        "refresh-model"
    }

    fn aliases(&self) -> &[&str] {
        &["refresh-models"]
    }

    fn description(&self) -> &str {
        "Refresh the model catalog from the remote server"
    }

    fn usage(&self) -> &str {
        "/refresh-model"
    }

    fn offered_when_session_less(&self) -> bool {
        true
    }

    fn run(&self, _ctx: &mut CommandExecCtx, _args: &str) -> CommandResult {
        CommandResult::Action(Action::RefreshModels)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn refresh_model_command_identity() {
        let cmd = RefreshModelCommand;
        assert_eq!(cmd.name(), "refresh-model");
        assert_eq!(cmd.aliases(), &["refresh-models"]);
        assert!(cmd.offered_when_session_less());
        assert_eq!(cmd.description(), "Refresh the model catalog from the remote server");
        assert_eq!(cmd.usage(), "/refresh-model");
    }
}
