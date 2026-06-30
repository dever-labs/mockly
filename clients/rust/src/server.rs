use crate::install::install;
use crate::types::{
    ActiveScenariosResponse, CallEntry, CallSummary, FaultConfig, InstallOptions, Mock,
    MockResponsePatch, Scenario, ServerOptions,
};
use crate::utils::{get_free_port, is_port_conflict};
use std::collections::HashMap;
use std::io::Write;
use std::process::{Child, Command, Stdio};
use tempfile::NamedTempFile;

pub struct MocklyServer {
    pub http_port: u16,
    pub api_port: u16,
    pub http_base: String,
    pub api_base: String,
    proc: Child,
    client: reqwest::blocking::Client,
}

impl MocklyServer {
    /// Start server using an already-installed binary. Retries up to 3 times on port conflict.
    pub fn create(opts: ServerOptions) -> Result<Self, Box<dyn std::error::Error>> {
        let bin = crate::install::get_binary_path(None)
            .ok_or("Mockly binary not found; use MocklyServer::ensure() to install it first")?;
        start_with_retries(&bin, opts, 3)
    }

    /// Install the binary if needed, then start the server.
    pub fn ensure(
        opts: ServerOptions,
        install_opts: InstallOptions,
    ) -> Result<Self, Box<dyn std::error::Error>> {
        let bin = install(install_opts)?;
        start_with_retries(&bin, opts, 3)
    }

    pub fn stop(&mut self) -> Result<(), Box<dyn std::error::Error>> {
        self.proc.kill()?;
        self.proc.wait()?;
        Ok(())
    }

    pub fn add_mock(&self, mock: &Mock) -> Result<(), Box<dyn std::error::Error>> {
        let resp = self
            .client
            .post(format!("{}/api/mocks/http", self.api_base))
            .json(mock)
            .send()?;
        check_status(resp, 201, "add_mock")
    }

    pub fn list_mocks(&self) -> Result<Vec<Mock>, Box<dyn std::error::Error>> {
        let resp = self
            .client
            .get(format!("{}/api/mocks/http", self.api_base))
            .send()?;
        expect_json(resp, 200, "list_mocks")
    }

    pub fn update_mock(
        &self,
        id: &str,
        mock: &Mock,
    ) -> Result<Mock, Box<dyn std::error::Error>> {
        let resp = self
            .client
            .put(format!("{}/api/mocks/http/{}", self.api_base, id))
            .json(mock)
            .send()?;
        expect_json(resp, 200, "update_mock")
    }

    pub fn patch_mock(
        &self,
        id: &str,
        patch: &MockResponsePatch,
    ) -> Result<Mock, Box<dyn std::error::Error>> {
        let resp = self
            .client
            .patch(format!("{}/api/mocks/http/{}", self.api_base, id))
            .json(patch)
            .send()?;
        expect_json(resp, 200, "patch_mock")
    }

    pub fn delete_mock(&self, id: &str) -> Result<(), Box<dyn std::error::Error>> {
        let resp = self
            .client
            .delete(format!("{}/api/mocks/http/{}", self.api_base, id))
            .send()?;
        check_status(resp, 200, "delete_mock")
    }

    pub fn get_state(&self) -> Result<HashMap<String, String>, Box<dyn std::error::Error>> {
        let resp = self
            .client
            .get(format!("{}/api/state", self.api_base))
            .send()?;
        expect_json(resp, 200, "get_state")
    }

    pub fn set_state(
        &self,
        state: &HashMap<String, String>,
    ) -> Result<HashMap<String, String>, Box<dyn std::error::Error>> {
        let resp = self
            .client
            .post(format!("{}/api/state", self.api_base))
            .json(state)
            .send()?;
        expect_json(resp, 200, "set_state")
    }

    pub fn delete_state(&self, key: &str) -> Result<(), Box<dyn std::error::Error>> {
        let resp = self
            .client
            .delete(format!("{}/api/state/{}", self.api_base, key))
            .send()?;
        check_status(resp, 200, "delete_state")
    }

    pub fn get_logs(
        &self,
        matched_id: Option<&str>,
    ) -> Result<Vec<CallEntry>, Box<dyn std::error::Error>> {
        let url = with_optional_matched_id(format!("{}/api/logs", self.api_base), matched_id)?;
        let resp = self.client.get(url).send()?;
        expect_json(resp, 200, "get_logs")
    }

    pub fn clear_logs(&self) -> Result<(), Box<dyn std::error::Error>> {
        let resp = self
            .client
            .delete(format!("{}/api/logs", self.api_base))
            .send()?;
        check_status(resp, 200, "clear_logs")
    }

    pub fn get_logs_count(&self, matched_id: Option<&str>) -> Result<i64, Box<dyn std::error::Error>> {
        let url = with_optional_matched_id(format!("{}/api/logs/count", self.api_base), matched_id)?;
        let resp = self.client.get(url).send()?;
        let count: CountResponse = expect_json(resp, 200, "get_logs_count")?;
        Ok(count.count)
    }

    pub fn list_scenarios(&self) -> Result<Vec<Scenario>, Box<dyn std::error::Error>> {
        let resp = self
            .client
            .get(format!("{}/api/scenarios", self.api_base))
            .send()?;
        expect_json(resp, 200, "list_scenarios")
    }

    pub fn create_scenario(&self, scenario: &Scenario) -> Result<Scenario, Box<dyn std::error::Error>> {
        let resp = self
            .client
            .post(format!("{}/api/scenarios", self.api_base))
            .json(scenario)
            .send()?;
        expect_json(resp, 201, "create_scenario")
    }

    pub fn get_scenario(&self, id: &str) -> Result<Scenario, Box<dyn std::error::Error>> {
        let resp = self
            .client
            .get(format!("{}/api/scenarios/{}", self.api_base, id))
            .send()?;
        expect_json(resp, 200, "get_scenario")
    }

    pub fn update_scenario(
        &self,
        id: &str,
        scenario: &Scenario,
    ) -> Result<Scenario, Box<dyn std::error::Error>> {
        let resp = self
            .client
            .put(format!("{}/api/scenarios/{}", self.api_base, id))
            .json(scenario)
            .send()?;
        expect_json(resp, 200, "update_scenario")
    }

    pub fn delete_scenario(&self, id: &str) -> Result<(), Box<dyn std::error::Error>> {
        let resp = self
            .client
            .delete(format!("{}/api/scenarios/{}", self.api_base, id))
            .send()?;
        check_status(resp, 200, "delete_scenario")
    }

    pub fn list_active_scenarios(
        &self,
    ) -> Result<ActiveScenariosResponse, Box<dyn std::error::Error>> {
        let resp = self
            .client
            .get(format!("{}/api/scenarios/active", self.api_base))
            .send()?;
        expect_json(resp, 200, "list_active_scenarios")
    }

    pub fn reset(&self) -> Result<(), Box<dyn std::error::Error>> {
        let resp = self
            .client
            .post(format!("{}/api/reset", self.api_base))
            .send()?;
        check_status(resp, 200, "reset")
    }

    pub fn activate_scenario(&self, id: &str) -> Result<(), Box<dyn std::error::Error>> {
        let resp = self
            .client
            .post(format!("{}/api/scenarios/{}/activate", self.api_base, id))
            .send()?;
        check_status(resp, 200, "activate_scenario")
    }

    pub fn deactivate_scenario(&self, id: &str) -> Result<(), Box<dyn std::error::Error>> {
        let resp = self
            .client
            .post(format!(
                "{}/api/scenarios/{}/deactivate",
                self.api_base, id
            ))
            .send()?;
        check_status(resp, 200, "deactivate_scenario")
    }

    pub fn set_fault(&self, config: &FaultConfig) -> Result<(), Box<dyn std::error::Error>> {
        let resp = self
            .client
            .post(format!("{}/api/fault/http", self.api_base))
            .json(config)
            .send()?;
        check_status(resp, 200, "set_fault")
    }

    pub fn clear_fault(&self) -> Result<(), Box<dyn std::error::Error>> {
        let resp = self
            .client
            .delete(format!("{}/api/fault", self.api_base))
            .send()?;
        let status = resp.status().as_u16();
        if status == 200 || status == 204 {
            Ok(())
        } else {
            Err(format!("clear_fault failed: expected HTTP 200/204, got {}", status).into())
        }
    }

    /// Returns recorded calls for the given mock ID.
    pub fn get_calls(&self, mock_id: &str) -> Result<CallSummary, Box<dyn std::error::Error>> {
        let resp = self
            .client
            .get(format!("{}/api/calls/http/{}", self.api_base, mock_id))
            .send()?;
        expect_json(resp, 200, "get_calls")
    }

    /// Clears recorded calls for the given mock ID.
    pub fn clear_calls(&self, mock_id: &str) -> Result<(), Box<dyn std::error::Error>> {
        let resp = self
            .client
            .delete(format!("{}/api/calls/http/{}", self.api_base, mock_id))
            .send()?;
        check_status(resp, 200, "clear_calls")
    }

    /// Clears all recorded HTTP calls across every mock.
    pub fn clear_all_calls(&self) -> Result<(), Box<dyn std::error::Error>> {
        let resp = self
            .client
            .delete(format!("{}/api/calls/http", self.api_base))
            .send()?;
        check_status(resp, 200, "clear_all_calls")
    }

    /// Blocks until mock_id has been called at least `count` times, or until
    /// `timeout_secs` seconds elapse. Returns the recorded calls on success.
    pub fn wait_for_calls(
        &self,
        mock_id: &str,
        count: u32,
        timeout_secs: u64,
    ) -> Result<CallSummary, Box<dyn std::error::Error>> {
        let body = serde_json::json!({
            "count": count,
            "timeout": format!("{}s", timeout_secs),
        });
        let resp = self
            .client
            .post(format!("{}/api/calls/http/{}/wait", self.api_base, mock_id))
            .json(&body)
            .send()?;
        let status = resp.status().as_u16();
        if status == 408 {
            return Err(format!(
                "wait_for_calls: timeout waiting for {} call(s) on '{}'",
                count, mock_id
            )
            .into());
        }
        if status != 200 {
            return Err(format!("wait_for_calls failed: expected HTTP 200, got {}", status).into());
        }
        Ok(resp.json()?)
    }
}

impl Drop for MocklyServer {
    fn drop(&mut self) {
        let _ = self.stop();
    }
}

#[derive(serde::Deserialize)]
struct CountResponse {
    count: i64,
}

// ── internals ────────────────────────────────────────────────────────────────

fn start_with_retries(
    bin: &std::path::Path,
    opts: ServerOptions,
    max_retries: u32,
) -> Result<MocklyServer, Box<dyn std::error::Error>> {
    let mut last_err: Box<dyn std::error::Error> = "no attempts made".into();

    for attempt in 0..=max_retries {
        let http_port = get_free_port()?;
        let api_port = get_free_port()?;

        match start(bin, http_port, api_port, &opts.scenarios) {
            Ok(server) => return Ok(server),
            Err(e) if attempt < max_retries && is_port_conflict(&e.to_string()) => {
                last_err = e;
                continue;
            }
            Err(e) => return Err(e),
        }
    }

    Err(last_err)
}

fn start(
    bin: &std::path::Path,
    http_port: u16,
    api_port: u16,
    scenarios: &[Scenario],
) -> Result<MocklyServer, Box<dyn std::error::Error>> {
    let config_file = write_config(http_port, api_port, scenarios)?;
    let config_path = config_file.path().to_owned();

    let child = Command::new(bin)
        .args(["start", "--config", config_path.to_str().unwrap()])
        .stdout(Stdio::null())
        .stderr(Stdio::piped())
        .spawn()
        .map_err(|e| format!("Failed to spawn Mockly binary: {}", e))?;

    let api_base = format!("http://127.0.0.1:{}", api_port);
    let http_base = format!("http://127.0.0.1:{}", http_port);
    let client = reqwest::blocking::Client::new();

    let mut server = MocklyServer {
        http_port,
        api_port,
        http_base,
        api_base,
        proc: child,
        client,
    };

    match wait_ready(&server.client, &server.api_base, 5_000) {
        Ok(()) => {
            drop(config_file);
            Ok(server)
        }
        Err(e) => {
            let stderr = collect_stderr(&mut server.proc);
            let _ = server.proc.kill();
            let _ = server.proc.wait();
            Err(format!(
                "Mockly did not become ready within timeout: {}
stderr: {}",
                e, stderr
            )
            .into())
        }
    }
}

fn write_config(
    http_port: u16,
    api_port: u16,
    scenarios: &[Scenario],
) -> Result<NamedTempFile, Box<dyn std::error::Error>> {
    let mut yaml = format!(
        "mockly:
  api:
    port: {}
protocols:
  http:
    enabled: true
    port: {}
",
        api_port, http_port
    );

    if !scenarios.is_empty() {
        yaml.push_str("scenarios:
");
        for s in scenarios {
            yaml.push_str(&format!("  - id: {}
    name: {}
", yaml_str(&s.id), yaml_str(&s.name)));
            if let Some(description) = &s.description {
                yaml.push_str(&format!("    description: {}
", yaml_str(description)));
            }
            if !s.patches.is_empty() {
                yaml.push_str("    patches:
");
                for p in &s.patches {
                    yaml.push_str(&format!("      - mock_id: {}
", yaml_str(&p.mock_id)));
                    if let Some(status) = p.status {
                        yaml.push_str(&format!("        status: {}
", status));
                    }
                    if let Some(ref body) = p.body {
                        yaml.push_str(&format!("        body: {}
", yaml_str(body)));
                    }
                    if let Some(ref headers) = p.headers {
                        if !headers.is_empty() {
                            yaml.push_str("        headers:
");
                            for (key, value) in headers {
                                yaml.push_str(&format!(
                                    "          {}: {}
",
                                    yaml_str(key),
                                    yaml_str(value)
                                ));
                            }
                        }
                    }
                    if let Some(ref delay) = p.delay {
                        yaml.push_str(&format!("        delay: {}
", yaml_str(delay)));
                    }
                    if let Some(disabled) = p.disabled {
                        yaml.push_str(&format!("        disabled: {}
", disabled));
                    }
                }
            }
        }
    }

    let mut file = NamedTempFile::new()?;
    file.write_all(yaml.as_bytes())?;
    Ok(file)
}

/// Minimal YAML single-quote escaper.
fn yaml_str(s: &str) -> String {
    format!("'{}'", s.replace('\'', "''"))
}

fn wait_ready(
    client: &reqwest::blocking::Client,
    api_base: &str,
    max_ms: u64,
) -> Result<(), Box<dyn std::error::Error>> {
    let deadline = std::time::Instant::now() + std::time::Duration::from_millis(max_ms);
    let poll = std::time::Duration::from_millis(50);

    while std::time::Instant::now() < deadline {
        if let Ok(resp) = client.get(format!("{}/api/protocols", api_base)).send() {
            if resp.status().is_success() {
                return Ok(());
            }
        }
        std::thread::sleep(poll);
    }

    Err(format!("server did not respond on {} within {}ms", api_base, max_ms).into())
}

fn collect_stderr(child: &mut Child) -> String {
    use std::io::Read;
    child
        .stderr
        .take()
        .map(|mut s| {
            let mut buf = String::new();
            let _ = s.read_to_string(&mut buf);
            buf
        })
        .unwrap_or_default()
}

fn expect_json<T: serde::de::DeserializeOwned>(
    resp: reqwest::blocking::Response,
    expected: u16,
    op: &str,
) -> Result<T, Box<dyn std::error::Error>> {
    let status = resp.status().as_u16();
    if status != expected {
        return Err(format!("{} failed: expected HTTP {}, got {}", op, expected, status).into());
    }
    Ok(resp.json()?)
}

fn check_status(
    resp: reqwest::blocking::Response,
    expected: u16,
    op: &str,
) -> Result<(), Box<dyn std::error::Error>> {
    let status = resp.status().as_u16();
    if status == expected {
        Ok(())
    } else {
        Err(format!("{} failed: expected HTTP {}, got {}", op, expected, status).into())
    }
}

fn with_optional_matched_id(
    base: String,
    matched_id: Option<&str>,
) -> Result<reqwest::Url, Box<dyn std::error::Error>> {
    let mut url = reqwest::Url::parse(&base)?;
    if let Some(matched_id) = matched_id {
        url.query_pairs_mut().append_pair("matched_id", matched_id);
    }
    Ok(url)
}

/// Test helpers – accessible from integration tests via `mockly_driver::server::test_helpers`.
#[doc(hidden)]
pub mod test_helpers {
    use super::{write_config, yaml_str, MocklyServer};
    use crate::types::Scenario;
    use tempfile::NamedTempFile;

    pub fn yaml_str_for_test(s: &str) -> String {
        yaml_str(s)
    }

    pub fn write_config_for_test(
        http_port: u16,
        api_port: u16,
        scenarios: &[Scenario],
    ) -> Result<NamedTempFile, Box<dyn std::error::Error>> {
        write_config(http_port, api_port, scenarios)
    }

    /// Construct a `MocklyServer` with a no-op child process for unit testing.
    /// The `http_port` is unused by API methods; only `api_port` matters.
    pub fn new_server_for_test(http_port: u16, api_port: u16) -> MocklyServer {
        use std::process::Command;
        let proc = if cfg!(windows) {
            Command::new("cmd").args(["/c", "exit 0"]).spawn().unwrap()
        } else {
            Command::new("sh").args(["-c", "exit 0"]).spawn().unwrap()
        };
        MocklyServer {
            http_port,
            api_port,
            http_base: format!("http://127.0.0.1:{}", http_port),
            api_base: format!("http://127.0.0.1:{}", api_port),
            proc,
            client: reqwest::blocking::Client::new(),
        }
    }
}
