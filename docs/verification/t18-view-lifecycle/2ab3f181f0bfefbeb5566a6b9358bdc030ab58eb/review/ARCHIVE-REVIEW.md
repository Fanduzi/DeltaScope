# Archive Review And Preservation Decision

The complete 44-file authored navigation diff is reviewed. Final facts,
entrypoint provenance, six raw Golden artifacts and policy copies match the
lead-checked sources. The two navigation wording corrections are listed in
the follow-up; no product or semantic oracle changes are required.

The 17 redactions stand. The lead independently checks every original and
sanitized hash, normalizes only the changed Python credential literal, and
checks command argv changes only at the four named credential environment
variables. All native SQL, assertions and remaining Python AST/argv stay
unchanged. The three sanitized scripts are historical source evidence, not
credentials to execute. Original sources remain privately available.

Initial validation attempts used an overbroad byte replacement and an
incomplete environment-variable allowlist. These rejected legitimate
redactions: the fixture value also occurs in non-credential account tokens,
and T18_MYSQL_PASSWORD was not initially included. The final checked helper
uses AST/argv scope and includes all four actual environment variable names.
These were verifier-scope errors, not changed query behavior or bad evidence.

Literal whitespace in copied output, patches and source snapshots is evidence,
not authored formatting. Do not normalize those original bytes. The
lead-authored archive-local attribute recipe exempts only 490 individually
listed, SHA-verified byte-identical copies already reported by Git. It does
not exempt authored Markdown, product code, a directory wildcard, or future
files. Preserve the finding list and recipe so this boundary is inspectable.

Final status is an independent-review candidate. There is no independent
acceptance, CI certification, merge, issue closure, release or T19 claim.
