package io.mockly.driver;

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

import java.io.IOException;
import java.net.ServerSocket;
import java.net.URI;
import java.net.URLEncoder;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Duration;
import java.util.ArrayList;
import java.util.HashMap;
import java.util.List;
import java.util.Map;

/**
 * Manages a Mockly server process lifecycle and exposes the Management REST API.
 */
public class MocklyServer implements AutoCloseable {

    public final int httpPort;
    public final int apiPort;
    /** Base URL of the HTTP mock server, e.g. {@code http://127.0.0.1:9000}. */
    public final String httpBase;
    /** Base URL of the management API, e.g. {@code http://127.0.0.1:9001}. */
    public final String apiBase;

    private final Process process;
    private final Path configFile;
    private final HttpClient httpClient;

    private MocklyServer(Process process, Path configFile, int httpPort, int apiPort) {
        this.process = process;
        this.configFile = configFile;
        this.httpPort = httpPort;
        this.apiPort = apiPort;
        this.httpBase = "http://127.0.0.1:" + httpPort;
        this.apiBase = "http://127.0.0.1:" + apiPort;
        this.httpClient = HttpClient.newBuilder()
                .connectTimeout(Duration.ofSeconds(5))
                .build();
    }

    // -------------------------------------------------------------------------
    // Factory methods
    // -------------------------------------------------------------------------

    public static MocklyServer create(MocklyConfig config) throws IOException, InterruptedException {
        String binaryPath = resolveBinaryPath(config);
        return startWithRetry(binaryPath, config, 3);
    }

    public static MocklyServer ensure(MocklyConfig config) throws IOException, InterruptedException {
        String binaryPath = config.getBinaryPath();
        if (binaryPath == null) {
            MocklyInstaller.InstallOptions opts = MocklyInstaller.InstallOptions.builder()
                    .binDir(config.getBinDir())
                    .version(config.getVersion())
                    .build();
            binaryPath = MocklyInstaller.install(opts);
        }
        return startWithRetry(binaryPath, config, 3);
    }

    // -------------------------------------------------------------------------
    // Lifecycle
    // -------------------------------------------------------------------------

    public void stop() throws InterruptedException {
        if (process.isAlive()) {
            process.destroy();
            process.waitFor();
        }
        if (configFile != null) {
            try { Files.deleteIfExists(configFile); } catch (IOException ignored) { }
        }
    }

    @Override
    public void close() throws Exception {
        stop();
    }

    // -------------------------------------------------------------------------
    // Management API
    // -------------------------------------------------------------------------

    public void addMock(Mock mock) throws IOException, InterruptedException {
        HttpResponse<String> resp = post("/api/mocks/http", toJson(mock));
        if (resp.statusCode() != 201) {
            throw new IOException("addMock failed: HTTP " + resp.statusCode() + " — " + resp.body());
        }
    }

    public List<Mock> listMocks() throws IOException, InterruptedException {
        HttpResponse<String> resp = get("/api/mocks/http");
        if (resp.statusCode() != 200) {
            throw new IOException("listMocks failed: HTTP " + resp.statusCode() + " — " + resp.body());
        }
        return parseMocks(resp.body());
    }

    public Mock updateMock(String id, Mock mock) throws IOException, InterruptedException {
        HttpResponse<String> resp = put("/api/mocks/http/" + id, toJson(mock));
        if (resp.statusCode() != 200) {
            throw new IOException("updateMock failed: HTTP " + resp.statusCode() + " — " + resp.body());
        }
        return parseMock(resp.body());
    }

    public Mock patchMock(String id, MockResponsePatch patch) throws IOException, InterruptedException {
        HttpResponse<String> resp = patch("/api/mocks/http/" + id, toJson(patch));
        if (resp.statusCode() != 200) {
            throw new IOException("patchMock failed: HTTP " + resp.statusCode() + " — " + resp.body());
        }
        return parseMock(resp.body());
    }

    public void deleteMock(String id) throws IOException, InterruptedException {
        HttpResponse<String> resp = delete("/api/mocks/http/" + id);
        if (resp.statusCode() != 200) {
            throw new IOException("deleteMock failed: HTTP " + resp.statusCode() + " — " + resp.body());
        }
    }

    public Map<String, String> getState() throws IOException, InterruptedException {
        HttpResponse<String> resp = get("/api/state");
        if (resp.statusCode() != 200) {
            throw new IOException("getState failed: HTTP " + resp.statusCode() + " — " + resp.body());
        }
        return parseStringMapBody(resp.body());
    }

    public Map<String, String> setState(Map<String, String> kvMap) throws IOException, InterruptedException {
        HttpResponse<String> resp = post("/api/state", jsonMap(kvMap));
        if (resp.statusCode() != 200) {
            throw new IOException("setState failed: HTTP " + resp.statusCode() + " — " + resp.body());
        }
        return parseStringMapBody(resp.body());
    }

    public void deleteState(String key) throws IOException, InterruptedException {
        HttpResponse<String> resp = delete("/api/state/" + key);
        if (resp.statusCode() != 200) {
            throw new IOException("deleteState failed: HTTP " + resp.statusCode() + " — " + resp.body());
        }
    }

    public List<CallEntry> getLogs() throws IOException, InterruptedException {
        return getLogs(null);
    }

    public List<CallEntry> getLogs(String matchedId) throws IOException, InterruptedException {
        HttpResponse<String> resp = get(withMatchedId("/api/logs", matchedId));
        if (resp.statusCode() != 200) {
            throw new IOException("getLogs failed: HTTP " + resp.statusCode() + " — " + resp.body());
        }
        return parseCallEntriesArray(resp.body());
    }

    public void clearLogs() throws IOException, InterruptedException {
        HttpResponse<String> resp = delete("/api/logs");
        if (resp.statusCode() != 200) {
            throw new IOException("clearLogs failed: HTTP " + resp.statusCode() + " — " + resp.body());
        }
    }

    public int getLogsCount() throws IOException, InterruptedException {
        return getLogsCount(null);
    }

    public int getLogsCount(String matchedId) throws IOException, InterruptedException {
        HttpResponse<String> resp = get(withMatchedId("/api/logs/count", matchedId));
        if (resp.statusCode() != 200) {
            throw new IOException("getLogsCount failed: HTTP " + resp.statusCode() + " — " + resp.body());
        }
        return (int) extractLong(resp.body(), "count");
    }

    public List<Scenario> listScenarios() throws IOException, InterruptedException {
        HttpResponse<String> resp = get("/api/scenarios");
        if (resp.statusCode() != 200) {
            throw new IOException("listScenarios failed: HTTP " + resp.statusCode() + " — " + resp.body());
        }
        return parseScenarios(resp.body());
    }

    public Scenario createScenario(Scenario scenario) throws IOException, InterruptedException {
        HttpResponse<String> resp = post("/api/scenarios", toJson(scenario));
        if (resp.statusCode() != 201) {
            throw new IOException("createScenario failed: HTTP " + resp.statusCode() + " — " + resp.body());
        }
        return parseScenario(resp.body());
    }

    public Scenario getScenario(String id) throws IOException, InterruptedException {
        HttpResponse<String> resp = get("/api/scenarios/" + id);
        if (resp.statusCode() != 200) {
            throw new IOException("getScenario failed: HTTP " + resp.statusCode() + " — " + resp.body());
        }
        return parseScenario(resp.body());
    }

    public Scenario updateScenario(String id, Scenario scenario) throws IOException, InterruptedException {
        HttpResponse<String> resp = put("/api/scenarios/" + id, toJson(scenario));
        if (resp.statusCode() != 200) {
            throw new IOException("updateScenario failed: HTTP " + resp.statusCode() + " — " + resp.body());
        }
        return parseScenario(resp.body());
    }

    public void deleteScenario(String id) throws IOException, InterruptedException {
        HttpResponse<String> resp = delete("/api/scenarios/" + id);
        if (resp.statusCode() != 200) {
            throw new IOException("deleteScenario failed: HTTP " + resp.statusCode() + " — " + resp.body());
        }
    }

    public ActiveScenariosResponse listActiveScenarios() throws IOException, InterruptedException {
        HttpResponse<String> resp = get("/api/scenarios/active");
        if (resp.statusCode() != 200) {
            throw new IOException("listActiveScenarios failed: HTTP " + resp.statusCode() + " — " + resp.body());
        }
        return parseActiveScenariosResponse(resp.body());
    }

    public void reset() throws IOException, InterruptedException {
        HttpResponse<String> resp = post("/api/reset", "");
        if (resp.statusCode() != 200) {
            throw new IOException("reset failed: HTTP " + resp.statusCode() + " — " + resp.body());
        }
    }

    public void activateScenario(String id) throws IOException, InterruptedException {
        HttpResponse<String> resp = post("/api/scenarios/" + id + "/activate", "");
        if (resp.statusCode() != 200) {
            throw new IOException("activateScenario failed: HTTP " + resp.statusCode() + " — " + resp.body());
        }
    }

    public void deactivateScenario(String id) throws IOException, InterruptedException {
        HttpResponse<String> resp = delete("/api/scenarios/" + id + "/activate");
        if (resp.statusCode() != 200) {
            throw new IOException("deactivateScenario failed: HTTP " + resp.statusCode() + " — " + resp.body());
        }
    }

    public void setFault(FaultConfig config) throws IOException, InterruptedException {
        HttpResponse<String> resp = post("/api/fault/http", toJson(config));
        if (resp.statusCode() != 200) {
            throw new IOException("setFault failed: HTTP " + resp.statusCode() + " — " + resp.body());
        }
    }

    public void clearFault() throws IOException, InterruptedException {
        HttpResponse<String> resp = delete("/api/fault");
        if (resp.statusCode() != 200 && resp.statusCode() != 204) {
            throw new IOException("clearFault failed: HTTP " + resp.statusCode() + " — " + resp.body());
        }
    }

    public CallSummary getCalls(String mockId) throws IOException, InterruptedException {
        HttpResponse<String> resp = get("/api/calls/http/" + mockId);
        if (resp.statusCode() != 200) {
            throw new IOException("getCalls failed: HTTP " + resp.statusCode() + " — " + resp.body());
        }
        return parseCallSummary(resp.body());
    }

    public void clearCalls(String mockId) throws IOException, InterruptedException {
        HttpResponse<String> resp = delete("/api/calls/http/" + mockId);
        if (resp.statusCode() != 200) {
            throw new IOException("clearCalls failed: HTTP " + resp.statusCode() + " — " + resp.body());
        }
    }

    public void clearAllCalls() throws IOException, InterruptedException {
        HttpResponse<String> resp = delete("/api/calls/http");
        if (resp.statusCode() != 200) {
            throw new IOException("clearAllCalls failed: HTTP " + resp.statusCode() + " — " + resp.body());
        }
    }

    public CallSummary waitForCalls(String mockId, int count, Duration timeout)
            throws IOException, InterruptedException {
        String json = "{\"count\":" + count + ",\"timeout\":\"" + timeout.toMillis() + "ms\"}";
        HttpResponse<String> resp = post("/api/calls/http/" + mockId + "/wait", json);
        if (resp.statusCode() == 408) {
            throw new IOException("waitForCalls: timeout waiting for " + count
                    + " call(s) on '" + mockId + "'");
        }
        if (resp.statusCode() != 200) {
            throw new IOException("waitForCalls failed: HTTP " + resp.statusCode() + " — " + resp.body());
        }
        return parseCallSummary(resp.body());
    }

    // -------------------------------------------------------------------------
    // Internal helpers — startup
    // -------------------------------------------------------------------------

    private static String resolveBinaryPath(MocklyConfig config) throws IOException {
        if (config.getBinaryPath() != null) {
            return config.getBinaryPath();
        }
        String found = MocklyInstaller.getBinaryPath(config.getBinDir());
        if (found == null) {
            throw new IOException(
                    "Mockly binary not found. Use MocklyServer.ensure() to install it first, "
                    + "or set MOCKLY_BINARY_PATH.");
        }
        return found;
    }

    private static MocklyServer startWithRetry(String binaryPath, MocklyConfig config, int maxAttempts)
            throws IOException, InterruptedException {

        IOException lastError = null;
        for (int attempt = 0; attempt < maxAttempts; attempt++) {
            int[] ports = (config.getHttpPort() > 0 && config.getApiPort() > 0)
                    ? new int[]{config.getHttpPort(), config.getApiPort()}
                    : getFreePorts(2);
            int httpPort = config.getHttpPort() > 0 ? config.getHttpPort() : ports[0];
            int apiPort  = config.getApiPort()  > 0 ? config.getApiPort()  : ports[1];

            try {
                return doStart(binaryPath, config, httpPort, apiPort);
            } catch (IOException e) {
                lastError = e;
                if (config.getHttpPort() > 0 || config.getApiPort() > 0) {
                    break;
                }
            }
        }
        throw new IOException("Failed to start Mockly server after " + maxAttempts + " attempts", lastError);
    }

    private static MocklyServer doStart(String binaryPath, MocklyConfig config, int httpPort, int apiPort)
            throws IOException, InterruptedException {

        Path configFile = writeConfig(httpPort, apiPort, config.getScenarios());

        List<String> cmd = new ArrayList<>();
        cmd.add(binaryPath);
        cmd.add("start");
        cmd.add("--config");
        cmd.add(configFile.toAbsolutePath().toString());
        cmd.add("--api-port");
        cmd.add(String.valueOf(apiPort));
        cmd.addAll(config.getExtraArgs());

        ProcessBuilder pb = new ProcessBuilder(cmd);
        pb.redirectErrorStream(true);
        pb.inheritIO();

        Process process = pb.start();
        MocklyServer server = new MocklyServer(process, configFile, httpPort, apiPort);

        try {
            server.waitReady(config.getStartupTimeoutMs());
        } catch (IOException e) {
            server.stop();
            throw e;
        }

        return server;
    }

    private static Path writeConfig(int httpPort, int apiPort, List<Scenario> scenarios) throws IOException {
        StringBuilder yaml = new StringBuilder();
        yaml.append("mockly:\n  api:\n    port: ").append(apiPort)
            .append("\nprotocols:\n  http:\n    enabled: true\n    port: ").append(httpPort).append("\n");

        if (scenarios != null && !scenarios.isEmpty()) {
            yaml.append("scenarios:\n");
            for (Scenario s : scenarios) {
                yaml.append("  - id: ").append(yamlStr(s.getId())).append("\n");
                yaml.append("    name: ").append(yamlStr(s.getName())).append("\n");
                if (s.getDescription() != null) {
                    yaml.append("    description: ").append(yamlStr(s.getDescription())).append("\n");
                }
                if (!s.getPatches().isEmpty()) {
                    yaml.append("    patches:\n");
                    for (ScenarioPatch p : s.getPatches()) {
                        yaml.append("      - mock_id: ").append(yamlStr(p.getMockId())).append("\n");
                        if (p.getStatus() != null) {
                            yaml.append("        status: ").append(p.getStatus()).append("\n");
                        }
                        if (p.getBody() != null) {
                            yaml.append("        body: ").append(yamlStr(p.getBody())).append("\n");
                        }
                        if (!p.getHeaders().isEmpty()) {
                            yaml.append("        headers:\n");
                            for (Map.Entry<String, String> header : p.getHeaders().entrySet()) {
                                yaml.append("          ")
                                        .append(yamlStr(header.getKey()))
                                        .append(": ")
                                        .append(yamlStr(header.getValue()))
                                        .append("\n");
                            }
                        }
                        if (p.getDelay() != null) {
                            yaml.append("        delay: ").append(yamlStr(p.getDelay())).append("\n");
                        }
                        if (p.getDisabled() != null) {
                            yaml.append("        disabled: ").append(p.getDisabled()).append("\n");
                        }
                    }
                }
            }
        }

        Path tmp = Files.createTempFile("mockly-config-", ".yaml");
        Files.writeString(tmp, yaml.toString());
        return tmp;
    }

    private static String yamlStr(String s) {
        return "'" + s.replace("'", "''") + "'";
    }

    private static int getFreePort() throws IOException {
        try (ServerSocket socket = new ServerSocket(0)) {
            socket.setReuseAddress(true);
            return socket.getLocalPort();
        }
    }

    private static int[] getFreePorts(int n) throws IOException {
        ServerSocket[] sockets = new ServerSocket[n];
        try {
            for (int i = 0; i < n; i++) {
                sockets[i] = new ServerSocket(0);
                sockets[i].setReuseAddress(true);
            }
            int[] ports = new int[n];
            for (int i = 0; i < n; i++) {
                ports[i] = sockets[i].getLocalPort();
            }
            return ports;
        } finally {
            for (ServerSocket s : sockets) {
                if (s != null) {
                    try { s.close(); } catch (IOException ignored) { }
                }
            }
        }
    }

    static boolean isPortConflict(String msg) {
        if (msg == null) return false;
        String lower = msg.toLowerCase(java.util.Locale.ROOT);
        return lower.contains("address already in use")
                || lower.contains("eaddrinuse")
                || lower.contains("bind: already in use");
    }

    private void waitReady(int maxMs) throws IOException, InterruptedException {
        long deadline = System.currentTimeMillis() + maxMs;
        IOException lastException = null;

        while (System.currentTimeMillis() < deadline) {
            if (!process.isAlive()) {
                throw new IOException("Mockly process exited unexpectedly with code " + process.exitValue());
            }
            try {
                HttpRequest req = HttpRequest.newBuilder()
                        .uri(URI.create(apiBase + "/api/protocols"))
                        .GET()
                        .timeout(Duration.ofMillis(1000))
                        .build();
                HttpResponse<String> resp = httpClient.send(req, HttpResponse.BodyHandlers.ofString());
                if (resp.statusCode() == 200) {
                    return;
                }
            } catch (IOException e) {
                lastException = e;
            }
            Thread.sleep(50);
        }

        throw new IOException("Mockly server did not become ready within " + maxMs + "ms", lastException);
    }

    // -------------------------------------------------------------------------
    // Internal helpers — HTTP
    // -------------------------------------------------------------------------

    private HttpResponse<String> get(String path) throws IOException, InterruptedException {
        HttpRequest req = HttpRequest.newBuilder()
                .uri(URI.create(apiBase + path))
                .GET()
                .timeout(Duration.ofSeconds(30))
                .build();
        return httpClient.send(req, HttpResponse.BodyHandlers.ofString());
    }

    private HttpResponse<String> post(String path, String jsonBody) throws IOException, InterruptedException {
        return sendWithBody("POST", path, jsonBody);
    }

    private HttpResponse<String> put(String path, String jsonBody) throws IOException, InterruptedException {
        return sendWithBody("PUT", path, jsonBody);
    }

    private HttpResponse<String> patch(String path, String jsonBody) throws IOException, InterruptedException {
        return sendWithBody("PATCH", path, jsonBody);
    }

    private HttpResponse<String> sendWithBody(String method, String path, String jsonBody)
            throws IOException, InterruptedException {
        HttpRequest.BodyPublisher publisher = jsonBody.isEmpty()
                ? HttpRequest.BodyPublishers.noBody()
                : HttpRequest.BodyPublishers.ofString(jsonBody);

        HttpRequest.Builder rb = HttpRequest.newBuilder()
                .uri(URI.create(apiBase + path))
                .method(method, publisher)
                .timeout(Duration.ofSeconds(10));

        if (!jsonBody.isEmpty()) {
            rb.header("Content-Type", "application/json");
        }

        return httpClient.send(rb.build(), HttpResponse.BodyHandlers.ofString());
    }

    private HttpResponse<String> delete(String path) throws IOException, InterruptedException {
        HttpRequest req = HttpRequest.newBuilder()
                .uri(URI.create(apiBase + path))
                .DELETE()
                .timeout(Duration.ofSeconds(10))
                .build();
        return httpClient.send(req, HttpResponse.BodyHandlers.ofString());
    }

    // -------------------------------------------------------------------------
    // JSON serialization (no external dependencies)
    // -------------------------------------------------------------------------

    static String toJson(Mock mock) {
        StringBuilder sb = new StringBuilder("{");
        sb.append("\"id\":").append(jsonString(mock.getId())).append(",");
        sb.append("\"request\":").append(toJson(mock.getRequest())).append(",");
        sb.append("\"response\":").append(toJson(mock.getResponse()));
        sb.append("}");
        return sb.toString();
    }

    static String toJson(Scenario scenario) {
        StringBuilder sb = new StringBuilder("{");
        sb.append("\"id\":").append(jsonString(scenario.getId())).append(",");
        sb.append("\"name\":").append(jsonString(scenario.getName()));
        if (scenario.getDescription() != null) {
            sb.append(",\"description\":").append(jsonString(scenario.getDescription()));
        }
        sb.append(",\"patches\":[");
        boolean first = true;
        for (ScenarioPatch patch : scenario.getPatches()) {
            if (!first) sb.append(",");
            sb.append(toJson(patch));
            first = false;
        }
        sb.append("]}");
        return sb.toString();
    }

    static String toJson(FaultConfig fault) {
        StringBuilder sb = new StringBuilder("{");
        sb.append("\"enabled\":").append(fault.isEnabled());
        if (fault.getDelay() != null) {
            sb.append(",\"delay\":").append(jsonString(fault.getDelay()));
        }
        if (fault.getStatus() != null) {
            sb.append(",\"status\":").append(fault.getStatus());
        }
        if (fault.getErrorRate() != null) {
            sb.append(",\"error_rate\":").append(fault.getErrorRate());
        }
        sb.append("}");
        return sb.toString();
    }

    static String toJson(MockResponsePatch patch) {
        StringBuilder sb = new StringBuilder("{");
        boolean needsComma = false;
        if (patch.getStatus() != null) {
            sb.append("\"status\":").append(patch.getStatus());
            needsComma = true;
        }
        if (patch.getBody() != null) {
            if (needsComma) sb.append(",");
            sb.append("\"body\":").append(jsonString(patch.getBody()));
            needsComma = true;
        }
        if (!patch.getHeaders().isEmpty()) {
            if (needsComma) sb.append(",");
            sb.append("\"headers\":").append(jsonMap(patch.getHeaders()));
            needsComma = true;
        }
        if (patch.getDelay() != null) {
            if (needsComma) sb.append(",");
            sb.append("\"delay\":").append(jsonString(patch.getDelay()));
        }
        sb.append("}");
        return sb.toString();
    }

    private static String toJson(MockRequest req) {
        StringBuilder sb = new StringBuilder("{");
        sb.append("\"method\":").append(jsonString(req.getMethod())).append(",");
        sb.append("\"path\":").append(jsonString(req.getPath()));
        if (!req.getHeaders().isEmpty()) {
            sb.append(",\"headers\":").append(jsonMap(req.getHeaders()));
        }
        sb.append("}");
        return sb.toString();
    }

    private static String toJson(MockResponse resp) {
        StringBuilder sb = new StringBuilder("{");
        sb.append("\"status\":").append(resp.getStatus());
        if (resp.getBody() != null) {
            sb.append(",\"body\":").append(jsonString(resp.getBody()));
        }
        if (!resp.getHeaders().isEmpty()) {
            sb.append(",\"headers\":").append(jsonMap(resp.getHeaders()));
        }
        if (resp.getDelay() != null) {
            sb.append(",\"delay\":").append(jsonString(resp.getDelay()));
        }
        sb.append("}");
        return sb.toString();
    }

    private static String toJson(ScenarioPatch patch) {
        StringBuilder sb = new StringBuilder("{");
        sb.append("\"mock_id\":").append(jsonString(patch.getMockId()));
        if (patch.getStatus() != null) {
            sb.append(",\"status\":").append(patch.getStatus());
        }
        if (patch.getBody() != null) {
            sb.append(",\"body\":").append(jsonString(patch.getBody()));
        }
        if (!patch.getHeaders().isEmpty()) {
            sb.append(",\"headers\":").append(jsonMap(patch.getHeaders()));
        }
        if (patch.getDelay() != null) {
            sb.append(",\"delay\":").append(jsonString(patch.getDelay()));
        }
        if (patch.getDisabled() != null) {
            sb.append(",\"disabled\":").append(patch.getDisabled());
        }
        sb.append("}");
        return sb.toString();
    }

    static String jsonString(String s) {
        if (s == null) return "null";
        return "\"" + s
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
            if (!first) sb.append(",");
            sb.append(jsonString(entry.getKey())).append(":").append(jsonString(entry.getValue()));
            first = false;
        }
        sb.append("}");
        return sb.toString();
    }

    // -------------------------------------------------------------------------
    // JSON response parsing (minimal — no external library dependency)
    // -------------------------------------------------------------------------

    static CallSummary parseCallSummary(String json) {
        String mockId = extractString(json, "mock_id");
        long count = extractLong(json, "count");
        List<CallEntry> calls = parseCallEntries(extractArrayJson(json, "calls"));
        return new CallSummary(mockId, count, calls);
    }

    static List<CallEntry> parseCallEntriesArray(String json) {
        return parseCallEntries(stripOuterBrackets(json));
    }

    static List<Mock> parseMocks(String json) {
        List<Mock> mocks = new ArrayList<>();
        for (String obj : splitTopLevelObjects(stripOuterBrackets(json))) {
            mocks.add(parseMock(obj));
        }
        return mocks;
    }

    static Mock parseMock(String json) {
        String requestJson = extractObjectJson(json, "request");
        String responseJson = extractObjectJson(json, "response");
        MockRequest request = parseMockRequest(requestJson == null ? "{}" : requestJson);
        MockResponse response = parseMockResponse(responseJson == null ? "{}" : responseJson);
        return Mock.builder(extractString(json, "id"), request, response).build();
    }

    static List<Scenario> parseScenarios(String json) {
        List<Scenario> scenarios = new ArrayList<>();
        for (String obj : splitTopLevelObjects(stripOuterBrackets(json))) {
            scenarios.add(parseScenario(obj));
        }
        return scenarios;
    }

    static Scenario parseScenario(String json) {
        Scenario.Builder builder = Scenario.builder(
                extractString(json, "id"),
                extractString(json, "name")
        );
        String description = extractString(json, "description");
        if (description != null) {
            builder.description(description);
        }
        for (ScenarioPatch patch : parseScenarioPatches(extractArrayJson(json, "patches"))) {
            builder.patch(patch);
        }
        return builder.build();
    }

    static ActiveScenariosResponse parseActiveScenariosResponse(String json) {
        return new ActiveScenariosResponse(
                parseStringArray(extractArrayJson(json, "active")),
                parseScenarios(arrayBodyToJson(extractArrayJson(json, "scenarios")))
        );
    }

    private static MockRequest parseMockRequest(String json) {
        MockRequest.Builder builder = MockRequest.builder(
                extractString(json, "method"),
                extractString(json, "path")
        );
        builder.headers(extractStringMap(json, "headers"));
        return builder.build();
    }

    private static MockResponse parseMockResponse(String json) {
        MockResponse.Builder builder = MockResponse.builder((int) extractLong(json, "status"));
        String body = extractString(json, "body");
        if (body != null) builder.body(body);
        builder.headers(extractStringMap(json, "headers"));
        String delay = extractString(json, "delay");
        if (delay != null) builder.delay(delay);
        return builder.build();
    }

    private static List<ScenarioPatch> parseScenarioPatches(String arrayBody) {
        List<ScenarioPatch> patches = new ArrayList<>();
        if (arrayBody == null || arrayBody.isBlank()) return patches;
        for (String obj : splitTopLevelObjects(arrayBody)) {
            ScenarioPatch.Builder builder = ScenarioPatch.builder(extractString(obj, "mock_id"));
            if (containsKey(obj, "status")) builder.status((int) extractLong(obj, "status"));
            String body = extractString(obj, "body");
            if (body != null) builder.body(body);
            builder.headers(extractStringMap(obj, "headers"));
            String delay = extractString(obj, "delay");
            if (delay != null) builder.delay(delay);
            Boolean disabled = extractBoolean(obj, "disabled");
            if (disabled != null) builder.disabled(disabled);
            patches.add(builder.build());
        }
        return patches;
    }

    private static List<CallEntry> parseCallEntries(String arrayBody) {
        List<CallEntry> result = new ArrayList<>();
        if (arrayBody == null || arrayBody.isBlank()) return result;
        for (String obj : splitTopLevelObjects(arrayBody)) {
            result.add(parseCallEntry(obj));
        }
        return result;
    }

    private static CallEntry parseCallEntry(String obj) {
        return new CallEntry(
                extractString(obj, "id"),
                extractString(obj, "timestamp"),
                extractString(obj, "protocol"),
                extractString(obj, "method"),
                extractString(obj, "path"),
                (int) extractLong(obj, "status"),
                extractLong(obj, "duration_ms"),
                extractStringMap(obj, "headers"),
                extractString(obj, "body"),
                extractString(obj, "matched_id"),
                extractStringMap(obj, "path_params")
        );
    }

    static String extractString(String json, String key) {
        String needle = "\"" + key + "\"";
        int ki = json.indexOf(needle);
        if (ki < 0) return null;
        int colon = json.indexOf(':', ki + needle.length());
        if (colon < 0) return null;
        int start = colon + 1;
        while (start < json.length() && Character.isWhitespace(json.charAt(start))) start++;
        if (start >= json.length()) return null;
        if (json.charAt(start) == '"') {
            StringBuilder sb = new StringBuilder();
            int i = start + 1;
            while (i < json.length()) {
                char c = json.charAt(i);
                if (c == '\\' && i + 1 < json.length()) {
                    char next = json.charAt(i + 1);
                    switch (next) {
                        case '"': sb.append('"'); i += 2; break;
                        case '\\': sb.append('\\'); i += 2; break;
                        case 'n': sb.append('\n'); i += 2; break;
                        case 'r': sb.append('\r'); i += 2; break;
                        case 't': sb.append('\t'); i += 2; break;
                        default: sb.append(next); i += 2; break;
                    }
                } else if (c == '"') {
                    break;
                } else {
                    sb.append(c);
                    i++;
                }
            }
            return sb.toString();
        }
        if (json.startsWith("null", start)) return null;
        return null;
    }

    static long extractLong(String json, String key) {
        String needle = "\"" + key + "\"";
        int ki = json.indexOf(needle);
        if (ki < 0) return 0;
        int colon = json.indexOf(':', ki + needle.length());
        if (colon < 0) return 0;
        int start = colon + 1;
        while (start < json.length() && Character.isWhitespace(json.charAt(start))) start++;
        int end = start;
        while (end < json.length() && (Character.isDigit(json.charAt(end)) || json.charAt(end) == '-')) end++;
        if (end == start) return 0;
        try { return Long.parseLong(json.substring(start, end)); } catch (NumberFormatException e) { return 0; }
    }

    private static Boolean extractBoolean(String json, String key) {
        String needle = "\"" + key + "\"";
        int ki = json.indexOf(needle);
        if (ki < 0) return null;
        int colon = json.indexOf(':', ki + needle.length());
        if (colon < 0) return null;
        int start = colon + 1;
        while (start < json.length() && Character.isWhitespace(json.charAt(start))) start++;
        if (json.startsWith("true", start)) return true;
        if (json.startsWith("false", start)) return false;
        return null;
    }

    private static Map<String, String> extractStringMap(String json, String key) {
        String objectJson = extractObjectJson(json, key);
        if (objectJson == null) return new HashMap<>();
        return parseStringMapBody(objectJson);
    }

    private static Map<String, String> parseStringMapBody(String json) {
        Map<String, String> result = new HashMap<>();
        String inner = stripOuterBraces(json);
        int pos = 0;
        while (pos < inner.length()) {
            while (pos < inner.length() && inner.charAt(pos) != '"') pos++;
            if (pos >= inner.length()) break;
            int keyStart = pos + 1;
            StringBuilder key = new StringBuilder();
            int i = keyStart;
            while (i < inner.length()) {
                char ch = inner.charAt(i);
                if (ch == '\\' && i + 1 < inner.length()) {
                    key.append(inner.charAt(i + 1));
                    i += 2;
                } else if (ch == '"') {
                    break;
                } else {
                    key.append(ch);
                    i++;
                }
            }
            if (i >= inner.length()) break;
            int colon = inner.indexOf(':', i);
            if (colon < 0) break;
            int valueStart = colon + 1;
            while (valueStart < inner.length() && Character.isWhitespace(inner.charAt(valueStart))) valueStart++;
            if (valueStart >= inner.length() || inner.charAt(valueStart) != '"') {
                pos = colon + 1;
                continue;
            }
            StringBuilder value = new StringBuilder();
            i = valueStart + 1;
            while (i < inner.length()) {
                char ch = inner.charAt(i);
                if (ch == '\\' && i + 1 < inner.length()) {
                    value.append(inner.charAt(i + 1));
                    i += 2;
                } else if (ch == '"') {
                    pos = i + 1;
                    break;
                } else {
                    value.append(ch);
                    i++;
                }
            }
            result.put(key.toString(), value.toString());
        }
        return result;
    }

    private static String extractObjectJson(String json, String key) {
        String needle = "\"" + key + "\"";
        int ki = json.indexOf(needle);
        if (ki < 0) return null;
        int colon = json.indexOf(':', ki + needle.length());
        if (colon < 0) return null;
        int start = colon + 1;
        while (start < json.length() && Character.isWhitespace(json.charAt(start))) start++;
        if (start >= json.length() || json.charAt(start) != '{') return null;
        int end = matchingBracket(json, start, '{', '}');
        if (end < 0) return null;
        return json.substring(start, end + 1);
    }

    private static String extractArrayJson(String json, String key) {
        String needle = "\"" + key + "\"";
        int ki = json.indexOf(needle);
        if (ki < 0) return null;
        int colon = json.indexOf(':', ki + needle.length());
        if (colon < 0) return null;
        int start = colon + 1;
        while (start < json.length() && Character.isWhitespace(json.charAt(start))) start++;
        if (start >= json.length() || json.charAt(start) != '[') return null;
        int end = matchingBracket(json, start, '[', ']');
        if (end < 0) return null;
        return json.substring(start + 1, end);
    }

    private static boolean containsKey(String json, String key) {
        return json.contains("\"" + key + "\"");
    }

    private static List<String> parseStringArray(String arrayBody) {
        List<String> result = new ArrayList<>();
        if (arrayBody == null || arrayBody.isBlank()) return result;
        int i = 0;
        while (i < arrayBody.length()) {
            while (i < arrayBody.length() && arrayBody.charAt(i) != '"') i++;
            if (i >= arrayBody.length()) break;
            StringBuilder sb = new StringBuilder();
            i++;
            while (i < arrayBody.length()) {
                char ch = arrayBody.charAt(i);
                if (ch == '\\' && i + 1 < arrayBody.length()) {
                    sb.append(arrayBody.charAt(i + 1));
                    i += 2;
                } else if (ch == '"') {
                    i++;
                    break;
                } else {
                    sb.append(ch);
                    i++;
                }
            }
            result.add(sb.toString());
        }
        return result;
    }

    private static int matchingBracket(String s, int openIdx, char open, char close) {
        int depth = 0;
        boolean inString = false;
        for (int i = openIdx; i < s.length(); i++) {
            char ch = s.charAt(i);
            if (ch == '"') {
                // Count consecutive preceding backslashes; an even count means the
                // quote is not escaped (the backslashes escape each other).
                int backslashes = 0;
                int j = i - 1;
                while (j >= openIdx && s.charAt(j) == '\\') { backslashes++; j--; }
                if (backslashes % 2 == 0) {
                    inString = !inString;
                }
            }
            if (inString) continue;
            if (ch == open) depth++;
            else if (ch == close) {
                depth--;
                if (depth == 0) return i;
            }
        }
        return -1;
    }

    private static List<String> splitTopLevelObjects(String arr) {
        List<String> objects = new ArrayList<>();
        if (arr == null || arr.isBlank()) return objects;
        int i = 0;
        while (i < arr.length()) {
            while (i < arr.length() && arr.charAt(i) != '{') i++;
            if (i >= arr.length()) break;
            int end = matchingBracket(arr, i, '{', '}');
            if (end < 0) break;
            objects.add(arr.substring(i, end + 1));
            i = end + 1;
        }
        return objects;
    }

    private static String stripOuterBrackets(String json) {
        String trimmed = json == null ? "" : json.trim();
        if (trimmed.startsWith("[") && trimmed.endsWith("]")) {
            return trimmed.substring(1, trimmed.length() - 1);
        }
        return trimmed;
    }

    private static String stripOuterBraces(String json) {
        String trimmed = json == null ? "" : json.trim();
        if (trimmed.startsWith("{") && trimmed.endsWith("}")) {
            return trimmed.substring(1, trimmed.length() - 1);
        }
        return trimmed;
    }

    private static String arrayBodyToJson(String arrayBody) {
        return arrayBody == null ? "[]" : "[" + arrayBody + "]";
    }

    private static String withMatchedId(String path, String matchedId) {
        if (matchedId == null || matchedId.isEmpty()) {
            return path;
        }
        return path + "?matched_id=" + URLEncoder.encode(matchedId, StandardCharsets.UTF_8);
    }
}
