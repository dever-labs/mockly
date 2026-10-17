#!/usr/bin/env node
// Thin CLI entry point for `npx mockly-install` — see README.md for usage.
// Delegates to the compiled install() logic in dist/install.js (the package
// must be built first; prepublishOnly/npm-publish always runs `npm run
// build` before this file is shipped, so dist/ is guaranteed to exist for
// consumers installing from the registry).
import { install } from '../dist/install.js'

install().catch((err) => {
  console.error(`mockly-install: ${err?.message ?? err}`)
  process.exitCode = 1
})
