# Basic Harness Example

This example proves that AgentKit core runtime can run without ADK, WB, Excel, HTTP, SSE, WebSocket, or external backend code.

Flow:

1. Fake model returns explicit structured `route_decision`.
2. Runtime validates the decision.
3. Runtime executes a registered read-only tool.
4. Runtime appends tool result into model context.
5. Fake model returns final assistant message.
6. Runtime returns completed `RunResult`.

Run:

```bash
go run ./examples/basic-harness
````

Expected shape:

```text
event=started ...
event=step ...
event=step ...
event=completed ...
status=completed steps=2 ledger_entries=3
Found 2 catalog items for boxes: archive box and shipping box.
```
