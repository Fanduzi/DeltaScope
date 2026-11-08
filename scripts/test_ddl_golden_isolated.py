#!/usr/bin/env python3
"""Offline contract tests for the isolated golden lifecycle entry.

No Docker daemon is contacted: IsoRun.docker is replaced by a stateful
fake daemon, and every assertion inspects the argv/env the entry would
have issued plus the verdict the ledger reached. Covers the
authorization seams: endpoint pinning, project scoping, append-only
generation ledger, refuse-mode teardown, and the absence of the stock
`down -v --remove-orphans` path.
"""

import copy
import importlib.util
import json
import os
import pathlib
import subprocess
import sys
import tempfile
import time
import types
import unittest
import unittest.mock

ROOT = pathlib.Path(__file__).resolve().parent.parent
sys.path.insert(0, str(ROOT / "scripts"))

SPEC = importlib.util.spec_from_file_location(
    "ddl_golden_isolated", ROOT / "scripts" / "ddl_golden_isolated.py"
)
iso = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(iso)

MANIFEST = {
    "task_id": "T21",
    "anchors": {
        "mysql57": {
            "service": "mysql57", "container": "c-mysql57",
            "image": "mysql:5.7.44",
        },
        "tidb85": {
            "service": "tidb85", "container": "c-tidb85",
            "client_service": "tidb85-client",
            "client_container": "c-tidb85-client",
            "image": "pingcap/tidb:v8.5.0",
        },
    },
}

DAEMON = "unix:///tmp/iso-vm/docker.sock"
RUN = "ds-t21-20990101T000000Z-deadbeef"
SID = "srv-feedface00112233"


class FakeDaemon:
    """Stateful fake for IsoRun.docker: object stores plus argv logging.

    containers: name -> {"id", "label", "image", "mounts"}
    networks:   id   -> compose project name
    volumes:    name -> {"driver", "label"}
    Inspect accepts either a container name or an ID.
    """

    def __init__(self, server_id=SID):
        self.server_id = server_id
        self.containers = {}
        self.networks = {}
        self.volumes = {}
        self.calls = []
        # Fault hooks used by the negative-control groups:
        # fail_next_up      -> simulate_up() reports compose failure once
        # error_containers  -> names answering a non-NotFound inspect error
        # inspect_error_once-> first inspect call errors once, then clears
        self.fail_next_up = False
        self.error_containers = set()
        self.inspect_error_once = False
        # ps --filter volume=<name> fails for these volume names
        self.ps_error_volumes = set()
        # docker volume rm <name> fails for these volume names
        self.fail_volume_rm = set()

    def bind(self, run):
        run.docker = lambda *a, timeout=180: self.dispatch(
            ["docker", *a], env=run.env)
        return self

    def add_container(self, name, cid, label=RUN, image="img", mounts=None):
        self.containers[name] = {
            "id": cid, "label": label, "image": image,
            "mounts": mounts or []}

    def _find_container(self, ref):
        if ref in self.containers:
            return ref, self.containers[ref]
        for name, c in self.containers.items():
            if c["id"] == ref:
                return name, c
        return None, None

    def dispatch(self, args, env=None):
        self.calls.append(list(args))
        a = args[1:]
        verb = a[0] if a else ""
        if verb == "info":
            if any("json" in x for x in a):
                return 0, json.dumps({
                    "ID": self.server_id, "Name": "iso-vm",
                    "OperatingSystem": "Ubuntu 24.04",
                    "ServerVersion": "29.5.2"}), ""
            return 0, self.server_id, ""
        if verb == "inspect":
            if self.inspect_error_once:
                self.inspect_error_once = False
                return 1, "", "daemon took too long to respond"
            name, c = self._find_container(a[-1])
            if c is None:
                return 1, "", f"Error: No such container: {a[-1]}"
            if name in self.error_containers:
                return 1, "", "permission denied on container object"
            return 0, "|".join([
                c["id"], name, c["label"], c["image"],
                json.dumps(c["mounts"])]), ""
        if verb == "network" and a[1] == "inspect":
            nid = a[-1]
            if nid not in self.networks:
                return 1, "", ("Error response from daemon: network "
                               f"{nid} not found")
            return 0, self.networks[nid], ""
        if verb == "network" and a[1] == "ls":
            return 0, "".join(f"{n}\n" for n in self.networks), ""
        if verb == "volume" and a[1] == "inspect":
            name = a[-1]
            if name not in self.volumes:
                return 1, "", ("Error response from daemon: get "
                               f"{name}: no such volume")
            v = self.volumes[name]
            return 0, f"{v['driver']}|{v['label']}", ""
        if verb == "ps":
            want_vol = None
            for i, x in enumerate(a):
                if x == "--filter" and i + 1 < len(a):
                    if a[i + 1].startswith("volume="):
                        want_vol = a[i + 1][len("volume="):]
            if want_vol is not None and want_vol in self.ps_error_volumes:
                return 1, "", "daemon timed out reading mount table"
            ids = []
            for c in self.containers.values():
                if want_vol is not None:
                    if any(m.get("Type") == "volume"
                           and m.get("Name") == want_vol
                           for m in c["mounts"]):
                        ids.append(c["id"])
                elif c["label"] == RUN:
                    ids.append(c["id"])
            return 0, "".join(i[:12] + "\n" for i in ids), ""
        if verb in ("stop", "rm"):
            if verb == "rm":
                ref = a[-1]
                for name, c in list(self.containers.items()):
                    if name == ref or c["id"] == ref:
                        del self.containers[name]
            return 0, a[-1], ""
        if verb == "network" and a[1] == "rm":
            self.networks.pop(a[-1], None)
            return 0, a[-1], ""
        if verb == "volume" and a[1] == "rm":
            if a[-1] in self.fail_volume_rm:
                return 1, "", "volume is in use by another container"
            self.volumes.pop(a[-1], None)
            return 0, a[-1], ""
        return 0, "", ""

    # -- state assertions ------------------------------------------------

    def verbs(self):
        return [" ".join(c[1:3]) for c in self.calls]

    def removed_containers(self):
        return [c[-1] for c in self.calls
                if c[1:2] == ["rm"] and c[2:3] != ["network"]]

    def removed_volumes(self):
        return [c[-1] for c in self.calls
                if c[1:3] == ["volume", "rm"]]

    def simulate_up(self):
        """Model one `compose up -d`: persistent services keep their
        identities, the run network exists once, the named mysql data
        volume persists, and the client container is RECREATED — a new
        ID plus a fresh anonymous /var/lib/mysql volume, leaving the
        previous generation's anonymous volume behind (as compose does
        for image-declared VOLUME dirs)."""
        self._up_count = getattr(self, "_up_count", 0) + 1
        n = self._up_count
        self.add_container("c-mysql57", "id-m57",
                           mounts=[{"Type": "volume",
                                    "Name": f"{RUN}_{RUN}-mysql57-data",
                                    "Destination": "/var/lib/mysql"}])
        self.add_container("c-tidb85", "id-t85")
        self.add_container(
            "c-tidb85-client", f"id-cl{n}",
            mounts=[{"Type": "volume", "Name": f"anonvol-{n}",
                     "Destination": "/var/lib/mysql"}])
        self.networks.setdefault(f"net-{RUN[:8]}", RUN)
        self.volumes.setdefault(
            f"{RUN}_{RUN}-mysql57-data", {"driver": "local", "label": RUN})
        self.volumes[f"anonvol-{n}"] = {"driver": "local", "label": RUN}


def make_run(tmp, state=None):
    run = iso.IsoRun(RUN, DAEMON, "tester", tmp)
    run._task = "T21"
    (state or FakeDaemon()).bind(run)
    return run


def seed_ledger(run, containers=None, networks=None, volumes=None,
                generations=None):
    """Receipts a teardown-capable run needs: recorded endpoint, daemon
    identity, and either an explicit generations list or legacy flat
    fields (normalized to a generation zero)."""
    run.receipts["daemon_endpoint"] = DAEMON
    run.receipts["daemon_identity"] = {"server_id": SID}
    if generations is not None:
        run.receipts["generations"] = generations
    run.receipts["containers"] = containers or []
    run.receipts["networks"] = networks or []
    run.receipts["volumes"] = volumes or []
    return run


class TestLedgerPersistenceDispatch(unittest.TestCase):
    """R11 persistence contract via the real CLI dispatch path: every
    command runs a fresh IsoRun instance against the same on-disk
    ledger, exactly like repeated ddl_golden_isolated invocations.

    The fake intercepts dg.run_cmd (the single external-command seam)
    so no docker/colima binary is ever reached; compose up calls mutate
    the fake daemon's state the way a real recreate would.
    """

    def _drive(self, tmp, state, *commands, run_id=RUN, daemon=DAEMON,
               owner="tester"):
        """Run each subcommand through iso.main() with a fresh argv —
        an independent invocation boundary per command."""
        orig_argv, orig_lm, orig_rc = (
            sys.argv, iso.dg.load_manifest, iso.dg.run_cmd)

        def fake_run_cmd(args, cwd=None, timeout=120, stdin_text=None,
                         env=None):
            args = list(args)
            if args[:2] == ["docker", "compose"]:
                if "up" in args:
                    if state.fail_next_up:
                        state.fail_next_up = False
                        return 1, "", "compose up failed (fake)"
                    state.simulate_up()
                    return 0, "", ""
                return 0, "", ""
            return state.dispatch(args, env=env)

        iso.dg.load_manifest = lambda task: MANIFEST
        iso.dg.run_cmd = fake_run_cmd
        try:
            for cmd in commands:
                sys.argv = [
                    "ddl_golden_isolated.py",
                    "--run-id", run_id, "--daemon", daemon,
                    "--owner", owner, "--artifact-dir", str(tmp), cmd]
                iso.main()
        finally:
            sys.argv, iso.dg.load_manifest, iso.dg.run_cmd = orig_argv, orig_lm, orig_rc

    def test_second_up_preserves_ledger_generations(self):
        """RED on unfixed tree: `load_run` skips reload for `up`, so a
        second up reinitializes the receipts dict and the first record()
        truncates every prior generation — the client's earlier
        anonymous volume orphans out of the cleanup set."""
        tmp = tempfile.mkdtemp()
        state = FakeDaemon()
        self._drive(tmp, state, "up", "up")
        disk = json.loads(
            (pathlib.Path(tmp) / "receipts.json").read_text())
        self.assertEqual(len(disk["generations"]), 2)
        anon = {v["name"] for g in disk["generations"]
                for v in g["volumes"] if v.get("kind") == "anonymous"}
        self.assertEqual(anon, {"anonvol-1", "anonvol-2"})

    def test_repeated_up_teardown_consumes_all_generations(self):
        """After up → up → run-style capture, rmvolumes must still
        consume volumes registered in the superseded generation —
        not only the last up's objects."""
        tmp = tempfile.mkdtemp()
        state = FakeDaemon()
        self._drive(tmp, state, "up", "up")
        # A run-wrapped capture (what ledger_compose records inside
        # `run`) appends another generation over the restored ledger.
        run = iso.IsoRun(RUN, DAEMON, "tester", tmp)
        raw = (pathlib.Path(tmp) / "receipts.json").read_bytes()
        run.receipts = json.loads(raw)
        run._ledger_snapshot = raw
        state.bind(run)
        run._capture_generation("compose.up.run")
        self._drive(tmp, state, "stop", "rm", "rmvolumes")
        self.assertEqual(sorted(state.removed_volumes()),
                         sorted(["anonvol-1", "anonvol-2",
                                 f"{RUN}_{RUN}-mysql57-data"]))
        disk = json.loads(
            (pathlib.Path(tmp) / "receipts.json").read_text())
        self.assertEqual(len(disk["generations"]), 3)

    def test_reobserved_marks_second_sighting_not_new_creation(self):
        """Group A continuation: re-capturing a still-live registered
        object marks reobserved, never a second creation record."""
        tmp = tempfile.mkdtemp()
        state = FakeDaemon()
        self._drive(tmp, state, "up", "up")
        disk = json.loads(
            (pathlib.Path(tmp) / "receipts.json").read_text())
        gen1 = disk["generations"][1]
        persistent = {c["name"]: c for c in gen1["containers"]
                      if c["name"] == "c-mysql57"}
        self.assertTrue(persistent["c-mysql57"].get("reobserved"))
        client = {c["name"]: c for c in gen1["containers"]
                  if c["name"] == "c-tidb85-client"}
        # The recreated client is a NEW id — a real creation, not a
        # re-observation.
        self.assertNotIn("reobserved", client["c-tidb85-client"])

    # -- Group B: protected and unregistered objects stay untouched ------
    def test_teardown_leaves_protected_and_unregistered(self):
        tmp = tempfile.mkdtemp()
        state = FakeDaemon()
        state.volumes["protected-old-data"] = {"driver": "local",
                                               "label": ""}
        state.volumes["foreign-anon-9f"] = {"driver": "local",
                                            "label": "other-run"}
        self._drive(tmp, state, "up", "up", "stop", "rm", "rmvolumes")
        self.assertIn("protected-old-data", state.volumes)
        self.assertIn("foreign-anon-9f", state.volumes)
        # nothing unregistered ever entered the rm argv
        for v in state.removed_volumes():
            self.assertNotIn(v, ["protected-old-data", "foreign-anon-9f"])

    # -- Group C: cross-run and identity isolation -----------------------
    def test_cross_run_ledger_refused(self):
        tmp = tempfile.mkdtemp()
        self._drive(tmp, FakeDaemon(), "up")
        with self.assertRaises(SystemExit):
            self._drive(tmp, FakeDaemon(), "up",
                        run_id="ds-t21-20990101T000000Z-other")
        disk = json.loads(
            (pathlib.Path(tmp) / "receipts.json").read_text())
        self.assertEqual(disk["run_id"], RUN)

    def test_resumed_run_endpoint_mismatch_refused(self):
        tmp = tempfile.mkdtemp()
        self._drive(tmp, FakeDaemon(), "up")
        with self.assertRaises(SystemExit):
            self._drive(tmp, FakeDaemon(), "up",
                        daemon="unix:///tmp/iso-vm/other.sock")

    def test_resumed_run_owner_mismatch_refused(self):
        tmp = tempfile.mkdtemp()
        self._drive(tmp, FakeDaemon(), "up")
        with self.assertRaises(SystemExit):
            self._drive(tmp, FakeDaemon(), "up", owner="someone-else")

    def test_teardown_without_ledger_refused(self):
        tmp = tempfile.mkdtemp()
        for cmd in ("stop", "rm", "rmvolumes"):
            with self.assertRaises(SystemExit):
                self._drive(tmp, FakeDaemon(), cmd)

    # -- Group D: corruption, interruption, concurrent writers ------------
    def test_corrupt_ledger_refused_not_reinitialised(self):
        tmp = tempfile.mkdtemp()
        self._drive(tmp, FakeDaemon(), "up")
        path = pathlib.Path(tmp) / "receipts.json"
        before = path.read_bytes()
        path.write_bytes(b"{ not json !!!")
        with self.assertRaises(SystemExit):
            self._drive(tmp, FakeDaemon(), "up")
        self.assertEqual(path.read_bytes(), b"{ not json !!!")
        self.assertNotEqual(path.read_bytes(), before)

    def test_concurrent_writer_conflict_refused(self):
        """Two same-run entries both holding the same snapshot: the
        second writer's stale snapshot refuses, so appended history from
        the first is never overwritten."""
        tmp = tempfile.mkdtemp()
        raw_path = pathlib.Path(tmp) / "receipts.json"
        entry_a = iso.IsoRun(RUN, DAEMON, "tester", tmp)
        entry_a.record("entry.a", 0)
        raw = raw_path.read_bytes()
        # entry_b "loaded" the pre-a state
        stale = iso.IsoRun(RUN, DAEMON, "tester", tmp)
        stale.receipts = json.loads(raw)
        stale._ledger_snapshot = b"{}\n"   # what it thinks is on disk
        with self.assertRaises(SystemExit):
            stale.record("entry.b", 0)
        disk = json.loads(raw_path.read_text())
        self.assertEqual([a["action"] for a in disk["actions"]],
                         ["entry.a"])

    def test_write_failure_keeps_last_valid_ledger(self):
        tmp = tempfile.mkdtemp()
        run = make_run(tmp)
        run.record("ok.write", 0)
        good = (pathlib.Path(tmp) / "receipts.json").read_bytes()
        orig_replace = iso.os.replace
        iso.os.replace = lambda *a, **k: (_ for _ in ()).throw(
            OSError("disk full"))
        try:
            with self.assertRaises(SystemExit):
                run.record("must.fail", 0)
        finally:
            iso.os.replace = orig_replace
        self.assertEqual(
            (pathlib.Path(tmp) / "receipts.json").read_bytes(), good)
        # tmp file cleaned up after the failure
        self.assertFalse(
            (pathlib.Path(tmp) / "receipts.json.tmp").exists())

    def test_ledger_file_mode_restricted(self):
        tmp = tempfile.mkdtemp()
        run = make_run(tmp)
        run.record("perm.check", 0)
        mode = (pathlib.Path(tmp) / "receipts.json").stat().st_mode & 0o777
        self.assertEqual(mode, 0o600)

    # -- Group E: up partial failure -------------------------------------
    def test_failed_up_keeps_history_and_marks_no_fake_zero(self):
        tmp = tempfile.mkdtemp()
        state = FakeDaemon()
        self._drive(tmp, state, "up")
        state.fail_next_up = True
        with self.assertRaises(SystemExit):
            self._drive(tmp, state, "up")
        disk = json.loads(
            (pathlib.Path(tmp) / "receipts.json").read_text())
        # old generation kept, failed compose.up recorded, nothing new
        # entered the ledger
        self.assertEqual(len(disk["generations"]), 1)
        self.assertTrue(any(a["action"] == "compose.up" and a["rc"] == 1
                            for a in disk["actions"]))

    def test_capture_incomplete_marked_on_partial_failure(self):
        tmp = tempfile.mkdtemp()
        state = FakeDaemon()
        state.add_container("c-mysql57", "id-m57")
        state.add_container("c-tidb85", "id-t85")
        state.add_container("c-tidb85-client", "id-clx")
        # The client exists but its inspect call errors — capture must
        # mark the generation incomplete instead of reporting a smaller
        # world.
        state.error_containers.add("c-tidb85-client")
        run = make_run(tmp, state)
        run._manifest = MANIFEST
        gen = run._capture_generation("test.capture")
        self.assertTrue(gen.get("capture_incomplete"))
        self.assertEqual([c["name"] for c in gen["containers"]],
                         ["c-mysql57", "c-tidb85"])
        self.assertTrue(any(a["action"] == "capture.error"
                            for a in run.receipts["actions"]))

    # -- Group F: teardown refusal classes and idempotent retry -----------
    def test_mounted_registered_volume_refused_and_kept(self):
        tmp = tempfile.mkdtemp()
        state = FakeDaemon()
        state.add_container(
            "c-cl", "id-cl",
            mounts=[{"Type": "volume", "Name": "ab12anon",
                     "Destination": "/var/lib/mysql"}])
        state.volumes["ab12anon"] = {"driver": "local", "label": ""}
        run = make_run(tmp, state)
        seed_ledger(run, generations=[{
            "at": "t0", "action": "compose.up",
            "containers": [{"name": "c-cl", "id": "id-cl"}],
            "networks": [],
            "volumes": [{"kind": "anonymous", "name": "ab12anon",
                         "mounted_by": "c-cl",
                         "destination": "/var/lib/mysql"}]}])
        with self.assertRaises(SystemExit):
            run.rmvolumes()
        self.assertEqual(state.removed_volumes(), [])
        self.assertIn("ab12anon", state.volumes)
        self.assertTrue(any(a["action"] == "volume.inuse.ab12anon"
                            for a in run.receipts["actions"]))

    def test_teardown_retry_after_failure_preserves_history(self):
        tmp = tempfile.mkdtemp()
        state = FakeDaemon()
        state.add_container("c1", "id-A")
        run = make_run(tmp, state)
        seed_ledger(run, containers=[{"name": "c1", "id": "id-A"}])
        state.inspect_error_once = True
        with self.assertRaises(SystemExit):
            run.stop()
        state.inspect_error_once = False
        run.stop()
        run.rm()
        self.assertEqual(state.removed_containers(), ["id-A"])
        acts = [a["action"] for a in run.receipts["actions"]]
        self.assertIn("identity.refuse", acts)
        self.assertIn("rm.id-A", acts)


class TestEndpointConstraints(unittest.TestCase):
    def test_forbidden_endpoints_rejected(self):
        for bad in ("unix:///var/run/docker.sock",
                    "/var/run/docker.sock",
                    "unix:///Users/fan/.docker/run/docker.sock"):
            with self.assertRaises(SystemExit):
                iso.IsoRun(RUN, bad, "tester", tempfile.mkdtemp())

    def test_empty_endpoint_rejected(self):
        with self.assertRaises(SystemExit):
            iso.IsoRun(RUN, "", "tester", tempfile.mkdtemp())

    def test_env_pins_daemon_and_strips_context(self):
        os.environ["DOCKER_CONTEXT"] = "desktop-linux"
        try:
            run = iso.IsoRun(RUN, DAEMON, "tester", tempfile.mkdtemp())
            self.assertEqual(run.env["DOCKER_HOST"], DAEMON)
            self.assertNotIn("DOCKER_CONTEXT", run.env)
            run.record("probe", 0)
            actions = [a["action"] for a in run.receipts["actions"]]
            self.assertIn("env.docker_context.cleared", actions)
            self.assertEqual(actions.index("env.docker_context.cleared"), 0)
        finally:
            os.environ.pop("DOCKER_CONTEXT", None)


class TestComposeScoping(unittest.TestCase):
    def test_compose_uses_run_project_and_override(self):
        tmp = tempfile.mkdtemp()
        run = make_run(tmp)
        run.write_override(MANIFEST)
        calls = []
        run.dg.run_cmd = lambda args, cwd=None, timeout=120, stdin_text=None, env=None: (
            calls.append({"args": list(args), "env": dict(env or {})})
            or (0, "", ""))
        run.compose("up", "-d", "mysql57")
        argv = calls[0]["args"]
        self.assertEqual(argv[:4], ["docker", "compose", "-p", RUN])
        files = [argv[i + 1] for i, a in enumerate(argv) if a == "-f"]
        self.assertEqual(len(files), 2)
        self.assertTrue(files[1].endswith("compose.override.yaml"))
        self.assertEqual(calls[0]["env"]["DOCKER_HOST"], DAEMON)

    def test_override_labels_and_named_volumes(self):
        run = make_run(tempfile.mkdtemp())
        override = run.write_override(MANIFEST).read_text()
        self.assertIn("org.deltascope.task: T21", override)
        self.assertIn(f"org.deltascope.run: {RUN}", override)
        self.assertIn("org.deltascope.owner: tester", override)
        self.assertIn(f"{RUN}-mysql57-data:/var/lib/mysql", override)
        self.assertNotIn(f"{RUN}-tidb85-data", override)

    # T23-S: non-mysql infrastructure services (PD/TiKV/TiFlash) declare
    # their data mounts via anchor 'data_volumes' so the override mounts
    # them as run-labelled named volumes — the ledger can classify them
    # instead of adopting anonymous mounts by name alone.
    def test_override_declared_data_volumes(self):
        manifest = copy.deepcopy(MANIFEST)
        manifest["anchors"]["tiflash"] = {
            "service": "t23s-tiflash", "container": "c-tf",
            "image": "pingcap/tiflash:v8.5.0", "infra_only": True,
            "data_volumes": [{"mount": "/data", "suffix": "data"},
                             {"mount": "/logs", "suffix": "logs"}],
        }
        run = make_run(tempfile.mkdtemp())
        override = run.write_override(manifest).read_text()
        self.assertIn(f"{RUN}-t23s-tiflash-data:/data", override)
        self.assertIn(f"{RUN}-t23s-tiflash-logs:/logs", override)
        self.assertIn(f"  {RUN}-t23s-tiflash-data:", override)
        self.assertIn(f"  {RUN}-t23s-tiflash-logs:", override)
        self.assertEqual(
            sorted(run._volumes),
            [f"{RUN}-mysql57-data", f"{RUN}-t23s-tiflash-data",
             f"{RUN}-t23s-tiflash-logs"])

    def test_override_data_volumes_on_client_service_rejected(self):
        # data_volumes belong to the anchor service; mounting them on the
        # client side would smuggle lifecycle-managed storage into a
        # disposable helper container.
        manifest = copy.deepcopy(MANIFEST)
        manifest["anchors"]["tidb85"]["data_volumes"] = [
            {"mount": "/data", "suffix": "data", "on": "client"}]
        run = make_run(tempfile.mkdtemp())
        with self.assertRaises(SystemExit):
            run.write_override(manifest)

    def test_override_data_volumes_bad_suffix_rejected(self):
        for suffix in ("../escape", "a/b", "white space", "", "-"):
            manifest = copy.deepcopy(MANIFEST)
            manifest["anchors"]["pd"] = {
                "service": "t23s-pd", "container": "c-pd",
                "image": "pingcap/pd:v8.5.0", "infra_only": True,
                "data_volumes": [{"mount": "/data", "suffix": suffix}],
            }
            run = make_run(tempfile.mkdtemp())
            with self.assertRaises(SystemExit, msg=f"suffix {suffix!r}"):
                run.write_override(manifest)

    def test_override_data_volumes_missing_fields_rejected(self):
        for entry in ({"suffix": "data"}, {"mount": "/data"},
                      {"mount": "", "suffix": "data"},
                      {"mount": "rel/path", "suffix": "data"}):
            manifest = copy.deepcopy(MANIFEST)
            manifest["anchors"]["pd"] = {
                "service": "t23s-pd", "container": "c-pd",
                "image": "pingcap/pd:v8.5.0", "infra_only": True,
                "data_volumes": [entry],
            }
            run = make_run(tempfile.mkdtemp())
            with self.assertRaises(SystemExit, msg=f"entry {entry!r}"):
                run.write_override(manifest)


class TestMultiServiceCapture(unittest.TestCase):
    """T23-S: a wider anchor set (PD/TiKV/TiFlash infra + TiDB servers)
    must all appear in the generation ledger — every declared container
    and every declared data volume, classified named vs anonymous."""

    def _manifest(self):
        manifest = copy.deepcopy(MANIFEST)
        for key, svc, img, vols in (
            ("pd", "t23s-pd", "pingcap/pd:v8.5.0", ["data"]),
            ("tikv", "t23s-tikv", "pingcap/tikv:v8.5.0", ["data"]),
            ("tiflash", "t23s-tiflash", "pingcap/tiflash:v8.5.0",
             ["data", "logs"]),
        ):
            manifest["anchors"][key] = {
                "service": svc, "container": f"c-{svc}",
                "image": img, "infra_only": True,
                "data_volumes": [{"mount": "/data" if s == "data"
                                  else "/logs", "suffix": s}
                                 for s in vols],
            }
        return manifest

    def test_capture_registers_infra_containers_and_volumes(self):
        manifest = self._manifest()
        state = FakeDaemon()
        state.add_container("c-mysql57", "id-m57")
        state.add_container("c-tidb85", "id-t85")
        state.add_container("c-tidb85-client", "id-cl")
        for svc in ("t23s-pd", "t23s-tikv", "t23s-tiflash"):
            mounts = []
            for anchor in manifest["anchors"].values():
                if anchor["service"] != svc:
                    continue
                for dv in anchor["data_volumes"]:
                    mounts.append({
                        "Type": "volume",
                        "Name": f"{RUN}_{RUN}-{svc}-{dv['suffix']}",
                        "Destination": dv["mount"]})
            state.add_container(f"c-{svc}", f"id-{svc}", mounts=mounts)
        run = make_run(tempfile.mkdtemp(), state)
        run._volumes = [
            f"{RUN}-{a['service']}-{dv['suffix']}"
            for a in manifest["anchors"].values()
            for dv in a.get("data_volumes", [])]
        run._volumes.append(f"{RUN}-mysql57-data")
        run._manifest = manifest
        gen = run._capture_generation("test.capture")
        names = {c["name"] for c in gen["containers"]}
        self.assertEqual(
            names, {"c-mysql57", "c-tidb85", "c-tidb85-client",
                    "c-t23s-pd", "c-t23s-tikv", "c-t23s-tiflash"})
        kinds = {v["name"]: v["kind"] for v in gen["volumes"]}
        for svc, vols in (("t23s-pd", ["data"]),
                          ("t23s-tikv", ["data"]),
                          ("t23s-tiflash", ["data", "logs"])):
            for suffix in vols:
                self.assertEqual(
                    kinds.get(f"{RUN}_{RUN}-{svc}-{suffix}"), "named",
                    f"{svc}-{suffix}")

    def test_capture_undeclared_infra_mount_is_anonymous(self):
        manifest = self._manifest()
        state = FakeDaemon()
        state.add_container("c-mysql57", "id-m57")
        state.add_container("c-tidb85", "id-t85")
        state.add_container("c-tidb85-client", "id-cl")
        state.add_container(
            "c-t23s-tikv", "id-t23s-tikv",
            mounts=[{"Type": "volume", "Name": "5f6e7d8canon",
                     "Destination": "/data"}])
        run = make_run(tempfile.mkdtemp(), state)
        run._volumes = [
            f"{RUN}-{a['service']}-{dv['suffix']}"
            for a in manifest["anchors"].values()
            for dv in a.get("data_volumes", [])]
        run._manifest = manifest
        gen = run._capture_generation("test.capture")
        kinds = {v["name"]: v["kind"] for v in gen["volumes"]}
        self.assertEqual(kinds.get("5f6e7d8canon"), "anonymous")


class TestCreationLedger(unittest.TestCase):
    def test_up_captures_daemon_identity_and_generation(self):
        tmp = tempfile.mkdtemp()
        state = FakeDaemon()
        state.add_container("c-mysql57", "id-m57")
        state.add_container("c-tidb85", "id-t85")
        state.add_container("c-tidb85-client", "id-cl")
        run = make_run(tmp, state)
        run.compose = lambda *a, timeout=300: (0, "", "")
        containers = run.up(MANIFEST, "T21")
        self.assertEqual([c["name"] for c in containers],
                         ["c-mysql57", "c-tidb85", "c-tidb85-client"])
        gens = run.receipts["generations"]
        self.assertEqual(len(gens), 1)
        self.assertEqual(gens[0]["action"], "compose.up")
        self.assertEqual(run.receipts["daemon_identity"]["server_id"], SID)

    def test_generation_capture_records_anonymous_volumes(self):
        tmp = tempfile.mkdtemp()
        state = FakeDaemon()
        named = f"{RUN}_{RUN}-mysql57-data"
        state.add_container(
            "c-mysql57", "id-m57",
            mounts=[{"Type": "volume", "Name": named,
                     "Destination": "/var/lib/mysql"}])
        state.add_container(
            "c-tidb85-client", "id-cl",
            mounts=[{"Type": "volume", "Name": "ab12cd34anon",
                     "Destination": "/var/lib/mysql"}])
        run = make_run(tmp, state)
        run._volumes = [f"{RUN}-mysql57-data"]
        run._manifest = MANIFEST
        gen = run._capture_generation("test.capture")
        kinds = {v["name"]: v["kind"] for v in gen["volumes"]}
        self.assertEqual(kinds.get(named), "named")
        self.assertEqual(kinds.get("ab12cd34anon"), "anonymous")

    def test_foreign_container_not_captured(self):
        tmp = tempfile.mkdtemp()
        state = FakeDaemon()
        state.add_container("c-mysql57", "id-m57", label="other-run")
        run = make_run(tmp, state)
        run._manifest = MANIFEST
        gen = run._capture_generation("test.capture")
        self.assertEqual(gen["containers"], [])


class TestTeardownLedger(unittest.TestCase):
    # 1. Same name, new ID, no registered rebuild -> refuse even when the
    #    live object carries this run's label.
    def test_unregistered_id_same_name_refused(self):
        tmp = tempfile.mkdtemp()
        state = FakeDaemon()
        state.add_container("c1", "id-B")          # live object, run label
        run = make_run(tmp, state)
        seed_ledger(run, containers=[{"name": "c1", "id": "id-A"}])
        with self.assertRaises(SystemExit):
            run._verified_ids()
        self.assertEqual(state.removed_containers(), [])

    # 2. A registered rebuild generation tears down precisely.
    def test_registered_regeneration_collected(self):
        tmp = tempfile.mkdtemp()
        state = FakeDaemon()
        state.add_container("c1", "id-B")
        run = make_run(tmp, state)
        seed_ledger(run, generations=[
            {"at": "t0", "action": "compose.up",
             "containers": [{"name": "c1", "id": "id-A"}],
             "networks": [], "volumes": []},
            {"at": "t1", "action": "compose.up.run",
             "containers": [{"name": "c1", "id": "id-B"}],
             "networks": [], "volumes": []},
        ])
        self.assertEqual(run._verified_ids(), ["id-B"])
        run.stop()
        run.rm()
        self.assertEqual(state.removed_containers(), ["id-B"])

    # 3. Same project but a different RUN_ID label refuses.
    def test_foreign_run_label_refused(self):
        tmp = tempfile.mkdtemp()
        state = FakeDaemon()
        state.add_container("c1", "id-A", label="ds-t21-other-run")
        run = make_run(tmp, state)
        seed_ledger(run, containers=[{"name": "c1", "id": "id-A"}])
        with self.assertRaises(SystemExit):
            run._verified_ids()

    # 4. Endpoint or daemon identity change refuses before any inspect.
    def test_daemon_binding_refuses_wrong_endpoint(self):
        tmp = tempfile.mkdtemp()
        run = iso.IsoRun(RUN, "unix:///tmp/other.sock", "tester", tmp)
        FakeDaemon().bind(run)
        seed_ledger(run, containers=[{"name": "c1", "id": "id-A"}])
        with self.assertRaises(SystemExit):
            run.stop()

    def test_daemon_binding_refuses_identity_mismatch(self):
        tmp = tempfile.mkdtemp()
        state = FakeDaemon(server_id="srv-different")
        run = make_run(tmp, state)
        seed_ledger(run, containers=[{"name": "c1", "id": "id-A"}])
        with self.assertRaises(SystemExit):
            run.stop()

    def test_daemon_unreachable_refuses(self):
        tmp = tempfile.mkdtemp()
        state = FakeDaemon()
        calls = state.calls

        def dead(args, env=None):
            calls.append(list(args))
            return 1, "", "Cannot connect to the Docker daemon"

        run = make_run(tmp)
        run.docker = lambda *a, timeout=180: dead(["docker", *a])
        seed_ledger(run, containers=[{"name": "c1", "id": "id-A"}])
        with self.assertRaises(SystemExit):
            run.stop()

    # 5. DOCKER_CONTEXT conflict is removed in the task env and recorded,
    #    never silently inherited.
    def test_docker_context_never_reaches_subprocess(self):
        os.environ["DOCKER_CONTEXT"] = "desktop-linux"
        try:
            state = FakeDaemon()
            run = make_run(tempfile.mkdtemp(), state)
            run.docker("ps")
            envs = [run.env]
            self.assertTrue(all("DOCKER_CONTEXT" not in e for e in envs))
        finally:
            os.environ.pop("DOCKER_CONTEXT", None)

    # 6. Explicit NotFound ends idempotently; daemon/permission/timeout
    #    errors refuse rather than masquerading as 'already gone'.
    def test_notfound_is_gone_not_error(self):
        tmp = tempfile.mkdtemp()
        state = FakeDaemon()   # no containers at all
        run = make_run(tmp, state)
        seed_ledger(run, containers=[{"name": "c1", "id": "id-A"}])
        self.assertEqual(run._verified_ids(), [])
        actions = [a["action"] for a in run.receipts["actions"]]
        self.assertIn("identity.gone", actions)

    def test_inspect_error_is_not_gone(self):
        tmp = tempfile.mkdtemp()

        def flaky(args, env=None):
            if args[1] == "info":
                return 0, SID, ""
            return 1, "", "permission denied while trying to connect"

        run = make_run(tmp)
        run.docker = lambda *a, timeout=180: flaky(["docker", *a])
        seed_ledger(run, containers=[{"name": "c1", "id": "id-A"}])
        with self.assertRaises(SystemExit):
            run._verified_ids()

    # 7. An unlabeled volume under a resolvable name is retained, not
    #    deleted, not fatal.
    def test_unlabeled_volume_retained(self):
        tmp = tempfile.mkdtemp()
        state = FakeDaemon()
        materialized = f"{RUN}_{RUN}-mysql57-data"
        state.volumes[materialized] = {"driver": "local", "label": ""}
        run = make_run(tmp, state)
        seed_ledger(run, volumes=[{"name": f"{RUN}-mysql57-data"}])
        run.rmvolumes()
        self.assertEqual(state.removed_volumes(), [])
        actions = [a["action"] for a in run.receipts["actions"]]
        self.assertIn(f"volume.retained.{materialized}", actions)

    def test_registered_named_and_anonymous_volumes_removed(self):
        tmp = tempfile.mkdtemp()
        state = FakeDaemon()
        materialized = f"{RUN}_{RUN}-mysql57-data"
        state.volumes[materialized] = {"driver": "local", "label": RUN}
        state.volumes["ab12anon"] = {"driver": "local", "label": ""}
        run = make_run(tmp, state)
        seed_ledger(run, generations=[{
            "at": "t0", "action": "compose.up", "containers": [],
            "networks": [],
            "volumes": [
                {"kind": "named", "declared": f"{RUN}-mysql57-data",
                 "name": materialized, "driver": "local"},
                {"kind": "anonymous", "name": "ab12anon",
                 "mounted_by": "c-cl", "destination": "/var/lib/mysql"},
            ]}])
        run.rmvolumes()
        self.assertEqual(sorted(state.removed_volumes()),
                         ["ab12anon", materialized])

    # 8. A name that resolves correctly but was never registered refuses
    #    (network variant: live project-labeled net absent from ledger).
    def test_unregistered_network_refused(self):
        tmp = tempfile.mkdtemp()
        state = FakeDaemon()
        state.networks["net-new"] = RUN  # same project, never captured
        run = make_run(tmp, state)
        seed_ledger(run, networks=["net-old"])  # net-old -> gone ok
        with self.assertRaises(SystemExit):
            run._verified_networks()

    # 9. A failed teardown preserves ledger, error, and retry records.
    def test_cleanup_failure_preserves_state(self):
        tmp = tempfile.mkdtemp()
        state = FakeDaemon()
        state.add_container("c1", "id-B")
        run = make_run(tmp, state)
        seed_ledger(run, containers=[{"name": "c1", "id": "id-A"}])
        result = run.patched_cleanup()
        self.assertEqual(result["compose_down_rc"], 1)
        self.assertIn("error", result)
        on_disk = json.loads(
            (pathlib.Path(tmp) / "receipts.json").read_text())
        self.assertTrue(on_disk["containers"])
        self.assertTrue(any(a["action"] == "identity.refuse"
                            for a in on_disk["actions"]))

    # 10. Ordinary idempotent teardown: everything already gone passes
    #     without loosened checks.
    def test_idempotent_teardown_all_gone(self):
        tmp = tempfile.mkdtemp()
        run = make_run(tmp, FakeDaemon())
        seed_ledger(run,
                    containers=[{"name": "c1", "id": "id-A"}],
                    networks=["net-gone"],
                    volumes=[{"name": f"{RUN}-v"}])
        run.stop()   # no exception
        run.rm()
        run.rmvolumes()
        actions = [a["action"] for a in run.receipts["actions"]]
        self.assertIn("identity.gone", actions)
        self.assertNotIn("identity.refuse", actions)

    def test_stock_cleanup_never_runs_down_v(self):
        tmp = tempfile.mkdtemp()
        state = FakeDaemon()
        state.add_container("c1", "id-c1")
        run = make_run(tmp, state)
        seed_ledger(run, containers=[{"name": "c1", "id": "id-c1"}])
        result = run.patched_cleanup()
        joined = " ".join(" ".join(c) for c in state.calls)
        self.assertNotIn("down", joined)
        self.assertNotIn("--remove-orphans", joined)
        self.assertTrue(result.get("isolated"))
        self.assertEqual(result["compose_down_rc"], 0)
        self.assertEqual(result["residual_containers"], [])


FAKE_DOCKER_PY = '''#!/usr/bin/env python3
"""PATH-level fake docker for cross-process lock tests. State under
FAKE_DOCKER_STATE; every call appends argv to argv.log so tests can see
exactly which daemon calls each entry issued and when."""
import json, os, pathlib, secrets, sys, time

STATE = pathlib.Path(os.environ["FAKE_DOCKER_STATE"]) / "state.json"
LOG = pathlib.Path(os.environ["FAKE_DOCKER_STATE"]) / "argv.log"
MAP = {"mysql57": "deltascope-ddl-golden-mysql57",
       "mysql80": "deltascope-ddl-golden-mysql80",
       "mysql84": "deltascope-ddl-golden-mysql84",
       "tidb85": "deltascope-ddl-golden-tidb85",
       "tidb85-client": "deltascope-ddl-golden-tidb85-client"}
IMG = {"mysql57": "mysql:5.7.44", "mysql80": "mysql:8.0.46",
       "mysql84": "mysql:8.4.10", "tidb85": "pingcap/tidb:v8.5.0",
       "tidb85-client": "mysql:8.4.10"}


def load():
    if STATE.exists():
        return json.loads(STATE.read_text())
    return {"containers": {}, "volumes": {}, "networks": {}}


def save(s):
    STATE.write_text(json.dumps(s))


args = sys.argv[1:]
LOG.parent.mkdir(parents=True, exist_ok=True)
with LOG.open("a") as fh:
    fh.write(json.dumps(args) + "\\n")
s = load()

if args[0] == "info":
    fmt = args[args.index("--format") + 1] if "--format" in args else ""
    if "{{json" in fmt:
        print(json.dumps({"ID": "fake-srv-1", "Name": "fake",
                          "OperatingSystem": "fake",
                          "ServerVersion": "0"}))
    else:
        print("fake-srv-1")
    sys.exit(0)

if args[0] == "compose" and "up" in args:
    stall = os.environ.get("FAKE_UP_STALL")
    if stall:
        mark = os.environ.get("FAKE_UP_MARK")
        if mark:
            pathlib.Path(mark).touch()
        deadline = time.monotonic() + 60
        while not pathlib.Path(stall).exists():
            if time.monotonic() > deadline:
                sys.exit(3)
            time.sleep(0.05)
    proj = args[args.index("-p") + 1]
    for svc in args[args.index("-d") + 1:]:
        name = MAP.get(svc)
        if not name:
            continue
        old = s["containers"].pop(name, None)
        if old:
            for v in s["volumes"].values():
                if old["id"] in v.get("users", []):
                    v["users"].remove(old["id"])
        cid = secrets.token_hex(32)
        mounts = []
        if svc == "tidb85-client":
            vname = secrets.token_hex(32)
            s["volumes"][vname] = {"label": "", "driver": "local",
                                   "users": [cid]}
            mounts.append({"Type": "volume", "Name": vname,
                           "Destination": "/var/lib/mysql"})
        else:
            vname = f"{proj}-{svc}-data"
            s["volumes"].setdefault(
                vname, {"label": proj, "driver": "local", "users": [cid]})
            mounts.append({"Type": "volume", "Name": vname,
                           "Destination": "/var/lib/mysql"})
        s["containers"][name] = {"id": cid, "label": proj,
                                 "image": IMG[svc], "mounts": mounts}
    s["networks"][f"net-{proj}"] = {"project": proj}
    save(s)
    sys.exit(0)

if args[0] == "inspect":
    ref = args[-1]
    c = s["containers"].get(ref)
    if c is None:
        for cc in s["containers"].values():
            if cc["id"] == ref or cc["id"].startswith(ref):
                c = cc
                break
    if c is None:
        print(f"Error: No such object: {ref}", file=sys.stderr)
        sys.exit(1)
    name = next(n for n, cc in s["containers"].items() if cc is c)
    print(f"{c['id']}|/{name}|{c['label']}|{c['image']}"
          f"|{json.dumps(c['mounts'])}")
    sys.exit(0)

if args[0] == "network":
    if args[1] == "ls":
        for nid in s["networks"]:
            print(nid)
        sys.exit(0)
    if args[1] == "inspect":
        net = s["networks"].get(args[-1])
        if net is None:
            print("Error: No such network", file=sys.stderr)
            sys.exit(1)
        print(net["project"])
        sys.exit(0)
    if args[1] == "rm":
        s["networks"].pop(args[-1], None)
        save(s)
        sys.exit(0)

if args[0] == "volume":
    if args[1] == "inspect":
        v = s["volumes"].get(args[-1])
        if v is None:
            print("Error: No such volume", file=sys.stderr)
            sys.exit(1)
        print(f"{v['driver']}|{v['label']}")
        sys.exit(0)
    if args[1] == "rm":
        s["volumes"].pop(args[-1], None)
        save(s)
        sys.exit(0)

if args[0] == "ps":
    joined = " ".join(args)
    if "volume=" in joined:
        vname = joined.split("volume=")[1].split('"')[0].split()[0]
        for u in s["volumes"].get(vname, {}).get("users", []):
            print(u)
    else:
        for c in s["containers"].values():
            print(c["id"])
    sys.exit(0)

if args[0] in ("stop", "rm"):
    if args[0] == "rm":
        for name, c in list(s["containers"].items()):
            if name == args[-1] or c["id"] == args[-1]:
                for v in s["volumes"].values():
                    if c["id"] in v.get("users", []):
                        v["users"].remove(c["id"])
                del s["containers"][name]
        save(s)
    sys.exit(0)

print(f"fake-docker: unhandled {args}", file=sys.stderr)
sys.exit(2)
'''


class TestCrossProcessEntryLock(unittest.TestCase):
    """B1: cooperating same-run entries serialise on a stable sidecar
    lock for the WHOLE entry lifetime — acquire before ledger load, hold
    across daemon side-effects, release at exit. Real main()
    subprocesses against a PATH-level fake daemon; no in-process
    shortcuts for the two-entry scenarios."""

    ISO_SCRIPT = ROOT / "scripts" / "ddl_golden_isolated.py"

    def _layout(self):
        tmp = pathlib.Path(tempfile.mkdtemp())
        fake = tmp / "fakebin"
        dstate = tmp / "dstate"
        art = tmp / "art"
        fake.mkdir(); dstate.mkdir(); art.mkdir()
        (fake / "docker").write_text(FAKE_DOCKER_PY)
        os.chmod(fake / "docker", 0o755)
        return tmp, fake, dstate, art

    def _spawn(self, fake, dstate, art, cmd, extra_env=None):
        env = dict(os.environ)
        env["PATH"] = f"{fake}:{env['PATH']}"
        env["FAKE_DOCKER_STATE"] = str(dstate)
        env.pop("DOCKER_HOST", None)
        env.pop("DOCKER_CONTEXT", None)
        env.update(extra_env or {})
        return subprocess.Popen(
            [sys.executable, str(self.ISO_SCRIPT),
             "--run-id", "r12lock", "--daemon", "unix:///fake",
             "--owner", "audit", "--artifact-dir", str(art),
             "--task", "T21", cmd],
            env=env, stdout=subprocess.PIPE, stderr=subprocess.PIPE)

    @staticmethod
    def _argv_lines(dstate):
        log = dstate / "argv.log"
        return log.read_text().strip().splitlines() if log.exists() else []

    @staticmethod
    def _wait_mark(path, timeout=15.0):
        deadline = time.monotonic() + timeout
        while not pathlib.Path(path).exists():
            if time.monotonic() > deadline:
                raise AssertionError(f"marker {path} never appeared")
            time.sleep(0.05)

    def _stall_env(self, tmp, name):
        release = tmp / f"{name}.release"
        mark = tmp / f"{name}.mark"
        return release, mark, {
            "FAKE_UP_STALL": str(release), "FAKE_UP_MARK": str(mark)}

    def test_two_entries_existing_run_serialise(self):
        """Scenario A: A holds the lock stalled inside compose up; B
        must not reach ANY daemon call until A finishes; afterwards B
        acquires, reloads, appends — both records preserved."""
        tmp, fake, dstate, art = self._layout()
        release, mark, env = self._stall_env(tmp, "A")
        # seed one completed up so the run ledger already exists
        seed = self._spawn(fake, dstate, art, "up")
        self.assertEqual(seed.wait(timeout=60), 0)
        baseline = len(self._argv_lines(dstate))
        A = self._spawn(fake, dstate, art, "up", env)
        self._wait_mark(mark)
        B = self._spawn(fake, dstate, art, "preflight")
        time.sleep(0.8)  # B's window to do daemon work — must be zero
        during = self._argv_lines(dstate)
        self.assertEqual(len(during), baseline + 2,
                         "B reached the daemon while A held the lock: "
                         f"{during[baseline:]}")
        release.touch()
        self.assertEqual(A.wait(timeout=60), 0)
        self.assertEqual(B.wait(timeout=60), 0)
        disk = json.loads((art / "receipts.json").read_text())
        actions = [a["action"] for a in disk["actions"]]
        self.assertEqual(actions.count("compose.up"), 2)
        self.assertIn("preflight.info", actions)
        self.assertEqual(len(disk["generations"]), 2)

    def test_first_init_second_entry_serialised(self):
        """Scenario B: empty ledger — the second entry must serialise
        behind the first, not race its own initialisation or get
        refused only at first record."""
        tmp, fake, dstate, art = self._layout()
        release, mark, env = self._stall_env(tmp, "A")
        A = self._spawn(fake, dstate, art, "up", env)
        self._wait_mark(mark)
        B = self._spawn(fake, dstate, art, "up")
        time.sleep(0.8)
        during = self._argv_lines(dstate)
        composes = [l for l in during if '"compose"' in l]
        self.assertEqual(len(composes), 1,
                         "second up reached compose during first-init "
                         f"race: {during}")
        release.touch()
        self.assertEqual(A.wait(timeout=60), 0)
        self.assertEqual(B.wait(timeout=60), 0)
        disk = json.loads((art / "receipts.json").read_text())
        self.assertEqual(len(disk["generations"]), 2)

    def test_lock_wait_timeout_refuses_without_side_effects(self):
        """Scenario C: B times out non-zero, issued no daemon call, and
        A's history stays intact."""
        tmp, fake, dstate, art = self._layout()
        seed = self._spawn(fake, dstate, art, "up")
        self.assertEqual(seed.wait(timeout=60), 0)
        baseline = len(self._argv_lines(dstate))
        release, mark, env = self._stall_env(tmp, "A")
        A = self._spawn(fake, dstate, art, "up", env)
        self._wait_mark(mark)
        B = self._spawn(fake, dstate, art, "preflight",
                        {"DS_ISO_LOCK_WAIT_SEC": "0.4"})
        b_rc = B.wait(timeout=30)
        self.assertNotEqual(b_rc, 0)
        self.assertEqual(len(self._argv_lines(dstate)), baseline + 2,
                         "timed-out entry touched the daemon")
        release.touch()
        self.assertEqual(A.wait(timeout=60), 0)

    def test_abnormal_exit_releases_lock(self):
        """Scenario D: A dies mid-entry; the OS releases the lock and
        B acquires + reads the last valid ledger."""
        tmp, fake, dstate, art = self._layout()
        seed = self._spawn(fake, dstate, art, "up")
        self.assertEqual(seed.wait(timeout=60), 0)
        release, mark, env = self._stall_env(tmp, "A")
        A = self._spawn(fake, dstate, art, "up", env)
        self._wait_mark(mark)
        A.kill()
        A.wait(timeout=10)
        B = self._spawn(fake, dstate, art, "preflight")
        self.assertEqual(B.wait(timeout=60), 0)
        disk = json.loads((art / "receipts.json").read_text())
        self.assertTrue(disk["generations"])

    def test_independent_runs_not_globally_serialized(self):
        """Scenario E: a different run's artifact dir = a different
        lock file; it is not blocked by this run's held lock."""
        tmp, fake, dstate, art = self._layout()
        art2 = tmp / "art2"; art2.mkdir()
        release, mark, env = self._stall_env(tmp, "A")
        A = self._spawn(fake, dstate, art, "up", env)
        self._wait_mark(mark)
        B = self._spawn(fake, dstate, art2, "preflight")
        self.assertEqual(B.wait(timeout=30), 0,
                         "independent run serialised on a foreign lock")
        release.touch()
        self.assertEqual(A.wait(timeout=60), 0)


class TestIncompleteStatusPropagation(unittest.TestCase):
    """B2: a subcommand whose requested operations leave unclosed
    results exits non-zero. The ledger keeps completed, refused and
    unknown parts; nothing turns 'kept objects' into 'all clean'."""

    def _drive(self, tmp, state, *commands, run_id=RUN, daemon=DAEMON,
               owner="tester"):
        orig_argv, orig_lm, orig_rc = (
            sys.argv, iso.dg.load_manifest, iso.dg.run_cmd)

        def fake_run_cmd(args, cwd=None, timeout=120, stdin_text=None,
                         env=None):
            args = list(args)
            if args[:2] == ["docker", "compose"]:
                if "up" in args:
                    if state.fail_next_up:
                        state.fail_next_up = False
                        return 1, "", "compose up failed (fake)"
                    state.simulate_up()
                    return 0, "", ""
                return 0, "", ""
            return state.dispatch(args, env=env)

        iso.dg.load_manifest = lambda task: MANIFEST
        iso.dg.run_cmd = fake_run_cmd
        try:
            for cmd in commands:
                sys.argv = [
                    "ddl_golden_isolated.py",
                    "--run-id", run_id, "--daemon", daemon,
                    "--owner", owner, "--artifact-dir", str(tmp), cmd]
                iso.main()
        finally:
            sys.argv, iso.dg.load_manifest, iso.dg.run_cmd = \
                orig_argv, orig_lm, orig_rc

    def _mounted_world(self, tmp, state):
        """One anonymous volume mounted by a live client + one free
        named volume, both registered in the ledger."""
        state.add_container(
            "c-cl", "id-cl",
            mounts=[{"Type": "volume", "Name": "v-anon",
                     "Destination": "/var/lib/mysql"}])
        state.volumes["v-anon"] = {"driver": "local", "label": ""}
        named = f"{RUN}_{RUN}-mysql57-data"
        state.volumes[named] = {"driver": "local", "label": RUN}
        run = iso.IsoRun(RUN, DAEMON, "tester", tmp)
        state.bind(run)
        raw_path = pathlib.Path(tmp) / "receipts.json"
        run.receipts["daemon_endpoint"] = DAEMON
        run.receipts["daemon_identity"] = {"server_id": SID}
        run.receipts["generations"] = [{
            "at": "t0", "action": "compose.up",
            "containers": [{"name": "c-cl", "id": "id-cl"}],
            "networks": [],
            "volumes": [
                {"kind": "named", "declared": f"{RUN}-mysql57-data",
                 "name": named, "driver": "local"},
                {"kind": "anonymous", "name": "v-anon",
                 "mounted_by": "c-cl", "destination": "/var/lib/mysql"}]}]
        run._ledger_snapshot = None
        run._persist_ledger()
        return run

    def test_all_volumes_inuse_nonzero_all_kept(self):
        """B2.1: every registered volume still mounted — nothing is
        deleted and the command fails instead of reporting clean."""
        tmp = pathlib.Path(tempfile.mkdtemp())
        state = FakeDaemon()
        self._mounted_world(tmp, state)
        # mount the named volume too so EVERY registered object is in-use
        state.add_container(
            "c-db", "id-db",
            mounts=[{"Type": "volume",
                     "Name": f"{RUN}_{RUN}-mysql57-data",
                     "Destination": "/var/lib/mysql"}])
        with self.assertRaises(SystemExit):
            self._drive(str(tmp), state, "rmvolumes")
        self.assertIn("v-anon", state.volumes)
        self.assertEqual(state.removed_volumes(), [])
        disk = json.loads((tmp / "receipts.json").read_text())
        self.assertTrue(any(a["action"].startswith("volume.inuse")
                            for a in disk["actions"]))

    def test_mixed_inuse_and_free_nonzero_partial_progress(self):
        """B2.2: the free volume is consumed, the mounted one kept,
        and the command still reports non-zero."""
        tmp = pathlib.Path(tempfile.mkdtemp())
        state = FakeDaemon()
        self._mounted_world(tmp, state)
        with self.assertRaises(SystemExit):
            self._drive(str(tmp), state, "rmvolumes")
        named = f"{RUN}_{RUN}-mysql57-data"
        self.assertNotIn(named, state.volumes)   # free one consumed
        self.assertIn("v-anon", state.volumes)   # mounted one kept

    def test_volume_usage_check_failure_nonzero(self):
        """B2.3: unreadable mount view refuses AND reports non-zero —
        it is not treated as NotFound and the volume survives."""
        tmp = pathlib.Path(tempfile.mkdtemp())
        state = FakeDaemon()
        self._mounted_world(tmp, state)
        state.ps_error_volumes.add("v-anon")
        with self.assertRaises(SystemExit):
            self._drive(str(tmp), state, "rmvolumes")
        self.assertIn("v-anon", state.volumes)
        disk = json.loads((tmp / "receipts.json").read_text())
        self.assertTrue(any(a["action"] == "volume.refuse"
                            for a in disk["actions"]))

    def test_volume_rm_failure_nonzero_volume_kept(self):
        """Delete failure is an unclosed result, not a green exit."""
        tmp = pathlib.Path(tempfile.mkdtemp())
        state = FakeDaemon()
        self._mounted_world(tmp, state)
        state.containers.pop("c-cl")      # free the anon volume
        state.fail_volume_rm.add("v-anon")
        with self.assertRaises(SystemExit):
            self._drive(str(tmp), state, "rmvolumes")
        self.assertIn("v-anon", state.volumes)
        disk = json.loads((tmp / "receipts.json").read_text())
        self.assertTrue(any(a["action"] == "volume.rm.v-anon"
                            and a["rc"] == 1 for a in disk["actions"]))

    def test_up_capture_incomplete_nonzero_but_persisted(self):
        """B2.5: an up whose capture hits an inspect error persists
        the incomplete generation and then exits non-zero instead of
        continuing on a smaller world view."""
        tmp = pathlib.Path(tempfile.mkdtemp())
        state = FakeDaemon()
        state.error_containers.add("c-tidb85-client")
        with self.assertRaises(SystemExit):
            self._drive(str(tmp), state, "up")
        disk = json.loads((tmp / "receipts.json").read_text())
        self.assertTrue(disk["generations"])
        self.assertTrue(disk["generations"][-1].get("capture_incomplete"))
        # observed objects were kept, not silently dropped
        self.assertTrue(disk["generations"][-1]["containers"])

    def test_run_wrapped_capture_incomplete_propagates(self):
        """B2.6: the same failure inside the run wrapper's compose-up
        capture path raises — the runner cannot wrap it as full
        success. This drives the exact _capture_or_fail call
        ledger_compose makes."""
        tmp = pathlib.Path(tempfile.mkdtemp())
        state = FakeDaemon()
        state.add_container("c-mysql57", "id-m57")
        state.add_container("c-tidb85", "id-t85")
        state.add_container("c-tidb85-client", "id-clx")
        state.error_containers.add("c-tidb85-client")
        run = make_run(str(tmp), state)
        run._manifest = MANIFEST
        with self.assertRaises(SystemExit):
            run._capture_or_fail("compose.up.run")
        disk = json.loads((tmp / "receipts.json").read_text())
        self.assertTrue(disk["generations"][-1].get("capture_incomplete"))

    def test_retry_after_unblock_succeeds_history_kept(self):
        """B2.8: once the mount frees, a retry succeeds — with the
        earlier inuse history still recorded, not erased."""
        tmp = pathlib.Path(tempfile.mkdtemp())
        state = FakeDaemon()
        self._mounted_world(tmp, state)
        with self.assertRaises(SystemExit):
            self._drive(str(tmp), state, "rmvolumes")
        state.containers.pop("c-cl")   # foreign/unblocked mount gone
        self._drive(str(tmp), state, "rmvolumes")
        self.assertNotIn("v-anon", state.volumes)
        disk = json.loads((tmp / "receipts.json").read_text())
        actions = [a["action"] for a in disk["actions"]]
        self.assertIn("volume.inuse.v-anon", actions)
        self.assertIn("volume.rm.v-anon", actions)


if __name__ == "__main__":
    unittest.main()


class TestLoadFailureLockRelease(unittest.TestCase):
    """R15: a post-acquire load failure must release the held lock, so a
    corrected same-process retry is not refused by a phantom holder.

    `_drive` invokes iso.main() in this very process per command — the
    same-process entry convention the leak violates. On the unfixed
    tool each scenario dies at a 2s phantom timeout; on the fixed tool
    the retry acquires immediately."""

    def setUp(self):
        self._orig_wait = os.environ.get("DS_ISO_LOCK_WAIT_SEC")
        os.environ["DS_ISO_LOCK_WAIT_SEC"] = "2"

    def tearDown(self):
        if self._orig_wait is None:
            os.environ.pop("DS_ISO_LOCK_WAIT_SEC", None)
        else:
            os.environ["DS_ISO_LOCK_WAIT_SEC"] = self._orig_wait

    def _drive(self, tmp, state, *commands, run_id=RUN, daemon=DAEMON,
               owner="tester"):
        """Same in-process entry boundary as the earlier classes."""
        orig_argv, orig_lm, orig_rc = (
            sys.argv, iso.dg.load_manifest, iso.dg.run_cmd)

        def fake_run_cmd(args, cwd=None, timeout=120, stdin_text=None,
                         env=None):
            args = list(args)
            if args[:2] == ["docker", "compose"]:
                if "up" in args:
                    if state.fail_next_up:
                        state.fail_next_up = False
                        return 1, "", "compose up failed (fake)"
                    state.simulate_up()
                    return 0, "", ""
                return 0, "", ""
            return state.dispatch(args, env=env)

        iso.dg.load_manifest = lambda task: MANIFEST
        iso.dg.run_cmd = fake_run_cmd
        try:
            for cmd in commands:
                sys.argv = [
                    "ddl_golden_isolated.py",
                    "--run-id", run_id, "--daemon", daemon,
                    "--owner", owner, "--artifact-dir", str(tmp), cmd]
                iso.main()
        finally:
            sys.argv, iso.dg.load_manifest, iso.dg.run_cmd = (
                orig_argv, orig_lm, orig_rc)

    def test_corrupt_ledger_failure_releases_lock_same_process(self):
        """A: entry acquires the lock, then refuses on corrupt JSON.
        After the caller restores the fixture bytes, a same-process
        retry acquires the same sidecar instead of timing out."""
        tmp = tempfile.mkdtemp()
        state = FakeDaemon()
        self._drive(tmp, state, "up")
        receipts = pathlib.Path(tmp) / "receipts.json"
        good = receipts.read_bytes()
        sidecar = receipts.with_name(receipts.name + ".lock")
        self.assertTrue(sidecar.exists())   # proof the entry acquired
        ino = sidecar.stat().st_ino
        containers_before = dict(state.containers)
        receipts.write_text("{corrupt")
        with self.assertRaises(SystemExit) as cm:
            self._drive(tmp, state, "up")
        self.assertIn("unreadable", str(cm.exception))
        # refused entry: zero resource side effects, ledger bytes intact
        self.assertEqual(dict(state.containers), containers_before)
        receipts.write_bytes(good)
        self.assertTrue(sidecar.exists())
        self.assertEqual(sidecar.stat().st_ino, ino)   # never unlinked
        self._drive(tmp, state, "up")   # same process — must not time out
        disk = json.loads(receipts.read_text())
        self.assertEqual(len(disk["generations"]), 2)

    def test_binding_mismatch_failure_releases_lock_same_process(self):
        """B: entry acquires, then refuses on an owner binding mismatch.
        Correcting only the caller's argument lets the retry proceed —
        the ledger's recorded identity is never rewritten to match."""
        tmp = tempfile.mkdtemp()
        state = FakeDaemon()
        self._drive(tmp, state, "up")
        with self.assertRaises(SystemExit) as cm:
            self._drive(tmp, state, "up", owner="someone-else")
        self.assertIn("identity mismatch", str(cm.exception))
        self._drive(tmp, state, "up")
        disk = json.loads(
            (pathlib.Path(tmp) / "receipts.json").read_text())
        self.assertEqual(disk["owner"], "tester")
        self.assertEqual(len(disk["generations"]), 2)

    def test_non_systemexit_load_error_releases_lock(self):
        """C: a load-phase error that is NOT a SystemExit (mocked read
        failure) must also release — the boundary catches
        BaseException, not only the tool's own refusal paths."""
        tmp = tempfile.mkdtemp()
        state = FakeDaemon()
        self._drive(tmp, state, "up")
        receipts = pathlib.Path(tmp) / "receipts.json"
        good = receipts.read_bytes()
        with unittest.mock.patch.object(
                pathlib.Path, "read_bytes",
                side_effect=RuntimeError("disk gone (mock)")):
            with self.assertRaises(RuntimeError):
                self._drive(tmp, state, "up")
        # ledger untouched by the aborted entry
        self.assertEqual(receipts.read_bytes(), good)
        self._drive(tmp, state, "up")
        disk = json.loads(receipts.read_text())
        self.assertEqual(len(disk["generations"]), 2)

    @unittest.skipUnless(hasattr(iso, "fcntl") and iso.fcntl,
                         "POSIX flock required")
    def test_successful_load_keeps_lock_held_until_release(self):
        """Control: after load_run returns, the lock is still held for
        dispatch — the new exception boundary must not unlock early.
        A competing flock on a fresh fd must fail while held and
        succeed after release_ledger_lock."""
        tmp = tempfile.mkdtemp()
        state = FakeDaemon()
        self._drive(tmp, state, "up")
        args = types.SimpleNamespace(
            run_id=RUN, daemon=DAEMON, owner="tester",
            artifact_dir=tmp, receipts=None, command="up", task="T21")
        run = iso.load_run(args)
        try:
            self.assertIsNotNone(run._lock_fd)
            fd2 = os.open(run.lock_path, os.O_RDWR)
            try:
                with self.assertRaises(BlockingIOError):
                    iso.fcntl.flock(
                        fd2, iso.fcntl.LOCK_EX | iso.fcntl.LOCK_NB)
            finally:
                os.close(fd2)
        finally:
            run.release_ledger_lock()
        self.assertIsNone(run._lock_fd)
        fd3 = os.open(run.lock_path, os.O_RDWR)
        try:
            iso.fcntl.flock(fd3, iso.fcntl.LOCK_EX | iso.fcntl.LOCK_NB)
            iso.fcntl.flock(fd3, iso.fcntl.LOCK_UN)
        finally:
            os.close(fd3)


if __name__ == "__main__":
    unittest.main()
