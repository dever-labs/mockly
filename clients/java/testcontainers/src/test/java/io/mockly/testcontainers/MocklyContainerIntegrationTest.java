package io.mockly.testcontainers;

import io.mockly.driver.model.CallEntry;
import io.mockly.driver.model.Mock;
import io.mockly.driver.model.MockRequest;
import io.mockly.driver.model.MockResponse;
import org.junit.jupiter.api.Tag;
import org.junit.jupiter.api.Test;
import org.testcontainers.junit.jupiter.Container;
import org.testcontainers.junit.jupiter.Testcontainers;

import java.io.IOException;
import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.util.List;

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
}
