#!/usr/bin/env python3
"""Offline contract tests for the isolated golden lifecycle entry.

No Docker daemon is contacted: ddl_golden.run_cmd is replaced by a
recording stub, and every assertion inspects the argv/env the entry would
have issued. Covers the authorization seams: endpoint pinning, project
scoping, receipt identity, per-phase teardown, and the absence of the
stock `down -v --remove-orphans` path.
"""

import importlib.util
import io
import json
import os
import pathlib
import sys
import tempfile
import unittest
import contextlib

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


def make_run(tmp, calls, responses=None):
    responses = responses or {}

    def fake_run_cmd(args, cwd=None, timeout=120, stdin_text=None, env=None):
        key = " ".join(args[:3])
        calls.append({"args": list(args), "env": dict(env or {})})
        return responses.get(key, (0, "", ""))

    run = iso.IsoRun(RUN, DAEMON, "tester", tmp)
    run._task = "T21"
    run.dg = type("DGStub", (), {})()
    run.dg.run_cmd = fake_run_cmd
    run.dg.COMPOSE_FILE = ROOT / "docker" / "ddl-golden-compose.yaml"
    run.dg.ROOT_DIR = ROOT
    run.docker = lambda *a, timeout=180: fake_run_cmd(["docker", *a], timeout=timeout, env=run.env)
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
        finally:
            os.environ.pop("DOCKER_CONTEXT", None)


class TestComposeScoping(unittest.TestCase):
    def test_compose_uses_run_project_and_override(self):
        calls = []
        run = make_run(tempfile.mkdtemp(), calls)
        run.write_override(MANIFEST)
        run.compose("up", "-d", "mysql57")
        argv = calls[0]["args"]
        self.assertEqual(argv[:4], ["docker", "compose", "-p", RUN])
        self.assertIn("-f", argv)
        files = [argv[i + 1] for i, a in enumerate(argv) if a == "-f"]
        self.assertEqual(len(files), 2)
        self.assertTrue(files[1].endswith("compose.override.yaml"))
        self.assertEqual(calls[0]["env"]["DOCKER_HOST"], DAEMON)

    def test_override_labels_and_named_volumes(self):
        tmp = tempfile.mkdtemp()
        run = make_run(tmp, [])
        override = run.write_override(MANIFEST).read_text()
        self.assertIn("org.deltascope.task: T21", override)
        self.assertIn(f"org.deltascope.run: {RUN}", override)
        self.assertIn("org.deltascope.owner: tester", override)
        self.assertIn(f"{RUN}-mysql57-data:/var/lib/mysql", override)
        self.assertNotIn(f"{RUN}-tidb85-data", override)


class TestReceiptIdentity(unittest.TestCase):
    def test_up_records_only_run_labeled_containers(self):
        tmp = tempfile.mkdtemp()
        calls = []

        def inspect_ok(args, cwd=None, timeout=120, stdin_text=None, env=None):
            calls.append(list(args))
            name = args[-1]
            if name == "c-mysql57":
                return 0, "abc123|img|" + RUN, ""
            if name == "c-tidb85":
                return 0, "def456|img|" + RUN, ""
            return 0, "zzz|img|other-run", ""

        run = iso.IsoRun(RUN, DAEMON, "tester", tmp)
        run._task = "T21"
        run.docker = lambda *a, timeout=180: inspect_ok(
            ["docker", *a], env=run.env)
        run.compose = lambda *a, timeout=300: (0, "", "")
        containers = run.up(MANIFEST, "T21")
        self.assertEqual([c["name"] for c in containers],
                         ["c-mysql57", "c-tidb85"])
        self.assertTrue(all(c["id"] for c in containers))

    def test_verified_ids_refuses_label_mismatch(self):
        tmp = tempfile.mkdtemp()
        run = make_run(tmp, [])
        run.receipts["containers"] = [{"name": "c-x", "id": "i-x"}]
        run.docker = lambda *a, timeout=180: (0, "other-run", "")
        with self.assertRaises(SystemExit):
            run._verified_ids(run.receipts["containers"])


class TestTeardownScope(unittest.TestCase):
    def test_teardown_targets_receipts_only(self):
        tmp = tempfile.mkdtemp()
        calls = []

        def dispatch(args, cwd=None, timeout=120, stdin_text=None, env=None):
            calls.append(list(args))
            if args[1] == "inspect":
                return 0, RUN, ""
            if args[1:3] == ["volume", "inspect"]:
                return 0, f"local|{RUN}|/var/lib/docker/volumes/x/_data", ""
            return 0, "", ""

        run = iso.IsoRun(RUN, DAEMON, "tester", tmp)
        run.docker = lambda *a, timeout=180: dispatch(["docker", *a], env=run.env)
        run.receipts["containers"] = [{"name": "c1", "id": "id-c1"}]
        run.receipts["networks"] = ["net-1"]
        run.receipts["volumes"] = [{"name": f"{RUN}-mysql57-data"}]
        run.stop()
        run.rm()
        run.rmvolumes()
        verbs = [" ".join(c[1:3]) for c in calls]
        self.assertIn("stop id-c1", verbs)
        self.assertIn("rm id-c1", verbs)
        self.assertIn("network rm", verbs)
        self.assertIn("volume rm", verbs)
        joined = " ".join(" ".join(c) for c in calls)
        for forbidden in ("down -v", "--remove-orphans", "prune", "rm -v"):
            self.assertNotIn(forbidden, joined)

    def test_volume_with_foreign_run_label_not_deleted(self):
        tmp = tempfile.mkdtemp()
        calls = []

        def dispatch(args, cwd=None, timeout=120, stdin_text=None, env=None):
            calls.append(list(args))
            if args[1:3] == ["volume", "inspect"]:
                return 0, "local|foreign-run|/mp", ""
            return 0, "", ""

        run = iso.IsoRun(RUN, DAEMON, "tester", tmp)
        run.docker = lambda *a, timeout=180: dispatch(["docker", *a], env=run.env)
        run.receipts["volumes"] = [{"name": "foreign-vol"}]
        run.rmvolumes()
        self.assertFalse(
            any(c[1:3] == ["volume", "rm"] for c in calls),
            "volume without this run's label must not be deleted",
        )

    def test_stock_cleanup_never_runs_down_v(self):
        tmp = tempfile.mkdtemp()
        calls = []
        run = make_run(tmp, calls)
        run.receipts["containers"] = [{"name": "c1", "id": "id-c1"}]
        run.receipts["networks"] = []
        run.docker = lambda *a, timeout=180: (0, RUN, "")
        result = run.patched_cleanup()
        joined = " ".join(" ".join(c["args"]) for c in calls)
        self.assertNotIn("down", joined)
        self.assertNotIn("--remove-orphans", joined)
        self.assertTrue(result.get("isolated"))


if __name__ == "__main__":
    unittest.main()
