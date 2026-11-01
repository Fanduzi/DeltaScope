# Credential scan and redactions

The source inventory was scanned without emitting credential values. The scan
targets the approved disposable fixture credential in password assignments,
command arguments and DSNs, plus common private-key and token forms. It is a
targeted scan, not a guarantee that all possible secrets are absent.

Seventeen copied files required redaction: fourteen historical command receipts
and three native probe source files. The original probe sources contained
fixture-credential literals, so their archived copies are **not byte-identical**;
only those literals were replaced with `<redacted>`. The scripts retain their
historical role and are not executable credentials. Original hashes, sanitized
hashes and source paths are recorded in `redaction-ledger.json`; the originals
remain private. No Golden artifact bytes or synthetic privacy sentinels were
redacted.
