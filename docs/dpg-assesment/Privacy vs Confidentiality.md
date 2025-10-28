# **Confidentiality vs Privacy in Tokenized Central Bank Money (tCeBM)**

### **1\. Conceptual Distinction: Privacy vs Confidentiality in Wholesale and Retail Contexts**

In the context of tokenized central-bank money (tCeBM), it is essential to differentiate **privacy** from **confidentiality**—terms often used interchangeably but that carry distinct implications.  
Broadly:

-   **Privacy** refers to the control individuals or entities have over their personal or transactional information and to the ability to remain **anonymous or pseudonymous** when conducting transactions.
    
-   **Confidentiality** refers to ensuring that the **contents or details** of communications and transactions remain **hidden from unauthorised parties**, even when participants themselves are identified.
    

In short, _anonymity_ protects _who_ participates, whereas _confidentiality_ protects _what_ occurs—the value, purpose, and metadata of a transaction.

In electronic-payment systems, anonymity shields identities; confidentiality protects message content. In tCeBM design, both properties are needed in different proportions depending on whether the system is **retail** (open to the public) or **wholesale** (restricted to financial institutions).

* * *

### **Retail tCeBMs — Protecting User Privacy**

A **retail tCeBM** aims to preserve users’ personal-data privacy, offering digital cash with attributes similar to physical banknotes: everyday payments without exposing personal spending behaviour. Surveys conducted by the Eurosystem revealed that citizens rank **privacy as the single most important feature** of a potential digital euro.

However, a fully anonymous tCeBM—like cash—is not legally feasible, as it would preclude anti-money-laundering (AML) and counter-terrorist-financing (CFT) oversight. Hence most central banks pursue **limited or tiered anonymity**.

For example, the European Central Bank tested _“anonymity vouchers”_—cryptographic tokens that permit low-value transactions without identity disclosure but trigger reporting once cumulative thresholds are exceeded.

* * *

### **Wholesale tCeBMs — Protecting Confidentiality Among Institutions**

In a **wholesale tCeBM** (restricted to central banks, commercial banks, and financial-market infrastructures), anonymity is irrelevant—the participants are already regulated and fully identified. The key requirement is **transactional confidentiality**: each participant must be able to settle and verify its own operations without learning the balances or exposures of competitors.

Traditional RTGS systems naturally enforce this through centralised architecture—each participant only sees its own queue and confirmations. When migrating to **distributed-ledger technology (DLT)**, however, this assumption breaks: replicated ledgers risk exposing all transactional data to all validators.

For instance, the **Central Bank of Brazil** highlighted in its _DREX_ pilot that a straightforward Besu-based DLT reveals participants, transactions, and balances to every node; pseudonymous public keys alone are insufficient for confidentiality. Therefore, **cryptographic or architectural layers** must be added so that decentralisation does not undermine commercial secrecy or statutory _banking secrecy laws_.

Brazil summarised the issue as a _trilemma between privacy, decentralisation, and programmability_: without dedicated privacy mechanisms, decentralised programmability exposes sensitive financial data—potentially conflicting with both user-data-protection (LGPD) and banking-secrecy requirements.

* * *

### **Summary**

-   **Retail tCeBM:** focus on **personal-data privacy** (protection of end-user identities and habits).
    
-   **Wholesale tCeBM:** focus on **institutional confidentiality** (protection of business-sensitive information between identified entities).
    
-   Both must balance these protections with **regulatory transparency** obligations.
    

The emerging consensus is for **conditional or revocable privacy**—anonymity/confidentiality towards most network participants, yet with lawful audit access for authorised authorities.


## **2\. Cryptographic Mechanisms to Preserve Confidentiality and Privacy**

Several cryptographic mechanisms have been designed to achieve privacy or confidentiality in digital-money systems. Below is an analysis of the main ones, including their advantages, limitations, and applicability from both a technical and regulatory perspective.

* * *

### **Zero-Knowledge Proofs (ZKP)**

**Definition:** Zero-Knowledge Proofs (ZKPs) allow a party to prove that a statement about private data is true (for instance, that a transaction is valid or that a user satisfies a requirement) **without revealing the underlying data**.

**Application in tCeBMs:**  
Within a tCeBM, zk-SNARKs or zk-STARKs can validate that a transaction obeys monetary rules without disclosing the amount or identity of the participants. Validators can thus verify correctness without accessing sensitive information.

**Advantages**

-   Resolve the tension between confidentiality and verifiability: every node can verify rules cryptographically.
    
-   Verification is computationally light (milliseconds once proofs are generated).
    
-   Aligns with AML/CFT: a user can prove, for example, that they remain below a holding limit or that KYC has been performed without revealing who they are.
    

**Limitations**

-   Proof generation is computationally heavy and often requires dedicated “provers.”
    
-   Some schemes (e.g., zk-SNARK) need a **trusted setup**, introducing governance questions (who runs the ceremony?).
    
-   Auditors and regulators must trust the underlying mathematics or possess the ability to reproduce proofs.
    

**Regional relevance:**  
The Central Bank of Brazil (BCB) actively explores ZKPs for DREX to balance privacy and auditability, acknowledging their maturity curve. In the LACNet context, ZKP integration into Besu is feasible because the EVM already supports zk-proof verification precompiles.

* * *

### **Blind Signatures**

**Definition:** Blind signatures, introduced by David Chaum, enable an issuer to sign a message **without seeing its content**.

**Application in tCeBMs:**  
A central bank can issue “digital banknotes”: a user blinds a token, the bank signs it, and later the user spends the token anonymously. The bank validates the signature but cannot link issuance and spending.

**Advantages**

-   Provides **strong anonymity from the issuer**, similar to physical cash.
    
-   Cryptographically simple and efficient, even offline.
    
-   Suitable for inclusion use-cases (low-value, offline payments).
    

**Limitations**

-   Requires a **double-spend prevention mechanism** (a registry of spent tokens).
    
-   If poorly designed, that registry can deanonymize users.
    
-   Lost tokens are irrecoverable (like lost cash).
    
-   Limited regulatory acceptance—probable use only below AML thresholds.
    

**Regulatory alignment:**  
Matches “limited-anonymity” models: e.g., ECB’s _anonymity vouchers_ concept allows anonymous payments up to a capped value.

* * *

### **Homomorphic Encryption (HE)**

**Definition:** Allows computation on encrypted data—operations like addition or multiplication can be performed **without decrypting** the values.

**Advantages**

-   Maximum privacy for computation—data stays encrypted end-to-end.
    
-   Enables privacy-preserving analytics (monetary statistics, risk models) while respecting data-minimization principles.
    

**Limitations**

-   Fully Homomorphic Encryption (FHE) remains **orders of magnitude slower** than normal computation—impractical for real-time settlement.
    
-   Implementations are complex and heavy on resources.
    
-   Useful mainly for **batch analytics**, not live transactions.
    

**Regulatory relevance:**  
Supports “privacy-by-design” compliance (GDPR/LGPD) by allowing aggregate analytics without accessing personal data. The BCB evaluated HE and concluded it is viable only for batch operations, not live payment flows.

* * *

### **ZK-Rollups**

**Definition:** A layer-2 solution grouping hundreds of off-chain transactions and posting a single validity proof on the main chain.

**Advantages**

-   Enhances **scalability and privacy** simultaneously.
    
-   Off-chain transactions remain hidden; on-chain record contains only aggregated proof.
    
-   Reduces on-chain data load and cost.
    

**Limitations**

-   Requires additional infrastructure (coordinators, sequencers).
    
-   “Data-availability” risks—metadata leaks if states are not properly concealed.
    
-   Regulators have less real-time visibility; mitigated if rollup operator is a regulated entity.
    

**Regional applicability:**  
Consortia in Brazil already prototype ZK-Rollups for DREX to improve confidentiality without losing supervisory auditability.

* * *

### **Selective Disclosure of Information**

**Definition:** Techniques that allow revealing **only specific data elements** to designated parties while hiding the rest.

**Use in tCeBMs:**  
Applies to both identity and transaction details:

-   Participants see what they need to reconcile a payment.
    
-   Regulators see minimal data (possibly anonymized) unless suspicion triggers full access.
    

**Technical Implementations**

-   **Cryptographic commitments** (e.g., Pedersen commitments): conceal values, later reveal them with a key.
    
-   **Split-key schemes:** sensitive data is encrypted so it can only be decrypted by combining several authorities’ keys (e.g., regulator + judiciary).
    

**Advantages**

-   Fine-grained control, aligning privacy with compliance.
    
-   Implements GDPR’s _data minimization_ and FATF’s _Travel Rule_ simultaneously.
    

**Limitations**

-   Requires a strong identity infrastructure and key governance.
    
-   Metadata correlation (timing, frequency) may still leak insights—mitigate with dummy transactions or traffic padding.
    

**Implementation in LACNet context:**  
Integration with LACChain ID (SSI-based) and Verifiable Credentials allows validating permissions and attributes without exposing PII, preserving cross-jurisdictional compliance.

* * *

### **Trusted Execution Environments (TEE, e.g., Intel SGX)**

**Definition:** Hardware enclaves that execute code in a protected memory region; even the node operator cannot view the processed data.

**Advantages**

-   Near-native performance compared with full cryptography.
    
-   Confidentiality preserved even against insider threats.
    

**Limitations**

-   Requires specialised hardware (limited availability).
    
-   Historical vulnerabilities (side-channels) raise caution.
    
-   Trust in hardware manufacturer introduces sovereignty considerations.
    

**Regional insight:**  
TEE penetration in Latin America is low. The DREX assessment found TEEs technically feasible but impractical for broad deployment; suitable only for specific auditing modules under hardware certification.

* * *

### **Comparative Summary**

**Mechanism**

**Purpose**

**Advantages**

**Limitations**

**Examples / Usage**

**Zero-Knowledge Proofs (ZKP)**

Validate rules without revealing data.

Publicly verifiable; flexible; high privacy.

Heavy proof generation; setup trust issues.

DREX (Brazil); BIS _Project Tourbillon_.

**Blind Signatures**

Unlink issuance from spending (user anonymity).

Simple; offline-capable; cash-like.

Double-spend registry; lost-token risk; limited compliance scope.

Offline tCeBM prototypes, low-value payments.

**Homomorphic Encryption**

Compute over ciphertexts.

Full data protection; useful for aggregate analytics.

Very slow; limited to batch use.

Statistical analytics pilots.

**ZK-Rollups**

Off-chain batching + on-chain proof.

Scalable; hides individual transactions.

Extra infra; less regulator visibility.

Brazilian consortia pilots; Aztec Network.

**Selective Disclosure**

Reveal only required data to each role.

Balances privacy/compliance; data minimisation.

Needs identity infra; metadata leakage possible.

ECB vouchers; Corda confidential identities; LACChain ID.

**Trusted Execution Environments**

Confidential processing in hardware enclaves.

Fast; insider-proof.

Hardware dependence; audit challenges.

SGX pilots; potential tCeBM wallet secure modules.

* * *

**Key insight:** No single mechanism suffices. Practical tCeBMs combine several layers—for example:

-   _Blind signatures_ for small offline payments,
    
-   _ZKPs_ for high-value validations, and
    
-   _Selective disclosure_ for supervisory access.
    

Wholesale systems prefer **ZKP + access-control architectures**: encrypted transactions visible only to involved parties and authorised regulators—protecting **institutional confidentiality** and enabling **auditable compliance**.

Residual risks (e.g., metadata inference) remain; countermeasures include _dummy traffic_, _batching_, and _time randomisation_, though these increase operational complexity and cost.

## **3\. Regulatory Compliance: Alignment of Privacy and Confidentiality with Legal Frameworks**

Any tCeBM or tokenized central-bank money must operate within existing legal and regulatory frameworks. This introduces explicit requirements regarding both **data privacy** and **transactional confidentiality**. Below we examine the most relevant frameworks and how the previously described techniques align—or conflict—with them.

* * *

### **Personal Data Protection Laws (GDPR, LGPD, etc.)**

Across jurisdictions—such as the European Union’s **General Data Protection Regulation (GDPR)**, Brazil’s **Lei Geral de Proteção de Dados (LGPD)**, and similar Latin American laws—citizens are guaranteed rights over their personal data. These frameworks enforce:

-   **Data minimization** (only collect what is necessary),
    
-   **Purpose limitation** (use data only for its declared aim),
    
-   **Consent or legitimate basis**, and
    
-   **User rights** (access, rectification, erasure).
    

**tCeBM implications:**  
Retail tCeBMs potentially generate vast quantities of personal-transaction data. Unless carefully designed, this makes the central bank a massive data controller. To comply, the architecture must:

-   Use **pseudonymization** or **anonymization** by default;
    
-   Limit data retention; and
    
-   Ensure access logging and role separation.
    

**Privacy techniques as compliance tools:**

-   **Selective disclosure** and **ZKPs** help achieve _privacy by design_ by preventing unnecessary data collection in the first place.
    
-   **Blind signatures** or “anonymity vouchers” can cover low-value payments where AML risk is low.
    
-   **Homomorphic encryption** can enable analytics or stress tests on aggregated, encrypted data—meeting supervisory needs without breaching data rights.
    

**Case example – Brazil (LGPD + Banking Secrecy):**  
The BCB’s DREX pilot found a conflict between programmability, decentralization, and legal secrecy obligations: a naïve DLT exposes too much. Therefore, any network like LNet must restrict visibility strictly by design. Confidentiality must be built in at the protocol level, and supervisory access must be formally logged and justified.

**Governance takeaway:**  
The architecture must specify **who can decrypt what** and **under which legal basis**, preserving an immutable trail for auditing access decisions.

* * *

### **AML/CFT Frameworks (FATF Recommendations)**

The **Financial Action Task Force (FATF)** requires that digital payment systems prevent money laundering and terrorist financing. One key rule is the **Travel Rule**:

> Institutions must transmit identifying information of the originator and beneficiary for electronic transfers above a given threshold (usually USD/EUR 1,000).

A tCeBM is no exception. While privacy-enhancing technologies can hide personal details from the public, they **must not preclude legitimate law-enforcement access**.

**Design implications:**

-   Absolute anonymity is incompatible with FATF.
    
-   Conditional or _revocable anonymity_ is acceptable: privacy during normal operations, traceability upon due legal process.
    
-   The system must allow regulated entities or competent authorities to “unmask” identities when warranted.
    

**Techniques to reconcile both:**

-   **ZKPs** can prove AML compliance without exposing identities (e.g., prove both wallets belong to regulated institutions).
    
-   **Threshold decryption**: private data is split among regulators; only a combination of keys—under judicial authorisation—reveals it.
    
-   **Transaction limits**: anonymous transfers are capped, and exceeding them triggers full KYC verification.
    

**Examples:**

-   **China’s e-CNY** implements “controllable anonymity”: basic wallets require only a phone number and have low limits, while larger accounts require full KYC.
    
-   **ECB’s digital euro PoC** applied the same principle via “anonymity vouchers” capped by value and time.
    

**Regulatory message:**  
Privacy layers are acceptable _only_ if they do not disable law enforcement’s ability to investigate suspicious transactions. Hence, system-level controls for lawful disclosure must be explicit and testable.

* * *

### **Information Security Standards (ISO 27001, NIST)**

tCeBMs will be classified as **critical financial infrastructure**, requiring robust security governance.

**ISO/IEC 27001** and the **NIST Cybersecurity Framework** prescribe controls over:

-   Confidentiality, integrity, availability (CIA),
    
-   Key management,
    
-   Access control,
    
-   Continuous monitoring.
    

**Operational implications for CBWeb3/LNet:**

-   All node communications must be encrypted (e.g., ECIES, TLS 1.3).
    
-   Sensitive off-chain data (API logs, dashboards) must use field-level encryption and access logging.
    
-   Smart contracts should undergo code review and security testing (OWASP Top 10 compliance).
    
-   Post-quantum readiness should be planned, in line with TORs.
    

**Relation to privacy tools:**

-   ZKPs and selective disclosure support the _confidentiality_ dimension of CIA.
    
-   Homomorphic encryption and TEEs extend that protection to data-in-use.
    
-   However, new cryptographic modules introduce new risks—governance must ensure regular audits and configuration control.
    

* * *

### **International Financial Standards (BIS, IMF, CEMLA)**

Global bodies such as the **Bank for International Settlements (BIS)**, the **International Monetary Fund (IMF)**, and **CEMLA** issue non-binding but influential principles. Key ones include:

-   Do not compromise financial stability or monetary control.
    
-   Preserve integrity and resilience of payment systems.
    
-   Foster public trust through transparency and data protection.
    

**Privacy as a trust enabler:**  
BIS highlights that central banks are uniquely positioned to handle data responsibly—being non-commercial entities—but must prove they do so transparently. Demonstrable _privacy by architecture_ enhances acceptance.

**Operational implication:**  
Each central bank in CBWeb3 should retain sovereignty over data relating to its jurisdiction while adhering to regional governance rules. The network must support jurisdictional data partitioning—possibly via sharding or scoped keys.

* * *

### **Local Sectoral Laws (e.g., Banking Secrecy in Brazil)**

Brazil’s **Lei Complementar 105/2001** guarantees banking secrecy and allows its breach only by judicial or regulatory order.

**DREX’s lesson:**  
The BCB explicitly identified the main challenge as _“balancing privacy of financial transactions (banking secrecy) with programmability and decentralization.”_  
The pilot implemented **network segmentation**: sub-networks where only authorized nodes validate certain data, reducing leakage.

**Implications for CBWeb3:**

-   Access control must be granular: banks see only their own operations, while central banks see the full picture.
    
-   The system must be able to produce _forensic disclosures_ on legal demand—i.e., decrypt or reconstruct a transaction when required.
    

**Governance consideration:**  
A “forensic access” function should exist but remain locked behind multi-party authorisation (e.g., central bank + judiciary).

* * *

### **Summary**

Regulatory compliance forces tCeBM designers into a delicate equilibrium:

-   Too little privacy: breaches citizens’ data rights and commercial secrecy.
    
-   Too much privacy: breaches AML and oversight obligations.
    

Most frameworks can be simultaneously satisfied if the system offers:

-   **Tiered privacy** (by value and risk),
    
-   **Selective disclosure** (by role and jurisdiction), and
    
-   **Lawful auditability** (via cryptographic and procedural controls).
    

This legal landscape strongly supports CBWeb3’s proposed direction: a _privacy-preserving but regulator-accessible_ design.

## **4\. Technologies and Architectures: Listrack, CCIP, Cacti, Hyperledger Besu**

To implement confidentiality and privacy in interoperable environments, several architectural options exist. This section analyses four key technologies referenced in CBWeb3 research—**Listrack**, **Chainlink CCIP**, **Hyperledger Cacti**, and **Hyperledger Besu**—comparing their approaches, trust models, and privacy capabilities.

* * *

### **Hyperledger Besu (LNet Network Layer)**

**Overview:**  
Hyperledger Besu is an open-source Ethereum client tailored for **permissioned networks**. In CBWeb3, it serves as the **base ledger layer** over which privacy and interoperability frameworks are built.

**Core features**

-   **Permissioning:** All nodes and participants are pre-approved (e.g., central banks, regulated institutions). This satisfies AML/KYC and FATF expectations and simplifies legal accountability.
    
-   **Encryption:** P2P communications use ECIES encryption; IBFT 2.0 consensus ensures fast finality and Byzantine fault tolerance.
    
-   **Private transactions:** Besu inherited Quorum’s “private-transaction manager” (Orion/Tessera). These allow bilateral encrypted transactions visible only to selected nodes, though Tessera is now deprecated. CBWeb3 therefore moves privacy to the **smart-contract layer** (ZKPs, encryption, commitments) instead of node-level managers.
    
-   **EVM compatibility:** Supports verification of zk-SNARK proofs (alt\_bn128 precompiles) and smart-contract libraries for commitments, range proofs, and ring-signature logic.
    
-   **Identity integration:** Through LNet’s **LACChain ID** system, each node and organisation has a verifiable DID/credential. Combined with verifiable credentials, this enables **selective disclosure**—transactions can prove authorisation without exposing personal data.
    

**Interoperability limitation:**  
Besu has no native cross-chain capabilities. Interoperability therefore relies on external layers—Cacti, CCIP, or Listrack—to bridge networks securely.

* * *

### **Listrack (Listen-and-Track Design)**

**Concept:**  
Proposed by BBChain researchers, Listrack enables interoperability **without third-party bridges**.  
Regulated nodes within a permissioned blockchain **listen** to an external blockchain, verify its events, and **track** them internally by posting a cryptographic hash on their own chain.

**Operation**

1.  A designated **trusted node** (e.g., a central-bank validator) observes a transaction on an external chain.
    
2.  It records a **state-pinning hash** representing that event in the local (regulated) ledger.
    
3.  Other validators verify the hash and confirm the transaction atomically.
    
4.  The design guarantees uniqueness, atomic reversals, and scalability—no heavy payloads or external oracles.
    

**Privacy and confidentiality:**  
Because Listrack only imports **hashes**, no sensitive data leaves the regulated domain. Within the tCeBM network, only authorised nodes can interpret these hashes, preserving confidentiality.  
Externally, transactions remain visible on the source chain but not linkable internally.

**Governance advantage:**  
All bridge functions are handled by regulated entities, eliminating dependency on non-regulated third parties. This fits CBWeb3’s need for sovereign, auditable cross-chain links.

**Status:**  
Academic prototype (TechRxiv 2023) validated in simulation. Suitable for _regulator-only corridors_ requiring on-chain verifiability without external oracles.

* * *

### **Hyperledger Cacti**

**Overview:**  
Cacti (formerly “Cactus”) is Hyperledger’s **modular interoperability framework**. It provides connectors, SDKs, and orchestration tools to link heterogeneous blockchains **without a common settlement chain**.

**Key characteristics**

-   **Autonomy:** Each network keeps its own governance and consensus; Cacti only coordinates atomic operations between them.
    
-   **Connectors:** Deploy lightweight **adapters** in each blockchain that exchange signed events and proofs.
    
-   **Trust models:** Supports hashed time-locks (HTLC), notary, or relay modes—configurable per corridor.
    
-   **Security and privacy:** Designed for regulated environments; connectors exchange only minimal required data (transaction hashes, ZK proofs, or encrypted payloads).
    
-   **Openness:** Fully open source, aligning with Digital Public Goods principles and LNet’s governance model.
    
-   **Implementation:** Demonstrated bridges such as Stellar ↔ Besu and Fabric ↔ Besu, confirming multi-platform viability.
    

**Confidentiality:**  
Since data sharing occurs only between connectors, each network can enforce its internal privacy model. For instance, a country using ZKPs can interact with another using access-controlled accounts; Cacti mediates proofs rather than full data.

**Limitations:**  
Configuration complexity and the need for standardised message schemas (ISO 20022) across networks. Governance is decentralised: participants must agree on connector maintenance and upgrade procedures.

**Role for CBWeb3:**  
Ideal for **intra-regional interoperability** among Latin-American central banks, allowing a self-governed “LAC Hub” under LNet stewardship.

* * *

### **Chainlink CCIP (Cross-Chain Interoperability Protocol)**

**Overview:**  
Chainlink’s CCIP provides interoperability **as a service** via a decentralised oracle network. Independent oracle nodes watch for events on one chain and execute corresponding actions on another.

**How it works**

1.  A smart contract in Chain A emits an event (e.g., “transfer 100 tokens to Chain B”).
    
2.  CCIP oracles detect the event, reach consensus, and submit a signed message to Chain B.
    
3.  The receiving contract executes the corresponding mint, lock, or state update.
    

**Features**

-   **Decentralised oracle layer:** Multiple independent nodes; no single point of failure.
    
-   **Unified API:** Developers call `sendToChain()` methods instead of building custom bridges.
    
-   **Enterprise integrations:** Already integrated with Hyperledger Besu for _secure, seamless cross-chain messaging_.
    
-   **Use cases:** Atomic PvP/DvP between tCeBMs on separate networks; message relays for tokenised assets.
    
-   **Adoption:** Piloted with SWIFT for cross-chain experiments and integrated into Brazil’s DREX pilot for cross-border testing.
    

**Privacy & governance considerations**

-   CCIP oracles **see the plaintext transaction data** on both chains; thus, confidentiality depends on oracle behaviour.
    
-   Institutional deployments can mitigate this by operating **permissioned oracle groups** (banks or LNet nodes acting as oracles).
    
-   Nevertheless, governance partially resides with Chainlink’s network, creating a dependency that must be contractually managed.
    
-   Messages themselves are not encrypted; sensitive data should be hashed or transmitted via secure side channels.
    

**Strengths**

-   Quick deployment and proven reliability.
    
-   Ideal for early cross-border pilots with existing partners.
    

**Trade-off**

-   Reduced sovereignty—reliance on a third-party oracle provider for critical settlement functions.
    

* * *

### **Comparative Table**

**Technology / Architecture**

**Interoperability Model**

**Trust Model**

**Privacy / Confidentiality Handling**

**Current Status / Example**

**Listrack**

On-chain interoperability by regulated nodes only (state-pinning + hashes).

Trusted regulated nodes; no third-party oracles.

Only hashes recorded; data stays inside regulated network.

Prototype (BBChain 2023); proposed for regulator-only corridors.

**Hyperledger Cacti**

Modular connectors between autonomous blockchains (no common ledger).

Federated self-governed connectors.

Exchanges only minimal/encrypted data; configurable privacy per corridor.

Active open-source project; used in Stellar ↔ Besu bridge.

**Chainlink CCIP**

Decentralised oracle network transferring messages between chains.

Distributed third-party oracles (public or permissioned).

Oracles view cleartext; permissioned setups under development.

Live in multiple ecosystems; used by SWIFT & DREX.

**Hyperledger Besu**

Base ledger for permissioned tCeBM; supports EVM and PETs.

Permissioned validators (central banks).

Node comms encrypted; smart-contract-level privacy via ZKP, commitments.

Foundation layer of LNet and DREX pilots.

* * *

### **Synthesis**

-   **Besu** provides the secure, permissioned base layer.
    
-   **Cacti** enables _self-governed regional interoperability_.
    
-   **CCIP** offers _rapid integration_ with external or global networks.
    
-   **Listrack** delivers _oracle-free interoperability_ for regulator-only corridors.
    

A hybrid approach is advisable:

-   Use **Cacti** for regional corridors under LNet governance.
    
-   Use **CCIP** for external connections or early pilots.
    
-   Deploy **Listrack** for sensitive bilateral exchanges where third-party involvement is unacceptable.
    

Each layer must preserve confidentiality:

-   Cacti: configurable encrypted messages;
    
-   Listrack: on-chain hash anchoring;
    
-   CCIP: restrict oracles and encrypt payloads.

## **5\. Practical Case Studies and Lessons Learned**

Several global pilot projects provide concrete lessons on how to address—or fail to address—privacy and confidentiality in tCeBMs. Below are three emblematic cases: **DREX (Brazil)**, **Digital Euro (European Union)**, and **e-CNY (China)**, along with observations from smaller initiatives.

* * *

### **5.1 DREX – The Brazilian “Real Digital”**

**Overview:**  
Brazil’s **DREX** project (formerly “Real Digital”) aims to support both **wholesale** and **retail** use cases, focusing on tokenized assets, PvP/DvP, and interbank payments.  
It is built on **Hyperledger Besu**, aligned with LNet’s technology stack, and subject to Brazil’s **LGPD** (data protection) and **banking secrecy** laws.

**Technical approach:**  
During **Phase 1 (2023–2024)**, the BCB tested several privacy solutions:

-   **Zero-Knowledge Proofs (ZKP)** via the **Anonymous Zether** protocol to hide balances and identities.
    
-   **Network segmentation** — creation of sub-networks/channels where only relevant parties validate data.
    
-   **Confidential computing** exploration (Intel SGX).
    
-   **Access-control enforcement:** only “token authority” nodes (the BCB) may view all balances and transactions.
    
-   Tests of other protocols, including **Rayls**, for private payments.
    

**Results and insights:**

-   None of the tested solutions fully satisfied both legal and operational requirements.
    
-   **Anonymous Zether** achieved strong confidentiality but impaired regulatory visibility—authorities could not inspect suspicious activity.
    
-   Cryptographic schemes introduced **operational risk**: loss of private keys could mean irreversible loss of funds.
    
-   Hence, Brazil prioritised finding a **balance between privacy and supervision**, concluding that excessive opacity is as problematic as excessive transparency.
    

**Next steps (Phase 2):**

-   Combine ZKPs with conditional disclosure for the BCB.
    
-   Continue segregating data and evaluating performance impacts.
    
-   Simplify architecture: start with clear-text functional prototypes, then progressively add privacy layers once stability and scalability are validated.
    

**Governance principle:**

> “Technology must not alter the legal nature of the asset.”  
> Therefore, the BCB retains ultimate control (e.g., freezing tokens under court order).

**Summary of DREX learnings:**

1.  **Balance privacy with oversight.** Over-encryption hinders compliance.
    
2.  **Current PETs are not plug-and-play.** Integration into smart-contract logic is complex.
    
3.  **Iterative rollout:** implement functionality first, then enhance privacy incrementally.
    
4.  **Legal compliance is non-negotiable.** Banking secrecy and LGPD set hard limits.
    
5.  **Performance impact:** privacy layers reduce throughput (~125 TPS observed).
    
6.  **Delay of production:** privacy and data-protection concerns must be resolved before launch.
    

* * *

### **5.2 Digital Euro (European Union)**

**Context:**  
The **European Central Bank (ECB)**, constrained by GDPR and public-trust considerations, made privacy the central design criterion for the digital euro.

**Key milestones:**

-   Public consultation (2021): **43 % of respondents** ranked privacy as the top priority.
    
-   ECB–R3 Corda proof-of-concept (2019): implemented **“anonymity vouchers”**—cryptographic allowances for small anonymous payments.
    
-   These vouchers capped total anonymous spending (e.g., €100 per month).
    
-   2023 legislative drafts include explicit “privacy-by-design” clauses, limiting data held by the Eurosystem and distributing user data among intermediaries.
    

**Findings:**

-   _Tiered privacy_ is viable: small payments nearly as private as cash; large payments fully traceable.
    
-   _Two-tier model_: intermediaries (banks) handle KYC and store granular data; ECB receives only aggregated statistics.
    
-   _Offline mode_: physical devices or cards enable private low-value payments without instant reporting.
    
-   _Pseudonymity instead of anonymity_: transactions identified by codes resolvable only by the user’s bank or authority upon legal request.
    

**Governance approach:**

-   ECB likely to operate as data controller under GDPR Art. 6(1)(e) (public interest).
    
-   Mandatory Data-Protection Impact Assessment (DPIA) and DPO appointment.
    
-   Public-communication strategy emphasising that “ECB will not access individual payment data.”
    

**Lessons for CBWeb3:**

1.  Implement **tiered privacy thresholds** and _lawful-access processes_.
    
2.  Use **intermediaries** for identity management to decentralise data handling.
    
3.  Consider **offline-payment functionality** for inclusion and user trust.
    
4.  Make **transparency and governance** explicit—privacy guarantees must be publicly documented.
    

* * *

### **5.3 e-CNY – China’s “Controllable Anonymity”**

**Overview:**  
China’s **e-CNY** is the world’s most advanced retail-tCeBM pilot.  
Its model of **“controllable anonymity” (可控匿名)** means users enjoy privacy from merchants and most institutions, but the state retains traceability.

**Implementation details:**

-   **Tiered wallets:**
    
    -   _Level 4_ — phone-number registration only, small limits, high anonymity.
        
    -   _Level 1_ — full KYC, higher limits, linked to bank accounts.
        
-   **Two-layer issuance:** PBoC → commercial banks → users.
    
-   **Offline capability:** NFC-based peer-to-peer transfers with later synchronization.
    
-   **Data handling:** merchants and private platforms (Alipay/WeChat) receive minimal personal data; PBoC retains complete oversight.
    
-   **Monitoring:** big-data and AI tools detect suspicious patterns while respecting “minimal necessary exposure.”
    

**Strategic rationale:**  
China frames privacy in contrast to **Big Tech**, not the state—arguing that e-CNY protects citizens better than commercial apps by preventing commercial data exploitation.

**Lessons:**

-   **Relative privacy** can still build user trust if compared to existing alternatives.
    
-   **Tiered KYC** enables inclusion (unbanked users with basic wallets).
    
-   **Centralised architecture** simplifies auditability but sacrifices decentralisation.
    
-   **Transparency to users** (clear limits, explicit consent) helps adoption despite reduced anonymity.
    

* * *

### **5.4 Other Notable Initiatives**

**Project**

**Region**

**Approach**

**Relevance to CBWeb3**

**Sand Dollar**

Bahamas

Account-based, simplified KYC for low-value wallets.

Demonstrates inclusion via tiered identity.

**DCash**

Eastern Caribbean

Two-layer model with centralised validation.

Early operational lessons on resilience.

**e-Krona**

Sweden

Began as token-based anonymous model, moved to R3 Corda.

Shows trade-offs between anonymity and compliance.

**Project Ubin / Jasper**

Singapore / Canada

Wholesale DLT with confidential transactions using ZKPs.

Validated privacy + verifiability in interbank settings.

**Project Helvetia / Khokha**

BIS / South Africa

ZKP-based wholesale pilots.

Demonstrated performance feasibility and legal soundness.

* * *

### **Key Takeaways for CBWeb3**

1.  **Hybrid privacy strategy:** combine architectural, procedural, and cryptographic controls.
    
2.  **Start wholesale, expand retail later:** institutional confidentiality is the immediate priority.
    
3.  **Use DREX and Euro Digital precedents:** adopt _tiered privacy_ and _conditional disclosure_.
    
4.  **Avoid over-engineering:** focus on mature, efficient PETs (ZKPs, selective disclosure); defer FHE.
    
5.  **Embed legal governance:** define regulator access, data retention, and cross-border data-sharing agreements from day one.
    
6.  **Performance planning:** expect throughput reduction with privacy layers—prepare scaling solutions (rollups, batching).
    
7.  **Transparency to participants:** clear communication and documented privacy guarantees are essential for trust.

## **6\. Implications for CBWeb3 Architecture and Governance**

The synthesis of all analyses—technical, regulatory, and comparative—reveals clear implications for the architecture, governance, and work distribution within the **CBWeb3** project. The goal is to ensure the system achieves _maximum privacy and confidentiality_ while maintaining _regulatory compliance_ and _operational viability_ in a **regional, interoperable environment**.

* * *

### **6.1 Architectural Implications**

#### **Layered Privacy Model**

CBWeb3 should adopt a **multi-layered privacy architecture**, integrating different mechanisms according to transaction type and risk level:

**Layer**

**Purpose**

**Techniques / Tools**

**Responsible Team**

**Network & Infrastructure**

Encryption in communication channels, access control, node isolation.

TLS 1.3 + ECIES; strict permissioning; IBFT 2.0 consensus; container hardening (OWASP/NIST).

**LNet**

**Smart-Contract Layer**

Confidential transactions, anonymised amounts, selective disclosure.

Zero-Knowledge Proofs; Pedersen Commitments; Blind Signatures; ZK-Rollups.

**GoLedger / ÁguilaHub**

**Identity & Credential Layer**

Pseudonymous and verifiable identity, access to lawful audit.

LACChain ID (DID/VC); selective disclosure credentials; SSI integration.

**Hopae / Privacy Team**

**Interoperability Layer**

Secure cross-chain exchange between networks.

Hyperledger Cacti; Chainlink CCIP; Listrack (for regulator-only corridors).

**Joint (LNet + GoLedger)**

**Governance & Compliance Layer**

Policy enforcement, key custody, regulator access.

Multi-sig governance contracts; HSM key management; threshold decryption for lawful access.

**CBWG / LNet Governance**

This separation allows the project to **evolve incrementally**—testing privacy modules independently and integrating them once validated.

* * *

#### **Privacy-by-Design Principles**

1.  **Data minimisation:** store only what is strictly necessary; off-chain logs must be anonymised or hashed.
    
2.  **Pseudonymisation:** all node identifiers (enodeID, wallet address) must be hashed; role metadata stored separately.
    
3.  **End-to-end encryption:** extend existing RLPx encryption to transaction payloads.
    
4.  **Metadata protection:** implement dummy transactions, randomised timing, and batch processing to mask activity patterns.
    
5.  **Forensic traceability:** maintain the ability to reconstruct complete transactions only through _multi-party authorisation_ (e.g., regulator + judiciary).
    

These design principles align with both **GDPR/LGPD** and **FATF Travel-Rule** requirements, ensuring lawful transparency without undermining participant confidentiality.

* * *

#### **Recommended Technologies**

-   **ZKP Framework:** deploy zk-SNARK or zk-STARK circuits off-chain (ZoKrates or similar) with on-chain verification.
    
-   **Selective-Disclosure Credentials:** implement using Verifiable Credentials & JSON-LD for cross-jurisdictional compatibility.
    
-   **Secure Containers & Enclaves:** explore TEE modules for critical computation (e.g., regulator analytics).
    
-   **Homomorphic Encryption for Analytics:** restricted to aggregate statistical queries.
    
-   **ZK-Rollups:** prototype for scalable privacy in high-volume corridors.
    

Each component should be modular so it can be replaced as cryptographic standards evolve.

* * *

### **6.2 Governance Implications**

#### **Multi-Stakeholder Governance**

CBWeb3 operates under a **federated governance model** involving:

-   **Central banks** – custodians of monetary policy and legal compliance.
    
-   **LNet** – network operator and infrastructure orchestrator.
    
-   **Hopae** – privacy and security research lead.
    
-   **GoLedger / ÁguilaHub** – implementation and DevOps.
    
-   **CBWG (Cross-Border Working Group)** – coordination of bilateral corridors.
    
-   **DPGWG (Digital Public Goods Working Group)** – oversight of open-source, transparency, and sustainability principles.
    

Each layer of governance must include:

-   Clear decision rights (technical, regulatory, operational).
    
-   Defined escalation and audit processes.
    
-   Documentation of key ceremonies and software releases.
    

* * *

#### **Regulator Access and Audit**

A central governance principle must ensure **selective visibility**:

-   Ordinary validators see only transactional hashes or ZK-proof outcomes.
    
-   Designated regulators hold **threshold-split keys** allowing controlled decryption.
    
-   All accesses are logged immutably on-chain.
    
-   Emergency “freeze” or “rollback” powers reside only with the central-bank consortium.
    

This model guarantees _lawful transparency_ without systemic exposure.

* * *

#### **Cross-Border Coordination**

-   **Within LAC region:** use **Hyperledger Cacti** connectors governed by a _regional charter_ that defines roles, key custody, and interoperability SLAs.
    
-   **With external networks:** employ **CCIP** through regulated oracle clusters; require independent audits of oracle performance and incident disclosure.
    
-   **For highly sensitive corridors:** implement **Listrack** state-pinning operated exclusively by central-bank validators.
    

All corridors must adopt **ISO 20022** message schemas for harmonised data exchange and regulatory reporting.

* * *

#### **Digital-Public-Goods (DPG) Governance**

CBWeb3 must adhere to DPG principles:

-   **Open licensing:** Apache 2.0 or equivalent.
    
-   **Public documentation and reproducibility:** GitHub repositories with CI/CD, SBOMs, and test scripts.
    
-   **Security transparency:** publish threat models and privacy KPIs.
    
-   **Community contributions:** encourage external reviews, particularly of PET modules.
    

These ensure that CBWeb3 remains a _global reference architecture_ for open, interoperable tCeBM infrastructures.

* * *

### **6.3 Operational Roadmap**

**Phase**

**Timeframe**

**Main Objectives**

**Deliverables**

**Phase 1 – Foundational Infrastructure**

Months 1–6

Deploy base Besu network on LNet; implement permissioning and encryption; define identity schema (LACChain ID).

Network documentation, security configuration, API gateways.

**Phase 2 – Privacy Layer Integration**

Months 7–12

Integrate ZKP proofs, selective disclosure credentials, and privacy test plan.

ZKP circuits, privacy test reports, audit logs.

**Phase 3 – Interoperability Corridors**

Months 13–18

Deploy Cacti/CCIP connectors and pilot Listrack in regulator-only corridor.

Cross-chain proof of concept, interoperability charter.

**Phase 4 – Performance & Compliance Validation**

Months 19–24

Execute full testnet pilot with wholesale use cases (DvP, PvP).

Metrics dashboard, compliance verification, regulatory report.

**Phase 5 – Production Readiness**

Months 25–30

Final governance approval, go-live checklist, documentation publication.

Hardening checklist, disaster-recovery plan, DPG certification.

* * *

### **6.4 Recommendations for the CBWG and DPGWG**

**For CBWG (Cross-Border Working Group):**

1.  Approve _tiered privacy thresholds_ for cross-border corridors.
    
2.  Define an _inter-central-bank data-sharing charter_ aligned with FATF and ISO 20022.
    
3.  Establish a _corridor governance model_ for oracles and connectors.
    
4.  Create a _regional privacy-audit protocol_ to periodically validate PET performance and regulatory adherence.
    

**For DPGWG (Digital Public Goods Working Group):**

1.  Ensure that privacy modules (ZKP, selective disclosure, Listrack connectors) are released as open-source components.
    
2.  Publish interoperability and privacy-testing toolkits as Digital Public Goods.
    
3.  Promote collaboration with academic and regulatory partners for continuous evaluation of emerging PETs (e.g., post-quantum, FHE).
    
4.  Develop a _regional knowledge base_ documenting lessons learned from CBWeb3 to serve future digital-currency projects.
    

* * *

### **6.5 Concluding Remarks**

The comparison between **confidentiality** and **privacy** in the tCeBM context demonstrates that:

-   **Confidentiality** protects _information content_ and ensures secure restricted access.
    
-   **Privacy** protects _identities, relationships, and metadata_.
    
-   Both must coexist under a single regulatory and architectural umbrella.
    

For CBWeb3, the optimal path is a **privacy-preserving, regulator-auditable, and interoperable** design, combining:

-   **ZKPs + selective disclosure** for verifiable privacy,
    
-   **Compartmentalised architecture** (Ptah-inspired) to isolate data flows,
    
-   **Interoperability frameworks** (Cacti, CCIP, Listrack) for regional and global corridors, and
    
-   **Transparent governance** aligned with Digital-Public-Goods standards.
    

Such a system will position CBWeb3 and its 12 participating central banks as **global benchmarks** for trustworthy, compliant, and sovereign digital-currency infrastructure—capable of reconciling innovation with the highest standards of data protection and financial integrity.