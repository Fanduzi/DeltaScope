import json
import subprocess
import sys

root = "/tmp/docker-cleanup-execution-20261101"
with open(root + "/after.json", encoding="utf-8") as stream:
    expected = json.load(stream)
ids = [item["id"] for item in expected]
run = subprocess.run(["docker", "inspect", *ids], capture_output=True, text=True)
if run.returncode:
    print("container inspect failed:", run.stderr.strip(), file=sys.stderr)
    sys.exit(1)
actual = {item["Id"]: item for item in json.loads(run.stdout)}
errors = []
for item in expected:
    found = actual.get(item["id"])
    if found is None:
        errors.append(f"missing {item['id']}")
        continue
    volumes = sorted(mount["Name"] for mount in found["Mounts"] if mount["Type"] == "volume")
    if not found["State"]["Running"] or found["Name"].lstrip("/") != item["name"] or volumes != sorted(item["volumes"]):
        errors.append(f"changed running state, name, or named mounts: {item['id']}")
with open(root + "/volumes-after.stdout.txt", encoding="utf-8") as stream:
    volume_baseline = {line.strip() for line in stream if line.strip()}
run = subprocess.run(["docker", "volume", "ls", "--format", "{{.Name}}"], capture_output=True, text=True)
if run.returncode:
    print("volume listing failed:", run.stderr.strip(), file=sys.stderr)
    sys.exit(1)
missing = volume_baseline - set(run.stdout.splitlines())
if missing:
    errors.append(f"missing previously preserved volumes: {sorted(missing)}")
if errors:
    print("\n".join(errors), file=sys.stderr)
    sys.exit(1)
print(f"{len(expected)} protected containers running with unchanged named mounts; {len(volume_baseline)} preserved volumes present")
