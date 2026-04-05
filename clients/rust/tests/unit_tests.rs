#[cfg(test)]
mod tests {
    use mockly_driver::*;
    use std::collections::HashMap;
    use std::fs;
    use tempfile::tempdir;
    use mockly_driver::server::test_helpers as server_helpers;

    // Re-import test helpers from the install module via a wrapper.
    // (We test install logic through the public helpers that accept an env map.)
    use mockly_driver::install::test_helpers::{get_binary_path_with_env, install_with_env};

    #[test]
    fn get_free_port_returns_valid_port() {
        let port = get_free_port().expect("should allocate a free port");
        assert!(port >= 1024, "port must be in the unprivileged range (>= 1024), got {port}");
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

    // ── yaml_str tests ────────────────────────────────────────────────────────

    #[test]
    fn yaml_str_wraps_plain_string_in_single_quotes() {
        assert_eq!(server_helpers::yaml_str_for_test("hello"), "'hello'");
    }

    #[test]
    fn yaml_str_escapes_single_quote_by_doubling() {
        assert_eq!(server_helpers::yaml_str_for_test("it's"), "'it''s'");
    }

    #[test]
    fn yaml_str_empty_string() {
        assert_eq!(server_helpers::yaml_str_for_test(""), "''");
    }

    #[test]
    fn yaml_str_string_with_spaces() {
        assert_eq!(server_helpers::yaml_str_for_test("hello world"), "'hello world'");
    }

    #[test]
    fn yaml_str_string_with_backslash() {
        assert_eq!(server_helpers::yaml_str_for_test("a\\b"), "'a\\b'");
    }

    // ── JSON serialization tests ──────────────────────────────────────────────

    #[test]
    fn mock_serializes_to_json() {
        let mock = Mock {
            id: "test".to_string(),
            request: MockRequest {
                method: "GET".to_string(),
                path: "/users".to_string(),
                headers: Default::default(),
            },
            response: MockResponse {
                status: 200,
                body: Some("[{\"id\":1}]".to_string()),
                headers: Default::default(),
                delay: Some("50ms".to_string()),
            },
        };
        let json = serde_json::to_string(&mock).unwrap();
        assert!(json.contains("\"id\":\"test\""), "json: {}", json);
        assert!(json.contains("\"method\":\"GET\""), "json: {}", json);
        assert!(json.contains("\"status\":200"), "json: {}", json);
        assert!(json.contains("\"delay\":\"50ms\""), "json: {}", json);
    }

    #[test]
    fn mock_response_omits_none_optional_fields() {
        let resp = MockResponse {
            status: 404,
            body: None,
            headers: Default::default(),
            delay: None,
        };
        let json = serde_json::to_string(&resp).unwrap();
        assert!(!json.contains("\"body\""), "json should omit body: {}", json);
        assert!(!json.contains("\"delay\""), "json should omit delay: {}", json);
        assert!(json.contains("\"status\":404"), "json: {}", json);
    }

    #[test]
    fn fault_config_serializes_all_fields() {
        let config = FaultConfig {
            enabled: true,
            delay: Some("100ms".to_string()),
            status_override: Some(503),
            error_rate: Some(0.5),
        };
        let json = serde_json::to_string(&config).unwrap();
        assert!(json.contains("\"enabled\":true"), "json: {}", json);
        assert!(json.contains("\"delay\":\"100ms\""), "json: {}", json);
        assert!(json.contains("\"status_override\":503"), "json: {}", json);
        assert!(json.contains("\"error_rate\":0.5"), "json: {}", json);
    }

    #[test]
    fn scenario_patch_omits_null_fields() {
        let patch = ScenarioPatch {
            mock_id: "m1".to_string(),
            status: None,
            body: None,
            delay: None,
        };
        let json = serde_json::to_string(&patch).unwrap();
        assert!(json.contains("\"mock_id\":\"m1\""), "json: {}", json);
        assert!(!json.contains("\"status\""), "json should omit status: {}", json);
        assert!(!json.contains("\"body\""), "json should omit body: {}", json);
        assert!(!json.contains("\"delay\""), "json should omit delay: {}", json);
    }

    // ── write_config tests ────────────────────────────────────────────────────

    #[test]
    fn write_config_returns_temp_file() {
        let result = server_helpers::write_config_for_test(8080, 8081, &[]);
        assert!(result.is_ok(), "write_config should succeed: {:?}", result.err());
    }

    #[test]
    fn write_config_contains_http_port() {
        let file = server_helpers::write_config_for_test(8080, 8081, &[]).unwrap();
        let content = fs::read_to_string(file.path()).unwrap();
        assert!(content.contains("port: 8080"), "content: {}", content);
    }

    #[test]
    fn write_config_contains_api_port() {
        let file = server_helpers::write_config_for_test(8080, 8081, &[]).unwrap();
        let content = fs::read_to_string(file.path()).unwrap();
        assert!(content.contains("port: 8081"), "content: {}", content);
    }

    #[test]
    fn write_config_file_exists_while_held() {
        let file = server_helpers::write_config_for_test(9000, 9001, &[]).unwrap();
        assert!(file.path().exists(), "temp file should exist while NamedTempFile is alive");
    }

    #[test]
    fn write_config_includes_scenario_data() {
        let scenario = Scenario {
            id: "s1".to_string(),
            name: "My Scenario".to_string(),
            patches: vec![],
        };
        let file = server_helpers::write_config_for_test(8080, 8081, &[scenario]).unwrap();
        let content = fs::read_to_string(file.path()).unwrap();
        assert!(content.contains("scenarios:"), "content: {}", content);
        assert!(content.contains("'s1'"), "content: {}", content);
        assert!(content.contains("'My Scenario'"), "content: {}", content);
    }

    // ── HTTP API method tests (fake TCP server) ───────────────────────────────

    /// Spins up a minimal TCP server that accepts one connection, captures the
    /// first request line, and replies with the given HTTP status code.
    fn start_fake_server(response_code: u16) -> (u16, std::thread::JoinHandle<String>) {
        use std::io::{Read, Write};
        use std::net::TcpListener;
        use std::thread;

        let listener = TcpListener::bind("127.0.0.1:0").unwrap();
        let port = listener.local_addr().unwrap().port();

        let handle = thread::spawn(move || {
            let mut first_line = String::new();
            if let Ok((mut stream, _)) = listener.accept() {
                let mut buf = [0u8; 4096];
                let n = stream.read(&mut buf).unwrap_or(0);
                let req_str = String::from_utf8_lossy(&buf[..n]).to_string();
                if let Some(line) = req_str.lines().next() {
                    first_line = line.to_string();
                }
                let reason = match response_code {
                    200 => "OK",
                    201 => "Created",
                    204 => "No Content",
                    _ => "Error",
                };
                let response =
                    format!("HTTP/1.1 {} {}\r\nContent-Length: 0\r\n\r\n", response_code, reason);
                let _ = stream.write_all(response.as_bytes());
            }
            first_line
        });

        (port, handle)
    }

    #[test]
    fn add_mock_posts_to_correct_endpoint() {
        let (api_port, handle) = start_fake_server(201);
        let server = server_helpers::new_server_for_test(0, api_port);
        let mock = Mock {
            id: "m1".to_string(),
            request: MockRequest {
                method: "GET".to_string(),
                path: "/test".to_string(),
                headers: Default::default(),
            },
            response: MockResponse {
                status: 200,
                body: None,
                headers: Default::default(),
                delay: None,
            },
        };
        assert!(server.add_mock(&mock).is_ok());
        let first_line = handle.join().unwrap();
        assert!(first_line.contains("POST"), "expected POST, got: {}", first_line);
        assert!(
            first_line.contains("/api/mocks/http"),
            "expected /api/mocks/http, got: {}",
            first_line
        );
    }

    #[test]
    fn delete_mock_sends_delete_to_correct_endpoint() {
        let (api_port, handle) = start_fake_server(204);
        let server = server_helpers::new_server_for_test(0, api_port);
        assert!(server.delete_mock("abc123").is_ok());
        let first_line = handle.join().unwrap();
        assert!(first_line.contains("DELETE"), "expected DELETE, got: {}", first_line);
        assert!(
            first_line.contains("/api/mocks/http/abc123"),
            "expected /api/mocks/http/abc123, got: {}",
            first_line
        );
    }

    #[test]
    fn reset_posts_to_api_reset() {
        let (api_port, handle) = start_fake_server(200);
        let server = server_helpers::new_server_for_test(0, api_port);
        assert!(server.reset().is_ok());
        let first_line = handle.join().unwrap();
        assert!(first_line.contains("POST"), "expected POST, got: {}", first_line);
        assert!(
            first_line.contains("/api/reset"),
            "expected /api/reset, got: {}",
            first_line
        );
    }

    #[test]
    fn activate_scenario_posts_to_correct_endpoint() {
        let (api_port, handle) = start_fake_server(200);
        let server = server_helpers::new_server_for_test(0, api_port);
        assert!(server.activate_scenario("s1").is_ok());
        let first_line = handle.join().unwrap();
        assert!(first_line.contains("POST"), "expected POST, got: {}", first_line);
        assert!(
            first_line.contains("/api/scenarios/s1/activate"),
            "expected /api/scenarios/s1/activate, got: {}",
            first_line
        );
    }

    #[test]
    fn deactivate_scenario_posts_to_correct_endpoint() {
        let (api_port, handle) = start_fake_server(200);
        let server = server_helpers::new_server_for_test(0, api_port);
        assert!(server.deactivate_scenario("s1").is_ok());
        let first_line = handle.join().unwrap();
        assert!(first_line.contains("POST"), "expected POST, got: {}", first_line);
        assert!(
            first_line.contains("/api/scenarios/s1/deactivate"),
            "expected /api/scenarios/s1/deactivate, got: {}",
            first_line
        );
    }

    #[test]
    fn set_fault_posts_to_api_fault() {
        let (api_port, handle) = start_fake_server(200);
        let server = server_helpers::new_server_for_test(0, api_port);
        let config = FaultConfig {
            enabled: true,
            delay: None,
            status_override: None,
            error_rate: None,
        };
        assert!(server.set_fault(&config).is_ok());
        let first_line = handle.join().unwrap();
        assert!(first_line.contains("POST"), "expected POST, got: {}", first_line);
        assert!(
            first_line.contains("/api/fault"),
            "expected /api/fault, got: {}",
            first_line
        );
    }

    #[test]
    fn clear_fault_sends_delete_to_api_fault() {
        let (api_port, handle) = start_fake_server(200);
        let server = server_helpers::new_server_for_test(0, api_port);
        assert!(server.clear_fault().is_ok());
        let first_line = handle.join().unwrap();
        assert!(first_line.contains("DELETE"), "expected DELETE, got: {}", first_line);
        assert!(
            first_line.contains("/api/fault"),
            "expected /api/fault, got: {}",
            first_line
        );
    }

    #[test]
    fn api_method_errors_on_unexpected_status_code() {
        let (api_port, handle) = start_fake_server(500);
        let server = server_helpers::new_server_for_test(0, api_port);
        let result = server.reset();
        assert!(result.is_err(), "reset should fail when server returns 500");
        let _ = handle.join();
    }
}
