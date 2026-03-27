import test from "node:test";
import assert from "node:assert/strict";
import fs from "node:fs/promises";
import os from "node:os";
import path from "node:path";

import { resolveCacheBinaryPath } from "../lib/cache.js";
import { ensureExecutable, formatBootstrapContext } from "../lib/launcher.js";

test("ensureExecutable reuses a cached binary without downloading", async () => {
  const tempDir = await fs.mkdtemp(path.join(os.tmpdir(), "deltascope-mcp-cache-"));
  const cachedBinary = resolveCacheBinaryPath({
    homeDir: tempDir,
    version: "v0.7.0",
    os: "linux",
    arch: "amd64"
  });
  await fs.mkdir(path.dirname(cachedBinary), { recursive: true });
  await fs.writeFile(cachedBinary, "#!/bin/sh\nexit 0\n", { mode: 0o755 });

  let downloadCalls = 0;
  const resolved = await ensureExecutable({
    version: "v0.7.0",
    homeDir: tempDir,
    platform: { os: "linux", arch: "amd64" },
    downloadBinary: async () => {
      downloadCalls += 1;
      return cachedBinary;
    }
  });

  assert.equal(resolved, cachedBinary);
  assert.equal(downloadCalls, 0);
});

test("formatBootstrapContext includes proxy guidance for download failures", () => {
  const text = formatBootstrapContext({
    version: "v0.7.0",
    platform: { os: "darwin", arch: "arm64" },
    archiveURL: "https://github.com/Fanduzi/DeltaScope/releases/download/v0.7.0/deltascope_0.7.0_darwin_arm64.tar.gz",
    destinationPath: "/tmp/cache/deltascope-mcp"
  });

  assert.match(text, /v0\.7\.0/);
  assert.match(text, /darwin-arm64/);
  assert.match(text, /NODE_USE_ENV_PROXY=1/);
  assert.match(text, /\/tmp\/cache\/deltascope-mcp/);
});
