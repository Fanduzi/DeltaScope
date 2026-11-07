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
            name, c = self._find_container(a[-1])
            if c is None:
                return 1, "", f"Error: No such container: {a[-1]}"
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
            names = [f"{c['id'][:12]}\n" for c in self.containers.values()
                     if c["label"] == RUN]
            return 0, "".join(names), ""
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
