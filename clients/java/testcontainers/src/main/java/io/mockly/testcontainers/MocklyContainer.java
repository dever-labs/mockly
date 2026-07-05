package io.mockly.testcontainers;

import com.fasterxml.jackson.annotation.JsonInclude;
import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.core.type.TypeReference;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.PropertyNamingStrategies;
import io.mockly.driver.model.ActiveScenariosResponse;
import io.mockly.driver.model.CallEntry;
import io.mockly.driver.model.CallSummary;
import io.mockly.driver.model.FaultConfig;
import io.mockly.driver.model.Mock;
import io.mockly.driver.model.MockRequest;
import io.mockly.driver.model.MockResponse;
import io.mockly.driver.model.MockResponsePatch;
import io.mockly.driver.model.Scenario;
import io.mockly.driver.model.ScenarioPatch;
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
import java.util.Collections;
import java.util.List;
import java.util.Map;
import java.util.stream.Collectors;

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

    private static final ObjectMapper OBJECT_MAPPER = new ObjectMapper()
            .setPropertyNamingStrategy(PropertyNamingStrategies.SNAKE_CASE)
            .setSerializationInclusion(JsonInclude.Include.NON_EMPTY);

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
        HttpResponse<String> response = post("/api/mocks/http", mock);
        requireStatus(response, 201, "addMock");
    }

    public List<Mock> listMocks() throws IOException, InterruptedException {
        return toMocks(getJsonList("/api/mocks/http", new TypeReference<List<MockDto>>() { }));
    }

    public Mock updateMock(String id, Mock mock) throws IOException, InterruptedException {
        return toMock(requestAndRead("PUT", "/api/mocks/http/" + encodeSegment(id), mock, 200, MockDto.class, "updateMock"));
    }

    public Mock patchMock(String id, MockResponsePatch patch) throws IOException, InterruptedException {
        return toMock(requestAndRead("PATCH", "/api/mocks/http/" + encodeSegment(id), patch, 200, MockDto.class, "patchMock"));
    }

    public void deleteMock(String id) throws IOException, InterruptedException {
        HttpResponse<String> response = delete("/api/mocks/http/" + encodeSegment(id));
        requireStatus(response, 200, "deleteMock");
    }

    public Map<String, String> getState() throws IOException, InterruptedException {
        return getJson("/api/state", new TypeReference<Map<String, String>>() { }, "getState");
    }

    public Map<String, String> setState(Map<String, String> kvMap) throws IOException, InterruptedException {
        return postAndRead("/api/state", kvMap, new TypeReference<Map<String, String>>() { }, 200, "setState");
    }

    public void deleteState(String key) throws IOException, InterruptedException {
        HttpResponse<String> response = delete("/api/state/" + encodeSegment(key));
        requireStatus(response, 200, "deleteState");
    }

    @Override
    public String getLogs() {
        try {
            HttpResponse<String> response = get(withMatchedId("/api/logs", null));
            requireStatus(response, 200, "getLogs");
            return response.body();
        } catch (InterruptedException e) {
            Thread.currentThread().interrupt();
            throw new IllegalStateException("getLogs interrupted", e);
        } catch (IOException e) {
            throw new UncheckedIOException("getLogs failed", e);
        }
    }

    public List<CallEntry> getApiLogs() throws IOException, InterruptedException {
        return getLogs((String) null);
    }

    public List<CallEntry> getLogs(String matchedId) throws IOException, InterruptedException {
        return toCallEntries(getJsonList(withMatchedId("/api/logs", matchedId), new TypeReference<List<CallEntryDto>>() { }));
    }

    public void clearLogs() throws IOException, InterruptedException {
        HttpResponse<String> response = delete("/api/logs");
        requireStatus(response, 200, "clearLogs");
    }

    public int getLogsCount() throws IOException, InterruptedException {
        return getLogsCount(null);
    }

    public int getLogsCount(String matchedId) throws IOException, InterruptedException {
        return getJson(withMatchedId("/api/logs/count", matchedId), CountResponse.class).count;
    }

    public List<Scenario> listScenarios() throws IOException, InterruptedException {
        return toScenarios(getJsonList("/api/scenarios", new TypeReference<List<ScenarioDto>>() { }));
    }

    public Scenario createScenario(Scenario scenario) throws IOException, InterruptedException {
        return toScenario(postAndRead("/api/scenarios", scenario, ScenarioDto.class, 201, "createScenario"));
    }

    public Scenario getScenario(String id) throws IOException, InterruptedException {
        return toScenario(getJson("/api/scenarios/" + encodeSegment(id), ScenarioDto.class));
    }

    public Scenario updateScenario(String id, Scenario scenario) throws IOException, InterruptedException {
        return toScenario(requestAndRead("PUT", "/api/scenarios/" + encodeSegment(id), scenario, 200, ScenarioDto.class, "updateScenario"));
    }

    public void deleteScenario(String id) throws IOException, InterruptedException {
        HttpResponse<String> response = delete("/api/scenarios/" + encodeSegment(id));
        requireStatus(response, 200, "deleteScenario");
    }

    public ActiveScenariosResponse listActiveScenarios() throws IOException, InterruptedException {
        return toActiveScenariosResponse(getJson("/api/scenarios/active", ActiveScenariosResponseDto.class));
    }

    public void reset() throws IOException, InterruptedException {
        HttpResponse<String> response = post("/api/reset", null);
        requireStatus(response, 200, "reset");
    }

    public void activateScenario(String id) throws IOException, InterruptedException {
        HttpResponse<String> response = post("/api/scenarios/" + encodeSegment(id) + "/activate", null);
        requireStatus(response, 200, "activateScenario");
    }

    public void deactivateScenario(String id) throws IOException, InterruptedException {
        HttpResponse<String> response = delete("/api/scenarios/" + encodeSegment(id) + "/activate");
        requireStatus(response, 200, "deactivateScenario");
    }

    public void setFault(FaultConfig config) throws IOException, InterruptedException {
        HttpResponse<String> response = post("/api/fault/http", config);
        requireStatus(response, 200, "setFault");
    }

    public void clearFault() throws IOException, InterruptedException {
        HttpResponse<String> response = delete("/api/fault");
        requireStatus(response, new int[]{200, 204}, "clearFault");
    }

    public CallSummary getCalls(String mockId) throws IOException, InterruptedException {
        return toCallSummary(getJson("/api/calls/http/" + encodeSegment(mockId), CallSummaryDto.class));
    }

    public void clearCalls(String mockId) throws IOException, InterruptedException {
        HttpResponse<String> response = delete("/api/calls/http/" + encodeSegment(mockId));
        requireStatus(response, 200, "clearCalls");
    }

    public void clearAllCalls() throws IOException, InterruptedException {
        HttpResponse<String> response = delete("/api/calls/http");
        requireStatus(response, 200, "clearAllCalls");
    }

    public CallSummary waitForCalls(String mockId, int count, Duration timeout) throws IOException, InterruptedException {
        HttpResponse<String> response = post(
                "/api/calls/http/" + encodeSegment(mockId) + "/wait",
                new WaitForCallsRequest(count, timeout.toMillis() + "ms"));
        if (response.statusCode() == 408) {
            throw new IOException("waitForCalls: timeout waiting for " + count + " call(s) on '" + mockId + "'");
        }
        requireStatus(response, 200, "waitForCalls");
        return toCallSummary(readJson(response.body(), CallSummaryDto.class));
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
        return request("GET", path, null);
    }

    private HttpResponse<String> post(String path, Object body) throws IOException, InterruptedException {
        return request("POST", path, body);
    }

    private HttpResponse<String> put(String path, Object body) throws IOException, InterruptedException {
        return request("PUT", path, body);
    }

    private HttpResponse<String> patch(String path, Object body) throws IOException, InterruptedException {
        return request("PATCH", path, body);
    }

    private HttpResponse<String> delete(String path) throws IOException, InterruptedException {
        return request("DELETE", path, null);
    }

    private HttpResponse<String> request(String method, String path, Object body) throws IOException, InterruptedException {
        HttpRequest.BodyPublisher publisher = body == null
                ? HttpRequest.BodyPublishers.noBody()
                : HttpRequest.BodyPublishers.ofString(writeJson(body));

        HttpRequest.Builder builder = HttpRequest.newBuilder()
                .uri(URI.create(getApiBase() + path))
                .method(method, publisher)
                .timeout(Duration.ofSeconds(10));

        if (body != null) {
            builder.header("Content-Type", "application/json");
        }

        return httpClient.send(builder.build(), HttpResponse.BodyHandlers.ofString());
    }

    private <T> T getJson(String path, Class<T> type) throws IOException, InterruptedException {
        return getJson(path, type, "GET " + path);
    }

    private <T> T getJson(String path, Class<T> type, String action) throws IOException, InterruptedException {
        HttpResponse<String> response = get(path);
        requireStatus(response, 200, action);
        return readJson(response.body(), type);
    }

    private <T> T getJson(String path, TypeReference<T> type, String action) throws IOException, InterruptedException {
        HttpResponse<String> response = get(path);
        requireStatus(response, 200, action);
        return readJson(response.body(), type);
    }

    private <T> List<T> getJsonList(String path, TypeReference<List<T>> type) throws IOException, InterruptedException {
        HttpResponse<String> response = get(path);
        requireStatus(response, 200, "GET " + path);
        return readJson(response.body(), type);
    }

    private <T> T postAndRead(String path, Object body, Class<T> type, int expectedStatus, String action)
            throws IOException, InterruptedException {
        return requestAndRead("POST", path, body, expectedStatus, type, action);
    }

    private <T> T postAndRead(String path, Object body, TypeReference<T> type, int expectedStatus, String action)
            throws IOException, InterruptedException {
        HttpResponse<String> response = post(path, body);
        requireStatus(response, expectedStatus, action);
        return readJson(response.body(), type);
    }

    private <T> T requestAndRead(String method, String path, Object body, int expectedStatus, Class<T> type, String action)
            throws IOException, InterruptedException {
        HttpResponse<String> response = request(method, path, body);
        requireStatus(response, expectedStatus, action);
        return readJson(response.body(), type);
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
        return writeJson(mock);
    }

    static String toJson(FaultConfig fault) {
        return writeJson(fault);
    }

    private static String writeJson(Object value) {
        try {
            return OBJECT_MAPPER.writeValueAsString(value);
        } catch (JsonProcessingException e) {
            throw new IllegalArgumentException("Unable to serialize value as JSON", e);
        }
    }

    private static <T> T readJson(String json, Class<T> type) throws IOException {
        return OBJECT_MAPPER.readValue(json, type);
    }

    private static <T> T readJson(String json, TypeReference<T> type) throws IOException {
        return OBJECT_MAPPER.readValue(json, type);
    }

    private static List<Mock> toMocks(List<MockDto> dtos) {
        if (dtos == null || dtos.isEmpty()) {
            return Collections.emptyList();
        }
        return dtos.stream().map(MocklyContainer::toMock).collect(Collectors.toList());
    }

    private static Mock toMock(MockDto dto) {
        return Mock.builder(
                dto.id,
                toMockRequest(dto.request),
                toMockResponse(dto.response))
                .build();
    }

    private static MockRequest toMockRequest(MockRequestDto dto) {
        MockRequest.Builder builder = MockRequest.builder(dto.method, dto.path);
        builder.headers(safeMap(dto.headers));
        return builder.build();
    }

    private static MockResponse toMockResponse(MockResponseDto dto) {
        MockResponse.Builder builder = MockResponse.builder(dto.status);
        if (dto.body != null) {
            builder.body(dto.body);
        }
        builder.headers(safeMap(dto.headers));
        if (dto.delay != null) {
            builder.delay(dto.delay);
        }
        return builder.build();
    }

    private static List<CallEntry> toCallEntries(List<CallEntryDto> dtos) {
        if (dtos == null || dtos.isEmpty()) {
            return Collections.emptyList();
        }
        return dtos.stream().map(MocklyContainer::toCallEntry).collect(Collectors.toList());
    }

    private static CallEntry toCallEntry(CallEntryDto dto) {
        return new CallEntry(
                dto.id,
                dto.timestamp,
                dto.protocol,
                dto.method,
                dto.path,
                dto.status,
                dto.durationMs,
                safeMap(dto.headers),
                dto.body,
                dto.matchedId,
                safeMap(dto.pathParams));
    }

    private static List<Scenario> toScenarios(List<ScenarioDto> dtos) {
        if (dtos == null || dtos.isEmpty()) {
            return Collections.emptyList();
        }
        return dtos.stream().map(MocklyContainer::toScenario).collect(Collectors.toList());
    }

    private static Scenario toScenario(ScenarioDto dto) {
        Scenario.Builder builder = Scenario.builder(dto.id, dto.name);
        if (dto.description != null) {
            builder.description(dto.description);
        }
        for (ScenarioPatchDto patch : safeList(dto.patches)) {
            builder.patch(toScenarioPatch(patch));
        }
        return builder.build();
    }

    private static ScenarioPatch toScenarioPatch(ScenarioPatchDto dto) {
        ScenarioPatch.Builder builder = ScenarioPatch.builder(dto.mockId);
        if (dto.status != null) {
            builder.status(dto.status);
        }
        if (dto.body != null) {
            builder.body(dto.body);
        }
        builder.headers(safeMap(dto.headers));
        if (dto.delay != null) {
            builder.delay(dto.delay);
        }
        if (dto.disabled != null) {
            builder.disabled(dto.disabled);
        }
        return builder.build();
    }

    private static ActiveScenariosResponse toActiveScenariosResponse(ActiveScenariosResponseDto dto) {
        return new ActiveScenariosResponse(safeList(dto.active), toScenarios(dto.scenarios));
    }

    private static CallSummary toCallSummary(CallSummaryDto dto) {
        return new CallSummary(dto.mockId, dto.count, toCallEntries(dto.calls));
    }

    private static Map<String, String> safeMap(Map<String, String> map) {
        return map == null ? Collections.emptyMap() : map;
    }

    private static <T> List<T> safeList(List<T> list) {
        return list == null ? Collections.emptyList() : list;
    }

    private static String withMatchedId(String path, String matchedId) {
        if (matchedId == null || matchedId.isEmpty()) {
            return path;
        }
        return path + "?matched_id=" + urlEncode(matchedId);
    }

    private static final class CountResponse {
        public int count;
    }

    private static final class WaitForCallsRequest {
        public final int count;
        public final String timeout;

        private WaitForCallsRequest(int count, String timeout) {
            this.count = count;
            this.timeout = timeout;
        }
    }

    private static final class MockDto {
        public String id;
        public MockRequestDto request;
        public MockResponseDto response;
    }

    private static final class MockRequestDto {
        public String method;
        public String path;
        public Map<String, String> headers;
    }

    private static final class MockResponseDto {
        public int status;
        public String body;
        public Map<String, String> headers;
        public String delay;
    }

    private static final class ScenarioDto {
        public String id;
        public String name;
        public String description;
        public List<ScenarioPatchDto> patches;
    }

    private static final class ScenarioPatchDto {
        public String mockId;
        public Integer status;
        public String body;
        public Map<String, String> headers;
        public String delay;
        public Boolean disabled;
    }

    private static final class ActiveScenariosResponseDto {
        public List<String> active;
        public List<ScenarioDto> scenarios;
    }

    private static final class CallSummaryDto {
        public String mockId;
        public long count;
        public List<CallEntryDto> calls;
    }

    private static final class CallEntryDto {
        public String id;
        public String timestamp;
        public String protocol;
        public String method;
        public String path;
        public int status;
        public long durationMs;
        public Map<String, String> headers;
        public String body;
        public String matchedId;
        public Map<String, String> pathParams;
    }
}
