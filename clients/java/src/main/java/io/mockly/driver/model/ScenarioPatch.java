package io.mockly.driver.model;

/** Overrides a mock's behaviour when a scenario is active. */
public class ScenarioPatch {
    private final String mockId;
    private final Integer status;
    private final String body;
    private final String delay;

    private ScenarioPatch(Builder builder) {
        this.mockId = builder.mockId;
        this.status = builder.status;
        this.body = builder.body;
        this.delay = builder.delay;
    }

    public String getMockId() { return mockId; }
    /** HTTP status override. May be null (unchanged). */
    public Integer getStatus() { return status; }
    /** Response body override. May be null (unchanged). */
    public String getBody() { return body; }
    /** Delay override, e.g. "200ms". May be null (unchanged). */
    public String getDelay() { return delay; }

    public static Builder builder(String mockId) {
        return new Builder(mockId);
    }

    public static class Builder {
        private final String mockId;
        private Integer status;
        private String body;
        private String delay;

        private Builder(String mockId) {
            this.mockId = mockId;
        }

        public Builder status(int status) {
            this.status = status;
            return this;
        }

        public Builder body(String body) {
            this.body = body;
            return this;
        }

        public Builder delay(String delay) {
            this.delay = delay;
            return this;
        }

        public ScenarioPatch build() {
            return new ScenarioPatch(this);
        }
    }
}
