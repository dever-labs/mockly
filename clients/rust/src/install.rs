use crate::types::InstallOptions;
use std::path::PathBuf;

pub const DEFAULT_VERSION: &str = concat!("v", env!("CARGO_PKG_VERSION"));
const GITHUB_BASE: &str = "https://github.com/dever-labs/mockly/releases/download";

/// Returns the path to an existing Mockly binary, or `None` if not found.
///
/// Search order:
/// 1. `MOCKLY_BINARY_PATH` env var
/// 2. `<bin_dir>/mockly[.exe]`
/// 3. `./bin/mockly[.exe]`
pub fn get_binary_path(bin_dir: Option<&str>) -> Option<PathBuf> {
    let exe = binary_name();

    // 1. Env override
    if let Ok(p) = std::env::var("MOCKLY_BINARY_PATH") {
        let path = PathBuf::from(&p);
        if path.exists() {
            return Some(path);
        }
    }

    // 2. Caller-supplied directory
    if let Some(dir) = bin_dir {
        let path = PathBuf::from(dir).join(&exe);
        if path.exists() {
            return Some(path);
        }
    }

    // 3. ./bin relative to cwd
    let default_path = PathBuf::from("bin").join(&exe);
    if default_path.exists() {
        return Some(default_path);
    }

    None
}

/// Downloads (or locates) the Mockly binary and returns its path.
///
/// Resolution order:
/// 1. `MOCKLY_BINARY_PATH` env var → return immediately
/// 2. Already installed (and `!force`) → return existing path
/// 3. `MOCKLY_NO_INSTALL` is set → return an error
/// 4. Download from GitHub releases (or `MOCKLY_DOWNLOAD_BASE_URL`)
pub fn install(opts: InstallOptions) -> Result<PathBuf, Box<dyn std::error::Error>> {
    // 1. Env-pinned binary
    if let Ok(p) = std::env::var("MOCKLY_BINARY_PATH") {
        let path = PathBuf::from(&p);
        if path.exists() {
            return Ok(path);
        }
    }

    let bin_dir = opts
        .bin_dir
        .as_deref()
        .unwrap_or("bin");

    let exe = binary_name();
    let dest = PathBuf::from(bin_dir).join(&exe);

    // 2. Already installed
    if dest.exists() && !opts.force {
        return Ok(dest);
    }

    // 3. Download forbidden
    if std::env::var("MOCKLY_NO_INSTALL").is_ok() {
        return Err("MOCKLY_NO_INSTALL is set; refusing to download Mockly binary".into());
    }

    // 4. Download
    let version = opts
        .version
        .clone()
        .or_else(|| std::env::var("MOCKLY_VERSION").ok())
        .unwrap_or_else(|| DEFAULT_VERSION.to_owned());

    let base = opts
        .base_url
        .as_deref()
        .map(|s| s.to_owned())
        .or_else(|| std::env::var("MOCKLY_DOWNLOAD_BASE_URL").ok())
        .unwrap_or_else(|| GITHUB_BASE.to_owned());

    let asset = get_asset_name()?;
    let url = format!("{}/{}/{}", base.trim_end_matches('/'), version, asset);

    if let Some(parent) = dest.parent() {
        std::fs::create_dir_all(parent)?;
    }

    let client = reqwest::blocking::Client::builder()
        .timeout(std::time::Duration::from_secs(300))
        .build()?;
    let response = client.get(&url).send()?;
    if !response.status().is_success() {
        return Err(format!(
            "Failed to download Mockly binary from {}: HTTP {}",
            url,
            response.status()
        )
        .into());
    }

    let bytes = response.bytes()?;
    std::fs::write(&dest, &bytes)?;

    #[cfg(unix)]
    {
        use std::os::unix::fs::PermissionsExt;
        let mut perms = std::fs::metadata(&dest)?.permissions();
        perms.set_mode(0o755);
        std::fs::set_permissions(&dest, perms)?;
    }

    Ok(dest)
}

pub(crate) fn binary_name() -> String {
    if cfg!(windows) {
        "mockly.exe".to_owned()
    } else {
        "mockly".to_owned()
    }
}

fn get_asset_name() -> Result<String, Box<dyn std::error::Error>> {
    let os = match std::env::consts::OS {
        "linux" => "linux",
        "macos" => "macos",
        "windows" => "windows",
        other => return Err(format!("Unsupported OS: {}", other).into()),
    };

    let arch = match std::env::consts::ARCH {
        "x86_64" => "amd64",
        "aarch64" => "arm64",
        other => return Err(format!("Unsupported architecture: {}", other).into()),
    };

    let name = if os == "windows" {
        format!("mockly-{}-{}.exe", os, arch)
    } else {
        format!("mockly-{}-{}", os, arch)
    };

    Ok(name)
}

/// Test helpers for verifying install logic without touching real env vars.
/// Hidden from public docs but accessible from integration tests.
#[doc(hidden)]
pub mod test_helpers {
    use super::binary_name;
    use crate::types::InstallOptions;
    use std::collections::HashMap;
    use std::path::PathBuf;

    /// Like `get_binary_path` but reads from `env` map instead of real env vars.
    pub fn get_binary_path_with_env(
        bin_dir: Option<&str>,
        env: &HashMap<String, String>,
    ) -> Option<PathBuf> {
        let exe = binary_name();

        if let Some(p) = env.get("MOCKLY_BINARY_PATH") {
            let path = PathBuf::from(p);
            if path.exists() {
                return Some(path);
            }
        }

        if let Some(dir) = bin_dir {
            let path = PathBuf::from(dir).join(&exe);
            if path.exists() {
                return Some(path);
            }
        }

        let default_path = PathBuf::from("bin").join(&exe);
        if default_path.exists() {
            return Some(default_path);
        }

        None
    }

    /// Like `install` but reads from `env` map instead of real env vars.
    pub fn install_with_env(
        opts: InstallOptions,
        env: &HashMap<String, String>,
    ) -> Result<PathBuf, Box<dyn std::error::Error>> {
        if let Some(p) = env.get("MOCKLY_BINARY_PATH") {
            let path = PathBuf::from(p);
            if path.exists() {
                return Ok(path);
            }
        }

        let bin_dir = opts.bin_dir.as_deref().unwrap_or("bin");
        let exe = binary_name();
        let dest = PathBuf::from(bin_dir).join(&exe);

        if dest.exists() && !opts.force {
            return Ok(dest);
        }

        if env.contains_key("MOCKLY_NO_INSTALL") {
            return Err("MOCKLY_NO_INSTALL is set; refusing to download Mockly binary".into());
        }

        Err("Would download binary (skipped in test helper)".into())
    }
}
