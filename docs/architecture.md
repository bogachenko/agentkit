# AgentKit Architecture

## Layer Responsibilities

### core/llm

Responsibility:
- Defines provider-neutral LLM message, part, request, response, role, and tool-call data structures.

Allowed decisions:
- Represent LLM conversation data.
- Represent tool calls and tool results as neutral data.
- Validate structural correctness of LLM-domain objects.

Forbidden decisions:
- No business intent detection.
- No prompt selection.
- No routing.
- No execution planning.
- No provider-specific assumptions.
- No marketplace, Excel, database, or API semantics.

Source of truth:
- Explicit Go types in core/llm.

Deterministic vs probabilistic:
- Fully deterministic.

---

### core/runtime

Responsibility:
- Controls agent run lifecycle.
- Routes steps.
- Maintains active task state.
- Maintains execution ledger.
- Applies runtime policy.
- Validates step transitions.
- Publishes runtime events.

Allowed decisions:
- Decide next deterministic runtime transition from explicit state.
- Accept or reject a route decision based on contracts.
- Track completed, failed, pending, and blocked steps.
- Enforce approval and policy boundaries.

Forbidden decisions:
- No semantic interpretation of user intent.
- No keyword-based classification.
- No retrieval alias handling.
- No entity extraction except already provided structured values.
- No tool-specific business logic.
- No executable plan invention from natural language.

Source of truth:
- Runtime state.
- Ledger.
- RouteDecision.
- Policy contract.
- Capability registry metadata.

Deterministic vs probabilistic:
- Runtime is deterministic.
- It may consume LLM-produced decisions, but must validate them deterministically.

---

### core/session

Responsibility:
- Defines session lifecycle contracts.
- Supports title, summary, compaction, and session state hooks.
- Keeps session metadata separate from business logic.

Allowed decisions:
- Store and update session metadata.
- Run configured summarization/title hooks.
- Compact history according to explicit compaction policy.

Forbidden decisions:
- No business action planning.
- No tool routing.
- No domain-specific interpretation.
- No hidden prompt-specific behavior.

Source of truth:
- Session event log.
- Session state.
- Explicit hook configuration.

Deterministic vs probabilistic:
- Session state updates are deterministic.
- Summary/title generation may be LLM-based, but only through explicit configured services.

---

### core/capability

Responsibility:
- Defines how external capabilities expose tools, callbacks, context, and metadata to the runtime.

Allowed decisions:
- Register tools.
- Register capability metadata.
- Declare required sources, permissions, and approval rules.
- Declare deterministic contracts for tool execution.

Forbidden decisions:
- No user intent classification.
- No business plan generation.
- No implicit fallback tools.
- No hidden dependency on a specific agent.

Source of truth:
- Capability registry metadata.
- Tool contracts.

Deterministic vs probabilistic:
- Capability registration and tool execution contracts are deterministic.
- A capability may expose LLM-facing tools, but not hide semantic decisions inside deterministic code.

---

### core/skill

Responsibility:
- Defines reusable skill metadata, instructions, references, limitations, and workflow documentation.

Allowed decisions:
- Load skill manifests.
- Validate skill structure.
- Provide skill text/resources to an agent builder.

Forbidden decisions:
- No runtime routing.
- No execution.
- No semantic classification.
- No business intent detection.

Source of truth:
- skill.yaml.
- SKILL.md.
- references/.

Deterministic vs probabilistic:
- Skill loading and validation are deterministic.
- LLM may use skill content during reasoning.

---

### core/port

Responsibility:
- Defines interfaces for external infrastructure.

Allowed decisions:
- Define contracts for model, tool, session store, publisher, clock, ID generator, logger, and tracer.

Forbidden decisions:
- No implementation logic.
- No provider-specific behavior.
- No business behavior.

Source of truth:
- Interface definitions.

Deterministic vs probabilistic:
- Fully deterministic.

---

### adapters/*

Responsibility:
- Converts provider-specific SDK objects into AgentKit core contracts and back.

Allowed decisions:
- Translate ADK/OpenAI/Gemini/session/tool/event types.
- Normalize provider-specific response shapes.
- Enforce provider-specific safety limits.

Forbidden decisions:
- No business logic.
- No runtime policy.
- No prompt-specific routing.
- No hidden fallback behavior.

Source of truth:
- Provider SDK types.
- AgentKit core contracts.

Deterministic vs probabilistic:
- Fully deterministic.

---

### capabilities/*

Responsibility:
- Implements reusable non-core integrations such as client-side tools, Excel, database, browser, files, or API access.

Allowed decisions:
- Execute explicit tool calls.
- Return structured results.
- Declare required permissions and approval boundaries.

Forbidden decisions:
- No semantic user intent detection.
- No plan composition.
- No hidden heuristics.
- No fallback execution paths.

Source of truth:
- Tool input schema.
- Tool output schema.
- Capability registry metadata.
- External system response.

Deterministic vs probabilistic:
- Tool execution is deterministic.
- LLM-facing reasoning must stay outside deterministic capability code.