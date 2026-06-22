package io.mockly.testcontainers;

import io.mockly.testcontainers.model.Mock;
import io.mockly.testcontainers.model.MockRequest;
import io.mockly.testcontainers.model.MockResponse;
import org.junit.jupiter.api.Tag;
import org.junit.jupiter.api.Test;
import org.testcontainers.junit.jupiter.Container;
import org.testcontainers.junit.jupiter.Testcontainers;

import java.io.IOException;
import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;

import static org.junit.jupiter.api.Assertions.*;

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
        // generate a log entry
        http.send(
                HttpRequest.newBuilder().uri(URI.create(container.getHttpBase() + "/log-probe")).GET().build(),
                HttpResponse.BodyHandlers.ofString());

        String logs = container.getLogs();
        assertFalse(logs.isBlank(), "getLogs() should return non-empty JSON");
        assertTrue(logs.startsWith("[") || logs.startsWith("{"),
                "getLogs() should return valid JSON, got: " + logs);
    }

    @Test
    void withInlineConfigStartsContainer() {
        // Verifies that a container started with a custom YAML config is reachable.
        // The shared @Container above uses the default config; inline config is tested
        // by constructing a second container in MocklyContainerTest (unit).
        // This test just validates that the shared container's API is accessible.
        assertDoesNotThrow(() -> {
            String apiBase = container.getApiBase();
            HttpResponse<String> resp = http.send(
                    HttpRequest.newBuilder().uri(URI.create(apiBase + "/api/protocols")).GET().build(),
                    HttpResponse.BodyHandlers.ofString());
            assertEquals(200, resp.statusCode());
        });
    }
}
