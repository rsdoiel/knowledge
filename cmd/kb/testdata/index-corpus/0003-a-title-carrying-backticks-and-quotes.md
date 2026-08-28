---
id: "0003"
title: "A title carrying `backticks` and \"double quotes\", which YAML must survive"
date: "2026-03-15"
status: superseded
kind: decision
trigger: implementation
project: fixture
phase: ""
supersedes: []
superseded_by: ["0004"]
relates_to: []
initiative: "migration"
session: ""
decisions: []
tags: []
uuid: "01a00000-0000-7000-8000-000000000003"
origin_host: "fixture"
---

**Context.** The YAML-quoting stress case, and the superseded side of a pair.

**Decision.** Titles round-trip verbatim.

**Rationale.** The struct declaration is the format specification.

**Rejected alternatives.** Escaping at render time.

**Consequences.** None.
