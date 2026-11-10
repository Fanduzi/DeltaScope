# T26-R2 SERVER semantic responsibility → safe-field mapping

> **R3 addendum (T26-R3, implementation stage).** The semantic map below
> is the *target* contract — none of it is implemented in R3. R3 ships
> `spec.ServerDeclaration` carrying only: kind, verbatim name, wrapper
> identity, option keys + value kinds + counts, duplicate keys, and the
> name-spelling rune count — **option literal values never enter the
> derived declaration and are never emitted** (the statement's internal
> RawSQL still holds the authored text — the never-stored claim scopes
> to declaration fields and serialized outputs, not the raw parse
> record). The
> emitted outcome is uniformly `coverage=incomplete` + named unaudited
> boundary (`boundary=unaudited`, `object=server`) on MySQL and vendor
> boundary on TiDB. No findings, no lifecycle post-state, no metadata
> reader — "server exists" stays provider-not-loaded until a later
> milestone loads it.

Proposal status: **awaiting reviewer ruling**. Answers section 四 of the
R2 work order — what is promised, which facts each promise needs, how the
parse stage obtains them, the safe internal representation, and which
findings/gaps/coverage/post-state each fact drives. Values are derived
**at parse time and then discarded**: recognition runs on the authentic
input; the kept representation is a derived fact set, never a mutated
input treated as semantics.

## 1. Responsibility map

| # | promised judgment | required fact(s) | parse-stage acquisition | safe internal rep | drives | approved-boundary / not-yet-covered |
|---|---|---|---|---|---|---|
| S1 | statement is a SERVER lifecycle DDL (identity + kind) | kind ∈ {create,alter,drop}; `server_name` | whole-input recognizer over authentic AST/lexemes | `Kind`, `Name` (authored, **verbatim** — identity, not a secret) | statement identity, coverage base, ordered-state key | name retained per T25 precedent (names are identities, not credentials); ruling asked if name itself is safe — proposed yes |
| S2 | statement is MySQL-dialect SERVER form vs vendor boundary | dialect flag | request dialect + grammar family | none extra | TiDB row → `unsupported` + `metadata.boundary=vendor` + `unsupported_statement` diagnostic + `coverage=incomplete` | boundary contract = T25 precedent |
| S3 | `FOREIGN DATA WRAPPER` clause recognized + represented | wrapper presence, wrapper name | recognizer captures wrapper token verbatim | `Wrapper string` (name only) | absent on CREATE = malformed (grammar); present value participates in boundary | boundary: whether non-`mysql` wrappers are refused or only noticed — docs don't restrict; propose notice-level, not refusal. Connection/engine semantics stay an unaudited aspect |
| S4 | OPTIONS clause shape | option key set, per-key value-kind (char-literal vs numeric-literal), duplicate keys | recognizer walks option pairs | `Options []ServerOption{Key string, ValueKind string}` + `DuplicateKeys []string`; **no values** | duplicates → notice/warning (MySQL last-wins? **[PEND]**); invalid key → parser error (grammar rejects unknown keys? **[PEND]** — strict vs permissive) | connection semantics of values never audited |
| S5 | value-related conditions that change promised adjudication | PORT numeric range 0–9999; char-literal length >64 (silent-truncate note) | literal classification + length measure + numeric range at parse time | derived booleans/int only: `PortOutOfRange bool`, `TruncatedKeys []string` | range violation → warning/refusal per ruling; truncation → notice (identity changes after silent truncation affects ordered-state key!) | silent truncation means authored name ≠ stored name — fact kept as `EffectiveName` derived (name truncated to 64) — **this IS semantics**, cannot be dropped |
| S6 | CREATE/ALTER/DROP lifecycle transition correctness | object existence premise (`server.existence`), authored intent | metadata fact (H-doc) | `server.existence` three-valued | findings: dup-create blocker, absent-alter/drop refusal vs no-op, gaps on unknown | existence=metadata, not parse |
| S7 | ALTER updates which known facts | option key set after alter | S4 parse + S6 premise | post-state records "options updated" as opaque marker — no values | post-state `servers[name].options_revision` bumped; never stores option map contents | full option-modeling (storing HOST etc.) is data-minimalization violation AND unnecessary |
| S8 | "生命周期可审计" precise states | — | — | ordered-state projection `state.servers[name] ∈ {present, absent}` + opaque option marker | post-state for: create→present; drop→absent; alter→present(marker); refused forms→unchanged | **exactly** these — not remote reachability, not FEDERATED table binding, not connection config simulation |
| S9 | boundary/unparseable forms keep identity+contract | per-statement kind + position even when not fully audited | recognizer returns typed-declaration or clean parser_error | typed boundary decl keeps `Kind`, `Name`, position | `unsupported` entry + `coverage=incomplete` + `unsupported_statement` diagnostic | a parser_error today loses identity — closing that is implementation scope |
| S10 | underdetermined statements fabricate no post-state | premise completeness | adjudication over S1–S7 | gaps list | unknown existence → `coverage=unverified` + gap, post-state unchanged | T04 contract: unknown never promotes to absent |

## 2. Items the reviewer explicitly flagged — answers

- **Wrapper**: kept verbatim as a name (S3). Connection/engine semantics
  of the wrapper value = unaudited aspect, same class as T25 engine
  boundary. Non-`mysql` wrapper values → notice (documented value is
  `mysql`; other values are legal-but-FEDERATED-optional) — propose
  notice-level finding `[proposed] ddl.mysql.server.create.wrapper.notice`.
- **Name handling**: authored case retained; identity key = case-
  insensitive fold per docs; silent truncation → `EffectiveName` derived
  at parse time, ordered-state keys use effective name; a name >64
  chars is legal-but-truncated, so findings reference authored name while
  post-state keys use effective name (both recorded).
- **Duplicate options**: retained as `DuplicateKeys`; MySQL behavior for
  duplicate OPTION keys **[PEND]** (last-wins vs error) — flagged for
  oracle, not assumed.
- **ALTER post-state**: only the option-marker revision + existence —
  never per-key values.
- **Value-derived facts that survive**: `PortOutOfRange`, `TruncatedKeys`,
  `DuplicateKeys`, per-key `ValueKind`, option count. Everything else
  about values is dropped at the recognizer boundary — before any
  serializer or fallback can see it.

## 3. What is NOT promised (stays boundary/deferred)

- Remote reachability, credential validity, HOST/USER/PASSWORD semantic
  correctness — engine boundary, unaudited aspect.
- FEDERATED table references (`CONNECTION='name'`) — different feature
  family; deferred to a later task (registered).
- `mysql.servers` row-model beyond existence — option-level state is an
  opaque marker, by data-minimalization design.
- Privilege enforcement (`SUPER`) — named unaudited aspect per existing
  pattern.
