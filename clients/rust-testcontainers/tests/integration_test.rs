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

use std::{collections::HashMap, time::Duration};

use mockly_testcontainers::{
    Mock, MockRequest, MockResponse, MockResponsePatch, MocklyContainer, MocklyImage, Scenario,
};
use reqwest::blocking::Client;
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

    let logs = c.get_logs(None).expect("get_logs failed");
    assert!(!logs.is_empty(), "logs should be non-empty");
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

#[test]
#[ignore = "requires Docker"]
fn list_mocks_returns_added_mocks() {
    let c = start_container();

    let mock = Mock {
        id: "list-mock".to_string(),
        request: MockRequest {
            method: "GET".to_string(),
            path: "/list".to_string(),
            headers: Default::default(),
        },
        response: MockResponse {
            status: 200,
            body: Some("listed".to_string()),
            headers: Default::default(),
            delay: None,
        },
    };

    c.add_mock(&mock).expect("add_mock failed");

    let mocks = c.list_mocks().expect("list_mocks failed");
    assert!(!mocks.is_empty(), "expected at least one mock");
    assert_eq!(mocks[0].id, mock.id);
}

#[test]
#[ignore = "requires Docker"]
fn update_and_patch_mock() {
    let c = start_container();
    let http_base = c.http_base();

    let original = Mock {
        id: "upd".to_string(),
        request: MockRequest {
            method: "GET".to_string(),
            path: "/upd".to_string(),
            headers: Default::default(),
        },
        response: MockResponse {
            status: 200,
            body: Some("orig".to_string()),
            headers: Default::default(),
            delay: None,
        },
    };

    c.add_mock(&original).expect("add_mock failed");

    let updated = Mock {
        response: MockResponse {
            body: Some("updated".to_string()),
            ..original.response.clone()
        },
        ..original.clone()
    };
    c.update_mock("upd", &updated).expect("update_mock failed");

    let resp = reqwest::blocking::get(format!("{http_base}/upd")).expect("GET /upd failed");
    assert_eq!(resp.status().as_u16(), 200);
    assert_eq!(resp.text().unwrap(), "updated");

    c.patch_mock(
        "upd",
        &MockResponsePatch {
            status: Some(418),
            body: None,
            headers: None,
            delay: None,
        },
    )
    .expect("patch_mock failed");

    let resp = reqwest::blocking::get(format!("{http_base}/upd")).expect("GET /upd failed");
    assert_eq!(resp.status().as_u16(), 418);
}

#[test]
#[ignore = "requires Docker"]
fn state_crud() {
    let c = start_container();

    let state = HashMap::from([("k".to_string(), "v".to_string())]);
    c.set_state(&state).expect("set_state failed");

    let current = c.get_state().expect("get_state failed");
    assert_eq!(current.get("k").map(String::as_str), Some("v"));

    c.delete_state("k").expect("delete_state failed");

    let current = c.get_state().expect("get_state after delete failed");
    assert!(!current.contains_key("k"));
}

#[test]
#[ignore = "requires Docker"]
fn get_logs_count() {
    let c = start_container();
    let http_base = c.http_base();

    let _ = reqwest::blocking::get(format!("{http_base}/logs-count"));

    let count = c.get_logs_count(None).expect("get_logs_count failed");
    assert!(count > 0, "expected log count to be greater than zero");
}

#[test]
#[ignore = "requires Docker"]
fn scenario_crud() {
    let c = start_container();

    c.list_scenarios().expect("initial list_scenarios failed");

    let scenario = Scenario {
        id: "tc-s".to_string(),
        name: "TC Scenario".to_string(),
        description: None,
        patches: vec![],
    };

    c.create_scenario(&scenario).expect("create_scenario failed");

    let fetched = c.get_scenario("tc-s").expect("get_scenario failed");
    assert_eq!(fetched.id, "tc-s");

    let scenarios = c.list_scenarios().expect("list_scenarios failed");
    assert!(!scenarios.is_empty(), "expected at least one scenario");

    c.delete_scenario("tc-s").expect("delete_scenario failed");
}

#[test]
#[ignore = "requires Docker"]
fn get_calls_and_clear() {
    let c = start_container();
    let http_base = c.http_base();

    let mock = Mock {
        id: "calls".to_string(),
        request: MockRequest {
            method: "GET".to_string(),
            path: "/calls".to_string(),
            headers: Default::default(),
        },
        response: MockResponse {
            status: 200,
            body: Some("ok".to_string()),
            headers: Default::default(),
            delay: None,
        },
    };

    c.add_mock(&mock).expect("add_mock failed");
    let _ = reqwest::blocking::get(format!("{http_base}/calls")).expect("GET /calls failed");

    let calls = c.get_calls("calls").expect("get_calls failed");
    assert!(calls.count > 0, "expected recorded calls");

    c.clear_calls("calls").expect("clear_calls failed");

    let calls = c.get_calls("calls").expect("get_calls after clear failed");
    assert_eq!(calls.count, 0);

    c.clear_all_calls().expect("clear_all_calls failed");
}

#[test]
#[ignore = "requires Docker"]
fn wait_for_calls() {
    let c = start_container();
    let http_base = c.http_base();
    let client = Client::new();

    let mock = Mock {
        id: "wait".to_string(),
        request: MockRequest {
            method: "GET".to_string(),
            path: "/wait".to_string(),
            headers: Default::default(),
        },
        response: MockResponse {
            status: 200,
            body: Some("ok".to_string()),
            headers: Default::default(),
            delay: None,
        },
    };

    c.add_mock(&mock).expect("add_mock failed");
    client
        .get(format!("{http_base}/wait"))
        .send()
        .expect("GET /wait failed");

    let calls = c
        .wait_for_calls("wait", 1, Duration::from_secs(10))
        .expect("wait_for_calls failed");
    assert!(calls.count >= 1, "expected at least one recorded call");
}
