package io.mockly.driver;

import com.sun.net.httpserver.HttpServer;
import io.mockly.driver.model.FaultConfig;
import io.mockly.driver.model.Mock;
import io.mockly.driver.model.MockRequest;
import io.mockly.driver.model.MockResponse;
import io.mockly.driver.model.Scenario;
import io.mockly.driver.model.ScenarioPatch;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;

import java.io.IOException;
import java.lang.reflect.Constructor;
import java.lang.reflect.Method;
import java.net.InetSocketAddress;
import java.net.ServerSocket;
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
            // Drain the request body before responding to avoid connection-reset errors
            // when the client is still sending a body (e.g. JSON for setFault/addMock).
            exchange.getRequestBody().readAllBytes();
            exchange.sendResponseHeaders(status, -1);
            exchange.close();
        });
        server.start();
        return server;
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
        HttpServer fakeServer = startFakeServer(204, paths, methods);
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
            assertEquals("/api/fault", paths.get(0));
            assertEquals("POST", methods.get(0));
        } finally {
            fakeServer.stop(0);
        }
    }

    @Test
    void clearFaultSendsDeleteToCorrectEndpoint() throws Exception {
        List<String> paths = new ArrayList<>();
        List<String> methods = new ArrayList<>();
        HttpServer fakeServer = startFakeServer(200, paths, methods);
        try {
            MocklyServer server = createTestServer(fakeServer.getAddress().getPort());
            server.clearFault();
            assertEquals("/api/fault", paths.get(0));
            assertEquals("DELETE", methods.get(0));
        } finally {
            fakeServer.stop(0);
        }
    }
}
