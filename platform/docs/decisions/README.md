# Registros de decisão de arquitetura (ADRs)

Este diretório contém os ADRs de **nível de plataforma** decididos para os findings
P1 da Rodada 2 (feedback IDB/LNet). Cada ADR segue a mesma estrutura: contexto (com
evidência `arquivo:linha` do código atual), opções com trade-offs, recomendação,
plano de implementação, esforço/cronograma e tabela de Status / Sign-off.

## Índice

| ADR | Título | Finding(s) | Status |
|-----|--------|-----------|--------|
| [ADR-001](ADR-001-privacidade-vs-auditabilidade.md) | Privacidade versus auditabilidade nos tokens de privacidade | A-ARCH-3 / 7.3 | Proposto |
| [ADR-002](ADR-002-onboarding-governanca-portal.md) | Modelo-alvo de onboarding pelo portal de governança | A-ARCH-2 / 7.2 | Proposto |
| [ADR-003](ADR-003-privacidade-no-hub.md) | Privacidade no hub do Scenario B | 9.2 | Proposto |
| [ADR-004](ADR-004-ambiente-de-staging.md) | Ambiente de staging (topologia e caminho para produção) | 12.5 | Proposto |
| ADR-005 (a abrir) | Escalabilidade N-spoke do Scenario A (A-ARCH-1) | A-ARCH-1 / T-P1-15 | A abrir — promover `docs/scenario-a-n-spoke-scalability-plan.md` |
| [ADR-006](ADR-006-isolamento-de-rede-do-hub.md) | Isolamento de rede do hub do Scenario B | R1-11.4 | Aceito e implementado |
| [ADR-007](ADR-007-rebase-de-chain-ids.md) | Rebase dos chain IDs (80000+ → 1337+) | R1-11.4 | Aceito; coordenação LNET pendente |
| [ADR-008](ADR-008-sessao-por-cookie-nos-portais.md) | Sessão por cookie HttpOnly nos portais | R1-11.4 | Aceito e implementado |

> **ADR-005 é bloqueante para a Wave 3.** T-P1-15 (A-ARCH-1) gate T-P1-16 e T-P1-19.
> O documento `docs/scenario-a-n-spoke-scalability-plan.md` é um plano de design
> sólido, mas ainda é um rascunho de time (sem opções, recomendação nem status de
> sign-off); deve ser promovido para este diretório como ADR-005 na mesma estrutura.

## ADRs retrospectivos (006 a 008)

Os ADRs 001 a 004 são **propostas** aguardando sign-off. Os 006 a 008 são **registros
retrospectivos**: documentam decisão já tomada e implementada, escritos porque o finding
R1-11.4 apontou que decisões estruturais da plataforma não tinham ADR. Decisão
implementada sem registro é indistinguível de acidente para quem chega depois.

Por isso a estrutura deles difere: não há "recomendação a aprovar", há decisão em vigor,
as alternativas que existiam no momento, e as consequências — inclusive as ruins. Onde
resta dependência externa (acordo com a LNET no ADR-007) ou lacuna de desenho (defesa
CSRF no ADR-008), está declarada em vez de omitida.

Duas das cinco decisões que o R1-11.4 nomeia **já estavam cobertas** e não geraram ADR
novo: `Zeto_Anon` e a não-adoção do Noto, ambas decididas no
[ADR-001](ADR-001-privacidade-vs-auditabilidade.md), cuja Opção B (implementar o domínio
Noto) foi explicitamente rejeitada em favor de Zeto com encryption-to-authority.

## Índice de viewpoints

O outro item do R1-11.4 — índice de viewpoints IEEE 1016 / ISO 42010 — está em
[`../architecture/viewpoints.md`](../architecture/viewpoints.md), que mapeia stakeholder →
preocupação → viewpoint → documento existente, e declara as lacunas que restam.

## Nota de escopo de numeração

A numeração `ADR-00N` deste diretório é de **nível de plataforma** e independente de
outros ADRs no repositório com escopo local, especificamente:

- `scenario-a/provisioning/docs/adr-001-cross-stack-enode-addressing.md`
- `scenario-a/provisioning/spikes/spk-02-live-join/docs/adr-002-live-validator-join.md`

Esses documentos usam numeração própria no escopo do toolkit/provisionamento e
**não** colidem com os ADRs de plataforma deste diretório, apesar do prefixo
numérico coincidente.

## Sign-off

Os ADRs 001, 002 e 003 exigem **sign-off escrito de IDB/LNet** (e CEMLA para o 002)
antes do início da implementação; o ADR-004 exige um **acordo de topologia com a
LNet**. O campo Status de cada documento só sai de **Proposto** quando a respectiva
tabela de Sign-off estiver preenchida com parte, decisão e data.

## Idioma

Estes ADRs estão em PT-BR. Como o conjunto será enviado a IDB/LNet, a consistência
de idioma de todo o diretório `docs/` (PT-BR vs. inglês) é uma decisão de
stakeholder ainda em aberto e fora do escopo destes documentos.
