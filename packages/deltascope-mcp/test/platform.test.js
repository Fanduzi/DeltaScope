import test from "node:test";
import assert from "node:assert/strict";

import { resolveArchiveName, resolveArchiveURL, resolveDeltaScopeVersion, resolvePlatform } from "../lib/releases.js";
import { resolveCacheBinaryPath } from "../lib/cache.js";

test("resolvePlatform normalizes supported hosts", () => {
  assert.deepEqual(resolvePlatform({ platform: "darwin", arch: "arm64" }), {
    os: "darwin",
    arch: "arm64"
  });
  assert.deepEqual(resolvePlatform({ platform: "linux", arch: "x64" }), {
    os: "linux",
    arch: "amd64"
  });
});

test("resolvePlatform rejects unsupported hosts", () => {
  assert.throws(() => resolvePlatform({ platform: "win32", arch: "x64" }), /unsupported platform/i);
  assert.throws(() => resolvePlatform({ platform: "linux", arch: "ppc64" }), /unsupported architecture/i);
});

test("resolveDeltaScopeVersion prefers env override and prefixes v", () => {
  assert.equal(resolveDeltaScopeVersion({ packageVersion: "0.7.0", envVersion: "" }), "v0.7.0");
  assert.equal(resolveDeltaScopeVersion({ packageVersion: "0.7.0", envVersion: "v0.8.0" }), "v0.8.0");
  assert.equal(resolveDeltaScopeVersion({ packageVersion: "0.7.0", envVersion: "0.8.1" }), "v0.8.1");
});

test("resolveArchiveName and resolveArchiveURL follow the release contract", () => {
  assert.equal(resolveArchiveName({ version: "v0.7.0", os: "darwin", arch: "arm64" }), "deltascope_0.7.0_darwin_arm64.tar.gz");
  assert.equal(
    resolveArchiveURL({
      repo: "Fanduzi/DeltaScope",
      version: "v0.7.0",
      os: "linux",
      arch: "amd64"
    }),
    "https://github.com/Fanduzi/DeltaScope/releases/download/v0.7.0/deltascope_0.7.0_linux_amd64.tar.gz"
  );
  assert.equal(
    resolveArchiveURL({
      baseURL: "https://mirror.example.com/deltascope/releases/download",
      version: "v0.7.0",
      os: "linux",
      arch: "amd64"
    }),
    "https://mirror.example.com/deltascope/releases/download/v0.7.0/deltascope_0.7.0_linux_amd64.tar.gz"
  );
});

test("resolveCacheBinaryPath keys cache by version and platform", () => {
  assert.equal(
    resolveCacheBinaryPath({
      homeDir: "/tmp/home",
      version: "v0.7.0",
      os: "darwin",
      arch: "arm64"
    }),
    "/tmp/home/.cache/deltascope-mcp/v0.7.0/darwin-arm64/deltascope-mcp"
  );
});
