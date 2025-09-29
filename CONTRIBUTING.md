# Participation & Collaboration Guidelines

Digital Public Group (DPGWG)

Version 0.1 – 11/07/2025

## Purpose & Mission

**Mission.** The DPG WG fosters an open, inclusive community that architects, builds, and validates the CBWeb3 technology stack and reference implementations as an _opensource Digital Public Good_ for the Latin American and Caribbean region, leveraging collective expertise from CEMLA, FLAR, LACChain and global technology contributors.

## Scope & Key Deliverables

- Produce requirement backlog, design docs, and prototype code.
- Supply technical artefacts for the **Final CBWeb3 Blueprint** (due **December 2026**).
- Provide release notes and API documentation for downstream adoption.

## Membership & Roles

| **Role** | **Eligibility** | **On‑boarding** | **Responsibilities** |
| --- | --- | --- | --- |
| **Maintainer Core** | LACChain technical lead + LFDT approvers | Listed in MAINTAINERS.md | Merge PRs, enforce workflow, coordinate with CBWG |
| **Cochair** | CEMLA / FLAR nominees | Endorsed by Maintainers | Facilitate meetings, curate backlog |
| **Contributor** | Any individual / organization interested in LAC | Add GitHub handle to /participants/ via PR | File issues, submit PRs, review code |

All members are granted _contributor_ access to the LFDT GitHub repository after providing their GitHub username.

## Governance & Decision Making

- **Consensus first.** Technical decisions strive for consensus in PR or Issue discussion.
- **Fallback vote.** If consensus is not achieved within 5 business days, a vote is triggered:
  - **Quorum:** 50 % + 1 of contributors who have at least one merged PR in the last 15 days.
  - **Voting unit:** One vote per GitHub handle meeting quorum criteria.
  - **Decision rule:** Simple majority.
  - Voting is captured via 👍 / 👎 reactions on the Issue or via a GitHub Discussion poll, tallied by Vote‑Bot.

## Meetings & Communication

- **Weekly Technical Sync.** Virtual, every Thursday, 1 hour, you are welcome to join by attending our weekly calls on Thursdays at 8AM Pacific: <https://zoom-lfx.platform.linuxfoundation.org/meetings/lf-decentralized-trust-labs?view=week>
- **Core Planning Call (CEMLA/FLAR/LACChain).** Mondays.
- **Async channels.** GitHub Issues & Discussions serves as the established forum. Mailing list at <cbweb3-dpg@lists.lfdt.io> for announcements.

## Collaboration Tools & Workflow

| **Need** | **Tool** | **Notes** |
| --- | --- | --- |
| Code, docs, CI/CD | GitHub LFDT/CBWeb3 | Branch based development, DCO signoff |
| Backlog | GitHub Projects | Columns: Inbox, In Progress, Review, Done |
| Automated votes | Vote‑Bot Action | Configured for 7‑day window |

## Timeline & Milestones (high-level)

| **Component** | **Target window** | **WG checkpoints** |
| --- | --- | --- |
| **C1 – Ecosystem Establishment** | 2023 Q4 – 2024 Q2 | Stakeholder alignment, roadmap approval |
| **C2 – Regional TestNet** | 2024 Q3 – 2025 Q2 | TestNet deployment, feature sprints |
| **C3 – Application Market Adoption** | 2025 Q2 – 2026 Q3 | Prototype showcase, scalability plan |
| **C4 – Impact & Evaluation** | 2026 Q2 – 2026 Q4 | Final metrics & evaluation |
| **CBWeb3 Blueprint** | **December 2026** | Contribution of technical annexes |

## Code of Conduct

This WG adopts the Linux Foundation Code of Conduct (<https://lf-decentralized-trust.github.io/governance/governing-documents/code-of-conduct.html>). All members must adhere to the stated principles of professionalism and respect. Violations are handled by the Chair and may lead to removal from the WG.

## Review & Approval

Drafts and other artefacts are reviewed by LFDT Maintainers and **LACChain** inside GitHub Pull Requests. Standard review window: **14 days**. Lack of comment is considered approval.
