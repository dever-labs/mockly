package io.mockly.driver;

import com.sun.net.httpserver.HttpServer;
import io.mockly.driver.model.FaultConfig;
import io.mockly.driver.model.Mock;
import io.mockly.driver.model.MockRequest;
import io.mockly.driver.model.MockResponse;
import io.mockly.driver.model.MockResponsePatch;
import io.mockly.driver.model.Scenario;
import io.mockly.driver.model.ScenarioPatch;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;

import java.io.IOException;
import java.lang.reflect.Constructor;
import java.lang.reflect.Method;
import java.net.InetSocketAddress;
import java.net.ServerSocket;
import java.nio.charset.StandardCharsets;
import java.time.Duration;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.HashMap;
import java.util.List;
import java.util.Map;

import static org.junit.jupiter.api.Assertions.*;

class MocklyDriverTest {

    // -------------------------------------------------------------------------
    // Port utility
    // -------------------------------------------------------------------------

    @Test
    void getFreePortReturnsValidPort() throws Exception {
        // Verify we can open a ServerSocket on port 0 and get a valid ephemeral port.
        int port;
        try (ServerSocket socket = new ServerSocket(0)) {
            socket.setReuseAddress(true);
            port = socket.getLocalPort();
        }
        assertTrue(port >= 1024 && port <= 65535,
                "Port should be in unprivileged range [1024, 65535], got: " + port);
    }

    // -------------------------------------------------------------------------
    // MocklyInstaller — getBinaryPath
    // -------------------------------------------------------------------------

    @Test
    void getBinaryPathReturnsNullWhenMissing(@TempDir Path tempDir) {
        Map<String, String> env = new HashMap<>(); // no MOCKLY_BINARY_PATH
        String result = MocklyInstaller.getBinaryPath(tempDir.toString(), env);
        assertNull(result, "Should return null when no binary exists in the directory");
    }

    @Test
    void getBinaryPathFindsExecutableInBinDir(@TempDir Path tempDir) throws Exception {
        // Place a fake executable in tempDir.
        String exeName = MocklyInstaller.isWindows() ? "mockly.exe" : "mockly";
        Path fakeExe = tempDir.resolve(exeName);
        Files.writeString(fakeExe, "#!/bin/sh\necho mockly");

        if (!MocklyInstaller.isWindows()) {
            // Make it executable so Files.isExecutable passes.
            fakeExe.toFile().setExecutable(true);
        }

        Map<String, String> env = new HashMap<>();
        String result = MocklyInstaller.getBinaryPath(tempDir.toString(), env);

        assertNotNull(result, "Should find the fake binary in binDir");
        assertTrue(result.contains("mockly"), "Returned path should reference 'mockly'");
    }

    @Test
    void getBinaryPathPrefersEnvVar(@TempDir Path tempDir) throws Exception {
        // Place a fake executable in tempDir and also set MOCKLY_BINARY_PATH to it.
        String exeName = MocklyInstaller.isWindows() ? "mockly.exe" : "mockly";
        Path fakeExe = tempDir.resolve(exeName);
        Files.writeString(fakeExe, "#!/bin/sh\necho mockly");
        fakeExe.toFile().setExecutable(true);

        Map<String, String> env = new HashMap<>();
        env.put("MOCKLY_BINARY_PATH", fakeExe.toAbsolutePath().toString());

        String result = MocklyInstaller.getBinaryPath(tempDir.toString(), env);
        assertEquals(fakeExe.toAbsolutePath().toString(), result,
                "MOCKLY_BINARY_PATH should take precedence");
    }

    // -------------------------------------------------------------------------
    // MocklyInstaller — install
    // -------------------------------------------------------------------------

    @Test
    void installThrowsWhenNoInstallSet(@TempDir Path tempDir) {
        Map<String, String> env = new HashMap<>();
        env.put("MOCKLY_NO_INSTALL", "1");

        MocklyInstaller.InstallOptions opts = MocklyInstaller.InstallOptions.builder()
                .binDir(tempDir.toString())
                .build();

        IOException ex = assertThrows(IOException.class,
                () -> MocklyInstaller.install(opts, env));
        assertTrue(ex.getMessage().contains("MOCKLY_NO_INSTALL"),
                "Exception message should mention MOCKLY_NO_INSTALL");
    }

    @Test
    void installReturnsStagedBinary(@TempDir Path tempDir) throws Exception {
        // Simulate MOCKLY_BINARY_PATH pointing to a pre-staged binary.
        String exeName = MocklyInstaller.isWindows() ? "mockly.exe" : "mockly";
        Path staged = tempDir.resolve(exeName);
        Files.writeString(staged, "#!/bin/sh\necho mockly");
        staged.toFile().setExecutable(true);

        Map<String, String> env = new HashMap<>();
        env.put("MOCKLY_BINARY_PATH", staged.toAbsolutePath().toString());

        MocklyInstaller.InstallOptions opts = MocklyInstaller.InstallOptions.builder()
                .binDir(tempDir.toString())
                .build();

        String result = MocklyInstaller.install(opts, env);
        assertEquals(staged.toAbsolutePath().toString(), result,
                "install() should return the staged binary path from MOCKLY_BINARY_PATH");
    }

    @Test
    void installThrowsWhenBinaryPathEnvPointsToMissingFile(@TempDir Path tempDir) {
        Map<String, String> env = new HashMap<>();
        env.put("MOCKLY_BINARY_PATH", tempDir.resolve("does-not-exist").toAbsolutePath().toString());

        MocklyInstaller.InstallOptions opts = MocklyInstaller.InstallOptions.builder()
                .binDir(tempDir.toString())
                .build();

        assertThrows(IOException.class, () -> MocklyInstaller.install(opts, env));
    }

    // -------------------------------------------------------------------------
    // MocklyInstaller — asset name
    // -------------------------------------------------------------------------

    @Test
    void getAssetNameReturnsNonNullString() {
        String asset = MocklyInstaller.getAssetName();
        assertNotNull(asset);
        assertTrue(asset.startsWith("mockly-"), "Asset name should start with 'mockly-', got: " + asset);
    }

    // -------------------------------------------------------------------------
    // JSON serialization
    // -------------------------------------------------------------------------

    @Test
    void jsonStringEscapesSpecialCharacters() {
        assertEquals("\"hello \\\"world\\\"\"", MocklyServer.jsonString("hello \"world\""));
        assertEquals("\"line1\\nline2\"", MocklyServer.jsonString("line1\nline2"));
        assertEquals("\"tab\\there\"", MocklyServer.jsonString("tab\there"));
        assertEquals("\"back\\\\slash\"", MocklyServer.jsonString("back\\slash"));
        assertEquals("null", MocklyServer.jsonString(null));
    }

    @Test
    void toJsonMockProducesValidJson() {
        Mock mock = Mock.builder("get-users",
                MockRequest.builder("GET", "/users")
                        .header("Authorization", "Bearer token")
                        .build(),
                MockResponse.builder(200)
                        .body("[{\"id\":1}]")
                        .header("Content-Type", "application/json")
                        .delay("50ms")
                        .build()
        ).build();

        String json = MocklyServer.toJson(mock);

        assertTrue(json.contains("\"id\":\"get-users\""), "JSON should contain id");
        assertTrue(json.contains("\"method\":\"GET\""), "JSON should contain method");
        assertTrue(json.contains("\"path\":\"/users\""), "JSON should contain path");
        assertTrue(json.contains("\"status\":200"), "JSON should contain status");
        assertTrue(json.contains("\"delay\":\"50ms\""), "JSON should contain delay");
        assertTrue(json.contains("Authorization"), "JSON should contain request header key");
        assertTrue(json.contains("Content-Type"), "JSON should contain response header key");
    }

    @Test
    void toJsonMockWithNoOptionalFieldsIsValid() {
        Mock mock = Mock.builder("minimal",
                MockRequest.builder("POST", "/data").build(),
                MockResponse.builder(204).build()
        ).build();

        String json = MocklyServer.toJson(mock);
        assertTrue(json.contains("\"id\":\"minimal\""));
        assertTrue(json.contains("\"status\":204"));
        // No delay field expected.
        assertFalse(json.contains("\"delay\""), "No delay should appear when not set");
    }

    @Test
    void toJsonFaultConfig() {
        FaultConfig fault = FaultConfig.builder(true)
                .delay("200ms")
                .statusOverride(503)
                .errorRate(0.5)
                .build();

        String json = MocklyServer.toJson(fault);
        assertTrue(json.contains("\"enabled\":true"), "JSON should contain enabled:true");
        assertTrue(json.contains("\"delay\":\"200ms\""), "JSON should contain delay");
        assertTrue(json.contains("\"status_override\":503"), "JSON should contain status_override");
        assertTrue(json.contains("\"error_rate\":0.5"), "JSON should contain error_rate");
    }

    @Test
    void jsonMapBuildsCorrectly() {
        Map<String, String> map = new HashMap<>();
        map.put("Content-Type", "application/json");

        String json = MocklyServer.jsonMap(map);
        assertTrue(json.startsWith("{") && json.endsWith("}"));
        assertTrue(json.contains("\"Content-Type\":\"application/json\""));
    }

    // -------------------------------------------------------------------------
    // MocklyConfig
    // -------------------------------------------------------------------------

    @Test
    void mocklyConfigDefaultsAreReasonable() {
        MocklyConfig cfg = MocklyConfig.builder().build();
        assertEquals(0, cfg.getHttpPort(), "Default httpPort should be 0 (auto)");
        assertEquals(0, cfg.getApiPort(), "Default apiPort should be 0 (auto)");
        assertEquals(MocklyInstaller.DEFAULT_VERSION, cfg.getVersion());
        assertTrue(cfg.getStartupTimeoutMs() > 0);
        assertTrue(cfg.getScenarios().isEmpty(), "Default scenarios should be empty");
    }

    @Test
    void mocklyConfigBuilderSetsValues() {
        Scenario sc = Scenario.builder("sc-1", "Test Scenario").build();
        MocklyConfig cfg = MocklyConfig.builder()
                .httpPort(9000)
                .apiPort(9001)
                .version("v1.2.3")
                .binDir("custom-bin")
                .startupTimeoutMs(5000)
                .scenario(sc)
                .build();

        assertEquals(9000, cfg.getHttpPort());
        assertEquals(9001, cfg.getApiPort());
        assertEquals("v1.2.3", cfg.getVersion());
        assertEquals("custom-bin", cfg.getBinDir());
        assertEquals(5000, cfg.getStartupTimeoutMs());
        assertEquals(1, cfg.getScenarios().size(), "Should have one scenario");
        assertEquals("sc-1", cfg.getScenarios().get(0).getId());
    }

    // -------------------------------------------------------------------------
    // isPortConflict (via reflection)
    // -------------------------------------------------------------------------

    private static boolean invokeIsPortConflict(String msg) throws Exception {
        Method m = MocklyServer.class.getDeclaredMethod("isPortConflict", String.class);
        m.setAccessible(true);
        return (Boolean) m.invoke(null, msg);
    }

    @Test
    void isPortConflictReturnsTrueForAddressAlreadyInUse() throws Exception {
        assertTrue(invokeIsPortConflict("address already in use"));
    }

    @Test
    void isPortConflictReturnsTrueForEADDRINUSE() throws Exception {
        assertTrue(invokeIsPortConflict("EADDRINUSE"));
    }

    @Test
    void isPortConflictReturnsTrueForBindAlreadyInUse() throws Exception {
        assertTrue(invokeIsPortConflict("bind: already in use"));
    }

    @Test
    void isPortConflictReturnsFalseForConnectionRefused() throws Exception {
        assertFalse(invokeIsPortConflict("connection refused"));
    }

    @Test
    void isPortConflictReturnsFalseForEmptyString() throws Exception {
        assertFalse(invokeIsPortConflict(""));
    }

    @Test
    void isPortConflictReturnsFalseForTimeout() throws Exception {
        assertFalse(invokeIsPortConflict("timeout"));
    }

    // -------------------------------------------------------------------------
    // writeConfig (via reflection)
    // -------------------------------------------------------------------------

    @Test
    void writeConfigCreatesFileWithCorrectPorts() throws Exception {
        Method m = MocklyServer.class.getDeclaredMethod("writeConfig", int.class, int.class, List.class);
        m.setAccessible(true);
        Path configPath = (Path) m.invoke(null, 8080, 8081, List.of());
        try {
            assertTrue(Files.exists(configPath), "Config file should be created");
            String content = Files.readString(configPath);
            assertTrue(content.contains("8080"), "Config should contain http port 8080");
            assertTrue(content.contains("8081"), "Config should contain api port 8081");
            assertTrue(content.contains("protocols:"), "Config should contain protocols section");
            assertTrue(content.contains("mockly:"), "Config should contain mockly section");
            assertTrue(content.contains("http:"), "Config should contain http section");
        } finally {
            Files.deleteIfExists(configPath);
        }
    }

    @Test
    void writeConfigFileIsValidYamlStructure() throws Exception {
        Method m = MocklyServer.class.getDeclaredMethod("writeConfig", int.class, int.class, List.class);
        m.setAccessible(true);
        Path configPath = (Path) m.invoke(null, 7070, 7071, List.of());
        try {
            String content = Files.readString(configPath);
            assertTrue(content.lines().anyMatch(l -> l.trim().equals("port: 7070")),
                    "File should have a line 'port: 7070'");
            assertTrue(content.lines().anyMatch(l -> l.trim().equals("port: 7071")),
                    "File should have a line 'port: 7071'");
            assertTrue(content.contains("enabled: true"), "HTTP should be enabled by default");
        } finally {
            Files.deleteIfExists(configPath);
        }
    }

    @Test
    void writeConfigIncludesScenarios() throws Exception {
        Method m = MocklyServer.class.getDeclaredMethod("writeConfig", int.class, int.class, List.class);
        m.setAccessible(true);
        ScenarioPatch patch = ScenarioPatch.builder("mock-1").status(404).delay("100ms").build();
        Scenario scenario = Scenario.builder("sc-1", "Error Scenario").patch(patch).build();
        Path configPath = (Path) m.invoke(null, 6060, 6061, List.of(scenario));
        try {
            String content = Files.readString(configPath);
            assertTrue(content.contains("scenarios:"), "Config should have scenarios section");
            assertTrue(content.contains("sc-1"), "Config should include scenario id");
            assertTrue(content.contains("Error Scenario"), "Config should include scenario name");
            assertTrue(content.contains("mock-1"), "Config should include patch mock_id");
            assertTrue(content.contains("status: 404"), "Config should include patch status");
            assertTrue(content.contains("delay:"), "Config should include patch delay");
        } finally {
            Files.deleteIfExists(configPath);
        }
    }

    // -------------------------------------------------------------------------
    // HTTP API methods — via embedded HttpServer
    // -------------------------------------------------------------------------

    /** Builds a MocklyServer wired to a fake API server on the given port. */
    private static MocklyServer createTestServer(int apiPort) throws Exception {
        // Start a trivial process that exits immediately so stop() is a no-op.
        ProcessBuilder pb = MocklyInstaller.isWindows()
                ? new ProcessBuilder("cmd", "/c", "echo", "done")
                : new ProcessBuilder("sh", "-c", "echo done");
        pb.redirectErrorStream(true);
        Process dummyProcess = pb.start();
        dummyProcess.waitFor();

        Constructor<MocklyServer> ctor = MocklyServer.class.getDeclaredConstructor(
                Process.class, Path.class, int.class, int.class);
        ctor.setAccessible(true);
        // httpPort=8080 is unused by the HTTP-method tests; apiPort controls apiBase.
        return ctor.newInstance(dummyProcess, null, 8080, apiPort);
    }

    /** Starts a single-handler HttpServer that always responds with the given status. */
    private static HttpServer startFakeServer(int status,
                                              List<String> capturedPaths,
                                              List<String> capturedMethods) throws IOException {
        HttpServer server = HttpServer.create(new InetSocketAddress(0), 0);
        server.createContext("/", exchange -> {
            capturedPaths.add(exchange.getRequestURI().getPath());
            capturedMethods.add(exchange.getRequestMethod());
            exchange.getRequestBody().readAllBytes();
            exchange.sendResponseHeaders(status, -1);
            exchange.close();
        });
        server.start();
        return server;
    }

    private static final class FakeServerResponse {
        private final int status;
        private final String body;

        private FakeServerResponse(int status, String body) {
            this.status = status;
            this.body = body;
        }
    }

    private static HttpServer startFakeJsonServer(FakeServerResponse response,
                                                  List<Map<String, String>> captured) throws IOException {
        HttpServer server = HttpServer.create(new InetSocketAddress(0), 0);
        server.createContext("/", exchange -> {
            String requestBody = new String(exchange.getRequestBody().readAllBytes(), StandardCharsets.UTF_8);
            String rawPath = exchange.getRequestURI().getRawPath();
            String rawQuery = exchange.getRequestURI().getRawQuery();
            Map<String, String> request = new HashMap<>();
            request.put("method", exchange.getRequestMethod());
            request.put("path", exchange.getRequestURI().getPath());
            request.put("pathAndQuery", rawQuery == null ? rawPath : rawPath + "?" + rawQuery);
            request.put("body", requestBody);
            captured.add(request);
            byte[] bodyBytes = response.body.getBytes(StandardCharsets.UTF_8);
            exchange.getResponseHeaders().add("Content-Type", "application/json");
            exchange.sendResponseHeaders(response.status, bodyBytes.length);
            exchange.getResponseBody().write(bodyBytes);
            exchange.close();
        });
        server.start();
        return server;
    }

    private static final String Q = "\"";

    private static String js(String key, String value) {
        return Q + key + Q + ":" + Q + value + Q;
    }

    private static String jn(String key, long value) {
        return Q + key + Q + ":" + value;
    }

    private static String jb(String key, boolean value) {
        return Q + key + Q + ":" + value;
    }

    private static String jo(String key, String value) {
        return Q + key + Q + ":" + value;
    }

    private static String obj(String... fields) {
        return "{" + String.join(",", fields) + "}";
    }

    private static String arr(String... values) {
        return "[" + String.join(",", values) + "]";
    }

    private static Mock sampleMock() {
        return Mock.builder("m1", MockRequest.builder("GET", "/ping").build(), MockResponse.builder(200).body("ok").build()).build();
    }

    private static Scenario sampleScenario(String name) {
        return Scenario.builder("s1", name).build();
    }

    private static String errorJson(String message) {
        return obj(js("error", message));
    }

    private static String sampleCallEntryJson() {
        return obj(
                js("id", "c1"),
                js("timestamp", "2026-01-01T00:00:00Z"),
                js("protocol", "http"),
                js("method", "GET"),
                js("path", "/ping"),
                jn("status", 200),
                jn("duration_ms", 5),
                js("matched_id", "m1")
        );
    }

    private static String mockJson(String id, int status, String body) {
        String response = body == null ? obj(jn("status", status)) : obj(jn("status", status), js("body", body));
        return obj(
                js("id", id),
                jo("request", obj(js("method", "GET"), js("path", "/ping"))),
                jo("response", response)
        );
    }

    private static String scenarioJson(String name) {
        return obj(js("id", "s1"), js("name", name), jo("patches", arr()));
    }

    private static String activeScenariosJson() {
        return obj(jo("active", arr(Q + "s1" + Q)), jo("scenarios", arr(scenarioJson("Test"))));
    }

    private static String callSummaryJson() {
        return obj(js("mock_id", "m1"), jn("count", 2), jo("calls", arr(sampleCallEntryJson())));
    }

    @Test
    void addMockSendsPostToCorrectEndpoint() throws Exception {
        List<String> paths = new ArrayList<>();
        List<String> methods = new ArrayList<>();
        HttpServer fakeServer = startFakeServer(201, paths, methods);
        try {
            MocklyServer server = createTestServer(fakeServer.getAddress().getPort());
            Mock mock = Mock.builder("t",
                    MockRequest.builder("GET", "/p").build(),
                    MockResponse.builder(200).build()).build();
            server.addMock(mock);
            assertEquals("/api/mocks/http", paths.get(0));
            assertEquals("POST", methods.get(0));
        } finally {
            fakeServer.stop(0);
        }
    }

    @Test
    void addMockThrowsOnErrorResponse() throws Exception {
        List<String> paths = new ArrayList<>();
        List<String> methods = new ArrayList<>();
        HttpServer fakeServer = startFakeServer(500, paths, methods);
        try {
            MocklyServer server = createTestServer(fakeServer.getAddress().getPort());
            Mock mock = Mock.builder("t",
                    MockRequest.builder("GET", "/p").build(),
                    MockResponse.builder(200).build()).build();
            assertThrows(IOException.class, () -> server.addMock(mock));
        } finally {
            fakeServer.stop(0);
        }
    }

    @Test
    void deleteMockSendsDeleteToCorrectEndpoint() throws Exception {
        List<String> paths = new ArrayList<>();
        List<String> methods = new ArrayList<>();
        HttpServer fakeServer = startFakeServer(200, paths, methods);
        try {
            MocklyServer server = createTestServer(fakeServer.getAddress().getPort());
            server.deleteMock("my-id");
            assertEquals("/api/mocks/http/my-id", paths.get(0));
            assertEquals("DELETE", methods.get(0));
        } finally {
            fakeServer.stop(0);
        }
    }

    @Test
    void resetSendsPostToCorrectEndpoint() throws Exception {
        List<String> paths = new ArrayList<>();
        List<String> methods = new ArrayList<>();
        HttpServer fakeServer = startFakeServer(200, paths, methods);
        try {
            MocklyServer server = createTestServer(fakeServer.getAddress().getPort());
            server.reset();
            assertEquals("/api/reset", paths.get(0));
            assertEquals("POST", methods.get(0));
        } finally {
            fakeServer.stop(0);
        }
    }

    @Test
    void activateScenarioSendsPostToCorrectEndpoint() throws Exception {
        List<String> paths = new ArrayList<>();
        List<String> methods = new ArrayList<>();
        HttpServer fakeServer = startFakeServer(200, paths, methods);
        try {
            MocklyServer server = createTestServer(fakeServer.getAddress().getPort());
            server.activateScenario("sc1");
            assertEquals("/api/scenarios/sc1/activate", paths.get(0));
            assertEquals("POST", methods.get(0));
        } finally {
            fakeServer.stop(0);
        }
    }

    @Test
    void deactivateScenarioSendsDeleteToCorrectEndpoint() throws Exception {
        List<String> paths = new ArrayList<>();
        List<String> methods = new ArrayList<>();
        HttpServer fakeServer = startFakeServer(200, paths, methods);
        try {
            MocklyServer server = createTestServer(fakeServer.getAddress().getPort());
            server.deactivateScenario("sc1");
            assertEquals("/api/scenarios/sc1/activate", paths.get(0));
            assertEquals("DELETE", methods.get(0));
        } finally {
            fakeServer.stop(0);
        }
    }

    @Test
    void setFaultSendsPostToCorrectEndpoint() throws Exception {
        List<String> paths = new ArrayList<>();
        List<String> methods = new ArrayList<>();
        HttpServer fakeServer = startFakeServer(200, paths, methods);
        try {
            MocklyServer server = createTestServer(fakeServer.getAddress().getPort());
            FaultConfig fault = FaultConfig.builder(true).delay("100ms").build();
            server.setFault(fault);
            assertEquals("/api/fault/http", paths.get(0));
        } finally {
            fakeServer.stop(0);
        }
    }

    @Test
    void clearFaultSendsDeleteToCorrectEndpoint() throws Exception {
        List<String> paths = new ArrayList<>();
        List<String> methods = new ArrayList<>();
        HttpServer fakeServer = startFakeServer(204, paths, methods);
        try {
            MocklyServer server = createTestServer(fakeServer.getAddress().getPort());
            server.clearFault();
            assertEquals("/api/fault", paths.get(0));
            assertEquals("DELETE", methods.get(0));
        } finally {
            fakeServer.stop(0);
        }
    }

    @Test
    void listMocksSendsGetAndParsesResponse() throws Exception {
        List<Map<String, String>> captured = new ArrayList<>();
        HttpServer fakeServer = startFakeJsonServer(new FakeServerResponse(200, arr(mockJson("m1", 200, "ok"))), captured);
        try {
            MocklyServer server = createTestServer(fakeServer.getAddress().getPort());
            List<Mock> mocks = server.listMocks();
            assertEquals("GET", captured.get(0).get("method"));
            assertEquals("/api/mocks/http", captured.get(0).get("path"));
            assertEquals("m1", mocks.get(0).getId());
        } finally {
            fakeServer.stop(0);
        }
    }

    @Test
    void listMocksThrowsOnErrorResponse() throws Exception {
        List<Map<String, String>> captured = new ArrayList<>();
        HttpServer fakeServer = startFakeJsonServer(new FakeServerResponse(500, errorJson("boom")), captured);
        try {
            MocklyServer server = createTestServer(fakeServer.getAddress().getPort());
            assertThrows(IOException.class, server::listMocks);
        } finally {
            fakeServer.stop(0);
        }
    }

    @Test
    void updateMockSendsPutAndParsesResponse() throws Exception {
        List<Map<String, String>> captured = new ArrayList<>();
        HttpServer fakeServer = startFakeJsonServer(new FakeServerResponse(200, mockJson("m1", 201, "updated")), captured);
        try {
            MocklyServer server = createTestServer(fakeServer.getAddress().getPort());
            Mock result = server.updateMock("m1", sampleMock());
            assertEquals("PUT", captured.get(0).get("method"));
            assertEquals("/api/mocks/http/m1", captured.get(0).get("path"));
            assertTrue(captured.get(0).get("body").contains("\"id\":\"m1\""));
            assertEquals(201, result.getResponse().getStatus());
        } finally {
            fakeServer.stop(0);
        }
    }

    @Test
    void updateMockThrowsOnErrorResponse() throws Exception {
        List<Map<String, String>> captured = new ArrayList<>();
        HttpServer fakeServer = startFakeJsonServer(new FakeServerResponse(500, errorJson("boom")), captured);
        try {
            MocklyServer server = createTestServer(fakeServer.getAddress().getPort());
            assertThrows(IOException.class, () -> server.updateMock("m1", sampleMock()));
        } finally {
            fakeServer.stop(0);
        }
    }

    @Test
    void patchMockSendsPatchAndParsesResponse() throws Exception {
        List<Map<String, String>> captured = new ArrayList<>();
        HttpServer fakeServer = startFakeJsonServer(new FakeServerResponse(200, mockJson("m1", 201, "patched")), captured);
        try {
            MocklyServer server = createTestServer(fakeServer.getAddress().getPort());
            Mock result = server.patchMock("m1", MockResponsePatch.builder().status(201).body("patched").build());
            assertEquals("PATCH", captured.get(0).get("method"));
            assertEquals("/api/mocks/http/m1", captured.get(0).get("path"));
            assertTrue(captured.get(0).get("body").contains("\"status\":201"));
            assertEquals("patched", result.getResponse().getBody());
        } finally {
            fakeServer.stop(0);
        }
    }

    @Test
    void patchMockThrowsOnErrorResponse() throws Exception {
        List<Map<String, String>> captured = new ArrayList<>();
        HttpServer fakeServer = startFakeJsonServer(new FakeServerResponse(500, errorJson("boom")), captured);
        try {
            MocklyServer server = createTestServer(fakeServer.getAddress().getPort());
            assertThrows(IOException.class, () -> server.patchMock("m1", MockResponsePatch.builder().status(201).build()));
        } finally {
            fakeServer.stop(0);
        }
    }

    @Test
    void getStateSendsGetAndParsesResponse() throws Exception {
        List<Map<String, String>> captured = new ArrayList<>();
        HttpServer fakeServer = startFakeJsonServer(new FakeServerResponse(200, obj(js("key1", "val1"))), captured);
        try {
            MocklyServer server = createTestServer(fakeServer.getAddress().getPort());
            Map<String, String> state = server.getState();
            assertEquals("GET", captured.get(0).get("method"));
            assertEquals("/api/state", captured.get(0).get("path"));
            assertEquals("val1", state.get("key1"));
        } finally {
            fakeServer.stop(0);
        }
    }

    @Test
    void getStateThrowsOnErrorResponse() throws Exception {
        List<Map<String, String>> captured = new ArrayList<>();
        HttpServer fakeServer = startFakeJsonServer(new FakeServerResponse(500, errorJson("boom")), captured);
        try {
            MocklyServer server = createTestServer(fakeServer.getAddress().getPort());
            assertThrows(IOException.class, server::getState);
        } finally {
            fakeServer.stop(0);
        }
    }

    @Test
    void setStateSendsPostAndParsesResponse() throws Exception {
        List<Map<String, String>> captured = new ArrayList<>();
        HttpServer fakeServer = startFakeJsonServer(new FakeServerResponse(200, obj(js("key1", "val1"))), captured);
        try {
            MocklyServer server = createTestServer(fakeServer.getAddress().getPort());
            Map<String, String> state = server.setState(Map.of("key1", "val1"));
            assertEquals("POST", captured.get(0).get("method"));
            assertEquals("/api/state", captured.get(0).get("path"));
            assertTrue(captured.get(0).get("body").contains("key1"));
            assertEquals("val1", state.get("key1"));
        } finally {
            fakeServer.stop(0);
        }
    }

    @Test
    void setStateThrowsOnErrorResponse() throws Exception {
        List<Map<String, String>> captured = new ArrayList<>();
        HttpServer fakeServer = startFakeJsonServer(new FakeServerResponse(500, errorJson("boom")), captured);
        try {
            MocklyServer server = createTestServer(fakeServer.getAddress().getPort());
            assertThrows(IOException.class, () -> server.setState(Map.of("key1", "val1")));
        } finally {
            fakeServer.stop(0);
        }
    }

    @Test
    void deleteStateSendsDelete() throws Exception {
        List<Map<String, String>> captured = new ArrayList<>();
        HttpServer fakeServer = startFakeJsonServer(new FakeServerResponse(200, ""), captured);
        try {
            MocklyServer server = createTestServer(fakeServer.getAddress().getPort());
            server.deleteState("key1");
            assertEquals("DELETE", captured.get(0).get("method"));
            assertEquals("/api/state/key1", captured.get(0).get("path"));
        } finally {
            fakeServer.stop(0);
        }
    }

    @Test
    void deleteStateThrowsOnErrorResponse() throws Exception {
        List<Map<String, String>> captured = new ArrayList<>();
        HttpServer fakeServer = startFakeJsonServer(new FakeServerResponse(500, errorJson("boom")), captured);
        try {
            MocklyServer server = createTestServer(fakeServer.getAddress().getPort());
            assertThrows(IOException.class, () -> server.deleteState("key1"));
        } finally {
            fakeServer.stop(0);
        }
    }

    @Test
    void getLogsSendsGetAndParsesResponse() throws Exception {
        List<Map<String, String>> captured = new ArrayList<>();
        HttpServer fakeServer = startFakeJsonServer(new FakeServerResponse(200, arr(sampleCallEntryJson())), captured);
        try {
            MocklyServer server = createTestServer(fakeServer.getAddress().getPort());
            List<io.mockly.driver.model.CallEntry> logs = server.getLogs("m1");
            assertEquals("GET", captured.get(0).get("method"));
            assertEquals("/api/logs?matched_id=m1", captured.get(0).get("pathAndQuery"));
            assertEquals("m1", logs.get(0).getMatchedId());
        } finally {
            fakeServer.stop(0);
        }
    }

    @Test
    void getLogsThrowsOnErrorResponse() throws Exception {
        List<Map<String, String>> captured = new ArrayList<>();
        HttpServer fakeServer = startFakeJsonServer(new FakeServerResponse(500, errorJson("boom")), captured);
        try {
            MocklyServer server = createTestServer(fakeServer.getAddress().getPort());
            assertThrows(IOException.class, server::getLogs);
        } finally {
            fakeServer.stop(0);
        }
    }

    @Test
    void clearLogsSendsDelete() throws Exception {
        List<Map<String, String>> captured = new ArrayList<>();
        HttpServer fakeServer = startFakeJsonServer(new FakeServerResponse(200, ""), captured);
        try {
            MocklyServer server = createTestServer(fakeServer.getAddress().getPort());
            server.clearLogs();
            assertEquals("DELETE", captured.get(0).get("method"));
            assertEquals("/api/logs", captured.get(0).get("path"));
        } finally {
            fakeServer.stop(0);
        }
    }

    @Test
    void clearLogsThrowsOnErrorResponse() throws Exception {
        List<Map<String, String>> captured = new ArrayList<>();
        HttpServer fakeServer = startFakeJsonServer(new FakeServerResponse(500, errorJson("boom")), captured);
        try {
            MocklyServer server = createTestServer(fakeServer.getAddress().getPort());
            assertThrows(IOException.class, server::clearLogs);
        } finally {
            fakeServer.stop(0);
        }
    }

    @Test
    void getLogsCountSendsGetAndParsesResponse() throws Exception {
        List<Map<String, String>> captured = new ArrayList<>();
        HttpServer fakeServer = startFakeJsonServer(new FakeServerResponse(200, obj(jn("count", 5))), captured);
        try {
            MocklyServer server = createTestServer(fakeServer.getAddress().getPort());
            int count = server.getLogsCount("m1");
            assertEquals("GET", captured.get(0).get("method"));
            assertEquals("/api/logs/count?matched_id=m1", captured.get(0).get("pathAndQuery"));
            assertEquals(5, count);
        } finally {
            fakeServer.stop(0);
        }
    }

    @Test
    void getLogsCountThrowsOnErrorResponse() throws Exception {
        List<Map<String, String>> captured = new ArrayList<>();
        HttpServer fakeServer = startFakeJsonServer(new FakeServerResponse(500, errorJson("boom")), captured);
        try {
            MocklyServer server = createTestServer(fakeServer.getAddress().getPort());
            assertThrows(IOException.class, server::getLogsCount);
        } finally {
            fakeServer.stop(0);
        }
    }

    @Test
    void listScenariosSendsGetAndParsesResponse() throws Exception {
        List<Map<String, String>> captured = new ArrayList<>();
        HttpServer fakeServer = startFakeJsonServer(new FakeServerResponse(200, arr(scenarioJson("Test"))), captured);
        try {
            MocklyServer server = createTestServer(fakeServer.getAddress().getPort());
            List<Scenario> scenarios = server.listScenarios();
            assertEquals("GET", captured.get(0).get("method"));
            assertEquals("/api/scenarios", captured.get(0).get("path"));
            assertEquals("s1", scenarios.get(0).getId());
        } finally {
            fakeServer.stop(0);
        }
    }

    @Test
    void listScenariosThrowsOnErrorResponse() throws Exception {
        List<Map<String, String>> captured = new ArrayList<>();
        HttpServer fakeServer = startFakeJsonServer(new FakeServerResponse(500, errorJson("boom")), captured);
        try {
            MocklyServer server = createTestServer(fakeServer.getAddress().getPort());
            assertThrows(IOException.class, server::listScenarios);
        } finally {
            fakeServer.stop(0);
        }
    }

    @Test
    void createScenarioSendsPostAndParsesResponse() throws Exception {
        List<Map<String, String>> captured = new ArrayList<>();
        HttpServer fakeServer = startFakeJsonServer(new FakeServerResponse(201, scenarioJson("Test")), captured);
        try {
            MocklyServer server = createTestServer(fakeServer.getAddress().getPort());
            Scenario scenario = server.createScenario(sampleScenario("Test"));
            assertEquals("POST", captured.get(0).get("method"));
            assertEquals("/api/scenarios", captured.get(0).get("path"));
            assertTrue(captured.get(0).get("body").contains("\"id\":\"s1\""));
            assertEquals("Test", scenario.getName());
        } finally {
            fakeServer.stop(0);
        }
    }

    @Test
    void createScenarioThrowsOnErrorResponse() throws Exception {
        List<Map<String, String>> captured = new ArrayList<>();
        HttpServer fakeServer = startFakeJsonServer(new FakeServerResponse(500, errorJson("boom")), captured);
        try {
            MocklyServer server = createTestServer(fakeServer.getAddress().getPort());
            assertThrows(IOException.class, () -> server.createScenario(sampleScenario("Test")));
        } finally {
            fakeServer.stop(0);
        }
    }

    @Test
    void getScenarioSendsGetAndParsesResponse() throws Exception {
        List<Map<String, String>> captured = new ArrayList<>();
        HttpServer fakeServer = startFakeJsonServer(new FakeServerResponse(200, scenarioJson("Test")), captured);
        try {
            MocklyServer server = createTestServer(fakeServer.getAddress().getPort());
            Scenario scenario = server.getScenario("s1");
            assertEquals("GET", captured.get(0).get("method"));
            assertEquals("/api/scenarios/s1", captured.get(0).get("path"));
            assertEquals("s1", scenario.getId());
        } finally {
            fakeServer.stop(0);
        }
    }

    @Test
    void getScenarioThrowsOnErrorResponse() throws Exception {
        List<Map<String, String>> captured = new ArrayList<>();
        HttpServer fakeServer = startFakeJsonServer(new FakeServerResponse(500, errorJson("boom")), captured);
        try {
            MocklyServer server = createTestServer(fakeServer.getAddress().getPort());
            assertThrows(IOException.class, () -> server.getScenario("s1"));
        } finally {
            fakeServer.stop(0);
        }
    }

    @Test
    void updateScenarioSendsPutAndParsesResponse() throws Exception {
        List<Map<String, String>> captured = new ArrayList<>();
        HttpServer fakeServer = startFakeJsonServer(new FakeServerResponse(200, scenarioJson("Updated")), captured);
        try {
            MocklyServer server = createTestServer(fakeServer.getAddress().getPort());
            Scenario scenario = server.updateScenario("s1", sampleScenario("Updated"));
            assertEquals("PUT", captured.get(0).get("method"));
            assertEquals("/api/scenarios/s1", captured.get(0).get("path"));
            assertTrue(captured.get(0).get("body").contains("Updated"));
            assertEquals("Updated", scenario.getName());
        } finally {
            fakeServer.stop(0);
        }
    }

    @Test
    void updateScenarioThrowsOnErrorResponse() throws Exception {
        List<Map<String, String>> captured = new ArrayList<>();
        HttpServer fakeServer = startFakeJsonServer(new FakeServerResponse(500, errorJson("boom")), captured);
        try {
            MocklyServer server = createTestServer(fakeServer.getAddress().getPort());
            assertThrows(IOException.class, () -> server.updateScenario("s1", sampleScenario("Updated")));
        } finally {
            fakeServer.stop(0);
        }
    }

    @Test
    void deleteScenarioSendsDelete() throws Exception {
        List<Map<String, String>> captured = new ArrayList<>();
        HttpServer fakeServer = startFakeJsonServer(new FakeServerResponse(200, ""), captured);
        try {
            MocklyServer server = createTestServer(fakeServer.getAddress().getPort());
            server.deleteScenario("s1");
            assertEquals("DELETE", captured.get(0).get("method"));
            assertEquals("/api/scenarios/s1", captured.get(0).get("path"));
        } finally {
            fakeServer.stop(0);
        }
    }

    @Test
    void deleteScenarioThrowsOnErrorResponse() throws Exception {
        List<Map<String, String>> captured = new ArrayList<>();
        HttpServer fakeServer = startFakeJsonServer(new FakeServerResponse(500, errorJson("boom")), captured);
        try {
            MocklyServer server = createTestServer(fakeServer.getAddress().getPort());
            assertThrows(IOException.class, () -> server.deleteScenario("s1"));
        } finally {
            fakeServer.stop(0);
        }
    }

    @Test
    void listActiveScenariosSendsGetAndParsesResponse() throws Exception {
        List<Map<String, String>> captured = new ArrayList<>();
        HttpServer fakeServer = startFakeJsonServer(new FakeServerResponse(200, activeScenariosJson()), captured);
        try {
            MocklyServer server = createTestServer(fakeServer.getAddress().getPort());
            io.mockly.driver.model.ActiveScenariosResponse response = server.listActiveScenarios();
            assertEquals("GET", captured.get(0).get("method"));
            assertEquals("/api/scenarios/active", captured.get(0).get("path"));
            assertEquals("s1", response.getActive().get(0));
            assertEquals("s1", response.getScenarios().get(0).getId());
        } finally {
            fakeServer.stop(0);
        }
    }

    @Test
    void listActiveScenariosThrowsOnErrorResponse() throws Exception {
        List<Map<String, String>> captured = new ArrayList<>();
        HttpServer fakeServer = startFakeJsonServer(new FakeServerResponse(500, errorJson("boom")), captured);
        try {
            MocklyServer server = createTestServer(fakeServer.getAddress().getPort());
            assertThrows(IOException.class, server::listActiveScenarios);
        } finally {
            fakeServer.stop(0);
        }
    }

    @Test
    void getCallsSendsGetAndParsesResponse() throws Exception {
        List<Map<String, String>> captured = new ArrayList<>();
        HttpServer fakeServer = startFakeJsonServer(new FakeServerResponse(200, callSummaryJson()), captured);
        try {
            MocklyServer server = createTestServer(fakeServer.getAddress().getPort());
            io.mockly.driver.model.CallSummary summary = server.getCalls("m1");
            assertEquals("GET", captured.get(0).get("method"));
            assertEquals("/api/calls/http/m1", captured.get(0).get("path"));
            assertEquals(2, summary.getCount());
            assertEquals("c1", summary.getCalls().get(0).getId());
        } finally {
            fakeServer.stop(0);
        }
    }

    @Test
    void getCallsThrowsOnErrorResponse() throws Exception {
        List<Map<String, String>> captured = new ArrayList<>();
        HttpServer fakeServer = startFakeJsonServer(new FakeServerResponse(500, errorJson("boom")), captured);
        try {
            MocklyServer server = createTestServer(fakeServer.getAddress().getPort());
            assertThrows(IOException.class, () -> server.getCalls("m1"));
        } finally {
            fakeServer.stop(0);
        }
    }

    @Test
    void clearCallsSendsDelete() throws Exception {
        List<Map<String, String>> captured = new ArrayList<>();
        HttpServer fakeServer = startFakeJsonServer(new FakeServerResponse(200, ""), captured);
        try {
            MocklyServer server = createTestServer(fakeServer.getAddress().getPort());
            server.clearCalls("m1");
            assertEquals("DELETE", captured.get(0).get("method"));
            assertEquals("/api/calls/http/m1", captured.get(0).get("path"));
        } finally {
            fakeServer.stop(0);
        }
    }

    @Test
    void clearCallsThrowsOnErrorResponse() throws Exception {
        List<Map<String, String>> captured = new ArrayList<>();
        HttpServer fakeServer = startFakeJsonServer(new FakeServerResponse(500, errorJson("boom")), captured);
        try {
            MocklyServer server = createTestServer(fakeServer.getAddress().getPort());
            assertThrows(IOException.class, () -> server.clearCalls("m1"));
        } finally {
            fakeServer.stop(0);
        }
    }

    @Test
    void clearAllCallsSendsDelete() throws Exception {
        List<Map<String, String>> captured = new ArrayList<>();
        HttpServer fakeServer = startFakeJsonServer(new FakeServerResponse(200, ""), captured);
        try {
            MocklyServer server = createTestServer(fakeServer.getAddress().getPort());
            server.clearAllCalls();
            assertEquals("DELETE", captured.get(0).get("method"));
            assertEquals("/api/calls/http", captured.get(0).get("path"));
        } finally {
            fakeServer.stop(0);
        }
    }

    @Test
    void clearAllCallsThrowsOnErrorResponse() throws Exception {
        List<Map<String, String>> captured = new ArrayList<>();
        HttpServer fakeServer = startFakeJsonServer(new FakeServerResponse(500, errorJson("boom")), captured);
        try {
            MocklyServer server = createTestServer(fakeServer.getAddress().getPort());
            assertThrows(IOException.class, server::clearAllCalls);
        } finally {
            fakeServer.stop(0);
        }
    }

    @Test
    void waitForCallsSendsPostAndParsesResponse() throws Exception {
        List<Map<String, String>> captured = new ArrayList<>();
        HttpServer fakeServer = startFakeJsonServer(new FakeServerResponse(200, callSummaryJson()), captured);
        try {
            MocklyServer server = createTestServer(fakeServer.getAddress().getPort());
            io.mockly.driver.model.CallSummary summary = server.waitForCalls("m1", 2, Duration.ofSeconds(5));
            assertEquals("POST", captured.get(0).get("method"));
            assertEquals("/api/calls/http/m1/wait", captured.get(0).get("path"));
            assertTrue(captured.get(0).get("body").contains("5000ms"));
            assertEquals(2, summary.getCount());
            assertEquals("c1", summary.getCalls().get(0).getId());
        } finally {
            fakeServer.stop(0);
        }
    }

    @Test
    void waitForCallsThrowsOnErrorResponse() throws Exception {
        List<Map<String, String>> captured = new ArrayList<>();
        HttpServer fakeServer = startFakeJsonServer(new FakeServerResponse(408, errorJson("timeout")), captured);
        try {
            MocklyServer server = createTestServer(fakeServer.getAddress().getPort());
            assertThrows(IOException.class, () -> server.waitForCalls("m1", 2, Duration.ofSeconds(5)));
        } finally {
            fakeServer.stop(0);
        }
    }

    @Test
    void extractStringHandlesPresentMissingAndNullValues() {
        assertEquals("value", MocklyServer.extractString(obj(js("key", "value")), "key"));
        assertNull(MocklyServer.extractString(obj(), "missing"));
        assertNull(MocklyServer.extractString(obj(jo("key", "null")), "key"));
    }

    @Test
    void extractLongHandlesNormalMissingAndNegativeValues() {
        assertEquals(5L, MocklyServer.extractLong(obj(jn("count", 5)), "count"));
        assertEquals(0L, MocklyServer.extractLong(obj(), "count"));
        assertEquals(-2L, MocklyServer.extractLong(obj(jn("count", -2)), "count"));
    }

    @Test
    void parseCallSummaryHandlesCallsArrayAndEmptyCalls() {
        io.mockly.driver.model.CallSummary summary = MocklyServer.parseCallSummary(callSummaryJson());
        assertEquals("m1", summary.getMockId());
        assertEquals(2, summary.getCount());
        assertEquals("c1", summary.getCalls().get(0).getId());

        io.mockly.driver.model.CallSummary empty = MocklyServer.parseCallSummary(obj(js("mock_id", "m1"), jn("count", 0), jo("calls", arr())));
        assertTrue(empty.getCalls().isEmpty());
    }

    @Test
    void parseCallSummaryHandlesEscapedBackslashBeforeClosingQuote() {
        String body = "C\\";
        String encodedBody = body.replace("\\", "\\\\");
        String json = obj(
                js("mock_id", "m1"),
                jn("count", 1),
                jo("calls", arr(obj(
                        js("id", "c1"),
                        js("timestamp", "2026-01-01T00:00:00Z"),
                        js("protocol", "http"),
                        js("method", "GET"),
                        js("path", "/ping"),
                        jn("status", 200),
                        jn("duration_ms", 5),
                        jo("headers", obj()),
                        jo("body", Q + encodedBody + Q),
                        js("matched_id", "m1"),
                        jo("path_params", obj())
                )))
        );
        io.mockly.driver.model.CallSummary summary = MocklyServer.parseCallSummary(json);
        assertEquals(body, summary.getCalls().get(0).getBody());
    }

}
