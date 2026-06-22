use reqwest::blocking::{Client, Response};
use serde_json::{Map, Value};
use testcontainers::core::Container;

use crate::image::{MocklyImage, API_PORT, HTTP_PORT};
use mockly_driver::{FaultConfig, Mock};

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

    pub fn delete_mock(&self, id: &str) -> Result<(), Box<dyn std::error::Error>> {
        self.expect_status(
            "DELETE",
            &format!("/api/mocks/http/{id}"),
            None,
            &[204],
            "delete_mock",
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

    pub fn get_logs(&self) -> Result<String, Box<dyn std::error::Error>> {
        let resp = self.request("GET", "/api/logs", None)?;
        let status = resp.status().as_u16();
        let body = resp.text()?;
        if status == 200 {
            Ok(body)
        } else {
            Err(format!("get_logs failed: expected HTTP 200, got {status}: {body}").into())
        }
    }

    pub fn clear_logs(&self) -> Result<(), Box<dyn std::error::Error>> {
        self.expect_status("DELETE", "/api/logs", None, &[200], "clear_logs")
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
            Err(
                format!("{op} failed: expected one of {:?}, got {status}: {body}", expected)
                    .into(),
            )
        }
    }

    fn request(
        &self,
        method: &str,
        path: &str,
        body: Option<Value>,
    ) -> Result<Response, Box<dyn std::error::Error>> {
        let url = format!("{}{}", self.api_base(), path);
        let builder = match method {
            "GET" => self.client.get(&url),
            "POST" => self.client.post(&url),
            "DELETE" => self.client.delete(&url),
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
    if let Some(status) = config.status_override {
        map.insert("status".to_string(), Value::Number(status.into()));
    }
    if let Some(error_rate) = config.error_rate {
        if let Some(number) = serde_json::Number::from_f64(error_rate) {
            map.insert("error_rate".to_string(), Value::Number(number));
        }
    }

    Value::Object(map)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn fault_payload_maps_status_override_to_status() {
        let payload = fault_payload(&FaultConfig {
            enabled: true,
            delay: Some("25ms".to_string()),
            status_override: Some(503),
            error_rate: Some(0.25),
        });

        assert_eq!(payload["delay"], "25ms");
        assert_eq!(payload["status"], 503);
        assert_eq!(payload["error_rate"], 0.25);
        assert!(payload.get("status_override").is_none());
        assert!(payload.get("enabled").is_none());
    }
}
