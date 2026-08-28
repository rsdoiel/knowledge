---
id: "0002"
title: "An empty trigger renders as the placeholder, not as spaces"
date: "2026-02-10"
status: accepted
kind: correction
trigger: ""
project: fixture
phase: "0.0.46"
supersedes: []
superseded_by: []
relates_to: ["0001"]
initiative: ""
session: ""
decisions: []
tags: []
uuid: "01a00000-0000-7000-8000-000000000002"
origin_host: "fixture"
---

**Context.** Converted records legitimately carry an empty trigger.

**Decision.** The column holds the placeholder so the title still starts at $7.

**Rationale.** awk splits on runs of whitespace; a padded empty column is not a field.

**Rejected alternatives.** Dropping the column.

**Consequences.** Every column always holds a value.
