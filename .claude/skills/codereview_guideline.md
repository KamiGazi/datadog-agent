You are reviewing a SKILL.md against Anthropic's official skill-creator guidelines.
Read the skill at SKILL_PATH.

## Dimension 1 — Description (triggering) / 25 pts
- Does it cover BOTH what the skill does AND when to trigger?
- Does it list concrete trigger terms a real user would type?
- Is it actively "pushy" — inviting use, not just describing?
- Is the scope narrow enough to avoid false triggers?
- Does it avoid being so vague that the agent undertriggers?

## Dimension 2 — Writing philosophy / 25 pts
- Does it explain WHY instructions matter, rather than just issuing commands?
- Does it avoid heavy-handed ALWAYS/NEVER/MUST in all-caps?
- Are instructions general enough to work across many prompts,
  not just the examples the author had in mind?
- Does it use imperative form ("Run X", not "X can be run")?
- Does it avoid over-narrow, example-specific rules that would
  cause the skill to overfit to particular inputs?

## Dimension 3 — Structure and progressive disclosure / 25 pts
- Is the body under 500 lines?
- If > 300 lines, is there a table of contents or clear section headers?
- Are bundled resources (scripts/, references/, assets/) used for things
  every invocation would otherwise recreate from scratch?
- Are references annotated with WHEN to load them?
- Does the skill lead with the common path, edge cases later?

## Dimension 4 — Output definition and examples / 25 pts
- Is the expected output format explicitly defined
  (with a template or example structure)?
- Are there concrete input/output examples, not just abstract descriptions?
- Are success criteria clear enough that two people would agree
  on whether the skill worked?
- Are dependencies or prerequisites stated?

## Output
Post a PR comment with:
- Scores per dimension
- Top 3 actionable improvements grounded in the guidelines above
- A suggested description rewrite if dimension 1 scored < 18
- Overall recommendation: Request Changes (<60) / Approve with suggestions (60–79) / Approve (≥80)
