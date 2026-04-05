# Mockly — Java Client

The Java client starts, controls, and stops a Mockly process from your JUnit tests.

## Install

### Maven

```xml
<dependency>
  <groupId>io.github.dever-labs</groupId>
  <artifactId>mockly-driver</artifactId>
  <version>0.4.7</version>
  <scope>test</scope>
</dependency>
```

### Gradle

```groovy
testImplementation 'io.github.dever-labs:mockly-driver:0.4.7'
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

// Remove a mock
server.deleteMock("get-orders");
```

### Scenarios

```java
// Activate a pre-configured scenario
server.activateScenario("payment-fail");

// Deactivate it
server.deactivateScenario("payment-fail");
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
