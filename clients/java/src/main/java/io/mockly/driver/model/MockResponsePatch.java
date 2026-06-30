package io.mockly.driver.model;

import java.util.Collections;
import java.util.HashMap;
import java.util.Map;

/** Partial response update for an existing HTTP mock. */
public class MockResponsePatch {
    private final Integer status;
    private final String body;
    private final Map<String, String> headers;
    private final String delay;

    private MockResponsePatch(Builder builder) {
        this.status = builder.status;
        this.body = builder.body;
        this.headers = Collections.unmodifiableMap(new HashMap<>(builder.headers));
        this.delay = builder.delay;
    }

    public Integer getStatus() { return status; }
    public String getBody() { return body; }
    public Map<String, String> getHeaders() { return headers; }
    public String getDelay() { return delay; }

    public static Builder builder() {
        return new Builder();
    }

    public static class Builder {
        private Integer status;
        private String body;
        private final Map<String, String> headers = new HashMap<>();
        private String delay;

        public Builder status(int status) {
            this.status = status;
            return this;
        }

        public Builder body(String body) {
            this.body = body;
            return this;
        }

        public Builder header(String name, String value) {
            this.headers.put(name, value);
            return this;
        }

        public Builder headers(Map<String, String> headers) {
            this.headers.putAll(headers);
            return this;
        }

        public Builder delay(String delay) {
            this.delay = delay;
            return this;
        }

        public MockResponsePatch build() {
            return new MockResponsePatch(this);
        }
    }
}
