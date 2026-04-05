package io.mockly.driver.model;

import java.util.Collections;
import java.util.HashMap;
import java.util.Map;

public class MockResponse {
    private final int status;
    private final String body;
    private final Map<String, String> headers;
    private final String delay;

    private MockResponse(Builder builder) {
        this.status = builder.status;
        this.body = builder.body;
        this.headers = Collections.unmodifiableMap(new HashMap<>(builder.headers));
        this.delay = builder.delay;
    }

    public int getStatus() { return status; }
    public String getBody() { return body; }
    public Map<String, String> getHeaders() { return headers; }
    /** Delay string, e.g. "50ms". May be null. */
    public String getDelay() { return delay; }

    public static Builder builder(int status) {
        return new Builder(status);
    }

    public static class Builder {
        private final int status;
        private String body;
        private final Map<String, String> headers = new HashMap<>();
        private String delay;

        private Builder(int status) {
            this.status = status;
        }

        public Builder body(String body) {
            this.body = body;
            return this;
        }

        public Builder header(String name, String value) {
            headers.put(name, value);
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

        public MockResponse build() {
            return new MockResponse(this);
        }
    }
}
