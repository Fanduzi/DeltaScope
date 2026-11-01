import json
import subprocess
import sys

command = [
    "docker", "compose", "-p", "deltascope-ddl-golden",
    "-f", "docker/ddl-golden-compose.yaml", "config", "--format", "json",
]
process = subprocess.run(command, capture_output=True, text=True)
if process.stderr:
    sys.stderr.write(process.stderr)
if process.returncode:
    sys.exit(process.returncode)
config = json.loads(process.stdout)
expected = {
    "mysql57": ("deltascope-ddl-golden-mysql57", "mysql:5.7.44"),
    "mysql80": ("deltascope-ddl-golden-mysql80", "mysql:8.0.46"),
    "mysql84": ("deltascope-ddl-golden-mysql84", "mysql:8.4.10"),
    "tidb85": ("deltascope-ddl-golden-tidb85", "pingcap/tidb:v8.5.0"),
    "tidb85-client": ("deltascope-ddl-golden-tidb85-client", "mysql:8.4.10"),
}
result = {}
for name, (container_name, image) in expected.items():
    service = config["services"][name]
    if service["container_name"] != container_name or service["image"] != image:
        raise ValueError(f"unexpected identity for {name}")
    ports = service.get("ports", [])
    if any(port.get("host_ip") != "127.0.0.1" for port in ports):
        raise ValueError(f"non-loopback port configured for {name}")
    if name.startswith("mysql") and service.get("environment", {}).get("MYSQL_ROOT_PASSWORD") != "root":
        raise ValueError(f"unexpected fixture root configuration for {name}")
    health = service.get("healthcheck", {})
    result[name] = {
        "container_name": container_name,
        "image": image,
        "platform": service.get("platform"),
        "ports": ports,
        "depends_on": service.get("depends_on", {}),
        "healthcheck_configured": bool(health.get("test")),
        "healthcheck_interval": health.get("interval"),
        "healthcheck_timeout": health.get("timeout"),
        "healthcheck_retries": health.get("retries"),
        "healthcheck_start_period": health.get("start_period"),
    }
print(json.dumps(result, indent=2))
