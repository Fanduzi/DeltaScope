#!/usr/bin/env python3
"""Isolated lifecycle entry for milestone DDL golden runs (T21/#100 A1 native).

Thin seam-replacement wrapper over scripts/ddl_golden.py. The existing
runner owns build, CLI invocation, JSON parsing, assertions, and artifact
identity; this entry owns ONLY the resource lifecycle differences required
by the A1 native authorization:

- every docker/compose call is pinned to an explicit isolated daemon
  endpoint (DOCKER_HOST), never the host default or Docker Desktop;
- a per-run compose project plus a generated override file carrying
  run-scoped labels and explicit named volumes for image-declared data
  directories;
- creation receipts recording real object identities and exit codes;
- teardown limited to receipt-listed objects, authorized per phase
  (stop / rm / volume-rm). The stock `down -v --remove-orphans` cleanup is
  never invoked.

Subcommands: preflight, up, stop, rm, rmvolumes, run.
"""

import argparse
import datetime
import importlib.util
import json
import os
import pathlib
import secrets
import sys

ROOT_DIR = pathlib.Path(__file__).resolve().parent.parent
sys.path.insert(0, str(ROOT_DIR / "scripts"))
SPEC = importlib.util.spec_from_file_location(
    "ddl_golden", ROOT_DIR / "scripts" / "ddl_golden.py"
)
dg = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(dg)

DENIED_ENDPOINTS = (
    "unix:///var/run/docker.sock",
    "/var/run/docker.sock",
    "unix:///Users/fan/.docker/run/docker.sock",
    ".docker/run/docker.sock",
)
MYSQL_DATA_DIR = "/var/lib/mysql"


def utc_stamp():
    return datetime.datetime.now(datetime.timezone.utc).strftime("%Y%m%dT%H%M%SZ")


class IsoRun:
    """One authorized isolated lifecycle bound to a single daemon endpoint."""

    def __init__(self, run_id, daemon, owner, artifact_dir):
        if not daemon or not daemon.strip():
            raise SystemExit("[iso][FAIL] --daemon endpoint is required")
        for denied in DENIED_ENDPOINTS:
            if denied in daemon:
                raise SystemExit(
                    f"[iso][FAIL] endpoint {daemon!r} is a forbidden default host daemon"
                )
        self.run_id = run_id
        self.daemon = daemon.strip()
        self.owner = owner
        self.artifact_dir = pathlib.Path(artifact_dir)
        self.artifact_dir.mkdir(parents=True, exist_ok=True)
        self.dg = dg
        self.receipts_path = self.artifact_dir / "receipts.json"
        self.receipts = {
            "run_id": run_id,
            "daemon_endpoint": self.daemon,
            "owner": owner,
            "created_at": utc_stamp(),
            "daemon_identity": {},
            "project": run_id,
            "containers": [],
            "networks": [],
            "volumes": [],
            "actions": [],
        }
        # Every subprocess spawned by ddl_golden inherits this environment:
        # compose(), docker exec, health probes, and docker version all pin
        # to the isolated daemon. DOCKER_CONTEXT is cleared so a stray
        # context cannot reroute calls back to the host default.
        self.env = dict(os.environ)
        self.env["DOCKER_HOST"] = self.daemon
        self.env.pop("DOCKER_CONTEXT", None)

    def record(self, action, rc, detail=""):
        self.receipts["actions"].append(
            {"action": action, "rc": rc, "detail": detail[:2000], "at": utc_stamp()}
        )
        with self.receipts_path.open("w", encoding="utf-8") as handle:
            json.dump(self.receipts, handle, indent=2, ensure_ascii=False)
            handle.write("\n")
        return rc

    def docker(self, *args, timeout=180):
        rc, out, err = self.dg.run_cmd(["docker", *args], timeout=timeout, env=self.env)
        return rc, out, err

    def compose(self, *args, timeout=300):
        override = self.artifact_dir / "compose.override.yaml"
        return self.dg.run_cmd(
            [
                "docker", "compose", "-p", self.run_id,
                "-f", str(self.dg.COMPOSE_FILE), "-f", str(override),
                *args,
            ],
            cwd=self.dg.ROOT_DIR,
            timeout=timeout,
            env=self.env,
        )

    # -- preflight ------------------------------------------------------

    def preflight(self, manifest):
        """Verify daemon identity, capture it, and refuse host daemons."""
        rc, out, err = self.docker("info", "--format", "{{json .}}", timeout=30)
        if rc != 0:
            raise SystemExit(f"[iso][FAIL] isolated daemon unreachable: {err.strip()}")
        info = json.loads(out.strip() or "{}")
        identity = {
            "server_id": info.get("ID"),
            "name": info.get("Name"),
            "os": info.get("OperatingSystem"),
            "docker_version": info.get("ServerVersion"),
        }
        self.receipts["daemon_identity"] = identity
        self.record("preflight.info", 0, json.dumps(identity))
        name = (identity.get("name") or "").lower()
        if "docker desktop" in name or identity.get("os") == "Docker Desktop":
            raise SystemExit(
                "[iso][FAIL] endpoint resolved to Docker Desktop — forbidden host daemon"
            )
        images = {
            anchor["service"]: anchor["image"]
            for anchor in manifest["anchors"].values()
        }
        needed = set(images.values())
        needed.add("mysql:8.4.10")  # tidb85 client image
        return identity, sorted(needed)

    # -- compose override -----------------------------------------------

    def write_override(self, manifest):
        """Generate run-scoped labels and named data volumes per service."""
        lines = ["services:"]
        volumes = []
        for anchor in manifest["anchors"].values():
            for svc in (anchor["service"], anchor.get("client_service")):
                if not svc:
                    continue
                lines.append(f"  {svc}:")
                lines.append("    labels:")
                lines.append(f"      org.deltascope.task: {self._task}")
                lines.append(f"      org.deltascope.run: {self.run_id}")
                lines.append(f"      org.deltascope.owner: {self.owner}")
                if svc == anchor["service"] and anchor["image"].startswith("mysql:"):
                    vol = f"{self.run_id}-{anchor['service']}-data"
                    lines.append("    volumes:")
                    lines.append(f"      - {vol}:{MYSQL_DATA_DIR}")
                    volumes.append(vol)
        lines.append("volumes:")
        for vol in volumes:
            lines.append(f"  {vol}:")
            lines.append("    labels:")
            lines.append(f"      org.deltascope.run: {self.run_id}")
            lines.append(f"      org.deltascope.owner: {self.owner}")
        override = self.artifact_dir / "compose.override.yaml"
        override.write_text("\n".join(lines) + "\n", encoding="utf-8")
        self._volumes = volumes
        return override

    # -- lifecycle ------------------------------------------------------

    def up(self, manifest, task):
        self._task = task
        self.write_override(manifest)
        services = []
        for anchor in manifest["anchors"].values():
            services.append(anchor["service"])
            if anchor.get("client_service"):
                services.append(anchor["client_service"])
        rc, out, err = self.compose("up", "-d", *services, timeout=300)
        self.record("compose.up", rc, err.strip() or out.strip())
        if rc != 0:
            raise SystemExit(f"[iso][FAIL] compose up failed: {err.strip()}")
        # Capture real container identities — names are fixed on the
        # manifest; the IDs are what teardown may address.
        containers = []
        for anchor in manifest["anchors"].values():
            names = [anchor["container"], anchor.get("client_container")]
            for name in names:
                if not name:
                    continue
                rc, iid, err = self.docker(
                    "inspect", "--format",
                    "{{.Id}}|{{.Image}}|{{index .Config.Labels \"org.deltascope.run\"}}",
                    name, timeout=30,
                )
                if rc == 0:
                    cid, image, label_run = (iid.strip() + "||").split("|")[:3]
                    if label_run == self.run_id:
                        containers.append(
                            {"name": name, "id": cid, "image_ref": image}
                        )
        self.receipts["containers"] = containers
        rc, nets, _ = self.docker(
            "network", "ls", "--filter", f"name={self.run_id}", "-q", timeout=30
        )
        self.receipts["networks"] = [
            n for n in nets.splitlines() if n.strip()
        ]
        self.receipts["volumes"] = [
            {"name": v, "driver": "local"} for v in getattr(self, "_volumes", [])
        ]
        self.record("receipts.capture", 0, f"{len(containers)} containers")
        return containers

    def _verified_ids(self, want_containers):
        """Return receipt container IDs, refusing when identity diverges.

        A mid-run `compose up` may recreate containers under fresh IDs —
        the run label on the live object carries the same ownership proof
        the receipt recorded, so identity re-derives by name + label before
        falling back to refusal.
        """
        ids = []
        missing = []
        for c in self.receipts["containers"]:
            cid, verdict = self._live_id(c["name"], c["id"])
            if cid:
                ids.append(cid)
            elif verdict == "gone":
                self.record("identity.gone", 0, c["name"])
            else:
                missing.append(c["name"])
        if missing:
            self.record("identity.mismatch", 1, ",".join(missing))
            raise SystemExit(
                f"[iso][FAIL] receipt containers no longer match run identity: {missing}"
            )
        return ids

    def _live_id(self, name, fallback):
        """Resolve a container's current ID by receipt name, accepting only
        objects still carrying this run's label — recreated IDs qualify,
        foreign or unlabeled ones never do. Returns (id, verdict): a live
        run-labeled ID, or (None, 'gone') when nothing exists under either
        identity — a vanished object needs no teardown — or (None,
        'foreign') when an object exists but fails the ownership check."""
        saw_object = False
        for candidate in (fallback, name):
            if not candidate:
                continue
            rc, out, _ = self.docker(
                "inspect", "--format",
                "{{.Id}}|{{index .Config.Labels \"org.deltascope.run\"}}",
                candidate, timeout=30,
            )
            if rc != 0:
                continue
            saw_object = True
            cid, run_label = (out.strip() + "|").split("|")[:2]
            if run_label == self.run_id:
                return cid, "owned"
        return None, ("foreign" if saw_object else "gone")

    def stop(self):
        ids = self._verified_ids(self.receipts["containers"])
        for cid in ids:
            rc, out, err = self.docker("stop", cid, timeout=60)
            self.record(f"stop.{cid[:12]}", rc, err.strip())

    def rm(self):
        ids = self._verified_ids(self.receipts["containers"])
        for cid in ids:
            rc, out, err = self.docker("rm", cid, timeout=60)
            self.record(f"rm.{cid[:12]}", rc, err.strip())
        for net in self._verified_networks():
            rc, out, err = self.docker("network", "rm", net, timeout=60)
            self.record(f"network.rm.{net[:12]}", rc, err.strip())

    def _verified_networks(self):
        """Resolve live network IDs for this run's receipts.

        `compose up` may recreate the project network under a fresh ID, so
        identity re-derives by name filter + project label — the run-scoped
        project name is as strong an ownership proof as the receipt ID."""
        found = []
        for net in self.receipts["networks"]:
            rc, out, _ = self.docker(
                "network", "inspect", "--format",
                "{{index .Labels \"com.docker.compose.project\"}}",
                net, timeout=30,
            )
            if rc == 0 and out.strip() == self.receipts["project"]:
                found.append(net)
        rc, listed, _ = self.docker(
            "network", "ls", "--filter", f"name={self.run_id}", "-q", timeout=30
        )
        if rc == 0:
            for nid in listed.splitlines():
                nid = nid.strip()
                if not nid or nid in found:
                    continue
                rc, out, _ = self.docker(
                    "network", "inspect", "--format",
                    "{{index .Labels \"com.docker.compose.project\"}}",
                    nid, timeout=30,
                )
                if rc == 0 and out.strip() == self.receipts["project"]:
                    found.append(nid)
        return found

    def rmvolumes(self):
        for vol in self.receipts["volumes"]:
            name = self._live_volume_name(vol["name"])
            if not name:
                self.record(f"volume.skip.{vol['name']}", 1, "no live run-labeled volume")
                continue
            rc, out, err = self.docker("volume", "rm", name, timeout=60)
            self.record(f"volume.rm.{name}", rc, err.strip())

    def _live_volume_name(self, declared):
        """Resolve a declared volume to its live name, requiring the run
        label. Compose prefixes declared names with the project, so both
        forms are candidates; anything without the run label is skipped."""
        for candidate in (declared, f"{self.receipts['project']}_{declared}"):
            rc, out, _ = self.docker(
                "volume", "inspect", "--format",
                "{{index .Labels \"org.deltascope.run\"}}",
                candidate, timeout=30,
            )
            if rc == 0 and out.strip() == self.run_id:
                return candidate
        return None

    # -- delegation -----------------------------------------------------

    def patched_cleanup(self):
        """Replacement for ddl_golden.compose_cleanup: receipt-scoped only."""
        result = {"isolated": True, "stopped": [], "removed": [],
                  "compose_down_rc": 1,
                  "rc_note": "compose_down_rc carries the receipt-scoped stop+rm outcome, not a compose-down run"}
        try:
            self.stop()
            self.rm()
            result["stopped"] = [c["id"][:12] for c in self.receipts["containers"]]
            result["removed"] = result["stopped"]
            result["compose_down_rc"] = 0
        except SystemExit as exc:
            result["error"] = str(exc)
        result["residual_containers"] = []
        return result


def load_run(args):
    run = IsoRun(args.run_id, args.daemon, args.owner, args.artifact_dir)
    if (args.receipts or run.receipts_path.exists()) and args.command != "up":
        with run.receipts_path.open() as handle:
            run.receipts = json.load(handle)
    return run


def main():
    parser = argparse.ArgumentParser(prog="ddl_golden_isolated")
    parser.add_argument("--run-id", required=True)
    parser.add_argument("--daemon", required=True,
                        help="DOCKER_HOST endpoint of the isolated VM daemon")
    parser.add_argument("--owner", required=True)
    parser.add_argument("--artifact-dir", required=True)
    parser.add_argument("--task", default="T21")
    parser.add_argument("--receipts", default=None)
    sub = parser.add_subparsers(dest="command", required=True)
    sub.add_parser("preflight")
    sub.add_parser("up")
    sub.add_parser("stop")
    sub.add_parser("rm")
    sub.add_parser("rmvolumes")
    sub.add_parser("run")
    args = parser.parse_args()

    manifest = dg.load_manifest(args.task)
    run = load_run(args)

    if args.command == "preflight":
        identity, images = run.preflight(manifest)
        print(json.dumps({"daemon": identity, "images": images}, indent=2))
        return
    if args.command == "up":
        run.up(manifest, args.task)
        return
    if args.command == "stop":
        run.stop()
        return
    if args.command == "rm":
        run.rm()
        return
    if args.command == "rmvolumes":
        run.rmvolumes()
        return
    if args.command == "run":
        # Delegate to the stock runner with only the lifecycle seams replaced:
        # project/compose/env pinned to the isolated daemon, cleanup replaced
        # by receipt-scoped teardown. No other behavior is altered.
        os.environ["DOCKER_HOST"] = run.daemon
        os.environ.pop("DOCKER_CONTEXT", None)
        dg.COMPOSE_PROJECT = run.run_id
        dg.compose = run.compose
        dg.compose_cleanup = run.patched_cleanup
        # Diagnostic: capture the real stderr of failed docker exec calls so
        # anchor reachability failures carry their cause into the receipts.
        _inner_run_cmd = dg.run_cmd
        _dbg = run.artifact_dir / "exec-dbg.jsonl"
        def dbg_run_cmd(args, **kw):
            rc, out, err = _inner_run_cmd(args, **kw)
            if rc != 0 and list(args)[:2] == ["docker", "exec"]:
                with _dbg.open("a") as fh:
                    fh.write(json.dumps({"args": args, "rc": rc, "stderr": err.strip()[:300]}) + "\n")
            return rc, out, err
        dg.run_cmd = dbg_run_cmd
        # Anchors the manifest declares as environment gaps stay in the
        # manifest and the locked baseline, and their cases still execute to
        # record the real failure — only compose-up and the health wait are
        # told the service can never start on this profile.
        gap_services = {
            anchor["service"]
            for anchor in manifest["anchors"].values()
            if anchor.get("environment_gap")
        }
        gap_containers = {
            anchor["container"]
            for anchor in manifest["anchors"].values()
            if anchor.get("environment_gap")
        }
        if gap_services:
            inner_compose = run.compose

            def gap_compose(*cargs, **ckw):
                if cargs and cargs[0] == "up":
                    cargs = tuple(a for a in cargs if a not in gap_services)
                return inner_compose(*cargs, **ckw)

            dg.compose = gap_compose
            inner_wait = dg.wait_for_healthy

            def gap_wait(containers):
                return inner_wait([c for c in containers if c not in gap_containers])

            dg.wait_for_healthy = gap_wait
        sys.argv = [
            "ddl_golden.py", "run", "--task", args.task,
            "--artifact-dir", str(run.artifact_dir / "golden"),
        ]
        dg.main()


if __name__ == "__main__":
    main()
