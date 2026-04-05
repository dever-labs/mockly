package io.mockly.driver.model;

/** Configuration for injecting a network fault into the mock server. */
public class FaultConfig {
    private final String type;
    private final Double probability;
    private final String delay;
    private final Integer statusCode;

    private FaultConfig(Builder builder) {
        this.type = builder.type;
        this.probability = builder.probability;
        this.delay = builder.delay;
        this.statusCode = builder.statusCode;
    }

    /** Fault type, e.g. "delay", "error", "reset". */
    public String getType() { return type; }
    /** Probability between 0.0 and 1.0. May be null (meaning always). */
    public Double getProbability() { return probability; }
    /** Delay string, e.g. "200ms". May be null. */
    public String getDelay() { return delay; }
    /** HTTP status code to return on error fault. May be null. */
    public Integer getStatusCode() { return statusCode; }

    public static Builder builder(String type) {
        return new Builder(type);
    }

    public static class Builder {
        private final String type;
        private Double probability;
        private String delay;
        private Integer statusCode;

        private Builder(String type) {
            this.type = type;
        }

        public Builder probability(double probability) {
            this.probability = probability;
            return this;
        }

        public Builder delay(String delay) {
            this.delay = delay;
            return this;
        }

        public Builder statusCode(int statusCode) {
            this.statusCode = statusCode;
            return this;
        }

        public FaultConfig build() {
            return new FaultConfig(this);
        }
    }
}
