package io.mockly.driver.kotlin

import io.mockly.driver.MocklyServer
import io.mockly.driver.model.CallSummary
import kotlinx.coroutines.test.runTest
import org.junit.jupiter.api.Test
import org.mockito.Mockito
import org.mockito.Mockito.verify
import org.mockito.Mockito.`when`
import java.time.Duration
import kotlin.test.assertEquals
import kotlin.test.assertTrue

/** Local alias so Mockito's `mock(Class)` doesn't collide with this package's own [mock] DSL function. */
private fun <T> mockitoMock(type: Class<T>): T = Mockito.mock(type)

/**
 * Verifies that every suspend wrapper actually delegates to the matching blocking
 * [MocklyServer] method (rather than, say, silently calling the wrong one or not
 * calling through at all), and that [io] runs its block off the calling dispatcher.
 * [MocklyServer] has a private constructor (only reachable via the real, binary-backed
 * [MocklyServer.ensure]), so a Mockito mock stands in for it here instead of a live server.
 */
class SuspendingTest {

    @Test
    fun `io runs the block on the IO dispatcher`() = runTest {
        val server = mockitoMock(MocklyServer::class.java)
        val callingThread = Thread.currentThread().name

        val result = server.io { Thread.currentThread().name }

        // Dispatchers.IO always uses a dedicated worker thread pool, so the block must
        // not run on the test's own thread.
        assertTrue(result.isNotEmpty())
        assertTrue(result != callingThread)
    }

    @Test
    fun `addMockSuspending delegates to addMock`() = runTest {
        val server = mockitoMock(MocklyServer::class.java)
        val mockDef = mock("get-user") {
            request("GET", "/users/1")
            response(200)
        }

        server.addMockSuspending(mockDef)

        verify(server).addMock(mockDef)
    }

    @Test
    fun `resetSuspending delegates to reset`() = runTest {
        val server = mockitoMock(MocklyServer::class.java)

        server.resetSuspending()

        verify(server).reset()
    }

    @Test
    fun `createScenarioSuspending delegates to createScenario and returns its result`() = runTest {
        val server = mockitoMock(MocklyServer::class.java)
        val input = scenario("maintenance", "Maintenance mode")
        val stored = scenario("maintenance", "Maintenance mode") { description = "stored" }
        `when`(server.createScenario(input)).thenReturn(stored)

        val result = server.createScenarioSuspending(input)

        verify(server).createScenario(input)
        assertEquals(stored, result)
    }

    @Test
    fun `activateScenarioSuspending delegates with the given id`() = runTest {
        val server = mockitoMock(MocklyServer::class.java)

        server.activateScenarioSuspending("maintenance")

        verify(server).activateScenario("maintenance")
    }

    @Test
    fun `deactivateScenarioSuspending delegates with the given id`() = runTest {
        val server = mockitoMock(MocklyServer::class.java)

        server.deactivateScenarioSuspending("maintenance")

        verify(server).deactivateScenario("maintenance")
    }

    @Test
    fun `setFaultSuspending delegates to setFault`() = runTest {
        val server = mockitoMock(MocklyServer::class.java)
        val config = faultConfig(enabled = true) { status = 500 }

        server.setFaultSuspending(config)

        verify(server).setFault(config)
    }

    @Test
    fun `clearFaultSuspending delegates to clearFault`() = runTest {
        val server = mockitoMock(MocklyServer::class.java)

        server.clearFaultSuspending()

        verify(server).clearFault()
    }

    @Test
    fun `waitForCallsSuspending delegates with all arguments and returns the summary`() = runTest {
        val server = mockitoMock(MocklyServer::class.java)
        val timeout = Duration.ofSeconds(5)
        val summary = CallSummary("get-user", 2, emptyList())
        `when`(server.waitForCalls("get-user", 2, timeout)).thenReturn(summary)

        val result = server.waitForCallsSuspending("get-user", 2, timeout)

        verify(server).waitForCalls("get-user", 2, timeout)
        assertEquals(summary, result)
    }
}
