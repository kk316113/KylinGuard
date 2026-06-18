# KylinGuard UI Reference and Product Shell Plan

## 1. Why Current UI Direction Is Not Enough

The current UI direction is not enough for KylinGuard because the product is becoming a real operations Agent, not a pile of demo cards.

- We should not let AI invent a backend console layout from scratch.
- We should not keep stacking tabs, cards, and raw JSON areas.
- KylinGuard needs an Agent Console, not a generic admin dashboard.
- The frontend must support the main product line: user natural-language task -> Agent execution -> tool evidence -> safety explanation -> final operations advice.

The primary screen should make the user feel they are working with an operations Agent. Security audit details are important, but they should explain the Agent run rather than become the main product surface.

## 2. Reference Framework Decision

| Candidate | Stack | Pros | Cons | Decision |
|---|---|---|---|---|
| Arco Design Pro Vue | Vue3 + TS + Arco | Same component family as current frontend, low migration cost, proven layout/dashboard/menu patterns | Visual style still needs KylinGuard-specific product design | Primary reference |
| Vue Vben Admin | Vue3 + Vite + TS + Ant Design Vue | Mature admin capabilities, layout, permission, menu, theme organization | Ant Design Vue component migration would be heavy | Secondary reference |
| Art Design Pro | Vue3 + TS + Element Plus | Strong visual polish and dashboard product feel | Element Plus-first template; migration would increase mixed UI debt | Visual reference only |
| OpenClaw | Agent product | Strong product logic reference for scenario-oriented Agent workflow | Not a Vue UI template and should not drive backend UI layout directly | Product logic reference |

Decision: use Arco Design Pro Vue as the primary UI reference because it matches the current stack most closely: Vue 3, TypeScript, and Arco Design Vue.

## 3. Product UI Definition

KylinGuard frontend is:

```text
Agent Console for secure Kylin OS operations.
```

It is not a traditional security audit dashboard. It is also not a generic admin page with unrelated metrics.

Core structure:

- Left: task sessions, scenario entry points, recent runs.
- Center: Agent conversation workspace.
- Right: execution insight panel or drawer.
- Top: runtime status, mode, system health, model status, policy status.

The user's natural language task remains the source of truth. Any scene or scenario metadata is for display, filtering, and report organization only.

## 4. Proposed Layout

### Left Sidebar

- Logo: KylinGuard / 麒盾.
- New Task action.
- Recent Runs list.
- Suggested Ops Prompts.
- Runtime Mode Badge.

The sidebar should feel like a task workspace, not a navigation tree of generic backend pages.

### Center Agent Workspace

- Chat-first interaction.
- Natural-language input.
- `final_answer` first.
- Agent execution timeline under the answer.
- Streaming-ready visual structure, even if current backend remains non-streaming.

The center area is the main product surface. It should explain what the Agent is doing without forcing the user to inspect JSON.

### Right Insight Panel

Use tabs inside the right panel, not top-level page stacking:

- Steps.
- Evidence.
- Audit.
- Tools.
- Report.

The right side is for details on demand. It should not compete with the natural-language conclusion.

### Top Status Bar

- Go Agent status.
- audit-core status.
- runtime mode.
- model name.
- policy status.

The top bar should communicate whether the product is running in real DeepSeek, mock, or deterministic mode.

## 5. What We Should Build First

The first implementation should build a Product Shell, not a full admin system:

- Transform `AgentChatWorkbench` into a three-column Agent Console shell.
- Keep existing Agent run API calls.
- Keep current response mapping and safety semantics.
- Do not split into many top-level pages.
- Do not introduce a traditional backend menu system yet.
- Do not introduce a new UI library.
- Reuse Arco Design Vue components.
- Use Arco Design Pro visual organization as reference.

This step should produce a stable product frame that future Agent features can fit into.

## 6. Component Plan

Recommended layout components:

```text
frontend/src/components/layout/
- KylinGuardShell.vue
- RuntimeStatusBar.vue
- TaskSidebar.vue
- InsightPanel.vue
```

Recommended Agent components:

```text
frontend/src/components/agent/
- AgentConversation.vue
- AgentStepTimeline.vue
- ToolEvidencePanel.vue
- AuditInsightPanel.vue
- ReportSummaryPanel.vue
```

The first pass can keep logic inside `AgentChatWorkbench.vue` while extracting only stable visual sections. Avoid premature component fragmentation.

## 7. Implementation Phases

Stage 17A-1:

- Build the three-column shell.
- Preserve existing `AgentChatWorkbench` request and response logic.
- Do not change backend APIs.

Stage 17A-2:

- Add right-side Insight Panel tabs.
- Tabs: Steps / Evidence / Audit / Tools / Report.

Stage 17A-3:

- Add Runtime Status Bar.
- Connect current health and capabilities APIs where already available.

Stage 17A-4:

- Polish visual hierarchy based on Arco Design Pro references.
- Refine spacing, density, dark-mode readiness, and dashboard-like status treatment.

## 8. Hard Constraints

- Do not introduce a new UI framework unless explicitly approved.
- Do not copy a whole template into the repository.
- Do not turn natural-language prompts into scenario IDs.
- Do not write fixed workflows.
- Do not save real API keys.
- Do not let safety audit become the primary screen.
- Do not change Agent Loop, Tool Policy, or TraceShield semantics for UI layout work.

## 9. Reference Notes

Arco Design Pro Vue is the primary reference because its repository describes a Vue 3, TypeScript, Arco Design based enterprise application template with page templates for dashboards, lists, forms, visualization, themes, and dark theme support.

Vue Vben Admin remains useful for studying mature admin information architecture, but it is Ant Design Vue based and would require heavier component migration.

Art Design Pro remains useful for visual taste and dashboard polish, but it is Element Plus based and would increase the current mixed-library cost.

OpenClaw should influence product thinking: task-first Agent workflow, tool execution transparency, and safety explanation. It should not be copied as a UI template.
