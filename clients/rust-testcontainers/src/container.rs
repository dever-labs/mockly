use reqwest::blocking::{Client, Response};
use serde::Deserialize;
use serde_json::{Map, Value};
use std::collections::HashMap;
use std::time::Duration;
use testcontainers::core::Container;

use crate::image::{MocklyImage, API_PORT, HTTP_PORT};
use mockly_driver::{
    ActiveScenariosResponse, CallEntry, CallSummary, FaultConfig, Mock, MockResponsePatch, Scenario,
};

pub struct MocklyContainer {
    inner: Container<MocklyImage>,
    http_port: u16,
    api: ApiClient,
}

impl MocklyContainer {
    pub fn new(container: Container<MocklyImage>) -> Self {
        let http_port = container
            .get_host_port_ipv4(HTTP_PORT)
            .expect("Mockly HTTP port should be exposed");
        let api_port = container
            .get_host_port_ipv4(API_PORT)
            .expect("Mockly API port should be exposed");
        Self {
            inner: container,
            http_port,
            api: ApiClient::new(format!("http://127.0.0.1:{api_port}")),
        }
    }

    pub fn inner(&self) -> &Container<MocklyImage> {
        &self.inner
    }

    pub fn into_inner(self) -> Container<MocklyImage> {
        self.inner
    }

    pub fn http_port(&self) -> u16 {
        self.http_port
    }

    pub fn api_port(&self) -> u16 {
        self.api
            .api_base
            .rsplit(':')
            .next()
            .and_then(|p| p.parse().ok())
            .expect("api_base should contain a valid port")
    }

    pub fn http_base(&self) -> String {
        format!("http://127.0.0.1:{}", self.http_port)
    }

    pub fn api_base(&self) -> String {
        self.api.api_base.clone()
    }

    pub fn add_mock(&self, mock: &Mock) -> Result<(), Box<dyn std::error::Error>> {
        self.api.add_mock(mock)
    }

    pub fn list_mocks(&self) -> Result<Vec<Mock>, Box<dyn std::error::Error>> {
        self.api.list_mocks()
    }

    pub fn update_mock(&self, id: &str, mock: &Mock) -> Result<Mock, Box<dyn std::error::Error>> {
        self.api.update_mock(id, mock)
    }

    pub fn patch_mock(
        &self,
        id: &str,
        patch: &MockResponsePatch,
    ) -> Result<Mock, Box<dyn std::error::Error>> {
        self.api.patch_mock(id, patch)
    }

    pub fn delete_mock(&self, id: &str) -> Result<(), Box<dyn std::error::Error>> {
        self.api.delete_mock(id)
    }

    pub fn get_state(&self) -> Result<HashMap<String, String>, Box<dyn std::error::Error>> {
        self.api.get_state()
    }

    pub fn set_state(
        &self,
        state: &HashMap<String, String>,
    ) -> Result<HashMap<String, String>, Box<dyn std::error::Error>> {
        self.api.set_state(state)
    }

    pub fn delete_state(&self, key: &str) -> Result<(), Box<dyn std::error::Error>> {
        self.api.delete_state(key)
    }

    pub fn reset(&self) -> Result<(), Box<dyn std::error::Error>> {
        self.api.reset()
    }

    pub fn activate_scenario(&self, id: &str) -> Result<(), Box<dyn std::error::Error>> {
        self.api.activate_scenario(id)
    }

    pub fn deactivate_scenario(&self, id: &str) -> Result<(), Box<dyn std::error::Error>> {
        self.api.deactivate_scenario(id)
    }

    pub fn list_scenarios(&self) -> Result<Vec<Scenario>, Box<dyn std::error::Error>> {
        self.api.list_scenarios()
    }

    pub fn create_scenario(
        &self,
        scenario: &Scenario,
    ) -> Result<Scenario, Box<dyn std::error::Error>> {
        self.api.create_scenario(scenario)
    }

    pub fn get_scenario(&self, id: &str) -> Result<Scenario, Box<dyn std::error::Error>> {
        self.api.get_scenario(id)
    }

    pub fn update_scenario(
        &self,
        id: &str,
        scenario: &Scenario,
    ) -> Result<Scenario, Box<dyn std::error::Error>> {
        self.api.update_scenario(id, scenario)
    }

    pub fn delete_scenario(&self, id: &str) -> Result<(), Box<dyn std::error::Error>> {
        self.api.delete_scenario(id)
    }

    pub fn list_active_scenarios(
        &self,
    ) -> Result<ActiveScenariosResponse, Box<dyn std::error::Error>> {
        self.api.list_active_scenarios()
    }

    pub fn set_fault(&self, config: &FaultConfig) -> Result<(), Box<dyn std::error::Error>> {
        self.api.set_fault(config)
    }

    pub fn clear_fault(&self) -> Result<(), Box<dyn std::error::Error>> {
        self.api.clear_fault()
    }

    pub fn get_logs(
        &self,
        matched_id: Option<&str>,
    ) -> Result<Vec<CallEntry>, Box<dyn std::error::Error>> {
        self.api.get_logs(matched_id)
    }

    pub fn clear_logs(&self) -> Result<(), Box<dyn std::error::Error>> {
        self.api.clear_logs()
    }

    pub fn get_logs_count(
        &self,
        matched_id: Option<&str>,
    ) -> Result<i64, Box<dyn std::error::Error>> {
        self.api.get_logs_count(matched_id)
    }

    pub fn get_calls(&self, mock_id: &str) -> Result<CallSummary, Box<dyn std::error::Error>> {
        self.api.get_calls(mock_id)
    }

    pub fn clear_calls(&self, mock_id: &str) -> Result<(), Box<dyn std::error::Error>> {
        self.api.clear_calls(mock_id)
    }

    pub fn clear_all_calls(&self) -> Result<(), Box<dyn std::error::Error>> {
        self.api.clear_all_calls()
    }

    pub fn wait_for_calls(
        &self,
        mock_id: &str,
        count: usize,
        timeout: Duration,
    ) -> Result<CallSummary, Box<dyn std::error::Error>> {
        self.api.wait_for_calls(mock_id, count, timeout)
    }
}

impl From<Container<MocklyImage>> for MocklyContainer {
    fn from(container: Container<MocklyImage>) -> Self {
        Self::new(container)
    }
}

/// ApiClient holds the HTTP-calling logic for the Mockly management API,
/// decoupled from the Docker-container-holding `MocklyContainer` wrapper so
/// it can be unit tested against a local fake HTTP server (see the `tests`
/// module below) without starting a real Docker container.
struct ApiClient {
    api_base: String,
    client: Client,
}

impl ApiClient {
    fn new(api_base: String) -> Self {
        Self {
            api_base,
            client: Client::new(),
        }
    }

    fn add_mock(&self, mock: &Mock) -> Result<(), Box<dyn std::error::Error>> {
        self.expect_status(
            "POST",
            "/api/mocks/http",
            Some(serde_json::to_value(mock)?),
            &[200, 201],
            "add_mock",
        )
    }

    fn list_mocks(&self) -> Result<Vec<Mock>, Box<dyn std::error::Error>> {
        self.get_json("/api/mocks/http")
    }

    fn update_mock(&self, id: &str, mock: &Mock) -> Result<Mock, Box<dyn std::error::Error>> {
        self.put_and_read(&format!("/api/mocks/http/{id}"), mock)
    }

    fn patch_mock(
        &self,
        id: &str,
        patch: &MockResponsePatch,
    ) -> Result<Mock, Box<dyn std::error::Error>> {
        self.patch_and_read(&format!("/api/mocks/http/{id}"), patch)
    }

    fn delete_mock(&self, id: &str) -> Result<(), Box<dyn std::error::Error>> {
        self.expect_status(
            "DELETE",
            &format!("/api/mocks/http/{id}"),
            None,
            &[200],
            "delete_mock",
        )
    }

    fn get_state(&self) -> Result<HashMap<String, String>, Box<dyn std::error::Error>> {
        self.get_json("/api/state")
    }

    fn set_state(
        &self,
        state: &HashMap<String, String>,
    ) -> Result<HashMap<String, String>, Box<dyn std::error::Error>> {
        self.post_and_read("/api/state", state)
    }

    fn delete_state(&self, key: &str) -> Result<(), Box<dyn std::error::Error>> {
        self.expect_status(
            "DELETE",
            &format!("/api/state/{key}"),
            None,
            &[200],
            "delete_state",
        )
    }

    fn reset(&self) -> Result<(), Box<dyn std::error::Error>> {
        self.expect_status("POST", "/api/reset", None, &[200], "reset")
    }

    fn activate_scenario(&self, id: &str) -> Result<(), Box<dyn std::error::Error>> {
        self.expect_status(
            "POST",
            &format!("/api/scenarios/{id}/activate"),
            None,
            &[200],
            "activate_scenario",
        )
    }

    fn deactivate_scenario(&self, id: &str) -> Result<(), Box<dyn std::error::Error>> {
        self.expect_status(
            "POST",
            &format!("/api/scenarios/{id}/deactivate"),
            None,
            &[200],
            "deactivate_scenario",
        )
    }

    fn list_scenarios(&self) -> Result<Vec<Scenario>, Box<dyn std::error::Error>> {
        self.get_json("/api/scenarios")
    }

    fn create_scenario(&self, scenario: &Scenario) -> Result<Scenario, Box<dyn std::error::Error>> {
        self.post_and_read_with_status("/api/scenarios", scenario, 201)
    }

    fn get_scenario(&self, id: &str) -> Result<Scenario, Box<dyn std::error::Error>> {
        self.get_json(&format!("/api/scenarios/{id}"))
    }

    fn update_scenario(
        &self,
        id: &str,
        scenario: &Scenario,
    ) -> Result<Scenario, Box<dyn std::error::Error>> {
        self.put_and_read(&format!("/api/scenarios/{id}"), scenario)
    }

    fn delete_scenario(&self, id: &str) -> Result<(), Box<dyn std::error::Error>> {
        self.expect_status(
            "DELETE",
            &format!("/api/scenarios/{id}"),
            None,
            &[200],
            "delete_scenario",
        )
    }

    fn list_active_scenarios(&self) -> Result<ActiveScenariosResponse, Box<dyn std::error::Error>> {
        self.get_json("/api/scenarios/active")
    }

    fn set_fault(&self, config: &FaultConfig) -> Result<(), Box<dyn std::error::Error>> {
        self.expect_status(
            "POST",
            "/api/fault/http",
            Some(fault_payload(config)),
            &[200],
            "set_fault",
        )
    }

    fn clear_fault(&self) -> Result<(), Box<dyn std::error::Error>> {
        self.expect_status("DELETE", "/api/fault", None, &[200, 204], "clear_fault")
    }

    fn get_logs(
        &self,
        matched_id: Option<&str>,
    ) -> Result<Vec<CallEntry>, Box<dyn std::error::Error>> {
        let url = self.with_optional_matched_id("/api/logs", matched_id)?;
        self.get_json_from_url(url, "/api/logs")
    }

    fn clear_logs(&self) -> Result<(), Box<dyn std::error::Error>> {
        self.expect_status("DELETE", "/api/logs", None, &[200], "clear_logs")
    }

    fn get_logs_count(&self, matched_id: Option<&str>) -> Result<i64, Box<dyn std::error::Error>> {
        let url = self.with_optional_matched_id("/api/logs/count", matched_id)?;
        let response: CountResponse = self.get_json_from_url(url, "/api/logs/count")?;
        Ok(response.count)
    }

    fn get_calls(&self, mock_id: &str) -> Result<CallSummary, Box<dyn std::error::Error>> {
        self.get_json(&format!("/api/calls/http/{mock_id}"))
    }

    fn clear_calls(&self, mock_id: &str) -> Result<(), Box<dyn std::error::Error>> {
        self.expect_status(
            "DELETE",
            &format!("/api/calls/http/{mock_id}"),
            None,
            &[200],
            "clear_calls",
        )
    }

    fn clear_all_calls(&self) -> Result<(), Box<dyn std::error::Error>> {
        self.expect_status("DELETE", "/api/calls/http", None, &[200], "clear_all_calls")
    }

    fn wait_for_calls(
        &self,
        mock_id: &str,
        count: usize,
        timeout: Duration,
    ) -> Result<CallSummary, Box<dyn std::error::Error>> {
        let resp = self.request(
            "POST",
            &format!("/api/calls/http/{mock_id}/wait"),
            Some(serde_json::json!({
                "count": count,
                "timeout": format!("{}ms", timeout.as_millis()),
            })),
        )?;
        let status = resp.status().as_u16();
        let body = resp.text()?;
        if status == 408 {
            return Err(format!(
                "wait_for_calls: timeout waiting for {count} call(s) on '{mock_id}'"
            )
            .into());
        }
        if status != 200 {
            return Err(
                format!("POST /api/calls/http/{mock_id}/wait failed: {status}: {body}").into(),
            );
        }
        Ok(serde_json::from_str(&body)?)
    }

    fn get_json<T: serde::de::DeserializeOwned>(
        &self,
        path: &str,
    ) -> Result<T, Box<dyn std::error::Error>> {
        let resp = self.request("GET", path, None)?;
        let status = resp.status().as_u16();
        let body = resp.text()?;
        if status == 200 {
            Ok(serde_json::from_str(&body)?)
        } else {
            Err(format!("GET {path} failed: {status}: {body}").into())
        }
    }

    fn get_json_from_url<T: serde::de::DeserializeOwned>(
        &self,
        url: reqwest::Url,
        path: &str,
    ) -> Result<T, Box<dyn std::error::Error>> {
        let resp = self.request_url("GET", url, None)?;
        let status = resp.status().as_u16();
        let body = resp.text()?;
        if status == 200 {
            Ok(serde_json::from_str(&body)?)
        } else {
            Err(format!("GET {path} failed: {status}: {body}").into())
        }
    }

    fn post_and_read<T: serde::de::DeserializeOwned>(
        &self,
        path: &str,
        body: &impl serde::Serialize,
    ) -> Result<T, Box<dyn std::error::Error>> {
        self.post_and_read_with_status(path, body, 200)
    }

    fn post_and_read_with_status<T: serde::de::DeserializeOwned>(
        &self,
        path: &str,
        body: &impl serde::Serialize,
        expected_status: u16,
    ) -> Result<T, Box<dyn std::error::Error>> {
        let resp = self.request("POST", path, Some(serde_json::to_value(body)?))?;
        self.read_json_response(resp, "POST", path, expected_status)
    }

    fn put_and_read<T: serde::de::DeserializeOwned>(
        &self,
        path: &str,
        body: &impl serde::Serialize,
    ) -> Result<T, Box<dyn std::error::Error>> {
        let resp = self.request("PUT", path, Some(serde_json::to_value(body)?))?;
        self.read_json_response(resp, "PUT", path, 200)
    }

    fn patch_and_read<T: serde::de::DeserializeOwned>(
        &self,
        path: &str,
        body: &impl serde::Serialize,
    ) -> Result<T, Box<dyn std::error::Error>> {
        let resp = self.request("PATCH", path, Some(serde_json::to_value(body)?))?;
        self.read_json_response(resp, "PATCH", path, 200)
    }

    fn read_json_response<T: serde::de::DeserializeOwned>(
        &self,
        resp: Response,
        method: &str,
        path: &str,
        expected_status: u16,
    ) -> Result<T, Box<dyn std::error::Error>> {
        let status = resp.status().as_u16();
        let body = resp.text()?;
        if status == expected_status {
            Ok(serde_json::from_str(&body)?)
        } else {
            Err(format!("{method} {path} failed: {status}: {body}").into())
        }
    }

    fn with_optional_matched_id(
        &self,
        path: &str,
        matched_id: Option<&str>,
    ) -> Result<reqwest::Url, Box<dyn std::error::Error>> {
        let mut url = reqwest::Url::parse(&format!("{}{}", self.api_base, path))?;
        if let Some(matched_id) = matched_id {
            url.query_pairs_mut().append_pair("matched_id", matched_id);
        }
        Ok(url)
    }

    fn expect_status(
        &self,
        method: &str,
        path: &str,
        body: Option<Value>,
        expected: &[u16],
        op: &str,
    ) -> Result<(), Box<dyn std::error::Error>> {
        let resp = self.request(method, path, body)?;
        let status = resp.status().as_u16();
        if expected.contains(&status) {
            Ok(())
        } else {
            let body = resp.text().unwrap_or_default();
            Err(format!(
                "{op} failed: expected one of {:?}, got {status}: {body}",
                expected
            )
            .into())
        }
    }

    fn request(
        &self,
        method: &str,
        path: &str,
        body: Option<Value>,
    ) -> Result<Response, Box<dyn std::error::Error>> {
        let url = reqwest::Url::parse(&format!("{}{}", self.api_base, path))?;
        self.request_url(method, url, body)
    }

    fn request_url(
        &self,
        method: &str,
        url: reqwest::Url,
        body: Option<Value>,
    ) -> Result<Response, Box<dyn std::error::Error>> {
        let builder = match method {
            "GET" => self.client.get(url),
            "POST" => self.client.post(url),
            "PUT" => self.client.put(url),
            "PATCH" => self.client.patch(url),
            "DELETE" => self.client.delete(url),
            other => return Err(format!("unsupported HTTP method: {other}").into()),
        };

        let builder = match body {
            Some(body) => builder.json(&body),
            None => builder,
        };

        Ok(builder.send()?)
    }
}

fn fault_payload(config: &FaultConfig) -> Value {
    let mut map = Map::new();

    if let Some(delay) = &config.delay {
        map.insert("delay".to_string(), Value::String(delay.clone()));
    }
    if let Some(status) = config.status {
        map.insert("status".to_string(), Value::Number(status.into()));
    }
    if let Some(error_rate) = config.error_rate {
        if let Some(number) = serde_json::Number::from_f64(error_rate) {
            map.insert("error_rate".to_string(), Value::Number(number));
        }
    }

    Value::Object(map)
}

#[derive(Deserialize)]
struct CountResponse {
    count: i64,
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn fault_payload_maps_status_override_to_status() {
        let payload = fault_payload(&FaultConfig {
            enabled: true,
            delay: Some("25ms".to_string()),
            status: Some(503),
            error_rate: Some(0.25),
        });

        assert_eq!(payload["delay"], "25ms");
        assert_eq!(payload["status"], 503);
        assert_eq!(payload["error_rate"], 0.25);
        assert!(payload.get("status_override").is_none());
        assert!(payload.get("enabled").is_none());
    }
}

#[cfg(test)]
mod http_tests {
    use super::*;
    use std::collections::HashMap;
    use std::io::{Read, Write};
    use std::net::TcpListener;
    use std::thread;
    use std::thread::JoinHandle;
    use std::time::Duration;

    /// Spins up a minimal TCP server that accepts one connection, captures
    /// the first request line, and replies with the given HTTP status code
    /// and an optional JSON body.
    fn start_fake_server(
        response_code: u16,
        body: Option<&'static str>,
    ) -> (u16, JoinHandle<String>) {
        let listener = TcpListener::bind("127.0.0.1:0").unwrap();
        let port = listener.local_addr().unwrap().port();

        let handle = thread::spawn(move || {
            let mut first_line = String::new();
            if let Ok((mut stream, _)) = listener.accept() {
                let mut buf = [0u8; 8192];
                let n = stream.read(&mut buf).unwrap_or(0);
                let req = String::from_utf8_lossy(&buf[..n]).to_string();
                if let Some(line) = req.lines().next() {
                    first_line = line.to_string();
                }
                let reason = match response_code {
                    200 => "OK",
                    201 => "Created",
                    204 => "No Content",
                    408 => "Request Timeout",
                    _ => "Error",
                };
                let body = body.unwrap_or("");
                let response = format!(
                    "HTTP/1.1 {response_code} {reason}\r\nContent-Type: application/json\r\nContent-Length: {}\r\n\r\n{body}",
                    body.len()
                );
                let _ = stream.write_all(response.as_bytes());
            }
            first_line
        });

        (port, handle)
    }

    fn fake_api(response_code: u16, body: Option<&'static str>) -> (ApiClient, JoinHandle<String>) {
        let (port, handle) = start_fake_server(response_code, body);
        (ApiClient::new(format!("http://127.0.0.1:{port}")), handle)
    }

    fn sample_mock(id: &str) -> Mock {
        Mock {
            id: id.to_string(),
            request: mockly_driver::MockRequest {
                method: "GET".to_string(),
                path: "/ping".to_string(),
                headers: HashMap::new(),
            },
            response: mockly_driver::MockResponse {
                status: 200,
                body: Some("ok".to_string()),
                headers: HashMap::new(),
                delay: None,
            },
        }
    }

    fn sample_scenario(id: &str, name: &str) -> Scenario {
        Scenario {
            id: id.to_string(),
            name: name.to_string(),
            description: None,
            patches: vec![],
        }
    }

    fn assert_request(first_line: &str, method: &str, path: &str) {
        assert!(
            first_line.contains(method),
            "expected {method}, got: {first_line}"
        );
        assert!(
            first_line.contains(path),
            "expected {path}, got: {first_line}"
        );
    }

    #[test]
    fn add_mock_posts_to_mocks_endpoint() {
        let (api, handle) = fake_api(201, None);
        assert!(api.add_mock(&sample_mock("m1")).is_ok());
        assert_request(&handle.join().unwrap(), "POST", "/api/mocks/http");
    }

    #[test]
    fn list_mocks_gets_and_parses_response() {
        let (api, handle) = fake_api(200, Some("[]"));
        let result = api.list_mocks().unwrap();
        assert!(result.is_empty());
        assert_request(&handle.join().unwrap(), "GET", "/api/mocks/http");
    }

    #[test]
    fn update_mock_puts_to_correct_path() {
        let body = r#"{"id":"m1","request":{"method":"GET","path":"/ping","headers":{}},"response":{"status":200,"body":"ok","headers":{}}}"#;
        let (api, handle) = fake_api(200, Some(body));
        let result = api.update_mock("m1", &sample_mock("m1")).unwrap();
        assert_eq!(result.id, "m1");
        assert_request(&handle.join().unwrap(), "PUT", "/api/mocks/http/m1");
    }

    #[test]
    fn patch_mock_patches_to_correct_path() {
        let body = r#"{"id":"m1","request":{"method":"GET","path":"/ping","headers":{}},"response":{"status":200,"body":"ok","headers":{}}}"#;
        let (api, handle) = fake_api(200, Some(body));
        let patch = MockResponsePatch {
            status: Some(200),
            body: None,
            headers: None,
            delay: None,
        };
        let result = api.patch_mock("m1", &patch).unwrap();
        assert_eq!(result.id, "m1");
        assert_request(&handle.join().unwrap(), "PATCH", "/api/mocks/http/m1");
    }

    #[test]
    fn delete_mock_deletes_to_correct_path() {
        let (api, handle) = fake_api(200, None);
        assert!(api.delete_mock("m1").is_ok());
        assert_request(&handle.join().unwrap(), "DELETE", "/api/mocks/http/m1");
    }

    #[test]
    fn get_state_gets_state_endpoint() {
        let (api, handle) = fake_api(200, Some(r#"{"key":"value"}"#));
        let result = api.get_state().unwrap();
        assert_eq!(result.get("key"), Some(&"value".to_string()));
        assert_request(&handle.join().unwrap(), "GET", "/api/state");
    }

    #[test]
    fn set_state_posts_state_endpoint() {
        let (api, handle) = fake_api(200, Some(r#"{"key":"value"}"#));
        let mut state = HashMap::new();
        state.insert("key".to_string(), "value".to_string());
        let result = api.set_state(&state).unwrap();
        assert_eq!(result.get("key"), Some(&"value".to_string()));
        assert_request(&handle.join().unwrap(), "POST", "/api/state");
    }

    #[test]
    fn delete_state_deletes_correct_key() {
        let (api, handle) = fake_api(200, None);
        assert!(api.delete_state("key").is_ok());
        assert_request(&handle.join().unwrap(), "DELETE", "/api/state/key");
    }

    #[test]
    fn reset_posts_to_reset_endpoint() {
        let (api, handle) = fake_api(200, None);
        assert!(api.reset().is_ok());
        assert_request(&handle.join().unwrap(), "POST", "/api/reset");
    }

    #[test]
    fn activate_scenario_posts_to_correct_endpoint() {
        let (api, handle) = fake_api(200, None);
        assert!(api.activate_scenario("s1").is_ok());
        assert_request(
            &handle.join().unwrap(),
            "POST",
            "/api/scenarios/s1/activate",
        );
    }

    #[test]
    fn deactivate_scenario_posts_to_correct_endpoint() {
        let (api, handle) = fake_api(200, None);
        assert!(api.deactivate_scenario("s1").is_ok());
        assert_request(
            &handle.join().unwrap(),
            "POST",
            "/api/scenarios/s1/deactivate",
        );
    }

    #[test]
    fn list_scenarios_gets_scenarios_endpoint() {
        let (api, handle) = fake_api(200, Some("[]"));
        let result = api.list_scenarios().unwrap();
        assert!(result.is_empty());
        assert_request(&handle.join().unwrap(), "GET", "/api/scenarios");
    }

    #[test]
    fn create_scenario_posts_and_parses_response() {
        let body = r#"{"id":"s1","name":"My Scenario","patches":[]}"#;
        let (api, handle) = fake_api(201, Some(body));
        let result = api
            .create_scenario(&sample_scenario("s1", "My Scenario"))
            .unwrap();
        assert_eq!(result.id, "s1");
        assert_request(&handle.join().unwrap(), "POST", "/api/scenarios");
    }

    #[test]
    fn get_scenario_gets_correct_path() {
        let body = r#"{"id":"s1","name":"My Scenario","patches":[]}"#;
        let (api, handle) = fake_api(200, Some(body));
        let result = api.get_scenario("s1").unwrap();
        assert_eq!(result.id, "s1");
        assert_request(&handle.join().unwrap(), "GET", "/api/scenarios/s1");
    }

    #[test]
    fn update_scenario_puts_and_parses_response() {
        let body = r#"{"id":"s1","name":"Updated","patches":[]}"#;
        let (api, handle) = fake_api(200, Some(body));
        let result = api
            .update_scenario("s1", &sample_scenario("s1", "Updated"))
            .unwrap();
        assert_eq!(result.name, "Updated");
        assert_request(&handle.join().unwrap(), "PUT", "/api/scenarios/s1");
    }

    #[test]
    fn delete_scenario_deletes_correct_path() {
        let (api, handle) = fake_api(200, None);
        assert!(api.delete_scenario("s1").is_ok());
        assert_request(&handle.join().unwrap(), "DELETE", "/api/scenarios/s1");
    }

    #[test]
    fn list_active_scenarios_gets_active_endpoint() {
        let body = r#"{"active":["s1"],"scenarios":[]}"#;
        let (api, handle) = fake_api(200, Some(body));
        assert!(api.list_active_scenarios().is_ok());
        assert_request(&handle.join().unwrap(), "GET", "/api/scenarios/active");
    }

    #[test]
    fn set_fault_posts_fault_payload() {
        let (api, handle) = fake_api(200, None);
        let config = FaultConfig {
            enabled: true,
            delay: Some("10ms".to_string()),
            status: Some(500),
            error_rate: None,
        };
        assert!(api.set_fault(&config).is_ok());
        assert_request(&handle.join().unwrap(), "POST", "/api/fault/http");
    }

    #[test]
    fn clear_fault_deletes_fault_endpoint() {
        let (api, handle) = fake_api(204, None);
        assert!(api.clear_fault().is_ok());
        assert_request(&handle.join().unwrap(), "DELETE", "/api/fault");
    }

    #[test]
    fn get_logs_gets_logs_endpoint() {
        let (api, handle) = fake_api(200, Some("[]"));
        let result = api.get_logs(None).unwrap();
        assert!(result.is_empty());
        assert_request(&handle.join().unwrap(), "GET", "/api/logs");
    }

    #[test]
    fn get_logs_with_matched_id_adds_query_param() {
        let (api, handle) = fake_api(200, Some("[]"));
        assert!(api.get_logs(Some("m1")).is_ok());
        let first_line = handle.join().unwrap();
        assert!(
            first_line.contains("matched_id=m1"),
            "expected matched_id query param, got: {first_line}"
        );
    }

    #[test]
    fn clear_logs_deletes_logs_endpoint() {
        let (api, handle) = fake_api(200, None);
        assert!(api.clear_logs().is_ok());
        assert_request(&handle.join().unwrap(), "DELETE", "/api/logs");
    }

    #[test]
    fn get_logs_count_parses_count_field() {
        let (api, handle) = fake_api(200, Some(r#"{"count":3}"#));
        let count = api.get_logs_count(None).unwrap();
        assert_eq!(count, 3);
        assert_request(&handle.join().unwrap(), "GET", "/api/logs/count");
    }

    #[test]
    fn get_calls_gets_calls_for_mock() {
        let body = r#"{"mock_id":"m1","count":2,"calls":[]}"#;
        let (api, handle) = fake_api(200, Some(body));
        assert!(api.get_calls("m1").is_ok());
        assert_request(&handle.join().unwrap(), "GET", "/api/calls/http/m1");
    }

    #[test]
    fn clear_calls_deletes_calls_for_mock() {
        let (api, handle) = fake_api(200, None);
        assert!(api.clear_calls("m1").is_ok());
        assert_request(&handle.join().unwrap(), "DELETE", "/api/calls/http/m1");
    }

    #[test]
    fn clear_all_calls_deletes_calls_endpoint() {
        let (api, handle) = fake_api(200, None);
        assert!(api.clear_all_calls().is_ok());
        assert_request(&handle.join().unwrap(), "DELETE", "/api/calls/http");
    }

    #[test]
    fn wait_for_calls_posts_to_wait_endpoint() {
        let body = r#"{"mock_id":"m1","count":1,"calls":[]}"#;
        let (api, handle) = fake_api(200, Some(body));
        let result = api
            .wait_for_calls("m1", 1, Duration::from_millis(100))
            .unwrap();
        assert_eq!(result.mock_id, "m1");
        assert_request(&handle.join().unwrap(), "POST", "/api/calls/http/m1/wait");
    }

    #[test]
    fn wait_for_calls_returns_error_on_timeout_status() {
        let (api, handle) = fake_api(408, None);
        let result = api.wait_for_calls("m1", 1, Duration::from_millis(50));
        assert!(result.is_err());
        assert!(result.unwrap_err().to_string().contains("timeout"));
        assert_request(&handle.join().unwrap(), "POST", "/api/calls/http/m1/wait");
    }

    #[test]
    fn add_mock_returns_error_on_unexpected_status() {
        let (api, handle) = fake_api(500, Some("boom"));
        let result = api.add_mock(&sample_mock("m1"));
        assert!(result.is_err());
        assert!(result.unwrap_err().to_string().contains("add_mock"));
        let _ = handle.join();
    }

    #[test]
    fn api_base_reflects_constructed_value() {
        let api = ApiClient::new("http://127.0.0.1:9999".to_string());
        assert_eq!(api.api_base, "http://127.0.0.1:9999");
    }
}
