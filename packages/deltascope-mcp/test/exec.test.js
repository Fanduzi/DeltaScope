import test from "node:test";
import assert from "node:assert/strict";
import fs from "node:fs/promises";
import os from "node:os";
import path from "node:path";

import { spawnBinary } from "../lib/launcher.js";

test("spawnBinary starts the native executable and forwards arguments", async () => {
  const tempDir = await fs.mkdtemp(path.join(os.tmpdir(), "deltascope-mcp-exec-"));
  const binaryPath = path.join(tempDir, "deltascope-mcp");
  await fs.writeFile(
    binaryPath,
    "#!/bin/sh\nprintf 'args:%s\\n' \"$*\"\n",
    { mode: 0o755 }
  );

  const child = spawnBinary(binaryPath, ["-connections-path", "/tmp/connections.yaml"], {
    stdio: ["ignore", "pipe", "pipe"]
  });

  let stdout = "";
  for await (const chunk of child.stdout) {
    stdout += chunk.toString();
  }

  const exitCode = await new Promise((resolve, reject) => {
    child.on("error", reject);
    child.on("close", resolve);
  });

  assert.equal(exitCode, 0);
  assert.equal(stdout.trim(), "args:-connections-path /tmp/connections.yaml");
});
