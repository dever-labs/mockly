package io.mockly.driver.kotlin

import io.mockly.driver.MocklyServer
import io.mockly.driver.model.CallSummary
import io.mockly.driver.model.FaultConfig
import io.mockly.driver.model.Mock
import io.mockly.driver.model.Scenario
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import java.time.Duration

/**
 * Runs a blocking [MocklyServer] call on [Dispatchers.IO] instead of the calling coroutine
 * dispatcher, since every [MocklyServer] method performs synchronous blocking I/O. All
 * other suspend wrappers in this file are built on top of this escape hatch, so it can
 * also be used directly for calls that don't have a named wrapper (e.g. [MocklyServer.getLogs]):
 * ```kotlin
 * val logs = server.io { getLogs() }
 * ```
 */
suspend fun <T> MocklyServer.io(block: MocklyServer.() -> T): T =
    withContext(Dispatchers.IO) { block() }

/** Suspending equivalent of [MocklyServer.addMock]. */
suspend fun MocklyServer.addMockSuspending(mock: Mock) = io { addMock(mock) }

/** Suspending equivalent of [MocklyServer.reset]. */
suspend fun MocklyServer.resetSuspending() = io { reset() }

/** Suspending equivalent of [MocklyServer.createScenario]. */
suspend fun MocklyServer.createScenarioSuspending(scenario: Scenario) =
    io { createScenario(scenario) }

/** Suspending equivalent of [MocklyServer.activateScenario]. */
suspend fun MocklyServer.activateScenarioSuspending(id: String) = io { activateScenario(id) }

/** Suspending equivalent of [MocklyServer.deactivateScenario]. */
suspend fun MocklyServer.deactivateScenarioSuspending(id: String) = io { deactivateScenario(id) }

/** Suspending equivalent of [MocklyServer.setFault]. */
suspend fun MocklyServer.setFaultSuspending(config: FaultConfig) = io { setFault(config) }

/** Suspending equivalent of [MocklyServer.clearFault]. */
suspend fun MocklyServer.clearFaultSuspending() = io { clearFault() }

/** Suspending equivalent of [MocklyServer.waitForCalls]. */
suspend fun MocklyServer.waitForCallsSuspending(mockId: String, count: Int, timeout: Duration): CallSummary =
    io { waitForCalls(mockId, count, timeout) }

/**
 * Suspending equivalent of [mocklyServer]: starts (or reuses) a Mockly server off the
 * calling coroutine dispatcher.
 */
suspend fun mocklyServerSuspending(block: MocklyConfigDsl.() -> Unit = {}): MocklyServer =
    withContext(Dispatchers.IO) { mocklyServer(block) }
