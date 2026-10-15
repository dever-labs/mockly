#!/usr/bin/env node
// Setup Mockly (main phase): installs the mockly binary for the runner's
// OS/arch, verifies its checksum, optionally starts it in the background,
// and waits for its management API to report ready.
//
// This is a dependency-free Node script (no npm packages, no node_modules
// to commit) so it can run directly via `runs.using: node20` without a
// build/bundle step. It relies only on Node builtins plus the global
// `fetch` (available since Node 18) for HTTP calls.
//
// Companion to cleanup.js, which runs in the post phase (after every other
// step in the calling job, even on failure — something a composite action
// cannot do, since composite action steps all run inline at the position
// of the `uses:` call instead of deferring to the end of the job).
'use strict';

const fs = require('fs');
const path = require('path');
const os = require('os');
const crypto = require('crypto');
const { spawn } = require('child_process');

const REPO = 'dever-labs/mockly';

function getInput(name, def) {
  const v = process.env[`INPUT_${name.toUpperCase()}`];
  return v === undefined || v === '' ? def : v;
}

function setOutput(name, value) {
  fs.appendFileSync(process.env.GITHUB_OUTPUT, `${name}=${value}\n`);
}

function saveState(name, value) {
  fs.appendFileSync(process.env.GITHUB_STATE, `${name}=${value}\n`);
}

function addToPath(dir) {
  fs.appendFileSync(process.env.GITHUB_PATH, `${dir}\n`);
}

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

// resolveTarget maps the runner's OS/arch to the release asset name mockly
// publishes (see .goreleaser.yaml / the "Build Binary" CI job), mirroring
// install.sh's OS/arch detection for the shell-based install path.
function resolveTarget() {
  const platformMap = { linux: 'linux', darwin: 'darwin', win32: 'windows' };
  const archMap = { x64: 'amd64', arm64: 'arm64' };

  const osName = platformMap[process.platform];
  const archName = archMap[process.arch];
  if (!osName || !archName) {
    throw new Error(
      `Unsupported runner platform/arch: ${process.platform}/${process.arch}`
    );
  }
  const ext = osName === 'windows' ? '.exe' : '';
  return { osName, archName, ext, binaryName: `mockly-${osName}-${archName}${ext}` };
}

async function resolveVersion(version) {
  if (version !== 'latest') {
    return version;
  }
  core_log('Resolving latest mockly version...');
  const res = await fetch(`https://api.github.com/repos/${REPO}/releases/latest`, {
    headers: {
      Accept: 'application/vnd.github+json',
      'User-Agent': 'mockly-setup-mockly-action',
    },
  });
  if (!res.ok) {
    throw new Error(`Failed to resolve latest version: HTTP ${res.status}`);
  }
  const data = await res.json();
  if (!data.tag_name) {
    throw new Error('Failed to resolve latest version: no tag_name in response');
  }
  return data.tag_name;
}

async function downloadAndVerify(version, binaryName, destPath) {
  const base = `https://github.com/${REPO}/releases/download/${version}`;
  core_log(`Downloading mockly ${version} (${binaryName})...`);

  const binRes = await fetch(`${base}/${binaryName}`);
  if (!binRes.ok) {
    throw new Error(
      `Failed to download ${binaryName} for ${version}: HTTP ${binRes.status}`
    );
  }
  const buf = Buffer.from(await binRes.arrayBuffer());

  const sumsRes = await fetch(`${base}/checksums.txt`);
  if (!sumsRes.ok) {
    throw new Error(`Failed to download checksums.txt for ${version}: HTTP ${sumsRes.status}`);
  }
  const sums = await sumsRes.text();
  const line = sums.split('\n').find((l) => l.trim().endsWith(binaryName));
  if (!line) {
    throw new Error(`No checksum entry for ${binaryName} in checksums.txt`);
  }
  const expected = line.trim().split(/\s+/)[0];
  const actual = crypto.createHash('sha256').update(buf).digest('hex');
  if (expected.toLowerCase() !== actual.toLowerCase()) {
    throw new Error(
      `Checksum mismatch for ${binaryName}: expected ${expected}, got ${actual}`
    );
  }

  fs.writeFileSync(destPath, buf);
  if (process.platform !== 'win32') {
    fs.chmodSync(destPath, 0o755);
  }
}

async function waitForReady(apiUrl, timeoutSeconds) {
  const deadline = Date.now() + timeoutSeconds * 1000;
  let lastErr;
  while (Date.now() < deadline) {
    try {
      const res = await fetch(`${apiUrl}/api/protocols`);
      if (res.ok) {
        return;
      }
      lastErr = new Error(`HTTP ${res.status}`);
    } catch (err) {
      lastErr = err;
    }
    await sleep(1000);
  }
  throw new Error(
    `mockly did not become ready within ${timeoutSeconds}s at ${apiUrl}/api/protocols` +
      (lastErr ? ` (last error: ${lastErr.message})` : '')
  );
}

// core_log is a tiny stand-in for @actions/core's info logging — kept
// local so this action has zero npm dependencies to install or bundle.
function core_log(msg) {
  console.log(msg);
}

async function main() {
  const version = getInput('version', 'latest');
  const config = getInput('config', 'mockly.yaml');
  const start = getInput('start', 'true') === 'true';
  const apiPort = getInput('api-port', '9091');
  const readyTimeout = parseInt(getInput('ready-timeout', '30'), 10);

  const tempDir = process.env.RUNNER_TEMP || os.tmpdir();
  const { binaryName } = resolveTarget();
  const resolvedVersion = await resolveVersion(version);
  const destPath = path.join(tempDir, process.platform === 'win32' ? 'mockly.exe' : 'mockly');

  await downloadAndVerify(resolvedVersion, binaryName, destPath);
  addToPath(tempDir);
  setOutput('version', resolvedVersion);
  core_log(`mockly ${resolvedVersion} installed to ${destPath}`);

  if (!start) {
    return;
  }

  const apiUrl = `http://localhost:${apiPort}`;
  const logPath = path.join(tempDir, 'mockly.log');
  const logFd = fs.openSync(logPath, 'a');

  const child = spawn(destPath, ['start', '-c', config, '--api-port', String(apiPort)], {
    detached: true,
    stdio: ['ignore', logFd, logFd],
  });
  child.unref();

  saveState('mockly_pid', String(child.pid));
  setOutput('api-url', apiUrl);
  setOutput('log-file', logPath);

  core_log(`Waiting for mockly to be ready at ${apiUrl}/api/protocols...`);
  try {
    await waitForReady(apiUrl, readyTimeout);
  } catch (err) {
    // Surface the captured log on startup failure, since the process may
    // already have exited (e.g. bad config) before the ready check ran.
    try {
      const log = fs.readFileSync(logPath, 'utf8');
      if (log.trim()) {
        core_log(`--- mockly log (${logPath}) ---\n${log}`);
      }
    } catch {
      // ignore — log file may not exist yet
    }
    throw err;
  }
  core_log('mockly is ready');
}

main().catch((err) => {
  console.error(err instanceof Error ? err.message : String(err));
  process.exitCode = 1;
});
