### Context

The governance portal was implemented for in-spoke governance only: each central bank manages onboarding and approval of commercial banks within its own jurisdiction. In the last meeting, Carolina clarified the original intent: the governance portal was meant for external entities such as SEMLA/FLAR, not just the central bank. These external entities act as orchestrators and record-keepers, not decision-makers — central banks retain signing authority on-chain. In other words, the portal is expected to assume that a third party (SEMLA/FLAR) performs the onboarding of banks on the platform, while central banks keep the signing authority. The current implementation does not reflect this role, and inter-spoke governance (how central banks govern across spokes) was identified as a gap, likely a separate phase.

### Objective

Review the governance portal scope against the original role/feature definitions, assess the gap between what was promised to the banks and what is currently implemented, and report back to Carolina — establishing the basis to rework the portal toward external-orchestrator, inter-spoke governance.

### Scope

- Review the current governance portal scope against the original role/feature definitions (owners: Samuel, Lucas).
- Assess the gap between what was promised to the banks and what is currently implemented.
- Document how the portal should model the external-orchestrator role (SEMLA/FLAR) as orchestrators/record-keepers, with central banks retaining on-chain signing authority.
- Capture Samuel's proposed technical path for internal discussion: SEMLA gets a wallet known across all spokes plus RPC access to central bank nodes, allowing it to interact with spokes without running infrastructure on each one (an additive layer, no refactoring required).
- Produce a written report/summary for Carolina.

### Out of Scope

- Technical implementation of the reworked portal (separate task, after prioritization and technical planning).
- Inter-spoke governance decision logic (how central banks govern across spokes) — identified as a likely separate phase.

### Functional Requirements

- The portal must support external entities (SEMLA/FLAR) acting as orchestrators and record-keepers of bank onboarding.
- Central banks must retain signing authority on-chain; external orchestrators must not become decision-makers.
- Onboarding of banks must be operable by the external orchestrator across spokes.

### Non-Functional Constraints

- Samuel's proposed approach should remain an additive layer on top of existing work, avoiding refactoring where possible.
- Must respect scenario isolation and the compliance gate; no bypass of existing authorization on the on-chain signing path.

### Assumptions Made

- **Task type** This first task covers scope review and gap assessment (Planning/Study); the actual portal rework and inter-spoke governance are follow-up tasks.
- **Priority** Left unset intentionally; to be defined in the upcoming prioritization step.
- **Ownership** Review action item is assigned to Samuel and Lucas per the meeting notes.

### Open Questions for PM

- Is the SEMLA/FLAR inter-spoke governance approach (shared wallet + RPC access to central bank nodes) approved for the next phase? (To be discussed internally before the next daily.)
- Should inter-spoke governance be scoped as its own separate phase/task?
- What exactly was promised to the banks, to anchor the gap assessment against a concrete baseline?

### Dependencies

Internal discussion of the SEMLA/FLAR inter-spoke governance approach before the next daily; follow-up technical planning task for the portal rework.