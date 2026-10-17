#!/usr/bin/env node
// `npx @dever-labs/mockly-cli <args>` (aliased as `mockly` via the "bin"
// field) — resolves the Mockly binary for the current platform (downloading
// it on first run if needed, exactly like @dever-labs/mockly-driver does for
// test suites) and execs it with all CLI args forwarded, e.g.:
//
//   npx @dever-labs/mockly-cli start --config mockly.yaml
//   npx @dever-labs/mockly-cli preset use keycloak
import { spawn } from 'node:child_process'
import { install, getBinaryPath } from '@dever-labs/mockly-driver'

async function main() {
  const binPath = getBinaryPath() ?? (await install())
  const args = process.argv.slice(2)

  const child = spawn(binPath, args, { stdio: 'inherit' })

  child.on('error', (err) => {
    console.error(`mockly: failed to launch binary at ${binPath}: ${err.message}`)
    process.exitCode = 1
  })

  child.on('exit', (code, signal) => {
    if (signal) {
      // Re-raise the same signal so the parent shell sees the expected
      // termination cause (e.g. Ctrl-C) instead of a plain non-zero exit.
      process.kill(process.pid, signal)
      return
    }
    process.exitCode = code ?? 1
  })
}

main().catch((err) => {
  console.error(`mockly: ${err?.message ?? err}`)
  process.exitCode = 1
})
