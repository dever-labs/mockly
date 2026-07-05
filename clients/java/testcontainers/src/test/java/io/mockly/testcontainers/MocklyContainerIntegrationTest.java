package io.mockly.testcontainers;

import io.mockly.driver.model.CallEntry;
import io.mockly.driver.model.CallSummary;
import io.mockly.driver.model.Mock;
import io.mockly.driver.model.MockRequest;
import io.mockly.driver.model.MockResponse;
import io.mockly.driver.model.MockResponsePatch;
import io.mockly.driver.model.Scenario;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Tag;
import org.junit.jupiter.api.Test;
import org.testcontainers.junit.jupiter.Container;
import org.testcontainers.junit.jupiter.Testcontainers;

import java.io.IOException;
import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.time.Duration;
import java.util.List;
import java.util.Map;

import static org.junit.jupiter.api.Assertions.assertDoesNotThrow;
import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertNotEquals;
import static org.junit.jupiter.api.Assertions.assertTrue;

/**
 * Integration tests that spin up a real Mockly Docker container.
 *
 * <p>These tests require a running Docker daemon. Run with:
 * <pre>mvn test -Dgroups=integration</pre>
 *
 * <p>Excluded from the default build (no {@code integration} tag) via:
 * <pre>mvn test -Dgroups='!integration'</pre>
 */
@Tag("integration")
@Testcontainers
class MocklyContainerIntegrationTest {

    @Container
    private static final MocklyContainer container = new MocklyContainer();

    private final HttpClient http = HttpClient.newHttpClient();

    @BeforeEach
    void resetContainer() throws IOException, InterruptedException {
        container.reset();
    }

    @Test
    void containerStartsAndApiIsReachable() throws IOException, InterruptedException {
        String apiBase = container.getApiBase();
        assertTrue(apiBase.startsWith("http://"), "apiBase should start with http://");

        HttpResponse<String> resp = http.send(
                HttpRequest.newBuilder().uri(URI.create(apiBase + "/api/protocols")).GET().build(),
                HttpResponse.BodyHandlers.ofString());
        assertEquals(200, resp.statusCode(), "GET /api/protocols should return 200");
    }

    @Test
    void addMockAndHitEndpointThenReset() throws IOException, InterruptedException {
        Mock mock = Mock.builder(
                "hello-mock",
                MockRequest.builder("GET", "/hello").build(),
                MockResponse.builder(200).body("world").build()
        ).build();

        container.addMock(mock);

        HttpResponse<String> resp = http.send(
                HttpRequest.newBuilder().uri(URI.create(container.getHttpBase() + "/hello")).GET().build(),
                HttpResponse.BodyHandlers.ofString());
        assertEquals(200, resp.statusCode(), "mock should return 200");
        assertEquals("world", resp.body(), "mock should return 'world'");

        container.reset();

        HttpResponse<String> afterReset = http.send(
                HttpRequest.newBuilder().uri(URI.create(container.getHttpBase() + "/hello")).GET().build(),
                HttpResponse.BodyHandlers.ofString());
        assertNotEquals(200, afterReset.statusCode(), "mock should be cleared after reset");
    }

    @Test
    void getLogsReturnsEntriesAfterRequest() throws IOException, InterruptedException {
        http.send(
                HttpRequest.newBuilder().uri(URI.create(container.getHttpBase() + "/log-probe")).GET().build(),
                HttpResponse.BodyHandlers.ofString());

        List<CallEntry> logs = container.getApiLogs();
        assertFalse(logs.isEmpty(), "getApiLogs() should return parsed log entries");
        assertTrue(logs.stream().anyMatch(entry -> "/log-probe".equals(entry.getPath())),
                "expected /log-probe entry in logs");
    }

    @Test
    void withInlineConfigStartsContainer() {
        assertDoesNotThrow(() -> {
            String apiBase = container.getApiBase();
            HttpResponse<String> resp = http.send(
                    HttpRequest.newBuilder().uri(URI.create(apiBase + "/api/protocols")).GET().build(),
                    HttpResponse.BodyHandlers.ofString());
            assertEquals(200, resp.statusCode());
        });
    }

    @Test
    void listMocksReturnsAddedMocks() throws IOException, InterruptedException {
        Mock mock = Mock.builder(
                "list-test",
                MockRequest.builder("GET", "/list-test").build(),
                MockResponse.builder(200).body("listed").build()
        ).build();

        container.addMock(mock);

        List<Mock> mocks = container.listMocks();
        assertFalse(mocks.isEmpty(), "listMocks() should return added mocks");
        assertEquals("list-test", mocks.get(0).getId());
    }

    @Test
    void updateAndPatchMock() throws IOException, InterruptedException {
        container.addMock(Mock.builder(
                "upd-test",
                MockRequest.builder("GET", "/upd-test").build(),
                MockResponse.builder(200).body("orig").build()
        ).build());

        container.updateMock("upd-test", Mock.builder(
                "upd-test",
                MockRequest.builder("GET", "/upd-test").build(),
                MockResponse.builder(200).body("updated").build()
        ).build());

        HttpResponse<String> updated = http.send(
                HttpRequest.newBuilder().uri(URI.create(container.getHttpBase() + "/upd-test")).GET().build(),
                HttpResponse.BodyHandlers.ofString());
        assertEquals(200, updated.statusCode());
        assertEquals("updated", updated.body());

        container.patchMock("upd-test", MockResponsePatch.builder().status(418).build());

        HttpResponse<String> patched = http.send(
                HttpRequest.newBuilder().uri(URI.create(container.getHttpBase() + "/upd-test")).GET().build(),
                HttpResponse.BodyHandlers.ofString());
        assertEquals(418, patched.statusCode());
    }

    @Test
    void stateCrudWorks() throws IOException, InterruptedException {
        container.setState(Map.of("k", "v"));

        Map<String, String> state = container.getState();
        assertTrue(state.containsKey("k"));
        assertEquals("v", state.get("k"));

        container.deleteState("k");

        Map<String, String> stateAfterDelete = container.getState();
        assertFalse(stateAfterDelete.containsKey("k"));
    }

    @Test
    void getLogsCountPositiveAfterRequest() throws IOException, InterruptedException {
        http.send(
                HttpRequest.newBuilder().uri(URI.create(container.getHttpBase() + "/count-probe")).GET().build(),
                HttpResponse.BodyHandlers.ofString());

        assertTrue(container.getLogsCount() > 0, "getLogsCount() should be positive after a request");
        assertDoesNotThrow(() -> container.getLogsCount("nonexistent"));
    }

    @Test
    void scenarioCrudWorks() throws IOException, InterruptedException {
        assertDoesNotThrow(() -> container.listScenarios());

        container.createScenario(Scenario.builder("tc-java-s", "TC Java").build());

        Scenario scenario = container.getScenario("tc-java-s");
        assertEquals("tc-java-s", scenario.getId());
        assertFalse(container.listScenarios().isEmpty(), "listScenarios() should contain created scenario");

        container.deleteScenario("tc-java-s");
    }

    @Test
    void getCallsAndClearWorks() throws IOException, InterruptedException {
        container.addMock(Mock.builder(
                "java-calls",
                MockRequest.builder("GET", "/java-calls").build(),
                MockResponse.builder(200).body("ok").build()
        ).build());

        http.send(
                HttpRequest.newBuilder().uri(URI.create(container.getHttpBase() + "/java-calls")).GET().build(),
                HttpResponse.BodyHandlers.ofString());

        CallSummary calls = container.getCalls("java-calls");
        assertTrue(calls.getCount() > 0, "getCalls() should report recorded calls");

        container.clearCalls("java-calls");

        CallSummary clearedCalls = container.getCalls("java-calls");
        assertEquals(0L, clearedCalls.getCount());

        assertDoesNotThrow(() -> container.clearAllCalls());
    }

    @Test
    void waitForCallsResolvesWhenHit() throws IOException, InterruptedException {
        container.addMock(Mock.builder(
                "java-wait",
                MockRequest.builder("GET", "/java-wait").build(),
                MockResponse.builder(200).body("ok").build()
        ).build());

        http.send(
                HttpRequest.newBuilder().uri(URI.create(container.getHttpBase() + "/java-wait")).GET().build(),
                HttpResponse.BodyHandlers.ofString());

        CallSummary summary = container.waitForCalls("java-wait", 1, Duration.ofSeconds(10));
        assertTrue(summary.getCount() >= 1, "waitForCalls() should observe at least one call");
    }
}
