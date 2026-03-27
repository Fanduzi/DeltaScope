import os from "node:os";

export function resolvePlatform({ platform = os.platform(), arch = os.arch() } = {}) {
  const resolvedPlatform = platform === "darwin" || platform === "linux" ? platform : null;
  if (resolvedPlatform === null) {
    throw new Error(`unsupported platform: ${platform}`);
  }

  const resolvedArch = (() => {
    switch (arch) {
      case "x64":
      case "amd64":
        return "amd64";
      case "arm64":
      case "aarch64":
        return "arm64";
      default:
        return null;
    }
  })();

  if (resolvedArch === null) {
    throw new Error(`unsupported architecture: ${arch}`);
  }

  return { os: resolvedPlatform, arch: resolvedArch };
}

export function resolveDeltaScopeVersion({ packageVersion, envVersion = "" }) {
  const source = envVersion || packageVersion;
  if (!source) {
    throw new Error("could not resolve DeltaScope version");
  }

  return source.startsWith("v") ? source : `v${source}`;
}

export function resolveArchiveName({ version, os, arch }) {
  return `deltascope_${version.replace(/^v/, "")}_${os}_${arch}.tar.gz`;
}

export function resolveArchiveURL({ repo = "Fanduzi/DeltaScope", version, os, arch }) {
  return `https://github.com/${repo}/releases/download/${version}/${resolveArchiveName({ version, os, arch })}`;
}
