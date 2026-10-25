# T11-R2 red evidence — real baseline run

`baseline_red.log` is the **first-run red** captured on the pre-fix
product (parent `d0c7cf33`, i.e. the R2 diff base) after the R2 test
files were authored — not a revert-stub replay. Command:

```
go test ./internal/application/audit/ -run 'TestT11R2' -v
```

Result: `rc=1`, **25 `--- FAIL` markers over 17 parent test functions**,
covering all four R2 groups. Fixtures used only legal declarations
(valid parent keys, real column definitions, provider-proven bindings);
every failure is a behavioral red, not a fixture or compile error —
the final run of the same tests on `988560c2` is green (52 PASS).

## Failing leaves by group

**R2-A — same-statement declared column facts (7 parents)**

- `TestT11R2MySQLAddColumnSuppliesPKMember` — pre-fix: 0 findings, 0
  gaps, complete/pass because the pre-state never had `id`.
- `TestT11R2MySQLModifyColumnSuppliesPKMember` /
  `TestT11R2MySQLModifyColumnNarrowsPKMember` — pre-fix read the old
  `INT` fact (false blocker / missed gap shape).
- `TestT11R2MySQLDeclaredMemberFeedsSiblingRules/{unsigned,auto_increment}_on_declared_int`
  — sibling rules shared the pre-state-only read.
- `TestT11R2MySQLRenameCarriesPreStateFact`,
  `TestT11R2MySQLDroppedMemberIsUndecidable` — rename/drop outcomes had
  no explicit verdict.
- `TestT11R2TiDBSameStatementDeclarationUnprovable/{member_declared,member_modified}_in_statement`
  — TiDB must not resolve a member the statement touches (native
  1072/8200); pre-fix resolved pre-state facts unconditionally.

**R2-D — complete-vs-partial referenced key (3 parents)**

- `TestT11R2RestrictOnPartialKeysRefuse/{partial_unique,partial_primary_key}`
  — pre-fix published the FK on a leftmost prefix under observed
  restrict ON; the successor `DROP FOREIGN KEY` then answered
  present instead of the native 1091.
- `TestT11R2RestrictUnreadPartialUndecidable`,
  `TestT11R2UnknownVersionPartialUndecidable` — unobserved flag /
  unproven version must stay undecidable, not publish.

**R2-E — self-reference and existing-binding dependencies (2 parents)**

- `TestT11R2SelfReferenceSoleCovererRefuses/{mysql,tidb}` — the
  referenced arm skipped the drop target's own entry, so the first
  `DROP PRIMARY KEY` landed and the second fabricated a missing PK.
- `TestT11R2ExistingNonStandardBindingSurvivesRestrict` — the
  creatability answer (secondary ineligible under restrict ON) was
  reused for the loaded binding; the proven-dependent member dropped.

**R2-F — DROP CHECK|CONSTRAINT kind coverage (5 parents)**

- `TestT11R2DropConstraintUnvouchesUniqueMember`,
  `TestT11R2DropConstraintUnvouchesPrimaryKeyObject`,
  `TestT11R2UnknownVersionDropConstraintUnvouches` — pre-fix kept the
  proven unique member / PK object readable after `DROP CONSTRAINT
  <name>` even though the unrecoverable keyword makes the fate
  unprovable.
- `TestT11R2DropConstraintPreservesUntargetedFacts`,
  `TestT11R2DropCheckKeywordSameAmbiguity` — pin the bounded shape:
  only the affected collection unvouches.

## Positive controls green at baseline (unchanged contracts)

`TestT11R2RestrictOnFullKeysLand`, `TestT11R2RestrictOffPartialKeysLand`,
`TestT11R2Pre84PartialKeysLand`, `TestT11R2TiDBPartialKeysLand`,
`TestT11R2SelfReferenceChildSideSoleCoverer`,
`TestT11R2ExistingBindingAlternativeCovererFrees`,
`TestT11R2UnvouchedChildKeepsDependencyUnprovable`,
`TestT11R2DropConstraintKeepsSecondaryMember`,
`TestT11R2DropConstraintMissingNameKeepsFacts`,
`TestT11R2TiDBDropConstraintNoOp`,
`TestT11R2MySQL57DropConstraintSyntaxRefusal`,
`TestT11R2DeclaredMemberRuleOff`, `TestT11R2MySQLAddColumnCompliantPKMember`,
`TestT11R2MySQLUnrelatedColumnActionKeepsPreStateFact` and the
remaining TiDB sub-arms — behavior was already correct and must stay so.
