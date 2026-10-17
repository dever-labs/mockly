package io.mockly.driver.kotlin

import org.junit.jupiter.api.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

class MocklyConfigDslTest {

    @Test
    fun `mocklyConfig applies defaults when block is empty`() {
        val config = mocklyConfig()

        assertEquals(0, config.httpPort)
        assertEquals(0, config.apiPort)
        assertTrue(config.extraArgs.isEmpty())
        assertTrue(config.scenarios.isEmpty())
    }

    @Test
    fun `mocklyConfig applies overrides`() {
        val config = mocklyConfig {
            httpPort = 8080
            apiPort = 8081
            binDir = "custom-bin"
            startupTimeoutMs = 5_000
            extraArg("--verbose")
            scenario("maintenance", "Maintenance mode") {
                patch("get-user") { status = 503 }
            }
        }

        assertEquals(8080, config.httpPort)
        assertEquals(8081, config.apiPort)
        assertEquals("custom-bin", config.binDir)
        assertEquals(5_000, config.startupTimeoutMs)
        assertEquals(listOf("--verbose"), config.extraArgs)
        assertEquals(1, config.scenarios.size)
        assertEquals("maintenance", config.scenarios[0].id)
    }

    @Test
    fun `mocklyConfig accepts a pre-built scenario`() {
        val maintenance = scenario("maintenance", "Maintenance mode")

        val config = mocklyConfig {
            scenario(maintenance)
        }

        assertEquals(listOf(maintenance), config.scenarios)
    }
}
