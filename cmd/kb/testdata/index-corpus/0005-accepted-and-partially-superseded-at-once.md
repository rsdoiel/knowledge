---
id: "0005"
title: "Accepted and partially superseded at once, which the sup flag exists for"
date: "2026-05-25"
status: accepted
kind: decision
trigger: live-test
project: fixture
phase: ""
supersedes: []
superseded_by: ["0004"]
relates_to: []
initiative: ""
session: ""
decisions: []
tags: []
uuid: "01a00000-0000-7000-8000-000000000005"
origin_host: "fixture"
---

**Context.** Because the unit is an episode, a later record can invalidate one
decision inside a multi-decision episode while the rest stand.

**Decision.** superseded_by does not imply status: superseded.

**Rationale.** clasm DR-0160 is accepted and carries superseded_by.

**Rejected alternatives.** Deriving one field from the other.

**Consequences.** The index flags this with sup rather than restating status.
