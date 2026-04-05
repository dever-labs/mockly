# mockly-driver

> Java client for [Mockly](https://github.com/dever-labs/mockly) — start/stop servers and manage HTTP mocks in tests.

**Java 11+ · Zero runtime dependencies · JUnit 5 friendly**

---

## Table of Contents

- [Quick Start](#quick-start)
- [Maven Dependency](#maven-dependency)
- [API Reference](#api-reference)
- [Environment Variables](#environment-variables)
- [Proxy Support](#proxy-support)
- [Air-Gap / Artifactory](#air-gap--artifactory)

---

## Quick Start

```java
import io.mockly.driver.*;
import io.mockly.driver.model.*;
import org.junit.jupiter.api.*;

@TestInstance(TestInstance.Lifecycle.PER_CLASS)
class MyServiceTest {

    private MocklyServer server;

    @BeforeAll
    void startServer() throws Exception {
        server = MocklyServer.ensure(MocklyConfig.builder().build());
    }

    @AfterAll
    void stopServer() throws Exception {
        if (server != null) server.stop();
    }

    @BeforeEach
    void resetMocks() throws Exception {
        server.reset();
    }

    @Test
    void returnsUsers() throws Exception {
        server.addMock(Mock.builder(
            "get-users",
            MockRequest.builder("GET", "/users")
                .header("Authorization", "Bearer token")
                .build(),
            MockResponse.builder(200)
                .body("[{\"id\":1,\"name\":\"Alice\"}]")
                .header("Content-Type", "application/json")
                .delay("10ms")
                .build()
        ).build());

        // Make requests against server.httpBase + "/users"
        // e.g. new URL(server.httpBase + "/users").openConnection()
    }
}
```

---

## Maven Dependency

```xml
<dependency>
    <groupId>io.github.dever-labs</groupId>
    <artifactId>mockly-driver</artifactId>
    <version>0.1.0</version>
    <scope>test</scope>
</dependency>
```

---

## API Reference

### `MocklyConfig`

Builder for server configuration.

```java
MocklyConfig config = MocklyConfig.builder()
    .httpPort(9000)          // fixed HTTP mock port (0 = pick free port)
    .apiPort(9001)           // fixed management API port (0 = pick free port)
    .version("v0.1.0")       // binary version to download
    .binDir("bin")           // directory to search / install binary
    .binaryPath("/usr/local/bin/mockly") // explicit binary path (skips search)
    .startupTimeoutMs(10_000)            // how long to wait for readiness
    .extraArg("--log-level=debug")       // additional CLI flags
    .build();
```

### `MocklyServer`

| Method | Description |
|---|---|
| `MocklyServer.create(config)` | Start using an already-installed binary. Throws if not found. |
| `MocklyServer.ensure(config)` | Download binary if missing, then start. |
| `server.stop()` | Stop the process and clean up the config file. |
| `server.close()` | Same as `stop()`. Enables try-with-resources. |
| `server.addMock(mock)` | Register an HTTP mock (`POST /api/mocks/http`). |
| `server.deleteMock(id)` | Remove an HTTP mock (`DELETE /api/mocks/http/{id}`). |
| `server.reset()` | Reset all mocks (`POST /api/reset`). |
| `server.activateScenario(id)` | Activate a scenario (`POST /api/scenarios/{id}/activate`). |
| `server.deactivateScenario(id)` | Deactivate a scenario. |
| `server.setFault(config)` | Inject a network fault (`POST /api/fault`). |
| `server.clearFault()` | Clear the active fault (`DELETE /api/fault`). |

Public fields on the server instance:

| Field | Description |
|---|---|
| `server.httpPort` | HTTP mock port |
| `server.apiPort` | Management API port |
| `server.httpBase` | `http://127.0.0.1:<httpPort>` |
| `server.apiBase` | `http://127.0.0.1:<apiPort>` |

### Model Classes

#### `Mock`

```java
Mock mock = Mock.builder(
    "my-mock-id",
    MockRequest.builder("GET", "/path").build(),
    MockResponse.builder(200).body("hello").build()
).build();
```

#### `MockRequest`

```java
MockRequest req = MockRequest.builder("POST", "/submit")
    .header("Content-Type", "application/json")
    .headers(Map.of("X-Custom", "value"))
    .build();
```

#### `MockResponse`

```java
MockResponse resp = MockResponse.builder(201)
    .body("{\"created\":true}")
    .header("Content-Type", "application/json")
    .delay("100ms")   // response delay
    .build();
```

#### `FaultConfig`

```java
FaultConfig fault = FaultConfig.builder("delay")
    .delay("500ms")
    .probability(0.3)  // 30% of requests
    .build();

server.setFault(fault);
// ...
server.clearFault();
```

#### `Scenario` / `ScenarioPatch`

```java
Scenario scenario = Scenario.builder("happy-path")
    .description("All services healthy")
    .mock(myMock)
    .build();

server.activateScenario("happy-path");
```

### `MocklyInstaller`

```java
// Locate existing binary.
String path = MocklyInstaller.getBinaryPath("bin");

// Download binary (respects environment variables).
String path = MocklyInstaller.install(
    MocklyInstaller.InstallOptions.builder()
        .binDir("bin")
        .version("v0.1.0")
        .build()
);
```

---

## Environment Variables

| Variable | Description |
|---|---|
| `MOCKLY_BINARY_PATH` | Absolute path to a pre-staged Mockly binary. Skips all download logic. |
| `MOCKLY_VERSION` | Binary version to download (overrides `MocklyConfig.version`). |
| `MOCKLY_DOWNLOAD_BASE_URL` | Base URL override for binary downloads (e.g. an Artifactory mirror). The full URL is `{base}/{version}/{asset}`. |
| `MOCKLY_NO_INSTALL` | When set to any value, throws instead of attempting a download. Useful in CI where the binary must be pre-staged. |

---

## Proxy Support

`MocklyInstaller` uses `java.net.http.HttpClient`, which honours the JVM proxy selector.
On most JVMs the default selector picks up the following **system properties**:

```
-Dhttps.proxyHost=proxy.example.com
-Dhttps.proxyPort=8080
-Dhttps.nonProxyHosts=localhost|127.*
```

Add these to your Maven Surefire configuration:

```xml
<plugin>
    <groupId>org.apache.maven.plugins</groupId>
    <artifactId>maven-surefire-plugin</artifactId>
    <configuration>
        <systemPropertyVariables>
            <https.proxyHost>${env.HTTPS_PROXY_HOST}</https.proxyHost>
            <https.proxyPort>${env.HTTPS_PROXY_PORT}</https.proxyPort>
        </systemPropertyVariables>
    </configuration>
</plugin>
```

Alternatively, on many JVMs setting the `HTTPS_PROXY` environment variable is also honoured by the system proxy selector.

---

## Air-Gap / Artifactory

In air-gapped environments, mirror the Mockly release assets to your internal registry and set:

```bash
export MOCKLY_DOWNLOAD_BASE_URL=https://artifactory.example.com/mockly/releases/download
export MOCKLY_VERSION=v0.1.0
```

The client will download from `{MOCKLY_DOWNLOAD_BASE_URL}/{MOCKLY_VERSION}/{asset}`.

Alternatively, pre-stage the binary and point to it directly:

```bash
export MOCKLY_BINARY_PATH=/opt/tools/mockly
export MOCKLY_NO_INSTALL=1  # fail fast if the binary is missing
```

---

## Binary Asset Names

| Platform | Asset |
|---|---|
| Linux x64 | `mockly-linux-amd64` |
| Linux arm64 | `mockly-linux-arm64` |
| macOS x64 | `mockly-darwin-amd64` |
| macOS arm64 | `mockly-darwin-arm64` |
| Windows x64 | `mockly-windows-amd64.exe` |

---

## License

[MIT](LICENSE)
