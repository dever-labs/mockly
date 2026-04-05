use crate::install::install;
use crate::types::{FaultConfig, InstallOptions, Mock, Scenario, ServerOptions};
use crate::utils::{get_free_port, is_port_conflict};
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

    pub fn delete_mock(&self, id: &str) -> Result<(), Box<dyn std::error::Error>> {
        let resp = self
            .client
            .delete(format!("{}/api/mocks/http/{}", self.api_base, id))
            .send()?;
        check_status(resp, 204, "delete_mock")
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
            .post(format!("{}/api/fault", self.api_base))
            .json(config)
            .send()?;
        check_status(resp, 200, "set_fault")
    }

    pub fn clear_fault(&self) -> Result<(), Box<dyn std::error::Error>> {
        let resp = self
            .client
            .delete(format!("{}/api/fault", self.api_base))
            .send()?;
        check_status(resp, 200, "clear_fault")
    }
}

impl Drop for MocklyServer {
    fn drop(&mut self) {
        let _ = self.stop();
    }
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

    // Keep the config file alive until the server is ready, then let it drop.
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
                "Mockly did not become ready within timeout: {}\nstderr: {}",
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
        "mockly:\n  api:\n    port: {}\nprotocols:\n  http:\n    enabled: true\n    port: {}\n",
        api_port, http_port
    );

    if !scenarios.is_empty() {
        yaml.push_str("scenarios:\n");
        for s in scenarios {
            yaml.push_str(&format!("  - id: {}\n    name: {}\n", yaml_str(&s.id), yaml_str(&s.name)));
            if !s.patches.is_empty() {
                yaml.push_str("    patches:\n");
                for p in &s.patches {
                    yaml.push_str(&format!("      - mock_id: {}\n", yaml_str(&p.mock_id)));
                    if let Some(status) = p.status {
                        yaml.push_str(&format!("        status: {}\n", status));
                    }
                    if let Some(ref body) = p.body {
                        yaml.push_str(&format!("        body: {}\n", yaml_str(body)));
                    }
                    if let Some(ref delay) = p.delay {
                        yaml.push_str(&format!("        delay: {}\n", yaml_str(delay)));
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
