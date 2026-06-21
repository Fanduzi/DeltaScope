---
name: milestone-grill
description: "Use when proposing, evaluating, or prioritizing the next milestone, release theme, roadmap item, or version scope. Forces evidence-first self-grilling before naming a milestone, especially after a release, when asked \"what next\", or when a proposed milestone may duplicate existing capability."
---

# Milestone Grill

Use this skill before suggesting a new milestone or version theme.

The goal is to prevent low-value roadmap churn: do not turn inertia, polish, or a thin wrapper around existing functionality into a release milestone.

## Core Rule

Do not name a milestone until the proposal survives the grill.

If the evidence is weak, say that no milestone is justified yet. Recommend an audit spike, maintenance patch, or no action instead.

## Grill Sequence

### 1. Current-State Check

Verify what already exists before proposing anything.

Ask and answer:

- What command, API, doc, checker, or release gate already covers this?
- Was this capability recently delivered in a prior milestone?
- Is the proposed work just a new label, flag, output mode, or wrapper around existing behavior?

Use repo inspection when local context can answer the question. Do not rely on memory for current product capability.

### 2. Pain Check

Define the concrete user pain.

The pain must be a real blocked workflow, repeated mistake, failed gate, incorrect output, unsupported-but-valuable input, or documented user confusion.

Reject vague drivers:

- "continue polishing"
- "make it nicer"
- "align surfaces" without a named drift
- "add a flag for convenience" without a blocked workflow

### 3. Duplicate Check

Argue against the proposal.

Ask:

- Is this already solved by an existing command or documented workflow?
- Would this add a public surface without removing real complexity?
- Would this make docs/tests/release work heavier without changing user outcomes?
- Is the proposal only a narrower formatting of an existing output?

If yes, do not recommend a milestone.

### 4. Value Check

State the user-visible outcome in one verifiable sentence.

Good:

- "Users can see the effective ON/OFF status of one configured rule before running an audit."
- "Release gates fail when current public CI examples drift from supported output formats."

Weak:

- "Improve UX."
- "Polish rule explanation."
- "Add another config snippet output."

If the one-sentence outcome is weak, downgrade the idea to audit or no action.

### 5. Boundary Check

Identify the product boundary before recommending work:

- Is this CLI-only, SDK, HTTP, MCP, docs, release tooling, parser, rules, or audit behavior?
- Does the work create or change a public contract?
- Does it require a decision record?
- Does it imply parser/rule/default-policy/audit/API semantics work?
- Does it need release, or can it be a patch/no-release maintenance change?

Do not assume every CLI affordance belongs in SDK/HTTP/MCP. Different surfaces can have different jobs.

### 6. Evidence Grade

Classify the idea:

- **Strong milestone:** clear pain, no existing solution, measurable outcome, scoped implementation, release-worthy.
- **Audit first:** plausible issue, insufficient evidence, needs inventory before implementation.
- **Patch:** small fix, docs drift, CI/tooling cleanup, no version-worthy product theme.
- **Do not do:** duplicate, weak value, high surface cost, or depends on unplanned prerequisites.

## Required Output

When asked "what next", output:

1. Current-state findings
2. Rejected or downgraded ideas, with reasons
3. At most two viable options
4. Evidence grade for each option
5. Recommended next action

If the recommendation is "audit first", define the audit deliverable and stop there. Do not invent implementation tasks.

## Proposal Template

Use this compact form:

```text
Current state:
- ...

Grill result:
- Rejected: ... because ...
- Downgraded to patch/audit: ... because ...

Viable option:
- Name:
- Evidence grade:
- User outcome:
- Why existing capabilities do not cover it:
- Boundary:
- Decision record needed:
- First task:
```

## Hard Stops

Stop and say the milestone is not justified when:

- the feature already exists
- the value is only "more convenient"
- the proposal adds a public output/flag without a blocked workflow
- the work is mostly docs or maintenance and does not need a release theme
- the real blocker is parser support, rule evidence, or external platform behavior not addressed by the proposal
- the proposal depends on "maybe users want this" rather than repo evidence or user confirmation
