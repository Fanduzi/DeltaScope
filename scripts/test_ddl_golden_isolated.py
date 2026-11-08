#!/usr/bin/env python3
"""Offline contract tests for the isolated golden lifecycle entry.

No Docker daemon is contacted: IsoRun.docker is replaced by a stateful
fake daemon, and every assertion inspects the argv/env the entry would
have issued plus the verdict the ledger reached. Covers the
authorization seams: endpoint pinning, project scoping, append-only
generation ledger, refuse-mode teardown, and the absence of the stock
`down -v --remove-orphans` path.
"""

import importlib.util
import json
import os
import pathlib
import sys
import tempfile
import unittest

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


if __name__ == "__main__":
    unittest.main()
