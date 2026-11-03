# Attempt 01 — golden compose network pool exhaustion

`make ddl-golden TASK=T19 ARTIFACT_DIR=/tmp/ddl-golden-t19r1` on HEAD
`efc77a9c`. Docker daemon reported `all predefined address pools have
been fully subnetted` when compose tried to create
`deltascope-ddl-golden_default` — IPAM exhaustion from unrelated
foreign networks on this host. 0 cases executed; not a code failure.

Accommodation for attempt 02 (documented, no oracle change): the
project network was pre-created with an explicit subnet outside the
default pools (`docker network create --subnet 10.203.0.0/16
deltascope-ddl-golden_default`). `compose down -v --remove-orphans`
still owns its lifecycle during cleanup.
