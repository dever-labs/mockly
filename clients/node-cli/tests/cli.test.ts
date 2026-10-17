import { describe, it, expect } from 'vitest'
import { spawn } from 'node:child_process'
import { writeFileSync, chmodSync, mkdtempSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { fileURLToPath } from 'node:url'

const binPath = fileURLToPath(new URL('../bin/mockly.mjs', import.meta.url))

/**
 * Writes a fake "mockly" binary (a shell script) that just echoes its argv,
 * so these tests can verify argument forwarding / exit code propagation
 * without needing a real platform binary or network access.
 */
function makeFakeBinary(script: string): string {
  const dir = mkdtempSync(join(tmpdir(), 'mockly-cli-test-'))
  const path = join(dir, 'fake-mockly')
  writeFileSync(path, `#!/bin/sh\n${script}\n`)
  chmodSync(path, 0o755)
  return path
}

function runCli(args: string[], env: NodeJS.ProcessEnv): Promise<{ code: number | null; stdout: string }> {
  return new Promise((resolve, reject) => {
    const child = spawn(process.execPath, [binPath, ...args], { env })
    let stdout = ''
    child.stdout.on('data', (d) => (stdout += d.toString()))
    child.on('error', reject)
    child.on('exit', (code) => resolve({ code, stdout }))
  })
}

describe('mockly CLI', () => {
  it('forwards argv to the resolved binary', async () => {
    const fake = makeFakeBinary('echo "args: $@"')
    const { code, stdout } = await runCli(['start', '--config', 'mockly.yaml'], {
      ...process.env,
      MOCKLY_BINARY_PATH: fake,
    })
    expect(code).toBe(0)
    expect(stdout).toContain('args: start --config mockly.yaml')
  })

  it('propagates the binary non-zero exit code', async () => {
    const fake = makeFakeBinary('exit 3')
    const { code } = await runCli([], { ...process.env, MOCKLY_BINARY_PATH: fake })
    expect(code).toBe(3)
  })
})
