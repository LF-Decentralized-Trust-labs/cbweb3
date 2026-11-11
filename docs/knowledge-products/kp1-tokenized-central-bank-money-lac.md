# Knowledge Product 1 — Draft v0.3
## Tokenized Central Bank Money in LAC: From Issuance to Cross-Border Integration
## Dinero Digital de Banco Central en ALC: De la Emisión a la Integración Transfronteriza

**Status:** Draft for working group review
**Version:** v0.3 — 2026-11-06
**Author:** CBWeb3 Working Group / LNET
**GitHub issue:** [#46 — Knowledge Product 1 Proposed Structure](https://github.com/LF-Decentralized-Trust-labs/cbweb3/issues/46)
**Audience:** Central bankers, policymakers, and broad financial sector stakeholders

---

> **Changelog v0.2 → v0.3 (2026-11-06)**
>
> Incorporating evidence and participant feedback from the 2026 CBWeb3 sandbox/UAT:
>
> - **§6** — Replaced generic completion criteria with evidence-based sandbox exit criteria; separated technical atomicity from legal finality; removed the unsupported six-to-twelve-month commitment
> - **§8** — Corrected Scenario A and Scenario B maturity; Scenario B is an experimental prototype, not a production-ready regional platform; made the CSD/Nuam model a candidate for evaluation rather than an adopted corridor
> - **§11** — Reworked the roadmap around evidence gates, institutional readiness, failure testing, production controls, economic viability and explicit go/conditional-go/no-go decisions
> - **§12** — Added observed operational risks from the 2026 sandbox, including silent pending states, false-positive health signals, reconciliation risk, onboarding friction and missing production controls
> - **NEW §13** — Added “Evidence and lessons from the 2026 sandbox/UAT”, clearly separating observed results, limitations and unresolved policy/design questions
> - **Throughout** — Distinguished sandbox evidence, technical ledger behavior, legal settlement finality, UAT participation and production readiness

---

> **Changelog v0.1 → v0.2 (2026-07-10)**
>
> Incorporating feedback from GitHub comments on issue #46 (JohnVillaDVL, santim9393-maker):
>
> - **§1** — Added a 4th decision dimension: the settlement infrastructure question (CB-operated vs. banks vs. authorized FMIs/CSDs)
> - **§2** — Added capital markets settlement opportunity paragraph; updated Developing capacity tier to note Nuam regional integration across Colombia, Chile, and Peru
> - **§3** — Added a 3rd instrument category: authorized FMI-operated tokenized central bank account balances
> - **§4, Domain 3** — Clarified that CSDs, clearinghouses, and other authorized FMIs may be eligible participants or synchronization nodes
> - **§5** — Added CSD/FMI-operated hybrid variant to Model B; deepened pros/cons for all models with explicit risks and requirements
> - **§6** — Clarified that "Blueprint" refers to a structured starting point, not a formal service design document; expanded eligible participant criteria to include authorized FMIs; added institutional settlement use case as alternative to domestic interbank use case
> - **§8** — Added explicit definitions of Scenario A and Scenario B; added Model 4 (Regional CSD Interconnection for Tokenized Cash) with full pros/cons
> - **§9** — Fixed duplicated Mechanism 2 text; added note on CSDs as regional liquidity infrastructure
> - **§10** — Added new subsection on the institutional role of CSDs in tokenized cash infrastructure
> - **§11** — Expanded roadmap with formal go/no-go decision gate, threat modeling, legal validation, operational resilience testing, economic model design, and benefit measurement framework
> - **NEW §12** — Risk Register covering 12 risk categories
> - **NEW Appendix A** — Glossary of key terms in English and Spanish
> - **Throughout** — Acronyms defined on first use; `[Source: TBD]` markers added to factual/quantitative claims pending citation

---

> **Editorial note (v0.1):** This draft incorporated all five feedback points raised by Yuri Carrillo (comment, April 17, 2026) and accepted by Carolina Velasquez (April 21, 2026):
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

Latin America and the Caribbean (LAC) stand at a pivotal juncture in the evolution of money. Cross-border payments in the region remain among the most expensive and slowest in the world. Correspondent banking relationships are thinning. Stablecoins and foreign digital currencies are gaining ground in dollarized and semi-dollarized economies, eroding the policy space available to central banks. Against this backdrop, tokenized central bank money has emerged not as a speculative technology experiment, but as a concrete policy instrument — one that several regional authorities are already exploring in earnest.

This document is addressed to the decision-makers who must determine whether and how to engage with this transformation. It is organized around the practical questions that senior policymakers face: What exactly is being issued? Is our legal framework ready? What operating model fits our institutional context? Where do we start, and how do we connect to the region?

The decision framework this document proposes has four dimensions. First, the **settlement asset question**: whether to issue tokenized central bank money (tCeBM) at the wholesale level, the retail level, or both, and how to position it relative to tokenized commercial bank deposits and private digital instruments. Second, the **operating model question**: whether the central bank leads the infrastructure, delegates to intermediaries, or adopts a hybrid arrangement, and what each choice implies for governance, costs, and systemic risk. Third, the **cross-border integration question**: whether to pursue bilateral corridors, join a shared regional platform, or modernize correspondent banking relationships — and how to sequence these choices over time. Fourth, the **settlement infrastructure question**: whether tCeBM will be operated exclusively by the central bank, by commercial banks, or through authorized financial market infrastructures (FMIs) such as central securities depositories (CSDs), clearinghouses, or settlement system operators. This fourth dimension determines who builds and runs the operational layer, who is accountable for settlement failures, and how tCeBM connects to existing market processes such as securities clearing, reconciliation, and custody.

The practical conclusion is that authorities can use controlled, no-value experimentation to determine whether tokenization adds value in a defined use case. Experimentation does not substitute for legal authority, production controls or comparison with non-tokenized alternatives. The CBWeb3 regional testnet provides reference infrastructure and interoperability tooling for that learning phase; any expansion beyond it remains conditional on the evidence gates in Sections 6 and 11.

---

## 2. Why Tokenized Central Bank Money Matters for LAC Now

The case for tCeBM in Latin America and the Caribbean is not built on technology enthusiasm. It is built on a specific set of structural pressures that are already reshaping the regional financial landscape, and that demand a policy response.

**The correspondent banking problem.** The number of active correspondent banking relationships in LAC has declined steadily over the past decade, driven by de-risking strategies among global banks [Source: TBD — BIS correspondent banking surveys]. For smaller economies and for specific transaction categories — particularly remittances and trade finance — this contraction has raised costs, lengthened settlement times, and in some corridors, effectively eliminated access to affordable international payment infrastructure. Tokenized central bank money may reduce dependence on some correspondent intermediaries by enabling coordinated PvP state transitions; whether this produces final legal discharge and net economic benefit must be established for each corridor.

**The stablecoin and dollarization pressure.** Across the region, stablecoin adoption is growing — particularly in economies with histories of currency instability or restricted access to US dollar accounts. While this adoption reflects genuine demand for more stable and accessible digital value, it also represents a potential transfer of monetary sovereignty. If a significant share of domestic transactions migrates to instruments denominated in foreign currencies and issued by private entities outside regulatory reach, the ability of central banks to implement monetary policy, observe systemic risk, and enforce anti-money laundering and counter-terrorist financing (AML/CFT) rules is materially diminished. Tokenized central bank money offers a sovereign alternative: digital, programmable, interoperable — but issued and governed by the monetary authority.

**The cross-border payment inefficiency.** Sending money across LAC borders is still largely slow, opaque, and expensive. Average remittance costs in the region remain above the G20 target of 3 percent [Source: TBD — World Bank Remittance Prices Worldwide]. For businesses, the inability to settle in local currencies forces reliance on dollar intermediation, adding FX costs and settlement risk. Tokenized cross-border settlement — whether through bilateral corridors or shared platforms — can compress settlement cycles from days to seconds and eliminate layers of intermediation that serve no function other than friction.

**The capital markets settlement opportunity.** Beyond retail and correspondent banking use cases, tokenized central bank money can address a structural inefficiency in institutional markets. In LAC, the coordination between payment systems, settlement banks, clearinghouses, custodians, and CSDs generates costs, waiting times, reconciliation needs, and operational risks in the post-trade cycle of capital markets transactions. tCeBM, as a common settlement layer accessible to authorized FMIs, could serve as the foundation for more efficient institutional settlement — not only for cross-border payments, but for securities transactions, derivatives clearing, and other capital market operations. This use case is particularly relevant for markets where CSDs are already integrated with central bank account balances and where post-trade processes generate significant operational costs.

**A differentiated regional landscape.** LAC is not a monolithic region, and any honest assessment of the opportunity must acknowledge the heterogeneity in country context, institutional capacity, and adoption readiness. Three broad tiers characterize the current landscape:

- *Advanced jurisdictions* — countries such as Brazil, Mexico, and Colombia that have well-developed regulatory frameworks, sophisticated central bank technology capacity, and active central bank digital currency (CBDC) or tokenization research programs. These countries are positioned to be early movers and regional anchors.

- *Developing capacity jurisdictions* — countries such as Peru, Chile, Ecuador, Costa Rica, and the Dominican Republic, where regulatory frameworks are solid but CBDC-specific legislation is still emerging and central bank technology teams are building capabilities. These countries benefit most from regional infrastructure and knowledge sharing. Notably, Colombia, Chile, and Peru also share an active regional capital markets integration process through Nuam, which integrates the stock exchanges of the three countries and is advancing toward common post-trade infrastructure including CSDs and central counterparty clearinghouses (CCPs). This regional integration provides a concrete institutional foundation for tCeBM pilots with cross-jurisdictional scope.

- *Nascent engagement jurisdictions* — smaller or less financially integrated economies where CBDC is not yet a policy priority but where tokenized payment infrastructure could address acute access and cost problems. For these countries, a plug-and-play approach — joining existing regional infrastructure rather than building from scratch — is the pragmatically viable path.

The CBWeb3 initiative is designed to serve all three tiers, with the regional testnet providing a shared experimentation environment and the governance framework ensuring that no single country's design choices are imposed on others.

---

## 3. What Is Being Issued? Clarifying the Forms of Digital Money

Policy discussions about tokenization are frequently imprecise about what is actually being issued. This imprecision creates confusion in legal analysis, in public communication, and in the design of interoperability standards. This section establishes the key distinctions.

**Tokenized central bank money (tCeBM)** is a digital representation of a liability of the central bank, issued on a distributed ledger (DLT) or programmable infrastructure. It carries the same credit quality and settlement finality as reserves held in traditional central bank accounts. Because the issuer is the monetary authority, tCeBM does not introduce credit risk or liquidity risk beyond what already exists in the monetary system. At the wholesale level, tCeBM functions as the ultimate settlement asset — the equivalent of a reserve balance, but programmable and capable of atomic settlement with other tokenized assets. At the retail level, it functions as a digital form of cash issued directly to the public, though most LAC jurisdictions are currently focused on wholesale applications.

**Tokenized commercial bank deposits (tDeposits)** are digital representations of claims on a commercial bank, issued on programmable infrastructure. They carry the credit risk of the issuing institution and are subject to deposit insurance limits. In a two-tier model — which most LAC jurisdictions favor — commercial banks issue tDeposits backed by reserves held at the central bank, while the central bank provides settlement infrastructure using tCeBM. The distinction matters for system design: interoperability between tDeposits issued by different banks requires a common settlement layer, and that layer is most efficiently provided by tCeBM.

**Authorized FMI-operated tokenized central bank account balances** refers to a digital representation of balances held by an authorized FMI — such as a CSD, clearinghouse, or settlement system operator — in accounts at the central bank, where the central bank retains the monetary liability but the FMI manages the operational layer. This category is distinct from direct tCeBM issuance: the FMI acts as an authorized intermediary or synchronization node rather than an independent issuer. Legal responsibility for the monetary liability remains with the central bank; operational responsibility for the representation and reconciliation layer rests with the authorized FMI. This model is relevant in markets where CSDs already manage central bank account balances as part of securities settlement and where extending this role to programmable infrastructure is a natural evolution rather than a structural break.

**Stablecoins and other private digital instruments** are tokens issued by private entities, typically backed by reserves of fiat currency, government securities, or other assets. Their value proposition is stability relative to a reference currency, combined with programmability and accessibility. However, they are not central bank liabilities, and their governance, reserve management, and regulatory treatment vary widely. Fiat-backed stablecoins denominated in US dollars introduce foreign currency denomination risk at scale if adopted broadly in LAC economies; algorithmic stablecoins introduce additional stability risks. From a policy perspective, private digital instruments are complements or potential competitors to tCeBM — not substitutes for it.

The CBWeb3 platform focuses on tCeBM issuance and wholesale settlement. The smart contracts governing issuance, Hash Time-Lock Contract (HTLC)-based cross-border settlement, and automated market-making (AMM)-based liquidity management are all designed around the central bank as the primary issuer and tCeBM as the settlement asset of record.

---

## 4. Legal and Regulatory Readiness: The Critical Path

Before embarking on a tokenized issuance program, authorities need to assess whether their existing legal and regulatory frameworks are fit for purpose — or whether legislative or regulatory changes are required. The following checklist structures this assessment across five domains.

**Domain 1 — Legal definition of central bank money.** Does existing law permit the central bank to issue money in digital or tokenized form? In some jurisdictions, central bank legislation references "banknotes and coins" in ways that could be interpreted as excluding digital equivalents. Clarification by legal opinion or regulatory guidance may be sufficient; in other cases, a legislative amendment is required.

**Domain 2 — Settlement finality.** Does the legal system recognize the finality of settlement in a distributed ledger environment? Settlement finality — the point at which a transfer is irrevocable — is foundational for central bank settlement infrastructure. Some jurisdictions' financial market infrastructure laws explicitly address finality; others rely on common law principles or general contract law, which may create uncertainty in the context of smart contract execution.

**Domain 3 — Access rules.** Who may hold or transact in tCeBM? At the wholesale level, access is typically restricted to regulated financial institutions — commercial banks, payment service providers, and potentially securities settlement entities. Jurisdictions should explicitly consider whether CSDs, clearinghouses, and other authorized FMIs may participate as direct account holders, technical operators, or authorized synchronization nodes. The legal basis for defining and enforcing access rules at each of these participation tiers, and for excluding non-regulated entities, must be clearly established.

**Domain 4 — AML/CFT and FX regulations.** How do existing anti-money laundering (AML), counter-terrorist financing (CFT), and foreign exchange regulations apply to tCeBM transactions? In a programmable environment, compliance obligations can be embedded in smart contract logic, but the legal recognition of this approach must be verified. Cross-border transactions in tCeBM may trigger FX reporting requirements or capital flow management measures that need to be coordinated with central bank issuance policy. Where multiple jurisdictions are involved, gaps in AML/CFT screening standards between participating countries represent a specific regulatory arbitrage risk.

**Domain 5 — Data protection and cybersecurity.** Does the privacy and data protection framework apply to transaction data recorded on a distributed ledger? If the ledger is permissioned and access-controlled, the central bank and participating institutions are data controllers and must comply with applicable data protection obligations — including, where relevant, Brazil's General Data Protection Law (LGPD) or equivalent national frameworks. Cybersecurity requirements for systemically important financial infrastructure must also be mapped to the technical architecture of the platform.

**Current status across LAC jurisdictions.** The following is a high-level summary of regulatory readiness by country tier, based on publicly available information as of 2025–2026 [Source: TBD — national central bank publications and regulatory frameworks]:

- *Brazil:* The Banco Central do Brasil has a strong legal basis for digital currency issuance under the LIFT Lab framework and the Drex program. Settlement finality, AML/CFT, and data protection frameworks (LGPD) are well-developed. Brazil is the most legally advanced jurisdiction in the region for CBDC purposes.
- *Mexico:* Banxico operates under a mandate that permits electronic money issuance, and the Fintech Law (2018) established a regulatory framework for digital assets. CBDC-specific legislation is under development but not yet enacted.
- *Colombia:* The Banco de la República has authority to issue digital money under its organic statute, though CBDC-specific regulations have not yet been issued. AML/CFT frameworks are robust.
- *Peru:* Banco Central de Reserva del Perú (BCRP) has conducted exploratory work; legal framework is generally permissive but lacks CBDC-specific provisions. The Superintendencia de Banca, Seguros y AFP (SBS), Peru's financial supervisor, would need to coordinate on access rules for institutions it supervises.
- *Chile:* Banco Central de Chile published a CBDC feasibility study in 2022. Legal authority for digital issuance is considered available under current statute, but implementing regulations are pending.
- *Costa Rica, Dominican Republic, Ecuador:* Active interest and exploratory research, but no formal legal framework adaptations underway. These countries represent strong candidates for regional infrastructure adoption rather than independent platform development.
- *Smaller and Eastern Caribbean economies:* The Eastern Caribbean Central Bank (ECCB)'s DCash initiative provides a regional precedent [Source: TBD — ECCB DCash documentation]. Legal frameworks vary by jurisdiction; the regional central bank model simplifies some coordination challenges.

Authorities in jurisdictions without CBDC-specific frameworks should not interpret this gap as a barrier to participation in regional experimentation. Most legal systems provide sufficient existing authority for a closed, controlled testnet phase. The legal work required before going live at scale is substantially greater, but can be pursued in parallel with technical capacity building.

---

## 5. Operating Models for Issuance (Non-Technical)

The choice of operating model is one of the most consequential decisions a central bank faces in designing a tokenized issuance program. It determines who builds and operates the infrastructure, who holds what risk, who has access to the settlement layer, and how the system evolves over time. This section presents three models and analyzes the advantages and disadvantages of each.

**Model A — Central Bank-Led**

In this model, the central bank designs, builds, and operates the core tokenized settlement infrastructure. Commercial banks and other regulated participants interact with the platform through standardized APIs but do not control the underlying infrastructure. The central bank maintains full custody of the issuance logic, the settlement ledger, and the governance rules.

*Advantages:* Maximum control over monetary policy transmission. Full visibility into settlement flows. No dependency on third-party infrastructure providers. Strongest alignment between issuance design and monetary policy objectives. Most credible architecture from a systemic risk standpoint. Full accountability chain in the event of settlement errors or system disruptions.

*Disadvantages:* Highest institutional capacity requirement — central banks in many LAC jurisdictions do not currently have the technology teams or operational experience to build and run complex distributed infrastructure. Long development timelines. Risk of building closed, proprietary systems that are difficult to interoperate with regional or global networks. High upfront cost. The central bank becomes the sole responsible party for all operational failures, which may not be appropriate for infrastructure that serves diverse market participants.

*Requirements:* Mature internal technology team with distributed ledger and cryptographic engineering capacity; dedicated operational team for 24/7 infrastructure management; comprehensive incident response and business continuity plan.

*Best suited for:* Tier 1 jurisdictions (Brazil, Mexico, Colombia) with strong technology capacity and a strategic interest in being regional infrastructure anchors.

**Model B — Hybrid**

In this model, the central bank defines the legal and policy framework, provides the settlement asset (tCeBM), and maintains supervisory oversight, but delegates the design, construction, and operation of the technical infrastructure to a regulated entity or consortium. This entity — which may be a central bank-owned company, a financial sector utility, a regulated technology provider, or an authorized FMI such as a CSD or clearinghouse — operates the platform under central bank license and supervision.

A notable variant of the hybrid model — particularly relevant for jurisdictions with mature capital markets infrastructure — involves delegating the operational layer to a CSD or other authorized FMI rather than a technology provider or commercial bank. In this variant, the central bank retains the monetary liability and issuance authority, while the CSD operates the technical layer, manages participant access, handles reconciliation, and provides connectivity to existing securities settlement workflows. This approach leverages established operational relationships between the CSD and market participants (commercial banks, custodians, settlement agents, issuers) and avoids the need to build parallel connectivity from scratch.

*Advantages:* Allows the central bank to focus on its core mandate (monetary policy and systemic stability) while leveraging private sector or consortium technology expertise. Faster to deploy than a fully central bank-built system. Can incorporate commercial innovation — smart contract features, user interfaces, integration tooling — more rapidly. Spreads operational costs across participants. In the CSD-operated variant, builds on existing participant relationships and post-trade integration points.

*Disadvantages:* The central bank is dependent on the regulated operator for platform availability and security. Governance of the operator entity requires careful design to avoid conflicts of interest. The legal relationship between the central bank and the operator must clearly establish liability for system failures, error handling procedures, participant disconnection protocols, and escalation paths. Risk of mission drift if commercial considerations influence platform design in ways that compromise monetary policy objectives. In the CSD-operated variant: CSD governance structures may not be designed for central bank monetary operations, creating potential accountability conflicts; requires clear agreements on applicable law, reconciliation with central bank accounts, and supervision responsibilities.

*Requirements:* Formal legal agreement between central bank and operator establishing the scope of delegation, liability allocation, supervisory access rights, and exit conditions; minimum operational standards defined by the central bank; clear crisis management protocols between the central bank and the operator.

*Best suited for:* Most LAC jurisdictions in Tiers 1 and 2, where central banks want to move faster than their internal technology capacity allows while maintaining legal and policy control. The CBWeb3 model — with LNET as infrastructure operator and IDB Lab as funder — is a variant of this approach.

**Model C — Fully Intermediated**

In this model, the central bank issues reserves to commercial banks in tokenized form but does not operate any retail or wholesale platform directly. Commercial banks and licensed payment service providers build the customer-facing and interbank settlement infrastructure on top of the tokenized reserve layer. The central bank's role is limited to issuance, oversight, and setting interoperability standards.

*Advantages:* Minimal operational burden on the central bank. Maximum leverage of existing private sector payment infrastructure and commercial innovation. Can be deployed incrementally, with each participating institution building its own stack. Avoids the risk of the central bank becoming a single point of failure for the payment system.

*Disadvantages:* Interoperability between different institutions' implementations is not automatic — it requires strong standards governance and enforcement. The central bank has less direct visibility into settlement flows, complicating AML/CFT oversight and systemic risk monitoring. Risk of fragmentation if participants build incompatible systems. Harder to ensure systemic resilience if the infrastructure is fragmented across many private operators. The central bank loses the ability to set consistent access rules and pricing for the settlement layer.

*Requirements:* Comprehensive interoperability standards with binding enforcement power; mandatory API conformance testing; strong supervisory capacity to monitor fragmented implementations; clear liability framework in the event of cross-institution settlement failures.

*Best suited for:* Jurisdictions where the central bank has a strong regulatory and supervisory mandate but limited appetite to operate infrastructure, and where the commercial banking sector has sufficient technical capacity to build interoperable implementations.

**Choosing a model.** In practice, most LAC jurisdictions will adopt a hybrid approach, with the balance between central bank control and private sector delegation varying by institutional context. The key principle is that the choice of operating model should be made deliberately and documented formally — because it will be very difficult to change once infrastructure is built and participants have invested in integration.

---

## 6. A Minimum Viable Implementation Path for Tokenized Issuance

The most common barrier to action is not legal uncertainty or technology readiness — it is the perceived scale and complexity of the undertaking. This section proposes a minimum viable approach: a deliberately scoped, low-risk starting point that allows institutions to begin learning and building capacity without committing to a full-scale transformation. The term "minimum viable" describes a structured operational starting point, not a formal service design blueprint — the goal is to define the narrowest possible scope that still generates real operational learning.

**The closed user group principle.** The recommended starting point is a closed, permissioned environment with a small, pre-defined set of regulated participants. This is not a pilot in the pejorative sense — it is a structurally sound approach to deploying settlement infrastructure, analogous to how real-time gross settlement (RTGS) systems were initially restricted to large-value interbank transactions before expanding to broader use cases.

**Eligibility criteria for the closed user group.** Participation in the initial closed group should be conditioned on meeting a defined set of criteria. Institutions should be:

1. *Regulated by a competent authority* in the jurisdiction — a licensed commercial bank, payment service provider, or securities settlement entity subject to prudential supervision and AML/CFT obligations.
2. *Technically capable of API integration* — able to connect to the platform's standardized API layer and complete a technical integration test with the central bank or platform operator.
3. *Operationally committed* — able to dedicate a named technical and legal contact, participate in testing sessions, and comply with incident reporting requirements.
4. *Legally cleared* — having received a formal legal opinion confirming that their participation in the closed group is consistent with applicable law and their institutional mandate.
5. *Contractually bound* — having signed a participation agreement with the central bank or platform operator that establishes the terms of access, liability allocation, and data governance.

In contexts where the initial use case involves securities settlement or institutional markets, CSDs, clearinghouses, and their connected settlement banks and custodians should also be considered eligible participants, provided they meet the same regulatory and operational criteria and have received authorization from the competent supervisory authority.

Institutions that meet these criteria but are not selected in the first cohort due to capacity constraints should be placed on a defined waitlist with clear timeline expectations.

**Recommended initial use cases.** Two use cases are appropriate for the initial closed group phase, depending on the institutional context of each jurisdiction:

The first is **domestic interbank settlement**: interbank settlement in domestic currency between two or more participants in the closed group, using a predefined payment message standard (ISO 20022 is recommended). This use case generates immediate operational learning — about latency, settlement finality confirmation, error handling, and reconciliation — without introducing foreign currency complexity, retail access considerations, or cross-border coordination requirements.

The second, particularly suited for jurisdictions with active capital markets infrastructure, is **institutional settlement via authorized FMI**: settlement of tokenized cash in an institutional process administered by an authorized FMI such as a CSD. The goal is not to extend access to retail users but to validate whether a tokenized representation of central bank account balances can be operationally managed by an authorized infrastructure with defined controls for access, reconciliation, traceability, and settlement finality. This use case is directly relevant to the operational needs of institutions such as Nuam, whose use case involves receiving cash from one jurisdiction to settle securities transactions in another.

**Sandbox exit criteria.** A closed-group sandbox should not be considered complete merely because its happy path succeeds. Exit requires: (a) at least two institutions completing the defined end-to-end technical flow in both intended directions; (b) reconciliation of reserve debits, token issuance, pending operations and final balances; (c) exercised timeout, refund, participant-disconnection and recovery paths; (d) evidence that functional health checks detect loss of peer communication and stuck confirmations; (e) resolution or explicit acceptance of material defects; and (f) a joint assessment documenting participant feedback, residual risks and the next decision gate. Atomic execution in the sandbox is technical evidence; it does not establish legal settlement finality.

**Timing and institutional effort.** The 2026 testing showed that elapsed time depends not only on software readiness but also on security review, VPN and virtual-machine provisioning, permissions, credentials, internal coordination and availability of counterpart institutions. This document therefore does not prescribe a generic six-to-twelve-month schedule. Each authority should estimate its timeline only after completing the readiness assessment and naming accountable legal, technology, cybersecurity, treasury and operations owners.

---

## 7. From Domestic Issuance to Real Utility

A central bank that has successfully completed a domestic issuance pilot has achieved something meaningful — but has not yet unlocked the most significant source of value in tokenized money. Domestic issuance alone replicates, in a new technological form, a payment infrastructure that already exists. The transformative potential of tokenized central bank money emerges when it becomes interoperable: when it can be used to settle cross-border transactions, when liquidity can flow between jurisdictions without multiple rounds of currency conversion, and when financial institutions in different countries can transact with each other using a shared settlement protocol.

This transition — from domestic issuance to regional utility — requires deliberate sequencing. It involves connecting domestic platforms to regional infrastructure, establishing legal and contractual frameworks for cross-border use, and resolving the liquidity management challenges that arise when non-convertible currencies are included in the settlement network.

The CBWeb3 platform is designed to support precisely this transition. Its hub-and-spoke architecture allows each country's central bank to maintain full sovereignty over its domestic issuance and settlement while connecting to a shared transnational hub for cross-border transactions. A country can join the regional network incrementally — beginning with bilateral connectivity to one counterpart jurisdiction — and expand connectivity over time as operational experience and legal frameworks mature.

The following two sections address the specific design choices involved in cross-border settlement and liquidity management.

---

## 8. Cross-Border Settlement Models for LAC

> **Maturity note on CBWeb3 testnet scenarios:** “Scenario A” is the HTLC-based PvP model implemented and exercised end to end in the sandbox, including a Costa Rica–Peru participant-operated flow. This demonstrates the technical workflow under test conditions, not legal finality or production readiness. “Scenario B” is an experimental AMM/liquidity-pool prototype whose happy path has been executed on a dedicated hub chain. It is not production-ready: privacy on the hub, signer concurrency, multi-validator operation, performance under stressed swaps, resilience and operational hardening remain open. The two scenarios must not be described as having the same maturity.

Four practical models exist for cross-border tokenized settlement, each with different implications for complexity, scalability, and the degree of regional coordination required.

**Model 1 — Tokenized Correspondent Banking (Enhanced PvP)**

In this model, the bilateral relationship between a domestic bank in Country A and its correspondent bank in Country B is replicated on tokenized infrastructure. Each tokenized leg is coordinated through an HTLC so that, within the tested technical model, both state transitions complete or the timeout/refund path applies. Scenario A implements and has exercised this workflow in the sandbox. Whether those ledger transitions discharge the underlying legal obligations with finality must be established separately in each jurisdiction and participation agreement.

*Advantages:* Builds on existing correspondent relationships and legal frameworks. No requirement for a shared regional infrastructure or centralized liquidity. Can be deployed bilaterally, one corridor at a time, without requiring multilateral coordination.

*Disadvantages:* Scales linearly with the number of corridors — N×(N-1)/2 bilateral agreements for full regional connectivity. Does not solve the liquidity pre-funding problem: each correspondent still needs to hold pre-funded balances in both currencies. Limited to currency pairs where a bilateral correspondent relationship already exists or can be established.

*Requirements:* HTLC-capable smart contract infrastructure at both ends; legal recognition of HTLC-based finality in both jurisdictions; bilateral participation agreement between institutions.

**Model 2 — Bilateral Corridors with Shared Settlement**

In this model, two central banks establish a direct, bilateral settlement arrangement using tokenized central bank money, bypassing the commercial correspondent layer entirely. The arrangement is governed by a bilateral agreement between the two monetary authorities and uses a shared settlement infrastructure — potentially the CBWeb3 hub — as the technical intermediary.

*Advantages:* Eliminates commercial bank intermediation from the interbank settlement layer, reducing cost and risk. Provides central banks with direct visibility into cross-border settlement flows. Stronger foundation for monetary policy coordination between jurisdictions.

*Disadvantages:* Requires bilateral treaty or memorandum of understanding between central banks — a process that can be lengthy and politically complex. Still scales as a mesh of bilateral agreements, though more slowly than commercial bilateral corridors. Requires both central banks to have sufficient technology maturity to connect to shared infrastructure. Raises questions about applicable law in the event of disputes and about the treatment of pending obligations if one central bank disconnects.

*Requirements:* Formal bilateral legal framework between monetary authorities; definition of governing law for cross-border disputes; collateral or pre-funded balance requirements to manage settlement risk during connectivity interruptions.

**Model 3 — Shared Regional Platform**

In this model, participating countries would connect to a multilateral settlement hub supporting pooled liquidity and automated market-making for foreign-exchange conversion. CBWeb3 Scenario B provides an experimental technical prototype of this model; it does not yet provide an approved multilateral legal framework, production liquidity arrangement or production-grade operating service.

*Potential advantages to validate:* A multilateral topology could avoid N×(N-1)/2 bilateral technical integrations. Pooled liquidity may reduce pre-funding for some corridors if sufficient, appropriately priced liquidity is actually committed. Connectivity and liquidity access for a new participant would remain subject to onboarding, legal agreements, capacity and governance. These are hypotheses for measurement, not demonstrated production outcomes.

*Disadvantages:* Highest coordination requirement — establishing a multilateral governance framework, allocating costs, and defining liability arrangements across multiple sovereign jurisdictions is complex. Requires a trusted, neutral operator for the hub infrastructure. Systemic risk is concentrated at the hub; robust technical and governance resilience is essential. AMM-based exchange rates may deviate from market rates for illiquid currency pairs.

*Requirements:* Multilateral legal framework; neutral hub operator with appropriate governance; AMM parameter governance process; circuit breakers and liquidity thresholds for non-convertible currency pairs; hub redundancy and business continuity standards.

**Model 4 — Regional CSD Interconnection for Tokenized Cash**

In this model, the CSDs of participating countries act as authorized intermediary infrastructures that operate, synchronize, or facilitate movements of tCeBM backed by central bank account balances. Each central bank retains the issuance authority and monetary liability; the CSDs contribute regional connectivity, operational standards, participant management, reconciliation capacity, and integration with existing institutional market processes. This model is particularly relevant where CSDs are already involved in cross-border post-trade integration and where the primary use case involves institutional settlement of securities rather than general interbank payments.

*Advantages:* Leverages existing infrastructure and established relationships between CSDs and market participants. Reduces duplication of operational investment. Facilitates adoption because participants are already connected to their domestic CSD. Strengthens regional integration through institutions with track records in cross-jurisdictional coordination. Enables controlled, institutional pilots without retail access considerations. Directly supports capital markets post-trade efficiency as a concrete and immediate use case.

*Disadvantages:* Requires clear agreements on liability, supervision, cybersecurity, operational continuity, reconciliation with central bank accounts, applicable law, error handling, and participant disconnection procedures. CSD governance structures may not be designed for central bank monetary operations, creating potential conflicts of accountability. Not all LAC jurisdictions have sufficiently mature CSDs to serve this function. Risk of operational fragmentation if each CSD develops incompatible connectivity standards.

*Requirements:* Central bank authorization for each CSD to operate tCeBM-related functions; formal agreement on liability allocation between central banks and CSDs; common connectivity and messaging standards between CSDs; coordinated supervisory oversight between central bank and securities regulator.

*Candidate for evaluation:* Jurisdictions where CSDs are already involved in regional integration and have established relationships with central banks and securities supervisors. Nuam's Colombia–Chile–Peru integration offers a relevant institutional context to study, but the 2026 CBWeb3 evidence establishes connectivity and testing participation only. It does not establish central-bank authorization, access to settlement accounts, a cross-border cash-finality model or selection of this corridor for a real-value pilot.

**Coexistence in practice.** These four models are not mutually exclusive. A realistic regional architecture will likely see multiple models in use simultaneously: enhanced bilateral correspondent banking for corridors with existing commercial relationships, direct central bank bilateral corridors for the highest-volume pairs, a shared regional platform as the backbone for multilateral connectivity, and CSD interconnection for institutional capital markets flows. The CBWeb3 platform is designed to support Models 1 through 3 directly, with Model 4 representing a natural extension as FMI participation expands.

---

## 9. Liquidity Management for Non-Convertible Currencies

The liquidity management challenge in a multi-currency tokenized settlement system is acute for LAC. Many currencies in the region are not freely convertible, and the FX markets for most LAC currency pairs are thin, fragmented, and expensive. A tokenized cross-border settlement system that does not adequately address liquidity will replicate — in a more technologically elegant form — the same cost and access problems that currently afflict correspondent banking.

Three mechanisms exist to manage liquidity in a multi-currency settlement system, and each involves trade-offs between efficiency, risk, and policy control.

**Mechanism 1 — Pre-funded Bilateral Balances**

Each participant pre-funds accounts denominated in the currencies of its counterparts. When a payment is initiated, it is settled against the pre-funded balance, and the balance is replenished periodically. This is the mechanism used in Scenario A of the CBWeb3 platform (HTLC PvP): each leg of the payment is locked before settlement is initiated, ensuring that both parties have the required funds available.

*Trade-offs:* Simple and low-risk — settlement cannot fail due to insufficient funds if pre-funding is enforced. But it ties up significant capital in pre-funded accounts across all active corridors, which is costly for institutions with constrained balance sheets. For a system with many active corridors and currencies, pre-funding requirements can become prohibitive.

**Mechanism 2 — Bilateral Credit Lines**

Counterpart central banks or commercial banks extend credit to each other for intraday settlement, with end-of-day net settlement in a reserve asset. This is analogous to the intraday credit facilities that central banks provide in traditional RTGS/LBTR systems.

*Trade-offs:* Reduces pre-funding requirements significantly, enabling more efficient use of liquidity. But introduces credit risk: if a participant cannot fund its end-of-day net position, the counterpart is exposed. Requires strong collateral frameworks and centralized credit risk management. Technically straightforward to implement but legally and institutionally complex — bilateral credit lines between central banks in different jurisdictions require legal authority and a formal credit agreement.

**Mechanism 3 — Pooled Liquidity with Automated Market-Making**

Participants contribute liquidity to a shared pool denominated in multiple currencies. When a cross-border payment is initiated, the AMM algorithm determines the exchange rate and routes the transaction through the pool, converting currencies automatically without requiring a pre-established bilateral credit relationship. This is Scenario B of the CBWeb3 platform.

*Trade-offs:* Most capital-efficient for a system with many active participants and corridors — liquidity is pooled rather than fragmented across bilateral pre-funding accounts. Exchange rates are determined algorithmically, which is transparent but may deviate from market rates for illiquid currency pairs, creating basis risk. The central bank cedes some control over the exchange rate at which its currency is converted — a politically sensitive consideration for some jurisdictions. Governance of the pool — who sets parameters, who manages the AMM algorithm, who bears residual risk — requires careful institutional design.

**A candidate FMI coordination approach** is to evaluate whether existing regional FMIs — including interconnected CSDs — can provide consolidated position visibility, intraday reconciliation and operational connectivity for authorized participants. These capabilities do not themselves supply credit, collateral or liquidity and therefore do not, by themselves, reduce pre-funding. Any liquidity benefit must be demonstrated through the funding arrangement and measured against the existing baseline.

**Candidate sequencing for evaluation.** Pre-funded bilateral balances (Mechanism 1) may be the simplest sandbox starting point and reduce the risk of insufficient funds, but they do not eliminate legal, operational, counterparty or reconciliation risk. Pooled AMM liquidity (Mechanism 3) should be considered only after governance is established and realistic stress, pricing, liquidity, privacy and recovery evidence meets approved criteria. Intraday credit facilities (Mechanism 2) require explicit credit authority, limits, collateral and loss-allocation rules.

---

## 10. Policy Considerations and Governance Approaches

Tokenized central bank money raises a set of policy questions that do not have universally correct answers. Different jurisdictions will resolve them differently, reflecting their monetary policy frameworks, financial stability concerns, and institutional traditions. This section frames the key considerations and presents the governance approaches that the CBWeb3 initiative has adopted.

**Monetary sovereignty.** The most fundamental policy concern is whether tokenized issuance preserves or erodes monetary sovereignty. Properly designed, tokenized central bank money strengthens sovereignty: it gives the monetary authority a digital instrument that can compete with private alternatives while remaining under public control. The risk to sovereignty arises if the technical infrastructure is designed in ways that limit the central bank's ability to set terms of access, control issuance volume, or enforce monetary policy transmission. This is why the CBWeb3 architecture maintains each country's spoke network as fully sovereign — the central bank of each jurisdiction controls its own issuance parameters and can disconnect from the regional hub at any time without affecting its domestic settlement operations.

**Financial stability.** The introduction of a new form of central bank money has implications for financial intermediation. If tCeBM at the wholesale level makes it significantly easier for large institutions to settle directly with the central bank, bypassing commercial bank intermediation, this could affect the funding model of commercial banks. This risk is more acute at the retail level (if retail CBDC displaces bank deposits) than at the wholesale level (where the central bank already provides settlement accounts). Most LAC jurisdictions are focused on wholesale applications for now, which substantially limits this risk.

**Privacy.** Transaction privacy is a significant concern for both individual and institutional users of a tokenized payment system. The CBWeb3 platform uses the Hyperledger Paladin privacy framework with Zeto zero-knowledge proof (ZKP) technology to enable transaction confidentiality: the existence of a transaction can be recorded on the ledger without revealing the amounts, identities of counterparties, or transaction details to unauthorized participants. Authorities must determine the appropriate balance between transaction privacy and the AML/CFT visibility that supervisory authorities require — and must ensure that this balance is embedded in the technical architecture, not left to ad hoc governance decisions. Privacy protections must extend to metadata as well as transaction content to avoid leaking sensitive information indirectly.

**The institutional role of CSDs in tokenized cash infrastructure.** CSDs and other FMIs can play a relevant role in the implementation of wholesale tCeBM, particularly in markets where they already operate critical institutional settlement processes. Their experience in participant management, reconciliation, traceability, operational continuity, access controls, and coordination with settlement banks positions them as natural candidates to act as operational nodes or synchronization infrastructures. A pragmatic approach involves representing central bank account balances in tokenized form, with the CSD operating the technical, functional, or reconciliation layer under explicit central bank authorization and with clear rules on accountability. The central bank retains the monetary liability, the issuance authority, access rules, and supervisory capacity; the CSD operates the representation layer under central bank oversight. This approach is particularly useful in LAC because it allows starting with a closed, wholesale, controlled use case without launching a general-purpose CBDC. It can also build on existing regional integration processes between exchanges, CCPs, CSDs, and technology providers, reducing adoption costs and improving the probability of effective institutional use.

**Governance of the CBWeb3 Working Group.** The CBWeb3 initiative operates through a Digital Public Good (DPG) Working Group hosted under the Linux Foundation Decentralized Trust governance framework. The working group has two co-chairs nominated by the Centro de Estudios Monetarios Latinoamericanos (CEMLA) and the Fondo Latinoamericano de Reservas (FLAR) — the two leading regional financial institutions — providing an institutional anchor that is independent of any single central bank or commercial interest. Technical decisions are made by a maintainer core of technical leads, with broader community input through a bi-weekly technical sync and a public issue tracker. Formal decisions that cannot reach consensus are resolved by GitVote, with a 50%+1 threshold and a seven-day comment period. This governance model balances openness — any institution can contribute — with accountability, ensuring that the infrastructure evolves in ways that reflect the needs of the regional community rather than any single participant.

---

## 11. Final Remarks: A Gradual and Coordinated Path Forward

The 2026 evidence supports continued controlled experimentation, not a universal decision to deploy. CBWeb3 provides a reference implementation and interoperability workflow tested under bounded sandbox conditions. It does not yet establish a production service, legal settlement finality, integration with live RTGS systems, an approved multi-country liability framework or a sustainable operating model. Each authority should proceed only when its measured problem, available alternatives and risk appetite justify the next gate.

**Summary Action Roadmap**

The following roadmap organizes the key actions by phase. The month ranges are indicative planning bands, not commitments. Phase transitions require an accountable `go`, `conditional go`, or `no-go` decision supported by evidence.

*Phase 1 — Legal and Institutional Readiness (Months 1–3)*

- Conduct internal legal assessment using the five-domain checklist in Section 4
- Identify gaps and determine whether legal opinions, regulatory guidance, or legislative changes are required
- Designate an internal project lead and establish a cross-functional working group (legal, technology, monetary policy)
- Engage with the CBWeb3 Working Group to begin the onboarding process
- Conduct threat modeling and initial security architecture review of the proposed implementation approach
- Document the formal institutional decision to proceed, with explicit go/no-go criteria for advancement to Phase 2

*Phase 2 — Technical Engagement and Testnet Connection (Months 3–6)*

- Complete technical integration with the CBWeb3 testnet using the standardized API specifications and test vectors available in the public repository
- Execute the 15-minute PvP quickstart tutorial and validate technical connectivity
- Define the initial closed user group according to the eligibility criteria in Section 6
- Commission legal validation of participation agreements, liability allocation, and smart contract enforceability under applicable law
- Begin design of the economic model: how will liquidity costs be allocated among participants? What incentive structures support active participation?
- Establish the benefit measurement framework: what metrics will be used to assess value creation relative to existing alternatives (RTGS, correspondent banking, ISO 20022 modernization)?
- Document the operating model: 24/7 operations and support cost, infrastructure capacity, refresh cycle, pricing or fee policy, expected volume and break-even assumptions
- Compare CBWeb3 with non-tokenized RTGS/API modernization, SWIFT-based approaches and private rails before attributing incremental benefit to tokenization

*Phase 3 — Closed Group Pilot (Months 6–12)*

- Execute the first set of domestic interbank settlement transactions (or institutional FMI-based settlement, as applicable) in the closed group environment
- Conduct operational resilience and cybersecurity testing against defined stress scenarios, including participant disconnection and hub failure
- Exercise negative paths: timeouts, refunds, stuck confirmations, unavailable peers, pending issuance reconciliation and recovery without duplicate issuance
- Complete post-pilot assessment and share findings with the CBWeb3 Working Group
- Measure outcomes against the benefit measurement framework established in Phase 2
- Establish a pilot governance framework: who decides on protocol upgrades? How are disputes resolved? What is the process for participant removal?
- Begin legal preparation for cross-border use: identify counterpart jurisdictions, initiate bilateral legal review, define participation agreement terms
- Formal go/no-go decision for Phase 4, based on documented evidence from pilot results and operational testing
- Treat the observed three-central-bank batch limit as a capacity constraint of the 2026 test environment; produce a measured capacity and scaling plan before expanding participation

*Phase 4 — Cross-Border Connectivity (Months 12–24)*

- Select the cross-border settlement model appropriate to your jurisdiction's context (Section 8)
- Establish bilateral or multilateral sandbox connectivity using the selected model; hub participation is conditional on Scenario B closing its maturity gaps
- Implement the sandbox-to-real-value transition plan, including independent audit of pilot results, governing law determination for multi-country disputes, and contingency plan for hub failure or CB disconnection during active obligations
- Before any real-value test, approve key custody, high availability, backup and recovery, functional monitoring, incident runbooks, RTGS/off-ledger reconciliation, participant notification and allocation of liability and losses
- Participate in the regional governance framework and contribute to the evolution of the CBWeb3 Digital Public Good infrastructure

The appropriate next step may be further sandbox work, a narrower use case, modernization of existing infrastructure, or a decision not to proceed. CBWeb3 offers reusable evidence and components for that decision; it should not predetermine the result.

---

## 12. Risk Register

The following register identifies the primary risk categories associated with tCeBM deployment in the LAC context. It is intended as a starting point for jurisdictional risk assessments, not as an exhaustive analysis. Each jurisdiction should adapt and extend this register based on its specific legal, operational, and market context.

| Risk Category | Description | Potential Impact | Mitigation Approach |
|---|---|---|---|
| **Hub concentration** | Systemic risk concentrated at the regional hub creates a single point of failure for all participating jurisdictions | High — failure of the hub could halt settlement for all connected countries simultaneously | Redundancy requirements; distributed hub architecture; operational resilience standards; each spoke remains operational independently of hub availability |
| **Operator/technology dependency** | Over-reliance on a single infrastructure operator or technology provider | Medium — operator failure or contract termination creates transition risk | Multi-vendor strategy; open-source components with documented standards; portability requirements in operator agreements; pre-defined exit and transition plan |
| **Smart contract failures** | Bugs, logic errors, or exploits in settlement smart contracts causing incorrect or failed settlement | High — financial losses, loss of settlement finality, reputational damage | Formal verification of critical contract logic; independent external audit before production deployment; staged deployment with circuit breakers; defined upgrade governance |
| **Legal-technical misalignment** | On-chain settlement finality not recognized as legal settlement finality in one or more participating jurisdictions | High — uncertainty about the legal status of settled transactions undermines participant confidence | Pre-deployment legal opinions from each jurisdiction; update to financial market infrastructure laws or regulations where needed; settlement finality framework clarification |
| **Non-convertible currency liquidity** | Insufficient market depth for AMM-based exchange of illiquid currency pairs, creating exchange rate deviations or settlement failures | Medium — AMM rates may deviate significantly from market rates for minor currency pairs | Pre-funding requirements for illiquid pairs; circuit breakers that halt AMM transactions when deviation exceeds threshold; minimum liquidity requirements for entry into AMM pool |
| **Regulatory arbitrage** | Participants structuring transactions to exploit differences in AML/CFT regimes, screening standards, or reporting requirements across jurisdictions | Medium — creates compliance gaps and reputational risk for the platform | Common AML/CFT minimum standards as participation condition; information-sharing agreements between supervisors; joint or coordinated oversight mechanisms |
| **Metadata leakage** | Transaction metadata (timing, volume patterns, counterparty patterns) revealing sensitive information even when transaction content is protected by ZKP | Medium — sensitive business information exposed to unauthorized parties | Privacy-preserving architecture review covering metadata as well as content; ZKP coverage extended to metadata where technically feasible; access controls on ledger visibility |
| **Exclusion of smaller actors** | High integration costs or technical complexity preventing smaller financial institutions from participating, concentrating benefits among large participants | Medium — undermines the DPG mandate and reproduces existing access inequalities in digital form | Standardized open-source APIs; sandbox environment with technical support; tiered participation categories with lighter-weight entry points; implementation support for smaller institutions |
| **Reputational risk from pilot failure** | Public perception damage if pilot fails, produces adverse outcomes, or is mischaracterized in media or political discourse | Medium — could set back CBDC adoption broadly across LAC | Controlled rollout with clear communication strategy; pre-defined success criteria and exit conditions; independent evaluation of pilot results before public disclosure |
| **Digital dollarization acceleration** | If tCeBM design is uncompetitive with USD-denominated stablecoins on programmability, cost, or accessibility, stablecoin adoption may accelerate rather than decrease | Medium-High — undermines the primary sovereignty motivation for tCeBM deployment | Competitive design analysis against USD stablecoins before deployment; pricing model that does not disadvantage local currency instruments; programmability features matching or exceeding private alternatives |
| **Cybersecurity incidents** | Attacks on node infrastructure, key management systems, communication channels, or smart contracts | High — could cause financial losses, data breaches, or extended settlement outages | Security standards compliance (e.g., ISO 27001 or equivalent); regular third-party security audits; defined incident response and recovery protocols; key management hardware security module requirements |
| **CB disconnection mid-obligation** | A central bank disconnecting from the hub while settlement obligations are pending, leaving counterparts with unresolved positions | High — counterparts bear principal risk if disconnection is not handled in a defined protocol | Legal framework for orderly disconnection; pre-funded collateral requirements; tested timeout/unwind process; define and approve treatment and priority of pending obligations before real-value use |
| **Silent pending operations and reconciliation** | Reserve balances may change while token issuance or privacy-layer confirmation remains pending for an extended period | High — apparent loss of value, duplicate-issuance risk and loss of participant trust | Explicit pending state; queue-age limits and alerts; transaction-level reconciliation; safe no-reissue runbook; participant notification and acknowledgment |
| **False-positive infrastructure health** | Ports or containers appear available while peer identity, privacy or confirmation services cannot complete cross-institution flows | High — outages remain undetected while operations accumulate | Functional readiness probes; peer-to-peer checks; synthetic end-to-end transactions; alerts for connection exhaustion, process/container disagreement and stuck confirmations |
| **Access and role mismatch** | Settlement-agent, custodian, correspondent or non-resident holding rules are ambiguous or not technically enforced | High — unauthorized holdings, inconsistent operations and unclear accountability | Approved role and access matrix; enforce non-resident rules in code and conformance tests; reconcile implementation with participation agreements |
| **FX source and parameter governance** | Manual or ungoverned exchange-rate inputs can produce inconsistent or manipulable settlement terms | Medium-High — mispricing, disputes and liquidity losses | Approved external source or controlled oracle process; freshness and deviation checks; fallback and dispute procedures; governed parameter changes |
| **RTGS and off-ledger integration** | Sandbox token balances are not reconciled with live reserve accounts or productive RTGS/LBTR systems | High — token settlement may not correspond to the legally authoritative cash record | Define authoritative ledger; adapter and reconciliation design; failure and rollback rules; joint testing with the relevant central-bank operations teams |
| **Performance and capacity shortfall** | Test throughput, participant slots or privacy operations may not meet production volumes or service levels | High — congestion, failed settlements and exclusion of participants | Versioned performance evidence; realistic stress tests; capacity model; scaling plan; production SLOs and admission limits |
| **Evidence and UAT governance gap** | Technical delivery or a successful demonstration may be misread as institutional acceptance or production authorization | Medium-High — premature deployment and inaccurate public claims | Versioned evidence bundle; participant-approved UAT scope and sign-off; explicit maturity labels; independent readiness decision by accountable authorities |

---

## 13. Evidence and lessons from the 2026 sandbox/UAT

The evidence available from the 2026 participant testing changes the basis on which CBWeb3 should be discussed. It moves Scenario A beyond a purely conceptual design, while also showing why a successful technical flow is only one part of institutional readiness.

> **Evidence note.** This section synthesizes participant feedback from BCRP dated 7 October 2026, BCCR feedback dated 30 September 2026, an internal record of the Costa Rica–Peru joint test, the 2 October 2026 DPGWG coordination notes, platform-delivery reviews, and the 6 November 2026 support incident review. Several of these are internal project records rather than public or independently audited sources; publication requires confirmation by the responsible institutions and removal of any restricted operational detail.

**What was observed.** During the first participant testing batch, the Banco Central de Reserva del Perú reported completing the Scenario A cross-border PvP flow in the sandbox, including onboarding, issuance, reserve tokenization, the FX agreement, HTLC locking, settlement and interoperability between participant networks. Banco Central de Costa Rica reported that the exercise made the roles and cross-border flow more concrete and valuable than a conceptual presentation alone. A Costa Rica–Peru participant-operated PvP was recorded as successful. These results are evidence that the Scenario A workflow can be operated end to end under the tested configuration.

**What participants learned.** The largest adoption frictions included internal security review, VPN and virtual-machine provisioning, permissions, credentials, environment preparation and coordination between institutions. Participants requested clearer definitions of settlement agent, custodian and correspondent roles; rules for non-resident tCeBM holdings; balance limits, remuneration, convertibility and redemption; foreign-exchange data governance; KYC guidance; RTGS/LBTR funding and integration; and a concrete scope and resource plan for a next phase. BCCR also asked for failure and exception scenarios, stronger participant interaction and a clearer distinction between laboratory capabilities and production requirements.

**Operational evidence from failure.** A later incident in the Chile Scenario A homologation environment showed that technical reachability is not equivalent to functional readiness. Five tokenization attempts totaling CLP 1,840,000 remained pending while reserve balances had already changed. After the affected nodes were restored, the queued issuances completed without reissuance and with no reported loss. The incident nevertheless exposed prolonged silent pending states, insufficient functional monitoring, missing queue-age escalation and a serious reconciliation problem. A successful post-recovery PvP retest was not included in the available incident evidence.

**Limits of the evidence.** The available record is participant feedback and technical evidence from a sandbox. It is not a jointly signed institutional UAT acceptance, production authorization, proof of legal settlement finality, connection to live RTGS/LBTR systems or evidence that every corridor and failure path works. The `platform-v1.0.0` tag records a software handoff baseline; it does not change those maturity boundaries. Scenario B remains an experimental prototype, as described in Section 8.

**Implications for the next phase.** Future testing should be organized around evidence, not demonstrations alone. At minimum it should include bidirectional and negative-path flows; transaction-level reconciliation; functional health and synthetic end-to-end monitoring; safe handling of pending issuance without duplicate reissuance; participant-wide impact analysis and notification; performance and capacity measurement; and formal participant review of results. Any transition to real value remains conditional on legal finality, RTGS integration, key custody, high availability, backup and recovery, independent security review, operating economics and an approved allocation of responsibility and losses.

---

# VERSIÓN EN ESPAÑOL

---

## 1. Resumen Ejecutivo: Un Marco de Decisión para las Autoridades de ALC

América Latina y el Caribe (ALC) se encuentran en un momento decisivo en la evolución del dinero. Los pagos transfronterizos en la región siguen siendo de los más costosos y lentos del mundo. Las relaciones de corresponsalía bancaria se están reduciendo. Las stablecoins y las monedas digitales extranjeras ganan terreno en economías dolarizadas o semi-dolarizadas, erosionando el espacio de política disponible para los bancos centrales. En este contexto, el dinero digital de banco central (tCeBM, por sus siglas en inglés: *tokenized Central Bank Money*) ha emergido no como un experimento tecnológico especulativo, sino como un instrumento de política concreto que varias autoridades regionales ya están explorando seriamente.

Este documento está dirigido a los tomadores de decisiones que deben determinar si involucrarse con esta transformación y cómo hacerlo. Se organiza en torno a las preguntas prácticas que enfrentan los altos funcionarios: ¿Qué se está emitiendo exactamente? ¿Está listo nuestro marco legal? ¿Qué modelo operativo se ajusta a nuestro contexto institucional? ¿Dónde empezamos y cómo nos conectamos con la región?

El marco de decisión que propone este documento tiene cuatro dimensiones. Primero, **la pregunta sobre el activo de liquidación**: si emitir tCeBM a nivel mayorista, minorista o ambos, y cómo posicionarlo respecto a los depósitos bancarios comerciales tokenizados y los instrumentos digitales privados. Segundo, **la pregunta sobre el modelo operativo**: si el banco central lidera la infraestructura, delega en intermediarios o adopta un esquema híbrido. Tercero, **la pregunta sobre la integración transfronteriza**: si perseguir corredores bilaterales, unirse a una plataforma regional compartida o modernizar las relaciones de corresponsalía bancaria. Cuarto, **la pregunta sobre la infraestructura de liquidación**: si el tCeBM será operado exclusivamente por el banco central, por bancos comerciales, o a través de infraestructuras del mercado financiero (IMF) autorizadas —como depósitos centrales de valores (DCV), cámaras de compensación u operadores de sistemas de liquidación—. Esta cuarta dimensión determina quién construye y opera la capa operacional, quién es responsable por fallas de liquidación y cómo el tCeBM se conecta con procesos de mercado existentes como la compensación de valores, la reconciliación y la custodia.

La conclusión práctica es que las autoridades pueden usar experimentación controlada y sin valor real para determinar si la tokenización agrega valor en un caso de uso definido. La experimentación no sustituye autoridad legal, controles productivos ni comparación con alternativas no tokenizadas. El testnet regional CBWeb3 proporciona infraestructura de referencia y herramientas de interoperabilidad para esa fase de aprendizaje; cualquier expansión queda condicionada a los gates de evidencia de las Secciones 6 y 11.

---

## 2. Por Qué el Dinero Digital de Banco Central Importa para ALC Ahora

El caso a favor del tCeBM en América Latina y el Caribe no se construye sobre el entusiasmo tecnológico. Se construye sobre un conjunto específico de presiones estructurales que ya están redefiniendo el panorama financiero regional y que exigen una respuesta de política.

**El problema de la corresponsalía bancaria.** El número de relaciones de corresponsalía bancaria activas en ALC ha disminuido sostenidamente durante la última década, impulsado por estrategias de reducción de riesgo de los bancos globales [Fuente: por confirmar — encuestas BIS sobre corresponsalía bancaria]. Para las economías más pequeñas y para ciertas categorías de transacciones —particularmente remesas y financiamiento de comercio exterior— esta contracción ha elevado los costos, alargado los tiempos de liquidación y, en algunos corredores, ha eliminado efectivamente el acceso a infraestructura de pagos internacionales asequible. El tCeBM podría reducir la dependencia de algunos intermediarios al coordinar transiciones PvP; la descarga jurídica definitiva y el beneficio económico neto deben establecerse para cada corredor.

**La presión de las stablecoins y la dolarización.** En toda la región, la adopción de stablecoins crece, particularmente en economías con historias de inestabilidad cambiaria o acceso restringido a cuentas en dólares. Esta adopción representa una potencial transferencia de soberanía monetaria: si una parte significativa de las transacciones domésticas migra hacia instrumentos denominados en monedas extranjeras emitidos por entidades privadas fuera del alcance regulatorio, la capacidad de los bancos centrales para implementar política monetaria, observar el riesgo sistémico y hacer cumplir las normas de prevención de lavado de dinero y financiamiento del terrorismo (PLD/FT) se ve materialmente afectada. El tCeBM ofrece una alternativa soberana: digital, programable, interoperable, pero emitida y gobernada por la autoridad monetaria.

**La ineficiencia de los pagos transfronterizos.** Enviar dinero a través de las fronteras de ALC sigue siendo lento, opaco y costoso. Los costos promedio de remesas en la región permanecen por encima del objetivo del G20 del 3% [Fuente: por confirmar — Banco Mundial, Remittance Prices Worldwide]. La liquidación transfronteriza tokenizada puede comprimir los ciclos de liquidación de días a segundos y eliminar capas de intermediación que no cumplen otra función que generar fricción.

**La oportunidad en la liquidación de mercados de capitales.** Más allá de los casos de uso de pagos al consumidor y corresponsalía bancaria, el tCeBM puede abordar una ineficiencia estructural en los mercados institucionales. En ALC, la coordinación entre sistemas de pago, bancos liquidadores, cámaras de compensación, custodios y depósitos centrales de valores (DCV) genera costos, tiempos de espera, necesidades de reconciliación y riesgos operativos en el ciclo post-negociación de transacciones de mercados de capitales. El tCeBM, como capa común de liquidación accesible a las IMF autorizadas, podría servir como fundamento para una liquidación institucional más eficiente, tanto para pagos transfronterizos como para transacciones de valores, compensación de derivados y otras operaciones de mercado de capitales.

**Un panorama regional diferenciado.** ALC no es una región monolítica. Tres grandes grupos caracterizan el panorama actual:

- *Jurisdicciones avanzadas* — países como Brasil, México y Colombia, posicionados para ser pioneros y anclas regionales.

- *Jurisdicciones en desarrollo de capacidades* — países como Perú, Chile, Ecuador, Costa Rica y República Dominicana. Cabe destacar que Colombia, Chile y Perú también comparten un proceso activo de integración de mercados de capitales a través de Nuam, que integra las bolsas de los tres países y avanza hacia infraestructura post-negociación común, incluyendo DCV y cámaras de compensación central (CCP). Esta integración regional proporciona una base institucional concreta para pilotos de tCeBM con alcance transfronterizo.

- *Jurisdicciones con involucramiento incipiente* — economías más pequeñas donde un enfoque plug-and-play es el camino pragmáticamente viable.

La iniciativa CBWeb3 está diseñada para servir a los tres grupos, con la red de pruebas regional brindando un entorno de experimentación compartido y el marco de gobernanza asegurando que las opciones de diseño de ningún país se impongan a los demás.

---

## 3. ¿Qué Se Está Emitiendo? Clarificando las Formas de Dinero Digital

Los debates de política sobre tokenización frecuentemente son imprecisos sobre lo que realmente se está emitiendo. Esta sección establece las distinciones clave.

**El dinero digital de banco central (tCeBM)** es una representación digital de un pasivo del banco central, emitida en un libro mayor distribuido (DLT) o infraestructura programable. Tiene la misma calidad crediticia y finalidad de liquidación que las reservas mantenidas en cuentas tradicionales del banco central. A nivel mayorista, el tCeBM funciona como el activo de liquidación definitivo — el equivalente de un saldo de reservas, pero programable y capaz de liquidación atómica con otros activos tokenizados. A nivel minorista, funciona como una forma digital del efectivo emitida directamente al público, aunque la mayoría de las jurisdicciones de ALC están actualmente enfocadas en aplicaciones mayoristas.

**Los depósitos bancarios comerciales tokenizados (tDeposits)** son representaciones digitales de derechos frente a un banco comercial, emitidos en infraestructura programable. Llevan el riesgo crediticio de la institución emisora y están sujetos a los límites del seguro de depósitos. La interoperabilidad entre tDeposits emitidos por distintos bancos requiere una capa de liquidación común, que se provee más eficientemente con tCeBM.

**Los saldos de cuentas del banco central operados por IMF autorizadas** designan una representación digital de saldos mantenidos por una IMF autorizada —como un DCV, cámara de compensación u operador de sistema de liquidación— en cuentas del banco central, donde el banco central retiene el pasivo monetario pero la IMF gestiona la capa operacional. Esta categoría es distinta de la emisión directa de tCeBM: la IMF actúa como intermediario autorizado o nodo de sincronización, no como emisor independiente. La responsabilidad legal por el pasivo monetario permanece con el banco central; la responsabilidad operacional por la capa de representación y reconciliación recae en la IMF autorizada.

**Las stablecoins y otros instrumentos digitales privados** son tokens emitidos por entidades privadas, no pasivos del banco central. Las stablecoins respaldadas en fiat denominadas en dólares introducen riesgo de denominación en moneda extranjera a escala; las stablecoins algorítmicas introducen riesgos de estabilidad adicionales. Desde una perspectiva de política, los instrumentos digitales privados son complementos o competidores potenciales del tCeBM, no sustitutos de este.

La plataforma CBWeb3 se enfoca en la emisión de tCeBM y la liquidación mayorista. Los contratos inteligentes que gobiernan la emisión, la liquidación transfronteriza basada en contratos de bloqueo por tiempo con hash (HTLC) y la gestión de liquidez basada en creación de mercado automatizada (AMM) están todos diseñados en torno al banco central como emisor principal y al tCeBM como activo de liquidación de referencia.

---

## 4. Preparación Legal y Regulatoria: El Camino Crítico

Antes de embarcarse en un programa de emisión tokenizada, las autoridades necesitan evaluar si sus marcos legales y regulatorios vigentes son aptos para el propósito. La siguiente lista de verificación estructura esta evaluación en cinco dominios.

**Dominio 1 — Definición legal del dinero del banco central.** ¿Permite la legislación vigente al banco central emitir dinero en forma digital o tokenizada?

**Dominio 2 — Finalidad de la liquidación.** ¿Reconoce el sistema legal la finalidad de la liquidación en un entorno de libro mayor distribuido?

**Dominio 3 — Reglas de acceso.** ¿Quién puede tener o transaccionar en tCeBM? A nivel mayorista, el acceso típicamente se restringe a instituciones financieras reguladas. Las jurisdicciones deben considerar explícitamente si los DCV, cámaras de compensación y otras IMF autorizadas pueden participar como titulares de cuentas directas, operadores técnicos o nodos de sincronización autorizados, y establecer la base legal para cada categoría de participación.

**Dominio 4 — Regulaciones PLD/FT y cambiarias.** ¿Cómo se aplican las regulaciones existentes contra el lavado de dinero (PLD), el financiamiento del terrorismo (FT) y las divisas a las transacciones de tCeBM? Las diferencias en los estándares de control PLD/FT entre países participantes representan un riesgo específico de arbitraje regulatorio que debe abordarse mediante estándares mínimos comunes.

**Dominio 5 — Protección de datos y ciberseguridad.** ¿Se aplica el marco de privacidad y protección de datos a los datos de transacciones registrados en un libro mayor distribuido? Donde aplique, los marcos nacionales de protección de datos —como la Ley General de Protección de Datos (LGPD) de Brasil o marcos equivalentes— deben cumplirse tanto por el banco central como por los participantes.

**Estado actual en las jurisdicciones de ALC.** [Fuente: por confirmar — publicaciones de bancos centrales nacionales y marcos regulatorios]:

- *Brasil:* Marco legal sólido bajo el programa Drex y el LIFT Lab. LGPD bien desarrollada. La jurisdicción legalmente más avanzada de la región.
- *México:* La Ley Fintech (2018) estableció el marco para activos digitales. Legislación CBDC en desarrollo.
- *Colombia:* Autoridad de emisión digital disponible bajo el estatuto orgánico del Banco de la República. Marcos PLD/FT robustos.
- *Perú:* Marco generalmente permisivo, sin disposiciones específicas sobre CBDC. La Superintendencia de Banca, Seguros y AFP (SBS) necesitaría coordinar las reglas de acceso.
- *Chile:* Estudio de factibilidad publicado en 2022. Autoridad legal disponible bajo el estatuto vigente; regulaciones de implementación pendientes.
- *Costa Rica, República Dominicana, Ecuador:* Interés activo pero sin adaptaciones formales al marco legal en curso.
- *Economías del Caribe Oriental:* La iniciativa DCash del Banco Central del Caribe Oriental (ECCB) proporciona un precedente regional [Fuente: por confirmar — documentación ECCB DCash].

---

## 5. Modelos Operativos para la Emisión (No Técnico)

La elección del modelo operativo determina quién construye y opera la infraestructura, quién asume qué riesgo, quién tiene acceso a la capa de liquidación y cómo evoluciona el sistema con el tiempo.

**Modelo A — Liderado por el Banco Central**

El banco central diseña, construye y opera la infraestructura central de liquidación tokenizada.

*Ventajas:* Máximo control sobre la transmisión de política monetaria. Visibilidad plena de los flujos de liquidación. Sin dependencia de proveedores externos. Cadena de responsabilidad clara ante errores operativos.

*Desventajas:* Máximo requerimiento de capacidad institucional. Largos plazos de desarrollo. Riesgo de construir sistemas propietarios difíciles de interoperar. Alto costo inicial. El banco central asume responsabilidad exclusiva por todas las fallas operacionales.

*Requerimientos:* Equipo tecnológico interno con capacidad en DLT e ingeniería criptográfica; gestión operacional 24/7; plan integral de continuidad de negocio.

*Más adecuado para:* Jurisdicciones del Grupo 1 (Brasil, México, Colombia).

**Modelo B — Híbrido**

El banco central define el marco legal y de política, provee el tCeBM y mantiene la supervisión, pero delega el diseño, construcción y operación de la infraestructura técnica en una entidad regulada o consorcio —que puede ser un banco de inversión, un proveedor de tecnología regulado, o una IMF autorizada como un DCV o cámara de compensación.

Una variante notable —particularmente relevante para jurisdicciones con infraestructura de mercados de capitales madura— implica delegar la capa operacional en un DCV u otra IMF autorizada. En esta variante, el banco central retiene el pasivo monetario y la autoridad de emisión, mientras el DCV opera la capa técnica, gestiona el acceso de participantes, maneja la reconciliación y provee conectividad con los flujos de liquidación de valores existentes. Este enfoque aprovecha las relaciones operativas establecidas entre el DCV y los participantes del mercado (bancos comerciales, custodios, agentes de liquidación, emisores) y evita construir conectividad paralela desde cero.

*Ventajas:* Permite al banco central enfocarse en su mandato principal mientras aprovecha la experiencia tecnológica externa. Más rápido de desplegar. En la variante DCV, construye sobre relaciones de participantes y puntos de integración post-negociación ya existentes.

*Desventajas:* El banco central depende del operador para disponibilidad y seguridad. La relación legal debe establecer claramente la responsabilidad por fallas del sistema, procedimientos de desconexión y protocolos de escalada. En la variante DCV: las estructuras de gobernanza del DCV pueden no estar diseñadas para operaciones monetarias del banco central, generando posibles conflictos de responsabilidad.

*Requerimientos:* Acuerdo legal formal entre banco central y operador; estándares operativos mínimos definidos por el banco central; protocolos claros de gestión de crisis.

*Más adecuado para:* La mayoría de las jurisdicciones de ALC en los Grupos 1 y 2. El modelo CBWeb3 —con LNET como operador e IDB Lab como financiador— es una variante de este enfoque.

**Modelo C — Totalmente Intermediado**

El banco central emite reservas a los bancos comerciales en forma tokenizada pero no opera ninguna plataforma directamente.

*Ventajas:* Carga operativa mínima para el banco central. Máximo aprovechamiento de la innovación del sector privado. Despliegue incremental.

*Desventajas:* La interoperabilidad no es automática. Menor visibilidad directa del banco central sobre los flujos de liquidación. Riesgo de fragmentación. Menor capacidad del banco central para establecer reglas de acceso y precios consistentes en la capa de liquidación.

*Requerimientos:* Estándares de interoperabilidad integrales con poder de enforcement vinculante; pruebas de conformidad API obligatorias; marco de responsabilidad claro ante fallas de liquidación entre instituciones.

---

## 6. Un Punto de Partida Mínimo Viable para la Emisión Tokenizada

La barrera más común para la acción no es la incertidumbre legal ni la madurez tecnológica — es la escala y complejidad percibida del emprendimiento. Esta sección propone un enfoque mínimo viable: un punto de partida deliberadamente acotado y de bajo riesgo. El término "mínimo viable" describe un punto de partida operacional estructurado, no un plano de diseño de servicio formal — el objetivo es definir el alcance más estrecho posible que genere aprendizaje operacional real.

**El principio del grupo cerrado de usuarios.** El punto de partida recomendado es un entorno cerrado y con permisos con un conjunto pequeño y predefinido de participantes regulados.

**Criterios de elegibilidad para el grupo cerrado:**

1. *Estar regulada por una autoridad competente* en la jurisdicción.
2. *Ser técnicamente capaz de integración por API*.
3. *Estar operacionalmente comprometida* — capaz de designar contactos técnico y legal nominados.
4. *Haber recibido autorización legal*.
5. *Estar vinculada contractualmente* — habiendo firmado un acuerdo de participación.

En contextos donde el caso de uso inicial involucre liquidación de valores o mercados institucionales, los DCV, cámaras de compensación y sus bancos liquidadores y custodios conectados también deberían ser considerados participantes elegibles, siempre que cumplan los mismos criterios regulatorios y operativos y hayan recibido autorización de la autoridad supervisora competente.

**Casos de uso iniciales recomendados.** Dos casos de uso son apropiados para la fase inicial del grupo cerrado:

El primero es la **liquidación interbancaria doméstica**: liquidación en moneda doméstica entre dos o más participantes del grupo cerrado, utilizando ISO 20022. Este caso de uso genera aprendizaje operacional inmediato sobre latencia, confirmación de finalidad, manejo de errores y reconciliación.

El segundo, particularmente adecuado para jurisdicciones con infraestructura activa de mercados de capitales, es la **liquidación institucional a través de IMF autorizada**: liquidación de efectivo tokenizado en un proceso institucional administrado por una IMF autorizada como un DCV. El objetivo no es extender el acceso a usuarios retail sino validar si una representación tokenizada de saldos de cuentas del banco central puede ser operada por una infraestructura autorizada con controles definidos. Este caso de uso es directamente relevante para las necesidades operativas de instituciones como Nuam, cuyo caso de uso implica recibir efectivo de una jurisdicción para liquidar transacciones de valores en otra.

**Criterios de salida del sandbox.** La fase del grupo cerrado no debe considerarse completa únicamente porque funcione el flujo exitoso. La salida requiere: (a) al menos dos instituciones completando el flujo técnico extremo a extremo en ambas direcciones previstas; (b) reconciliación de débitos de reservas, emisión de tokens, operaciones pendientes y saldos finales; (c) pruebas de expiración, reversión, desconexión de participantes y recuperación; (d) evidencia de que los controles de salud funcional detectan pérdida de comunicación entre pares y confirmaciones atascadas; (e) resolución o aceptación explícita de defectos materiales; y (f) una evaluación conjunta que documente retroalimentación, riesgos residuales y la siguiente decisión. La ejecución atómica en el sandbox es evidencia técnica; no establece finalidad jurídica de liquidación.

**Plazo y esfuerzo institucional.** Las pruebas de 2026 mostraron que el tiempo depende no solo del software, sino también de revisión de seguridad, aprovisionamiento de VPN y máquinas virtuales, permisos, credenciales, coordinación interna y disponibilidad de contrapartes. Este documento no prescribe por tanto un plazo genérico de seis a doce meses. Cada autoridad debe estimarlo después de evaluar su preparación y designar responsables jurídicos, tecnológicos, de ciberseguridad, tesorería y operaciones.

---

## 7. De la Emisión Doméstica a la Utilidad Real

Un banco central que haya completado con éxito un piloto de emisión doméstica ha logrado algo significativo — pero aún no ha desbloqueado la fuente más importante de valor del dinero tokenizado. El potencial transformador emerge cuando el tCeBM se vuelve interoperable: cuando puede utilizarse para liquidar transacciones transfronterizas, cuando la liquidez puede fluir entre jurisdicciones sin múltiples rondas de conversión de divisas y cuando las instituciones financieras de diferentes países pueden transaccionar entre sí utilizando un protocolo de liquidación compartido.

La plataforma CBWeb3 está diseñada para apoyar precisamente esta transición. Su arquitectura hub-and-spoke permite que el banco central de cada país mantenga plena soberanía sobre su emisión y liquidación doméstica mientras se conecta a un hub transnacional compartido para transacciones transfronterizas.

---

## 8. Modelos de Liquidación Transfronteriza para ALC

> **Nota de madurez sobre los escenarios del testnet CBWeb3:** El “Escenario A” es el modelo PvP basado en HTLC implementado y ejercitado de extremo a extremo en el sandbox, incluido un flujo Costa Rica–Perú operado por participantes. Esto demuestra el flujo técnico bajo condiciones de prueba, no finalidad jurídica ni preparación productiva. El “Escenario B” es un prototipo experimental de AMM/pool de liquidez cuyo flujo exitoso fue ejecutado en una cadena hub dedicada. No está listo para producción: siguen abiertos la privacidad en el hub, concurrencia de firmantes, operación multi-validador, rendimiento bajo swaps exigentes, resiliencia y hardening operativo. Los dos escenarios no deben presentarse con la misma madurez.

Existen cuatro modelos prácticos para la liquidación transfronteriza tokenizada.

**Modelo 1 — Corresponsalía Bancaria Tokenizada (PvP Mejorado)**

La relación bilateral entre un banco doméstico y su banco corresponsal se replica en infraestructura tokenizada. Mediante HTLC, dentro del modelo técnico probado, ambas transiciones de estado se completan o se aplica la expiración/reversión. El Escenario A implementa y ejercitó este flujo en el sandbox. La descarga definitiva de las obligaciones jurídicas debe establecerse separadamente en cada jurisdicción y acuerdo de participación.

*Ventajas:* Construye sobre relaciones existentes. Sin necesidad de infraestructura regional compartida. Despliegue bilateral, corredor por corredor.

*Desventajas:* Escala linealmente. No resuelve el pre-fondeo de liquidez. Limitado a pares de monedas con corresponsalía existente.

*Requerimientos:* Infraestructura de contratos inteligentes compatible con HTLC en ambos extremos; reconocimiento legal de la finalidad HTLC en ambas jurisdicciones.

**Modelo 2 — Corredores Bilaterales con Liquidación Compartida**

Dos bancos centrales establecen un acuerdo de liquidación bilateral directo, omitiendo completamente la capa de corresponsalía comercial.

*Ventajas:* Elimina la intermediación comercial. Visibilidad directa de los flujos transfronterizos. Base más sólida para la coordinación de política monetaria.

*Desventajas:* Requiere tratado o MOU entre bancos centrales. Proceso prolongado. Plantea preguntas sobre la ley aplicable en disputas y el tratamiento de obligaciones pendientes si un banco central se desconecta.

*Requerimientos:* Marco legal bilateral formal; definición de ley aplicable para disputas transfronterizas; requisitos de colateral o saldo pre-fondado para gestionar el riesgo durante interrupciones de conectividad.

**Modelo 3 — Plataforma Regional Compartida**

En este modelo, los países participantes se conectarían a un hub multilateral con liquidez agrupada y creación de mercado automatizada. El Escenario B proporciona un prototipo técnico experimental; todavía no proporciona un marco jurídico multilateral aprobado, un acuerdo productivo de liquidez ni un servicio operativo listo para producción.

*Ventajas potenciales por validar:* Una topología multilateral podría evitar N×(N-1)/2 integraciones técnicas bilaterales. La liquidez agrupada podría reducir el prefondeo en algunos corredores si se aporta liquidez suficiente y con precio adecuado. La conectividad y el acceso a liquidez de nuevos participantes seguirían sujetos a onboarding, acuerdos legales, capacidad y gobernanza. Son hipótesis a medir, no resultados productivos demostrados.

*Desventajas:* Mayor requerimiento de coordinación multilateral. Riesgo sistémico concentrado en el hub. Los tipos de cambio AMM pueden desviarse de los precios de mercado para pares de divisas poco líquidos.

*Requerimientos:* Marco legal multilateral; operador del hub neutral con gobernanza apropiada; proceso de gobernanza de parámetros AMM; cortacircuitos y umbrales de liquidez para pares de monedas no convertibles.

**Modelo 4 — Interconexión Regional de DCV para Efectivo Tokenizado**

En este modelo, los DCV de los países participantes actúan como infraestructuras intermediarias autorizadas que operan, sincronizan o facilitan movimientos de tCeBM respaldado por saldos de cuentas del banco central. Cada banco central retiene la autoridad de emisión y el pasivo monetario; los DCV aportan conectividad regional, estándares operativos, gestión de participantes, capacidad de reconciliación e integración con los procesos institucionales de mercado existentes.

*Ventajas:* Aprovecha infraestructura existente y relaciones establecidas entre DCV y participantes del mercado. Reduce la duplicación de inversión operacional. Facilita la adopción porque los participantes ya están conectados a su DCV doméstico. Fortalece la integración regional a través de instituciones con trayectoria en coordinación transfronteriza. Habilita pilotos controlados e institucionales sin consideraciones de acceso retail. Apoya directamente la eficiencia post-negociación de los mercados de capitales como caso de uso concreto e inmediato.

*Desventajas:* Requiere acuerdos claros sobre responsabilidad, supervisión, ciberseguridad, continuidad operativa, reconciliación con cuentas del banco central, ley aplicable, manejo de errores y procedimientos de desconexión de participantes. Las estructuras de gobernanza de los DCV pueden no estar diseñadas para operaciones monetarias del banco central. No todos los DCV de ALC tienen suficiente madurez para cumplir esta función.

*Requerimientos:* Autorización del banco central para cada DCV; acuerdo formal sobre asignación de responsabilidades; estándares de conectividad y mensajería comunes entre DCV; supervisión coordinada entre el banco central y el regulador de valores.

*Candidato para evaluación:* Las jurisdicciones donde los DCV ya participan en procesos de integración regional. La integración Colombia–Chile–Perú de Nuam ofrece un contexto institucional relevante para estudiar, pero la evidencia CBWeb3 de 2026 demuestra conectividad y participación en pruebas, no autorización de bancos centrales, acceso a cuentas de liquidación, un modelo de finalidad transfronteriza ni la selección del corredor para un piloto con valor real.

**Coexistencia en la práctica.** Estos cuatro modelos no son mutuamente excluyentes y la arquitectura regional probablemente verá los cuatro en uso simultáneo: corresponsalía mejorada para corredores con relaciones comerciales existentes, corredores bilaterales directos para los pares de mayor volumen, una plataforma regional compartida como columna vertebral multilateral, e interconexión de DCV para flujos institucionales de mercados de capitales.

---

## 9. Gestión de Liquidez para Monedas No Convertibles

El desafío de gestión de liquidez en un sistema de liquidación tokenizado multi-moneda es agudo para ALC, donde muchas monedas no son libremente convertibles.

**Mecanismo 1 — Saldos Bilaterales Pre-fondados** (Escenario A / HTLC): Simple y de bajo riesgo, pero inmoviliza capital significativo en todos los corredores activos.

**Mecanismo 2 — Líneas de Crédito Bilaterales**: Reduce el pre-fondeo pero introduce riesgo crediticio y complejidad institucional. Análogo a las facilidades de crédito intradía que los bancos centrales proveen en los sistemas tradicionales de LBTR (Liquidación Bruta en Tiempo Real).

**Mecanismo 3 — Liquidez Agrupada con AMM** (Escenario B): Más eficiente en capital, pero los tipos de cambio algorítmicos pueden desviarse de precios de mercado para pares poco líquidos. La gobernanza del pool requiere un diseño institucional cuidadoso.

**Un enfoque candidato de coordinación mediante IMF** consiste en evaluar si las IMF regionales —incluyendo DCV interconectados— pueden proporcionar visibilidad consolidada, reconciliación intradía y conectividad operacional. Estas capacidades no aportan por sí mismas crédito, colateral o liquidez y, por tanto, no reducen automáticamente el prefondeo. Cualquier beneficio debe demostrarse a través del acuerdo de fondeo y medirse contra la línea base existente.

**Secuencia candidata para evaluación.** Los saldos bilaterales pre-fondados pueden ser el punto de partida más simple en sandbox y reducen el riesgo de fondos insuficientes, pero no eliminan riesgos jurídicos, operativos, de contraparte o reconciliación. La liquidez AMM debe considerarse solo después de establecer gobernanza y obtener evidencia satisfactoria de estrés, precios, liquidez, privacidad y recuperación. Las líneas de crédito intradía requieren autoridad crediticia, límites, colateral y reglas de asignación de pérdidas.

---

## 10. Consideraciones de Política y Enfoques de Gobernanza

**Soberanía monetaria.** La arquitectura CBWeb3 mantiene la red spoke de cada país como completamente soberana — el banco central controla sus propios parámetros de emisión y puede desconectarse del hub regional en cualquier momento sin afectar sus operaciones domésticas de liquidación.

**Estabilidad financiera.** La mayoría de las jurisdicciones de ALC están enfocadas en aplicaciones mayoristas, lo que limita sustancialmente el riesgo de desintermediación bancaria.

**Privacidad.** La plataforma CBWeb3 utiliza el marco de privacidad Hyperledger Paladin con pruebas de conocimiento cero (ZKP) Zeto para habilitar la confidencialidad de las transacciones, equilibrando privacidad y visibilidad PLD/FT. Las protecciones de privacidad deben extenderse a los metadatos de transacciones y no solo al contenido de las mismas.

**El rol institucional de los DCV en la infraestructura de efectivo tokenizado.** Los DCV y otras IMF pueden desempeñar un papel relevante en la implementación del tCeBM mayorista, particularmente en mercados donde ya operan procesos críticos de liquidación institucional. Su experiencia en gestión de participantes, reconciliación, trazabilidad, continuidad operativa, controles de acceso y coordinación con bancos liquidadores los posiciona como candidatos naturales para actuar como nodos operativos o infraestructuras de sincronización. Un enfoque pragmático implica representar los saldos de cuentas del banco central en forma tokenizada, con el DCV operando la capa técnica, funcional o de reconciliación bajo autorización explícita del banco central y con reglas claras de responsabilidad. El banco central retiene el pasivo monetario, la autoridad de emisión, las reglas de acceso y la capacidad supervisora; el DCV opera la capa de representación bajo supervisión del banco central. Este enfoque es particularmente útil en ALC porque permite comenzar con un caso de uso cerrado, mayorista y controlado sin lanzar un CBDC de propósito general, y puede construirse sobre los procesos de integración regional existentes entre bolsas, CCP, DCV y proveedores de tecnología.

**Gobernanza del Grupo de Trabajo CBWeb3.** El grupo opera bajo el marco de Linux Foundation Decentralized Trust, con co-presidentes nominados por el Centro de Estudios Monetarios Latinoamericanos (CEMLA) y el Fondo Latinoamericano de Reservas (FLAR). Las decisiones formales sin consenso se resuelven por GitVote (50%+1, siete días de comentarios). Este modelo equilibra apertura —cualquier institución puede contribuir— con responsabilidad.

---

## 11. Reflexiones Finales: Un Camino Gradual y Coordinado hacia Adelante

La evidencia de 2026 respalda continuar con experimentación controlada, no una decisión universal de desplegar. CBWeb3 ofrece una implementación de referencia y un flujo de interoperabilidad probado bajo condiciones acotadas de sandbox. Todavía no constituye un servicio productivo, no establece finalidad jurídica, no está integrado con LBTR productivos y no cuenta con un marco multinacional aprobado de responsabilidad ni con un modelo operativo sostenible. Cada autoridad debe avanzar solo cuando el problema medido, las alternativas disponibles y su tolerancia al riesgo justifiquen la siguiente etapa.

**Hoja de Ruta de Acción Resumida**

Los rangos de meses son bandas indicativas de planificación, no compromisos. Cada transición exige una decisión responsable de `avance`, `avance condicionado` o `no avance`, sustentada en evidencia.

*Fase 1 — Preparación Legal e Institucional (Meses 1–3)*

- Realizar evaluación legal interna con la lista de verificación de cinco dominios (Sección 4)
- Identificar brechas y determinar si requieren opiniones legales, guía regulatoria o cambios legislativos
- Designar un líder de proyecto interno y establecer un grupo de trabajo multifuncional (legal, tecnología, política monetaria)
- Iniciar contacto con el Grupo de Trabajo CBWeb3
- Realizar modelado de amenazas y revisión inicial de arquitectura de seguridad
- Documentar la decisión institucional formal de proceder, con criterios explícitos de avance o pausa hacia la Fase 2

*Fase 2 — Involucramiento Técnico y Conexión al Testnet (Meses 3–6)*

- Completar integración técnica con el testnet CBWeb3
- Ejecutar el tutorial de inicio rápido PvP de 15 minutos y validar conectividad técnica
- Definir el grupo cerrado de usuarios inicial (criterios en Sección 6)
- Encargar validación legal de acuerdos de participación, asignación de responsabilidades y ejecutabilidad de contratos inteligentes bajo la ley aplicable
- Comenzar diseño del modelo económico: ¿cómo se asignarán los costos de liquidez entre participantes? ¿Qué estructuras de incentivos apoyan la participación activa?
- Establecer el marco de medición de beneficios respecto a alternativas existentes (LBTR, corresponsalía bancaria, modernización ISO 20022)
- Documentar el modelo operativo: costos de operación y soporte 24/7, capacidad, renovación de infraestructura, política de tarifas, volumen esperado y punto de equilibrio
- Comparar CBWeb3 con modernización no tokenizada de LBTR/APIs, enfoques basados en SWIFT y rails privados antes de atribuir beneficios incrementales a la tokenización

*Fase 3 — Piloto de Grupo Cerrado (Meses 6–12)*

- Ejecutar el primer conjunto de transacciones de liquidación (interbancaria doméstica o a través de IMF, según corresponda) en el entorno del grupo cerrado
- Realizar pruebas de resiliencia operacional y ciberseguridad ante escenarios de estrés definidos
- Ejercitar rutas negativas: expiraciones, reversión, confirmaciones atascadas, pares no disponibles, reconciliación de emisiones pendientes y recuperación sin emisión duplicada
- Completar la evaluación post-piloto y compartir hallazgos con el Grupo de Trabajo CBWeb3
- Medir resultados contra el marco de medición de beneficios establecido en la Fase 2
- Establecer el marco de gobernanza del piloto: ¿quién decide las actualizaciones de protocolo? ¿Cómo se resuelven las disputas?
- Iniciar preparación legal para uso transfronterizo
- Decisión formal de avance o pausa para la Fase 4, basada en evidencia documentada de los resultados del piloto
- Tratar el límite observado de tres bancos centrales por lote como restricción del entorno de pruebas de 2026 y producir un plan medido de capacidad y escalamiento

*Fase 4 — Conectividad Transfronteriza (Meses 12–24)*

- Seleccionar el modelo de liquidación transfronteriza apropiado (Sección 8)
- Establecer conectividad bilateral o multilateral en sandbox según el modelo seleccionado; la participación mediante hub queda condicionada al cierre de las brechas de madurez del Escenario B
- Implementar el plan de transición de sandbox a valor real, incluyendo auditoría independiente de resultados del piloto, determinación de la ley aplicable para disputas multi-país, y plan de contingencia ante falla del hub o desconexión de un banco central con obligaciones activas pendientes
- Antes de cualquier prueba con valor real, aprobar custodia de llaves, alta disponibilidad, respaldo y recuperación, monitoreo funcional, runbooks de incidentes, reconciliación con LBTR/sistemas externos, notificación a participantes y asignación de responsabilidades y pérdidas
- Participar en el marco de gobernanza regional y contribuir a la evolución de la infraestructura CBWeb3 como Bien Público Digital (BPD)

El siguiente paso apropiado puede ser más trabajo de sandbox, un caso de uso más acotado, modernizar infraestructura existente o decidir no avanzar. CBWeb3 ofrece evidencia y componentes reutilizables para esa decisión; no debe predeterminar su resultado.

---

## 12. Registro de Riesgos

El siguiente registro identifica las principales categorías de riesgo asociadas con el despliegue de tCeBM en el contexto de ALC. Es un punto de partida para evaluaciones de riesgo jurisdiccionales, no un análisis exhaustivo.

| Categoría de Riesgo | Descripción | Impacto Potencial | Enfoque de Mitigación |
|---|---|---|---|
| **Concentración en el hub** | El riesgo sistémico concentrado en el hub regional crea un punto único de falla | Alto | Requisitos de redundancia; arquitectura de hub distribuida; estándares de resiliencia operacional |
| **Dependencia operador/tecnología** | Excesiva dependencia de un único operador o proveedor tecnológico | Medio | Estrategia multi-proveedor; componentes open-source; requisitos de portabilidad; plan de salida predefinido |
| **Fallas en contratos inteligentes** | Errores, fallas de lógica o exploits en los contratos inteligentes de liquidación | Alto | Verificación formal de lógica crítica; auditoría externa independiente; despliegue gradual con cortacircuitos |
| **Desalineación legal-técnica** | La finalidad de liquidación on-chain no reconocida como finalidad legal en una o más jurisdicciones | Alto | Opiniones legales pre-despliegue; actualización de marcos de finalidad de liquidación donde sea necesario |
| **Liquidez de monedas no convertibles** | Profundidad de mercado insuficiente para el intercambio AMM de pares de divisas ilíquidos | Medio | Requisitos de pre-fondeo para pares ilíquidos; cortacircuitos ante desviaciones de tipo de cambio; umbrales mínimos de liquidez |
| **Arbitraje regulatorio** | Participantes estructurando transacciones para explotar diferencias en regímenes PLD/FT entre jurisdicciones | Medio | Estándares mínimos PLD/FT comunes como condición de participación; acuerdos de intercambio de información entre supervisores |
| **Fuga de metadatos** | Los metadatos de transacciones revelan información sensible incluso cuando el contenido está protegido | Medio | Revisión de arquitectura de privacidad cubriendo metadatos; cobertura ZKP extendida a metadatos donde sea técnicamente factible |
| **Exclusión de actores pequeños** | Altos costos de integración que impiden la participación de instituciones financieras más pequeñas | Medio | APIs estándar open-source; entorno sandbox con soporte técnico; categorías de participación escalonadas |
| **Riesgo reputacional** | Daño a la percepción pública si el piloto falla o produce resultados adversos | Medio | Despliegue controlado; estrategia de comunicación clara; criterios de salida predefinidos |
| **Aceleración de la dolarización digital** | Si el diseño del tCeBM no es competitivo, la adopción de stablecoins puede acelerarse en lugar de reducirse | Medio-Alto | Análisis comparativo de diseño frente a stablecoins en USD; modelo de precios que no perjudique instrumentos en moneda local |
| **Incidentes de ciberseguridad** | Ataques a infraestructura de nodos, sistemas de gestión de claves o canales de comunicación | Alto | Cumplimiento de estándares de seguridad (ISO 27001 o equivalente); auditorías de seguridad periódicas; protocolos de respuesta a incidentes |
| **Desconexión del BC con obligaciones pendientes** | Un banco central desconectándose del hub mientras hay obligaciones de liquidación pendientes | Alto | Marco legal para desconexión ordenada; colateral pre-fondado; proceso probado de expiración/reversión; definir y aprobar el tratamiento y prioridad de obligaciones pendientes antes del uso con valor real |
| **Operaciones pendientes silenciosas y reconciliación** | Los saldos de reservas pueden cambiar mientras la emisión o confirmación de privacidad permanece pendiente por un periodo prolongado | Alto | Estado pendiente explícito; límites y alertas por antigüedad de cola; reconciliación por transacción; runbook seguro de no reemisión; notificación y confirmación de participantes |
| **Señales de salud falsamente positivas** | Puertos o contenedores aparecen disponibles aunque identidad, privacidad o confirmación entre instituciones no funcionen | Alto | Pruebas funcionales de preparación; controles entre pares; transacciones sintéticas extremo a extremo; alertas por agotamiento de conexiones y confirmaciones atascadas |
| **Desalineación de acceso y roles** | Las reglas de agente de liquidación, custodio, corresponsal o tenencia por no residentes son ambiguas o no se aplican técnicamente | Alto | Matriz aprobada de roles y acceso; aplicar reglas de no residentes en código y pruebas de conformidad; alinear implementación y acuerdos de participación |
| **Gobernanza de fuente FX y parámetros** | Fuentes manuales o no gobernadas de tipo de cambio pueden producir términos inconsistentes o manipulables | Medio-Alto | Fuente externa aprobada o proceso de oráculo controlado; controles de vigencia y desviación; procedimientos de contingencia y disputa; cambios de parámetros gobernados |
| **Integración con LBTR y registros externos** | Los saldos tokenizados del sandbox no están reconciliados con cuentas de reservas o LBTR productivos | Alto | Definir el registro autoritativo; diseño de adaptadores y reconciliación; reglas de falla y reversión; pruebas conjuntas con operaciones del banco central |
| **Brecha de rendimiento y capacidad** | El throughput, cupos de participantes u operaciones de privacidad de prueba pueden no cumplir volúmenes o niveles de servicio productivos | Alto | Evidencia de rendimiento versionada; estrés realista; modelo de capacidad; plan de escalamiento; SLO productivos y límites de admisión |
| **Brecha de gobernanza de evidencia y UAT** | Una entrega técnica o demostración exitosa puede confundirse con aceptación institucional o autorización productiva | Medio-Alto | Paquete de evidencia versionado; alcance y aprobación UAT por participantes; etiquetas explícitas de madurez; decisión independiente de autoridades responsables |

---

## 13. Evidencia y lecciones del sandbox/UAT 2026

La evidencia disponible de las pruebas con participantes en 2026 cambia la base sobre la cual debe discutirse CBWeb3. Lleva al Escenario A más allá de un diseño puramente conceptual y, al mismo tiempo, demuestra por qué un flujo técnico exitoso es solo una parte de la preparación institucional.

> **Nota de evidencia.** Esta sección sintetiza retroalimentación del BCRP del 7 de octubre de 2026, retroalimentación del BCCR del 30 de septiembre de 2026, un registro interno de la prueba conjunta Costa Rica–Perú, notas de coordinación DPGWG del 2 de octubre de 2026, revisiones de entregables de plataforma y la revisión del incidente de soporte del 6 de noviembre de 2026. Varias son fuentes internas del proyecto y no fuentes públicas ni auditadas independientemente; su publicación exige confirmación de las instituciones responsables y eliminación de cualquier detalle operativo restringido.

**Qué se observó.** Durante el primer lote de pruebas, el Banco Central de Reserva del Perú reportó haber completado en el sandbox el flujo PvP transfronterizo del Escenario A, incluyendo onboarding, emisión, tokenización de reservas, acuerdo FX, bloqueo HTLC, liquidación e interoperabilidad entre redes participantes. El Banco Central de Costa Rica indicó que el ejercicio permitió comprender de forma práctica los roles y el flujo transfronterizo. Se registró como exitoso un PvP Costa Rica–Perú operado por participantes. Estos resultados evidencian que el flujo del Escenario A puede operarse de extremo a extremo bajo la configuración probada.

**Qué aprendieron los participantes.** Las mayores fricciones de adopción incluyeron revisión interna de seguridad, aprovisionamiento de VPN y máquinas virtuales, permisos, credenciales, preparación del entorno y coordinación entre instituciones. Se solicitó aclarar los roles de agente de liquidación, custodio y corresponsal; las reglas para tenencia de tCeBM por no residentes; límites de saldo, remuneración, convertibilidad y redención; gobernanza de datos cambiarios; lineamientos KYC; fondeo e integración con LBTR; y alcance y recursos de una siguiente fase. BCCR también pidió escenarios de falla y excepción, mayor interacción entre participantes y una distinción clara entre capacidades de laboratorio y requisitos productivos.

**Evidencia operacional derivada de una falla.** Un incidente posterior en el entorno de homologación de Chile para el Escenario A mostró que la conectividad técnica no equivale a preparación funcional. Cinco intentos de tokenización por CLP 1.840.000 quedaron pendientes mientras los saldos de reservas ya habían cambiado. Tras recuperar los nodos afectados, las emisiones en cola se completaron sin reemisión y sin pérdida reportada. Sin embargo, el incidente reveló estados pendientes silenciosos prolongados, monitoreo funcional insuficiente, ausencia de escalamiento por antigüedad de cola y un problema serio de reconciliación. La evidencia disponible del incidente no incluyó un nuevo PvP exitoso posterior a la recuperación.

**Límites de la evidencia.** El registro disponible corresponde a retroalimentación de participantes y evidencia técnica de sandbox. No constituye una aceptación UAT institucional firmada, autorización productiva, prueba de finalidad jurídica, conexión con LBTR productivos ni evidencia de que funcionen todos los corredores y rutas de falla. La etiqueta `platform-v1.0.0` registra una línea base de entrega de software; no modifica esos límites de madurez. El Escenario B continúa siendo un prototipo experimental, según la Sección 8.

**Implicaciones para la siguiente fase.** Las pruebas futuras deben organizarse alrededor de evidencia y no solo de demostraciones. Como mínimo deben incluir flujos bidireccionales y negativos; reconciliación por transacción; monitoreo funcional y transacciones sintéticas extremo a extremo; manejo seguro de emisiones pendientes sin duplicarlas; análisis y notificación de impacto a todos los participantes; medición de rendimiento y capacidad; y revisión formal de resultados por los participantes. Cualquier transición a valor real queda condicionada a finalidad jurídica, integración con LBTR, custodia de llaves, alta disponibilidad, respaldo y recuperación, revisión independiente de seguridad, economía operativa y una asignación aprobada de responsabilidades y pérdidas.

---

# APPENDIX A — Glossary of Key Terms / APÉNDICE A — Glosario de Términos Clave

**AMM (Automated Market-Making / Creación de Mercado Automatizada):** An algorithmic mechanism that automatically determines exchange rates and routes transactions through a shared liquidity pool, without requiring pre-established bilateral agreements between each pair of participants. Used in Scenario B of the CBWeb3 platform. / Mecanismo algorítmico que determina automáticamente los tipos de cambio y enruta transacciones a través de un pool de liquidez compartido. Utilizado en el Escenario B de la plataforma CBWeb3.

**AML/CFT (Anti-Money Laundering / Counter-Terrorist Financing — PLD/FT: Prevención de Lavado de Dinero y Financiamiento del Terrorismo):** Regulatory frameworks requiring financial institutions to detect and prevent the use of financial systems for money laundering and terrorist financing. / Marcos regulatorios que requieren que las instituciones financieras detecten y prevengan el uso del sistema financiero para lavado de dinero y financiamiento del terrorismo.

**CBDC (Central Bank Digital Currency — Moneda Digital de Banco Central):** A digital form of central bank money, distinct from physical cash and from commercial bank deposits. Can be wholesale (accessible only to regulated financial institutions) or retail (accessible to the general public). / Forma digital del dinero del banco central, distinta del efectivo físico y de los depósitos bancarios comerciales. Puede ser mayorista (accesible solo a instituciones financieras reguladas) o minorista (accesible al público en general).

**CEMLA (Centro de Estudios Monetarios Latinoamericanos):** The Center for Latin American Monetary Studies, a regional body that coordinates monetary and financial research and serves as co-chair of the CBWeb3 Digital Public Good Working Group. / Centro de Estudios Monetarios Latinoamericanos, organismo regional que coordina la investigación monetaria y financiera y actúa como co-presidente del Grupo de Trabajo de Bien Público Digital de CBWeb3.

**CSD / DCV (Central Securities Depository — Depósito Central de Valores):** A financial market infrastructure that holds securities in dematerialized or immobilized form and enables securities settlement, typically through book-entry transfers. CSDs often maintain connections with central banks (for cash settlement) and with clearinghouses and custodians. / Infraestructura del mercado financiero que mantiene valores en forma desmaterializada o inmovilizada y habilita la liquidación de valores, típicamente mediante transferencias contables. Los DCV suelen mantener conexiones con bancos centrales (para liquidación de efectivo) y con cámaras de compensación y custodios.

**ECCB (Eastern Caribbean Central Bank — Banco Central del Caribe Oriental):** The central bank of the Eastern Caribbean Currency Union, issuer of the DCash digital currency — a regional CBDC precedent for LAC. / Banco central de la Unión Monetaria del Caribe Oriental, emisor de la moneda digital DCash — un precedente regional de CBDC para ALC.

**FLAR (Fondo Latinoamericano de Reservas — Latin American Reserve Fund):** A regional financial institution that manages foreign exchange reserves for member countries and serves as co-chair of the CBWeb3 Digital Public Good Working Group. / Institución financiera regional que administra las reservas de divisas de los países miembros y actúa como co-presidente del Grupo de Trabajo de Bien Público Digital de CBWeb3.

**FMI / IMF in financial context (Financial Market Infrastructure — Infraestructura del Mercado Financiero):** In this document, FMI refers to Financial Market Infrastructure, not the International Monetary Fund. FMIs include CSDs, clearinghouses (CCPs), central counterparties, and settlement system operators. / En este documento, IMF hace referencia a Infraestructura del Mercado Financiero, no al Fondo Monetario Internacional. Las IMF incluyen DCV, cámaras de compensación (CCP), contrapartes centrales y operadores de sistemas de liquidación.

**Fiat currency / Moneda fiat:** Legal tender issued by a government or central bank, not backed by a physical commodity. Its value derives from government decree and public trust. / Moneda de curso legal emitida por un gobierno o banco central, no respaldada por un bien físico. Su valor deriva de decreto gubernamental y confianza pública.

**GitVote:** A governance tool used in the CBWeb3 repository that enables on-chain voting on formal decisions, with a configurable approval threshold and comment period. / Herramienta de gobernanza utilizada en el repositorio CBWeb3 que habilita la votación en cadena sobre decisiones formales, con umbral de aprobación configurable y período de comentarios.

**HTLC (Hash Time-Lock Contract — Contrato de Bloqueo por Tiempo con Hash):** A cryptographic mechanism used in cross-border tokenized settlement that ensures atomicity: both legs of a transaction (debit in Currency A, credit in Currency B) settle simultaneously or neither settles, eliminating principal risk. / Mecanismo criptográfico utilizado en la liquidación transfronteriza tokenizada que garantiza atomicidad: ambas piernas de una transacción se liquidan simultáneamente o ninguna lo hace, eliminando el riesgo de principal.

**ISO 20022:** An international standard for electronic data interchange between financial institutions, used for payment message formatting. The CBWeb3 platform recommends ISO 20022 for interbank settlement messages in the initial closed group phase. / Estándar internacional para el intercambio de datos electrónicos entre instituciones financieras, utilizado para el formato de mensajes de pago. La plataforma CBWeb3 recomienda ISO 20022 para los mensajes de liquidación interbancaria en la fase inicial del grupo cerrado.

**LAC / ALC (Latin America and the Caribbean — América Latina y el Caribe):** The geographic and political region encompassing Spanish-, Portuguese-, French-, and Dutch-speaking countries of the Americas south of the United States, plus the Caribbean island nations. / La región geográfica y política que abarca los países de habla española, portuguesa, francesa y neerlandesa de las Américas al sur de los Estados Unidos, más las naciones insulares del Caribe.

**LGPD (Lei Geral de Proteção de Dados):** Brazil's General Data Protection Law, enacted in 2018, regulating the processing of personal data in Brazil. / Ley General de Protección de Datos de Brasil, promulgada en 2018, que regula el tratamiento de datos personales en Brasil.

**LIFT Lab:** A financial innovation initiative of the Banco Central do Brasil used to develop and test the Drex wholesale CBDC. / Iniciativa de innovación financiera del Banco Central do Brasil utilizada para desarrollar y probar el CBDC mayorista Drex.

**Nuam:** The operator of stock exchanges and CCPs/CSDs across Colombia, Chile, and Peru, active in the CBWeb3 Working Group and advancing regional post-trade integration. / Operador de bolsas de valores y CCP/DCV en Colombia, Chile y Perú, activo en el Grupo de Trabajo CBWeb3 y avanzando en la integración post-negociación regional.

**Plug-and-play:** A design principle where a component or system can be connected to and used by other systems with minimal configuration or integration effort. / Principio de diseño donde un componente o sistema puede conectarse y ser utilizado por otros sistemas con mínimo esfuerzo de configuración o integración.

**PvP (Payment versus Payment — Pago contra Pago):** A settlement mechanism where the transfer of one currency is conditional on the simultaneous transfer of another currency, eliminating the risk that one party delivers without receiving. / Mecanismo de liquidación donde la transferencia de una moneda es condicional a la transferencia simultánea de otra moneda, eliminando el riesgo de que una parte entregue sin recibir.

**RTGS / LBTR (Real-Time Gross Settlement — Liquidación Bruta en Tiempo Real):** A payment system in which settlement of funds or securities occurs individually, on an order-by-order basis, in real time. / Sistema de pago en el que la liquidación de fondos o valores ocurre de forma individual, orden por orden, en tiempo real.

**SBS (Superintendencia de Banca, Seguros y AFP):** Peru's financial supervisor responsible for regulating and supervising banks, insurance companies, and pension fund administrators. / Supervisor financiero del Perú responsable de regular y supervisar bancos, compañías de seguros y administradoras de fondos de pensiones.

**Spoke network / Red Spoke:** In the CBWeb3 hub-and-spoke architecture, a spoke is the domestic tokenized settlement network of an individual country's central bank. Each spoke connects to the shared regional hub for cross-border transactions while remaining sovereign for domestic operations. / En la arquitectura hub-and-spoke de CBWeb3, un spoke es la red de liquidación tokenizada doméstica del banco central de un país individual.

**tCeBM (Tokenized Central Bank Money — Dinero Digital de Banco Central Tokenizado):** A digital representation of a central bank's monetary liability, issued on distributed ledger or programmable infrastructure. See Section 3 for a full discussion of the forms of digital money. / Representación digital del pasivo monetario de un banco central, emitida en libro mayor distribuido o infraestructura programable. Ver Sección 3 para una discusión completa de las formas de dinero digital.

**tDeposits (Tokenized Commercial Bank Deposits — Depósitos Bancarios Tokenizados):** Digital representations of claims on commercial banks, issued on programmable infrastructure. Distinct from tCeBM because the credit risk is that of the commercial bank, not the central bank. / Representaciones digitales de derechos frente a bancos comerciales, emitidas en infraestructura programable. Distintos del tCeBM porque el riesgo de crédito es el del banco comercial, no del banco central.

**Token / Tokenization / Token / Tokenización:** A "token" is a digital unit of value or rights recorded on a programmable ledger. "Tokenization" is the process of converting a real-world asset or financial instrument into a digital token that can be transferred, settled, or programmed on such a ledger. / Un "token" es una unidad digital de valor o derechos registrada en un libro mayor programable. La "tokenización" es el proceso de convertir un activo del mundo real o instrumento financiero en un token digital.

**Zero-knowledge proof / ZKP (Prueba de conocimiento cero):** A cryptographic technique that allows one party to prove the truth of a statement without revealing any information beyond the validity of the statement itself. Used in the CBWeb3 platform via the Zeto framework to enable transaction confidentiality while preserving verifiability. / Técnica criptográfica que permite a una parte demostrar la veracidad de una afirmación sin revelar información más allá de la validez de la afirmación en sí. Utilizada en la plataforma CBWeb3 a través del marco Zeto para habilitar la confidencialidad de transacciones preservando la verificabilidad.

---

*Fin del documento — Knowledge Product 1, Draft v0.2*
*Prepared by CBWeb3 Working Group / LNET — 2026-07-10*
*For review and comment: open a thread on GitHub issue [#46](https://github.com/LF-Decentralized-Trust-labs/cbweb3/issues/46)*
