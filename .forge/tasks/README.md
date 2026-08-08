# Chatster Task Board

GitHub is the task source of truth; the Markdown files in this directory are the Forge mirror used to preserve acceptance criteria and dependency metadata.

| ID | Issue | Title | Status | Priority | Depends on |
|----|-------|-------|--------|----------|------------|
| 0001 | [#19](https://github.com/AlisinaDevelo/Chatster/issues/19) | Close the public-proof and scale-ready backlog | done | epic | - |
| 0002 | [#20](https://github.com/AlisinaDevelo/Chatster/issues/20) | Deploy and verify the public Render demo | ready | now | - |
| 0003 | [#21](https://github.com/AlisinaDevelo/Chatster/issues/21) | Add Playwright browser-matrix smoke coverage | done | now | - |
| 0004 | [#22](https://github.com/AlisinaDevelo/Chatster/issues/22) | Gracefully drain WebSocket clients on shutdown | done | next | - |
| 0005 | [#23](https://github.com/AlisinaDevelo/Chatster/issues/23) | Add opt-in OpenTelemetry tracing | done | later | - |
| 0006 | [#24](https://github.com/AlisinaDevelo/Chatster/issues/24) | Define the storage repository boundary for Postgres mode | done | next | - |
| 0007 | [#25](https://github.com/AlisinaDevelo/Chatster/issues/25) | Add an optional Postgres repository mode | done | later | 0006 |
| 0008 | [#26](https://github.com/AlisinaDevelo/Chatster/issues/26) | Define the cross-instance room event contract | done | next | - |
| 0009 | [#27](https://github.com/AlisinaDevelo/Chatster/issues/27) | Add an optional Redis room fan-out adapter | backlog | later | 0008 |
| 0010 | [#28](https://github.com/AlisinaDevelo/Chatster/issues/28) | Bind authenticated identity to sessions and room authorization | backlog | later | - |
| 0011 | [#29](https://github.com/AlisinaDevelo/Chatster/issues/29) | Virtualize long room history without losing accessibility | done | later | - |

Run the managed ledger from the repository root:

```bash
python3 /Users/alisinakarimi/.codex/plugins/cache/forge/forge/3.6.0/skills/task-ledger/scripts/forge-tasks.py \
  --repo AlisinaDevelo/Chatster --json status
```
