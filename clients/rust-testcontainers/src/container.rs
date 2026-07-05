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
    client: Client,
}

impl MocklyContainer {
    pub fn new(container: Container<MocklyImage>) -> Self {
        Self {
            inner: container,
            client: Client::new(),
        }
    }

    pub fn inner(&self) -> &Container<MocklyImage> {
        &self.inner
    }

    pub fn into_inner(self) -> Container<MocklyImage> {
        self.inner
    }

    pub fn http_port(&self) -> u16 {
        self.inner
            .get_host_port_ipv4(HTTP_PORT)
            .expect("Mockly HTTP port should be exposed")
    }

    pub fn api_port(&self) -> u16 {
        self.inner
            .get_host_port_ipv4(API_PORT)
            .expect("Mockly API port should be exposed")
    }

    pub fn http_base(&self) -> String {
        format!("http://127.0.0.1:{}", self.http_port())
    }

    pub fn api_base(&self) -> String {
        format!("http://127.0.0.1:{}", self.api_port())
    }

    pub fn add_mock(&self, mock: &Mock) -> Result<(), Box<dyn std::error::Error>> {
        self.expect_status(
            "POST",
            "/api/mocks/http",
            Some(serde_json::to_value(mock)?),
            &[200, 201],
            "add_mock",
        )
    }

    pub fn list_mocks(&self) -> Result<Vec<Mock>, Box<dyn std::error::Error>> {
        self.get_json("/api/mocks/http")
    }

    pub fn update_mock(&self, id: &str, mock: &Mock) -> Result<Mock, Box<dyn std::error::Error>> {
        self.put_and_read(&format!("/api/mocks/http/{id}"), mock)
    }

    pub fn patch_mock(
        &self,
        id: &str,
        patch: &MockResponsePatch,
    ) -> Result<Mock, Box<dyn std::error::Error>> {
        self.patch_and_read(&format!("/api/mocks/http/{id}"), patch)
    }

    pub fn delete_mock(&self, id: &str) -> Result<(), Box<dyn std::error::Error>> {
        self.expect_status(
            "DELETE",
            &format!("/api/mocks/http/{id}"),
            None,
            &[200],
            "delete_mock",
        )
    }

    pub fn get_state(&self) -> Result<HashMap<String, String>, Box<dyn std::error::Error>> {
        self.get_json("/api/state")
    }

    pub fn set_state(
        &self,
        state: &HashMap<String, String>,
    ) -> Result<HashMap<String, String>, Box<dyn std::error::Error>> {
        self.post_and_read("/api/state", state)
    }

    pub fn delete_state(&self, key: &str) -> Result<(), Box<dyn std::error::Error>> {
        self.expect_status(
            "DELETE",
            &format!("/api/state/{key}"),
            None,
            &[200],
            "delete_state",
        )
    }

    pub fn reset(&self) -> Result<(), Box<dyn std::error::Error>> {
        self.expect_status("POST", "/api/reset", None, &[200], "reset")
    }

    pub fn activate_scenario(&self, id: &str) -> Result<(), Box<dyn std::error::Error>> {
        self.expect_status(
            "POST",
            &format!("/api/scenarios/{id}/activate"),
            None,
            &[200],
            "activate_scenario",
        )
    }

    pub fn deactivate_scenario(&self, id: &str) -> Result<(), Box<dyn std::error::Error>> {
        self.expect_status(
            "POST",
            &format!("/api/scenarios/{id}/deactivate"),
            None,
            &[200],
            "deactivate_scenario",
        )
    }

    pub fn list_scenarios(&self) -> Result<Vec<Scenario>, Box<dyn std::error::Error>> {
        self.get_json("/api/scenarios")
    }

    pub fn create_scenario(
        &self,
        scenario: &Scenario,
    ) -> Result<Scenario, Box<dyn std::error::Error>> {
        self.post_and_read_with_status("/api/scenarios", scenario, 201)
    }

    pub fn get_scenario(&self, id: &str) -> Result<Scenario, Box<dyn std::error::Error>> {
        self.get_json(&format!("/api/scenarios/{id}"))
    }

    pub fn update_scenario(
        &self,
        id: &str,
        scenario: &Scenario,
    ) -> Result<Scenario, Box<dyn std::error::Error>> {
        self.put_and_read(&format!("/api/scenarios/{id}"), scenario)
    }

    pub fn delete_scenario(&self, id: &str) -> Result<(), Box<dyn std::error::Error>> {
        self.expect_status(
            "DELETE",
            &format!("/api/scenarios/{id}"),
            None,
            &[200],
            "delete_scenario",
        )
    }

    pub fn list_active_scenarios(
        &self,
    ) -> Result<ActiveScenariosResponse, Box<dyn std::error::Error>> {
        self.get_json("/api/scenarios/active")
    }

    pub fn set_fault(&self, config: &FaultConfig) -> Result<(), Box<dyn std::error::Error>> {
        self.expect_status(
            "POST",
            "/api/fault/http",
            Some(fault_payload(config)),
            &[200],
            "set_fault",
        )
    }

    pub fn clear_fault(&self) -> Result<(), Box<dyn std::error::Error>> {
        self.expect_status("DELETE", "/api/fault", None, &[200, 204], "clear_fault")
    }

    pub fn get_logs(
        &self,
        matched_id: Option<&str>,
    ) -> Result<Vec<CallEntry>, Box<dyn std::error::Error>> {
        let url = self.with_optional_matched_id("/api/logs", matched_id)?;
        self.get_json_from_url(url, "/api/logs")
    }

    pub fn clear_logs(&self) -> Result<(), Box<dyn std::error::Error>> {
        self.expect_status("DELETE", "/api/logs", None, &[200], "clear_logs")
    }

    pub fn get_logs_count(
        &self,
        matched_id: Option<&str>,
    ) -> Result<i64, Box<dyn std::error::Error>> {
        let url = self.with_optional_matched_id("/api/logs/count", matched_id)?;
        let response: CountResponse = self.get_json_from_url(url, "/api/logs/count")?;
        Ok(response.count)
    }

    pub fn get_calls(&self, mock_id: &str) -> Result<CallSummary, Box<dyn std::error::Error>> {
        self.get_json(&format!("/api/calls/http/{mock_id}"))
    }

    pub fn clear_calls(&self, mock_id: &str) -> Result<(), Box<dyn std::error::Error>> {
        self.expect_status(
            "DELETE",
            &format!("/api/calls/http/{mock_id}"),
            None,
            &[200],
            "clear_calls",
        )
    }

    pub fn clear_all_calls(&self) -> Result<(), Box<dyn std::error::Error>> {
        self.expect_status("DELETE", "/api/calls/http", None, &[200], "clear_all_calls")
    }

    pub fn wait_for_calls(
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
        let mut url = reqwest::Url::parse(&format!("{}{}", self.api_base(), path))?;
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
        let url = reqwest::Url::parse(&format!("{}{}", self.api_base(), path))?;
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

impl From<Container<MocklyImage>> for MocklyContainer {
    fn from(container: Container<MocklyImage>) -> Self {
        Self::new(container)
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
