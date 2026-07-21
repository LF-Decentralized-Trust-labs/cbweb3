
# CBWeb3 — New Participant Handbook

> **Version:** 1.1 — June 2026
> **Maintained by:** Facundo Rodriguez, Project Manager — frodriguez@lnet.global
> **Project codes:** RG-T4567 / ATN/KS-21330-RG

---

## 1. About CBWeb3

CBWeb3 is a regional blockchain testnet for Latin America and the Caribbean, developed under a Technical Cooperation Agreement between **LNET** (executing agency) and **BID Lab** (funder). The platform enables tokenized central bank money (tCeBM) issuance and cross-border Payment-versus-Payment (PvP) settlement between central banks and commercial banks.

The network uses a **Hub & Spoke** architecture: each country operates its own private blockchain node (Spoke), and a shared transnational settlement layer (Hub) connects them for cross-border flows.

The **DPG Working Group (DPGWG)** coordinates open-source governance, with **CEMLA** and **FLAR** serving as co-chairs of both the technical and monetary advisory tracks.

> **Key principle:** Participation in the CBWeb3 testnet does not require a commitment to launch a CBDC or tokenized money program. Each institution retains full sovereignty over its policy decisions, technology choices, and future deployment strategy. The testnet is a shared regional experimentation environment — it reduces uncertainty without prescribing any particular architecture or issuance model.

---

## 2. Who This Handbook Is For

This handbook applies to three distinct participant profiles:

| Profile | Examples | Entry point |
|---------|----------|-------------|
| **Central Banks (CBWG)** | BCRA, BCB, Banrep, Banxico, BCCR, etc. | Invited by CEMLA |
| **Commercial Banks & Private Sector (DPGWG)** | Banco do Brasil, Nuam Exchange, Alliance Enterprise, Banco Inter | LNET legal clearance + agreement |
| **Ecosystem / DPG Contributors** | Universities, fintechs, partner orgs, Korean ecosystem | Direct outreach via GitHub or Discord |

> Regardless of profile, all participants go through a **legal/compliance phase** before getting environment access.

### 2.1 Participation Levels

Within each profile, institutions can choose the level of engagement that matches their objectives and operational readiness. These levels are not mutually exclusive — an institution can start as an Observer and progress to a higher level over time.

| Level | Description | Suitable for |
|-------|-------------|--------------|
| **Observer** | Attends working group meetings, accesses documentation, reviews technical specs, and monitors testnet activities — without deploying infrastructure. | Institutions in initial evaluation or internal policy discussions. |
| **Technical Participant** | Deploys a testnet node or connects through a hosted sandbox, enabling direct execution of test transactions and network interaction. | Institutions seeking hands-on operational and technical experience. |
| **Pilot Sponsor** | Coordinates domestic testing activities involving regulated financial institutions within their jurisdiction. May issue test tCeBM, define access policies, and supervise testing scenarios involving local participants. | Central banks preparing for pilot programs or proof-of-concept initiatives. |
| **Cross-Border Participant** | Engages in interoperability testing with other jurisdictions, including PvP settlement, liquidity management, and multi-currency settlement scenarios. | Institutions evaluating regional integration use cases. |

> Starting as an Observer requires no infrastructure commitment. Institutions can upgrade their participation level at any time by contacting **Facundo Rodriguez** (frodriguez@lnet.global).

---

## 3. Points of Contact

Before anything else, know who to call. These are the people who can unblock you at each step.

### Project Management

| Name                   | Role                                                                    | Email                  | Organization |
| ---------------------- | ----------------------------------------------------------------------- | ---------------------- | ------------ |
| **Facundo Rodriguez**  | Project Manager — primary PM contact                                    | frodriguez@lnet.global | LNET        |
| **Carolina Velásquez** | Blockchain Solutions Architect — technical contact and VPN access owner | cvelasquez@lnet.global | LNET        |
| **Luis Bocchi**        | CTO                                                                     | lbocchi@lnet.global    | LNET        |

### Governance Co-chairs (Institutional Partners)

| Name | Role | Email | Organization |
|------|------|-------|-------------|
| **Julio César Rodríguez Burgos** | CEMLA focal point — Lead Researcher | jrodriguez@cemla.org | CEMLA |
| **Gerardo Hernández Del Valle** | Director of FMI at CEMLA | ghernandez@cemla.org | CEMLA |
| **Deily Lozada Moreno** | FLAR focal point | dlozada@flar.net | FLAR |
| **Carlos Álvarez Guevara** | FLAR co-lead | calvarez@flar.net | FLAR |

### Funder

| Name | Role | Email | Organization |
|------|------|-------|-------------|
| **Chitman Henrique** | IDB/CMF focal point | henriquec@iadb.org | IDB |
| **Diego Mauricio Herrera Falla** | Principal Sector Specialist | diegohe@iadb.org | IDB |
| **Javier Madariaga** | BID Lab key contact | jmadariaga@iadb.org | BID Lab |

### Platform / Technology

| Name | Role | Email | Organization |
|------|------|-------|-------------|
| **Keiji Sakai** | Main technical contact | (via internal channels) | ÁguilaHub / GoLedger |
| **Marcelo Hirata** | Managing Partner | marcelo.hirata@aguilahub.com | ÁguilaHub |

> **Critical rule for Central Banks:** CEMLA manages all official communications to central banks. Do not contact central banks directly without routing through CEMLA.

---

## 4. Legal & Compliance Phase

This phase must be completed **before** any environment access is granted. The process differs slightly by participant profile.

### 4.1 Central Banks

Central banks are invited through an official communication sent by **CEMLA** (via dimf@cemla.org) on behalf of the organizing team. The invitation includes:

1. A formal letter of invitation to participate in the CBWeb3 testing environment.
2. Technical documentation (VPN requirements, WireGuard setup guide).
3. Reference to the project forum at https://forocbweb3.cemla.org for all official materials.

Central banks do not individually execute a commercial agreement with LNET. Their participation is governed by the overarching framework agreement between CEMLA/FLAR and the project.

> If your institution has not received the official CEMLA invitation, contact **Julio César Rodríguez Burgos** (jrodriguez@cemla.org) to be added to the distribution list.

### 4.2 Commercial Banks & Private Sector

Commercial bank and private sector participants follow a structured agreement process:

**Step 1 — Expression of Interest:** The institution confirms interest in participating. This can happen via a DPGWG call, direct outreach, or referral from an existing partner.

**Step 2 — LNET Legal Clearance:** LNET reviews the institution's profile and confirms eligibility. The legal team reviews the participation agreement template.

> Contact: **Facundo Rodriguez** (frodriguez@lnet.global) to initiate. Legal clearance is handled internally by LNET.

**Step 3 — Agreement Execution:** A Participation Agreement (or MOU) is prepared and signed via **DocuSign**. The agreement covers the scope of participation, data handling, and confidentiality.

> Once the agreement is ready, DocuSign is sent by the LNET team. Signature from both parties is required before proceeding.

**Step 4 — Onboarding Session Scheduling:** After the agreement is executed, an onboarding session is scheduled — typically a structured call covering platform overview, access credentials, and testing workflow. Coordinate with **Carolina Velásquez** (cvelasquez@lnet.global) to schedule this session.

### 4.3 MoUs for Institutional Partners (CEMLA, FLAR, etc.)

Institutional MoUs follow a multi-step legal process:

1. Terms proposed by LNET legal team.
2. Counterparty legal review and clearance.
3. LNET legal sign-off.
4. Execution.

> Status as of May 2026: MoUs with CEMLA and FLAR are in negotiation. Contact **Facundo Rodriguez** for current status.

### 4.4 Frequently Asked Questions — Legal & Compliance

---

**We are a central bank and have not received the official CEMLA invitation. What do we do?**

Contact **Julio César Rodríguez Burgos** (jrodriguez@cemla.org) directly and request to be added to the CBWeb3 distribution list. Do not contact LNET for this — CEMLA manages all official CB communications and is the correct entry point.

---

**Can we start VPN or technical onboarding before the legal agreement is signed?**

No. Legal/compliance clearance is a hard prerequisite for environment access. VPN credentials and portal credentials are only issued after the relevant agreement is in place. Starting the WireGuard installation on your side is fine, but LNET will not issue the `.conf` file until compliance is complete.

---

**How long does the DocuSign process typically take?**

Once LNET sends the agreement, turnaround depends on your institution's internal review process. LNET's side typically signs within 1–2 business days of receiving a countersigned document. If your legal team needs more than two weeks, flag it early to **Facundo Rodriguez** (frodriguez@lnet.global) so the project timeline can be adjusted.

---

**Our legal team wants to modify the terms of the Participation Agreement or MOU. Is that possible?**

Minor clarifications can be accommodated. Substantive changes require LNET legal review and may extend the timeline. Contact **Facundo Rodriguez** to open that conversation — do not mark up the document and return it via DocuSign without prior coordination, as that will restart the process.

---

**We are a commercial bank that also wants to contribute technically (code, smart contracts). Which profile applies?**

Start as a Commercial Bank participant (Section 4.2). Once the Participation Agreement is signed, notify **Facundo Rodriguez** of your intent to contribute technically. You can then also be onboarded as a DPG Contributor (Section 4.3 pathway) — the two profiles are not mutually exclusive, but the legal phase must be completed under the commercial bank profile first.

---

**What if our institution is not a central bank or commercial bank — for example, a university, fintech, or technology partner?**

You fall under the Ecosystem / DPG Contributor profile. This track does not require a formal agreement for open-source contributions via GitHub. For access to the testnet environment, contact **Facundo Rodriguez** (frodriguez@lnet.global) — eligibility is reviewed case by case.

---

## 5. Technical Onboarding — VPN Access

Once legal/compliance is complete, institutions choose a connection model before proceeding with VPN setup.

### 5.0 Connection Options: Hosted Sandbox vs. Self-Hosted Node

Participating institutions may connect to the CBWeb3 testnet in one of two ways:

**Option A — Hosted Sandbox**

The institution accesses a pre-configured test environment managed by the CBWeb3 infrastructure operator (LNET / ÁguilaHub). There is no infrastructure to deploy — access is provided immediately after VPN and credential provisioning. This option is recommended for institutions doing initial evaluation or those without dedicated infrastructure teams available for the testing phase.

**Option B — Self-Hosted Node**

The institution deploys and operates its own Hyperledger Besu node connected to the CBWeb3 testnet. This requires coordination with the LNET technical team and additional setup time, but provides a more realistic operational experience and allows the institution to evaluate its own infrastructure, security posture, and governance requirements in a live environment. After node deployment, connectivity tests are performed using standardized APIs and test vectors.

> If your institution is interested in the self-hosted node path, contact **Facundo Rodriguez** (frodriguez@lnet.global) early — this option involves additional provisioning steps and should not be started in parallel with the legal phase without prior coordination.

For the testing phase (September–October 2026), most participants will use the **Hosted Sandbox**. The first technical step for both options is **VPN connectivity**.

### 5.1 What You Need

The CBWeb3 environment is accessed exclusively via **WireGuard VPN**. You will need:

- A server or workstation with internet connectivity
- WireGuard installed (instructions provided in the VPN guide)
- A customized `.conf` file generated by LNET for your institution

### 5.2 How to Get Access

**Submit your access request to:**
**Carolina Velásquez — cvelasquez@lnet.global**

Include the following in your request:
- Institution name
- Contact name and role
- Public WireGuard key (generated by your team following the setup guide)

The LNET team will generate a customized `.conf` file for your institution and return it to you.

### 5.3 VPN Setup Steps

1. **Install WireGuard** on your institution's server or workstation. Follow the guide distributed with the onboarding invitation (available at https://forocbweb3.cemla.org).
2. **Generate your key pair** (public + private key) following the instructions in the guide.
3. **Send your public key** to cvelasquez@lnet.global with your access request.
4. **Receive your `.conf` file** from the LNET team.
5. **Import the `.conf` file** into WireGuard and activate the tunnel.
6. **Test connectivity** — if you cannot reach the environment, open a support request (see Section 8).

> **Deadline:** VPN configuration should be completed before the end of May 2026 to remain aligned with the test execution roadmap.

> **Security note:** The `.conf` file is customized per institution and contains your private network credentials. Do not share it with third parties.

### 5.4 Frequently Asked Questions — VPN

The following questions were raised by central bank participants during onboarding calls and are documented here to avoid repeating the same clarifications.

---

**Is the VPN shared with other central banks?**

Yes. The architecture is point-to-multipoint: all participating institutions connect to the same VPN infrastructure. However, firewall rules isolate each institution at the policy level — at the network layer you are on a shared network, but no institution can reach another institution's resources. There is no risk of lateral movement between peers.

---

**Can we get a dedicated, point-to-point VPN instead?**

Yes, this is feasible if your institution's security policies require it. If the firewall-based isolation described above is not sufficient, LNET can provision a separate VPN instance exclusively for your institution, creating a true point-to-point tunnel between your infrastructure and the CBWeb3 environment. Contact **Carolina Velásquez** (cvelasquez@lnet.global) to request this assessment. LNET will also provide documentation describing how peers are configured so your security team can evaluate the shared setup before deciding.

---

**Does the VPN capture all my machine's internet traffic (full tunnel) or only traffic to CBWeb3 (split tunnel)?**

Split tunnel. Only traffic destined for the CBWeb3 services is routed through the VPN. Your machine's general internet traffic is not affected.

---

**What firewall rules does our institution need to open?**

Allow **outbound UDP port 51820** to the VPN endpoint hostname (provided in your `.conf` file). No inbound rules are required. The exact hostname will be confirmed by the LNET team in the configuration documentation sent to your institution.

---

**What network segments will be accessible once connected?**

Only the two segments defined in your `.conf` file: the segment hosting CBWeb3 services and the client connectivity segment. You will not have broader network access. The specific ranges are included in the configuration file delivered by LNET.

---

**What information do we need to send to request VPN access?**

Send the following to cvelasquez@lnet.global:

- Full name and role/title of the person requesting access
- Institutional email address
- Purpose / use case description
- WireGuard **public key** (generated by your team — see Section 5.3)
- Operating system of the machine where WireGuard will be installed

LNET will return a customized `.conf` file. No IP address sharing is required — only the public key.

---

**How long are WireGuard keys valid? Is there a rotation policy?**

Keys do not expire automatically. They can be invalidated at any time by destroying the private key on your side. For the testing phase (~3 months), no formal rotation schedule is currently defined. If your institution's security policy requires key rotation (e.g., every 3 or 6 months), contact Carolina Velásquez to arrange it.

---

**Do we need to integrate a custom application, or is a web portal enough for testing?**

A web portal is sufficient for testing. After VPN connectivity is established, LNET will issue username and password credentials. All test scenarios (Scenario A and Scenario B) can be executed through the browser-based portals — no command-line access or API integration is required. API/HTTP integration is available as an optional capability for institutions that want to automate workflows.

---

**Are there alternatives to WireGuard?**

WireGuard is the primary and recommended client. OpenVPN is a comparable alternative at a similar certification level, and LNET can provide documentation on it if required for your procurement or security review. A reverse-tunnel arrangement (where your institution hosts the VPN server and LNET connects inbound) is also technically feasible but requires more customization and coordination. Raise this need early with Carolina Velásquez if it applies.

---

### 5.5 Reference Material

- Platform demo video: https://www.youtube.com/watch?v=2TcuEIy4dlM
- Project forum (all official documentation): https://forocbweb3.cemla.org

---

## 6. Platform Access — After VPN Is Active

Once VPN connectivity is confirmed, participants get access to the CBWeb3 platform portals. The platform consists of five web portals, each mapped to a participant role.

### 6.1 Portal Overview

| Portal | Who uses it | Key actions |
|--------|------------|-------------|
| **Treasury Portal** | Central bank treasury teams | Issue and redeem tCeBM, authorize payments |
| **Bank Integration Portal** | Commercial bank teams | Bridge tCeBM to Hub, initiate FX swaps |
| **Supervisor Portal** | Central bank oversight teams | Monitor cross-border flows, audit settlement records |
| **Governance Portal** | Governance participants | Administrative actions, parameter updates, access control |
| **NOC Portal** | Network operations (LNET/LFDT) | Network monitoring and infrastructure status |

### 6.2 Authentication

Access to all portals uses **username and password** credentials provisioned by the LNET team. Credentials are issued per institution during or after the onboarding session. Authentication is backed by **OAuth 2.0 / JWT** (Keycloak).

> Credential requests and resets: contact **Carolina Velásquez** (cvelasquez@lnet.global).

### 6.3 GoLedger / ÁguilaHub — Technical Collaboration

Development partners who need access to the technical layer (APIs, smart contracts, backend services) also need:

- An invitation to GoLedger's **Notion portal** (tech management and documentation). Request via **Keiji Sakai** or **Marcelo Hirata** (marcelo.hirata@aguilahub.com).
- Access to the **External CBWeb3 SharePoint** (CBWeb3TeamExternal) — shared with GoLedger/ÁguilaHub teams. Request via **Facundo Rodriguez**.

---

## 7. Joining the Working Groups & Meetings

CBWeb3 runs two parallel governance tracks. New participants should join the group that matches their profile.

### 7.1 Central Bank Working Group (CBWG)

- **Cadence:** Monthly — Wednesdays, 12:00–13:00
- **Audience:** Central banks, CEMLA, FLAR, IDB, LNET
- **Purpose:** Policy discussions, platform updates, testing coordination
- **How to join:** All invitations are managed by CEMLA. Contact **Julio César Rodríguez Burgos** (jrodriguez@cemla.org) to be added.

> Do not request invitations directly from LNET — CEMLA manages the CB distribution list.

### 7.2 DPG Working Group (DPGWG)

- **Cadence:** Bi-weekly — Fridays, 14:00–14:45
- **Audience:** Commercial banks, private sector partners, fintech, academic partners
- **Purpose:** Technical architecture, network deployment, smart contracts, knowledge products
- **How to join:** Contact **Facundo Rodriguez** (frodriguez@lnet.global) or **Yuri Carrillo** (yuri.carrillo@alliensoft.com) to be added to the distribution list.

### 7.3 CBWeb3 Lab Community Calls

- **Cadence:** Bi-weekly (led by Carolina Velásquez)
- **Audience:** Entire community — open participation
- **How to join:** Join the **Discord #cbweb3 channel** and watch for meeting announcements. Recordings are posted to the LFDT YouTube channel after each call.
- **Discord:** https://discord.lfdecentralizedtrust.org

### 7.4 Co-leaders Coordination Call

- **Cadence:** Weekly — Mondays, 12:30–13:00
- **Audience:** CEMLA, FLAR, IDB/CMF, LNET (closed — institutional leads only)

### 7.5 Internal LNET Weekly Standup

- **Cadence:** Weekly — Thursdays, 12:00–12:30
- **Audience:** LNET team only
- **Purpose:** Project execution review, ClickUp board review

---

## 8. Collaboration Tools & Access

| Tool | Purpose | How to get access |
|------|---------|------------------|
| **Discord #cbweb3** | Community chat, meeting announcements, recordings | Open — join discord.lfdecentralizedtrust.org |
| **ForoCBWeb3 (CEMLA)** | Official presentations, VPN guide, documentation | Open — https://forocbweb3.cemla.org |
| **GitHub — LFDT/CBWeb3** | Code, docs, CI/CD, issue tracking | Open — DCO signoff required for contributions |
| **External CBWeb3 SharePoint** | Shared docs with GoLedger/ÁguilaHub | Request access from Facundo Rodriguez |
| **Internal LNET SharePoint** | Internal planning docs, Results Matrix, PSR | LNET team only |
| **GoLedger Notion portal** | Tech management (backend, API, smart contracts) | Invitation needed — contact Marcelo Hirata |
| **ClickUp** | Internal task management (LNET only) | LNET team only |
| **IDB Lab Project Manager** | Funder reporting platform | LNET and BID Lab access only |

> **GitHub contributors:** All pull requests require a DCO (Developer Certificate of Origin) sign-off. Add `-s` to your commits (`git commit -s`). Merges are controlled by the Maintainer Core (LNET technical lead + LFDT approvers).

---

## 9. Testing Phase — What to Expect

The CBWeb3 testing phase is scheduled for **September–October 2026**, with platform deployment completing around June/July 2026.

### 9.1 What You Will Be Testing

Participants will execute structured test scenarios through the platform portals (no command-line interaction required for central bank and commercial bank participants). The two main scenarios are:

**Scenario A — Enhanced Correspondent Banking (HTLC/PvP):** A bilateral cross-border FX swap executed atomically using Hash Time-Lock Contracts. Tests include FX agreement, fund locking, atomic settlement, and timeout/refund flows.

**Scenario B — Automated Market Maker (AMM):** A hub-based liquidity pool for multi-currency exchange. Tests include bridging tCeBM to the hub, exchange rate consultation, and atomic swap execution.

### 9.2 Pre-Testing Checklist

Before test execution begins, confirm the following:

- [ ] VPN connection active and stable.
- [ ] Portal credentials received and login confirmed.
- [ ] Designated test team members identified within your institution.
- [ ] Point of contact shared with LNET (name, role, email).

A full structured checklist will be distributed before the test execution phase begins. The checklist was presented at the 9th CBWG session (May 2026).

### 9.3 Frequently Asked Questions — Testing Phase

---

**Who from our institution should participate in test sessions?**

At minimum, one person who will operate the platform portal (the person executing transactions). It's useful — but not required — to also have a technical contact available in case of connectivity issues. You do not need developers or blockchain specialists to execute the test scenarios; the portals are browser-based and designed for business users.

---

**Will we receive a test script, or is it free-form exploration?**

Structured test scripts will be distributed before the test execution phase begins. Each scenario (A and B) has defined steps, expected outcomes, and data inputs. You will not be expected to freestyle through the platform.

---

**What happens if we cannot complete a scenario during the session?**

Incomplete scenarios are not a failure. The purpose of this phase is to surface friction, bugs, and usability issues — not to pass/fail participants. Document what you were able to complete and where you got stuck, and report it via the feedback form or directly to **Carolina Velásquez** (cvelasquez@lnet.global). Sessions can be rescheduled if needed.

---

**How do we report a bug versus a user experience issue?**

Both go through the same channel: the feedback form distributed after each session. Use the category field to distinguish between a technical bug (something didn't work as expected) and a UX issue (something worked but was confusing or difficult to use). For critical blockers discovered mid-session, contact **Carolina Velásquez** directly.

---

**Are test sessions recorded?**

This will be confirmed before test execution begins. If recording is enabled, participants will be notified in advance. All feedback and session data is handled under the confidentiality terms of your Participation Agreement.

---

**What if our VPN drops during a test session?**

Reconnect using your WireGuard client. Your session state on the platform should persist. If reconnecting does not restore access within a few minutes, contact **Carolina Velásquez** (cvelasquez@lnet.global) for immediate support. Having her contact information open during test sessions is recommended.

---

**Can multiple people from our institution participate simultaneously?**

Yes. Multiple team members can connect to the VPN and access the portals concurrently, provided each person has received their own credentials. Do not share credentials between individuals. If additional credential sets are needed, request them from Carolina Velásquez before the test session.

---

**Does our institution need to operate a blockchain node to participate in testing?**

No. The CBWeb3 infrastructure (nodes, smart contracts, settlement layer) is managed by LNET and GoLedger/ÁguilaHub. Participants interact exclusively through the web portals. Deploying your own node is optional and involves additional infrastructure costs — contact **Facundo Rodriguez** if your institution is interested in exploring this for Phase 2.

---

### 9.4 Post-Testing Assessment

At the conclusion of testing activities, participating institutions are encouraged to conduct a structured internal assessment. The results of this assessment can inform future pilot programs, regulatory initiatives, or deeper participation in regional interoperability efforts.

The assessment should cover the following dimensions:

| Dimension | Key questions |
|-----------|--------------|
| **Legal readiness** | Are existing frameworks sufficient for tCeBM operations? What regulatory changes would be required for production? |
| **Operational readiness** | Can the institution operate this type of system with current staff and processes? What gaps exist? |
| **Technical performance** | Did the platform meet latency, throughput, and reliability requirements? Were integration points manageable? |
| **Security considerations** | Are network isolation, key management, and access control adequate for institutional requirements? |
| **Governance requirements** | What decisions would the institution need to make about rule-setting, dispute resolution, and policy participation? |
| **Interoperability outcomes** | Which cross-border settlement scenarios worked well? What friction points emerged? |

> LNET will distribute a structured post-testing questionnaire to support this assessment. Results feed directly into the **CBWeb3 Blueprint** (December 2026) and any Phase 2 planning.

### 9.5 Feedback

Participant feedback — especially on VPN complexity and user experience — is actively collected by the LNET team. You will receive a feedback form after each testing session. This input directly shapes the CBWeb3 Blueprint published in December 2026.

---

## 10. Support Channels

| Issue type | Primary contact | Channel |
|-----------|----------------|---------|
| VPN connectivity, portal access, credentials | Carolina Velásquez | cvelasquez@lnet.global |
| Legal agreements, participation status | Facundo Rodriguez | frodriguez@lnet.global |
| Meeting invitations — central banks | CEMLA | jrodriguez@cemla.org |
| Meeting invitations — DPGWG | Facundo Rodriguez | frodriguez@lnet.global |
| GitHub / open-source contributions | LNET technical lead | File an issue on GitHub LFDT/CBWeb3 |
| Platform technical issues (dev layer) | ÁguilaHub / GoLedger | marcelo.hirata@aguilahub.com |
| Community questions | Discord #cbweb3 | discord.lfdecentralizedtrust.org |
| Official project documentation | ForoCBWeb3 | https://forocbweb3.cemla.org |

> Response time expectation for VPN and access issues: LNET team targets resolution within 2 business days. Raise in the Discord channel if no response within that window.

---

## 11. Quick Reference — Onboarding Checklist

Use this checklist to track your institution's progress through onboarding.

### For Central Banks

- [ ] Received official CEMLA invitation email
- [ ] Reviewed VPN guide (available at ForoCBWeb3)
- [ ] Sent VPN access request to cvelasquez@lnet.global (include public WireGuard key)
- [ ] Received and imported `.conf` file from LNET
- [ ] VPN connectivity confirmed
- [ ] Portal credentials received
- [ ] Login to Treasury Portal or Supervisor Portal confirmed
- [ ] Added to CBWG meeting distribution list
- [ ] Joined Discord #cbweb3

### For Commercial Banks & Private Sector

- [ ] Expression of interest confirmed with LNET (frodriguez@lnet.global)
- [ ] LNET legal clearance obtained
- [ ] Participation Agreement executed via DocuSign
- [ ] Onboarding session scheduled with Carolina Velásquez
- [ ] Onboarding session completed
- [ ] VPN access request submitted (cvelasquez@lnet.global)
- [ ] VPN connectivity confirmed
- [ ] Portal credentials received (Bank Integration Portal)
- [ ] Login confirmed
- [ ] Added to DPGWG meeting distribution list
- [ ] Joined Discord #cbweb3

### For Ecosystem / DPG Contributors

- [ ] GitHub account created (if not already active)
- [ ] DCO sign-off configured (`git config --global user.signingkey` / `-s` on commits)
- [ ] CBWeb3 GitHub repository forked or cloned: https://github.com/LF-Decentralized-Trust-labs/cbweb3
- [ ] First issue or PR filed
- [ ] Joined Discord #cbweb3
- [ ] Joined bi-weekly CBWeb3 Lab Community Call

---

## 12. Project Timeline at a Glance

### 12.1 Project Milestones

| Milestone | Target date |
|-----------|------------|
| Platform remote deployment | ~June/July 2026 |
| Test execution phase | ~September/October 2026 |
| Knowledge products published | ~November/December 2026 |
| CBWeb3 Blueprint (final) | December 2026 |

The project operates on 7-month contract cycles (May–December 2026), with four formal deliverables to BID Lab that trigger payment milestones.

### 12.2 Typical Onboarding Timeline Per Institution

The following estimates represent typical durations for each phase of the onboarding process. Institutions may progress through these stages incrementally based on internal priorities and readiness levels.

| Activity | Estimated Duration |
|----------|-------------------|
| Working Group engagement (Observer participation) | 2–4 weeks |
| Governance and administrative setup (legal clearance, agreement execution) | 2–6 weeks |
| Technical connection (VPN + portal credentials) | 1–4 weeks |
| Domestic testing (Scenario A and initial platform familiarization) | 1–3 months |
| Cross-border testing (Scenario B, multi-jurisdictional coordination) | 1–6 months |

> Institutions targeting the **September 2026 test execution phase** should initiate the legal/compliance process no later than **July 2026** to allow sufficient time for governance setup and technical connection. Contact **Facundo Rodriguez** (frodriguez@lnet.global) to start the process.

---

*For updates to this handbook, contact Facundo Rodriguez — frodriguez@lnet.global.*

