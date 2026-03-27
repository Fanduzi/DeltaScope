import fs from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import { spawn } from "node:child_process";

async function runTar(args) {
  const child = spawn("tar", args, { stdio: ["ignore", "pipe", "pipe"] });
  let stderr = "";
  for await (const chunk of child.stderr) {
    stderr += chunk.toString();
  }

  const exitCode = await new Promise((resolve, reject) => {
    child.on("error", reject);
    child.on("close", resolve);
  });

  if (exitCode !== 0) {
    throw new Error(`tar failed: ${stderr.trim()}`);
  }
}

export async function downloadAndExtractBinary({
  archiveURL,
  destinationPath,
  fetchImpl = globalThis.fetch
}) {
  if (typeof fetchImpl !== "function") {
    throw new Error("fetch implementation is required");
  }

  const response = await fetchImpl(archiveURL);
  if (!response.ok) {
    throw new Error(`failed to download ${archiveURL}`);
  }

  const tempDir = await fs.mkdtemp(path.join(os.tmpdir(), "deltascope-mcp-archive-"));
  const archivePath = path.join(tempDir, "archive.tar.gz");
  const extractDir = path.join(tempDir, "extract");

  try {
    await fs.mkdir(extractDir, { recursive: true });
    await fs.writeFile(archivePath, Buffer.from(await response.arrayBuffer()));
    await runTar(["-xzf", archivePath, "-C", extractDir]);
    await fs.mkdir(path.dirname(destinationPath), { recursive: true });
    await fs.copyFile(path.join(extractDir, "deltascope-mcp"), destinationPath);
    await fs.chmod(destinationPath, 0o755);
    return destinationPath;
  } finally {
    await fs.rm(tempDir, { recursive: true, force: true });
  }
}
