package io.mockly.testcontainers;

import io.mockly.testcontainers.model.FaultConfig;
import io.mockly.testcontainers.model.Mock;
import io.mockly.testcontainers.model.MockRequest;
import io.mockly.testcontainers.model.MockResponse;
import org.testcontainers.containers.GenericContainer;
import org.testcontainers.containers.wait.strategy.Wait;
import org.testcontainers.images.builder.Transferable;
import org.testcontainers.utility.DockerImageName;

import java.io.IOException;
import java.io.UncheckedIOException;
import java.net.URI;
import java.net.URLEncoder;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.nio.charset.StandardCharsets;
import java.time.Duration;
import java.util.Map;

public class MocklyContainer extends GenericContainer<MocklyContainer> {

    public static final String DEFAULT_IMAGE = "ghcr.io/dever-labs/mockly:latest";
    public static final int HTTP_PORT = 8090;
    public static final int API_PORT = 9091;

    private static final String DEFAULT_CONFIG_PATH = "/config/mockly.yaml";
    private static final String DEFAULT_CONFIG = "mockly:\n"
            + "  api:\n"
            + "    port: " + API_PORT + "\n"
            + "protocols:\n"
            + "  http:\n"
            + "    enabled: true\n"
            + "    port: " + HTTP_PORT + "\n";

    private final HttpClient httpClient = HttpClient.newBuilder()
            .connectTimeout(Duration.ofSeconds(5))
            .build();

    private String configuredYaml = DEFAULT_CONFIG;

    public MocklyContainer() {
        this(DEFAULT_IMAGE);
    }

    public MocklyContainer(String imageName) {
        this(DockerImageName.parse(imageName));
    }

    public MocklyContainer(DockerImageName imageName) {
        super(imageName);
        withExposedPorts(HTTP_PORT, API_PORT);
        waitingFor(Wait.forHttp("/api/protocols")
                .forPort(API_PORT)
                .forStatusCode(200)
                .withStartupTimeout(Duration.ofSeconds(60)));
        applyConfig(DEFAULT_CONFIG);
    }

    public MocklyContainer withInlineConfig(String yaml) {
        applyConfig(yaml);
        return this;
    }

    public String getHttpBase() {
        return "http://" + getHost() + ":" + getMappedPort(HTTP_PORT);
    }

    public String getApiBase() {
        return "http://" + getHost() + ":" + getMappedPort(API_PORT);
    }

    public void addMock(Mock mock) throws IOException, InterruptedException {
        HttpResponse<String> response = post("/api/mocks/http", toJson(mock));
        requireStatus(response, 201, "addMock");
    }

    public void deleteMock(String id) throws IOException, InterruptedException {
        HttpResponse<String> response = delete("/api/mocks/http/" + encodeSegment(id));
        requireStatus(response, 204, "deleteMock");
    }

    public void reset() throws IOException, InterruptedException {
        HttpResponse<String> response = post("/api/reset", "");
        requireStatus(response, 200, "reset");
    }

    public void activateScenario(String id) throws IOException, InterruptedException {
        HttpResponse<String> response = post("/api/scenarios/" + encodeSegment(id) + "/activate", "");
        requireStatus(response, 200, "activateScenario");
    }

    public void deactivateScenario(String id) throws IOException, InterruptedException {
        HttpResponse<String> response = post("/api/scenarios/" + encodeSegment(id) + "/deactivate", "");
        requireStatus(response, 200, "deactivateScenario");
    }

    public void setFault(FaultConfig config) throws IOException, InterruptedException {
        HttpResponse<String> response = post("/api/fault/http", toJson(config));
        requireStatus(response, 200, "setFault");
    }

    public void clearFault() throws IOException, InterruptedException {
        HttpResponse<String> response = delete("/api/fault");
        requireStatus(response, new int[]{200, 204}, "clearFault");
    }

    @Override
    public String getLogs() {
        try {
            HttpResponse<String> response = get("/api/logs");
            requireStatus(response, 200, "getLogs");
            return response.body();
        } catch (InterruptedException e) {
            Thread.currentThread().interrupt();
            throw new IllegalStateException("getLogs interrupted", e);
        } catch (IOException e) {
            throw new UncheckedIOException("getLogs failed", e);
        }
    }

    public String getLogs(String matchedId) throws IOException, InterruptedException {
        HttpResponse<String> response = get("/api/logs?matched_id=" + urlEncode(matchedId));
        requireStatus(response, 200, "getLogs");
        return response.body();
    }

    public void clearLogs() throws IOException, InterruptedException {
        HttpResponse<String> response = delete("/api/logs");
        requireStatus(response, 200, "clearLogs");
    }

    String getConfiguredYaml() {
        return configuredYaml;
    }

    private void applyConfig(String yaml) {
        this.configuredYaml = yaml;
        withCopyToContainer(Transferable.of(yaml.getBytes(StandardCharsets.UTF_8), 0644), DEFAULT_CONFIG_PATH);
        withCommand("start", "-c", DEFAULT_CONFIG_PATH);
    }

    private HttpResponse<String> get(String path) throws IOException, InterruptedException {
        HttpRequest request = HttpRequest.newBuilder()
                .uri(URI.create(getApiBase() + path))
                .GET()
                .timeout(Duration.ofSeconds(10))
                .build();
        return httpClient.send(request, HttpResponse.BodyHandlers.ofString());
    }

    private HttpResponse<String> post(String path, String jsonBody) throws IOException, InterruptedException {
        HttpRequest.BodyPublisher publisher = jsonBody.isEmpty()
                ? HttpRequest.BodyPublishers.noBody()
                : HttpRequest.BodyPublishers.ofString(jsonBody);

        HttpRequest.Builder builder = HttpRequest.newBuilder()
                .uri(URI.create(getApiBase() + path))
                .POST(publisher)
                .timeout(Duration.ofSeconds(10));

        if (!jsonBody.isEmpty()) {
            builder.header("Content-Type", "application/json");
        }

        return httpClient.send(builder.build(), HttpResponse.BodyHandlers.ofString());
    }

    private HttpResponse<String> delete(String path) throws IOException, InterruptedException {
        HttpRequest request = HttpRequest.newBuilder()
                .uri(URI.create(getApiBase() + path))
                .DELETE()
                .timeout(Duration.ofSeconds(10))
                .build();
        return httpClient.send(request, HttpResponse.BodyHandlers.ofString());
    }

    private static void requireStatus(HttpResponse<String> response, int status, String action) throws IOException {
        requireStatus(response, new int[]{status}, action);
    }

    private static void requireStatus(HttpResponse<String> response, int[] statuses, String action) throws IOException {
        for (int status : statuses) {
            if (response.statusCode() == status) {
                return;
            }
        }
        throw new IOException(action + " failed: HTTP " + response.statusCode() + " — " + response.body());
    }

    private static String encodeSegment(String value) {
        return urlEncode(value).replace("+", "%20");
    }

    private static String urlEncode(String value) {
        return URLEncoder.encode(value, StandardCharsets.UTF_8);
    }

    static String toJson(Mock mock) {
        StringBuilder sb = new StringBuilder("{");
        sb.append("\"id\":").append(jsonString(mock.getId())).append(",");
        sb.append("\"request\":").append(toJson(mock.getRequest())).append(",");
        sb.append("\"response\":").append(toJson(mock.getResponse()));
        sb.append("}");
        return sb.toString();
    }

    private static String toJson(MockRequest request) {
        StringBuilder sb = new StringBuilder("{");
        sb.append("\"method\":").append(jsonString(request.getMethod())).append(",");
        sb.append("\"path\":").append(jsonString(request.getPath()));
        if (!request.getHeaders().isEmpty()) {
            sb.append(",\"headers\":").append(jsonMap(request.getHeaders()));
        }
        sb.append("}");
        return sb.toString();
    }

    private static String toJson(MockResponse response) {
        StringBuilder sb = new StringBuilder("{");
        sb.append("\"status\":").append(response.getStatus());
        if (response.getBody() != null) {
            sb.append(",\"body\":").append(jsonString(response.getBody()));
        }
        if (!response.getHeaders().isEmpty()) {
            sb.append(",\"headers\":").append(jsonMap(response.getHeaders()));
        }
        if (response.getDelay() != null) {
            sb.append(",\"delay\":").append(jsonString(response.getDelay()));
        }
        sb.append("}");
        return sb.toString();
    }

    static String toJson(FaultConfig fault) {
        StringBuilder sb = new StringBuilder("{");
        sb.append("\"enabled\":").append(fault.isEnabled());
        if (fault.getDelay() != null) {
            sb.append(",\"delay\":").append(jsonString(fault.getDelay()));
        }
        if (fault.getStatusOverride() != null) {
            sb.append(",\"status_override\":").append(fault.getStatusOverride());
        }
        if (fault.getErrorRate() != null) {
            sb.append(",\"error_rate\":").append(fault.getErrorRate());
        }
        sb.append("}");
        return sb.toString();
    }

    static String jsonString(String value) {
        if (value == null) {
            return "null";
        }
        return "\"" + value
                .replace("\\", "\\\\")
                .replace("\"", "\\\"")
                .replace("\n", "\\n")
                .replace("\r", "\\r")
                .replace("\t", "\\t")
                + "\"";
    }

    static String jsonMap(Map<String, String> map) {
        StringBuilder sb = new StringBuilder("{");
        boolean first = true;
        for (Map.Entry<String, String> entry : map.entrySet()) {
            if (!first) {
                sb.append(",");
            }
            sb.append(jsonString(entry.getKey())).append(":").append(jsonString(entry.getValue()));
            first = false;
        }
        sb.append("}");
        return sb.toString();
    }
}
