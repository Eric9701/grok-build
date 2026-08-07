use anyhow::{Context, bail};
use std::env;
use std::path::{Path, PathBuf};
use std::process::Command;

fn check_protoc_good(protoc: &Path) -> anyhow::Result<()> {
    let output = Command::new(protoc)
        .arg("--version")
        .output()
        .context("Failed to execute protoc")?;

    if !output.status.success() {
        let stdout = String::from_utf8_lossy(&output.stdout);
        let stderr = String::from_utf8_lossy(&output.stderr);
        bail!(
            "protoc --version failed, likely dotslash is missing; \
             try `cargo install dotslash`; stdout: {stdout:?}, stderr: {stderr:?}"
        );
    }
    Ok(())
}

fn is_github_actions() -> bool {
    env::var_os("GITHUB_ACTIONS").is_some()
}

/// Candidate relative paths under the repo `bin/` tree, in preference order.
///
/// `bin/protoc` is a Unix/macOS [dotslash](https://dotslash-cli.com/) wrapper and is
/// not a valid Win32 PE (os error 193 on Windows). Windows local builds instead use
/// the unpacked zip at `bin/protoc-win64/bin/protoc.exe`.
fn local_protoc_candidates() -> &'static [&'static str] {
    if cfg!(windows) {
        &[
            "bin/protoc-win64/bin/protoc.exe",
            "bin/protoc.exe",
            "bin/protoc",
        ]
    } else {
        &["bin/protoc"]
    }
}

/// Find `protoc` command.
///
/// Search order:
/// 1. `$PROTOC` environment variable (set by Bazel `build_script_env` or user override)
/// 2. Repo-local `bin/…` walking up parent directories (`bin/protoc` on Unix; on Windows
///    prefer `bin/protoc-win64/bin/protoc.exe`)
/// 3. `protoc` on `$PATH` (system install or other tooling)
///
/// When a local candidate exists but fails to execute (e.g. the Unix dotslash wrapper
/// on Windows, or Bazel remote execution without `dotslash`), the error is not fatal —
/// we try the next candidate / PATH fallback.
///
/// Returns `Ok(None)` if not found and not in a strict environment (GitHub Actions).
pub fn find_protoc() -> anyhow::Result<Option<PathBuf>> {
    // 1. Check the PROTOC env var first. This is the standard override used by prost-build
    //    and is set by Bazel cargo_build_script build_script_env to point at a hermetic
    //    protoc binary instead of the dotslash wrapper.
    if let Ok(protoc_env) = env::var("PROTOC") {
        let protoc = PathBuf::from(&protoc_env);
        if protoc.try_exists()? {
            check_protoc_good(&protoc)?;
            return Ok(Some(protoc));
        }
    }

    // 2. Walk up directories looking for a repo-local protoc.
    let cwd = env::current_dir()?;
    let mut dir = cwd.clone();
    let mut dir_rel = PathBuf::new();
    loop {
        for candidate in local_protoc_candidates() {
            // Return relative path to make build more deterministic.
            let protoc = dir_rel.join(candidate);
            if !protoc.try_exists()? {
                continue;
            }
            match check_protoc_good(&protoc) {
                Ok(()) => return Ok(Some(protoc)),
                Err(e) => {
                    eprintln!(
                        "local protoc found at `{}` but failed to execute: {e:#}; \
                         trying next candidate / PATH fallback",
                        protoc.display()
                    );
                }
            }
        }
        if !dir.pop() {
            break;
        }
        dir_rel.push("..");
    }

    // 3. Try protoc from PATH (system install or other tooling).
    if check_protoc_good(Path::new("protoc")).is_ok() {
        return Ok(Some(PathBuf::from("protoc")));
    }

    // 4. Not found anywhere.
    if is_github_actions() {
        return Err(anyhow::anyhow!(
            "`protoc` not found (checked $PROTOC env, repo bin/, and PATH)"
        ));
    }
    eprintln!("`protoc` not found; likely it is missing in docker image");
    Ok(None)
}
