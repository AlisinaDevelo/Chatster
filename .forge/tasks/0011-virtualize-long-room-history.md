---
id: 0011
title: Virtualize long room history without losing accessibility
status: done
agent: frontend-specialist
model: sonnet
depends_on: []
parent: 0001
---

## Goal
Keep the chat UI responsive when room histories become large while preserving the current log semantics, keyboard scrolling, scroll anchoring, and quiet-update preference.

## Acceptance criteria
- [x] Establish a measured threshold and baseline for message count, render time, memory, and scroll-to-latest behavior before choosing a library.
- [x] Virtualization preserves `role="log"`, accessible message content, keyboard focus, reduced motion, own-message styling, timestamps, and room transitions.
- [x] Add component/browser coverage for initial history, append-at-bottom, reading older content, reconnect merge, and switching rooms.
- [x] Keep the dependency small and document the tradeoff in `docs/FRONTEND.md`; do not optimize before the baseline demonstrates a real need.

## Context
The current message list is intentionally simple O(n) append behavior. This is an optional performance track after browser smoke establishes the real interaction baseline.

## Notes
Coordinate with issue 0003 so browser-level scroll assertions and accessibility checks cover the same behavior.
