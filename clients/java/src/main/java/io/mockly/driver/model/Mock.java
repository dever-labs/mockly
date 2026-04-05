package io.mockly.driver.model;

public class Mock {
    private final String id;
    private final MockRequest request;
    private final MockResponse response;

    private Mock(Builder builder) {
        this.id = builder.id;
        this.request = builder.request;
        this.response = builder.response;
    }

    public String getId() { return id; }
    public MockRequest getRequest() { return request; }
    public MockResponse getResponse() { return response; }

    public static Builder builder(String id, MockRequest request, MockResponse response) {
        return new Builder(id, request, response);
    }

    public static class Builder {
        private final String id;
        private final MockRequest request;
        private final MockResponse response;

        private Builder(String id, MockRequest request, MockResponse response) {
            this.id = id;
            this.request = request;
            this.response = response;
        }

        public Mock build() {
            return new Mock(this);
        }
    }
}
