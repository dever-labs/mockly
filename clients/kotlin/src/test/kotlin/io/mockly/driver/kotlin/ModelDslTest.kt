package io.mockly.driver.kotlin

import org.junit.jupiter.api.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNull
import kotlin.test.assertTrue

class ModelDslTest {

    @Test
    fun `mock builds request and response`() {
        val getUser = mock("get-user") {
            request("GET", "/users/1") {
                header("Accept", "application/json")
            }
            response(200) {
                body = """{"id":1}"""
                delay = "10ms"
                header("Content-Type", "application/json")
            }
        }

        assertEquals("get-user", getUser.id)
        assertEquals("GET", getUser.request.method)
        assertEquals("/users/1", getUser.request.path)
        assertEquals("application/json", getUser.request.headers["Accept"])
        assertEquals(200, getUser.response.status)
        assertEquals("""{"id":1}""", getUser.response.body)
        assertEquals("10ms", getUser.response.delay)
        assertEquals("application/json", getUser.response.headers["Content-Type"])
    }

    @Test
    fun `mock without request throws`() {
        val error = assertFailsWithMessage { mock("broken") { response(200) } }
        assertTrue(error.contains("request"))
    }

    @Test
    fun `mock without response throws`() {
        val error = assertFailsWithMessage { mock("broken") { request("GET", "/") } }
        assertTrue(error.contains("response"))
    }

    @Test
    fun `scenario builds patches`() {
        val maintenance = scenario("maintenance", "Maintenance mode") {
            description = "Returns 503 for get-user"
            patch("get-user") {
                status = 503
                body = """{"error":"unavailable"}"""
                delay = "5ms"
                disabled = false
                header("Retry-After", "30")
            }
        }

        assertEquals("maintenance", maintenance.id)
        assertEquals("Maintenance mode", maintenance.name)
        assertEquals("Returns 503 for get-user", maintenance.description)
        assertEquals(1, maintenance.patches.size)

        val patch = maintenance.patches[0]
        assertEquals("get-user", patch.mockId)
        assertEquals(503, patch.status)
        assertEquals("""{"error":"unavailable"}""", patch.body)
        assertEquals("5ms", patch.delay)
        assertEquals(false, patch.disabled)
        assertEquals("30", patch.headers["Retry-After"])
    }

    @Test
    fun `scenario defaults to no description and no patches`() {
        val empty = scenario("empty", "Empty scenario")
        assertNull(empty.description)
        assertTrue(empty.patches.isEmpty())
    }

    @Test
    fun `faultConfig builds with defaults`() {
        val fault = faultConfig()
        assertTrue(fault.isEnabled)
        assertNull(fault.delay)
        assertNull(fault.status)
        assertNull(fault.errorRate)
    }

    @Test
    fun `faultConfig builds with overrides`() {
        val fault = faultConfig(enabled = true) {
            delay = "200ms"
            status = 500
            errorRate = 0.5
        }

        assertTrue(fault.isEnabled)
        assertEquals("200ms", fault.delay)
        assertEquals(500, fault.status)
        assertEquals(0.5, fault.errorRate)
    }

    @Test
    fun `faultConfig can be disabled`() {
        val fault = faultConfig(enabled = false)
        assertFalse(fault.isEnabled)
    }

    private fun assertFailsWithMessage(block: () -> Unit): String {
        return try {
            block()
            throw AssertionError("Expected an IllegalStateException to be thrown")
        } catch (e: IllegalStateException) {
            e.message ?: ""
        }
    }
}
