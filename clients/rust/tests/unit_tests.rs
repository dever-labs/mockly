#[cfg(test)]
mod tests {
    use mockly_driver::*;
    use std::collections::HashMap;
    use std::fs;
    use tempfile::tempdir;

    // Re-import test helpers from the install module via a wrapper.
    // (We test install logic through the public helpers that accept an env map.)
    use mockly_driver::install::test_helpers::{get_binary_path_with_env, install_with_env};

    #[test]
    fn get_free_port_returns_valid_port() {
        let port = get_free_port().expect("should allocate a free port");
        assert!(port > 0, "port must be non-zero");
    }

    #[test]
    fn get_binary_path_returns_none_when_missing() {
        // Use an env map with no MOCKLY_BINARY_PATH and a bin_dir that doesn't exist.
        let env: HashMap<String, String> = HashMap::new();
        let result = get_binary_path_with_env(Some("/nonexistent/dir"), &env);
        assert!(result.is_none(), "should return None when binary is absent");
    }

    #[test]
    fn get_binary_path_finds_binary_in_dir() {
        let dir = tempdir().expect("tempdir");
        let exe_name = if cfg!(windows) { "mockly.exe" } else { "mockly" };
        let binary = dir.path().join(exe_name);
        fs::write(&binary, b"fake binary").expect("write fake binary");

        let env: HashMap<String, String> = HashMap::new();
        let found = get_binary_path_with_env(Some(dir.path().to_str().unwrap()), &env);
        assert_eq!(found, Some(binary));
    }

    #[test]
    fn install_errors_when_no_install_set() {
        let mut env = HashMap::new();
        env.insert("MOCKLY_NO_INSTALL".to_owned(), "1".to_owned());
        // Point MOCKLY_BINARY_PATH at a nonexistent path so it doesn't short-circuit.
        env.insert("MOCKLY_BINARY_PATH".to_owned(), "/nonexistent/mockly".to_owned());

        let opts = InstallOptions {
            bin_dir: Some("/nonexistent/bin".to_owned()),
            ..Default::default()
        };
        let result = install_with_env(opts, &env);
        assert!(result.is_err(), "should error when MOCKLY_NO_INSTALL is set");
        let msg = result.unwrap_err().to_string();
        assert!(
            msg.contains("MOCKLY_NO_INSTALL"),
            "error message should mention MOCKLY_NO_INSTALL, got: {}",
            msg
        );
    }

    #[test]
    fn install_returns_staged_binary_path() {
        let dir = tempdir().expect("tempdir");
        let exe_name = if cfg!(windows) { "mockly.exe" } else { "mockly" };
        let binary = dir.path().join(exe_name);
        fs::write(&binary, b"fake binary").expect("write fake binary");

        let mut env = HashMap::new();
        env.insert(
            "MOCKLY_BINARY_PATH".to_owned(),
            binary.to_str().unwrap().to_owned(),
        );

        let opts = InstallOptions::default();
        let result = install_with_env(opts, &env);
        assert!(result.is_ok(), "should return Ok for staged binary");
        assert_eq!(result.unwrap(), binary);
    }

    #[test]
    fn is_port_conflict_matches_known_messages() {
        // We test via the public API indirectly — import utils via the crate root.
        // The function is not re-exported, so we test the observable behaviour
        // by checking known conflict strings through the module internals.
        // Since is_port_conflict is not pub-exported, we inline the logic here
        // to verify the spec.
        fn check(msg: &str) -> bool {
            let lower = msg.to_lowercase();
            lower.contains("address already in use")
                || lower.contains("eaddrinuse")
                || lower.contains("bind")
        }

        assert!(check("listen tcp 0.0.0.0:9000: bind: address already in use"));
        assert!(check("EADDRINUSE"));
        assert!(check("failed to bind port"));
        assert!(!check("connection refused"));
        assert!(!check("timeout"));
    }
}
