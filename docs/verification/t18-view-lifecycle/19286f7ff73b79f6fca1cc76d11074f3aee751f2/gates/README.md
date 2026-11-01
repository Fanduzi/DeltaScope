# Final gates

Every receipt contains the executed command, captured stdout/stderr, return
code, and the pre/post command HEAD. All non-lint gates completed with rc=0 on
`19286f7ff73b79f6fca1cc76d11074f3aee751f2`.

The scoped and full-repository lint commands return rc=1 because they reproduce
inherited diagnostics. `20-lint-comparison` compares diagnostic identity by
file, complete linter message, and multiplicity against the accepted T17-tail
baseline and reports zero added or removed diagnostics.
