#!/usr/bin/env node
/**
 * CLI entry point for installing the Mockly binary.
 *
 * Usage:
 *   node node_modules/mockly-node/bin/install.mjs
 *   npx mockly-node-install
 *
 * Options (via environment variables):
 *   MOCKLY_VERSION            Override binary version (default: bundled default)
 *   MOCKLY_DOWNLOAD_BASE_URL  Override base download URL (Artifactory / mirrors)
 *   MOCKLY_BINARY_PATH        Use a pre-existing binary at this path
 *   MOCKLY_NO_INSTALL         Fail with instructions instead of downloading
 *   HTTPS_PROXY / HTTP_PROXY  Route download through HTTP proxy
 */

import { install } from '../dist/install.js'

try {
  const binPath = await install({ force: process.argv.includes('--force') })
  process.stdout.write(`mockly binary ready at ${binPath}\n`)
} catch (err) {
  process.stderr.write(`\nmockly-node install failed:\n${err.message}\n\n`)
  process.exit(1)
}
