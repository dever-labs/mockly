package io.mockly.driver.model;

import java.util.Collections;
import java.util.HashMap;
import java.util.Map;

public class MockRequest {
    private final String method;
    private final String path;
    private final Map<String, String> headers;

    private MockRequest(Builder builder) {
        this.method = builder.method;
        this.path = builder.path;
        this.headers = Collections.unmodifiableMap(new HashMap<>(builder.headers));
    }

    public String getMethod() { return method; }
    public String getPath() { return path; }
    public Map<String, String> getHeaders() { return headers; }

    public static Builder builder(String method, String path) {
        return new Builder(method, path);
    }

    public static class Builder {
        private final String method;
        private final String path;
        private final Map<String, String> headers = new HashMap<>();

        private Builder(String method, String path) {
            this.method = method;
            this.path = path;
        }

        public Builder header(String name, String value) {
            headers.put(name, value);
            return this;
        }

        public Builder headers(Map<String, String> headers) {
            this.headers.putAll(headers);
            return this;
        }

        public MockRequest build() {
            return new MockRequest(this);
        }
    }
}
