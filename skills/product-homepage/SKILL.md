---
name: product-homepage
description: Generate or revise a product homepage from repository facts, with explicit user choice between replication and visual innovation.
---

# Product Homepage

## When to Use
- Build a new product homepage from repo facts
- Revise an existing homepage without losing validated messaging
- Reuse a homepage narrative skeleton while changing the visual direction

## Hard Rules
- Repository facts first
- Ask mode first only when mode is still uncertain
- Ask file strategy before writing only when the target path or overwrite behavior is still uncertain
- Do not overstate product scope
- Use frontend-design for refresh and custom visual paths by default, but if it is unavailable continue with an inline design-summary path under the same locked constraints, visual dimensions, and variation rules

## Workflow
1. Gather product facts from repo
2. Ask homepage mode
3. Ask file strategy
4. Fill missing product inputs only if repo facts are insufficient
5. Produce the Required Design Summary
6. Implement or revise the target static HTML homepage
7. Explain what changed this time

## Product Facts: Source Priority

Always gather homepage facts in this order:
1. Current user instructions
2. Repository-local facts
   - `README.md`
   - relevant `docs/`
   - existing landing page files
   - release notes if needed for trust framing
3. Lightweight git history when context is still missing

Do not use GitHub deep research by default.
Only expand to GitHub issues, PRs, or releases when:
- the user explicitly requests it, or
- repository-local facts are insufficient to support homepage claims

If expansion is needed, ask first.

## Fact Gathering Checklist

Before drafting homepage content:
- identify the product name
- extract one-sentence positioning
- identify target users
- identify core scenarios
- identify hard scope boundaries
- collect trust evidence from docs or releases
- note any sections the user explicitly wants preserved

## Required Branching Questions

Ask these questions in order, but only when the answer is not already clear from the user's request.

### Question 1: Homepage mode
If the user already clearly specified a mode, confirm that mode and continue to file strategy instead of re-asking.
Otherwise, ask the user to choose one:
- Replicate Current Homepage
- Keep Narrative, Refresh Visuals
- Fully Custom Homepage

### Question 2: File strategy
If the target file is already explicit, confirm that target and whether to overwrite it.
Otherwise, ask the user to choose one:
- Create a new file
- Overwrite a specified target file

## Mode Rules

### Replicate Current Homepage
- keep the current information architecture unless the user asks to reorder
- keep visual language close to the reference page
- do not change the core layout pattern, section order, hero composition, card system, or background treatment unless the user explicitly asks
- allow only limited changes to typography tuning, palette details, spacing rhythm, icon treatment, and polish
- `frontend-design` is optional
- if no current homepage or reference file exists, stop treating replicate or refresh as actionable from a continuity baseline; ask the user to provide a reference target or switch to Fully Custom Homepage

### Keep Narrative, Refresh Visuals
- keep the homepage narrative skeleton
- keep product facts and trust evidence grounded in the repo
- change at least three visual dimensions from the fixed list in Visual Dimensions
- use `frontend-design` by default; if it is unavailable, continue with an inline design-summary path under the same locked constraints, visual dimensions, and variation rules

### Fully Custom Homepage
- do not inherit the current page layout by default
- inherit only product facts and explicit user constraints
- use `frontend-design` unless the user explicitly asks for a plain implementation path; if `frontend-design` is unavailable, continue with an inline design-summary path under the same locked constraints, visual dimensions, and variation rules

## Homepage Methodology

Apply these rules unless the user explicitly overrides them:
- lead with outcomes, not internal architecture
- state hard scope boundaries early
- present scenarios before product surfaces
- show enough evidence to trust the product without dumping the full README
- keep examples sparse and representative
- treat bilingual support as first-class when requested
- treat code examples as explanatory, not decorative
- keep motion restrained and compatible with reduced motion
- preserve validated sections the user explicitly values

## Default Homepage Skeleton

Use this sequence unless the user asks for a different structure:
1. Hero
2. Why now / why this matters
3. Core scenarios
4. How it works
5. Key benefits / outcomes
6. Integrations / surfaces
7. Examples
8. Trust / proof
9. Final CTA

## Visual Dimensions

Use this closed list of countable visual dimensions:
- typography system
- color and contrast system
- background treatment
- layout density
- hero composition
- card and panel system
- navigation and CTA treatment
- illustration or icon style

Count only changes to these dimensions.
Micro-tweaks such as minor spacing adjustments, border-radius nudges, shadow tuning, or small animation polish do not count as separate dimensions.

## Variation Intensity

Apply variation intensity across the fixed Visual Dimensions list.
Continuity means keeping a dimension recognizably close to the reference page.
Novelty means changing a dimension to a clearly different treatment.

Support these levels:
- Conservative — keep most dimensions continuous; change no more than two dimensions with novelty
- Balanced — keep some dimensions continuous; change at least three dimensions with novelty
- Bold — change most dimensions with novelty; keep continuity only where needed for usability or explicit constraints

Defaults:
- Replicate Current Homepage → Conservative
- Keep Narrative, Refresh Visuals → Balanced
- Fully Custom Homepage → Balanced unless the user requests Bold

## Style Direction

When the user chooses an innovation or custom path, declare a style direction before implementation.
Map the chosen direction to four concrete page decisions before implementation:
- hero composition
- section rhythm
- card system
- background treatment

Possible directions include:
- technical editorial — restrained typography, strong reading rhythm, article-like sections
- quiet enterprise — muted palette, calm spacing, polished trust-oriented UI
- high-contrast console — dark surfaces, terminal-inspired accents, sharper contrast edges
- structured minimal — sparse composition, disciplined grid, low-decoration surfaces
- dense operator dashboard — compact layout, data-forward cards, control-heavy interface cues

## Anti-Repetition Rules

When the user chooses an innovation or custom path:
- do not repeat the same combination of font pairing, hero structure, card system, and background treatment from the last reference page
- change at least three visual dimensions from the fixed Visual Dimensions list
- include a short "what changed this time" note

## Required Design Summary

Before editing files, provide a concise summary containing:
- chosen homepage mode
- chosen file strategy
- confirmed product boundaries
- narrative skeleton
- style direction
- whether `frontend-design` will be invoked
- which constraints are locked versus allowed to vary downstream
- preserved sections
- what will change relative to any reference page

After providing the summary, wait for user confirmation only when homepage mode, file strategy, or material constraints are still uncertain. If the user already made those explicit, proceed directly.

## Downstream Skill Use

### `frontend-design`
- Replicate Current Homepage: optional
- Keep Narrative, Refresh Visuals: required by default, with inline design-summary fallback when unavailable
- Fully Custom Homepage: required by default, with inline design-summary fallback when unavailable

Use `frontend-design` only after product facts, homepage mode, file strategy, and design summary are established.
If `frontend-design` is unavailable for refresh or custom modes, continue inline by producing the same design-summary decisions directly, keeping the locked constraints unchanged and applying the same visual dimensions and variation-intensity rules.
Pass these inputs into `frontend-design`:
- product facts grounded in repo sources
- chosen homepage mode and file strategy
- required narrative skeleton and preserved sections
- style direction and variation intensity
- explicit constraints such as bilingual needs, accessibility, framework, or target file
- reference page links or file paths when continuity matters

Expect these outputs back:
- a concrete visual direction for the homepage
- the specific visual dimensions that change versus the reference
- which dimensions preserve continuity versus introduce novelty
- how many visual dimensions changed in total
- why the narrative skeleton and preserved sections remain intact
- implementation-ready guidance for hero, section rhythm, card/panel system, CTA treatment, and background treatment
- any required UI tokens, interaction constraints, or content presentation rules

Use that returned output to keep repo-grounded messaging and structure intact while applying the visual system during implementation or revision.

### External Research

Do not invoke GitHub deep research by default.
Only use research mode when repo facts are insufficient for homepage claims or when the user explicitly requests broader research.
Ask before doing so.

Research should answer concrete questions such as:
- what comparable products or category patterns users will recognize
- what terminology or framing is common in the category
- what external proof points are safe to cite or paraphrase
- whether a broader market claim needs corroboration beyond repo materials

Research may add evidence such as:
- public competitor positioning examples
- category terminology patterns
- third-party benchmarks, reviews, or ecosystem references
- official public docs, release notes, or announcements outside the repo

Before research output is used in homepage copy or structure, convert it into homepage-safe claim boundaries and evidence constraints.

Repository facts remain authoritative for product capabilities, scope, supported surfaces, and trust claims unless the user explicitly asks for broader external claims.
If repo facts are insufficient and the user declines external research, restrict homepage copy to verified repo facts only, omit unsupported positioning or market claims, and include a short evidence-gap note that explains which claims could not be substantiated from repository materials.

## Failure Modes to Avoid
- generic AI-sounding product copy detached from the repo
- overstating product scope
- deleting validated blocks by default
- turning the top half of the page into a README summary dump
- generating visually identical results across runs
- forcing experimentation when the user asked for continuity
