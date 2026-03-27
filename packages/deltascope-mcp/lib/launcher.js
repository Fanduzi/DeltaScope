import fs from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import { spawn } from "node:child_process";

import { resolveCacheBinaryPath, resolveCacheMetadataPath } from "./cache.js";
import { resolvePlatform } from "./releases.js";

async function fileExists(targetPath) {
  try {
    await fs.access(targetPath);
    return true;
  } catch {
    return false;
  }
}

async function readJSON(pathname) {
  try {
    return JSON.parse(await fs.readFile(pathname, "utf8"));
  } catch {
    return null;
  }
}

async function acquireLock(lockDir, { staleLockMs = 60000, lockTimeoutMs = 10000, lockRetryDelayMs = 100 } = {}) {
  const startedAt = Date.now();
  for (;;) {
    try {
      await fs.mkdir(lockDir);
      return;
    } catch (error) {
      if (error && error.code === "EEXIST") {
        try {
          const stat = await fs.stat(lockDir);
          if (Date.now() - stat.mtimeMs > staleLockMs) {
            await fs.rm(lockDir, { recursive: true, force: true });
            continue;
          }
        } catch {
          continue;
        }
        if (Date.now() - startedAt > lockTimeoutMs) {
          throw new Error("timed out waiting for launcher cache lock");
        }
        await new Promise((resolve) => setTimeout(resolve, lockRetryDelayMs));
        continue;
      }
      throw error;
    }
  }
}

async function releaseLock(lockDir) {
  await fs.rm(lockDir, { recursive: true, force: true });
}

export async function ensureExecutable({
  version,
  homeDir = os.homedir(),
  platform = resolvePlatform(),
  archiveURL = "",
  checksumsURL = "",
  staleLockMs,
  lockTimeoutMs,
  lockRetryDelayMs,
  downloadBinary
}) {
  const binaryPath = resolveCacheBinaryPath({
    homeDir,
    version,
    os: platform.os,
    arch: platform.arch
  });
  const metadataPath = resolveCacheMetadataPath({
    homeDir,
    version,
    os: platform.os,
    arch: platform.arch
  });
  const cacheDir = path.dirname(binaryPath);
  const lockDir = path.join(cacheDir, ".lock");
  const expectedMetadata = {
    version,
    os: platform.os,
    arch: platform.arch,
    archiveURL,
    checksumsURL
  };

  const isCacheValid = async () => {
    if (!(await fileExists(binaryPath))) {
      return false;
    }
    const metadata = await readJSON(metadataPath);
    if (!metadata) {
      return false;
    }
    return Object.entries(expectedMetadata).every(([key, value]) => metadata[key] === value);
  };

  if (await isCacheValid()) {
    return binaryPath;
  }

  if (typeof downloadBinary !== "function") {
    throw new Error("downloadBinary is required when the DeltaScope MCP binary is not cached");
  }

  await fs.mkdir(cacheDir, { recursive: true });
  await acquireLock(lockDir, {
    staleLockMs,
    lockTimeoutMs,
    lockRetryDelayMs
  });
  try {
    if (await isCacheValid()) {
      return binaryPath;
    }

    const { binaryPath: downloadedBinaryPath, archiveChecksum } = await downloadBinary(binaryPath);
    const tempMetadataPath = `${metadataPath}.tmp-${process.pid}`;
    const finalMetadata = {
      ...expectedMetadata,
      archiveChecksum
    };
    await fs.writeFile(tempMetadataPath, `${JSON.stringify(finalMetadata, null, 2)}\n`);
    await fs.rename(downloadedBinaryPath, binaryPath);
    await fs.rename(tempMetadataPath, metadataPath);
    return binaryPath;
  } finally {
    await releaseLock(lockDir);
  }
}

export function spawnBinary(binaryPath, args = [], options = {}) {
  return spawn(binaryPath, args, options);
}

export function formatBootstrapContext({ version, platform, archiveURL, destinationPath }) {
  return [
    `DeltaScope MCP launcher context:`,
    `  version: ${version}`,
    `  platform: ${platform.os}-${platform.arch}`,
    `  archive: ${archiveURL}`,
    `  cache target: ${destinationPath}`,
    `  checksums: always verified against the official GitHub release checksums file`,
    `  hint: if your network requires a proxy, set HTTP_PROXY / HTTPS_PROXY and NODE_USE_ENV_PROXY=1`
  ].join("\n");
}
