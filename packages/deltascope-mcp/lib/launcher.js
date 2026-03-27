import fs from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import { spawn } from "node:child_process";

import { resolveCacheBinaryPath } from "./cache.js";
import { resolvePlatform } from "./releases.js";

async function fileExists(targetPath) {
  try {
    await fs.access(targetPath);
    return true;
  } catch {
    return false;
  }
}

export async function ensureExecutable({
  version,
  homeDir = os.homedir(),
  platform = resolvePlatform(),
  downloadBinary
}) {
  const binaryPath = resolveCacheBinaryPath({
    homeDir,
    version,
    os: platform.os,
    arch: platform.arch
  });

  if (await fileExists(binaryPath)) {
    return binaryPath;
  }

  if (typeof downloadBinary !== "function") {
    throw new Error("downloadBinary is required when the DeltaScope MCP binary is not cached");
  }

  await fs.mkdir(path.dirname(binaryPath), { recursive: true });
  return downloadBinary(binaryPath);
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
    `  hint: if your network requires a proxy, set HTTP_PROXY / HTTPS_PROXY and NODE_USE_ENV_PROXY=1`
  ].join("\n");
}
