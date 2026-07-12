# Knowledge Product 1 — Draft v0.2
## Tokenized Central Bank Money in LAC: From Issuance to Cross-Border Integration
## Dinero Digital de Banco Central en ALC: De la Emisión a la Integración Transfronteriza

**Status:** Draft for working group review
**Version:** v0.2 — 2026-07-10
**Author:** CBWeb3 Working Group / LNET
**GitHub issue:** [#46 — Knowledge Product 1 Proposed Structure](https://github.com/LF-Decentralized-Trust-labs/cbweb3/issues/46)
**Audience:** Central bankers, policymakers, and broad financial sector stakeholders

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

The practical conclusion of this document is that the conditions for action are already in place. Countries do not need to wait for perfect legal certainty, fully tested technology, or regional consensus before beginning. A deliberate, gradual approach — starting with a closed group of regulated participants in a controlled domestic use case — allows institutions to learn, build capacity, and reduce risks before expanding scope. The CBWeb3 regional testnet exists precisely to support this experimentation phase, providing shared infrastructure, interoperability tooling, and a community of practice across central banks and financial institutions in LAC.

---

## 2. Why Tokenized Central Bank Money Matters for LAC Now

The case for tCeBM in Latin America and the Caribbean is not built on technology enthusiasm. It is built on a specific set of structural pressures that are already reshaping the regional financial landscape, and that demand a policy response.

**The correspondent banking problem.** The number of active correspondent banking relationships in LAC has declined steadily over the past decade, driven by de-risking strategies among global banks [Source: TBD — BIS correspondent banking surveys]. For smaller economies and for specific transaction categories — particularly remittances and trade finance — this contraction has raised costs, lengthened settlement times, and in some corridors, effectively eliminated access to affordable international payment infrastructure. Tokenized central bank money, used as a settlement asset in a direct or semi-direct model, can reduce dependence on correspondent intermediaries by enabling payment-versus-payment (PvP) settlement with cryptographic finality.

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

> **Note on CBWeb3 testnet scenarios:** Throughout this section, "Scenario A" refers to the HTLC-based PvP settlement model implemented on the CBWeb3 testnet, enabling atomic bilateral settlement between two participants. "Scenario B" refers to the AMM-based pooled liquidity model, enabling multilateral settlement without pre-established bilateral arrangements. Both are live on the CBWeb3 testnet and available for technical validation by participating institutions.

Four practical models exist for cross-border tokenized settlement, each with different implications for complexity, scalability, and the degree of regional coordination required.

**Model 1 — Tokenized Correspondent Banking (Enhanced PvP)**

In this model, the bilateral relationship between a domestic bank in Country A and its correspondent bank in Country B is replicated on tokenized infrastructure. Each leg of the payment — the debit in Country A's currency and the credit in Country B's currency — is settled atomically using an HTLC. Settlement finality in both legs is conditional: neither leg settles unless both settle simultaneously, eliminating the principal risk that exists in traditional correspondent banking. Scenario A of the CBWeb3 platform implements exactly this model and has been tested end-to-end.

*Advantages:* Builds on existing correspondent relationships and legal frameworks. No requirement for a shared regional infrastructure or centralized liquidity. Can be deployed bilaterally, one corridor at a time, without requiring multilateral coordination.

*Disadvantages:* Scales linearly with the number of corridors — N×(N-1)/2 bilateral agreements for full regional connectivity. Does not solve the liquidity pre-funding problem: each correspondent still needs to hold pre-funded balances in both currencies. Limited to currency pairs where a bilateral correspondent relationship already exists or can be established.

*Requirements:* HTLC-capable smart contract infrastructure at both ends; legal recognition of HTLC-based finality in both jurisdictions; bilateral participation agreement between institutions.

**Model 2 — Bilateral Corridors with Shared Settlement**

In this model, two central banks establish a direct, bilateral settlement arrangement using tokenized central bank money, bypassing the commercial correspondent layer entirely. The arrangement is governed by a bilateral agreement between the two monetary authorities and uses a shared settlement infrastructure — potentially the CBWeb3 hub — as the technical intermediary.

*Advantages:* Eliminates commercial bank intermediation from the interbank settlement layer, reducing cost and risk. Provides central banks with direct visibility into cross-border settlement flows. Stronger foundation for monetary policy coordination between jurisdictions.

*Disadvantages:* Requires bilateral treaty or memorandum of understanding between central banks — a process that can be lengthy and politically complex. Still scales as a mesh of bilateral agreements, though more slowly than commercial bilateral corridors. Requires both central banks to have sufficient technology maturity to connect to shared infrastructure. Raises questions about applicable law in the event of disputes and about the treatment of pending obligations if one central bank disconnects.

*Requirements:* Formal bilateral legal framework between monetary authorities; definition of governing law for cross-border disputes; collateral or pre-funded balance requirements to manage settlement risk during connectivity interruptions.

**Model 3 — Shared Regional Platform**

In this model, participating countries connect to a single, multilateral settlement hub that supports multi-currency settlement without requiring pre-established bilateral arrangements between every pair of participants. The hub provides liquidity pooling, automated market-making for FX conversion, and a single legal and governance framework for all participants. Scenario B of the CBWeb3 platform implements this model using an AMM algorithm.

*Advantages:* Maximum scalability — N participants can settle with each other without N×(N-1)/2 bilateral agreements. Most efficient use of liquidity — pooled liquidity reduces pre-funding requirements significantly. Enables new participants to join incrementally, with immediate connectivity to all existing members.

*Disadvantages:* Highest coordination requirement — establishing a multilateral governance framework, allocating costs, and defining liability arrangements across multiple sovereign jurisdictions is complex. Requires a trusted, neutral operator for the hub infrastructure. Systemic risk is concentrated at the hub; robust technical and governance resilience is essential. AMM-based exchange rates may deviate from market rates for illiquid currency pairs.

*Requirements:* Multilateral legal framework; neutral hub operator with appropriate governance; AMM parameter governance process; circuit breakers and liquidity thresholds for non-convertible currency pairs; hub redundancy and business continuity standards.

**Model 4 — Regional CSD Interconnection for Tokenized Cash**

In this model, the CSDs of participating countries act as authorized intermediary infrastructures that operate, synchronize, or facilitate movements of tCeBM backed by central bank account balances. Each central bank retains the issuance authority and monetary liability; the CSDs contribute regional connectivity, operational standards, participant management, reconciliation capacity, and integration with existing institutional market processes. This model is particularly relevant where CSDs are already involved in cross-border post-trade integration and where the primary use case involves institutional settlement of securities rather than general interbank payments.

*Advantages:* Leverages existing infrastructure and established relationships between CSDs and market participants. Reduces duplication of operational investment. Facilitates adoption because participants are already connected to their domestic CSD. Strengthens regional integration through institutions with track records in cross-jurisdictional coordination. Enables controlled, institutional pilots without retail access considerations. Directly supports capital markets post-trade efficiency as a concrete and immediate use case.

*Disadvantages:* Requires clear agreements on liability, supervision, cybersecurity, operational continuity, reconciliation with central bank accounts, applicable law, error handling, and participant disconnection procedures. CSD governance structures may not be designed for central bank monetary operations, creating potential conflicts of accountability. Not all LAC jurisdictions have sufficiently mature CSDs to serve this function. Risk of operational fragmentation if each CSD develops incompatible connectivity standards.

*Requirements:* Central bank authorization for each CSD to operate tCeBM-related functions; formal agreement on liability allocation between central banks and CSDs; common connectivity and messaging standards between CSDs; coordinated supervisory oversight between central bank and securities regulator.

*Best suited for:* Jurisdictions where CSDs are already involved in regional integration processes and have established relationships with central banks and banking supervisors. The Colombia-Chile-Peru corridor, given the Nuam integration across exchanges, CCPs, and CSDs in these three markets, is the most immediate candidate for this model.

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

**An additional approach for reducing pre-funding requirements** is to leverage existing regional FMIs — including interconnected CSDs — as liquidity coordination mechanisms for authorized participants. Under this arrangement, CSDs can provide consolidated visibility of positions across participants in different jurisdictions, facilitate intraday reconciliation processes, and act as operational connection points between participants. This approach can improve the efficiency of cross-border liquidity use and reduce the need for multiple idle balances held across different markets simultaneously, building on the CSD's existing role as a hub for institutional participant connectivity.

**Recommended approach.** For the initial phase of regional deployment, pre-funded bilateral balances (Mechanism 1) are recommended: they are operationally simple, eliminate settlement risk, and require no multilateral governance. As the network grows and liquidity management costs become significant, a transition to pooled liquidity with AMM (Mechanism 3) becomes attractive — but only after the AMM governance framework has been established and the algorithm has been validated in production-equivalent conditions. Intraday credit facilities (Mechanism 2) may be appropriate for specific high-volume bilateral corridors between jurisdictions with strong bilateral institutional relationships.

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

The analysis presented in this document leads to a clear conclusion: LAC central banks and financial authorities have both the motivation and the means to act on tokenized central bank money. The motivation is structural — the pressures of correspondent banking decline, stablecoin expansion, and cross-border payment inefficiency are not going away, and they will intensify as digital financial infrastructure becomes more central to global commerce. The means are available — the CBWeb3 regional testnet provides tested infrastructure, proven interoperability protocols, and a governance framework designed for the institutional context of LAC.

The path forward does not require all countries to move at the same pace, nor does it require full legal certainty before taking the first step. What it requires is clarity of intention, institutional commitment, and a willingness to learn from controlled experimentation.

**Summary Action Roadmap**

The following roadmap organizes the key actions by phase, providing a structured path from initial assessment to regional integration. Phase transitions are conditioned on explicit go/no-go decisions to ensure that scaling happens on the basis of evidence, not momentum.

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
- Participate in at least one bi-weekly community call to engage with the technical working group
- Define the initial closed user group according to the eligibility criteria in Section 6
- Commission legal validation of participation agreements, liability allocation, and smart contract enforceability under applicable law
- Begin design of the economic model: how will liquidity costs be allocated among participants? What incentive structures support active participation?
- Establish the benefit measurement framework: what metrics will be used to assess value creation relative to existing alternatives (RTGS, correspondent banking, ISO 20022 modernization)?

*Phase 3 — Closed Group Pilot (Months 6–12)*

- Execute the first set of domestic interbank settlement transactions (or institutional FMI-based settlement, as applicable) in the closed group environment
- Conduct operational resilience and cybersecurity testing against defined stress scenarios, including participant disconnection and hub failure
- Complete post-pilot assessment and share findings with the CBWeb3 Working Group
- Measure outcomes against the benefit measurement framework established in Phase 2
- Establish a pilot governance framework: who decides on protocol upgrades? How are disputes resolved? What is the process for participant removal?
- Begin legal preparation for cross-border use: identify counterpart jurisdictions, initiate bilateral legal review, define participation agreement terms
- Formal go/no-go decision for Phase 4, based on documented evidence from pilot results and operational testing

*Phase 4 — Cross-Border Connectivity (Months 12–24)*

- Select the cross-border settlement model appropriate to your jurisdiction's context (Section 8)
- Establish bilateral or multilateral connectivity through the CBWeb3 hub
- Implement the sandbox-to-real-value transition plan, including independent audit of pilot results, governing law determination for multi-country disputes, and contingency plan for hub failure or CB disconnection during active obligations
- Participate in the regional governance framework and contribute to the evolution of the CBWeb3 Digital Public Good infrastructure

Central banks that begin this process now — even at a modest scale — will be better positioned to shape the regional architecture than those that wait for a consensus that may never fully arrive. The CBWeb3 initiative is an invitation to participate in building shared infrastructure for a regional financial future that serves the priorities of Latin America and the Caribbean.

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
| **CB disconnection mid-obligation** | A central bank disconnecting from the hub while settlement obligations are pending, leaving counterparts with unresolved positions | High — counterparts bear principal risk if disconnection is not handled in a defined protocol | Legal framework for orderly disconnection; pre-funded collateral requirements; automatic settlement finalization protocol triggered by disconnection event; defined priority rules for pending obligations |

---

---

# VERSIÓN EN ESPAÑOL

---

## 1. Resumen Ejecutivo: Un Marco de Decisión para las Autoridades de ALC

América Latina y el Caribe (ALC) se encuentran en un momento decisivo en la evolución del dinero. Los pagos transfronterizos en la región siguen siendo de los más costosos y lentos del mundo. Las relaciones de corresponsalía bancaria se están reduciendo. Las stablecoins y las monedas digitales extranjeras ganan terreno en economías dolarizadas o semi-dolarizadas, erosionando el espacio de política disponible para los bancos centrales. En este contexto, el dinero digital de banco central (tCeBM, por sus siglas en inglés: *tokenized Central Bank Money*) ha emergido no como un experimento tecnológico especulativo, sino como un instrumento de política concreto que varias autoridades regionales ya están explorando seriamente.

Este documento está dirigido a los tomadores de decisiones que deben determinar si involucrarse con esta transformación y cómo hacerlo. Se organiza en torno a las preguntas prácticas que enfrentan los altos funcionarios: ¿Qué se está emitiendo exactamente? ¿Está listo nuestro marco legal? ¿Qué modelo operativo se ajusta a nuestro contexto institucional? ¿Dónde empezamos y cómo nos conectamos con la región?

El marco de decisión que propone este documento tiene cuatro dimensiones. Primero, **la pregunta sobre el activo de liquidación**: si emitir tCeBM a nivel mayorista, minorista o ambos, y cómo posicionarlo respecto a los depósitos bancarios comerciales tokenizados y los instrumentos digitales privados. Segundo, **la pregunta sobre el modelo operativo**: si el banco central lidera la infraestructura, delega en intermediarios o adopta un esquema híbrido. Tercero, **la pregunta sobre la integración transfronteriza**: si perseguir corredores bilaterales, unirse a una plataforma regional compartida o modernizar las relaciones de corresponsalía bancaria. Cuarto, **la pregunta sobre la infraestructura de liquidación**: si el tCeBM será operado exclusivamente por el banco central, por bancos comerciales, o a través de infraestructuras del mercado financiero (IMF) autorizadas —como depósitos centrales de valores (DCV), cámaras de compensación u operadores de sistemas de liquidación—. Esta cuarta dimensión determina quién construye y opera la capa operacional, quién es responsable por fallas de liquidación y cómo el tCeBM se conecta con procesos de mercado existentes como la compensación de valores, la reconciliación y la custodia.

La conclusión práctica de este documento es que las condiciones para actuar ya están dadas. Los países no necesitan esperar certeza jurídica perfecta, tecnología completamente probada ni consenso regional antes de comenzar. Un enfoque deliberado y gradual —comenzando con un grupo cerrado de participantes regulados en un caso de uso doméstico controlado— permite a las instituciones aprender, desarrollar capacidades y reducir riesgos antes de ampliar el alcance. La red de pruebas regional CBWeb3 existe precisamente para apoyar esta fase de experimentación, brindando infraestructura compartida, herramientas de interoperabilidad y una comunidad de práctica entre bancos centrales e instituciones financieras de ALC.

---

## 2. Por Qué el Dinero Digital de Banco Central Importa para ALC Ahora

El caso a favor del tCeBM en América Latina y el Caribe no se construye sobre el entusiasmo tecnológico. Se construye sobre un conjunto específico de presiones estructurales que ya están redefiniendo el panorama financiero regional y que exigen una respuesta de política.

**El problema de la corresponsalía bancaria.** El número de relaciones de corresponsalía bancaria activas en ALC ha disminuido sostenidamente durante la última década, impulsado por estrategias de reducción de riesgo de los bancos globales [Fuente: por confirmar — encuestas BIS sobre corresponsalía bancaria]. Para las economías más pequeñas y para ciertas categorías de transacciones —particularmente remesas y financiamiento de comercio exterior— esta contracción ha elevado los costos, alargado los tiempos de liquidación y, en algunos corredores, ha eliminado efectivamente el acceso a infraestructura de pagos internacionales asequible. El tCeBM, utilizado como activo de liquidación en un modelo directo o semi-directo, puede reducir la dependencia de intermediarios de corresponsalía al habilitar la liquidación pago-contra-pago (PvP) con finalidad criptográfica.

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

**Criterios de éxito.** La fase del grupo cerrado debe considerarse completa cuando: (a) al menos dos instituciones hayan completado la liquidación extremo a extremo con finalidad criptográfica; (b) el banco central haya validado el libro mayor de liquidación; y (c) se haya completado una evaluación post-piloto compartida con todos los participantes.

**Cronograma.** Un cronograma realista es de seis a doce meses desde la decisión hasta la finalización exitosa de la fase del grupo cerrado.

---

## 7. De la Emisión Doméstica a la Utilidad Real

Un banco central que haya completado con éxito un piloto de emisión doméstica ha logrado algo significativo — pero aún no ha desbloqueado la fuente más importante de valor del dinero tokenizado. El potencial transformador emerge cuando el tCeBM se vuelve interoperable: cuando puede utilizarse para liquidar transacciones transfronterizas, cuando la liquidez puede fluir entre jurisdicciones sin múltiples rondas de conversión de divisas y cuando las instituciones financieras de diferentes países pueden transaccionar entre sí utilizando un protocolo de liquidación compartido.

La plataforma CBWeb3 está diseñada para apoyar precisamente esta transición. Su arquitectura hub-and-spoke permite que el banco central de cada país mantenga plena soberanía sobre su emisión y liquidación doméstica mientras se conecta a un hub transnacional compartido para transacciones transfronterizas.

---

## 8. Modelos de Liquidación Transfronteriza para ALC

> **Nota sobre los escenarios del testnet CBWeb3:** A lo largo de esta sección, "Escenario A" hace referencia al modelo de liquidación PvP basado en HTLC implementado en el testnet CBWeb3, que habilita la liquidación bilateral atómica entre dos participantes. "Escenario B" hace referencia al modelo de liquidez agrupada basado en AMM, que habilita la liquidación multilateral sin acuerdos bilaterales preestablecidos. Ambos están disponibles en el testnet CBWeb3 para validación técnica por parte de las instituciones participantes.

Existen cuatro modelos prácticos para la liquidación transfronteriza tokenizada.

**Modelo 1 — Corresponsalía Bancaria Tokenizada (PvP Mejorado)**

La relación bilateral entre un banco doméstico y su banco corresponsal se replica en infraestructura tokenizada, con liquidación atómica mediante HTLC. El Escenario A de CBWeb3 implementa este modelo y ha sido probado de extremo a extremo.

*Ventajas:* Construye sobre relaciones existentes. Sin necesidad de infraestructura regional compartida. Despliegue bilateral, corredor por corredor.

*Desventajas:* Escala linealmente. No resuelve el pre-fondeo de liquidez. Limitado a pares de monedas con corresponsalía existente.

*Requerimientos:* Infraestructura de contratos inteligentes compatible con HTLC en ambos extremos; reconocimiento legal de la finalidad HTLC en ambas jurisdicciones.

**Modelo 2 — Corredores Bilaterales con Liquidación Compartida**

Dos bancos centrales establecen un acuerdo de liquidación bilateral directo, omitiendo completamente la capa de corresponsalía comercial.

*Ventajas:* Elimina la intermediación comercial. Visibilidad directa de los flujos transfronterizos. Base más sólida para la coordinación de política monetaria.

*Desventajas:* Requiere tratado o MOU entre bancos centrales. Proceso prolongado. Plantea preguntas sobre la ley aplicable en disputas y el tratamiento de obligaciones pendientes si un banco central se desconecta.

*Requerimientos:* Marco legal bilateral formal; definición de ley aplicable para disputas transfronterizas; requisitos de colateral o saldo pre-fondado para gestionar el riesgo durante interrupciones de conectividad.

**Modelo 3 — Plataforma Regional Compartida**

Los países participantes se conectan a un único hub de liquidación multilateral con creación de mercado automatizada. El Escenario B de CBWeb3 implementa este modelo mediante un algoritmo AMM.

*Ventajas:* Máxima escalabilidad. Uso más eficiente de la liquidez. Nuevos participantes se incorporan con conectividad inmediata con todos los miembros.

*Desventajas:* Mayor requerimiento de coordinación multilateral. Riesgo sistémico concentrado en el hub. Los tipos de cambio AMM pueden desviarse de los precios de mercado para pares de divisas poco líquidos.

*Requerimientos:* Marco legal multilateral; operador del hub neutral con gobernanza apropiada; proceso de gobernanza de parámetros AMM; cortacircuitos y umbrales de liquidez para pares de monedas no convertibles.

**Modelo 4 — Interconexión Regional de DCV para Efectivo Tokenizado**

En este modelo, los DCV de los países participantes actúan como infraestructuras intermediarias autorizadas que operan, sincronizan o facilitan movimientos de tCeBM respaldado por saldos de cuentas del banco central. Cada banco central retiene la autoridad de emisión y el pasivo monetario; los DCV aportan conectividad regional, estándares operativos, gestión de participantes, capacidad de reconciliación e integración con los procesos institucionales de mercado existentes.

*Ventajas:* Aprovecha infraestructura existente y relaciones establecidas entre DCV y participantes del mercado. Reduce la duplicación de inversión operacional. Facilita la adopción porque los participantes ya están conectados a su DCV doméstico. Fortalece la integración regional a través de instituciones con trayectoria en coordinación transfronteriza. Habilita pilotos controlados e institucionales sin consideraciones de acceso retail. Apoya directamente la eficiencia post-negociación de los mercados de capitales como caso de uso concreto e inmediato.

*Desventajas:* Requiere acuerdos claros sobre responsabilidad, supervisión, ciberseguridad, continuidad operativa, reconciliación con cuentas del banco central, ley aplicable, manejo de errores y procedimientos de desconexión de participantes. Las estructuras de gobernanza de los DCV pueden no estar diseñadas para operaciones monetarias del banco central. No todos los DCV de ALC tienen suficiente madurez para cumplir esta función.

*Requerimientos:* Autorización del banco central para cada DCV; acuerdo formal sobre asignación de responsabilidades; estándares de conectividad y mensajería comunes entre DCV; supervisión coordinada entre el banco central y el regulador de valores.

*Más adecuado para:* El corredor Colombia-Chile-Perú, dada la integración Nuam entre bolsas, CCP y DCV en estos tres mercados, es el candidato más inmediato para este modelo.

**Coexistencia en la práctica.** Estos cuatro modelos no son mutuamente excluyentes y la arquitectura regional probablemente verá los cuatro en uso simultáneo: corresponsalía mejorada para corredores con relaciones comerciales existentes, corredores bilaterales directos para los pares de mayor volumen, una plataforma regional compartida como columna vertebral multilateral, e interconexión de DCV para flujos institucionales de mercados de capitales.

---

## 9. Gestión de Liquidez para Monedas No Convertibles

El desafío de gestión de liquidez en un sistema de liquidación tokenizado multi-moneda es agudo para ALC, donde muchas monedas no son libremente convertibles.

**Mecanismo 1 — Saldos Bilaterales Pre-fondados** (Escenario A / HTLC): Simple y de bajo riesgo, pero inmoviliza capital significativo en todos los corredores activos.

**Mecanismo 2 — Líneas de Crédito Bilaterales**: Reduce el pre-fondeo pero introduce riesgo crediticio y complejidad institucional. Análogo a las facilidades de crédito intradía que los bancos centrales proveen en los sistemas tradicionales de LBTR (Liquidación Bruta en Tiempo Real).

**Mecanismo 3 — Liquidez Agrupada con AMM** (Escenario B): Más eficiente en capital, pero los tipos de cambio algorítmicos pueden desviarse de precios de mercado para pares poco líquidos. La gobernanza del pool requiere un diseño institucional cuidadoso.

**Un enfoque adicional para reducir los requerimientos de pre-fondeo** es aprovechar las IMF regionales existentes —incluyendo DCV interconectados— como mecanismos de coordinación de liquidez para los participantes autorizados. Bajo este esquema, los DCV pueden proporcionar visibilidad consolidada de posiciones entre participantes de distintas jurisdicciones, facilitar procesos de reconciliación intradía y actuar como puntos de conexión operativa. Este enfoque puede mejorar la eficiencia en el uso de la liquidez transfronteriza y reducir la necesidad de mantener múltiples saldos ociosos en distintos mercados simultáneamente.

**Enfoque recomendado.** Para la fase inicial, se recomiendan los saldos bilaterales pre-fondados. La transición al AMM resulta atractiva a medida que la red crece, pero solo después de que el marco de gobernanza esté establecido y el algoritmo validado en condiciones equivalentes a producción.

---

## 10. Consideraciones de Política y Enfoques de Gobernanza

**Soberanía monetaria.** La arquitectura CBWeb3 mantiene la red spoke de cada país como completamente soberana — el banco central controla sus propios parámetros de emisión y puede desconectarse del hub regional en cualquier momento sin afectar sus operaciones domésticas de liquidación.

**Estabilidad financiera.** La mayoría de las jurisdicciones de ALC están enfocadas en aplicaciones mayoristas, lo que limita sustancialmente el riesgo de desintermediación bancaria.

**Privacidad.** La plataforma CBWeb3 utiliza el marco de privacidad Hyperledger Paladin con pruebas de conocimiento cero (ZKP) Zeto para habilitar la confidencialidad de las transacciones, equilibrando privacidad y visibilidad PLD/FT. Las protecciones de privacidad deben extenderse a los metadatos de transacciones y no solo al contenido de las mismas.

**El rol institucional de los DCV en la infraestructura de efectivo tokenizado.** Los DCV y otras IMF pueden desempeñar un papel relevante en la implementación del tCeBM mayorista, particularmente en mercados donde ya operan procesos críticos de liquidación institucional. Su experiencia en gestión de participantes, reconciliación, trazabilidad, continuidad operativa, controles de acceso y coordinación con bancos liquidadores los posiciona como candidatos naturales para actuar como nodos operativos o infraestructuras de sincronización. Un enfoque pragmático implica representar los saldos de cuentas del banco central en forma tokenizada, con el DCV operando la capa técnica, funcional o de reconciliación bajo autorización explícita del banco central y con reglas claras de responsabilidad. El banco central retiene el pasivo monetario, la autoridad de emisión, las reglas de acceso y la capacidad supervisora; el DCV opera la capa de representación bajo supervisión del banco central. Este enfoque es particularmente útil en ALC porque permite comenzar con un caso de uso cerrado, mayorista y controlado sin lanzar un CBDC de propósito general, y puede construirse sobre los procesos de integración regional existentes entre bolsas, CCP, DCV y proveedores de tecnología.

**Gobernanza del Grupo de Trabajo CBWeb3.** El grupo opera bajo el marco de Linux Foundation Decentralized Trust, con co-presidentes nominados por el Centro de Estudios Monetarios Latinoamericanos (CEMLA) y el Fondo Latinoamericano de Reservas (FLAR). Las decisiones formales sin consenso se resuelven por GitVote (50%+1, siete días de comentarios). Este modelo equilibra apertura —cualquier institución puede contribuir— con responsabilidad.

---

## 11. Reflexiones Finales: Un Camino Gradual y Coordinado hacia Adelante

Los bancos centrales y las autoridades financieras de ALC tienen tanto la motivación como los medios para actuar sobre el tCeBM. El camino hacia adelante no requiere que todos los países avancen al mismo ritmo ni plena certeza legal antes de dar el primer paso.

**Hoja de Ruta de Acción Resumida**

Las transiciones de fase están condicionadas a decisiones explícitas de avance o pausa (go/no-go) para asegurar que el escalamiento ocurra sobre la base de evidencia, no de inercia.

*Fase 1 — Preparación Legal e Institucional (Meses 1–3)*

- Realizar evaluación legal interna con la lista de verificación de cinco dominios (Sección 4)
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

*Fase 3 — Piloto de Grupo Cerrado (Meses 6–12)*

- Ejecutar el primer conjunto de transacciones de liquidación (interbancaria doméstica o a través de IMF, según corresponda) en el entorno del grupo cerrado
- Realizar pruebas de resiliencia operacional y ciberseguridad ante escenarios de estrés definidos
- Completar la evaluación post-piloto y compartir hallazgos con el Grupo de Trabajo CBWeb3
- Medir resultados contra el marco de medición de beneficios establecido en la Fase 2
- Establecer el marco de gobernanza del piloto: ¿quién decide las actualizaciones de protocolo? ¿Cómo se resuelven las disputas?
- Iniciar preparación legal para uso transfronterizo
- Decisión formal de avance o pausa para la Fase 4, basada en evidencia documentada de los resultados del piloto

*Fase 4 — Conectividad Transfronteriza (Meses 12–24)*

- Seleccionar el modelo de liquidación transfronteriza apropiado (Sección 8)
- Establecer conectividad a través del hub CBWeb3
- Implementar el plan de transición de sandbox a valor real, incluyendo auditoría independiente de resultados del piloto, determinación de la ley aplicable para disputas multi-país, y plan de contingencia ante falla del hub o desconexión de un banco central con obligaciones activas pendientes
- Participar en el marco de gobernanza regional y contribuir a la evolución de la infraestructura CBWeb3 como Bien Público Digital (BPD)

Los bancos centrales que comiencen este proceso ahora estarán mejor posicionados para dar forma a la arquitectura regional. La iniciativa CBWeb3 es una invitación a participar en la construcción de infraestructura compartida para un futuro financiero regional que sirva a las prioridades de América Latina y el Caribe.

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
| **Desconexión del BC con obligaciones pendientes** | Un banco central desconectándose del hub mientras hay obligaciones de liquidación pendientes | Alto | Marco legal para desconexión ordenada; requisitos de colateral pre-fondado; protocolo de finalización automática de liquidación |

---

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
