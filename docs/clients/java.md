# Mockly — Java Client

The Java client starts, controls, and stops a Mockly process from your JUnit tests.

## Install

### Maven

```xml
<dependency>
  <groupId>io.github.dever-labs</groupId>
  <artifactId>mockly-driver</artifactId>
  <version>0.13.0</version> <!-- x-release-please-version -->
  <scope>test</scope>
</dependency>
```

### Gradle

```groovy
testImplementation 'io.github.dever-labs:mockly-driver:0.13.0' // x-release-please-version
```

## Quickstart

```java
import io.mockly.driver.MocklyServer;
import io.mockly.driver.MocklyConfig;
import io.mockly.driver.model.*;

try (MocklyServer server = MocklyServer.ensure(MocklyConfig.builder().build())) {
    server.addMock(Mock.builder("get-user",
        MockRequest.builder("GET", "/users/1").build(),
        MockResponse.builder(200)
            .body("{\"id\":1,\"name\":\"Alice\"}")
            .header("Content-Type", "application/json")
            .build()
    ).build());

    // Point your service under test at server.httpBase
    // e.g. "http://127.0.0.1:45678"
}
```

## Factory methods

| Method | Description |
|---|---|
| `MocklyServer.ensure(config)` | Downloads the binary if not present, then starts the server. **Recommended for most cases.** |
| `MocklyServer.create(config)` | Starts using an already-installed binary. Throws if the binary is not found. |

Both implement `AutoCloseable` — safe to use with try-with-resources.

Both retry up to 3 times on ephemeral port conflicts.

## Configuration

```java
MocklyConfig config = MocklyConfig.builder()
    .scenarios(List.of(
        Scenario.builder("payment-fail", "Payment Failure")
            .patch(ScenarioPatch.builder("charge")
                .status(503)
                .body("{\"error\":\"unavailable\"}")
                .build())
            .build()
    ))
    .startupTimeoutMs(15_000)
    .build();

MocklyServer server = MocklyServer.ensure(config);
```

## API reference

### Mocks

```java
import io.mockly.driver.model.*;
import java.util.List;
import java.util.Map;

// Add a mock
server.addMock(Mock.builder("get-orders",
    MockRequest.builder("GET", "/orders")
        .header("Authorization", "Bearer *")
        .build(),
    MockResponse.builder(200)
        .body("[{\"id\":1}]")
        .header("Content-Type", "application/json")
        .delay("100ms")
        .build()
).build());

// Inspect the currently registered mocks
List<Mock> mocks = server.listMocks();

// Replace a mock definition
Mock updated = server.updateMock("get-orders", Mock.builder("get-orders",
    MockRequest.builder("GET", "/orders").build(),
    MockResponse.builder(200)
        .body("[{\"id\":1},{\"id\":2}]")
        .header("Content-Type", "application/json")
        .build()
).build());

// Patch only the response fields you want to change
Mock patched = server.patchMock("get-orders", MockResponsePatch.builder()
    .status(201)
    .body("[]")
    .header("X-Mock-Version", "v2")
    .delay("250ms")
    .build());

// Remove a mock
server.deleteMock("get-orders");
```

### Scenarios

```java
import io.mockly.driver.model.*;
import java.time.Duration;
import java.util.List;
import java.util.Map;

Scenario createdScenario = server.createScenario(Scenario.builder("slow-checkout", "Slow checkout")
    .description("Used for retry-path tests")
    .patch(ScenarioPatch.builder("charge")
        .status(503)
        .delay("750ms")
        .build())
    .build());

List<Scenario> scenarios = server.listScenarios();
Scenario loadedScenario = server.getScenario("slow-checkout");

Scenario updatedScenario = server.updateScenario("slow-checkout", Scenario.builder(
        loadedScenario.getId(),
        "Slow checkout v2")
    .description(loadedScenario.getDescription())
    .patches(loadedScenario.getPatches())
    .build());

// Activate a scenario before exercising your service
server.activateScenario("slow-checkout");
ActiveScenariosResponse activeScenarios = server.listActiveScenarios();
System.out.println(activeScenarios.getActive());

// Deactivate or delete it when you're done
server.deactivateScenario("slow-checkout");
server.deleteScenario("slow-checkout");
```

### Call verification

```java
CallSummary summary = server.waitForCalls("get-orders", 2, Duration.ofSeconds(5));
if (summary.getCount() != 2) {
    throw new IllegalStateException("Expected 2 calls, got " + summary.getCount());
}

CallSummary latestCalls = server.getCalls("get-orders");
System.out.println(latestCalls.getCalls().get(0).getPath());

server.clearCalls("get-orders");
server.clearAllCalls();
```

### State

```java
Map<String, String> state = server.getState();
System.out.println(state.get("order-status"));

Map<String, String> updatedState = server.setState(Map.of(
    "order-status", "pending",
    "retry-count", "1"
));
System.out.println(updatedState.get("retry-count"));

server.deleteState("retry-count");
```

### Logs

```java
List<CallEntry> allLogs = server.getLogs();
List<CallEntry> matchedLogs = server.getLogs("get-orders");

int totalLogs = server.getLogsCount();
int matchedCount = server.getLogsCount("get-orders");
System.out.println(totalLogs + " total / " + matchedCount + " matched");
System.out.println(allLogs.isEmpty() ? null : allLogs.get(0).getPath());
System.out.println(matchedLogs.isEmpty() ? null : matchedLogs.get(0).getMatchedId());

server.clearLogs();
```

### Fault injection

```java
// Add latency and override status codes on all requests
server.setFault(FaultConfig.builder(true)
    .delay("500ms")
    .statusOverride(503)
    .errorRate(0.5)
    .build());

// Remove the fault
server.clearFault();
```

### Reset and stop

```java
// Reset all dynamic mocks, active scenarios, and faults; keeps startup config
server.reset();

// Stop (or use try-with-resources / @AfterAll)
server.stop();
```

## Integration with JUnit 5

### Shared server for a test class

```java
import io.mockly.driver.MocklyServer;
import io.mockly.driver.MocklyConfig;
import io.mockly.driver.model.*;
import org.junit.jupiter.api.*;

@TestInstance(TestInstance.Lifecycle.PER_CLASS)
class PaymentServiceTest {

    MocklyServer mockly;

    @BeforeAll
    void startMockly() throws Exception {
        mockly = MocklyServer.ensure(MocklyConfig.builder().build());
    }

    @AfterAll
    void stopMockly() throws Exception {
        if (mockly != null) mockly.stop();
    }

    @BeforeEach
    void reset() throws Exception {
        mockly.reset(); // isolate each test
    }

    @Test
    void returnsUserFromMock() throws Exception {
        mockly.addMock(Mock.builder("get-user",
            MockRequest.builder("GET", "/users/1").build(),
            MockResponse.builder(200).body("{\"id\":1}").build()
        ).build());

        // ... call your service, assert response ...
    }

    @Test
    void handles503ViaScenario() throws Exception {
        mockly.addMock(Mock.builder("charge",
            MockRequest.builder("POST", "/charge").build(),
            MockResponse.builder(200).body("{\"ok\":true}").build()
        ).build());

        mockly.activateScenario("payment-fail");

        // ... assert your service handles 503 gracefully ...
    }
}
```

### Try-with-resources (per-test isolation)

```java
@Test
void isolated() throws Exception {
    try (MocklyServer server = MocklyServer.ensure(MocklyConfig.builder().build())) {
        server.addMock(/* ... */);
        // ... test ...
    }
}
```

## Server properties

| Field | Description |
|---|---|
| `server.httpBase` | Base URL of the mock HTTP server, e.g. `http://127.0.0.1:45123` |
| `server.apiBase` | Base URL of the management API, e.g. `http://127.0.0.1:45124` |
| `server.httpPort` | Numeric HTTP port |
| `server.apiPort` | Numeric API port |

## Testcontainers

Mockly also ships a Docker-backed Testcontainers module for Java: Maven artifact `io.github.dever-labs:mockly-testcontainers` and package `io.mockly.testcontainers`.

Use it instead of the driver when you want Docker-managed lifecycle, no local binary download, and the same container image in local tests and CI.

### Install

```xml
<dependency>
  <groupId>io.github.dever-labs</groupId>
  <artifactId>mockly-testcontainers</artifactId>
  <version>0.13.0</version> <!-- x-release-please-version -->
  <scope>test</scope>
</dependency>
```

### Example

```java
import io.mockly.testcontainers.MocklyContainer;
import io.mockly.testcontainers.model.Mock;
import io.mockly.testcontainers.model.MockRequest;
import io.mockly.testcontainers.model.MockResponse;
import org.junit.jupiter.api.Test;
import org.testcontainers.junit.jupiter.Container;
import org.testcontainers.junit.jupiter.Testcontainers;

import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;

import static org.junit.jupiter.api.Assertions.assertEquals;

@Testcontainers
class PaymentServiceTest {
    @Container
    static MocklyContainer mockly = new MocklyContainer();

    @Test
    void returnsUserFromContainer() throws Exception {
        mockly.addMock(Mock.builder(
                "get-user",
                MockRequest.builder("GET", "/users/1").build(),
                MockResponse.builder(200).body("{\"id\":1}").build()
        ).build());

        HttpResponse<String> response = HttpClient.newHttpClient().send(
                HttpRequest.newBuilder(URI.create(mockly.getHttpBase() + "/users/1")).GET().build(),
                HttpResponse.BodyHandlers.ofString());

        assertEquals(200, response.statusCode());
        assertEquals("{\"id\":1}", response.body());
    }
}
```

### Key API

- `new MocklyContainer()` / `withInlineConfig(yaml)`
- `getHttpBase()` / `getApiBase()`
- `addMock`, `deleteMock`, `reset`
- `activateScenario`, `deactivateScenario`
- `setFault`, `clearFault`

### Requirements

- Java 11+
- Docker

See `clients/java/testcontainers/README.md` for the full module reference.
