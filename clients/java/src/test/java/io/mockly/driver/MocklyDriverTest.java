package io.mockly.driver;

import io.mockly.driver.model.FaultConfig;
import io.mockly.driver.model.Mock;
import io.mockly.driver.model.MockRequest;
import io.mockly.driver.model.MockResponse;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;

import java.io.IOException;
import java.net.ServerSocket;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.HashMap;
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
        assertTrue(port > 0 && port <= 65535, "Port should be in valid range, got: " + port);
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
        FaultConfig fault = FaultConfig.builder("delay")
                .delay("200ms")
                .probability(0.5)
                .build();

        String json = MocklyServer.toJson(fault);
        assertTrue(json.contains("\"type\":\"delay\""));
        assertTrue(json.contains("\"delay\":\"200ms\""));
        assertTrue(json.contains("\"probability\":0.5"));
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
    }

    @Test
    void mocklyConfigBuilderSetsValues() {
        MocklyConfig cfg = MocklyConfig.builder()
                .httpPort(9000)
                .apiPort(9001)
                .version("v1.2.3")
                .binDir("custom-bin")
                .startupTimeoutMs(5000)
                .build();

        assertEquals(9000, cfg.getHttpPort());
        assertEquals(9001, cfg.getApiPort());
        assertEquals("v1.2.3", cfg.getVersion());
        assertEquals("custom-bin", cfg.getBinDir());
        assertEquals(5000, cfg.getStartupTimeoutMs());
    }
}
