# SDG alignment

*DPG Standard, indicator 1 — Relevance to the Sustainable Development Goals.*

CBWeb3 is public infrastructure for **wholesale** cross-border settlement between central
banks and commercial banks in Latin America and the Caribbean. Its contribution to the
2030 Agenda is therefore indirect but specific: it attacks the cost, speed and
counterparty risk of the correspondent-banking layer that sits underneath every
cross-border payment in the region, and it does so as an open, co-governed common rather
than as a proprietary product.

The causal chain is stated plainly so it can be judged: CBWeb3 does not itself lower the
price a migrant pays to send money home. It lowers the cost and settlement risk borne by
the institutions that intermediate that payment, and it removes the licensing and
vendor-dependency cost of the infrastructure itself for any central bank in the region
that adopts it.

## Primary targets

### Target 10.c — reduce remittance transaction costs

> *Reduce to less than 3 per cent the transaction costs of migrant remittances and
> eliminate remittance corridors with costs higher than 5 per cent.* (Indicator 10.c.1)

Remittance pricing in the region is driven substantially by correspondent banking: prefunded
nostro/vostro accounts, multi-day settlement, FX spreads and the credit risk of
non-simultaneous exchange. CBWeb3's Enhanced Correspondent Banking flow settles both legs
of a currency exchange atomically through Hash Time-Lock Contracts across two sovereign
networks — neither side can take the other's funds without releasing its own. Eliminating
principal risk and the prefunding it forces is a direct input to the cost base that
10.c.1 measures.

### Target 8.10 — strengthen domestic financial institutions

> *Strengthen the capacity of domestic financial institutions to encourage and expand
> access to banking, insurance and financial services for all.*

Smaller central banks and commercial banks in the region cannot individually fund the
design, security review and operation of tokenized settlement infrastructure. CBWeb3
supplies that capacity as a shared, openly licensed reference implementation with public
documentation, conformance test vectors and a sandbox, so that the capability does not
depend on the size of the institution's technology budget.

### Target 9.1 — resilient, sustainable regional infrastructure

> *Develop quality, reliable, sustainable and resilient infrastructure … to support
> economic development and human well-being, with a focus on affordable and equitable
> access for all.*

The hub-and-spoke topology is deliberately non-extractive: each country operates its own
domestic network and retains sovereignty over its own ledger and monetary policy, while
interoperating through a shared settlement layer. There is no central operator able to
charge rent, exclude a participant, or discontinue the system. The networks are gasless
Hyperledger Besu QBFT chains, so participation carries no per-transaction fee and no
proof-of-work energy cost.

### Target 17.6 and 17.16 — cooperation and multi-stakeholder partnership

> *Enhance … cooperation on and access to science, technology and innovation …* /
> *Enhance the global partnership for sustainable development, complemented by
> multi-stakeholder partnerships.*

CBWeb3 is built by exactly such a partnership: IDB Lab as sponsor, LNet as infrastructure
coordinator, CEMLA and FLAR as regional monetary and financial advisers, central banks as
participants, and LF Decentralized Trust as the neutral open-source host. The working
group is open to any individual or organisation, its decisions are taken in public on
GitHub, and every technical artifact — design documents, contracts, test vectors, research
papers — is published under an open licence as it is produced.

## Secondary contributions

| Target | How CBWeb3 relates to it |
|---|---|
| **8.3** — formalisation and growth of MSMEs | Cheaper, faster cross-border settlement lowers a fixed cost that falls hardest on small exporters and importers. |
| **9.3** — access of small enterprises to financial services | Same mechanism, from the access rather than the cost side. |
| **16.6** — effective, accountable, transparent institutions | Settlement is recorded on an auditable ledger; supervisory access is role-separated and every privileged action is logged. See [Do no harm](do-no-harm.md). |
| **17.9** — capacity building | Public manuals, a conformance suite and a sandbox exist so institutions can build competence without a vendor engagement. |

## Honest limits

- CBWeb3 is a **test network**. No SDG outcome has been measured, because no live
  economic activity runs on it. What can be evidenced today is the openness and
  reusability of the infrastructure, not its realised impact.
- The link to target 10.c is structural, not attributable: a fall in remittance prices
  would depend on the pricing decisions of the institutions using the rails, not on the
  rails alone.
- Retail financial inclusion is explicitly **out of scope**. CBWeb3 serves institutions;
  it is not a consumer wallet and does not reach end users directly.
