package io.mockly.driver.kotlin

import io.mockly.driver.model.FaultConfig
import io.mockly.driver.model.Mock
import io.mockly.driver.model.MockRequest
import io.mockly.driver.model.MockResponse
import io.mockly.driver.model.Scenario
import io.mockly.driver.model.ScenarioPatch

/** Kotlin builder backing [MockDsl.request]. Mirrors [MockRequest.Builder]. */
class MockRequestDsl internal constructor(private val method: String, private val path: String) {
    private val headers = mutableMapOf<String, String>()

    /** Adds a header to match on the mocked request. */
    fun header(name: String, value: String) {
        headers[name] = value
    }

    internal fun build(): MockRequest {
        val builder = MockRequest.builder(method, path)
        headers.forEach { (name, value) -> builder.header(name, value) }
        return builder.build()
    }
}

/** Kotlin builder backing [MockDsl.response]. Mirrors [MockResponse.Builder]. */
class MockResponseDsl internal constructor(private val status: Int) {
    /** Response body, e.g. a JSON string. */
    var body: String? = null

    /** Artificial delay before responding, e.g. `"50ms"`. */
    var delay: String? = null

    private val headers = mutableMapOf<String, String>()

    /** Adds a response header. */
    fun header(name: String, value: String) {
        headers[name] = value
    }

    internal fun build(): MockResponse {
        val builder = MockResponse.builder(status)
        body?.let { builder.body(it) }
        delay?.let { builder.delay(it) }
        headers.forEach { (name, value) -> builder.header(name, value) }
        return builder.build()
    }
}

/**
 * Kotlin builder backing the [mock] DSL function. Mirrors [Mock.Builder], but defers
 * building the request/response until [request]/[response] are called with their own
 * nested DSL blocks.
 */
class MockDsl internal constructor(private val id: String) {
    private var request: MockRequest? = null
    private var response: MockResponse? = null

    /** Defines the request this mock matches. */
    fun request(method: String, path: String, block: MockRequestDsl.() -> Unit = {}) {
        request = MockRequestDsl(method, path).apply(block).build()
    }

    /** Defines the response this mock returns. */
    fun response(status: Int, block: MockResponseDsl.() -> Unit = {}) {
        response = MockResponseDsl(status).apply(block).build()
    }

    internal fun build(): Mock {
        val builtRequest = checkNotNull(request) { "mock(\"$id\") { ... } must call request(method, path)" }
        val builtResponse = checkNotNull(response) { "mock(\"$id\") { ... } must call response(status)" }
        return Mock.builder(id, builtRequest, builtResponse).build()
    }
}

/**
 * Builds a [Mock] using a Kotlin DSL, e.g.:
 * ```kotlin
 * val getUser = mock("get-user") {
 *     request("GET", "/users/1")
 *     response(200) {
 *         body = """{"id":1}"""
 *         header("Content-Type", "application/json")
 *     }
 * }
 * server.addMock(getUser)
 * ```
 */
fun mock(id: String, block: MockDsl.() -> Unit): Mock = MockDsl(id).apply(block).build()

/** Kotlin builder backing [ScenarioDsl.patch]. Mirrors [ScenarioPatch.Builder]. */
class ScenarioPatchDsl internal constructor(private val mockId: String) {
    /** Overrides the mock's response status while the scenario is active. */
    var status: Int? = null

    /** Overrides the mock's response body while the scenario is active. */
    var body: String? = null

    /** Overrides the mock's response delay while the scenario is active. */
    var delay: String? = null

    /** Disables the mock entirely while the scenario is active. */
    var disabled: Boolean? = null

    private val headers = mutableMapOf<String, String>()

    /** Overrides/adds a response header while the scenario is active. */
    fun header(name: String, value: String) {
        headers[name] = value
    }

    internal fun build(): ScenarioPatch {
        val builder = ScenarioPatch.builder(mockId)
        status?.let { builder.status(it) }
        body?.let { builder.body(it) }
        delay?.let { builder.delay(it) }
        disabled?.let { builder.disabled(it) }
        headers.forEach { (name, value) -> builder.header(name, value) }
        return builder.build()
    }
}

/** Kotlin builder backing the [scenario] DSL function. Mirrors [Scenario.Builder]. */
class ScenarioDsl internal constructor(private val id: String, private val name: String) {
    /** Human-readable description of the scenario. */
    var description: String? = null

    private val patches = mutableListOf<ScenarioPatch>()

    /** Adds a patch overriding [mockId]'s response while this scenario is active. */
    fun patch(mockId: String, block: ScenarioPatchDsl.() -> Unit) {
        patches += ScenarioPatchDsl(mockId).apply(block).build()
    }

    internal fun build(): Scenario {
        val builder = Scenario.builder(id, name)
        description?.let { builder.description(it) }
        patches.forEach { builder.patch(it) }
        return builder.build()
    }
}

/**
 * Builds a [Scenario] using a Kotlin DSL, e.g.:
 * ```kotlin
 * val maintenance = scenario("maintenance", "Maintenance mode") {
 *     patch("get-user") { status = 503; body = """{"error":"unavailable"}""" }
 * }
 * server.createScenario(maintenance)
 * server.activateScenario(maintenance.id)
 * ```
 */
fun scenario(id: String, name: String, block: ScenarioDsl.() -> Unit = {}): Scenario =
    ScenarioDsl(id, name).apply(block).build()

/** Kotlin builder backing the [faultConfig] DSL function. Mirrors [FaultConfig.Builder]. */
class FaultConfigDsl internal constructor(private val enabled: Boolean) {
    /** Artificial delay added to every request, e.g. `"200ms"`. */
    var delay: String? = null

    /** HTTP status code to return instead of the matched mock's status. */
    var status: Int? = null

    /** Probability (0.0-1.0) that the fault fires. */
    var errorRate: Double? = null

    internal fun build(): FaultConfig {
        val builder = FaultConfig.builder(enabled)
        delay?.let { builder.delay(it) }
        status?.let { builder.status(it) }
        errorRate?.let { builder.errorRate(it) }
        return builder.build()
    }
}

/**
 * Builds a [FaultConfig] using a Kotlin DSL, e.g.:
 * ```kotlin
 * server.setFault(faultConfig {
 *     delay = "200ms"
 *     errorRate = 0.5
 * })
 * ```
 */
fun faultConfig(enabled: Boolean = true, block: FaultConfigDsl.() -> Unit = {}): FaultConfig =
    FaultConfigDsl(enabled).apply(block).build()
