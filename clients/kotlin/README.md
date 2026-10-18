# mockly-driver-kotlin

> Idiomatic Kotlin DSL and coroutine extensions for [Mockly](https://github.com/dever-labs/mockly), built on top of [`mockly-driver`](../java/README.md).

**Kotlin 1.9+ (JVM 11) · Trailing-lambda DSL · Optional coroutine support**

---

## Table of Contents

- [Why a separate module?](#why-a-separate-module)
- [Quick Start](#quick-start)
- [Maven Dependency](#maven-dependency)
- [DSL Reference](#dsl-reference)
- [Coroutine Support](#coroutine-support)
- [Environment Variables](#environment-variables)

---

## Why a separate module?

[`mockly-driver`](../java/README.md) exposes a Java builder-pattern API (`Mock.builder(...).build()`), which works
fine from Kotlin but isn't idiomatic there. `mockly-driver-kotlin` wraps the exact same client — same binary
download/startup logic, same HTTP API — with:

- **Trailing-lambda DSL builders** for `MocklyConfig`, `Mock`, `Scenario`, `ScenarioPatch`, and `FaultConfig`.
- **Named/default arguments** instead of chained `.foo(x).bar(y)` calls.
- **Optional `suspend` wrappers** around `MocklyServer`'s blocking HTTP calls, for use from coroutines.

It depends on `mockly-driver` directly — no server/protocol logic is duplicated here.

---

## Quick Start

```kotlin
import io.mockly.driver.kotlin.*
import org.junit.jupiter.api.*

class MyServiceTest {

    private lateinit var server: io.mockly.driver.MocklyServer

    @BeforeAll
    fun startServer() {
        server = mocklyServer { httpPort = 0 }
    }

    @AfterAll
    fun stopServer() {
        server.stop()
    }

    @BeforeEach
    fun resetMocks() {
        server.reset()
    }

    @Test
    fun returnsUsers() {
        server.addMock(mock("get-users") {
            request("GET", "/users")
            response(200) {
                body = """[{"id":1,"name":"Alice"}]"""
                header("Content-Type", "application/json")
                delay = "10ms"
            }
        })

        // Make requests against server.httpBase + "/users"
    }
}
```

---

## Maven Dependency

```xml
<dependency>
    <groupId>io.github.dever-labs</groupId>
    <artifactId>mockly-driver-kotlin</artifactId>
    <version>0.16.0</version> <!-- x-release-please-version -->
</dependency>
```

This transitively pulls in `mockly-driver` (same version) and `kotlinx-coroutines-core`.

---

## DSL Reference

### `mocklyConfig { ... }` / `mocklyServer { ... }`

```kotlin
val server = mocklyServer {
    httpPort = 8080            // 0 = pick a free port (default)
    apiPort = 8081
    version = "v0.16.0"        // x-release-please-version
    binDir = "bin"
    startupTimeoutMs = 10_000
    extraArg("--log-level=debug")
    scenario("maintenance", "Maintenance mode") {
        patch("get-user") { status = 503 }
    }
}
```

`mocklyServer { }` is equivalent to `MocklyServer.ensure(mocklyConfig { })`. Use `mocklyConfig { }` alone if you
need the `MocklyConfig` object without starting a server.

### `mock(id) { ... }`

```kotlin
val getUser = mock("get-user") {
    request("GET", "/users/1") {
        header("Accept", "application/json")
    }
    response(200) {
        body = """{"id":1}"""
        delay = "10ms"
        header("Content-Type", "application/json")
    }
}
```

### `scenario(id, name) { ... }`

```kotlin
val maintenance = scenario("maintenance", "Maintenance mode") {
    description = "Returns 503 for get-user"
    patch("get-user") {
        status = 503
        body = """{"error":"unavailable"}"""
        disabled = false
        header("Retry-After", "30")
    }
}

server.createScenario(maintenance)
server.activateScenario(maintenance.id)
```

### `faultConfig { ... }`

```kotlin
server.setFault(faultConfig {
    delay = "200ms"
    errorRate = 0.5
})
server.clearFault()
```

All builder functions return plain `mockly-driver` model objects (`Mock`, `Scenario`, `FaultConfig`, `MocklyConfig`),
so they can be passed directly to any existing `MocklyServer` method.

---

## Coroutine Support

Every `MocklyServer` method performs blocking I/O. For use inside `suspend` functions, this module provides a
generic escape hatch plus a handful of named wrappers that run on `Dispatchers.IO`:

```kotlin
import io.mockly.driver.kotlin.*

suspend fun setUp(server: io.mockly.driver.MocklyServer) {
    server.addMockSuspending(myMock)
    server.activateScenarioSuspending("maintenance")
    server.setFaultSuspending(faultConfig { delay = "100ms" })

    // Escape hatch for calls without a named wrapper:
    val logs = server.io { getLogs() }
}

suspend fun startServer() = mocklyServerSuspending { httpPort = 0 }
```

| Wrapper | Wraps |
|---|---|
| `mocklyServerSuspending { }` | `mocklyServer { }` |
| `server.addMockSuspending(mock)` | `server.addMock(mock)` |
| `server.resetSuspending()` | `server.reset()` |
| `server.createScenarioSuspending(scenario)` | `server.createScenario(scenario)` |
| `server.activateScenarioSuspending(id)` | `server.activateScenario(id)` |
| `server.deactivateScenarioSuspending(id)` | `server.deactivateScenario(id)` |
| `server.setFaultSuspending(config)` | `server.setFault(config)` |
| `server.clearFaultSuspending()` | `server.clearFault()` |
| `server.waitForCallsSuspending(mockId, count, timeout)` | `server.waitForCalls(mockId, count, timeout)` |
| `server.io { ... }` | Any other blocking `MocklyServer` call |

---

## Environment Variables

Same as [`mockly-driver`](../java/README.md#environment-variables): `MOCKLY_BINARY_PATH`, `MOCKLY_VERSION`,
`MOCKLY_DOWNLOAD_BASE_URL`, `MOCKLY_NO_INSTALL`.

---

## License

[MIT](LICENSE)
