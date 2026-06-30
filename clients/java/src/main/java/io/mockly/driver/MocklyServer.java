package io.mockly.driver;

import io.mockly.driver.model.CallEntry;
import io.mockly.driver.model.CallSummary;
import io.mockly.driver.model.FaultConfig;
import io.mockly.driver.model.Mock;
import io.mockly.driver.model.MockRequest;
import io.mockly.driver.model.MockResponse;
import io.mockly.driver.model.Scenario;
import io.mockly.driver.model.ScenarioPatch;

import java.io.IOException;
import java.net.ServerSocket;
import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Duration;
import java.time.Duration;
import java.util.ArrayList;
import java.util.HashMap;
import java.util.List;
import java.util.Map;

/**
 * Manages a Mockly server process lifecycle and exposes the Management REST API.
 *
 * <pre>{@code
 * try (MocklyServer server = MocklyServer.ensure(MocklyConfig.builder().build())) {
 *     server.addMock(Mock.builder("hello",
 *         MockRequest.builder("GET", "/hello").build(),
 *         MockResponse.builder(200).body("world").build()
 *     ).build());
 *
 *     // ... your tests against server.httpBase ...
 * }
 * }</pre>
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

    /**
     * Starts the server using an already-installed binary.
     * Retries up to 3 times if port allocation races occur.
     * @param config server configuration
     * @return a running {@link MocklyServer} instance
     * @throws IOException if the binary cannot be found or the server fails to start
     * @throws InterruptedException if the thread is interrupted while waiting for the server to be ready
     */
    public static MocklyServer create(MocklyConfig config) throws IOException, InterruptedException {
        String binaryPath = resolveBinaryPath(config);
        return startWithRetry(binaryPath, config, 3);
    }

    /**
     * Installs the binary if not present, then starts the server.
     * @param config server configuration
     * @return a running {@link MocklyServer} instance
     * @throws IOException if installation or server startup fails
     * @throws InterruptedException if the thread is interrupted while waiting for the server to be ready
     */
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

    /**
     * Stops the server and cleans up the temporary config file.
     * @throws InterruptedException if the thread is interrupted while waiting for the process to exit
     */
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

    /**
     * Registers a new HTTP mock.
     * @param mock the mock definition to register
     * @throws IOException if the server returns an unexpected response
     * @throws InterruptedException if the thread is interrupted during the HTTP request
     */
    public void addMock(Mock mock) throws IOException, InterruptedException {
        String json = toJson(mock);
        HttpResponse<String> resp = post("/api/mocks/http", json);
        if (resp.statusCode() != 201) {
            throw new IOException("addMock failed: HTTP " + resp.statusCode() + " — " + resp.body());
        }
    }

    /**
     * Removes a registered HTTP mock by ID.
     * @param id the ID of the mock to delete
     * @throws IOException if the server returns an unexpected response
     * @throws InterruptedException if the thread is interrupted during the HTTP request
     */
    public void deleteMock(String id) throws IOException, InterruptedException {
        HttpResponse<String> resp = delete("/api/mocks/http/" + id);
        if (resp.statusCode() != 204) {
            throw new IOException("deleteMock failed: HTTP " + resp.statusCode() + " — " + resp.body());
        }
    }

    /**
     * Resets all mocks and state on the server.
     * @throws IOException if the server returns an unexpected response
     * @throws InterruptedException if the thread is interrupted during the HTTP request
     */
    public void reset() throws IOException, InterruptedException {
        HttpResponse<String> resp = post("/api/reset", "");
        if (resp.statusCode() != 200) {
            throw new IOException("reset failed: HTTP " + resp.statusCode() + " — " + resp.body());
        }
    }

    /**
     * Activates a pre-configured scenario by ID.
     * @param id the scenario ID to activate
     * @throws IOException if the server returns an unexpected response
     * @throws InterruptedException if the thread is interrupted during the HTTP request
     */
    public void activateScenario(String id) throws IOException, InterruptedException {
        HttpResponse<String> resp = post("/api/scenarios/" + id + "/activate", "");
        if (resp.statusCode() != 200) {
            throw new IOException("activateScenario failed: HTTP " + resp.statusCode() + " — " + resp.body());
        }
    }

    /**
     * Deactivates a scenario by ID.
     * @param id the scenario ID to deactivate
     * @throws IOException if the server returns an unexpected response
     * @throws InterruptedException if the thread is interrupted during the HTTP request
     */
    public void deactivateScenario(String id) throws IOException, InterruptedException {
        HttpResponse<String> resp = delete("/api/scenarios/" + id + "/activate");
        if (resp.statusCode() != 200) {
            throw new IOException("deactivateScenario failed: HTTP " + resp.statusCode() + " — " + resp.body());
        }
    }

    /**
     * Injects a network fault.
     * @param config the fault configuration to apply
     * @throws IOException if the server returns an unexpected response
     * @throws InterruptedException if the thread is interrupted during the HTTP request
     */
    public void setFault(FaultConfig config) throws IOException, InterruptedException {
        String json = toJson(config);
        HttpResponse<String> resp = post("/api/fault", json);
        if (resp.statusCode() != 200) {
            throw new IOException("setFault failed: HTTP " + resp.statusCode() + " — " + resp.body());
        }
    }

    /**
     * Clears any active fault.
     * @throws IOException if the server returns an unexpected response
     * @throws InterruptedException if the thread is interrupted during the HTTP request
     */
    public void clearFault() throws IOException, InterruptedException {
        HttpResponse<String> resp = delete("/api/fault");
        if (resp.statusCode() != 200) {
            throw new IOException("clearFault failed: HTTP " + resp.statusCode() + " — " + resp.body());
        }
    }

    /**
     * Returns recorded calls for the given mock ID.
     * @param mockId the mock ID to look up
     * @throws IOException if the server returns an unexpected response
     * @throws InterruptedException if the thread is interrupted during the HTTP request
     */
    public CallSummary getCalls(String mockId) throws IOException, InterruptedException {
        HttpResponse<String> resp = get("/api/calls/http/" + mockId);
        if (resp.statusCode() != 200) {
            throw new IOException("getCalls failed: HTTP " + resp.statusCode() + " — " + resp.body());
        }
        return parseCallSummary(resp.body());
    }

    /**
     * Clears recorded calls for the given mock ID.
     * @param mockId the mock ID to clear calls for
     * @throws IOException if the server returns an unexpected response
     * @throws InterruptedException if the thread is interrupted during the HTTP request
     */
    public void clearCalls(String mockId) throws IOException, InterruptedException {
        HttpResponse<String> resp = delete("/api/calls/http/" + mockId);
        if (resp.statusCode() != 200) {
            throw new IOException("clearCalls failed: HTTP " + resp.statusCode() + " — " + resp.body());
        }
    }

    /**
     * Clears all recorded HTTP calls across every mock.
     * @throws IOException if the server returns an unexpected response
     * @throws InterruptedException if the thread is interrupted during the HTTP request
     */
    public void clearAllCalls() throws IOException, InterruptedException {
        HttpResponse<String> resp = delete("/api/calls/http");
        if (resp.statusCode() != 200) {
            throw new IOException("clearAllCalls failed: HTTP " + resp.statusCode() + " — " + resp.body());
        }
    }

    /**
     * Blocks until the mock has been called at least {@code count} times, or until
     * {@code timeout} elapses. Throws on timeout.
     * @param mockId  the mock ID to wait on
     * @param count   minimum number of calls to wait for
     * @param timeout maximum time to wait
     * @throws IOException if the server returns an unexpected response or timeout is reached
     * @throws InterruptedException if the thread is interrupted during the HTTP request
     */
    public CallSummary waitForCalls(String mockId, int count, Duration timeout)
            throws IOException, InterruptedException {
        String json = "{\"count\":" + count + ",\"timeout\":\"" + timeout.getSeconds() + "s\"}";
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
                // If ports were explicitly provided, don't retry.
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
        // Inherit stdout/stderr so logs are visible in test output.
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
                        if (p.getDelay() != null) {
                            yaml.append("        delay: ").append(yamlStr(p.getDelay())).append("\n");
                        }
                    }
                }
            }
        }

        Path tmp = Files.createTempFile("mockly-config-", ".yaml");
        Files.writeString(tmp, yaml.toString());
        return tmp;
    }

    /** Minimal YAML single-quote escaper. */
    private static String yamlStr(String s) {
        return "'" + s.replace("'", "''") + "'";
    }

    private static int getFreePort() throws IOException {
        try (ServerSocket socket = new ServerSocket(0)) {
            socket.setReuseAddress(true);
            return socket.getLocalPort();
        }
    }

    /**
     * Allocates {@code n} free ports atomically by holding all sockets open
     * simultaneously before closing them together. This prevents another process
     * from claiming a port in the gap between sequential allocations.
     */
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
        HttpRequest.BodyPublisher publisher = jsonBody.isEmpty()
                ? HttpRequest.BodyPublishers.noBody()
                : HttpRequest.BodyPublishers.ofString(jsonBody);

        HttpRequest.Builder rb = HttpRequest.newBuilder()
                .uri(URI.create(apiBase + path))
                .POST(publisher)
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

    /**
     * Parses the JSON body returned by the /api/calls/http/{mockId} and
     * /api/calls/http/{mockId}/wait endpoints into a {@link CallSummary}.
     *
     * This is a purposely simple implementation that handles the known schema
     * without requiring an external JSON library. It is not a general parser.
     */
    static CallSummary parseCallSummary(String json) {
        String mockId  = extractString(json, "mock_id");
        long   count   = extractLong(json, "count");
        List<CallEntry> calls = parseCallEntries(json);
        return new CallSummary(mockId, count, calls);
    }

    private static List<CallEntry> parseCallEntries(String json) {
        List<CallEntry> result = new ArrayList<>();
        // Locate the "calls" array.
        int arrStart = json.indexOf("\"calls\"");
        if (arrStart < 0) return result;
        int openBracket = json.indexOf('[', arrStart);
        if (openBracket < 0) return result;
        int closeBracket = matchingBracket(json, openBracket, '[', ']');
        if (closeBracket < 0) return result;

        String arr = json.substring(openBracket + 1, closeBracket).trim();
        // Split on top-level object boundaries.
        List<String> objects = splitTopLevelObjects(arr);
        for (String obj : objects) {
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

    /** Extracts the first string value for the given JSON key, or null. */
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
            // Quoted string
            StringBuilder sb = new StringBuilder();
            int i = start + 1;
            while (i < json.length()) {
                char c = json.charAt(i);
                if (c == '\\' && i + 1 < json.length()) {
                    char next = json.charAt(i + 1);
                    switch (next) {
                        case '"':  sb.append('"'); i += 2; break;
                        case '\\': sb.append('\\'); i += 2; break;
                        case 'n':  sb.append('\n'); i += 2; break;
                        case 'r':  sb.append('\r'); i += 2; break;
                        case 't':  sb.append('\t'); i += 2; break;
                        default:   sb.append(next); i += 2; break;
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

    /** Extracts a long (integer) value for the given JSON key, or 0. */
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

    /** Extracts a JSON object as a Map<String,String> for the given key. */
    private static Map<String, String> extractStringMap(String json, String key) {
        Map<String, String> result = new HashMap<>();
        String needle = "\"" + key + "\"";
        int ki = json.indexOf(needle);
        if (ki < 0) return result;
        int colon = json.indexOf(':', ki + needle.length());
        if (colon < 0) return result;
        int start = colon + 1;
        while (start < json.length() && Character.isWhitespace(json.charAt(start))) start++;
        if (start >= json.length() || json.charAt(start) != '{') return result;
        int end = matchingBracket(json, start, '{', '}');
        if (end < 0) return result;
        String inner = json.substring(start + 1, end);
        // Parse key:value pairs
        int pos = 0;
        while (pos < inner.length()) {
            while (pos < inner.length() && inner.charAt(pos) != '"') pos++;
            if (pos >= inner.length()) break;
            int kStart = pos + 1;
            int kEnd = inner.indexOf('"', kStart);
            if (kEnd < 0) break;
            String k = inner.substring(kStart, kEnd);
            int c2 = inner.indexOf(':', kEnd);
            if (c2 < 0) break;
            int vStart = c2 + 1;
            while (vStart < inner.length() && Character.isWhitespace(inner.charAt(vStart))) vStart++;
            if (vStart >= inner.length() || inner.charAt(vStart) != '"') { pos = c2 + 1; continue; }
            StringBuilder sb = new StringBuilder();
            int i = vStart + 1;
            while (i < inner.length()) {
                char ch = inner.charAt(i);
                if (ch == '\\' && i + 1 < inner.length()) {
                    sb.append(inner.charAt(i + 1)); i += 2;
                } else if (ch == '"') {
                    pos = i + 1; break;
                } else {
                    sb.append(ch); i++;
                }
            }
            result.put(k, sb.toString());
        }
        return result;
    }

    /** Finds the index of the matching closing bracket for the opening bracket at `openIdx`. */
    private static int matchingBracket(String s, int openIdx, char open, char close) {
        int depth = 0;
        for (int i = openIdx; i < s.length(); i++) {
            if (s.charAt(i) == open)  depth++;
            else if (s.charAt(i) == close) { depth--; if (depth == 0) return i; }
        }
        return -1;
    }

    /** Splits a JSON array body (without surrounding []) into individual top-level object strings. */
    private static List<String> splitTopLevelObjects(String arr) {
        List<String> objects = new ArrayList<>();
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
}
