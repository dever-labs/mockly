package io.mockly.driver;

import java.io.IOException;
import java.io.InputStream;
import java.net.InetSocketAddress;
import java.net.ProxySelector;
import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.Paths;
import java.nio.file.StandardCopyOption;
import java.nio.file.attribute.PosixFilePermission;
import java.time.Duration;
import java.util.EnumSet;
import java.util.Map;
import java.util.Set;

/**
 * Downloads and locates the Mockly binary.
 *
 * <h2>Proxy support</h2>
 * {@code HttpClient} uses {@link ProxySelector#getDefault()}, which on most JVMs picks up
 * the {@code https.proxyHost} / {@code https.proxyPort} system properties.
 * Set them via {@code -Dhttps.proxyHost=proxy.example.com -Dhttps.proxyPort=8080},
 * or export {@code HTTPS_PROXY} before starting the JVM on systems where the JVM
 * inherits the environment-based proxy selector.
 */
public class MocklyInstaller {

    public static final String DEFAULT_VERSION = "v0.1.0";

    private static final String DEFAULT_DOWNLOAD_BASE =
            "https://github.com/dever-labs/mockly/releases/download";

    // -------------------------------------------------------------------------
    // Public API
    // -------------------------------------------------------------------------

    /**
     * Returns the path to an installed binary, or {@code null} if none is found.
     * Search order:
     * <ol>
     *   <li>{@code MOCKLY_BINARY_PATH} environment variable</li>
     *   <li>{@code <binDir>/mockly[.exe]}</li>
     *   <li>{@code ./bin/mockly[.exe]}</li>
     * </ol>
     */
    public static String getBinaryPath(String binDir) {
        return getBinaryPath(binDir, System.getenv());
    }

    /** Package-private overload for testing (accepts an explicit env map). */
    static String getBinaryPath(String binDir, Map<String, String> env) {
        String fromEnv = env.get("MOCKLY_BINARY_PATH");
        if (fromEnv != null && Files.isExecutable(Paths.get(fromEnv))) {
            return fromEnv;
        }

        String exeName = isWindows() ? "mockly.exe" : "mockly";

        if (binDir != null) {
            Path candidate = Paths.get(binDir, exeName);
            if (Files.isExecutable(candidate)) {
                return candidate.toAbsolutePath().toString();
            }
        }

        Path fallback = Paths.get("bin", exeName);
        if (Files.isExecutable(fallback)) {
            return fallback.toAbsolutePath().toString();
        }

        return null;
    }

    /**
     * Ensures the binary is available. Returns its path.
     * Throws if {@code MOCKLY_NO_INSTALL} is set and the binary is not already staged.
     */
    public static String install(InstallOptions opts) throws IOException {
        return install(opts, System.getenv());
    }

    /** Package-private overload for testing. */
    static String install(InstallOptions opts, Map<String, String> env) throws IOException {
        // 1. Honour an explicit staged binary.
        String fromEnv = env.get("MOCKLY_BINARY_PATH");
        if (fromEnv != null) {
            Path p = Paths.get(fromEnv);
            if (Files.exists(p)) {
                return p.toAbsolutePath().toString();
            }
            throw new IOException("MOCKLY_BINARY_PATH is set to '" + fromEnv
                    + "' but the file does not exist.");
        }

        // 2. Already installed?
        String existing = getBinaryPath(opts.getBinDir(), env);
        if (existing != null) {
            return existing;
        }

        // 3. MOCKLY_NO_INSTALL guard.
        if (env.containsKey("MOCKLY_NO_INSTALL")) {
            throw new IOException(
                    "Mockly binary not found and MOCKLY_NO_INSTALL is set. "
                    + "Pre-stage the binary or set MOCKLY_BINARY_PATH.");
        }

        // 4. Determine version and download URL.
        String version = env.getOrDefault("MOCKLY_VERSION",
                opts.getVersion() != null ? opts.getVersion() : DEFAULT_VERSION);
        String baseUrl = env.getOrDefault("MOCKLY_DOWNLOAD_BASE_URL", DEFAULT_DOWNLOAD_BASE);

        String asset = getAssetName();
        String url = baseUrl + "/" + version + "/" + asset;

        // 5. Download.
        Path binDir = Paths.get(opts.getBinDir());
        Files.createDirectories(binDir);

        String exeName = isWindows() ? "mockly.exe" : "mockly";
        Path dest = binDir.resolve(exeName);

        downloadFile(url, dest);

        // 6. Make executable on POSIX systems.
        if (!isWindows()) {
            try {
                Set<PosixFilePermission> perms = EnumSet.of(
                        PosixFilePermission.OWNER_READ,
                        PosixFilePermission.OWNER_WRITE,
                        PosixFilePermission.OWNER_EXECUTE,
                        PosixFilePermission.GROUP_READ,
                        PosixFilePermission.GROUP_EXECUTE,
                        PosixFilePermission.OTHERS_READ,
                        PosixFilePermission.OTHERS_EXECUTE
                );
                Files.setPosixFilePermissions(dest, perms);
            } catch (UnsupportedOperationException ignored) {
                // Non-POSIX filesystem; skip.
            }
        }

        return dest.toAbsolutePath().toString();
    }

    // -------------------------------------------------------------------------
    // Internal helpers
    // -------------------------------------------------------------------------

    static String getAssetName() {
        String os = System.getProperty("os.name", "").toLowerCase();
        String arch = System.getProperty("os.arch", "").toLowerCase();

        boolean isWindows = os.contains("win");
        boolean isMac = os.contains("mac") || os.contains("darwin");
        boolean isArm = arch.contains("aarch64") || arch.contains("arm");

        if (isWindows) {
            return "mockly-windows-amd64.exe";
        } else if (isMac) {
            return isArm ? "mockly-darwin-arm64" : "mockly-darwin-amd64";
        } else {
            // Linux or unknown — default to Linux.
            return isArm ? "mockly-linux-arm64" : "mockly-linux-amd64";
        }
    }

    private static void downloadFile(String url, Path dest) throws IOException {
        HttpClient client = HttpClient.newBuilder()
                .proxy(ProxySelector.getDefault())
                .followRedirects(HttpClient.Redirect.ALWAYS)
                .connectTimeout(Duration.ofSeconds(30))
                .build();

        HttpRequest request = HttpRequest.newBuilder()
                .uri(URI.create(url))
                .GET()
                .timeout(Duration.ofMinutes(5))
                .build();

        try {
            HttpResponse<InputStream> response =
                    client.send(request, HttpResponse.BodyHandlers.ofInputStream());

            if (response.statusCode() < 200 || response.statusCode() >= 300) {
                throw new IOException("Failed to download Mockly binary from " + url
                        + " — HTTP " + response.statusCode());
            }

            try (InputStream in = response.body()) {
                Files.copy(in, dest, StandardCopyOption.REPLACE_EXISTING);
            }
        } catch (InterruptedException e) {
            Thread.currentThread().interrupt();
            throw new IOException("Download interrupted", e);
        }
    }

    static boolean isWindows() {
        return System.getProperty("os.name", "").toLowerCase().contains("win");
    }

    // -------------------------------------------------------------------------
    // InstallOptions
    // -------------------------------------------------------------------------

    public static class InstallOptions {
        private final String binDir;
        private final String version;

        private InstallOptions(Builder builder) {
            this.binDir = builder.binDir;
            this.version = builder.version;
        }

        public String getBinDir() { return binDir; }
        public String getVersion() { return version; }

        public static Builder builder() { return new Builder(); }

        public static class Builder {
            private String binDir = "bin";
            private String version = DEFAULT_VERSION;

            public Builder binDir(String binDir) { this.binDir = binDir; return this; }
            public Builder version(String version) { this.version = version; return this; }

            public InstallOptions build() { return new InstallOptions(this); }
        }
    }
}
