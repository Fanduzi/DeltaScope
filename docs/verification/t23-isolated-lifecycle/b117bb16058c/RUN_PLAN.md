# RUN_PLAN — T23 isolated-entry bounded real lifecycle (R16 order)

- Fixed tool: b117bb16058cecaaa354af3997948b32d1761921
  sha256 862b686869aa4fe12d205cdf719c4d4e2677d46ba6b09cc2394337b920fc4d59
- Profile: ds-t21-20261107t1158z-e514c8 (colima, existing, dedicated)
- Expected daemon id: 35b66e6e-49f3-4615-b556-6a10062d4737
- Expected endpoint: unix:///Users/fan/.colima/ds-t21-20261107t1158z-e514c8/docker.sock
- Manifest/task: T21 (fixture identity only; no product SQL matrix, no `run` subcommand)
- Resource cap: ≤5 concurrent task containers, ≤10 cumulative container identities, exactly one recreation cycle

## Recreation path decision

The accepted entry exposes only preflight/up/run/stop/rm/rmvolumes;
`up` fans out `compose up -d` over ALL declared services — no
service-scoped or force-recreate flag exists, and the order forbids
calling compose recreate outside the entry's lock/capture protection.
=> ALTERNATIVE PATH (sanctioned): one `stop` + `rm` of the run's
registered containers/networks (NO rmvolumes), then `up` again under
the same RUN_ID + receipts. The client container gets a genuinely new
ID and a fresh anonymous volume; the previous generation keeps the
old anonymous volume's creation attribution in the ledger.

## Sequence

1. preflight + up  (gen0: first registration)
2. up             (gen1: restore ledger, reobserved, no config change)
3. stop; rm       (registered objects removed, volumes retained)
4. up             (gen2: real recreation — new IDs, new anon volume)
5. stop; rm       (final container/network teardown)
6. rmvolumes      (cumulative V: all generations' volumes)
7. read-only reconciliation via approved endpoint; stop profile

## Stop rules

Any non-zero stage: preserve attempt/receipts/observations, no
re-creation, ledger-scoped cleanup only via the entry, report
unclosed items verbatim. No manual docker rm as a fallback.
