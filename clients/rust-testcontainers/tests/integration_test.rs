//! Integration tests for the Mockly testcontainers module.
//!
//! These tests start a real Docker container and exercise the full API
//! against a live Mockly server.
//!
//! Run with:
//! ```
//! cargo test -- --ignored
//! ```
//!
//! Requires:
//! - A running Docker daemon
//! - Cargo >= 1.85 (transitive deps use edition 2024)

use mockly_testcontainers::{Mock, MockRequest, MockResponse, MocklyContainer, MocklyImage};
use testcontainers::runners::SyncRunner;

/// Start a container and return it. The container stops when dropped.
fn start_container() -> MocklyContainer {
    let image = MocklyImage::default();
    let container = image.start().expect("failed to start Mockly container");
    MocklyContainer::new(container)
}

#[test]
#[ignore = "requires Docker"]
fn container_api_is_reachable() {
    let c = start_container();
    let api_base = c.api_base();

    let resp = reqwest::blocking::get(format!("{api_base}/api/protocols"))
        .expect("GET /api/protocols failed");
    assert_eq!(resp.status().as_u16(), 200);
}

#[test]
#[ignore = "requires Docker"]
fn add_mock_hit_endpoint_and_reset() {
    let c = start_container();
    let http_base = c.http_base();

    let mock = Mock {
        id: "hello-mock".to_string(),
        request: MockRequest {
            method: "GET".to_string(),
            path: "/hello".to_string(),
            headers: Default::default(),
        },
        response: MockResponse {
            status: 200,
            body: Some("world".to_string()),
            headers: Default::default(),
            delay: None,
        },
    };

    c.add_mock(&mock).expect("add_mock failed");

    let resp = reqwest::blocking::get(format!("{http_base}/hello")).expect("GET /hello failed");
    assert_eq!(resp.status().as_u16(), 200);
    assert_eq!(resp.text().unwrap(), "world");

    c.reset().expect("reset failed");

    let after = reqwest::blocking::get(format!("{http_base}/hello"))
        .expect("GET /hello after reset failed");
    assert_ne!(
        after.status().as_u16(),
        200,
        "mock should be cleared after reset"
    );
}

#[test]
#[ignore = "requires Docker"]
fn get_logs_returns_json_after_request() {
    let c = start_container();
    let http_base = c.http_base();

    // generate a log entry (404 is fine)
    let _ = reqwest::blocking::get(format!("{http_base}/log-probe"));

    let logs = c.get_logs().expect("get_logs failed");
    assert!(!logs.is_empty(), "logs should be non-empty");
    serde_json::from_str::<serde_json::Value>(&logs).expect("logs should be valid JSON");
}

#[test]
#[ignore = "requires Docker"]
fn with_inline_config_starts_container() {
    let custom_config =
        "mockly:\n  api:\n    port: 9091\nprotocols:\n  http:\n    enabled: true\n    port: 8090\n";
    let image = MocklyImage::default().with_inline_config(custom_config);
    let container = image
        .start()
        .expect("failed to start container with inline config");
    let c = MocklyContainer::new(container);

    let resp = reqwest::blocking::get(format!("{}/api/protocols", c.api_base()))
        .expect("GET /api/protocols failed");
    assert_eq!(resp.status().as_u16(), 200);
}
