#!/usr/bin/env python3
import argparse
import datetime
import hashlib
import json
from pathlib import Path
import re
import subprocess
import sys

OLD_REF = "8d1253e2fea0e1fac6bd150a8e46f436486f7205"
FILES = {
    "derived": "internal/application/audit/batch_state_t06a17_test.go",
    "provider": "internal/infrastructure/metadata/mysql/provider_t06a17_test.go",
    "alter": "internal/infrastructure/parser/tidb/extractor_t06a17_test.go",
}
TESTS = {
    "derived": "TestT06A17DerivedPreStatePreservesType",
    "provider": "TestT06A17ProviderParameterFidelity",
    "alter": "TestT06A17AlterColumnDefinitions",
}
CASES = [
    ("derived", "missing-column", "snapshot.Columns = snapshot.Columns[:len(snapshot.Columns)-1]", "derived pre-state:"),
    ("derived", "empty-columns", "snapshot.Columns = nil", "derived pre-state:"),
    ("derived", "duplicate-column", "snapshot.Columns[1] = snapshot.Columns[0]", "duplicate column"),
    ("provider", "missing-column", "snapshot.Columns = snapshot.Columns[:len(snapshot.Columns)-1]", "provider columns:"),
    ("provider", "empty-columns", "snapshot.Columns = nil", "provider columns:"),
    ("provider", "duplicate-column", "snapshot.Columns[1] = snapshot.Columns[0]", "duplicate column"),
    ("alter", "nil-definition", "statement.DDL.Alter[0].Column.Definition = nil", "Column.Definition is nil"),
    ("alter", "missing-action", "statement.DDL.Alter = nil", "alter actions = 0"),
    ("alter", "nil-column", "statement.DDL.Alter[0].Column = nil", "alter.Column is nil"),
    ("alter", "wrong-target", 'statement.DDL.Alter[0].Column.Definition.Name = "not_expected"', "Definition.Name"),
    ("alter", "duplicate-action", "statement.DDL.Alter = append(statement.DDL.Alter, statement.DDL.Alter[0])", "alter actions = 2"),
]


def now():
    return datetime.datetime.now(datetime.timezone.utc).isoformat()


def git(repo, *args):
    return subprocess.check_output(["git", *args], cwd=repo)


def inject(source, kind, statement):
    if kind == "provider":
        anchor = '\tif err != nil {\n\t\tt.Fatalf("LoadTableSnapshot: %v", err)\n\t}\n'
        if source.count(anchor) != 1:
            raise ValueError("provider injection anchor must occur exactly once")
        return source.replace(anchor, anchor + "\t" + statement + "\n", 1)
    if kind == "derived":
        anchor = '\t\t\tif err != nil {\n\t\t\t\tt.Fatalf("pre-state: %v", err)\n\t\t\t}\n'
        if source.count(anchor) != 1:
            raise ValueError("derived injection anchor must occur exactly once")
        return source.replace(anchor, anchor + "\t\t\t" + statement + "\n", 1)
    function_anchor = "func TestT06A17AlterColumnDefinitions("
    if source.count(function_anchor) != 1:
        raise ValueError("ALTER test function must occur exactly once")
    function_start = source.index(function_anchor)
    prefix, body = source[:function_start], source[function_start:]
    pattern = r"(?m)^([\t ]*)statement := t06a2Extract\(t, (?:spec\.DialectMySQL|dialect), tc\.sql\)\n"
    matches = list(re.finditer(pattern, body))
    if len(matches) != 1:
        raise ValueError("ALTER injection anchor must occur exactly once inside its test")
    match = matches[0]
    return prefix + body[:match.end()] + match.group(1) + statement + "\n" + body[match.end():]


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--repo", default=str(Path(__file__).resolve().parents[2]))
    parser.add_argument("--out", required=True)
    parser.add_argument("--strict-worktree", action="store_true")
    parser.add_argument("--kind", choices=tuple(FILES), help="narrow a development run; omit for the final complete proof")
    args = parser.parse_args()
    repo = Path(args.repo).resolve(strict=True)
    out = Path(args.out).resolve()
    out.mkdir(parents=True, exist_ok=False)
    head = git(repo, "rev-parse", "HEAD").decode().strip()
    sources = {}
    for kind, relative in FILES.items():
        old = git(repo, "show", f"{OLD_REF}:{relative}").decode()
        strict = (repo / relative).read_text() if args.strict_worktree else git(repo, "show", f"{head}:{relative}").decode()
        sources[kind] = {"old": old, "strict": strict}
    cases = [case for case in CASES if args.kind is None or case[0] == args.kind]
    records = []
    for kind, name, mutation, assertion in cases:
        relative = FILES[kind]
        for version in ("old", "strict"):
            directory = out / f"{kind}-{name}-{version}"
            directory.mkdir()
            source = sources[kind][version]
            mutated = inject(source, kind, mutation)
            overlay_source = directory / Path(relative).name
            overlay_source.write_text(mutated)
            overlay = directory / "overlay.json"
            overlay.write_text(json.dumps({"Replace": {str(repo / relative): str(overlay_source)}}, indent=2) + "\n")
            command = ["go", "test", "-count=1", "-overlay", str(overlay), "./" + str(Path(relative).parent), "-run", "^" + TESTS[kind] + "$", "-v"]
            before = git(repo, "status", "--porcelain=v1", "--untracked-files=all", "--", relative).decode()
            metadata = {
                "argv": command, "cwd": str(repo), "started_at": now(),
                "head_sha": git(repo, "rev-parse", "HEAD").decode().strip(),
                "source_ref": OLD_REF if version == "old" else ("uncommitted-worktree" if args.strict_worktree else head),
                "source_path": relative,
                "source_sha256": hashlib.sha256(source.encode()).hexdigest(),
                "overlay_source_sha256": hashlib.sha256(mutated.encode()).hexdigest(),
                "constructed_fault": mutation,
                "task_file_status_before": before,
            }
            with (directory / "stdout").open("wb") as stdout, (directory / "stderr").open("wb") as stderr:
                rc = subprocess.run(command, cwd=repo, stdout=stdout, stderr=stderr).returncode
            stdout = (directory / "stdout").read_text()
            stderr = (directory / "stderr").read_text()
            combined = stdout + stderr
            metadata.update({"finished_at": now(), "returncode": rc,
                "head_sha_after": git(repo, "rev-parse", "HEAD").decode().strip(),
                "task_file_status_after": git(repo, "status", "--porcelain=v1", "--untracked-files=all", "--", relative).decode()})
            (directory / "metadata.json").write_text(json.dumps(metadata, ensure_ascii=False, indent=2) + "\n")
            (directory / "returncode").write_text(str(rc) + "\n")
            matched = rc == 0 if version == "old" else (rc == 1 and "--- FAIL: " + TESTS[kind] in stdout and assertion in stdout)
            matched = matched and "panic:" not in combined and "build failed" not in combined and "no tests to run" not in combined
            matched = matched and metadata["head_sha"] == metadata["head_sha_after"] == head and before == metadata["task_file_status_after"]
            records.append({"kind": kind, "case": name, "assertions": version, "returncode": rc, "expected_observation": matched, "path": str(directory)})
            print(f"{kind}/{name}/{version}: rc={rc} expected_observation={matched}", flush=True)
    expected_records = 2 * len(cases)
    summary = {"head_sha": head, "old_assertion_ref": OLD_REF,
        "strict_source": "uncommitted-worktree" if args.strict_worktree else head,
        "selected_kind": args.kind, "expected_records": expected_records,
        "constructed_faults_only": True, "native_database_fault_injection": False,
        "records": records, "all_expected_observations": expected_records > 0 and len(records) == expected_records and all(row["expected_observation"] for row in records)}
    (out / "summary.json").write_text(json.dumps(summary, ensure_ascii=False, indent=2) + "\n")
    return 0 if summary["all_expected_observations"] else 1


if __name__ == "__main__":
    sys.exit(main())
