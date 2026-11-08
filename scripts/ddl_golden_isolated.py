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
import re
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
NOT_FOUND_RE = re.compile(r"no such|not found|does not exist", re.IGNORECASE)


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
            "generations": [],
            "actions": [],
        }
        # Every subprocess spawned by ddl_golden inherits this environment:
        # compose(), docker exec, health probes, and docker version all pin
        # to the isolated daemon. DOCKER_CONTEXT is cleared so a stray
        # context cannot reroute calls back to the host default — the
        # removal is recorded once it happens, never silently.
        self._cleared_context = os.environ.get("DOCKER_CONTEXT")
        self.env = dict(os.environ)
        self.env["DOCKER_HOST"] = self.daemon
        self.env.pop("DOCKER_CONTEXT", None)

    def record(self, action, rc, detail=""):
        if getattr(self, "_cleared_context", None) is not None:
            self.receipts["actions"].append(
                {"action": "env.docker_context.cleared", "rc": 0,
                 "detail": f"DOCKER_CONTEXT={self._cleared_context!r} removed "
                           "from this task's child env only",
                 "at": utc_stamp()}
            )
            self._cleared_context = None
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

    # -- object inspection: ok / notfound / error are distinct verdicts --

    @staticmethod
    def _verdict(rc, err):
        """Classify an inspect failure: only an explicit NotFound may be
        treated as 'gone'. Daemon unreachable, permission denied, context
        errors and timeouts are 'error' — never idempotent absence."""
        if rc == 0:
            return "ok"
        return "notfound" if NOT_FOUND_RE.search(err or "") else "error"

    def _inspect_container(self, ref):
        try:
            rc, out, err = self.docker(
                "inspect", "--format",
                "{{.Id}}|{{.Name}}|"
                "{{index .Config.Labels \"org.deltascope.run\"}}|"
                "{{.Image}}|{{json .Mounts}}",
                ref, timeout=30,
            )
        except Exception as exc:  # subprocess timeout / OSError
            return {"verdict": "error", "detail": str(exc)}
        verdict = self._verdict(rc, err)
        if verdict != "ok":
            return {"verdict": verdict, "detail": (err or "").strip()}
        parts = out.strip().split("|", 4)
        parts += [""] * (5 - len(parts))
        try:
            mounts = json.loads(parts[4]) if parts[4] else []
        except json.JSONDecodeError:
            mounts = []
        return {"verdict": "ok", "id": parts[0], "name": parts[1].lstrip("/"),
                "label": parts[2], "image": parts[3], "mounts": mounts}

    def _inspect_network(self, nid):
        try:
            rc, out, err = self.docker(
                "network", "inspect", "--format",
                "{{index .Labels \"com.docker.compose.project\"}}",
                nid, timeout=30,
            )
        except Exception as exc:
            return {"verdict": "error", "detail": str(exc)}
        verdict = self._verdict(rc, err)
        if verdict != "ok":
            return {"verdict": verdict, "detail": (err or "").strip()}
        return {"verdict": "ok", "project": out.strip()}

    def _inspect_volume(self, name):
        try:
            rc, out, err = self.docker(
                "volume", "inspect", "--format",
                "{{.Driver}}|{{index .Labels \"org.deltascope.run\"}}",
                name, timeout=30,
            )
        except Exception as exc:
            return {"verdict": "error", "detail": str(exc)}
        verdict = self._verdict(rc, err)
        if verdict != "ok":
            return {"verdict": verdict, "detail": (err or "").strip()}
        parts = out.strip().split("|")
        return {"verdict": "ok", "driver": parts[0],
                "label": parts[1] if len(parts) > 1 else ""}

    # -- daemon binding --------------------------------------------------

    def _capture_daemon_identity(self):
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
        return identity

    def _verify_daemon_binding(self):
        """Teardown only runs against the daemon this run's receipts
        recorded: same endpoint string AND same live server ID. Anything
        else — different endpoint, unreachable daemon, foreign server —
        refuses before a single object is touched."""
        want = self.receipts.get("daemon_endpoint")
        if not want or self.daemon != want:
            self.record("daemon.endpoint.mismatch", 1,
                        f"arg={self.daemon} recorded={want}")
            raise SystemExit(
                f"[iso][FAIL] endpoint {self.daemon!r} does not match "
                f"receipts endpoint {want!r}"
            )
        ident = (self.receipts.get("daemon_identity") or {}).get("server_id")
        try:
            rc, out, err = self.docker("info", "--format", "{{.ID}}", timeout=30)
        except Exception as exc:
            rc, out, err = 1, "", str(exc)
        if rc != 0:
            self.record("daemon.unreachable", 1, (err or "").strip())
            raise SystemExit(
                "[iso][FAIL] cannot verify daemon identity — daemon unreachable"
            )
        live = out.strip()
        if not ident or live != ident:
            self.record("daemon.identity.mismatch", 1,
                        f"recorded={ident} live={live}")
            raise SystemExit(
                "[iso][FAIL] live daemon ID does not match creation-time identity"
            )

    # -- preflight ------------------------------------------------------

    def preflight(self, manifest):
        """Verify daemon identity, capture it, and refuse host daemons."""
        identity = self._capture_daemon_identity()
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

    # -- append-only generation ledger ------------------------------------
    #
    # Every controlled create/up is followed by a capture pass that reads
    # the daemon's real object identities and APPENDS them as a new
    # generation. Teardown consumes only IDs and names the ledger holds;
    # a live object that shares our name and labels but was never captured
    # is refused, never adopted.

    def _generations(self):
        gens = self.receipts.get("generations")
        if gens:
            return gens
        containers = self.receipts.get("containers") or []
        networks = self.receipts.get("networks") or []
        volumes = self.receipts.get("volumes") or []
        if not (containers or networks or volumes):
            return []
        return [{
            "at": self.receipts.get("created_at"), "action": "legacy.capture",
            "containers": list(containers),
            "networks": [{"id": n} if isinstance(n, str) else dict(n)
                         for n in networks],
            "volumes": [{"declared": v["name"], "name": None,
                         "kind": v.get("kind", "named"),
                         "driver": v.get("driver", "local")}
                        for v in volumes],
        }]

    def _registered_container_pairs(self):
        pairs = {}
        for gen in self._generations():
            for c in gen.get("containers", []):
                pairs.setdefault(c["name"], set()).add(c["id"])
        return pairs

    def _registered_network_ids(self):
        ids = set()
        for gen in self._generations():
            for n in gen.get("networks", []):
                ids.add(n["id"] if isinstance(n, dict) else n)
        return ids

    def _registered_volumes(self):
        named, anon = {}, set()
        for gen in self._generations():
            for v in gen.get("volumes", []):
                if v.get("kind") == "anonymous":
                    if v.get("name"):
                        anon.add(v["name"])
                    continue
                decl = v.get("declared") or v.get("name")
                if not decl:
                    continue
                cands = named.setdefault(decl, set())
                if v.get("name"):
                    cands.add(v["name"])
        return named, anon

    def _declared_volume_names(self):
        declared = set(getattr(self, "_volumes", []) or [])
        for gen in self._generations():
            for v in gen.get("volumes", []):
                if v.get("kind") != "anonymous":
                    declared.add(v.get("declared") or v.get("name"))
        return {d for d in declared if d}

    def _classify_volume_name(self, vname):
        for decl in self._declared_volume_names():
            if vname in (decl, f"{self.receipts['project']}_{decl}"):
                return decl
        return None

    def _capture_generation(self, action):
        """Post-create capture: read real identities and append a
        generation record. Old -> new ID transitions stay visible because
        generations are never rewritten."""
        manifest = getattr(self, "_manifest", None)
        gen = {"at": utc_stamp(), "action": action,
               "containers": [], "networks": [], "volumes": []}
        if manifest:
            for anchor in manifest["anchors"].values():
                for name in (anchor.get("container"),
                             anchor.get("client_container")):
                    if not name:
                        continue
                    r = self._inspect_container(name)
                    if r["verdict"] == "error":
                        self.record("capture.error", 1,
                                    f"{name}: {r['detail'][:160]}")
                        continue
                    if r["verdict"] != "ok":
                        continue
                    if r["label"] != self.run_id:
                        self.record("capture.foreign", 1,
                                    f"{name}: label={r['label']!r}")
                        continue
                    gen["containers"].append(
                        {"name": name, "id": r["id"],
                         "image_ref": r["image"]})
                    for mount in r["mounts"]:
                        if mount.get("Type") != "volume":
                            continue
                        vname = mount.get("Name") or ""
                        decl = self._classify_volume_name(vname)
                        if decl:
                            entry = {"kind": "named", "declared": decl,
                                     "name": vname}
                        else:
                            entry = {"kind": "anonymous", "name": vname,
                                     "mounted_by": name,
                                     "destination": mount.get("Destination")}
                        if not any(v.get("name") == vname
                                   for v in gen["volumes"]):
                            gen["volumes"].append(entry)
        rc, nets, err = self.docker(
            "network", "ls", "--filter", f"name={self.run_id}", "-q",
            timeout=30)
        if rc != 0:
            self.record("capture.network-list.error", 1, (err or "").strip())
        else:
            for nid in filter(None, (x.strip() for x in nets.splitlines())):
                v = self._inspect_network(nid)
                if v["verdict"] == "ok" and v["project"] == self.receipts["project"]:
                    gen["networks"].append({"id": nid})
        for decl in self._declared_volume_names():
            if any(v.get("declared") == decl for v in gen["volumes"]):
                continue
            for cand in (f"{self.receipts['project']}_{decl}", decl):
                v = self._inspect_volume(cand)
                if v["verdict"] == "ok" and v["label"] == self.run_id:
                    gen["volumes"].append(
                        {"kind": "named", "declared": decl, "name": cand,
                         "driver": v["driver"]})
                    break
                if v["verdict"] == "ok":
                    self.record("capture.volume-foreign", 1,
                                f"{cand}: label={v['label']!r}")
                    break
        self.receipts.setdefault("generations", []).append(gen)
        # Flat fields mirror the latest generation for readability; the
        # ledger is the authority.
        self.receipts["containers"] = list(gen["containers"])
        self.receipts["networks"] = [n["id"] for n in gen["networks"]]
        self.receipts["volumes"] = [
            {"name": v.get("declared") or v["name"],
             "driver": v.get("driver", "local")}
            for v in gen["volumes"] if v.get("kind") != "anonymous"]
        self.record(
            "generation.capture", 0,
            f"{action}: {len(gen['containers'])}c/"
            f"{len(gen['networks'])}n/{len(gen['volumes'])}v")
        return gen

    # -- lifecycle ------------------------------------------------------

    def up(self, manifest, task):
        self._task = task
        self._manifest = manifest
        self._capture_daemon_identity()
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
        gen = self._capture_generation("compose.up")
        return gen["containers"]

    # -- teardown: consume the ledger only --------------------------------

    def _verified_ids(self):
        """Container IDs authorized for teardown: only IDs the ledger
        registered. A live object holding a registered name with an
        unregistered ID — even when run-labeled — refuses the whole
        teardown rather than being adopted."""
        pairs = self._registered_container_pairs()
        ids, refuse = [], []
        for name, reg_ids in pairs.items():
            for rid in reg_ids:
                r = self._inspect_container(rid)
                if r["verdict"] == "ok":
                    if r["label"] != self.run_id:
                        refuse.append(f"{name}:{rid[:12]} relabeled")
                    else:
                        ids.append(rid)
                elif r["verdict"] == "notfound":
                    self.record("identity.gone", 0, f"{name}:{rid[:12]}")
                else:
                    refuse.append(f"{name}:{rid[:12]} error:{r['detail'][:80]}")
            live = self._inspect_container(name)
            if live["verdict"] == "ok":
                if live["label"] != self.run_id:
                    refuse.append(f"{name} held-by-foreign-label")
                elif live["id"] not in reg_ids:
                    refuse.append(f"{name} unregistered-id {live['id'][:12]}")
            elif live["verdict"] == "error":
                refuse.append(f"{name} error:{live['detail'][:80]}")
        if refuse:
            self.record("identity.refuse", 1, ";".join(refuse))
            raise SystemExit(
                f"[iso][FAIL] ledger identity verification failed: {refuse}")
        return sorted(set(ids))

    def _verified_networks(self):
        """Network IDs authorized for teardown: registered IDs only.
        A live network under this run's name whose ID no generation
        captured is refused, not adopted."""
        registered = self._registered_network_ids()
        project = self.receipts["project"]
        ids, refuse = [], []
        for nid in registered:
            r = self._inspect_network(nid)
            if r["verdict"] == "ok":
                if r["project"] != project:
                    refuse.append(f"net:{nid[:12]} foreign-project")
                else:
                    ids.append(nid)
            elif r["verdict"] == "notfound":
                self.record("identity.gone", 0, f"net:{nid[:12]}")
            else:
                refuse.append(f"net:{nid[:12]} error:{r['detail'][:80]}")
        try:
            rc, listed, err = self.docker(
                "network", "ls", "--filter", f"name={self.run_id}", "-q",
                timeout=30)
        except Exception as exc:
            rc, listed, err = 1, "", str(exc)
        if rc != 0:
            refuse.append(f"network-list error:{(err or '')[:80]}")
        else:
            for nid in filter(None, (x.strip() for x in listed.splitlines())):
                if nid in registered:
                    continue
                r = self._inspect_network(nid)
                if r["verdict"] == "ok" and r["project"] == project:
                    refuse.append(f"net:{nid[:12]} unregistered")
                elif r["verdict"] == "error":
                    refuse.append(f"net:{nid[:12]} error:{r['detail'][:80]}")
        if refuse:
            self.record("network.refuse", 1, ";".join(refuse))
            raise SystemExit(
                f"[iso][FAIL] ledger network verification failed: {refuse}")
        return sorted(set(ids))

    def stop(self):
        self._verify_daemon_binding()
        acted = []
        for cid in self._verified_ids():
            rc, out, err = self.docker("stop", cid, timeout=60)
            self.record(f"stop.{cid[:12]}", rc, err.strip())
            acted.append(cid)
        return acted

    def rm(self):
        self._verify_daemon_binding()
        acted = []
        for cid in self._verified_ids():
            rc, out, err = self.docker("rm", cid, timeout=60)
            self.record(f"rm.{cid[:12]}", rc, err.strip())
            acted.append(cid)
        for nid in self._verified_networks():
            rc, out, err = self.docker("network", "rm", nid, timeout=60)
            self.record(f"network.rm.{nid[:12]}", rc, err.strip())
        return acted

    def rmvolumes(self):
        self._verify_daemon_binding()
        named, anon = self._registered_volumes()
        project = self.receipts["project"]
        for decl, cands in named.items():
            candidates = sorted(cands) + [decl, f"{project}_{decl}"]
            seen = set()
            handled = False
            for cand in candidates:
                if cand in seen:
                    continue
                seen.add(cand)
                v = self._inspect_volume(cand)
                if v["verdict"] == "ok":
                    if v["label"] == self.run_id:
                        rc, out, err = self.docker("volume", "rm", cand,
                                                   timeout=60)
                        self.record(f"volume.rm.{cand}", rc, err.strip())
                    else:
                        self.record(f"volume.retained.{cand}", 0,
                                    f"label={v['label']!r} — not this run")
                    handled = True
                    break
                if v["verdict"] == "error":
                    self.record("volume.refuse", 1,
                                f"{cand}: {v['detail'][:120]}")
                    raise SystemExit(
                        f"[iso][FAIL] volume inspect failed: {cand}")
            if not handled:
                self.record("volume.gone", 0, decl)
        for vname in sorted(anon):
            v = self._inspect_volume(vname)
            if v["verdict"] == "ok":
                rc, out, err = self.docker("volume", "rm", vname, timeout=60)
                self.record(f"volume.rm.{vname}", rc, err.strip())
            elif v["verdict"] == "error":
                self.record("volume.refuse", 1,
                            f"{vname}: {v['detail'][:120]}")
                raise SystemExit(
                    f"[iso][FAIL] volume inspect failed: {vname}")
            else:
                self.record("volume.gone", 0, vname)

    # -- delegation -----------------------------------------------------

    def patched_cleanup(self):
        """Replacement for ddl_golden.compose_cleanup: ledger-scoped only.
        The rc field reports the real stop+rm outcome; it is never set to
        a synthetic value to satisfy a validator."""
        result = {"isolated": True, "stopped": [], "removed": [],
                  "compose_down_rc": 1,
                  "rc_note": "compose_down_rc carries the receipt-scoped "
                             "stop+rm outcome, not a compose-down run"}
        try:
            stopped = self.stop()
            removed = self.rm()
            result["stopped"] = [i[:12] for i in stopped]
            result["removed"] = [i[:12] for i in removed]
            result["compose_down_rc"] = 0
        except SystemExit as exc:
            result["error"] = str(exc)
        try:
            rc, out, _ = self.docker(
                "ps", "-aq", "--filter",
                f"label=org.deltascope.run={self.run_id}", timeout=30)
            result["residual_containers"] = (
                [c for c in out.splitlines() if c.strip()] if rc == 0
                else ["<unverifiable>"])
        except Exception:
            result["residual_containers"] = ["<unverifiable>"]
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
        # by ledger-scoped teardown. No other behavior is altered.
        run._task = args.task
        run._manifest = manifest
        run.write_override(manifest)
        os.environ["DOCKER_HOST"] = run.daemon
        os.environ.pop("DOCKER_CONTEXT", None)
        dg.COMPOSE_PROJECT = run.run_id

        # Every controlled compose up is followed by an append-only
        # generation capture, so mid-run recreations enter the ledger as
        # new generations instead of orphaning receipt IDs.
        base_compose = run.compose

        def ledger_compose(*cargs, **ckw):
            result = base_compose(*cargs, **ckw)
            if cargs and cargs[0] == "up" and result[0] == 0:
                run._capture_generation("compose.up.run")
            return result

        dg.compose = ledger_compose
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
            def gap_compose(*cargs, **ckw):
                if cargs and cargs[0] == "up":
                    cargs = tuple(a for a in cargs if a not in gap_services)
                return ledger_compose(*cargs, **ckw)

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
        archive_attempt(run.artifact_dir, args.task)


def archive_attempt(artifact_dir, task):
    """Snapshot golden/<task> into a unique attempt directory —
    <task>.attempts/<head12>/<attempt-N>/. Refuses silent reuse: the next
    free index is computed and an existing directory is never
    overwritten. A SHA256SUMS manifest and a 'latest' index file follow
    so a re-run preserves history instead of clobbering the prior
    artifact (issue #101 T22 R5 finding)."""
    import hashlib
    import shutil

    golden = artifact_dir / "golden" / task
    if not golden.is_dir():
        return
    head = "unknown-head"
    artifact = golden / "artifact.json"
    if artifact.is_file():
        try:
            payload = json.loads(artifact.read_text())
            head = (payload.get("head_sha") or payload.get("head") or "unknown-head")[:12]
        except Exception:
            pass
    attempts = artifact_dir / "golden" / f"{task}.attempts" / head
    attempts.mkdir(parents=True, exist_ok=True)
    index = 0
    while True:
        index += 1
        dest = attempts / f"attempt-{index}"
        if not dest.exists():
            break
    dest.mkdir()
    for item in sorted(golden.iterdir()):
        if item.name.startswith("bin"):
            continue
        if item.is_dir():
            shutil.copytree(item, dest / item.name)
        else:
            shutil.copy2(item, dest / item.name)
    sums = []
    for path in sorted(dest.rglob("*")):
        if path.is_file() and path.name != "SHA256SUMS":
            sums.append(f"{hashlib.sha256(path.read_bytes()).hexdigest()}  {path.relative_to(dest)}\n")
    (dest / "SHA256SUMS").write_text("".join(sums))
    (attempts / "latest").write_text(dest.name + "\n")
    print(f"[attempt] archived {task} evidence -> {dest} ({len(sums)} files hashed)")


if __name__ == "__main__":
    main()
