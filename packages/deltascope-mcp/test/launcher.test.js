import test from "node:test";
import assert from "node:assert/strict";
import fs from "node:fs/promises";
import os from "node:os";
import path from "node:path";

import { resolveCacheBinaryPath, resolveCacheMetadataPath } from "../lib/cache.js";
import { ensureExecutable, formatBootstrapContext } from "../lib/launcher.js";

test("ensureExecutable reuses a cached binary without downloading", async () => {
  const tempDir = await fs.mkdtemp(path.join(os.tmpdir(), "deltascope-mcp-cache-"));
  const cachedBinary = resolveCacheBinaryPath({
    homeDir: tempDir,
    version: "v0.7.0",
    os: "linux",
    arch: "amd64"
  });
  const metadataPath = resolveCacheMetadataPath({
    homeDir: tempDir,
    version: "v0.7.0",
    os: "linux",
    arch: "amd64"
  });
  await fs.mkdir(path.dirname(cachedBinary), { recursive: true });
  await fs.writeFile(cachedBinary, "#!/bin/sh\nexit 0\n", { mode: 0o755 });
  await fs.writeFile(
    metadataPath,
    `${JSON.stringify({
      version: "v0.7.0",
      os: "linux",
      arch: "amd64",
      archiveURL: "https://github.com/Fanduzi/DeltaScope/releases/download/v0.7.0/deltascope_0.7.0_linux_amd64.tar.gz",
      checksumsURL: "https://github.com/Fanduzi/DeltaScope/releases/download/v0.7.0/deltascope_0.7.0_checksums.txt",
      archiveChecksum: "abc123"
    })}\n`
  );

  let downloadCalls = 0;
  const resolved = await ensureExecutable({
    version: "v0.7.0",
    homeDir: tempDir,
    platform: { os: "linux", arch: "amd64" },
    archiveURL: "https://github.com/Fanduzi/DeltaScope/releases/download/v0.7.0/deltascope_0.7.0_linux_amd64.tar.gz",
    checksumsURL: "https://github.com/Fanduzi/DeltaScope/releases/download/v0.7.0/deltascope_0.7.0_checksums.txt",
    downloadBinary: async () => {
      downloadCalls += 1;
      return cachedBinary;
    }
  });

  assert.equal(resolved, cachedBinary);
  assert.equal(downloadCalls, 0);
});

test("ensureExecutable redownloads when cache metadata is missing", async () => {
  const tempDir = await fs.mkdtemp(path.join(os.tmpdir(), "deltascope-mcp-cache-"));
  const cachedBinary = resolveCacheBinaryPath({
    homeDir: tempDir,
    version: "v0.7.0",
    os: "linux",
    arch: "amd64"
  });
  const metadataPath = resolveCacheMetadataPath({
    homeDir: tempDir,
    version: "v0.7.0",
    os: "linux",
    arch: "amd64"
  });
  await fs.mkdir(path.dirname(cachedBinary), { recursive: true });
  await fs.writeFile(cachedBinary, "stale\n", { mode: 0o755 });
  await fs.rm(metadataPath, { force: true });

  let downloadCalls = 0;
  const resolved = await ensureExecutable({
    version: "v0.7.0",
    homeDir: tempDir,
    platform: { os: "linux", arch: "amd64" },
    archiveURL: "https://mirror.example.com/v0.7.0/archive.tar.gz",
    checksumsURL: "https://github.com/Fanduzi/DeltaScope/releases/download/v0.7.0/deltascope_0.7.0_checksums.txt",
    downloadBinary: async () => {
      downloadCalls += 1;
      const stagedPath = `${cachedBinary}.tmp-${process.pid}`;
      await fs.writeFile(stagedPath, "#!/bin/sh\nexit 0\n", { mode: 0o755 });
      return {
        binaryPath: stagedPath,
        archiveChecksum: "abc123"
      };
    }
  });

  assert.equal(resolved, cachedBinary);
  assert.equal(downloadCalls, 1);
  assert.equal(await fs.readFile(cachedBinary, "utf8"), "#!/bin/sh\nexit 0\n");
  const metadata = JSON.parse(await fs.readFile(metadataPath, "utf8"));
  assert.equal(metadata.archiveChecksum, "abc123");
});

test("ensureExecutable times out on a fresh lock and recovers from a stale lock", async () => {
  const tempDir = await fs.mkdtemp(path.join(os.tmpdir(), "deltascope-mcp-lock-"));
  const cachedBinary = resolveCacheBinaryPath({
    homeDir: tempDir,
    version: "v0.7.0",
    os: "linux",
    arch: "amd64"
  });
  const lockDir = path.join(path.dirname(cachedBinary), ".lock");
  await fs.mkdir(lockDir, { recursive: true });

  await assert.rejects(
    () =>
      ensureExecutable({
        version: "v0.7.0",
        homeDir: tempDir,
        platform: { os: "linux", arch: "amd64" },
        archiveURL: "https://github.com/Fanduzi/DeltaScope/releases/download/v0.7.0/deltascope_0.7.0_linux_amd64.tar.gz",
        checksumsURL: "https://github.com/Fanduzi/DeltaScope/releases/download/v0.7.0/deltascope_0.7.0_checksums.txt",
        lockTimeoutMs: 50,
        lockRetryDelayMs: 10,
        downloadBinary: async () => {
          throw new Error("should not download while a fresh lock is held");
        }
      }),
    /timed out waiting for launcher cache lock/i
  );

  const staleTime = new Date(Date.now() - 120000);
  await fs.utimes(lockDir, staleTime, staleTime);

  let downloadCalls = 0;
  const resolved = await ensureExecutable({
    version: "v0.7.0",
    homeDir: tempDir,
    platform: { os: "linux", arch: "amd64" },
    archiveURL: "https://github.com/Fanduzi/DeltaScope/releases/download/v0.7.0/deltascope_0.7.0_linux_amd64.tar.gz",
    checksumsURL: "https://github.com/Fanduzi/DeltaScope/releases/download/v0.7.0/deltascope_0.7.0_checksums.txt",
    staleLockMs: 1000,
    lockTimeoutMs: 100,
    lockRetryDelayMs: 10,
    downloadBinary: async () => {
      downloadCalls += 1;
      const stagedPath = `${cachedBinary}.tmp-${process.pid}`;
      await fs.writeFile(stagedPath, "#!/bin/sh\nexit 0\n", { mode: 0o755 });
      return {
        binaryPath: stagedPath,
        archiveChecksum: "recovered"
      };
    }
  });

  assert.equal(resolved, cachedBinary);
  assert.equal(downloadCalls, 1);
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
