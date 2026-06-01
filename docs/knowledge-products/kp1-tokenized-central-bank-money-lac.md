# Knowledge Product 1 — Draft v0.1
## Tokenized Central Bank Money in LAC: From Issuance to Cross-Border Integration
## Dinero Digital de Banco Central en ALC: De la Emisión a la Integración Transfronteriza

**Status:** Draft for internal review
**Version:** v0.1 — 2026-05-19
**Author:** CBWeb3 Working Group / LNET
**GitHub issue:** [#46 — Knowledge Product 1 Proposed Structure](https://github.com/LF-Decentralized-Trust-labs/cbweb3/issues/46)
**Audience:** Central bankers, policymakers, and broad financial sector stakeholders

---

> **Editorial note (v0.1):** This draft incorporates all five feedback points raised by Yuri Carrillo (comment, April 17, 2026) and accepted by Carolina Velasquez (April 21, 2026):
> 1. Section 2 — LAC context disaggregated by country tier, not treated as homogeneous
> 2. Section 4 — Regulatory checklist supplemented by current status across LAC jurisdictions
> 3. Section 5 — Operating models include explicit pros/cons analysis for each
> 4. Section 6 — Closed user group operationalized with eligibility criteria
> 5. Section 11 — Final remarks include a summary action roadmap

---

---

# ENGLISH VERSION

---

## 1. Executive Summary: A Decision Framework for LAC Authorities

Latin America and the Caribbean stand at a pivotal juncture in the evolution of money. Cross-border payments in the region remain among the most expensive and slowest in the world. Correspondent banking relationships are thinning. Stablecoins and foreign digital currencies are gaining ground in dollarized and semi-dollarized economies, eroding the policy space available to central banks. Against this backdrop, tokenized central bank money has emerged not as a speculative technology experiment, but as a concrete policy instrument — one that several regional authorities are already exploring in earnest.

This document is addressed to the decision-makers who must determine whether and how to engage with this transformation. It is organized around the practical questions that senior policymakers face: What exactly is being issued? Is our legal framework ready? What operating model fits our institutional context? Where do we start, and how do we connect to the region?

The decision framework this document proposes has three dimensions. First, the **settlement asset question**: whether to issue tokenized central bank money at the wholesale level, the retail level, or both, and how to position it relative to tokenized commercial bank deposits and private digital instruments. Second, the **operating model question**: whether the central bank leads the infrastructure, delegates to intermediaries, or adopts a hybrid arrangement, and what each choice implies for governance, costs, and systemic risk. Third, the **cross-border integration question**: whether to pursue bilateral corridors, join a shared regional platform, or modernize correspondent banking relationships — and how to sequence these choices over time.

The practical conclusion of this document is that the conditions for action are already in place. Countries do not need to wait for perfect legal certainty, fully tested technology, or regional consensus before beginning. A deliberate, gradual approach — starting with a closed group of regulated participants in a controlled domestic use case — allows institutions to learn, build capacity, and reduce risks before expanding scope. The CBWeb3 regional testnet exists precisely to support this experimentation phase, providing shared infrastructure, interoperability tooling, and a community of practice across central banks and financial institutions in LAC.

---

## 2. Why Tokenized Central Bank Money Matters for LAC Now

The case for tokenized central bank money in Latin America and the Caribbean is not built on technology enthusiasm. It is built on a specific set of structural pressures that are already reshaping the regional financial landscape, and that demand a policy response.

**The correspondent banking problem.** The number of active correspondent banking relationships in LAC has declined steadily over the past decade, driven by de-risking strategies among global banks. For smaller economies and for specific transaction categories — particularly remittances and trade finance — this contraction has raised costs, lengthened settlement times, and in some corridors, effectively eliminated access to affordable international payment infrastructure. Tokenized central bank money, used as a settlement asset in a direct or semi-direct model, can reduce dependence on correspondent intermediaries by enabling payment-versus-payment settlement with cryptographic finality.

**The stablecoin and dollarization pressure.** Across the region, stablecoin adoption is growing — particularly in economies with histories of currency instability or restricted access to US dollar accounts. While this adoption reflects genuine demand for more stable and accessible digital value, it also represents a potential transfer of monetary sovereignty. If a significant share of domestic transactions migrates to instruments denominated in foreign currencies and issued by private entities outside regulatory reach, the ability of central banks to implement monetary policy, observe systemic risk, and enforce AML/CFT rules is materially diminished. Tokenized central bank money offers a sovereign alternative: digital, programmable, interoperable — but issued and governed by the monetary authority.

**The cross-border payment inefficiency.** Sending money across LAC borders is still largely slow, opaque, and expensive. Average remittance costs in the region remain above the G20 target of 3 percent. For businesses, the inability to settle in local currencies forces reliance on dollar intermediation, adding FX costs and settlement risk. Tokenized cross-border settlement — whether through bilateral corridors or shared platforms — can compress settlement cycles from days to seconds and eliminate layers of intermediation that serve no function other than friction.

**A differentiated regional landscape.** LAC is not a monolithic region, and any honest assessment of the opportunity must acknowledge the heterogeneity in country context, institutional capacity, and adoption readiness. Three broad tiers characterize the current landscape:

- *Advanced jurisdictions* — countries such as Brazil, Mexico, and Colombia that have well-developed regulatory frameworks, sophisticated central bank technology capacity, and active CBDC or tokenization research programs. These countries are positioned to be early movers and regional anchors.

- *Developing capacity jurisdictions* — countries such as Peru, Chile, Ecuador, Costa Rica, and the Dominican Republic, where regulatory frameworks are solid but CBDC-specific legislation is still emerging and central bank technology teams are building capabilities. These countries benefit most from regional infrastructure and knowledge sharing.

- *Nascent engagement jurisdictions* — smaller or less financially integrated economies where CBDC is not yet a policy priority but where tokenized payment infrastructure could address acute access and cost problems. For these countries, a plug-and-play approach — joining existing regional infrastructure rather than building from scratch — is the pragmatically viable path.

The CBWeb3 initiative is designed to serve all three tiers, with the regional testnet providing a shared experimentation environment and the governance framework ensuring that no single country's design choices are imposed on others.

---

## 3. What Is Being Issued? Clarifying the Forms of Digital Money

Policy discussions about tokenization are frequently imprecise about what is actually being issued. This imprecision creates confusion in legal analysis, in public communication, and in the design of interoperability standards. This section establishes the key distinctions.

**Tokenized central bank money (tCeBM)** is a digital representation of a liability of the central bank, issued on a distributed ledger or programmable infrastructure. It carries the same credit quality and settlement finality as reserves held in traditional central bank accounts. Because the issuer is the monetary authority, tCeBM does not introduce credit risk or liquidity risk beyond what already exists in the monetary system. At the wholesale level, tCeBM functions as the ultimate settlement asset — the equivalent of a reserve balance, but programmable and capable of atomic settlement with other tokenized assets. At the retail level, it functions as a digital form of cash issued directly to the public, though most LAC jurisdictions are currently focused on wholesale applications.

**Tokenized commercial bank deposits (tDeposits)** are digital representations of claims on a commercial bank, issued on programmable infrastructure. They carry the credit risk of the issuing institution and are subject to deposit insurance limits. In a two-tier model — which most LAC jurisdictions favor — commercial banks issue tDeposits backed by reserves held at the central bank, while the central bank provides settlement infrastructure using tCeBM. The distinction matters for system design: interoperability between tDeposits issued by different banks requires a common settlement layer, and that layer is most efficiently provided by tCeBM.

**Stablecoins and other private digital instruments** are tokens issued by private entities, typically backed by reserves of fiat currency, government securities, or other assets. Their value proposition is stability relative to a reference currency, combined with programmability and accessibility. However, they are not central bank liabilities, and their governance, reserve management, and regulatory treatment vary widely. Fiat-backed stablecoins denominated in US dollars introduce foreign currency denomination risk at scale if adopted broadly in LAC economies; algorithmic stablecoins introduce additional stability risks. From a policy perspective, private digital instruments are complements or potential competitors to tCeBM — not substitutes for it.

The CBWeb3 platform focuses on tCeBM issuance and wholesale settlement. The smart contracts governing issuance, HTLC-based cross-border settlement, and AMM-based liquidity management are all designed around the central bank as the primary issuer and the tCeBM as the settlement asset of record.

---

## 4. Legal and Regulatory Readiness: The Critical Path

Before embarking on a tokenized issuance program, authorities need to assess whether their existing legal and regulatory frameworks are fit for purpose — or whether legislative or regulatory changes are required. The following checklist structures this assessment across five domains.

**Domain 1 — Legal definition of central bank money.** Does existing law permit the central bank to issue money in digital or tokenized form? In some jurisdictions, central bank legislation references "banknotes and coins" in ways that could be interpreted as excluding digital equivalents. Clarification by legal opinion or regulatory guidance may be sufficient; in other cases, a legislative amendment is required.

**Domain 2 — Settlement finality.** Does the legal system recognize the finality of settlement in a distributed ledger environment? Settlement finality — the point at which a transfer is irrevocable — is foundational for central bank settlement infrastructure. Some jurisdictions' financial market infrastructure laws explicitly address finality; others rely on common law principles or general contract law, which may create uncertainty in the context of smart contract execution.

**Domain 3 — Access rules.** Who may hold or transact in tCeBM? At the wholesale level, access is typically restricted to regulated financial institutions — commercial banks, payment service providers, and potentially securities settlement entities. The legal basis for defining and enforcing access rules, and for excluding non-regulated entities, must be clearly established.

**Domain 4 — AML/CFT and FX regulations.** How do existing anti-money laundering, counter-terrorist financing, and foreign exchange regulations apply to tCeBM transactions? In a programmable environment, compliance obligations can be embedded in smart contract logic, but the legal recognition of this approach must be verified. Cross-border transactions in tCeBM may trigger FX reporting requirements or capital flow management measures that need to be coordinated with central bank issuance policy.

**Domain 5 — Data protection and cybersecurity.** Does the privacy and data protection framework apply to transaction data recorded on a distributed ledger? If the ledger is permissioned and access-controlled, the central bank and participating institutions are data controllers and must comply with applicable data protection obligations. Cybersecurity requirements for systemically important financial infrastructure must also be mapped to the technical architecture of the platform.

**Current status across LAC jurisdictions.** The following is a high-level summary of regulatory readiness by country tier, based on publicly available information as of 2025–2026:

- *Brazil:* The Banco Central do Brasil has a strong legal basis for digital currency issuance under the LIFT Lab framework and the Drex program. Settlement finality, AML/CFT, and data protection frameworks (LGPD) are well-developed. Brazil is the most legally advanced jurisdiction in the region for CBDC purposes.
- *Mexico:* Banxico operates under a mandate that permits electronic money issuance, and the Fintech Law (2018) established a regulatory framework for digital assets. CBDC-specific legislation is under development but not yet enacted.
- *Colombia:* The Banco de la República has authority to issue digital money under its organic statute, though CBDC-specific regulations have not yet been issued. AML/CFT frameworks are robust.
- *Peru:* Banco Central de Reserva del Perú has conducted exploratory work; legal framework is generally permissive but lacks CBDC-specific provisions. The SBS (financial supervisor) would need to coordinate on access rules.
- *Chile:* Banco Central de Chile published a CBDC feasibility study in 2022. Legal authority for digital issuance is considered available under current statute, but implementing regulations are pending.
- *Costa Rica, Dominican Republic, Ecuador:* Active interest and exploratory research, but no formal legal framework adaptations underway. These countries represent strong candidates for regional infrastructure adoption rather than independent platform development.
- *Smaller and Eastern Caribbean economies:* The ECCB's DCash initiative provides a regional precedent. Legal frameworks vary by jurisdiction; the regional central bank model simplifies some coordination challenges.

Authorities in jurisdictions without CBDC-specific frameworks should not interpret this gap as a barrier to participation in regional experimentation. Most legal systems provide sufficient existing authority for a closed, controlled testnet phase. The legal work required before going live at scale is substantially greater, but can be pursued in parallel with technical capacity building.

---

## 5. Operating Models for Issuance (Non-Technical)

The choice of operating model is one of the most consequential decisions a central bank faces in designing a tokenized issuance program. It determines who builds and operates the infrastructure, who holds what risk, who has access to the settlement layer, and how the system evolves over time. This section presents three models and analyzes the advantages and disadvantages of each.

**Model A — Central Bank-Led**

In this model, the central bank designs, builds, and operates the core tokenized settlement infrastructure. Commercial banks and other regulated participants interact with the platform through standardized APIs but do not control the underlying infrastructure. The central bank maintains full custody of the issuance logic, the settlement ledger, and the governance rules.

*Advantages:* Maximum control over monetary policy transmission. Full visibility into settlement flows. No dependency on third-party infrastructure providers. Strongest alignment between issuance design and monetary policy objectives. Most credible architecture from a systemic risk standpoint.

*Disadvantages:* Highest institutional capacity requirement — central banks in many LAC jurisdictions do not currently have the technology teams or operational experience to build and run complex distributed infrastructure. Long development timelines. Risk of building closed, proprietary systems that are difficult to interoperate with regional or global networks. High upfront cost.

*Best suited for:* Tier 1 jurisdictions (Brazil, Mexico, Colombia) with strong technology capacity and a strategic interest in being regional infrastructure anchors.

**Model B — Hybrid**

In this model, the central bank defines the legal and policy framework, provides the settlement asset (tCeBM), and maintains supervisory oversight, but delegates the design, construction, and operation of the technical infrastructure to a regulated entity or consortium. This entity — which may be a central bank-owned company, a financial sector utility, or a regulated technology provider — operates the platform under central bank license and supervision.

*Advantages:* Allows the central bank to focus on its core mandate (monetary policy and systemic stability) while leveraging private sector or consortium technology expertise. Faster to deploy than a fully central bank-built system. Can incorporate commercial innovation — smart contract features, user interfaces, integration tooling — more rapidly. Spreads operational costs across participants.

*Disadvantages:* The central bank is dependent on the regulated operator for platform availability and security. Governance of the operator entity requires careful design to avoid conflicts of interest. The legal relationship between the central bank and the operator must clearly establish liability for system failures. Risk of mission drift if commercial considerations influence platform design in ways that compromise monetary policy objectives.

*Best suited for:* Most LAC jurisdictions in Tiers 1 and 2, where central banks want to move faster than their internal technology capacity allows while maintaining legal and policy control. The CBWeb3 model — with LNET as infrastructure operator and IDB Lab as funder — is a variant of this approach.

**Model C — Fully Intermediated**

In this model, the central bank issues reserves to commercial banks in tokenized form but does not operate any retail or wholesale platform directly. Commercial banks and licensed payment service providers build the customer-facing and interbank settlement infrastructure on top of the tokenized reserve layer. The central bank's role is limited to issuance, oversight, and setting interoperability standards.

*Advantages:* Minimal operational burden on the central bank. Maximum leverage of existing private sector payment infrastructure and commercial innovation. Can be deployed incrementally, with each participating institution building its own stack. Avoids the risk of the central bank becoming a single point of failure for the payment system.

*Disadvantages:* Interoperability between different institutions' implementations is not automatic — it requires strong standards governance and enforcement. The central bank has less direct visibility into settlement flows. Risk of fragmentation if participants build incompatible systems. Harder to ensure systemic resilience if the infrastructure is fragmented across many private operators.

*Best suited for:* Jurisdictions where the central bank has a strong regulatory and supervisory mandate but limited appetite to operate infrastructure, and where the commercial banking sector has sufficient technical capacity to build interoperable implementations.

**Choosing a model.** In practice, most LAC jurisdictions will adopt a hybrid approach, with the balance between central bank control and private sector delegation varying by institutional context. The key principle is that the choice of operating model should be made deliberately and documented formally — because it will be very difficult to change once infrastructure is built and participants have invested in integration.

---

## 6. A Minimum Viable Blueprint for Tokenized Issuance

The most common barrier to action is not legal uncertainty or technology readiness — it is the perceived scale and complexity of the undertaking. This section proposes a minimum viable approach: a deliberately scoped, low-risk starting point that allows institutions to begin learning and building capacity without committing to a full-scale transformation.

**The closed user group principle.** The recommended starting point is a closed, permissioned environment with a small, pre-defined set of regulated participants. This is not a pilot in the pejorative sense — it is a structurally sound approach to deploying settlement infrastructure, analogous to how real-time gross settlement systems were initially restricted to large-value interbank transactions before expanding to broader use cases.

**Eligibility criteria for the closed user group.** Participation in the initial closed group should be conditioned on meeting a defined set of criteria. Institutions should be:

1. *Regulated by a competent authority* in the jurisdiction — a licensed commercial bank, payment service provider, or securities settlement entity subject to prudential supervision and AML/CFT obligations.
2. *Technically capable of API integration* — able to connect to the platform's standardized API layer and complete a technical integration test with the central bank or platform operator.
3. *Operationally committed* — able to dedicate a named technical and legal contact, participate in testing sessions, and comply with incident reporting requirements.
4. *Legally cleared* — having received a formal legal opinion confirming that their participation in the closed group is consistent with applicable law and their institutional mandate.
5. *Contractually bound* — having signed a participation agreement with the central bank or platform operator that establishes the terms of access, liability allocation, and data governance.

Institutions that meet these criteria but are not selected in the first cohort due to capacity constraints should be placed on a defined waitlist with clear timeline expectations.

**Recommended initial use case.** The first use case should be as simple as possible: interbank settlement in domestic currency between two or more participants in the closed group, using a predefined payment message standard (ISO 20022 is recommended). This use case generates immediate operational learning — about latency, settlement finality confirmation, error handling, and reconciliation — without introducing foreign currency complexity, retail access considerations, or cross-border coordination requirements.

**Success criteria.** The closed group phase should be considered complete when: (a) at least two institutions have completed end-to-end settlement of a defined set of test transactions with cryptographic finality; (b) the central bank has validated that the settlement ledger correctly reflects all transactions; and (c) a post-pilot assessment has been completed and shared with all participants.

**Timeline.** A realistic timeline from decision to successful completion of the closed group phase is six to twelve months, depending on legal preparation time and technology integration complexity. Central banks that are already operating on the CBWeb3 testnet can compress this timeline significantly, as the core infrastructure, API specifications, and test vectors are already available.

---

## 7. From Domestic Issuance to Real Utility

A central bank that has successfully completed a domestic issuance pilot has achieved something meaningful — but has not yet unlocked the most significant source of value in tokenized money. Domestic issuance alone replicates, in a new technological form, a payment infrastructure that already exists. The transformative potential of tokenized central bank money emerges when it becomes interoperable: when it can be used to settle cross-border transactions, when liquidity can flow between jurisdictions without multiple rounds of currency conversion, and when financial institutions in different countries can transact with each other using a shared settlement protocol.

This transition — from domestic issuance to regional utility — requires deliberate sequencing. It involves connecting domestic platforms to regional infrastructure, establishing legal and contractual frameworks for cross-border use, and resolving the liquidity management challenges that arise when non-convertible currencies are included in the settlement network.

The CBWeb3 platform is designed to support precisely this transition. Its hub-and-spoke architecture allows each country's central bank to maintain full sovereignty over its domestic issuance and settlement while connecting to a shared transnational hub for cross-border transactions. A country can join the regional network incrementally — beginning with bilateral connectivity to one counterpart jurisdiction — and expand connectivity over time as operational experience and legal frameworks mature.

The following two sections address the specific design choices involved in cross-border settlement and liquidity management.

---

## 8. Cross-Border Settlement Models for LAC

Three practical models exist for cross-border tokenized settlement, each with different implications for complexity, scalability, and the degree of regional coordination required.

**Model 1 — Tokenized Correspondent Banking (Enhanced PvP)**

In this model, the bilateral relationship between a domestic bank in Country A and its correspondent bank in Country B is replicated on tokenized infrastructure. Each leg of the payment — the debit in Country A's currency and the credit in Country B's currency — is settled atomically using a Hash Time-Lock Contract (HTLC). Settlement finality in both legs is conditional: neither leg settles unless both settle simultaneously, eliminating the principal risk that exists in traditional correspondent banking.

*Advantages:* Builds on existing correspondent relationships and legal frameworks. No requirement for a shared regional infrastructure or centralized liquidity. Can be deployed bilaterally, one corridor at a time, without requiring multilateral coordination. Scenario A of the CBWeb3 platform implements exactly this model and has been tested end-to-end.

*Disadvantages:* Scales linearly with the number of corridors — N*(N-1)/2 bilateral agreements for full regional connectivity. Does not solve the liquidity pre-funding problem: each correspondent still needs to hold pre-funded balances in both currencies. Limited to currency pairs where a bilateral correspondent relationship already exists or can be established.

**Model 2 — Bilateral Corridors with Shared Settlement**

In this model, two central banks establish a direct, bilateral settlement arrangement using tokenized central bank money, bypassing the commercial correspondent layer entirely. The arrangement is governed by a bilateral agreement between the two monetary authorities and uses a shared settlement infrastructure — potentially the CBWeb3 hub — as the technical intermediary.

*Advantages:* Eliminates commercial bank intermediation from the interbank settlement layer, reducing cost and risk. Provides central banks with direct visibility into cross-border settlement flows. Stronger foundation for monetary policy coordination between jurisdictions.

*Disadvantages:* Requires bilateral treaty or memorandum of understanding between central banks — a process that can be lengthy and politically complex. Still scales as a mesh of bilateral agreements, though more slowly than commercial bilateral corridors. Requires both central banks to have sufficient technology maturity to connect to shared infrastructure.

**Model 3 — Shared Regional Platform**

In this model, participating countries connect to a single, multilateral settlement hub that supports multi-currency settlement without requiring pre-established bilateral arrangements between every pair of participants. The hub provides liquidity pooling, automated market-making for FX conversion, and a single legal and governance framework for all participants.

*Advantages:* Maximum scalability — N participants can settle with each other without N*(N-1)/2 bilateral agreements. Most efficient use of liquidity — pooled liquidity reduces pre-funding requirements significantly. Enables new participants to join incrementally, with immediate connectivity to all existing members. The AMM (Scenario B) of the CBWeb3 platform implements this model.

*Disadvantages:* Highest coordination requirement — establishing a multilateral governance framework, allocating costs, and defining liability arrangements across multiple sovereign jurisdictions is complex. Requires a trusted, neutral operator for the hub infrastructure. Systemic risk is concentrated at the hub; robust technical and governance resilience is essential.

**Coexistence in practice.** These three models are not mutually exclusive. A realistic regional architecture will likely see all three in use simultaneously: enhanced bilateral correspondent banking for corridors with existing commercial relationships, direct central bank bilateral corridors for the highest-volume, most strategically significant pairs, and a shared regional platform as the backbone for multilateral connectivity. The CBWeb3 platform is designed to support all three, with Scenario A (HTLC PvP) and Scenario B (AMM hub) as the two primary pathways.

---

## 9. Liquidity Management for Non-Convertible Currencies

The liquidity management challenge in a multi-currency tokenized settlement system is acute for LAC. Many currencies in the region are not freely convertible, and the FX markets for most LAC currency pairs are thin, fragmented, and expensive. A tokenized cross-border settlement system that does not adequately address liquidity will replicate — in a more technologically elegant form — the same cost and access problems that currently afflict correspondent banking.

Three mechanisms exist to manage liquidity in a multi-currency settlement system, and each involves trade-offs between efficiency, risk, and policy control.

**Mechanism 1 — Pre-funded Bilateral Balances**

Each participant pre-funds accounts denominated in the currencies of its counterparts. When a payment is initiated, it is settled against the pre-funded balance, and the balance is replenished periodically. This is the mechanism used in Scenario A of the CBWeb3 platform (HTLC PvP): each leg of the payment is locked before settlement is initiated, ensuring that both parties have the required funds available.

*Trade-offs:* Simple and low-risk — settlement cannot fail due to insufficient funds if pre-funding is enforced. But it ties up significant capital in pre-funded accounts across all active corridors, which is costly for institutions with constrained balance sheets. For a system with many active corridors and currencies, pre-funding requirements can become prohibitive.

**Mechanism 2 — Bilateral Credit Lines**

Counterpart central banks or commercial banks extend credit to each other for intraday settlement, with end-of-day net settlement in a reserve asset. This is analogous to the intraday credit facilities that central banks provide in traditional RTGS systems.

*Trade-offs:* Reduces pre-funding requirements significantly, enabling more efficient use of liquidity. But introduces credit risk: if a participant cannot fund its end-of-day net position, the counterpart is exposed. Requires strong collateral frameworks and centralized credit risk management. Technically straightforward to implement but legally and institutionally complex — bilateral credit lines between central banks in different jurisdictions require legal authority and a formal credit agreement.

**Mechanism 3 — Pooled Liquidity with Automated Market-Making**

Participants contribute liquidity to a shared pool denominated in multiple currencies. When a cross-border payment is initiated, the AMM algorithm determines the exchange rate and routes the transaction through the pool, converting currencies automatically without requiring a pre-established bilateral credit relationship. This is Scenario B of the CBWeb3 platform.

*Trade-offs:* Most capital-efficient for a system with many active participants and corridors — liquidity is pooled rather than fragmented across bilateral pre-funding accounts. Exchange rates are determined algorithmically, which is transparent but may deviate from market rates for illiquid currency pairs, creating basis risk. The central bank cedes some control over the exchange rate at which its currency is converted — a politically sensitive consideration for some jurisdictions. Governance of the pool — who sets parameters, who manages the AMM algorithm, who bears residual risk — requires careful institutional design.

**Recommended approach.** For the initial phase of regional deployment, pre-funded bilateral balances (Mechanism 1) are recommended: they are operationally simple, eliminate settlement risk, and require no multilateral governance. As the network grows and liquidity management costs become significant, a transition to pooled liquidity with AMM (Mechanism 3) becomes attractive — but only after the AMM governance framework has been established and the algorithm has been validated in production-equivalent conditions. Intraday credit facilities (Mechanism 2) may be appropriate for specific high-volume bilateral corridors between jurisdictions with strong bilateral institutional relationships.

---

## 10. Policy Considerations and Governance Approaches

Tokenized central bank money raises a set of policy questions that do not have universally correct answers. Different jurisdictions will resolve them differently, reflecting their monetary policy frameworks, financial stability concerns, and institutional traditions. This section frames the key considerations and presents the governance approaches that the CBWeb3 initiative has adopted.

**Monetary sovereignty.** The most fundamental policy concern is whether tokenized issuance preserves or erodes monetary sovereignty. Properly designed, tokenized central bank money strengthens sovereignty: it gives the monetary authority a digital instrument that can compete with private alternatives while remaining under public control. The risk to sovereignty arises if the technical infrastructure is designed in ways that limit the central bank's ability to set terms of access, control issuance volume, or enforce monetary policy transmission. This is why the CBWeb3 architecture maintains each country's spoke network as fully sovereign — the central bank of each jurisdiction controls its own issuance parameters and can disconnect from the regional hub at any time without affecting its domestic settlement operations.

**Financial stability.** The introduction of a new form of central bank money has implications for financial intermediation. If tCeBM at the wholesale level makes it significantly easier for large institutions to settle directly with the central bank, bypassing commercial bank intermediation, this could affect the funding model of commercial banks. This risk is more acute at the retail level (if retail CBDC displaces bank deposits) than at the wholesale level (where the central bank already provides settlement accounts). Most LAC jurisdictions are focused on wholesale applications for now, which substantially limits this risk.

**Privacy.** Transaction privacy is a significant concern for both individual and institutional users of a tokenized payment system. The CBWeb3 platform uses the Hyperledger Paladin privacy framework with Zeto zero-knowledge proofs to enable transaction confidentiality: the existence of a transaction can be recorded on the ledger without revealing the amounts, identities of counterparties, or transaction details to unauthorized participants. Authorities must determine the appropriate balance between transaction privacy and the AML/CFT visibility that supervisory authorities require — and must ensure that this balance is embedded in the technical architecture, not left to ad hoc governance decisions.

**Governance of the CBWeb3 Working Group.** The CBWeb3 initiative operates through a Digital Public Good Working Group hosted under the Linux Foundation Decentralized Trust governance framework. The working group has two co-chairs nominated by CEMLA and FLAR — the two leading regional financial institutions — providing an institutional anchor that is independent of any single central bank or commercial interest. Technical decisions are made by a maintainer core of technical leads, with broader community input through a bi-weekly technical sync and a public issue tracker. Formal decisions that cannot reach consensus are resolved by GitVote, with a 50%+1 threshold and a seven-day comment period. This governance model balances openness — any institution can contribute — with accountability, ensuring that the infrastructure evolves in ways that reflect the needs of the regional community rather than any single participant.

---

## 11. Final Remarks: A Gradual and Coordinated Path Forward

The analysis presented in this document leads to a clear conclusion: LAC central banks and financial authorities have both the motivation and the means to act on tokenized central bank money. The motivation is structural — the pressures of correspondent banking decline, stablecoin expansion, and cross-border payment inefficiency are not going away, and they will intensify as digital financial infrastructure becomes more central to global commerce. The means are available — the CBWeb3 regional testnet provides tested infrastructure, proven interoperability protocols, and a governance framework designed for the institutional context of LAC.

The path forward does not require all countries to move at the same pace, nor does it require full legal certainty before taking the first step. What it requires is clarity of intention, institutional commitment, and a willingness to learn from controlled experimentation.

**Summary Action Roadmap**

The following roadmap organizes the key actions by phase, providing a structured path from initial assessment to regional integration:

*Phase 1 — Legal and Institutional Readiness (Months 1–3)*

- Conduct internal legal assessment using the five-domain checklist in Section 4
- Identify gaps and determine whether legal opinions, regulatory guidance, or legislative changes are required
- Designate an internal project lead and establish a cross-functional working group (legal, technology, monetary policy)
- Engage with the CBWeb3 Working Group to begin the onboarding process

*Phase 2 — Technical Engagement and Testnet Connection (Months 3–6)*

- Complete technical integration with the CBWeb3 testnet using the standardized API specifications and test vectors available in the public repository
- Execute the 15-minute PvP quickstart tutorial and validate technical connectivity
- Participate in at least one bi-weekly community call to engage with the technical working group
- Define the initial closed user group according to the eligibility criteria in Section 6

*Phase 3 — Closed Group Pilot (Months 6–12)*

- Execute the first set of domestic interbank settlement transactions in the closed group environment
- Complete post-pilot assessment and share findings with the CBWeb3 Working Group
- Begin legal preparation for cross-border use: identify counterpart jurisdictions, initiate bilateral legal review, define participation agreement terms

*Phase 4 — Cross-Border Connectivity (Months 12–24)*

- Select the cross-border settlement model appropriate to your jurisdiction's context (Section 8)
- Establish bilateral or multilateral connectivity through the CBWeb3 hub
- Participate in the regional governance framework and contribute to the evolution of the CBWeb3 Blueprint

Central banks that begin this process now — even at a modest scale — will be better positioned to shape the regional architecture than those that wait for a consensus that may never fully arrive. The CBWeb3 initiative is an invitation to participate in building shared infrastructure for a regional financial future that serves the priorities of Latin America and the Caribbean.

---

---

# VERSIÓN EN ESPAÑOL

---

## 1. Resumen Ejecutivo: Un Marco de Decisión para las Autoridades de ALC

América Latina y el Caribe se encuentran en un momento decisivo en la evolución del dinero. Los pagos transfronterizos en la región siguen siendo de los más costosos y lentos del mundo. Las relaciones de corresponsalía bancaria se están reduciendo. Las stablecoins y las monedas digitales extranjeras ganan terreno en economías dolarizadas o semi-dolarizadas, erosionando el espacio de política disponible para los bancos centrales. En este contexto, el dinero digital de banco central ha emergido no como un experimento tecnológico especulativo, sino como un instrumento de política concreto que varias autoridades regionales ya están explorando seriamente.

Este documento está dirigido a los tomadores de decisiones que deben determinar si involucrarse con esta transformación y cómo hacerlo. Se organiza en torno a las preguntas prácticas que enfrentan los altos funcionarios: ¿Qué se está emitiendo exactamente? ¿Está listo nuestro marco legal? ¿Qué modelo operativo se ajusta a nuestro contexto institucional? ¿Dónde empezamos y cómo nos conectamos con la región?

El marco de decisión que propone este documento tiene tres dimensiones. Primero, **la pregunta sobre el activo de liquidación**: si emitir dinero digital de banco central a nivel mayorista, minorista o ambos, y cómo posicionarlo respecto a los depósitos bancarios comerciales tokenizados y los instrumentos digitales privados. Segundo, **la pregunta sobre el modelo operativo**: si el banco central lidera la infraestructura, delega en intermediarios o adopta un esquema híbrido, y qué implica cada opción para la gobernanza, los costos y el riesgo sistémico. Tercero, **la pregunta sobre la integración transfronteriza**: si perseguir corredores bilaterales, unirse a una plataforma regional compartida o modernizar las relaciones de corresponsalía bancaria, y cómo secuenciar estas decisiones en el tiempo.

La conclusión práctica de este documento es que las condiciones para actuar ya están dadas. Los países no necesitan esperar certeza jurídica perfecta, tecnología completamente probada ni consenso regional antes de comenzar. Un enfoque deliberado y gradual —comenzando con un grupo cerrado de participantes regulados en un caso de uso doméstico controlado— permite a las instituciones aprender, desarrollar capacidades y reducir riesgos antes de ampliar el alcance. La red de pruebas regional CBWeb3 existe precisamente para apoyar esta fase de experimentación, brindando infraestructura compartida, herramientas de interoperabilidad y una comunidad de práctica entre bancos centrales e instituciones financieras de ALC.

---

## 2. Por Qué el Dinero Digital de Banco Central Importa para ALC Ahora

El caso a favor del dinero digital de banco central en América Latina y el Caribe no se construye sobre el entusiasmo tecnológico. Se construye sobre un conjunto específico de presiones estructurales que ya están redefiniendo el panorama financiero regional y que exigen una respuesta de política.

**El problema de la corresponsalía bancaria.** El número de relaciones de corresponsalía bancaria activas en ALC ha disminuido sostenidamente durante la última década, impulsado por estrategias de reducción de riesgo de los bancos globales. Para las economías más pequeñas y para ciertas categorías de transacciones —particularmente remesas y financiamiento de comercio exterior— esta contracción ha elevado los costos, alargado los tiempos de liquidación y, en algunos corredores, ha eliminado efectivamente el acceso a infraestructura de pagos internacionales asequible. El dinero digital de banco central, utilizado como activo de liquidación en un modelo directo o semi-directo, puede reducir la dependencia de intermediarios de corresponsalía al habilitar la liquidación pago-contra-pago con finalidad criptográfica.

**La presión de las stablecoins y la dolarización.** En toda la región, la adopción de stablecoins crece —particularmente en economías con historias de inestabilidad cambiaria o acceso restringido a cuentas en dólares estadounidenses. Si bien esta adopción refleja una demanda genuina de valor digital más estable y accesible, también representa una potencial transferencia de soberanía monetaria. Si una parte significativa de las transacciones domésticas migra hacia instrumentos denominados en monedas extranjeras emitidos por entidades privadas fuera del alcance regulatorio, la capacidad de los bancos centrales para implementar política monetaria, observar el riesgo sistémico y hacer cumplir las normas AML/CFT se ve materialmente afectada. El dinero digital de banco central ofrece una alternativa soberana: digital, programable, interoperable, pero emitida y gobernada por la autoridad monetaria.

**La ineficiencia de los pagos transfronterizos.** Enviar dinero a través de las fronteras de ALC sigue siendo lento, opaco y costoso. Los costos promedio de remesas en la región permanecen por encima del objetivo del G20 del 3%. Para las empresas, la imposibilidad de liquidar en monedas locales obliga a depender de la intermediación en dólares, añadiendo costos cambiarios y riesgo de liquidación. La liquidación transfronteriza tokenizada —ya sea a través de corredores bilaterales o plataformas compartidas— puede comprimir los ciclos de liquidación de días a segundos y eliminar capas de intermediación que no cumplen otra función que generar fricción.

**Un panorama regional diferenciado.** ALC no es una región monolítica, y cualquier evaluación honesta de la oportunidad debe reconocer la heterogeneidad en el contexto de cada país, la capacidad institucional y la disposición para la adopción. Tres grandes grupos caracterizan el panorama actual:

- *Jurisdicciones avanzadas* — países como Brasil, México y Colombia, con marcos regulatorios bien desarrollados, sofisticada capacidad tecnológica en sus bancos centrales y programas activos de investigación en CBDC o tokenización. Estos países están posicionados para ser pioneros y anclas regionales.

- *Jurisdicciones en desarrollo de capacidades* — países como Perú, Chile, Ecuador, Costa Rica y República Dominicana, donde los marcos regulatorios son sólidos pero la legislación específica sobre CBDC aún está emergiendo y los equipos tecnológicos de los bancos centrales están desarrollando sus capacidades. Estos países se benefician más de la infraestructura regional y el intercambio de conocimientos.

- *Jurisdicciones con involucramiento incipiente* — economías más pequeñas o menos integradas financieramente donde el CBDC aún no es una prioridad de política, pero donde la infraestructura de pagos tokenizados podría abordar problemas agudos de acceso y costo. Para estos países, un enfoque plug-and-play —unirse a la infraestructura regional existente en lugar de construir desde cero— es el camino pragmáticamente viable.

La iniciativa CBWeb3 está diseñada para servir a los tres grupos, con la red de pruebas regional brindando un entorno de experimentación compartido y el marco de gobernanza asegurando que las opciones de diseño de ningún país se impongan a los demás.

---

## 3. ¿Qué Se Está Emitiendo? Clarificando las Formas de Dinero Digital

Los debates de política sobre tokenización frecuentemente son imprecisos sobre lo que realmente se está emitiendo. Esta imprecisión genera confusión en el análisis legal, en la comunicación pública y en el diseño de estándares de interoperabilidad. Esta sección establece las distinciones clave.

**El dinero digital de banco central (tCeBM)** es una representación digital de un pasivo del banco central, emitida en un libro mayor distribuido o infraestructura programable. Tiene la misma calidad crediticia y finalidad de liquidación que las reservas mantenidas en cuentas tradicionales del banco central. Dado que el emisor es la autoridad monetaria, el tCeBM no introduce riesgo de crédito ni de liquidez más allá de lo que ya existe en el sistema monetario. A nivel mayorista, el tCeBM funciona como el activo de liquidación definitivo —el equivalente de un saldo de reservas, pero programable y capaz de liquidación atómica con otros activos tokenizados. A nivel minorista, funciona como una forma digital del efectivo emitida directamente al público, aunque la mayoría de las jurisdicciones de ALC están actualmente enfocadas en aplicaciones mayoristas.

**Los depósitos bancarios comerciales tokenizados (tDeposits)** son representaciones digitales de derechos frente a un banco comercial, emitidos en infraestructura programable. Llevan el riesgo crediticio de la institución emisora y están sujetos a los límites del seguro de depósitos. En un modelo de dos niveles —que favorece la mayoría de las jurisdicciones de ALC— los bancos comerciales emiten tDeposits respaldados por reservas mantenidas en el banco central, mientras que el banco central provee infraestructura de liquidación utilizando tCeBM. La distinción importa para el diseño del sistema: la interoperabilidad entre tDeposits emitidos por distintos bancos requiere una capa de liquidación común, y esa capa se provee más eficientemente con tCeBM.

**Las stablecoins y otros instrumentos digitales privados** son tokens emitidos por entidades privadas, típicamente respaldados por reservas de moneda fiat, valores gubernamentales u otros activos. Su propuesta de valor es la estabilidad respecto a una moneda de referencia, combinada con programabilidad y accesibilidad. Sin embargo, no son pasivos de banco central, y su gobernanza, gestión de reservas y tratamiento regulatorio varían ampliamente. Las stablecoins respaldadas en fiat denominadas en dólares estadounidenses introducen riesgo de denominación en moneda extranjera a escala si se adoptan ampliamente en las economías de ALC; las stablecoins algorítmicas introducen riesgos de estabilidad adicionales. Desde una perspectiva de política, los instrumentos digitales privados son complementos o competidores potenciales del tCeBM —no sustitutos de este.

La plataforma CBWeb3 se enfoca en la emisión de tCeBM y la liquidación mayorista. Los contratos inteligentes que gobiernan la emisión, la liquidación transfronteriza basada en HTLC y la gestión de liquidez basada en AMM están todos diseñados en torno al banco central como emisor principal y al tCeBM como activo de liquidación de referencia.

---

## 4. Preparación Legal y Regulatoria: El Camino Crítico

Antes de embarcarse en un programa de emisión tokenizada, las autoridades necesitan evaluar si sus marcos legales y regulatorios vigentes son aptos para el propósito, o si se requieren cambios legislativos o regulatorios. La siguiente lista de verificación estructura esta evaluación en cinco dominios.

**Dominio 1 — Definición legal del dinero del banco central.** ¿Permite la legislación vigente al banco central emitir dinero en forma digital o tokenizada? En algunas jurisdicciones, la legislación del banco central hace referencia a "billetes y monedas" de manera que podría interpretarse como excluyente de los equivalentes digitales. La aclaración mediante opinión legal o guía regulatoria puede ser suficiente; en otros casos, se requiere una enmienda legislativa.

**Dominio 2 — Finalidad de la liquidación.** ¿Reconoce el sistema legal la finalidad de la liquidación en un entorno de libro mayor distribuido? La finalidad de la liquidación —el punto en que una transferencia es irrevocable— es fundamental para la infraestructura de liquidación del banco central. Las leyes de infraestructura del mercado financiero de algunas jurisdicciones abordan explícitamente la finalidad; otras se apoyan en principios del derecho consuetudinario o el derecho contractual general, lo que puede generar incertidumbre en el contexto de la ejecución de contratos inteligentes.

**Dominio 3 — Reglas de acceso.** ¿Quién puede tener o transaccionar en tCeBM? A nivel mayorista, el acceso típicamente se restringe a instituciones financieras reguladas —bancos comerciales, proveedores de servicios de pago y, potencialmente, entidades de liquidación de valores. La base legal para definir y hacer cumplir las reglas de acceso, y para excluir a entidades no reguladas, debe estar claramente establecida.

**Dominio 4 — Regulaciones AML/CFT y cambiarias.** ¿Cómo se aplican las regulaciones existentes contra el lavado de dinero, el financiamiento del terrorismo y las divisas a las transacciones de tCeBM? En un entorno programable, las obligaciones de cumplimiento pueden incorporarse en la lógica de los contratos inteligentes, pero debe verificarse el reconocimiento legal de este enfoque. Las transacciones transfronterizas en tCeBM pueden activar requisitos de reporte cambiario o medidas de gestión de flujos de capital que deben coordinarse con la política de emisión del banco central.

**Dominio 5 — Protección de datos y ciberseguridad.** ¿Se aplica el marco de privacidad y protección de datos a los datos de transacciones registrados en un libro mayor distribuido? Si el libro mayor es con permisos y control de acceso, el banco central y las instituciones participantes son responsables del tratamiento de datos y deben cumplir con las obligaciones de protección de datos aplicables. Los requisitos de ciberseguridad para infraestructura financiera sistémicamente importante también deben mapearse a la arquitectura técnica de la plataforma.

**Estado actual en las jurisdicciones de ALC.** El siguiente es un resumen de alto nivel de la preparación regulatoria por grupo de países, basado en información pública disponible a 2025–2026:

- *Brasil:* El Banco Central do Brasil cuenta con una sólida base legal para la emisión de moneda digital bajo el marco LIFT Lab y el programa Drex. Los marcos de finalidad de liquidación, AML/CFT y protección de datos (LGPD) están bien desarrollados. Brasil es la jurisdicción legalmente más avanzada de la región para propósitos de CBDC.
- *México:* Banxico opera bajo un mandato que permite la emisión de dinero electrónico, y la Ley Fintech (2018) estableció un marco regulatorio para activos digitales. La legislación específica sobre CBDC está en desarrollo pero aún no ha sido promulgada.
- *Colombia:* El Banco de la República tiene autoridad para emitir dinero digital bajo su estatuto orgánico, aunque aún no se han emitido regulaciones específicas sobre CBDC. Los marcos AML/CFT son robustos.
- *Perú:* El Banco Central de Reserva del Perú ha realizado trabajo exploratorio; el marco legal es generalmente permisivo pero carece de disposiciones específicas sobre CBDC. La SBS (supervisor financiero) necesitaría coordinar sobre las reglas de acceso.
- *Chile:* El Banco Central de Chile publicó un estudio de factibilidad sobre CBDC en 2022. La autoridad legal para la emisión digital se considera disponible bajo el estatuto vigente, pero las regulaciones de implementación están pendientes.
- *Costa Rica, República Dominicana, Ecuador:* Interés activo e investigación exploratoria, pero sin adaptaciones formales al marco legal en curso. Estos países representan candidatos sólidos para la adopción de infraestructura regional en lugar del desarrollo de plataformas independientes.
- *Economías más pequeñas y del Caribe Oriental:* La iniciativa DCash del ECCB proporciona un precedente regional. Los marcos legales varían por jurisdicción; el modelo de banco central regional simplifica algunos desafíos de coordinación.

Las autoridades en jurisdicciones sin marcos específicos sobre CBDC no deben interpretar esta brecha como una barrera para participar en la experimentación regional. La mayoría de los sistemas legales proveen autoridad existente suficiente para una fase de testnet cerrada y controlada. El trabajo legal requerido antes de operar a escala completa es sustancialmente mayor, pero puede realizarse en paralelo con el desarrollo de capacidades técnicas.

---

## 5. Modelos Operativos para la Emisión (No Técnico)

La elección del modelo operativo es una de las decisiones más trascendentales que enfrenta un banco central al diseñar un programa de emisión tokenizada. Determina quién construye y opera la infraestructura, quién asume qué riesgo, quién tiene acceso a la capa de liquidación y cómo evoluciona el sistema con el tiempo. Esta sección presenta tres modelos y analiza las ventajas y desventajas de cada uno.

**Modelo A — Liderado por el Banco Central**

En este modelo, el banco central diseña, construye y opera la infraestructura central de liquidación tokenizada. Los bancos comerciales y otros participantes regulados interactúan con la plataforma a través de APIs estandarizadas, pero no controlan la infraestructura subyacente.

*Ventajas:* Máximo control sobre la transmisión de política monetaria. Visibilidad plena de los flujos de liquidación. Sin dependencia de proveedores de infraestructura de terceros. Mayor alineación entre el diseño de la emisión y los objetivos de política monetaria. Arquitectura más creíble desde el punto de vista del riesgo sistémico.

*Desventajas:* Máximo requerimiento de capacidad institucional — los bancos centrales de muchas jurisdicciones de ALC no tienen actualmente los equipos tecnológicos ni la experiencia operativa para construir y operar infraestructura distribuida compleja. Largos plazos de desarrollo. Riesgo de construir sistemas cerrados y propietarios que son difíciles de interoperar con redes regionales o globales. Alto costo inicial.

*Más adecuado para:* Jurisdicciones del Grupo 1 (Brasil, México, Colombia) con fuerte capacidad tecnológica y un interés estratégico en ser anclas de infraestructura regional.

**Modelo B — Híbrido**

En este modelo, el banco central define el marco legal y de política, provee el activo de liquidación (tCeBM) y mantiene la supervisión regulatoria, pero delega el diseño, construcción y operación de la infraestructura técnica en una entidad regulada o consorcio.

*Ventajas:* Permite al banco central enfocarse en su mandato principal (política monetaria y estabilidad sistémica) mientras aprovecha la experiencia tecnológica del sector privado o del consorcio. Más rápido de desplegar que un sistema construido completamente por el banco central. Puede incorporar innovación comercial más rápidamente. Distribuye los costos operativos entre los participantes.

*Desventajas:* El banco central depende del operador regulado para la disponibilidad y seguridad de la plataforma. La gobernanza de la entidad operadora requiere un diseño cuidadoso para evitar conflictos de interés. La relación legal entre el banco central y el operador debe establecer claramente la responsabilidad por fallas del sistema.

*Más adecuado para:* La mayoría de las jurisdicciones de ALC en los Grupos 1 y 2. El modelo CBWeb3 —con LNET como operador de infraestructura e IDB Lab como financiador— es una variante de este enfoque.

**Modelo C — Totalmente Intermediado**

En este modelo, el banco central emite reservas a los bancos comerciales en forma tokenizada pero no opera ninguna plataforma mayorista o minorista directamente. Los bancos comerciales y los proveedores de servicios de pago con licencia construyen la infraestructura de atención al cliente y de liquidación interbancaria sobre la capa de reservas tokenizadas.

*Ventajas:* Carga operativa mínima para el banco central. Máximo aprovechamiento de la infraestructura de pagos del sector privado y la innovación comercial. Puede desplegarse de manera incremental. Evita el riesgo de que el banco central se convierta en un punto único de falla para el sistema de pagos.

*Desventajas:* La interoperabilidad entre las implementaciones de diferentes instituciones no es automática; requiere una sólida gobernanza y enforcement de estándares. El banco central tiene menos visibilidad directa de los flujos de liquidación. Riesgo de fragmentación si los participantes construyen sistemas incompatibles.

*Más adecuado para:* Jurisdicciones donde el banco central tiene un fuerte mandato regulatorio y supervisorio pero apetito limitado para operar infraestructura, y donde el sector bancario comercial tiene suficiente capacidad técnica para construir implementaciones interoperables.

---

## 6. Un Blueprint Mínimo Viable para la Emisión Tokenizada

La barrera más común para la acción no es la incertidumbre legal ni la madurez tecnológica — es la escala y complejidad percibida del emprendimiento. Esta sección propone un enfoque mínimo viable: un punto de partida deliberadamente acotado y de bajo riesgo que permite a las instituciones comenzar a aprender y desarrollar capacidades sin comprometerse con una transformación a gran escala.

**El principio del grupo cerrado de usuarios.** El punto de partida recomendado es un entorno cerrado y con permisos, con un conjunto pequeño y predefinido de participantes regulados. Esto no es un piloto en sentido peyorativo — es un enfoque estructuralmente sólido para desplegar infraestructura de liquidación, análogo a cómo los sistemas de liquidación bruta en tiempo real (LBTR) inicialmente se restringieron a transacciones interbancarias de alto valor antes de expandirse a casos de uso más amplios.

**Criterios de elegibilidad para el grupo cerrado.** La participación en el grupo cerrado inicial debe condicionarse al cumplimiento de un conjunto definido de criterios. Las instituciones deben:

1. *Estar reguladas por una autoridad competente* en la jurisdicción.
2. *Ser técnicamente capaces de integración por API* — poder conectarse a la capa API estandarizada de la plataforma.
3. *Estar operacionalmente comprometidas* — capaces de designar un contacto técnico y legal nominado y participar en las sesiones de prueba.
4. *Haber recibido autorización legal* — habiendo obtenido una opinión legal formal que confirme que su participación es consistente con la legislación aplicable.
5. *Estar vinculadas contractualmente* — habiendo firmado un acuerdo de participación que establece los términos de acceso, asignación de responsabilidades y gobernanza de datos.

**Caso de uso inicial recomendado.** El primer caso de uso debe ser lo más simple posible: liquidación interbancaria en moneda doméstica entre dos o más participantes del grupo cerrado, utilizando ISO 20022.

**Criterios de éxito.** La fase del grupo cerrado debe considerarse completa cuando: (a) al menos dos instituciones hayan completado la liquidación extremo a extremo con finalidad criptográfica; (b) el banco central haya validado el libro mayor de liquidación; y (c) se haya completado una evaluación post-piloto compartida con todos los participantes.

**Cronograma.** Un cronograma realista desde la decisión hasta la finalización exitosa de la fase del grupo cerrado es de seis a doce meses, dependiendo del tiempo de preparación legal y la complejidad de integración tecnológica.

---

## 7. De la Emisión Doméstica a la Utilidad Real

Un banco central que haya completado con éxito un piloto de emisión doméstica ha logrado algo significativo — pero aún no ha desbloqueado la fuente más importante de valor del dinero tokenizado. El potencial transformador emerge cuando el tCeBM se vuelve interoperable: cuando puede utilizarse para liquidar transacciones transfronterizas, cuando la liquidez puede fluir entre jurisdicciones sin múltiples rondas de conversión de divisas, y cuando las instituciones financieras de diferentes países pueden transaccionar entre sí utilizando un protocolo de liquidación compartido.

La plataforma CBWeb3 está diseñada para apoyar precisamente esta transición. Su arquitectura hub-and-spoke permite que el banco central de cada país mantenga plena soberanía sobre su emisión y liquidación doméstica mientras se conecta a un hub transnacional compartido para transacciones transfronterizas.

---

## 8. Modelos de Liquidación Transfronteriza para ALC

Existen tres modelos prácticos para la liquidación transfronteriza tokenizada.

**Modelo 1 — Corresponsalía Bancaria Tokenizada (PvP Mejorado)**

La relación bilateral entre un banco doméstico y su banco corresponsal se replica en infraestructura tokenizada, con liquidación atómica mediante HTLC. El Escenario A de CBWeb3 implementa exactamente este modelo y ha sido probado de extremo a extremo.

*Ventajas:* Construye sobre relaciones existentes. Sin necesidad de infraestructura regional compartida ni liquidez centralizada. Despliegue bilateral, corredor por corredor.

*Desventajas:* Escala linealmente. No resuelve el pre-fondeo de liquidez. Limitado a pares de monedas con corresponsalía existente.

**Modelo 2 — Corredores Bilaterales con Liquidación Compartida**

Dos bancos centrales establecen un acuerdo de liquidación bilateral directo, omitiendo completamente la capa de corresponsalía comercial.

*Ventajas:* Elimina la intermediación comercial. Visibilidad directa de los flujos transfronterizos. Base más sólida para la coordinación de política monetaria.

*Desventajas:* Requiere tratado o MOU entre bancos centrales. Proceso prolongado y políticamente complejo.

**Modelo 3 — Plataforma Regional Compartida**

Los países participantes se conectan a un único hub de liquidación multilateral con market-making automatizado. El AMM (Escenario B) de CBWeb3 implementa este modelo.

*Ventajas:* Máxima escalabilidad. Uso más eficiente de la liquidez. Nuevos participantes se incorporan con conectividad inmediata con todos los miembros.

*Desventajas:* Mayor requerimiento de coordinación multilateral. Riesgo sistémico concentrado en el hub.

**Coexistencia en la práctica.** Estos tres modelos no son mutuamente excluyentes y la arquitectura regional probablemente verá los tres en uso simultáneo.

---

## 9. Gestión de Liquidez para Monedas No Convertibles

El desafío de gestión de liquidez en un sistema de liquidación tokenizado multi-moneda es agudo para ALC, donde muchas monedas no son libremente convertibles.

**Mecanismo 1 — Saldos Bilaterales Pre-fondados** (Escenario A / HTLC): Simple y de bajo riesgo, pero inmoviliza capital significativo.

**Mecanismo 2 — Líneas de Crédito Bilaterales**: Reduce el pre-fondeo pero introduce riesgo crediticio y complejidad institucional.

**Mecanismo 3 — Liquidez Agrupada con AMM** (Escenario B): Más eficiente en capital, pero los tipos de cambio algorítmicos pueden desviarse de precios de mercado para pares poco líquidos.

**Enfoque recomendado.** Para la fase inicial, se recomiendan los saldos bilaterales pre-fondados. La transición al AMM resulta atractiva a medida que la red crece, pero solo después de que el marco de gobernanza esté establecido y el algoritmo validado.

---

## 10. Consideraciones de Política y Enfoques de Gobernanza

**Soberanía monetaria.** La arquitectura CBWeb3 mantiene la red spoke de cada país como completamente soberana — el banco central controla sus propios parámetros de emisión y puede desconectarse del hub regional en cualquier momento.

**Estabilidad financiera.** La mayoría de las jurisdicciones de ALC están enfocadas en aplicaciones mayoristas, lo que limita sustancialmente el riesgo de desintermediación bancaria.

**Privacidad.** La plataforma CBWeb3 utiliza el marco de privacidad Hyperledger Paladin con pruebas de conocimiento cero Zeto para habilitar la confidencialidad de las transacciones, equilibrando privacidad y visibilidad AML/CFT.

**Gobernanza del Grupo de Trabajo CBWeb3.** El grupo opera bajo el marco de Linux Foundation Decentralized Trust, con co-presidentes nominados por CEMLA y FLAR. Las decisiones formales sin consenso se resuelven por GitVote (50%+1, siete días de comentarios).

---

## 11. Reflexiones Finales: Un Camino Gradual y Coordinado hacia Adelante

Los bancos centrales y las autoridades financieras de ALC tienen tanto la motivación como los medios para actuar sobre el dinero digital de banco central. El camino hacia adelante no requiere que todos los países avancen al mismo ritmo, ni requiere plena certeza legal antes de dar el primer paso.

**Hoja de Ruta de Acción Resumida**

*Fase 1 — Preparación Legal e Institucional (Meses 1–3)*

- Realizar evaluación legal interna con la lista de verificación de cinco dominios (Sección 4)
- Designar un líder de proyecto interno y establecer un grupo de trabajo multifuncional
- Iniciar contacto con el Grupo de Trabajo CBWeb3

*Fase 2 — Involucramiento Técnico y Conexión al Testnet (Meses 3–6)*

- Completar integración técnica con el testnet CBWeb3
- Ejecutar el tutorial de inicio rápido PvP de 15 minutos
- Definir el grupo cerrado de usuarios inicial (criterios en Sección 6)

*Fase 3 — Piloto de Grupo Cerrado (Meses 6–12)*

- Ejecutar el primer conjunto de transacciones de liquidación interbancaria doméstica
- Completar la evaluación post-piloto y compartir hallazgos con el Grupo de Trabajo
- Iniciar preparación legal para uso transfronterizo

*Fase 4 — Conectividad Transfronteriza (Meses 12–24)*

- Seleccionar el modelo de liquidación transfronteriza apropiado (Sección 8)
- Establecer conectividad a través del hub CBWeb3
- Participar en el marco de gobernanza regional

Los bancos centrales que comiencen este proceso ahora estarán mejor posicionados para dar forma a la arquitectura regional. La iniciativa CBWeb3 es una invitación a participar en la construcción de infraestructura compartida para un futuro financiero regional que sirva a las prioridades de América Latina y el Caribe.

---

*Fin del documento — Knowledge Product 1, Draft v0.1*
*Prepared by CBWeb3 Working Group / LNET — 2026-05-19*
*For review and comment: open a thread on GitHub issue #46*
