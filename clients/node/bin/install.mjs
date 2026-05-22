#!/usr/bin/env node
// Runs automatically on `npm install` (postinstall) and via `npx mockly-install`.
// Downloads the Mockly binary for the current platform from GitHub releases,
// unless a custom source is configured via environment variables.
//
// Environment variables:
//   MOCKLY_BINARY_PATH       — skip download; use this pre-staged binary path
//   MOCKLY_NO_INSTALL        — skip download; throw if binary not found
//   MOCKLY_DOWNLOAD_BASE_URL — override download source (Artifactory, local mirror, etc.)
//   MOCKLY_VERSION           — override binary version (default: matches npm package version)
//   HTTPS_PROXY / HTTP_PROXY — route download through an HTTP proxy

import { install } from '../dist/install.js'

const isPostinstall = process.env.npm_lifecycle_event === 'postinstall'

install().then((p) => {
  console.log(`mockly: ready at ${p}`)
}).catch((err) => {
  if (isPostinstall) {
    // Don't fail `npm install` — the binary can be fetched later via `npx mockly-install`.
    console.warn(`mockly: skipping binary download — ${err.message}`)
    console.warn('mockly: run "npx mockly-install" to download manually, or set MOCKLY_BINARY_PATH.')
  } else {
    console.error(`mockly: ${err.message}`)
    process.exit(1)
  }
})
