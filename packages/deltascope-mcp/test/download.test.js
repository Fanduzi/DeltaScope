import test from "node:test";
import assert from "node:assert/strict";
import fs from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import { spawnSync } from "node:child_process";
import crypto from "node:crypto";

import { downloadAndExtractBinary } from "../lib/download.js";

test("downloadAndExtractBinary downloads a release archive and extracts deltascope-mcp", async () => {
  const tempDir = await fs.mkdtemp(path.join(os.tmpdir(), "deltascope-mcp-download-"));
  const fixtureDir = path.join(tempDir, "fixture");
  const archivePath = path.join(tempDir, "deltascope_0.7.0_linux_amd64.tar.gz");
  const destinationPath = path.join(tempDir, "cache", "deltascope-mcp");
  await fs.mkdir(fixtureDir, { recursive: true });
  await fs.writeFile(path.join(fixtureDir, "deltascope-mcp"), "#!/bin/sh\necho launcher-ok\n", { mode: 0o755 });

  const tarResult = spawnSync("tar", ["-czf", archivePath, "-C", fixtureDir, "deltascope-mcp"], {
    stdio: "pipe"
  });
  assert.equal(tarResult.status, 0, tarResult.stderr.toString());

  const archiveBuffer = await fs.readFile(archivePath);
  const archiveSha = crypto.createHash("sha256").update(archiveBuffer).digest("hex");
  const extractedPath = await downloadAndExtractBinary({
    archiveURL: "https://example.com/deltascope_0.7.0_linux_amd64.tar.gz",
    checksumsURL: "https://github.com/Fanduzi/DeltaScope/releases/download/v0.7.0/deltascope_0.7.0_checksums.txt",
    archiveName: "deltascope_0.7.0_linux_amd64.tar.gz",
    destinationPath,
    fetchImpl: async (url) => {
      if (url.endsWith(".tar.gz")) {
        return {
          ok: true,
          arrayBuffer: async () => archiveBuffer
        };
      }
      return {
        ok: true,
        text: async () => `${archiveSha}  deltascope_0.7.0_linux_amd64.tar.gz\n`
      };
    }
  });

  assert.equal(extractedPath.binaryPath.endsWith(".tmp-" + process.pid), true);
  assert.equal(extractedPath.archiveChecksum, archiveSha);
  assert.equal(await fs.readFile(extractedPath.binaryPath, "utf8"), "#!/bin/sh\necho launcher-ok\n");
});

test("downloadAndExtractBinary rejects checksum mismatches", async () => {
  const tempDir = await fs.mkdtemp(path.join(os.tmpdir(), "deltascope-mcp-download-"));
  const fixtureDir = path.join(tempDir, "fixture");
  const archivePath = path.join(tempDir, "deltascope_0.7.0_linux_amd64.tar.gz");
  await fs.mkdir(fixtureDir, { recursive: true });
  await fs.writeFile(path.join(fixtureDir, "deltascope-mcp"), "#!/bin/sh\necho launcher-ok\n", { mode: 0o755 });

  const tarResult = spawnSync("tar", ["-czf", archivePath, "-C", fixtureDir, "deltascope-mcp"], {
    stdio: "pipe"
  });
  assert.equal(tarResult.status, 0, tarResult.stderr.toString());

  const archiveBuffer = await fs.readFile(archivePath);
  await assert.rejects(
    () =>
      downloadAndExtractBinary({
        archiveURL: "https://example.com/deltascope_0.7.0_linux_amd64.tar.gz",
        checksumsURL: "https://github.com/Fanduzi/DeltaScope/releases/download/v0.7.0/deltascope_0.7.0_checksums.txt",
        archiveName: "deltascope_0.7.0_linux_amd64.tar.gz",
        destinationPath: path.join(tempDir, "cache", "deltascope-mcp"),
        fetchImpl: async (url) => {
          if (url.endsWith(".tar.gz")) {
            return {
              ok: true,
              arrayBuffer: async () => archiveBuffer
            };
          }
          return {
            ok: true,
            text: async () => `${"0".repeat(64)}  deltascope_0.7.0_linux_amd64.tar.gz\n`
          };
        }
      }),
    /checksum mismatch/i
  );
});
