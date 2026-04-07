#!/usr/bin/env node

import fs from "node:fs/promises";
import process from "node:process";

import { downloadAndExtractBinary } from "../lib/download.js";
import {
  ensureExecutable,
  formatBootstrapContext,
  spawnBinary,
} from "../lib/launcher.js";
import {
  resolveArchiveName,
  resolveArchiveURL,
  resolveChecksumsURL,
  resolveDeltaScopeVersion,
  resolvePlatform,
} from "../lib/releases.js";

const packageJson = JSON.parse(
  await fs.readFile(new URL("../package.json", import.meta.url), "utf8"),
);
const version = resolveDeltaScopeVersion({
  packageVersion: packageJson.version,
  envVersion: process.env.DELTASCOPE_MCP_VERSION ?? "",
});
const platform = resolvePlatform();
const archiveURL = resolveArchiveURL({
  baseURL: process.env.DELTASCOPE_MCP_BASE_URL ?? "",
  version,
  os: platform.os,
  arch: platform.arch,
});
const archiveName = resolveArchiveName({
  version,
  os: platform.os,
  arch: platform.arch,
});
const checksumsURL = resolveChecksumsURL({ version });
const preferredChecksumsURL = resolveChecksumsURL({
  version,
  os: platform.os,
  arch: platform.arch,
});
const checksumsURLs = [...new Set([preferredChecksumsURL, checksumsURL])];

function log(message) {
  process.stderr.write(`[deltascope-mcp-launcher] ${message}\n`);
}

log(`resolved DeltaScope version ${version}`);
log(`detected platform ${platform.os}-${platform.arch}`);

const binaryPath = await ensureExecutable({
  version,
  platform,
  archiveURL,
  checksumsURL,
  downloadBinary: (destinationPath) =>
    (async () => {
      log(`cache miss; downloading ${archiveURL}`);
      log(`cache target ${destinationPath}`);
      log(
        checksumsURLs.length > 1
          ? `verifying archive against ${checksumsURLs[0]} (fallback ${checksumsURLs[1]})`
          : `verifying archive against ${checksumsURLs[0]}`,
      );
      try {
        const result = await downloadAndExtractBinary({
          archiveURL,
          checksumsURL: preferredChecksumsURL,
          checksumsURLs,
          archiveName,
          destinationPath,
        });
        log(
          `downloaded archive and staged native binary for ${destinationPath}`,
        );
        return result;
      } catch (error) {
        process.stderr.write(
          `${formatBootstrapContext({
            version,
            platform,
            archiveURL,
            destinationPath,
          })}\n`,
        );
        throw error;
      }
    })(),
});

log(`launching native binary ${binaryPath}`);

const child = spawnBinary(binaryPath, process.argv.slice(2), {
  stdio: "inherit",
  env: process.env,
});

child.on("error", (error) => {
  process.stderr.write(`deltascope-mcp launcher: ${error.message}\n`);
  process.exit(1);
});

child.on("close", (code) => {
  process.exit(code ?? 1);
});
