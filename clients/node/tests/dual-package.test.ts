import { describe, it, expect, beforeAll, afterAll } from 'vitest'
import { execFileSync } from 'child_process'
import { mkdtempSync, writeFileSync, rmSync, readdirSync } from 'fs'
import { tmpdir } from 'os'
import { join } from 'path'

// ── Dual ESM/CJS package resolution (regression test for #387) ───────────────
//
// `tests/unit.test.ts`'s "CJS build" test loads `dist/index.cjs` via a
// relative path, which already catches syntax/runtime breakage in the CJS
// bundle itself. It does NOT, however, exercise the package's `exports` map
// the way a real consumer does: resolving `@dever-labs/mockly-driver` by
// name through Node's module resolver, which is exactly how #387 was
// originally triggered (a `require()` from an unrelated project).
//
// This test packs the package with `npm pack`, installs the resulting
// tarball into a scratch project, and runs one CJS consumer (`require`) and
// one ESM consumer (`import`) against the *installed* package by name.

describe('dual-package consumers (require + import by package name)', () => {
  let consumerDir: string
  let tarballPath: string

  beforeAll(() => {
    const packageRoot = join(__dirname, '..')

    consumerDir = mkdtempSync(join(tmpdir(), 'mockly-dual-package-'))

    const packOutput = execFileSync(
      'npm',
      ['pack', '--silent', '--pack-destination', consumerDir],
      { cwd: packageRoot, encoding: 'utf8' },
    ).trim()
    const tarballName = packOutput.split('\n').pop()!.trim()
    tarballPath = join(consumerDir, tarballName)

    writeFileSync(join(consumerDir, 'package.json'), JSON.stringify({ name: 'mockly-dual-package-consumer', private: true }))
    execFileSync('npm', ['install', '--silent', '--no-audit', '--no-fund', tarballPath], { cwd: consumerDir })

    writeFileSync(
      join(consumerDir, 'consumer.cjs'),
      "const mod = require('@dever-labs/mockly-driver');\n" +
        "console.log(JSON.stringify({\n" +
        "  MocklyServer: typeof mod.MocklyServer,\n" +
        "  install: typeof mod.install,\n" +
        "  getBinaryPath: typeof mod.getBinaryPath,\n" +
        "  getFreePort: typeof mod.getFreePort,\n" +
        "  DEFAULT_MOCKLY_VERSION: typeof mod.DEFAULT_MOCKLY_VERSION,\n" +
        "}));\n",
    )

    writeFileSync(
      join(consumerDir, 'consumer.mjs'),
      "import * as mod from '@dever-labs/mockly-driver';\n" +
        "console.log(JSON.stringify({\n" +
        "  MocklyServer: typeof mod.MocklyServer,\n" +
        "  install: typeof mod.install,\n" +
        "  getBinaryPath: typeof mod.getBinaryPath,\n" +
        "  getFreePort: typeof mod.getFreePort,\n" +
        "  DEFAULT_MOCKLY_VERSION: typeof mod.DEFAULT_MOCKLY_VERSION,\n" +
        "}));\n",
    )
  }, 60_000)

  afterAll(() => {
    if (consumerDir) rmSync(consumerDir, { recursive: true, force: true })
  })

  const expectedShape = {
    MocklyServer: 'function',
    install: 'function',
    getBinaryPath: 'function',
    getFreePort: 'function',
    DEFAULT_MOCKLY_VERSION: 'string',
  }

  it('resolves via require() (CommonJS) when installed by package name', () => {
    const out = execFileSync('node', ['consumer.cjs'], { cwd: consumerDir, encoding: 'utf8' })
    expect(JSON.parse(out)).toEqual(expectedShape)
  })

  it('resolves via import (ESM) when installed by package name', () => {
    const out = execFileSync('node', ['consumer.mjs'], { cwd: consumerDir, encoding: 'utf8' })
    expect(JSON.parse(out)).toEqual(expectedShape)
  })

  it('packs a tarball containing both the CJS and ESM builds', () => {
    // Sanity check the files we just asserted against actually shipped.
    const files = readdirSync(join(consumerDir, 'node_modules/@dever-labs/mockly-driver/dist'))
    expect(files).toContain('index.cjs')
    expect(files).toContain('index.js')
  })
})
