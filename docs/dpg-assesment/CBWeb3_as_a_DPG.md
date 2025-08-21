# CBWeb3 as a DPG


## Introduction

This is a dynamic base document for achieving the goal of CBWeb3 as a DPG. It will initially serve to identify the tasks required by the working group, and as these tasks are developed, the document will be updated.

## Summary of requirements for a Digital Public Good (DPG)

A Digital Public Good is an open-source digital solution that meets rigorous standards and contributes to sustainable development. According to the Digital Public Goods Alliance (DPGA), there are nine conditions a project must meet to be recognized as a DPG:

- **Relevance to the Sustainable Development Goals (SDGs):** The solution must demonstrate that it contributes to achieving one or more SDGs of the UN 2030 Agenda.

- **Open Licensing**: It must use an approved open license that allows for reuse and adaptation (e.g., Apache, MIT, GPL, etc.).

- **Clearly Defined Ownership**: The organization or group that owns or is responsible for the project must be publicly declared, including copyright or trademark rights, if applicable.

- **Platform Independence**: The project must not depend on proprietary platforms, vendors, or components with no open alternatives. Any closed dependencies must be replaceable with open equivalents without significant effort.

- **Comprehensive Documentation**: There must be sufficient technical documentation to enable a competent third party to install, use, and maintain the solution independently. This includes open repositories, installation manuals, user guides, etc.

- **Extraction of Non-Personal Data**: If the solution handles data, it must allow for the export and import of non-sensitive/personal data in open formats (CSV, JSON, XML, public APIs, etc.).

- **Privacy and Applicable Laws**: If you handle personal data (PII), you must comply with relevant privacy laws (e.g., GDPR or other local laws) and have public privacy policies and terms of use.

- **Open Standards and Best Practices**: You must adhere to open standards (e.g., protocols, formats) and recognized principles or best practices in your field (e.g., Digital Development Principles, security standards, accessibility, etc.).

- **Design for “Do No Harm**”: The solution must have been designed with potential harm in mind. For example, if you handle personal data, explain how privacy and security are protected; if you allow user-generated content, have moderation mechanisms; if you allow user interaction, include codes of conduct and anti-harassment measures. In short, ensure that the tool does not cause harm and mitigates risks from its inception.

These criteria form the DPG standard that the DPGA examines in its technical review process for each candidate.

## About the CBWeb3 project (CBDC context)

**CBWeb3** is an initiative focused on experimentation with central bank digital currencies and tokenized assets in Latin America and the Caribbean. CBWeb3 seeks to create a "regional test network for tokenized securities markets" that enables cross-border interoperability between central banks, financial institutions, and citizens of various countries in the region. This is a sandbox or test network where central banks can experiment with token issuance, cross-border payments, real-time settlement, and other use cases in a secure environment before putting them into production. The value of this test network lies in offering a single interoperable protocol that ensures compatibility and collaboration among participating countries.

The CBWeb3 project is part of the efforts of LACChain, an alliance led by the IDB Lab (Inter-American Development Bank, Innovation Lab) to promote public permissioned blockchain in the region. In fact, it is promoting "the first regional test network for tokenized money, in collaboration with the Center of Latin American Monetary Studies (CEMLA), Latin American Reserve Fund (FLAR), and Central Banks," which aligns with the goals of CBWeb3 (a regional sandbox for central banks), according to recent releases. This prototypical effort relies on LACChain's blockchain infrastructure to offer a common platform for experimentation, leveraging a permissioned Hyperledger Besu network provided by LACNet (the non-profit orchestrator of the LACChain network) to ensure neutrality, regulatory compliance, and the absence of transaction fees.

## CBWeb3 analysis vs. DPG criteria

The degree of compliance of CBWeb3 with each DPG criterion examined below, indicating current strengths and gaps to be addressed to achieve Digital Public Good status.

### Relevance to Sustainable Development Goals (SDGs)

**Current situation**: The central objective of CBWeb3 – enabling infrastructure for cross-border payments and tokenization in Latin America – **is aligned with several SDGs**. In particular, CBWeb3 could contribute to:

- **SDG 8 (Decent Work and Economic Growth) and SDG 9 (Industry, Innovation, and Infrastructure):** By modernizing payments and securities markets infrastructure, it facilitates financial innovation and regional economic efficiency.

- **SDG 10 (Reduced Inequalities):** By exploring solutions such as more accessible cross-border payments (e.g., cheaper, and faster remittances), it can empower vulnerable populations and reduce financial gaps. In fact, the IDB Lab explicitly seeks with LACChain to “reduce inequality and empower people” in the region, a goal to which CBWeb3 would contribute by improving regional financial inclusion.

- **SDG 17 (Partnerships for the Goals):** The project involves collaboration between multiple countries and institutions (IDB Lab, CEMLA, FLAR, national central banks), exemplifying international partnerships for common infrastructure.

**What is missing or needs to be improved**: Within the Digital Public Good Working Group (DPG WG), we must clearly articulate and document this link to the SDGs. To meet this requirement, it is recommended to prepare a public description (e.g., in the repository in the documentation) that explains how CBWeb3 contributes to specific goals and targets of the relevant SDGs. For example, targets such as 10.c (reducing remittance transaction costs) could be mentioned in the case of cross-border payments, among others. This clarity will help the DPGA recognize its impact on sustainable development, a fundamental requirement for a DPG.

### Open license approved.

**Status:** CBWeb3 already meets this criterion. The source code will be publicly available under the Apache 2.0 License, an open license approved by the DPGA (listed among those accepted for being permissive and compatible with the definition of open source). This means that anyone can use, modify, and distribute the software, which is essential for it to be a digital public good.

The documentation is written in Markdown, which is compatible with open licenses.

**What is missing or needs to be improved**: In principle, there is nothing critical at this point, given that the choice of Apache 2.0 is appropriate and is explicitly indicated in the repository. Within the README and/or project website, in addition to the license, we should highlight the open nature of the project and encourage the community to contribute. It is also important to ensure that all project content (code, documentation, etc.) is covered by open licenses. If the project incorporates data or other types of resources in the future, they must also be covered by compatible open licenses.

### Clear ownership and governance

**Status:** CBWeb3 is an IDB Lab project in partnership with CEMLA/FLAR and participating central banks. Code maintenance will be done under the leadership of LACChain and in collaboration with LFDT and all DPG WG participants.

CBWeb3 has the participation of organizations such as CEMLA and FLAR, along with several national Central Banks, in collaboration with IDB Lab and IDB Group.

**What is missing or needs to be improved**: To meet this criterion, we must provide formal clarity regarding the project's ownership and governance. Specifically, it is recommended to:

- **Identify the owner organization or consortium**: for example, clarify in the documentation that it is a LACChain project in partnership with CEMLA/FLAR and participating central banks. This can be done in the public documentation (README, website) by indicating "CBWeb3 is a project co-developed by X, Y, Z" and who maintains the code.

- **Legal or institutional references**: Include links to official pages that mention the project. For example, link to IDB press releases or agreements with CEMLA/FLAR regarding CBWeb3. Also mention the policy on the use of trademarks or logos, if applicable (e.g., if "CBWeb3" is a name coined by LACChain/IDB).

- **Open governance**: Describe how decisions are made in the project, in line with open best practices, and specify how the DPG WG operates.

In short, make explicit who supports and directs CBWeb3. This will reassure the DPGA that the project has continuity and clear institutional support, and it is also useful information for the broader community.

### Platform independence (no mandatory proprietary dependencies)

**Current situation**: CBWeb3 is built on a completely open infrastructure: it uses the LACChain network based on Hyperledger Besu (open-source enterprise Ethereum), which makes it independent of closed blockchain providers. It does not depend on proprietary software to run at its core, as both the network and smart contracts can be implemented with open technologies. This is a strong point, as it is not "locked" into a proprietary solution but operates in an interoperable open-source environment (Ethereum/Besu).

This must remain the case, so any external integration must adhere to open standards and be independent of a particular vendor. Interoperability integrations with current Swift systems must be optional on the network, meaning the entire system should be able to function without them.

Another dependency is whether a cloud service or specific proprietary hardware will be used to deploy nodes, but the expectation is that any component can be replaced with open alternatives (for example, deploying on on-premises servers instead of relying on a single cloud provider).

**TO DOs**: The DPG documentation should explain how CBWeb3 avoids proprietary lock-in. Specific recommendations:

- **List the project's technological dependencies** and indicate which ones are open. For example: Hyperledger Besu (open source), standard APIs (open). If integrations such as SWIFT or CCIP are planned, clarify that they are not mandatory and that alternative paths exist (e.g., using ISO 20022 messaging over the internet, CACTI, or other open mechanisms).

- **Ensure open alternatives**: If any component is closed, document an alternative. For example, if a KYC module from a private provider is used, suggest that it could be replaced with an open source decentralized digital identity module. In the case of SWIFT and CCIP, emphasize that the sandbox could only work on the blockchain network or with open payment networks, and that SWIFT would only be an additional connection for interoperability testing with traditional banks, not a requirement for all users.

- **Proof of independence:** It might be useful to demonstrate that the system runs in 100% open-source environments without the need for proprietary licenses.

By detailing this, CBWeb3 would demonstrate full compliance with the platform-independent criterion. In short: ensuring by design that no critical element of the system requires the use of proprietary software or services without an equivalent open option.

### Open and complete documentation

**Current situation**: Documentation in progress.

**What is missing or needs to be improved**: We must develop robust documentation before applying as a DPG. In particular, the following documents or sections are recommended:

- **General Project README:** Explaining what CBWeb3 is, its purpose, main components, and project status. It should include basic instructions for getting started (how to install or where to find more information).

- **Technical Deployment Guide:** Steps to deploy the CBWeb3 test net or node. Include system requirements, Hyperledger Besu node configuration on LACChain, how to connect to the sandbox, etc. Ideally, provide scripts or containers (Docker, Kubernetes) to facilitate installation.

- **API and Smart Contract Documentation:** Document smart contract interfaces and APIs for integration (e.g., for issuing tokens, making payments). An API reference or example contract calls (ABI, etc.) could be published.

- **Architecture and Design Description:** A technical document explaining the system architecture: modules (e.g., CBDC issuance module, payment module, exchange rate oracles, integration with banking systems, etc.), how they interact, transaction flow diagram, security considerations, etc.

- **Functional Guide (Use Cases):** Explain with concrete cases how a central bank would use the sandbox, how a currency is tokenized, how an international transfer and settlement are carried out, etc., in simple terms. This helps both experts and non-experts grasp the value.

- **Contribution and Governance**: Include a section on how to contribute (given that it is open source) and how the project's governance is handled (this links to criterion 3). Also reference the adopted Code of Conduct (LF Labs) for project participants.

### Data extraction and sharing mechanisms (non-PII)

**Status:** Since CBWeb3 focuses on tokenized financial transactions, it will handle transaction data, balances, issuance records, etc. Most of this data will not be personal data, but rather operational data of the network (e.g., records on the blockchain). By the very nature of the blockchain technology used, much of the data is already accessible in a standard way: in Hyperledger Besu/Ethereum, any node or client can extract transaction and block information via open JSON-RPC or GraphQL APIs. This means that the system de facto allows querying and exporting data in open structured formats (JSON) without relying on proprietary software. To ensure auditability, CBWeb3 must provide block explorers or dashboards that display activity data, indicating that the information is publicly available for auditing (at least within the consortium). However, to formally comply with this criterion, it is necessary to verify what type of data CBWeb3 will handle and how it can be extracted:

- If it will only handle public transactional data on the blockchain, then it is already largely compliant (since anyone with node access can export it).

- If it will also handle configuration data or reports (for example, lists of participants or statistics), it would be necessary to ensure that they are also offered in open formats.

- If it handles personal data (PII) – for example, user identity information in KYC processes – that would not fall under this criterion (which refers to non-PII data), but would fall under privacy (criterion 7). For non-personal data, the focus would be on operational data, performance, etc.

**What is missing or needs to be improved**: To align with this point, the mechanisms for importing/exporting non-sensitive data should be explicitly described. Some recommendations:

- **Document APIs and formats**: In the technical documentation, include a Data Export/Interoperability section, explaining how transaction records can be obtained, preferably with examples. For example: "CBWeb3 exposes transactions and account statements through the Ethereum JSON-RPC API (JSON format), allowing transaction listings to be exported to CSV/JSON." If there is a batch interface for extracting data, mention it.

- **Clarify data scope**: Ensure that any non-sensitive data (e.g., daily transaction volumes, performance metrics) can be easily downloaded. You could even provide sample datasets (in CSV/JSON) with test data from the sandbox network for analysis purposes, without including any PII.

- **Importing data**: If the platform allows importing any data (for example, uploading a set of test transactions or integrating parameters such as exchange rates), also indicate that this is done using open formats.

In short, it must be demonstrated that “**non-personal data can be exported in non-proprietary formats.**” Given the open nature of the underlying blockchain, this should be straightforward to justify, but it is important to include it in the DPG request to meet the criteria.

(*Note*: In the case of CBWeb3, much of the relevant data is likely to be simulated financial transactions; still, it is worth clarifying that there is nothing preventing entities from extracting their information and using it outside the platform.)

### Privacy and legal compliance

**Current Status**: This criterion addresses the protection of personal data and compliance with applicable laws (national and international), especially privacy. In CBWeb3, the privacy issue primarily arises if the project involves user identification or the handling of personal data in the context of CBDC testing. Given that this is a central bank environment, certain use cases may require identifying participants (e.g., KYC verification of users testing the platform, or ensuring that only authorized personnel access the sandbox).

This would involve handling sensitive personal data (identities, documents, etc.) to verify users. Decisions regarding the type of data handled will be made within the DPG WG. In addition, there is a dedicated team for research and testing to ensure privacy compliance in the solution. However, to be a DPG, attention to this aspect must be demonstrated:

CBWeb3 will not collect PII directly (all tests will be conducted with fictitious data or with institutions, not individuals), something that must be publicly specified.

In addition to privacy, "*other applicable laws*" could include financial regulations, cybersecurity, accessibility, etc. Given the multilateral nature of the law, it is difficult to list them all, but at least demonstrating compliance with global standards (e.g., GDPR for citizen data) would be important.

**What is missing or needs to be improved**: To meet this criterion, CBWeb3 must:

- **Develop and publish a Privacy Policy** for the project, even if it is a test project. Include what personal data (if any) is collected, for what purpose, how it is stored, who has access and for how long, and how it is secured. Indicate that it is currently a controlled environment but is committed to respecting privacy principles.

- **Cite compliance with relevant laws**: Given that the participants are institutions from different countries, mention that the data protection laws of those countries and equivalent good practices are followed. Compliance with financial regulations applicable to the sandbox could also be mentioned (e.g., central bank guidelines on CBDC testing, if they exist). In general, it demonstrates awareness of the legal framework.

- **Anonymization in test environments**: A good practice is to ensure that, in the sandbox, any personal data is protected or anonymized. If, for example, account holders were simulated, anonymous identifiers could be used on the blockchain (typically, only cryptographic addresses are on-chain, but if there is personal metadata off-chain, it must be protected).

- **Information Security**: Although the criterion addresses privacy, it is relevant to mention security measures taken to protect any sensitive information (in transit and at rest), and perhaps certifications or standards followed (ISO 27001, security by design principles, etc.), as this reinforces the "do no harm" approach to individual data.

In short, CBWeb3 must demonstrate compliance with "privacy by design" and relevant laws, or failing that, that it does not process personal data at all. Include links to the privacy policy or terms of use in the documentation. Since the DPGA requires links to such policies if PII is present, this is something the team should prepare before applying.

### Open standards and good practices

**Status**: CBWeb3, being a financial interoperability project, already incorporates several open standards in its design, although they have not been formally listed. Some of them are:

- At the blockchain level, CBWeb3 uses Ethereum standards (for example, ERC-20 or ERC-1400 tokens to represent tokenized currencies or securities). Ethereum is a widely adopted set of open standards.

- Although not something that will be integrated in the initial phase, integration with banking systems will be considered in the future. This will be based on ISO 20022 for financial messaging (which is an open international standard for payments), especially if integrations with payment systems or SWIFT are to be considered. In fact, if integration with SWIFT or others is carried out, it will use ISO 20022 XML or similar formats, which are open.

- In digital identity, there are several standards that will be evaluated: OIDC, DID, and PKI, all of which are or are based on open specifications.

In either case, it is necessary to include a list of implemented open standards and specifications.

**What is missing or needs to be improved**: The team should prepare a list/documentation of standards and principles followed, for example:

- **Open technical standards**: List open protocols, formats, and frameworks used. Example: "CBWeb3 follows Ethereum standards (ERC-20, ERC-721) for tokenization; complies with NIST FIPS security specifications for cryptography (if applicable); adopts the DID decentralized identity framework (if this is the one chosen)," etc. Provide links where possible (e.g., to the definitions of these standards).

- **Principles and best practices**: Mention alignment with recognized principles. For example, the Donor Principles for Digital Development, which include designing with the user, understanding the environment, designing for sustainability, using open software, open data, etc., fit perfectly with the philosophy of a DPG. Highlighting that CBWeb3, due to its open and collaborative design, follows these principles would be positive. Financial industry-specific best practices can also be mentioned, such as adherence to BIS (Bank for International Settlements) recommendations for CBDCs, W3C standards, or Alliance for Financial Inclusion (AFI) guidelines, if applicable.

- **Software and documentation quality**: Highlight the use of version control (Git), continuous integration, contract security testing (results from the external firm hired for this area), etc., as evidence of good technical practices in project development.

The objective is to demonstrate that "the solution complies with open standards, best practices, and recognized principles.

### Do No Harm by Design Approach

**Current situation**: This criterion seeks to ensure that the project has considered and mitigated potential adverse effects on its users or society. Given that CBWeb3 is a financial infrastructure platform, there are several angles to review:

- **Protection of personal data (PII):** This was already discussed in criterion 7, but it is revisited here from a harm perspective: if CBWeb3 collects or stores PII (e.g., citizen data in pilots), measures must be taken to avoid risks such as data breaches, misuse of sensitive information, or privacy violations. It is essential that any PII be properly protected (encryption, access controls) and that its use be minimized to the minimum necessary.

- **Inappropriate content**: This is not particularly applicable in this context, as CBWeb3 is not a public user-generated content platform. There is no "content" in the traditional sense (posts, images) that could be illegal or inappropriate. Therefore, the content moderation aspect would not be relevant, and the project could indicate that it does not apply (N/A) because the platform does not host user content.

- **User interaction and behavior**: Here, "users" would primarily be developers/contributors and participating institutions. In the development community, it is important to prevent harassing or discriminatory behavior. In this regard, since the project is part of the Linux Foundation environment, it is already covered by the LF Decentralized Trust Code of Conduct, which all contributors must follow. This provides a framework against harassment and promotes an inclusive environment. However, it would be worth reiterating.

- **Other potential harm**: The most important consideration is financial or technological risk. While the Do No Harm criterion focuses on social aspects, in this context we might consider: could CBWeb3, by mistake, cause disruption to real systems or lead to bad decisions? Since it is an isolated sandbox, the risk to the public is low. Even so, it is important to ensure that any test results are not misinterpreted and do not cause a loss of confidence. Also, since this is a test environment, it is important to clarify that no real money from end users is involved, thus avoiding potential financial damage.

**What is missing or can be improved**: To demonstrate compliance with this criterion, CBWeb3 should document:

- **Security and privacy measures adopted**: This links to criterion 7, but here they would be presented as measures to prevent harm. For example: "Personal data (e.g., ID documents for KYC) are stored encrypted and used only for validation, being deleted after testing," or "No personal data is written to the public blockchain, only anonymized hashed IDs." This should also document any impact assessments conducted.

- **Code of Conduct and Community Policies**: Make it clear that the project operates under a code of conduct for contributors, providing a link to it (for example, the Hyperledger or Linux Foundation code, which already exists). This ensures that any participant knows how to report inappropriate behavior and that a respectful environment is fostered.

- **Limited scope of the sandbox (security):** Explain that the sandbox is designed in isolation so as not to affect real systems and that it includes controls to prevent misuse. For example, mention that tokenized coins on the test net cannot be moved outside of it (there is no risk of leakage into real markets), and that participants are verified entities, which prevents fraud.

- **Analyze potential social impacts:** An interesting point would be to state that CBWeb3, as an infrastructure, “will not negatively affect any population” but rather seeks to benefit. However, they must be attentive to ethical considerations: for example, ensuring that the technology does not exclude certain groups (making sure that when implemented, they consider the inclusion of unbanked people, etc.). This may seem futuristic, but showing awareness of this is appreciated.

In short, CBWeb3 must provide a “do no harm” report that addresses these areas, even briefly: personal data protection, absence of harmful content, code of conduct for collaborators, and mitigation of any risks associated with their project. Since it is a technical project between institutions, most of it focuses on data and conduct in the development community. Preparing this information will strengthen the DPG application.
