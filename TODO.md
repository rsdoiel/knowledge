
# Action items

## Requested features

- [ ] `kb project` has no way to update a project's description. `kb project`
  supports `add`, `list`, `show`, `concepts` and `set-status`; `set-status` is
  the only in-place mutation of an existing row, so a description can be set at
  `add` time and never again. Raised 2026-08-26 from a concrete case in
  `~/WorkLab/agents/knowledge.db`: the `dev-process` project's description names
  `DESIGN_DECIDE_PLAN.md`, a file renamed to `DESIGN_REVIEW_PLAN_IMPLEMENT.md`
  that same day. A description is grounding context -- it is what `kb project
  show` prints and what the FTS index returns -- so the stale filename keeps
  being handed to sessions as current, and there is no supported way to correct
  it short of SQL against `knowledge.db`, which is exactly what the CLI exists
  to avoid. Proposed shape: `kb project set-description NAME DESCRIPTION`,
  matching the shape `set-status` already establishes. Having `add` upsert
  instead would be terser but is easy to trigger by accident, and would let a
  typo'd name silently overwrite a real project. The same gap applies to
  `kb observation` (`add`/`list`/`show`/`sources`) and `kb concept`
  (`add`/`list`), both add-only for their descriptive text -- worth deciding
  whether this is one verb or a general "correct a row you already wrote" pass
  before designing it. Whichever way, the write should refresh the FTS row and
  touch the merge columns, so a description edited on two machines reconciles
  under `kb merge` rather than silently picking one.
