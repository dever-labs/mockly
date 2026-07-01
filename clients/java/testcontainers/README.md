# mockly-testcontainers

Run Mockly in Docker-backed Java tests with Testcontainers.

- **Maven artifact:** `io.github.dever-labs:mockly-testcontainers`
- **Java package:** `io.mockly.testcontainers`
- **Image:** `ghcr.io/dever-labs/mockly:latest`

## Requirements

- Java 11+
- Docker

## Install

### Maven

```xml
<dependency>
  <groupId>io.github.dever-labs</groupId>
  <artifactId>mockly-testcontainers</artifactId>
  <version>0.12.4</version> <!-- x-release-please-version -->
  <scope>test</scope>
</dependency>
```

### Gradle

```groovy
testImplementation 'io.github.dever-labs:mockly-testcontainers:0.12.4' // x-release-please-version
```

## Quickstart

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

    private final HttpClient http = HttpClient.newHttpClient();

    @Test
    void returnsMockedUser() throws Exception {
        mockly.addMock(Mock.builder(
                "get-user",
                MockRequest.builder("GET", "/users/1").build(),
                MockResponse.builder(200).body("{\"id\":1}").build()
        ).build());

        HttpResponse<String> response = http.send(
                HttpRequest.newBuilder(URI.create(mockly.getHttpBase() + "/users/1")).GET().build(),
                HttpResponse.BodyHandlers.ofString());

        assertEquals(200, response.statusCode());
        assertEquals("{\"id\":1}", response.body());
    }
}
```

## When to use the testcontainers module

Use `mockly-testcontainers` when you want Docker-managed lifecycle, no native binary download, or the same Mockly environment in local tests and CI.

Use the driver client when you prefer starting the Mockly binary directly in the JVM.

## Construction and configuration

`MocklyContainer` extends `GenericContainer<MocklyContainer>`.

| API | Description |
|---|---|
| `new MocklyContainer()` | Start from the default Mockly image and default config. |
| `new MocklyContainer(String imageName)` | Override the Docker image name. |
| `withInlineConfig(String yaml)` | Replace `/config/mockly.yaml` with inline YAML before startup. |
| `getHttpBase()` | Return the mock HTTP base URL. |
| `getApiBase()` | Return the management API base URL. |

### Custom YAML config

```java
MocklyContainer mockly = new MocklyContainer()
        .withInlineConfig(
                "mockly:\n"
                        + "  api:\n"
                        + "    port: 9091\n"
                        + "protocols:\n"
                        + "  http:\n"
                        + "    enabled: true\n"
                        + "    port: 8090\n");
```

## Management API helpers

| Method | Description |
|---|---|
| `addMock(Mock mock)` | Register a dynamic HTTP mock. |
| `deleteMock(String id)` | Delete a mock by ID. |
| `reset()` | Remove dynamic mocks, deactivate scenarios, and clear faults. |
| `activateScenario(String id)` | Activate a configured scenario. |
| `deactivateScenario(String id)` | Deactivate a configured scenario. |
| `setFault(FaultConfig config)` | Apply a global HTTP fault. |
| `clearFault()` | Remove the active fault. |
| `getLogs()` / `getLogs(String matchedId)` | Read request logs as JSON. |
| `clearLogs()` | Clear stored request logs. |

## Model types

Use the bundled model classes from `io.mockly.testcontainers.model`:

- `Mock`
- `MockRequest`
- `MockResponse`
- `FaultConfig`

These use builders so test setup stays concise.
