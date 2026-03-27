import path from "node:path";

export function resolveCacheRoot({ homeDir }) {
  return path.join(homeDir, ".cache", "deltascope-mcp");
}

export function resolveCacheBinaryPath({ homeDir, version, os, arch }) {
  return path.join(resolveCacheRoot({ homeDir }), version, `${os}-${arch}`, "deltascope-mcp");
}
