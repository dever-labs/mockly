# Mockly — Kotlin Client

`mockly-driver-kotlin` is a thin, idiomatic Kotlin wrapper over the [Java client](java.md): same binary
download/startup logic, same HTTP protocol, no duplicated functionality. It replaces the Java client's
builder-pattern API (`Mock.builder(...).build()`) with trailing-lambda DSL builders and named/default
arguments, and adds optional `suspend` wrappers for use from coroutines.

## Install

### Maven

```xml
<dependency>
  <groupId>io.github.dever-labs</groupId>
  <artifactId>mockly-driver-kotlin</artifactId>
  <version>0.16.0</version> <!-- x-release-please-version -->
  <scope>test</scope>
</dependency>
```

### Gradle

```kotlin
testImplementation("io.github.dever-labs:mockly-driver-kotlin:0.16.0") // x-release-please-version
```

## Quickstart

```kotlin
import io.mockly.driver.kotlin.*

val server = mocklyServer { httpPort = 0 }

server.addMock(mock("get-user") {
    request("GET", "/users/1")
    response(200) {
        body = """{"id":1,"name":"Alice"}"""
        header("Content-Type", "application/json")
    }
})

// Point your service under test at server.httpBase
// e.g. "http://127.0.0.1:45678"

server.stop()
```

`mocklyServer { }` is equivalent to `MocklyServer.ensure(mocklyConfig { })` on the Java client — it downloads
the binary if missing, then starts the server.

## Configuration

```kotlin
import io.mockly.driver.kotlin.*

val server = mocklyServer {
    httpPort = 8080            // fixed HTTP mock port (0 = pick free port, the default)
    apiPort = 8081              // fixed management API port (0 = pick free port)
    version = "v0.16.0"          // x-release-please-version
    binDir = "bin"              // directory to search / install binary
    startupTimeoutMs = 15_000
    extraArg("--log-level=debug")
    scenario("payment-fail", "Payment Failure") {
        patch("charge") {
            status = 503
            body = """{"error":"unavailable"}"""
        }
    }
}
```

## DSL builders

### `mock(id) { ... }`

```kotlin
val getOrders = mock("get-orders") {
    request("GET", "/orders") {
        header("Authorization", "******")
    }
    response(200) {
        body = """[{"id":1}]"""
        header("Content-Type", "application/json")
        delay = "100ms"
    }
}

server.addMock(getOrders)
```

### `scenario(id, name) { ... }`

```kotlin
val slowCheckout = scenario("slow-checkout", "Slow checkout") {
    description = "Used for retry-path tests"
    patch("charge") {
        status = 503
        delay = "750ms"
    }
}

server.createScenario(slowCheckout)
server.activateScenario("slow-checkout")
// ...
server.deactivateScenario("slow-checkout")
server.deleteScenario("slow-checkout")
```

### `faultConfig { ... }`

```kotlin
server.setFault(faultConfig {
    delay = "500ms"
    status = 503
    errorRate = 0.5
})
server.clearFault()
```

Every builder returns a plain `mockly-driver` model object (`Mock`, `Scenario`, `FaultConfig`, `MocklyConfig`),
so it can also be passed to any `MocklyServer` method directly — the DSL doesn't replace or hide the Java API,
it just makes constructing these objects more idiomatic in Kotlin.

## Everything else from the Java client works unchanged

Call verification, state, logs, reset/stop, and the `server.httpBase` / `server.apiBase` fields all come
straight from `mockly-driver` — see the [Java client docs](java.md#api-reference) for the full reference.

## Coroutine support

`MocklyServer`'s methods are blocking. For `suspend` functions, this module provides a generic `io { }` wrapper
plus named `suspend` equivalents for the most common operations, all running on `Dispatchers.IO`:

```kotlin
import io.mockly.driver.kotlin.*

suspend fun setUp(server: io.mockly.driver.MocklyServer) {
    server.addMockSuspending(getOrders)
    server.activateScenarioSuspending("slow-checkout")
    server.setFaultSuspending(faultConfig { delay = "100ms" })

    // Escape hatch for any blocking call without a named wrapper:
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

## Requirements

- Kotlin 1.9+ targeting JVM 11+
- Same platform/binary requirements as the [Java client](java.md)

See [`clients/kotlin/README.md`](https://github.com/dever-labs/mockly/blob/main/clients/kotlin/README.md) for
the full module reference.
