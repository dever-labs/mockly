package io.mockly.driver.kotlin

import io.mockly.driver.MocklyConfig
import io.mockly.driver.MocklyServer
import io.mockly.driver.model.Scenario

/**
 * Kotlin builder backing the [mocklyConfig] DSL function. Mirrors [MocklyConfig.Builder]
 * with mutable properties instead of fluent setters, so unset values fall back to the
 * same defaults as the Java builder.
 */
class MocklyConfigDsl internal constructor() {
    /** Explicit path to the mockly binary. If null, auto-resolved. */
    var binaryPath: String? = null

    /** Fixed HTTP mock port. Leave at 0 to pick a free port automatically. */
    var httpPort: Int = 0

    /** Fixed management API port. Leave at 0 to pick a free port automatically. */
    var apiPort: Int = 0

    /** Mockly version to download if not already present. Defaults to [MocklyConfig.Builder]'s own default. */
    var version: String? = null

    /** Directory to search for / install the binary. Defaults to {@code ./bin}. */
    var binDir: String? = null

    /** How long to wait for the server to be ready, in milliseconds. */
    var startupTimeoutMs: Int? = null

    private val extraArgs = mutableListOf<String>()
    private val scenarios = mutableListOf<Scenario>()

    /** Adds a single extra CLI argument passed to `mockly start`. */
    fun extraArg(arg: String) {
        extraArgs += arg
    }

    /** Adds a [Scenario] built elsewhere to the startup config. */
    fun scenario(scenario: Scenario) {
        scenarios += scenario
    }

    /** Builds and adds a [Scenario] using the [scenario] DSL function. */
    fun scenario(id: String, name: String, block: ScenarioDsl.() -> Unit = {}) {
        scenarios += ScenarioDsl(id, name).apply(block).build()
    }

    internal fun toConfig(): MocklyConfig {
        val builder = MocklyConfig.builder()
        binaryPath?.let { builder.binaryPath(it) }
        builder.httpPort(httpPort)
        builder.apiPort(apiPort)
        version?.let { builder.version(it) }
        binDir?.let { builder.binDir(it) }
        startupTimeoutMs?.let { builder.startupTimeoutMs(it) }
        extraArgs.forEach { builder.extraArg(it) }
        builder.scenarios(scenarios)
        return builder.build()
    }
}

/**
 * Builds a [MocklyConfig] using a Kotlin DSL, e.g.:
 * ```kotlin
 * val config = mocklyConfig {
 *     httpPort = 8080
 *     scenario("maintenance", "Maintenance mode") {
 *         patch("get-user") { status = 503 }
 *     }
 * }
 * ```
 */
fun mocklyConfig(block: MocklyConfigDsl.() -> Unit = {}): MocklyConfig =
    MocklyConfigDsl().apply(block).toConfig()

/**
 * Starts (or reuses, via [MocklyServer.ensure]) a Mockly server configured with the
 * [mocklyConfig] DSL. Equivalent to `MocklyServer.ensure(mocklyConfig(block))`.
 */
fun mocklyServer(block: MocklyConfigDsl.() -> Unit = {}): MocklyServer =
    MocklyServer.ensure(mocklyConfig(block))
