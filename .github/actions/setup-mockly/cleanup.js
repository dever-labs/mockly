#!/usr/bin/env node
// Setup Mockly (post phase): stops the mockly process started by main.js.
//
// This file is registered as `runs.post` in action.yml, which (unlike a
// composite action's inline steps) genuinely runs after every other step in
// the calling job — including the consumer's actual test step — and also
// runs when an earlier step in the job failed, so mockly doesn't linger as
// an orphaned background process on self-hosted runners.
'use strict';

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

function isAlive(pid) {
  try {
    // Signal 0 doesn't actually send a signal; it only probes whether the
    // process exists and is one we're permitted to signal.
    process.kill(pid, 0);
    return true;
  } catch {
    return false;
  }
}

async function main() {
  const pidStr = process.env.STATE_mockly_pid;
  if (!pidStr) {
    // Either `start: false` was used, or main.js failed before spawning
    // mockly — nothing to clean up.
    return;
  }
  const pid = parseInt(pidStr, 10);
  if (!Number.isInteger(pid) || !isAlive(pid)) {
    return;
  }

  console.log(`Stopping mockly (pid ${pid})...`);
  try {
    process.kill(pid, 'SIGTERM');
  } catch {
    return; // already gone
  }

  for (let i = 0; i < 10 && isAlive(pid); i++) {
    await sleep(200);
  }
  if (isAlive(pid)) {
    try {
      process.kill(pid, 'SIGKILL');
    } catch {
      // already gone
    }
  }

  console.log(
    'mockly stopped. If this job failed, its log output (the "log-file" ' +
      'output of this action) can be uploaded for debugging, e.g.:\n' +
      '  - if: failure()\n' +
      '    uses: actions/upload-artifact@v4\n' +
      '    with:\n' +
      '      name: mockly-logs\n' +
      '      path: ${{ steps.<this-step-id>.outputs.log-file }}'
  );
}

main().catch((err) => {
  console.error(err instanceof Error ? err.message : String(err));
  // A cleanup failure shouldn't fail the whole job after tests already ran.
});
