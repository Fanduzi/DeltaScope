# Lead Checked Native Follow-ups

Exact lead-authored recipe ran once; no product oracle was delegated.
Raw command/stdout/stderr/rc for each stage is in this directory.
These are native observations, not product projection assertions.

All four fixed anchors:
- Hostless named DEFINER creates the view and records @% in SHOW CREATE.
  This does not establish that the account exists or is authorized.
- DROP TABLE v where v is a view returns1051 and leaves the view unchanged.
- DROP TABLE IF EXISTS t,v removes base table t, preserves view v, rc0.
- RENAME TABLE t TO renamed_t leaves stored view definition referencing t.
  The view is still a catalog entry, not automatically rebound.
- MODIFY t.id BIGINT produces observed view-column BIGINT too.
  Do not republish an old provider-observed INT after prospective source MODIFY.

Mixed DROP TABLE t,v and v,t without IF EXISTS:
- MySQL5.7.44 and TiDB8.5.0:1051, base t removed, view v preserved.
- MySQL8.0.46 and8.4.10:1051, base t and view v both preserved.
This is distinct from TiDB DROP VIEW wrong-kind stopping semantics.

Direct base attributes are not uniformly view properties:
- All anchors: base AUTO_INCREMENT is absent in the view's EXTRA.
- MySQL view id from NOT NULL AUTO_INCREMENT reports NOT NULL, default0;
  payload retains default5.
- TiDB view id/payload report nullable YES and NULL defaults.
Thus cloning whole base Column is unsound; named-only derived view columns
are safe without inventing cross-vendor defaults/constraints/types.

Every inspected COLUMNS read returned rc0 in this run. Recipe treats a read
error as an error receipt, never as empty columns. Own schemas were dropped
and remaining COUNT was0; canonical containers left healthy for reuse.
