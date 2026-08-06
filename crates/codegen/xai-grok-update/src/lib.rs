pub mod auto_update;
pub mod version;
mod version_policy;

pub use auto_update::UpdateStatus;
pub use version::{
    UpdateConfig, channel_label, channel_name, resolve_cli_base_urls, write_version_cache,
    DEFAULT_CLI_BASE_URL,
};
pub use version_policy::enforce_version_policy_or_exit;
