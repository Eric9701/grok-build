//! `atlas models` subcommand.

use anyhow::Result;
use clap::Subcommand;
use tokio_util::sync::CancellationToken;
use xai_grok_shell::agent::config::Config as AgentConfig;
use xai_grok_shell::cli_models::{AuthStatus, list_models, refresh_models};

use crate::client_identity::{PAGER_CLIENT_TYPE, PAGER_CLIENT_VERSION};

#[derive(Debug, clap::Args, Clone, Default)]
pub struct ModelsArgs {
    #[command(subcommand)]
    pub command: Option<ModelsCommand>,
}

#[derive(Debug, Subcommand, Clone)]
pub enum ModelsCommand {
    /// List available models
    List,
    /// Fetch the model catalog from the remote server immediately
    Refresh,
}

pub async fn run(args: ModelsArgs, agent_config: &AgentConfig) -> Result<()> {
    match args.command {
        None | Some(ModelsCommand::List) => list_available_models(agent_config).await,
        Some(ModelsCommand::Refresh) => refresh_and_list(agent_config).await,
    }
}

pub async fn list_available_models(agent_config: &AgentConfig) -> Result<()> {
    print_auth_banner(agent_config);
    let state = with_models_agent(agent_config, |tx| async move {
        list_models(&tx, PAGER_CLIENT_TYPE, PAGER_CLIENT_VERSION).await
    })
    .await?;
    print_catalog(&state);
    Ok(())
}

async fn refresh_and_list(agent_config: &AgentConfig) -> Result<()> {
    print_auth_banner(agent_config);
    let state = with_models_agent(agent_config, |tx| async move {
        refresh_models(&tx, PAGER_CLIENT_TYPE, PAGER_CLIENT_VERSION).await
    })
    .await?;
    println!("Refreshed model catalog from remote.");
    println!();
    print_catalog(&state);
    Ok(())
}

fn print_auth_banner(agent_config: &AgentConfig) {
    match AuthStatus::resolve(agent_config) {
        AuthStatus::ApiKey => println!("You are using XAI_API_KEY."),
        AuthStatus::LoggedIn(host) => println!("You are logged in with {}.", host),
        AuthStatus::ModelCredentials(model) => {
            println!("Model '{model}' is using its own API key.");
        }
        AuthStatus::DeploymentKey => println!("You are authenticated via deployment key."),
        AuthStatus::NotAuthenticated => println!("You are not authenticated."),
    }
    println!();
}

fn print_catalog(state: &agent_client_protocol::SessionModelState) {
    println!("Default model: {}", state.current_model_id.0);
    println!();
    println!("Available models:");
    for m in &state.available_models {
        if m.model_id == state.current_model_id {
            println!("  * {} (default)", m.model_id.0);
        } else {
            println!("  - {}", m.model_id.0);
        }
    }
}

async fn with_models_agent<F, Fut>(
    agent_config: &AgentConfig,
    f: F,
) -> Result<agent_client_protocol::SessionModelState>
where
    F: FnOnce(xai_acp_lib::AcpAgentTx) -> Fut,
    Fut: std::future::Future<Output = Result<agent_client_protocol::SessionModelState>>,
{
    let cancel = CancellationToken::new();
    xai_grok_telemetry::startup::mark_utility_process();
    let spawned = crate::acp::spawn::spawn_grok_shell(agent_config.clone(), &cancel, None).await?;
    let _agent_guard =
        crate::acp::spawn::AgentShutdownGuard::new(cancel.clone(), Some(spawned.thread_handle));
    f(spawned.channel.tx).await
}
