# Maintainers

This file is the authoritative list of maintainers referenced by
[CONTRIBUTING.md](CONTRIBUTING.md). Roles follow the DPG Working Group model defined
there: **Maintainer Core**, **Cochair** and **Contributor**.

Changes to this file require a pull request approved by the existing Maintainer Core.

## Maintainer Core

Merge pull requests, enforce the workflow, and coordinate with the CBWG.

| Name | GitHub | Affiliation | Area |
|---|---|---|---|
| Carolina Velásquez | [@carolacnet](https://github.com/carolacnet) | LNet | Technical lead — governance, documentation, DPG |

The Maintainer Core is deliberately small while the project's first release stabilises.
Additional seats — one for `platform/`, one for the Toolbox and one for documentation —
are open and are filled by nomination through the process in
[Becoming a maintainer](#becoming-a-maintainer), not by appointment.

## Cochairs

Facilitate meetings and curate the backlog. The two co-chair seats belong to
[CEMLA](https://www.cemla.org/) and [FLAR](https://www.flar.net/), which nominate their
own representatives; the seats are held by those institutions and their current nominees
are recorded here as each nomination is confirmed.

## Platform maintainers

Own `platform/` — the CBWeb3 system code contributed by AguilaHub / GoLedger under
contract with IDB Lab.

| Name | GitHub | Affiliation |
|---|---|---|
| Samuel Venzi | [@samuelvenzi](https://github.com/samuelvenzi) | GoLedger |
| Marcos Sarres | [@goledger](https://github.com/goledger) | GoLedger |
| André Macedo | [@andremacedopv](https://github.com/andremacedopv) | GoLedger |
| Lucas Campelo Santiago | [@lucas-campelo](https://github.com/lucas-campelo) | GoLedger |
| Alexandre Harrison | [@Xandyhoss](https://github.com/Xandyhoss) | GoLedger |
| Luiz Jeronymo | [@LFJeronymo](https://github.com/LFJeronymo) | AguilaHub |

These are the authors of the platform code, listed so that provenance and domain knowledge
are attributable. Merge authority over `platform/` in this repository rests with the
Maintainer Core and with [CODEOWNERS](CODEOWNERS); the engagement under which this code was
contributed is complete.

## Becoming a maintainer

Contributors who have shown sustained, high-quality participation may be nominated by an
existing maintainer. Nomination is by pull request against this file and follows the
decision-making process in [CONTRIBUTING.md](CONTRIBUTING.md): consensus first, with a
GitVote fallback (50% + 1 quorum, simple majority).

## Emeritus

Maintainers who step down are recorded here, with thanks.

| Name | GitHub | Period |
|---|---|---|
| — | — | — |

---

## A note on repository access

Write access to this repository is currently granted through the
`@LF-Decentralized-Trust-labs/cbweb3-maintainers` team, whose membership is broader than
the Maintainer Core listed above — it includes participants from the central banks,
CEMLA, FLAR, nuam and the platform vendor.

Being a member of that team means you can contribute directly; it does not by itself
make you a maintainer. Review authority is defined by [CODEOWNERS](CODEOWNERS) and by
this file.
