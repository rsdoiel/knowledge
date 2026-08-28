---
id: "0004"
title: "The superseding side of the pair, carrying an initiative"
date: "2026-04-20"
status: accepted
kind: decision
trigger: plan-review
project: fixture
phase: ""
supersedes: ["0003"]
superseded_by: []
relates_to: []
initiative: "migration"
session: "abc123"
decisions: []
tags: []
uuid: "01a00000-0000-7000-8000-000000000004"
origin_host: "fixture"
---

**Context.** The accepted side of a supersession.

**Decision.** Supersession is stored in one direction only.

**Rationale.** The inverse is synthesised on read.

**Rejected alternatives.** Storing both edges.

**Consequences.** None.
