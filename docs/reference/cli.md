# CLI Reference

`deltascope` is the primary operator surface for local audits, CI checks, and agent workflows.

## Commands

```text
deltascope audit
deltascope capabilities
deltascope config init
deltascope config lint
deltascope config show-default
deltascope rules list
deltascope rules show <rule-id>
deltascope rules search <keyword>
deltascope version
```

## Global Flags

- `--config`: path to YAML policy config
- `--dialect`: `mysql` or `tidb`
- `--fail-on`: `blocker`, `warning`, `notice`, or `none`
- `--format`: `markdown` or `json`
- `--quiet`: suppress non-result chatter
- `--version`: print only the semantic version

## Exit Codes

- `0`: audit completed below the failure threshold
- `1`: audit completed and reached `--fail-on`
- `2`: bad user input such as invalid flags, malformed SQL request data, or unreadable config
- `3`: runtime/internal failure
