//! `x.ai/models/list` and `x.ai/models/refresh`.

use agent_client_protocol::{self as acp};

use super::super::mvp_agent::MvpAgent;
use crate::session::ExtMethodResult;

/// Model state, after a bounded wait for the first catalog. Process chat
/// mode serves the chat catalog, as `initialize` does.
pub(crate) async fn handle(
    agent: &MvpAgent,
    _args: &acp::ExtRequest,
) -> Result<acp::ExtResponse, acp::Error> {
    let state = if crate::agent::chat_modes::process_chat_mode_enabled() {
        agent.chat_modes.model_state().await
    } else {
        agent.models_manager.wait_for_first_catalog().await;
        agent.model_state(None)
    };
    ExtMethodResult::success(state)
        .to_ext_response()
        .map_err(|e| acp::Error::internal_error().data(e.to_string()))
}

/// Invalidate the disk cache and fetch `/v1/models` now, then return the
/// updated catalog. Chat-mode processes keep serving `/rest/modes`.
pub(crate) async fn handle_refresh(
    agent: &MvpAgent,
    args: &acp::ExtRequest,
) -> Result<acp::ExtResponse, acp::Error> {
    if crate::agent::chat_modes::process_chat_mode_enabled() {
        return handle(agent, args).await;
    }
    match agent.models_manager.force_refresh().await {
        Ok(_) => ExtMethodResult::success(agent.model_state(None))
            .to_ext_response()
            .map_err(|e| acp::Error::internal_error().data(e.to_string())),
        Err(e) => ExtMethodResult::<acp::SessionModelState>::failure(e)
            .to_ext_response()
            .map_err(|e| acp::Error::internal_error().data(e.to_string())),
    }
}
