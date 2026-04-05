# mockly-driver

Rust client for [Mockly](https://github.com/dever-labs/mockly) — a fast HTTP mock server for integration testing.

## Features

- Start and stop a local Mockly process from test code
- Register, delete, and reset HTTP mocks
- Activate and deactivate test scenarios
- Inject network faults (delay, error rate, status override)
- Automatic binary download (or point at a pre-staged binary)
- Synchronous API — works with standard `#[test]` without any async runtime

## Installation

```toml
[dev-dependencies]
mockly-driver = "0.1.0"
```

## Quick start

```rust
use mockly_driver::{MocklyServer, ServerOptions, Mock, MockRequest, MockResponse, InstallOptions};
use std::collections::HashMap;

#[test]
fn test_get_users() {
    // Start server (downloads binary on first run)
    let mut server = MocklyServer::ensure(
        ServerOptions::default(),
        InstallOptions::default(),
    ).expect("start mockly");

    // Register a mock
    server.add_mock(&Mock {
        id: "get-users".into(),
        request: MockRequest {
            method: "GET".into(),
            path: "/users".into(),
            headers: HashMap::new(),
        },
        response: MockResponse {
            status: 200,
            body: Some(r#"[{"id":1}]"#.into()),
            headers: [("Content-Type".into(), "application/json".into())].into(),
            delay: None,
        },
    }).expect("add mock");

    // Hit the mock server
    let resp = reqwest::blocking::get(
        format!("{}/users", server.http_base)
    ).expect("request");
    assert_eq!(resp.status(), 200);

    server.stop().expect("stop");
}
```

## Environment variables

| Variable | Description |
|---|---|
| `MOCKLY_BINARY_PATH` | Absolute path to a pre-staged binary |
| `MOCKLY_DOWNLOAD_BASE_URL` | Base URL override (Artifactory / mirrors) |
| `MOCKLY_VERSION` | Version override (default: `v0.1.0`) |
| `MOCKLY_NO_INSTALL` | Return an error instead of downloading |
| `HTTPS_PROXY` / `HTTP_PROXY` | Proxy for downloads (handled by reqwest) |

## API

### `MocklyServer`

| Method | Description |
|---|---|
| `MocklyServer::create(opts)` | Start with an already-installed binary |
| `MocklyServer::ensure(opts, install_opts)` | Install binary if needed, then start |
| `server.stop()` | Kill the process |
| `server.add_mock(&mock)` | `POST /api/mocks/http` |
| `server.delete_mock(id)` | `DELETE /api/mocks/http/{id}` |
| `server.reset()` | `POST /api/reset` |
| `server.activate_scenario(id)` | `POST /api/scenarios/{id}/activate` |
| `server.deactivate_scenario(id)` | `POST /api/scenarios/{id}/deactivate` |
| `server.set_fault(&config)` | `POST /api/fault` |
| `server.clear_fault()` | `DELETE /api/fault` |

`MocklyServer` also implements `Drop`, so the process is killed automatically when it goes out of scope.

### `install(opts) -> Result<PathBuf>`

Download (or locate) the Mockly binary.

### `get_free_port() -> Result<u16>`

Bind to `127.0.0.1:0` and return the OS-assigned port. Useful for custom setups.

## License

MIT
