# T15 gates directory

`INDEX.md` is the gate table: 19 required gates plus committed-diff L2
checks, the full-repo lint diagnostic, and the codegraph impact analysis,
each archived as `<name>.command.txt` / `.stdout.txt` / `.stderr.txt` /
`.rc.txt`. `l2_committed_range_check.sh` is the frozen L2 checker the
committed-diff rows ran — kept verbatim so the recorded commands stay
reproducible byte-for-byte.
