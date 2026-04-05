package io.mockly.driver;

import io.mockly.driver.model.FaultConfig;
import io.mockly.driver.model.Mock;
import io.mockly.driver.model.MockRequest;
import io.mockly.driver.model.MockResponse;

import java.io.IOException;
import java.net.ServerSocket;
import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Duration;
import java.util.ArrayList;
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
     */
    public static MocklyServer create(MocklyConfig config) throws IOException, InterruptedException {
        String binaryPath = resolveBinaryPath(config);
        return startWithRetry(binaryPath, config, 3);
    }

    /**
     * Installs the binary if not present, then starts the server.
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

    /** Stops the server and cleans up the temporary config file. */
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

    /** Registers a new HTTP mock. */
    public void addMock(Mock mock) throws IOException, InterruptedException {
        String json = toJson(mock);
        HttpResponse<String> resp = post("/api/mocks/http", json);
        if (resp.statusCode() != 201) {
            throw new IOException("addMock failed: HTTP " + resp.statusCode() + " — " + resp.body());
        }
    }

    /** Removes a registered HTTP mock by ID. */
    public void deleteMock(String id) throws IOException, InterruptedException {
        HttpResponse<String> resp = delete("/api/mocks/http/" + id);
        if (resp.statusCode() != 204) {
            throw new IOException("deleteMock failed: HTTP " + resp.statusCode() + " — " + resp.body());
        }
    }

    /** Resets all mocks and state on the server. */
    public void reset() throws IOException, InterruptedException {
        HttpResponse<String> resp = post("/api/reset", "");
        if (resp.statusCode() != 200) {
            throw new IOException("reset failed: HTTP " + resp.statusCode() + " — " + resp.body());
        }
    }

    /** Activates a pre-configured scenario by ID. */
    public void activateScenario(String id) throws IOException, InterruptedException {
        HttpResponse<String> resp = post("/api/scenarios/" + id + "/activate", "");
        if (resp.statusCode() != 200) {
            throw new IOException("activateScenario failed: HTTP " + resp.statusCode() + " — " + resp.body());
        }
    }

    /** Deactivates a scenario by ID. */
    public void deactivateScenario(String id) throws IOException, InterruptedException {
        HttpResponse<String> resp = post("/api/scenarios/" + id + "/deactivate", "");
        if (resp.statusCode() != 200) {
            throw new IOException("deactivateScenario failed: HTTP " + resp.statusCode() + " — " + resp.body());
        }
    }

    /** Injects a network fault. */
    public void setFault(FaultConfig config) throws IOException, InterruptedException {
        String json = toJson(config);
        HttpResponse<String> resp = post("/api/fault", json);
        if (resp.statusCode() != 200) {
            throw new IOException("setFault failed: HTTP " + resp.statusCode() + " — " + resp.body());
        }
    }

    /** Clears any active fault. */
    public void clearFault() throws IOException, InterruptedException {
        HttpResponse<String> resp = delete("/api/fault");
        if (resp.statusCode() != 200) {
            throw new IOException("clearFault failed: HTTP " + resp.statusCode() + " — " + resp.body());
        }
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
            int httpPort = config.getHttpPort() > 0 ? config.getHttpPort() : getFreePort();
            int apiPort  = config.getApiPort()  > 0 ? config.getApiPort()  : getFreePort();

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

        Path configFile = writeConfig(httpPort, apiPort);

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

    private static Path writeConfig(int httpPort, int apiPort) throws IOException {
        String yaml = "mockly:\n"
                + "  api:\n"
                + "    port: " + apiPort + "\n"
                + "protocols:\n"
                + "  http:\n"
                + "    enabled: true\n"
                + "    port: " + httpPort + "\n";

        Path tmp = Files.createTempFile("mockly-config-", ".yaml");
        Files.writeString(tmp, yaml);
        return tmp;
    }

    private static int getFreePort() throws IOException {
        try (ServerSocket socket = new ServerSocket(0)) {
            socket.setReuseAddress(true);
            return socket.getLocalPort();
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
        sb.append("\"type\":").append(jsonString(fault.getType()));
        if (fault.getProbability() != null) {
            sb.append(",\"probability\":").append(fault.getProbability());
        }
        if (fault.getDelay() != null) {
            sb.append(",\"delay\":").append(jsonString(fault.getDelay()));
        }
        if (fault.getStatusCode() != null) {
            sb.append(",\"statusCode\":").append(fault.getStatusCode());
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
}
