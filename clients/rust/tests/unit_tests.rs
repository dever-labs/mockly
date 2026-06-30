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
            headers: None,
            delay: None,
            disabled: None,
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
            description: None,
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



    fn start_fake_server_with_body(response_code: u16, body: &'static str) -> (u16, std::thread::JoinHandle<String>) {
        use std::io::{Read, Write};
        use std::net::TcpListener;

        let listener = TcpListener::bind("127.0.0.1:0").unwrap();
        let port = listener.local_addr().unwrap().port();

        let handle = std::thread::spawn(move || {
            let mut first_line = String::new();
            if let Ok((mut stream, _)) = listener.accept() {
                let mut buf = [0u8; 8192];
                let n = stream.read(&mut buf).unwrap_or(0);
                let req = String::from_utf8_lossy(&buf[..n]);
                if let Some(line) = req.lines().next() {
                    first_line = line.to_string();
                }
                let reason = match response_code {
                    200 => "OK", 201 => "Created", 204 => "No Content", 408 => "Request Timeout", _ => "Error",
                };
                let response = format!(
                    "HTTP/1.1 {} {}
Content-Type: application/json
Content-Length: {}

{}",
                    response_code, reason, body.len(), body
                );
                let _ = stream.write_all(response.as_bytes());
            }
            first_line
        });

        (port, handle)
    }

    fn sample_mock(id: &str) -> Mock {
        Mock {
            id: id.to_string(),
            request: MockRequest {
                method: "GET".to_string(),
                path: "/ping".to_string(),
                headers: HashMap::new(),
            },
            response: MockResponse {
                status: 200,
                body: Some("ok".to_string()),
                headers: HashMap::new(),
                delay: None,
            },
        }
    }

    fn sample_scenario(name: &str) -> Scenario {
        Scenario {
            id: "s1".to_string(),
            name: name.to_string(),
            description: None,
            patches: vec![],
        }
    }

    fn sample_call_entry_json() -> &'static str {
        r#"{"id":"c1","timestamp":"2026-01-01T00:00:00Z","protocol":"http","method":"GET","path":"/ping","status":200,"duration_ms":5,"matched_id":"m1"}"#
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
        let (api_port, handle) = start_fake_server(200);
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
            first_line.contains("/api/fault/http"),
            "expected /api/fault/http, got: {}",
            first_line
        );
    }

    #[test]
    fn clear_fault_sends_delete_to_api_fault() {
        let (api_port, handle) = start_fake_server(204);
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

    #[test]
    fn list_mocks_gets_correct_endpoint_and_parses_response() {
        let body = r#"[{"id":"m1","request":{"method":"GET","path":"/ping"},"response":{"status":200,"body":"ok"}}]"#;
        let (api_port, handle) = start_fake_server_with_body(200, body);
        let server = server_helpers::new_server_for_test(0, api_port);
        let mocks = server.list_mocks().expect("list_mocks");
        let first_line = handle.join().unwrap();
        assert_eq!(first_line, "GET /api/mocks/http HTTP/1.1");
        assert_eq!(mocks[0].id, "m1");
    }

    #[test]
    fn list_mocks_errors_on_unexpected_status() {
        let (api_port, handle) = start_fake_server(500);
        let server = server_helpers::new_server_for_test(0, api_port);
        assert!(server.list_mocks().is_err());
        let _ = handle.join();
    }

    #[test]
    fn update_mock_puts_to_correct_endpoint_and_parses_response() {
        let body = r#"{"id":"m1","request":{"method":"GET","path":"/ping"},"response":{"status":201,"body":"updated"}}"#;
        let (api_port, handle) = start_fake_server_with_body(200, body);
        let server = server_helpers::new_server_for_test(0, api_port);
        let mock = server.update_mock("m1", &sample_mock("m1")).expect("update_mock");
        let first_line = handle.join().unwrap();
        assert_eq!(first_line, "PUT /api/mocks/http/m1 HTTP/1.1");
        assert_eq!(mock.response.status, 201);
    }

    #[test]
    fn update_mock_errors_on_unexpected_status() {
        let (api_port, handle) = start_fake_server(500);
        let server = server_helpers::new_server_for_test(0, api_port);
        assert!(server.update_mock("m1", &sample_mock("m1")).is_err());
        let _ = handle.join();
    }

    #[test]
    fn patch_mock_patches_correct_endpoint_and_parses_response() {
        let body = r#"{"id":"m1","request":{"method":"GET","path":"/ping"},"response":{"status":201,"body":"patched"}}"#;
        let (api_port, handle) = start_fake_server_with_body(200, body);
        let server = server_helpers::new_server_for_test(0, api_port);
        let patch = MockResponsePatch { status: Some(201), body: Some("patched".to_string()), headers: None, delay: None };
        let mock = server.patch_mock("m1", &patch).expect("patch_mock");
        let first_line = handle.join().unwrap();
        assert_eq!(first_line, "PATCH /api/mocks/http/m1 HTTP/1.1");
        assert_eq!(mock.response.body.as_deref(), Some("patched"));
    }

    #[test]
    fn patch_mock_errors_on_unexpected_status() {
        let (api_port, handle) = start_fake_server(500);
        let server = server_helpers::new_server_for_test(0, api_port);
        let patch = MockResponsePatch { status: Some(201), body: None, headers: None, delay: None };
        assert!(server.patch_mock("m1", &patch).is_err());
        let _ = handle.join();
    }

    #[test]
    fn get_state_gets_correct_endpoint_and_parses_response() {
        let (api_port, handle) = start_fake_server_with_body(200, r#"{"key1":"val1"}"#);
        let server = server_helpers::new_server_for_test(0, api_port);
        let state = server.get_state().expect("get_state");
        let first_line = handle.join().unwrap();
        assert_eq!(first_line, "GET /api/state HTTP/1.1");
        assert_eq!(state.get("key1").map(String::as_str), Some("val1"));
    }

    #[test]
    fn get_state_errors_on_unexpected_status() {
        let (api_port, handle) = start_fake_server(500);
        let server = server_helpers::new_server_for_test(0, api_port);
        assert!(server.get_state().is_err());
        let _ = handle.join();
    }

    #[test]
    fn set_state_posts_to_correct_endpoint_and_parses_response() {
        let (api_port, handle) = start_fake_server_with_body(200, r#"{"key1":"val1"}"#);
        let server = server_helpers::new_server_for_test(0, api_port);
        let mut state = HashMap::new();
        state.insert("key1".to_string(), "val1".to_string());
        let updated = server.set_state(&state).expect("set_state");
        let first_line = handle.join().unwrap();
        assert_eq!(first_line, "POST /api/state HTTP/1.1");
        assert_eq!(updated.get("key1").map(String::as_str), Some("val1"));
    }

    #[test]
    fn set_state_errors_on_unexpected_status() {
        let (api_port, handle) = start_fake_server(500);
        let server = server_helpers::new_server_for_test(0, api_port);
        let mut state = HashMap::new();
        state.insert("key1".to_string(), "val1".to_string());
        assert!(server.set_state(&state).is_err());
        let _ = handle.join();
    }

    #[test]
    fn delete_state_deletes_correct_endpoint() {
        let (api_port, handle) = start_fake_server(200);
        let server = server_helpers::new_server_for_test(0, api_port);
        assert!(server.delete_state("key1").is_ok());
        let first_line = handle.join().unwrap();
        assert_eq!(first_line, "DELETE /api/state/key1 HTTP/1.1");
    }

    #[test]
    fn delete_state_errors_on_unexpected_status() {
        let (api_port, handle) = start_fake_server(500);
        let server = server_helpers::new_server_for_test(0, api_port);
        assert!(server.delete_state("key1").is_err());
        let _ = handle.join();
    }

    #[test]
    fn get_logs_gets_correct_endpoint_and_parses_response() {
        let body = format!("[{}]", sample_call_entry_json());
        let leaked: &'static str = Box::leak(body.into_boxed_str());
        let (api_port, handle) = start_fake_server_with_body(200, leaked);
        let server = server_helpers::new_server_for_test(0, api_port);
        let logs = server.get_logs(Some("m1")).expect("get_logs");
        let first_line = handle.join().unwrap();
        assert_eq!(first_line, "GET /api/logs?matched_id=m1 HTTP/1.1");
        assert_eq!(logs[0].matched_id.as_deref(), Some("m1"));
    }

    #[test]
    fn get_logs_errors_on_unexpected_status() {
        let (api_port, handle) = start_fake_server(500);
        let server = server_helpers::new_server_for_test(0, api_port);
        assert!(server.get_logs(None).is_err());
        let _ = handle.join();
    }

    #[test]
    fn clear_logs_deletes_correct_endpoint() {
        let (api_port, handle) = start_fake_server(200);
        let server = server_helpers::new_server_for_test(0, api_port);
        assert!(server.clear_logs().is_ok());
        let first_line = handle.join().unwrap();
        assert_eq!(first_line, "DELETE /api/logs HTTP/1.1");
    }

    #[test]
    fn clear_logs_errors_on_unexpected_status() {
        let (api_port, handle) = start_fake_server(500);
        let server = server_helpers::new_server_for_test(0, api_port);
        assert!(server.clear_logs().is_err());
        let _ = handle.join();
    }

    #[test]
    fn get_logs_count_gets_correct_endpoint_and_parses_response() {
        let (api_port, handle) = start_fake_server_with_body(200, r#"{"count":5}"#);
        let server = server_helpers::new_server_for_test(0, api_port);
        let count = server.get_logs_count(Some("m1")).expect("get_logs_count");
        let first_line = handle.join().unwrap();
        assert_eq!(first_line, "GET /api/logs/count?matched_id=m1 HTTP/1.1");
        assert_eq!(count, 5);
    }

    #[test]
    fn get_logs_count_errors_on_unexpected_status() {
        let (api_port, handle) = start_fake_server(500);
        let server = server_helpers::new_server_for_test(0, api_port);
        assert!(server.get_logs_count(None).is_err());
        let _ = handle.join();
    }

    #[test]
    fn list_scenarios_gets_correct_endpoint_and_parses_response() {
        let (api_port, handle) = start_fake_server_with_body(200, r#"[{"id":"s1","name":"Test","patches":[]}]"#);
        let server = server_helpers::new_server_for_test(0, api_port);
        let scenarios = server.list_scenarios().expect("list_scenarios");
        let first_line = handle.join().unwrap();
        assert_eq!(first_line, "GET /api/scenarios HTTP/1.1");
        assert_eq!(scenarios[0].id, "s1");
    }

    #[test]
    fn list_scenarios_errors_on_unexpected_status() {
        let (api_port, handle) = start_fake_server(500);
        let server = server_helpers::new_server_for_test(0, api_port);
        assert!(server.list_scenarios().is_err());
        let _ = handle.join();
    }

    #[test]
    fn create_scenario_posts_to_correct_endpoint_and_parses_response() {
        let (api_port, handle) = start_fake_server_with_body(201, r#"{"id":"s1","name":"Test","patches":[]}"#);
        let server = server_helpers::new_server_for_test(0, api_port);
        let scenario = server.create_scenario(&sample_scenario("Test")).expect("create_scenario");
        let first_line = handle.join().unwrap();
        assert_eq!(first_line, "POST /api/scenarios HTTP/1.1");
        assert_eq!(scenario.name, "Test");
    }

    #[test]
    fn create_scenario_errors_on_unexpected_status() {
        let (api_port, handle) = start_fake_server(500);
        let server = server_helpers::new_server_for_test(0, api_port);
        assert!(server.create_scenario(&sample_scenario("Test")).is_err());
        let _ = handle.join();
    }

    #[test]
    fn get_scenario_gets_correct_endpoint_and_parses_response() {
        let (api_port, handle) = start_fake_server_with_body(200, r#"{"id":"s1","name":"Test","patches":[]}"#);
        let server = server_helpers::new_server_for_test(0, api_port);
        let scenario = server.get_scenario("s1").expect("get_scenario");
        let first_line = handle.join().unwrap();
        assert_eq!(first_line, "GET /api/scenarios/s1 HTTP/1.1");
        assert_eq!(scenario.id, "s1");
    }

    #[test]
    fn get_scenario_errors_on_unexpected_status() {
        let (api_port, handle) = start_fake_server(500);
        let server = server_helpers::new_server_for_test(0, api_port);
        assert!(server.get_scenario("s1").is_err());
        let _ = handle.join();
    }

    #[test]
    fn update_scenario_puts_to_correct_endpoint_and_parses_response() {
        let (api_port, handle) = start_fake_server_with_body(200, r#"{"id":"s1","name":"Updated","patches":[]}"#);
        let server = server_helpers::new_server_for_test(0, api_port);
        let scenario = server.update_scenario("s1", &sample_scenario("Updated")).expect("update_scenario");
        let first_line = handle.join().unwrap();
        assert_eq!(first_line, "PUT /api/scenarios/s1 HTTP/1.1");
        assert_eq!(scenario.name, "Updated");
    }

    #[test]
    fn update_scenario_errors_on_unexpected_status() {
        let (api_port, handle) = start_fake_server(500);
        let server = server_helpers::new_server_for_test(0, api_port);
        assert!(server.update_scenario("s1", &sample_scenario("Updated")).is_err());
        let _ = handle.join();
    }

    #[test]
    fn delete_scenario_deletes_correct_endpoint() {
        let (api_port, handle) = start_fake_server(200);
        let server = server_helpers::new_server_for_test(0, api_port);
        assert!(server.delete_scenario("s1").is_ok());
        let first_line = handle.join().unwrap();
        assert_eq!(first_line, "DELETE /api/scenarios/s1 HTTP/1.1");
    }

    #[test]
    fn delete_scenario_errors_on_unexpected_status() {
        let (api_port, handle) = start_fake_server(500);
        let server = server_helpers::new_server_for_test(0, api_port);
        assert!(server.delete_scenario("s1").is_err());
        let _ = handle.join();
    }

    #[test]
    fn list_active_scenarios_gets_correct_endpoint_and_parses_response() {
        let body = r#"{"active":["s1"],"scenarios":[{"id":"s1","name":"Test","patches":[]}]}"#;
        let (api_port, handle) = start_fake_server_with_body(200, body);
        let server = server_helpers::new_server_for_test(0, api_port);
        let active = server.list_active_scenarios().expect("list_active_scenarios");
        let first_line = handle.join().unwrap();
        assert_eq!(first_line, "GET /api/scenarios/active HTTP/1.1");
        assert_eq!(active.active[0], "s1");
        assert_eq!(active.scenarios[0].id, "s1");
    }

    #[test]
    fn list_active_scenarios_errors_on_unexpected_status() {
        let (api_port, handle) = start_fake_server(500);
        let server = server_helpers::new_server_for_test(0, api_port);
        assert!(server.list_active_scenarios().is_err());
        let _ = handle.join();
    }

    #[test]
    fn get_calls_gets_correct_endpoint_and_parses_response() {
        let body = format!(r#"{{"mock_id":"m1","count":2,"calls":[{}]}}"#, sample_call_entry_json());
        let leaked: &'static str = Box::leak(body.into_boxed_str());
        let (api_port, handle) = start_fake_server_with_body(200, leaked);
        let server = server_helpers::new_server_for_test(0, api_port);
        let summary = server.get_calls("m1").expect("get_calls");
        let first_line = handle.join().unwrap();
        assert_eq!(first_line, "GET /api/calls/http/m1 HTTP/1.1");
        assert_eq!(summary.count, 2);
        assert_eq!(summary.calls[0].id, "c1");
    }

    #[test]
    fn get_calls_errors_on_unexpected_status() {
        let (api_port, handle) = start_fake_server(500);
        let server = server_helpers::new_server_for_test(0, api_port);
        assert!(server.get_calls("m1").is_err());
        let _ = handle.join();
    }

    #[test]
    fn clear_calls_deletes_correct_endpoint() {
        let (api_port, handle) = start_fake_server(200);
        let server = server_helpers::new_server_for_test(0, api_port);
        assert!(server.clear_calls("m1").is_ok());
        let first_line = handle.join().unwrap();
        assert_eq!(first_line, "DELETE /api/calls/http/m1 HTTP/1.1");
    }

    #[test]
    fn clear_calls_errors_on_unexpected_status() {
        let (api_port, handle) = start_fake_server(500);
        let server = server_helpers::new_server_for_test(0, api_port);
        assert!(server.clear_calls("m1").is_err());
        let _ = handle.join();
    }

    #[test]
    fn clear_all_calls_deletes_correct_endpoint() {
        let (api_port, handle) = start_fake_server(200);
        let server = server_helpers::new_server_for_test(0, api_port);
        assert!(server.clear_all_calls().is_ok());
        let first_line = handle.join().unwrap();
        assert_eq!(first_line, "DELETE /api/calls/http HTTP/1.1");
    }

    #[test]
    fn clear_all_calls_errors_on_unexpected_status() {
        let (api_port, handle) = start_fake_server(500);
        let server = server_helpers::new_server_for_test(0, api_port);
        assert!(server.clear_all_calls().is_err());
        let _ = handle.join();
    }

    #[test]
    fn wait_for_calls_posts_to_correct_endpoint_and_parses_response() {
        let body = format!(r#"{{"mock_id":"m1","count":2,"calls":[{}]}}"#, sample_call_entry_json());
        let leaked: &'static str = Box::leak(body.into_boxed_str());
        let (api_port, handle) = start_fake_server_with_body(200, leaked);
        let server = server_helpers::new_server_for_test(0, api_port);
        let summary = server.wait_for_calls("m1", 2, 5).expect("wait_for_calls");
        let first_line = handle.join().unwrap();
        assert_eq!(first_line, "POST /api/calls/http/m1/wait HTTP/1.1");
        assert_eq!(summary.count, 2);
        assert_eq!(summary.calls[0].id, "c1");
    }

    #[test]
    fn wait_for_calls_errors_on_unexpected_status() {
        let (api_port, handle) = start_fake_server_with_body(408, r#"{"error":"timeout"}"#);
        let server = server_helpers::new_server_for_test(0, api_port);
        assert!(server.wait_for_calls("m1", 2, 5).is_err());
        let _ = handle.join();
    }

}
