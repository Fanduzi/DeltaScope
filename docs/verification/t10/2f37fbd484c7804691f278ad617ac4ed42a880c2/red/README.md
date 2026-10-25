# T10-R5 red/green evidence

`test-t10r5` red proof: the four new `*_t10r5_test.go` files were copied
verbatim onto a clean `git worktree` of the R4 archive tree `1d60d578`
(product code = R4 product `16915291`) and executed — every file compiled
and every behavioral assertion FAILED against the R4 semantics. A
compile failure would have been reported as a non-test error; none
occurred, so every `--- FAIL` line is behavioral red.

- `t10r5-tests-on-r4-product.log` — 28 top-level test FAILs across
  spec / rule-ddl / metadata-mysql / audit on the R4 product tree.
- `t10r5-tests-on-r5-worktree.log` — the same `-run TestT10R5` suite on
  the R5 working tree: 4 packages `ok`, zero failures.

The original in-session red capture was lost with the pre-commit
session log; the worktree replay above reproduces the same red on the
same test bytes and is fully inspectable.
