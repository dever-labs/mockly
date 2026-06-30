package io.mockly.driver.model;

/** Global fault injection configuration. */
public class FaultConfig {
    private final boolean enabled;
    private final String delay;
    private final Integer status;
    private final Double errorRate;

    private FaultConfig(Builder builder) {
        this.enabled = builder.enabled;
        this.delay = builder.delay;
        this.status = builder.status;
        this.errorRate = builder.errorRate;
    }

    public boolean isEnabled() { return enabled; }
    /** Artificial delay added to every request, e.g. "200ms". May be null.
     * @return delay string, or {@code null}
     */
    public String getDelay() { return delay; }
    /** HTTP status code to return instead of the matched mock's status. May be null.
     * @return status, or {@code null}
     */
    public Integer getStatus() { return status; }
    /** Probability (0.0–1.0) that the fault fires; 0 means always. May be null.
     * @return error rate, or {@code null}
     */
    public Double getErrorRate() { return errorRate; }

    public static Builder builder(boolean enabled) {
        return new Builder(enabled);
    }

    public static class Builder {
        private final boolean enabled;
        private String delay;
        private Integer status;
        private Double errorRate;

        private Builder(boolean enabled) {
            this.enabled = enabled;
        }

        public Builder delay(String delay) {
            this.delay = delay;
            return this;
        }

        public Builder status(int status) {
            this.status = status;
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
