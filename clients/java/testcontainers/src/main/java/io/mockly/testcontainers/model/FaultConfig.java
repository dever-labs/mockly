package io.mockly.testcontainers.model;

public class FaultConfig {
    private final boolean enabled;
    private final String delay;
    private final Integer statusOverride;
    private final Double errorRate;

    private FaultConfig(Builder builder) {
        this.enabled = builder.enabled;
        this.delay = builder.delay;
        this.statusOverride = builder.statusOverride;
        this.errorRate = builder.errorRate;
    }

    public boolean isEnabled() { return enabled; }
    public String getDelay() { return delay; }
    public Integer getStatusOverride() { return statusOverride; }
    public Double getErrorRate() { return errorRate; }

    public static Builder builder(boolean enabled) {
        return new Builder(enabled);
    }

    public static class Builder {
        private final boolean enabled;
        private String delay;
        private Integer statusOverride;
        private Double errorRate;

        private Builder(boolean enabled) {
            this.enabled = enabled;
        }

        public Builder delay(String delay) {
            this.delay = delay;
            return this;
        }

        public Builder statusOverride(int statusOverride) {
            this.statusOverride = statusOverride;
            return this;
        }

        public Builder errorRate(double errorRate) {
            this.errorRate = errorRate;
            return this;
        }

        public FaultConfig build() {
            return new FaultConfig(this);
        }
    }
}
