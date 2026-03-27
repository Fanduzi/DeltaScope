import fs from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import { spawn } from "node:child_process";
import crypto from "node:crypto";

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

function sha256(buffer) {
  return crypto.createHash("sha256").update(buffer).digest("hex");
}

function parseChecksums(text) {
  const map = new Map();
  for (const rawLine of text.split(/\r?\n/)) {
    const line = rawLine.trim();
    if (!line) {
      continue;
    }
    const match = line.match(/^([a-fA-F0-9]{64})\s+\*?(.+)$/);
    if (!match) {
      continue;
    }
    map.set(match[2].trim(), match[1].toLowerCase());
  }
  return map;
}

export async function downloadAndExtractBinary({
  archiveURL,
  checksumsURL,
  archiveName,
  destinationPath,
  fetchImpl = globalThis.fetch
}) {
  if (typeof fetchImpl !== "function") {
    throw new Error("fetch implementation is required");
  }

  const [archiveResponse, checksumsResponse] = await Promise.all([
    fetchImpl(archiveURL),
    fetchImpl(checksumsURL)
  ]);

  if (!archiveResponse.ok) {
    throw new Error(`failed to download ${archiveURL}`);
  }
  if (!checksumsResponse.ok) {
    throw new Error(`failed to download ${checksumsURL}`);
  }

  const tempDir = await fs.mkdtemp(path.join(os.tmpdir(), "deltascope-mcp-archive-"));
  const archivePath = path.join(tempDir, "archive.tar.gz");
  const extractDir = path.join(tempDir, "extract");
  const archiveBuffer = Buffer.from(await archiveResponse.arrayBuffer());
  const checksums = parseChecksums(await checksumsResponse.text());
  const expectedChecksum = checksums.get(archiveName);
  const actualChecksum = sha256(archiveBuffer);

  try {
    if (!expectedChecksum) {
      throw new Error(`missing checksum for ${archiveName}`);
    }
    if (actualChecksum !== expectedChecksum) {
      throw new Error(`checksum mismatch for ${archiveName}`);
    }
    await fs.mkdir(extractDir, { recursive: true });
    await fs.writeFile(archivePath, archiveBuffer);
    await runTar(["-xzf", archivePath, "-C", extractDir]);
    await fs.mkdir(path.dirname(destinationPath), { recursive: true });
    const tempBinaryPath = `${destinationPath}.tmp-${process.pid}`;
    await fs.copyFile(path.join(extractDir, "deltascope-mcp"), tempBinaryPath);
    await fs.chmod(tempBinaryPath, 0o755);
    return {
      binaryPath: tempBinaryPath,
      archiveChecksum: expectedChecksum
    };
  } finally {
    await fs.rm(tempDir, { recursive: true, force: true });
  }
}
