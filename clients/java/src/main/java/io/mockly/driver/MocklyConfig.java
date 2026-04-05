package io.mockly.driver;

import java.util.ArrayList;
import java.util.List;

/**
 * Configuration for starting a MocklyServer instance.
 */
public class MocklyConfig {
    private final String binaryPath;
    private final int httpPort;
    private final int apiPort;
    private final String version;
    private final String binDir;
    private final int startupTimeoutMs;
    private final List<String> extraArgs;

    private MocklyConfig(Builder builder) {
        this.binaryPath = builder.binaryPath;
        this.httpPort = builder.httpPort;
        this.apiPort = builder.apiPort;
        this.version = builder.version;
        this.binDir = builder.binDir;
        this.startupTimeoutMs = builder.startupTimeoutMs;
        this.extraArgs = builder.extraArgs;
    }

    /** Explicit path to the mockly binary. If null, auto-resolved. */
    public String getBinaryPath() { return binaryPath; }
    /** HTTP mock port. 0 = pick a free port. */
    public int getHttpPort() { return httpPort; }
    /** Management API port. 0 = pick a free port. */
    public int getApiPort() { return apiPort; }
    /** Mockly version to download if not present. */
    public String getVersion() { return version; }
    /** Directory to search for / install the binary. Defaults to ./bin. */
    public String getBinDir() { return binDir; }
    /** How long to wait for the server to be ready, in milliseconds. Default 10000. */
    public int getStartupTimeoutMs() { return startupTimeoutMs; }
    /** Additional CLI arguments passed to `mockly start`. */
    public List<String> getExtraArgs() { return extraArgs; }

    public static Builder builder() {
        return new Builder();
    }

    public static class Builder {
        private String binaryPath;
        private int httpPort = 0;
        private int apiPort = 0;
        private String version = MocklyInstaller.DEFAULT_VERSION;
        private String binDir = "bin";
        private int startupTimeoutMs = 10_000;
        private final List<String> extraArgs = new ArrayList<>();

        public Builder binaryPath(String binaryPath) {
            this.binaryPath = binaryPath;
            return this;
        }

        /** Fixed HTTP mock port. Omit (or set 0) to pick a free port automatically. */
        public Builder httpPort(int httpPort) {
            this.httpPort = httpPort;
            return this;
        }

        /** Fixed management API port. Omit (or set 0) to pick a free port automatically. */
        public Builder apiPort(int apiPort) {
            this.apiPort = apiPort;
            return this;
        }

        public Builder version(String version) {
            this.version = version;
            return this;
        }

        public Builder binDir(String binDir) {
            this.binDir = binDir;
            return this;
        }

        public Builder startupTimeoutMs(int startupTimeoutMs) {
            this.startupTimeoutMs = startupTimeoutMs;
            return this;
        }

        public Builder extraArg(String arg) {
            extraArgs.add(arg);
            return this;
        }

        public MocklyConfig build() {
            return new MocklyConfig(this);
        }
    }
}
