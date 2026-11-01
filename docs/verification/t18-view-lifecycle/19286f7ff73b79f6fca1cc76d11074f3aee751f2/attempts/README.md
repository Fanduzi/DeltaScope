# Diagnostic attempts

These artifacts document environment-injection failures before the successful
T18 run. They are not passing semantic evidence and are not mixed into the
`golden/` results.

- `t18-missing-fixture-env`: the shell lacked the manifest-declared
  `T18_MYSQL_PASSWORD`, so database cases could not run `SELECT VERSION()`.
- `t18-missing-cli-password-env`: the shell lacked the manifest-declared
  `DS_T18_GOLDEN_PW`, so metadata CLI cases exited with `invalid password
  source` after database setup had already succeeded.

The passing evidence is `golden/T18/artifact.json` from the third run, where
both required environment names were supplied without recording their values.
