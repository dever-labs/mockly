package io.mockly.testcontainers;

import io.mockly.driver.model.FaultConfig;
import io.mockly.driver.model.Mock;
import io.mockly.driver.model.MockRequest;
import io.mockly.driver.model.MockResponse;
import org.junit.jupiter.api.Test;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertSame;
import static org.junit.jupiter.api.Assertions.assertTrue;

class MocklyContainerTest {

    @Test
    void testDefaultImage() {
        MocklyContainer container = new MocklyContainer();

        assertEquals(MocklyContainer.DEFAULT_IMAGE, container.getDockerImageName());
    }

    @Test
    void testDefaultPorts() {
        assertEquals(8090, MocklyContainer.HTTP_PORT);
        assertEquals(9091, MocklyContainer.API_PORT);
    }

    @Test
    void testWithInlineConfig() {
        MocklyContainer container = new MocklyContainer();
        String yaml = "mockly:\n  api:\n    port: 9191\nprotocols:\n  http:\n    enabled: true\n    port: 8181\n";

        MocklyContainer returned = container.withInlineConfig(yaml);

        assertSame(container, returned);
        assertEquals(yaml, container.getConfiguredYaml());
    }

    @Test
    void testJsonSerializationMock() {
        Mock mock = Mock.builder(
                "hello",
                MockRequest.builder("GET", "/hello")
                        .header("Accept", "application/json")
                        .build(),
                MockResponse.builder(200)
                        .body("world")
                        .header("Content-Type", "text/plain")
                        .delay("50ms")
                        .build())
                .build();

        String json = MocklyContainer.toJson(mock);

        assertTrue(json.contains("\"id\":\"hello\""));
        assertTrue(json.contains("\"method\":\"GET\""));
        assertTrue(json.contains("\"path\":\"/hello\""));
        assertTrue(json.contains("\"status\":200"));
        assertTrue(json.contains("\"body\":\"world\""));
        assertTrue(json.contains("\"delay\":\"50ms\""));
    }

    @Test
    void testJsonSerializationFaultConfig() {
        FaultConfig faultConfig = FaultConfig.builder(true)
                .delay("200ms")
                .status(503)
                .errorRate(0.5)
                .build();

        String json = MocklyContainer.toJson(faultConfig);

        assertTrue(json.contains("\"enabled\":true"));
        assertTrue(json.contains("\"delay\":\"200ms\""));
        assertTrue(json.contains("\"status\":503"));
        assertTrue(json.contains("\"error_rate\":0.5"));
    }
}
