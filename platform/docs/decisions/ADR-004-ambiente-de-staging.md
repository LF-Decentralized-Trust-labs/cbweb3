# ADR-004: Ambiente de staging (topologia e caminho para produção)

**Status**: Proposto
**Data**: 2026-07-23
**Decisores**: Time de arquitetura CBWeb3 (AH/GL) — decisão exige coordenação com LNet
**Findings relacionados**: 12.5 (feedback LNet, Rodada 2)
**Compromisso impactado**: D6 v2 (staging K8s multi-região de longa duração)

---

## Contexto

O D6 v2 comprometeu um ambiente de **staging Kubernetes multi-região de longa
duração**. O estado atual não suporta esse compromisso:

- **Não há manifestos K8s/Helm.** A orquestração de runtime existe apenas em
  Docker Compose (um stack por cenário; `deploy/local/` e templates do toolkit em
  `scenario-a/provisioning/templates/*/docker-compose.yaml`). Não há charts Helm,
  manifestos K8s nem definição de topologia multi-região.

- **A FASE 4 (staging/prod) do toolkit não está implementada.** O próprio toolkit
  declara a limitação:
  - `scenario-a/samples/README.md:327-330`: "PHASE 4 (staging/prod) not
    implemented. Only `environment: local` is accepted by the CLI. The prod
    implementations of `keyProvider` (real KMS) and `certSource` (real CA) are
    stubs that return `ErrNotImplemented`. Promoting to prod will require only
    different values in the manifest — no engine changes."
  - Ou seja, o desenho do toolkit já prevê a promoção a `environment: prod` como
    troca de valores no manifesto, mas as implementações de produção do
    `keyProvider` (KMS real) e do `certSource` (CA real) ainda são stubs.

**Conclusão do contexto**: existe um gap entre o compromisso D6 v2 (staging K8s
multi-região) e a realidade (apenas Docker Compose local; FASE 4 do toolkit não
implementada; KMS/CA de produção como stubs). O finding 12.5 exige uma decisão de
topologia de staging, a divisão de responsabilidades AH/GL versus LNet, e o caminho
para promover o toolkit a `environment: prod`.

---

## Opções

### Opção A — Staging K8s multi-região operado pela LNet, entregas empacotadas por AH/GL

AH/GL entrega os artefatos empacotados (imagens, charts Helm, manifestos, bundles
de join e o toolkit com FASE 4 implementada); a LNet opera a infraestrutura K8s
multi-região (clusters, rede, KMS, CA) e conduz o deploy no ambiente de staging.

**Prós**
- Divisão de responsabilidade alinhada ao modelo de consórcio: quem opera a
  infraestrutura soberana (LNet) mantém controle de KMS/CA/clusters; AH/GL entrega
  software versionado e reprodutível.
- Aproveita o desenho já previsto do toolkit (promoção via manifesto), concentrando
  o trabalho de AH/GL em implementar os provedores de produção, não em operar
  infraestrutura.
- Evita que AH/GL assuma custódia de material sensível (chaves de produção).

**Contras**
- Exige contrato de interface claro entre o empacotamento (AH/GL) e a operação
  (LNet), incluindo requisitos de cluster, versões e segredos.
- Depende da capacidade e do calendário da LNet para operar staging de longa
  duração.

### Opção B — Staging K8s multi-região operado por AH/GL (turnkey)

AH/GL provisiona e opera o ambiente de staging K8s multi-região completo,
entregando um ambiente pronto para a LNet.

**Prós**
- Menor dependência do calendário operacional da LNet no curto prazo.
- AH/GL mantém controle fim-a-fim durante a fase de staging.

**Contras**
- Coloca AH/GL na custódia de KMS/CA e infraestrutura soberana — desalinhado com o
  modelo de consórcio permissionado e com a divisão de confiança esperada por
  bancos centrais.
- Alto custo operacional e de segurança para AH/GL; não escala para produção
  (produção necessariamente é operada pelas autoridades).

### Opção C — Staging em Docker Compose de longa duração (sem K8s por ora)

Estender o Docker Compose atual para um ambiente de staging persistente, adiando o
K8s multi-região.

**Prós**
- Menor esforço imediato; reaproveita o que existe.

**Contras**
- **Não cumpre o compromisso D6 v2** (K8s multi-região de longa duração).
- Docker Compose não modela multi-região nem alta disponibilidade; não é caminho
  válido para produção. Não recomendado como estado final.

---

## Recomendação

**Adotar a Opção A — staging K8s multi-região operado pela LNet, com AH/GL
entregando os artefatos empacotados e implementando a FASE 4 do toolkit.**

Justificativa: é a única opção que cumpre o D6 v2 e respeita o modelo de confiança
do consórcio (autoridades operam a infraestrutura soberana e custodiam KMS/CA;
AH/GL entrega software versionado e reprodutível). Aproveita o desenho já previsto
do toolkit — a promoção a `environment: prod` é troca de valores no manifesto — de
modo que o trabalho de AH/GL concentra-se em substituir os stubs `keyProvider`
(KMS) e `certSource` (CA) por implementações reais e produzir os manifestos/charts,
sem assumir operação de infraestrutura.

**Divisão de responsabilidades recomendada (a coordenar com LNet):**

| Responsabilidade | Owner |
|------------------|-------|
| Toolkit FASE 4 (`environment: staging`/`prod`), provedores KMS/CA reais | AH/GL |
| Charts Helm / manifestos K8s, imagens versionadas, bundles de join | AH/GL |
| Clusters K8s multi-região, rede, custódia de KMS/CA | LNet |
| Operação e deploy no staging de longa duração | LNet |
| Definição de topologia multi-região (regiões, HA, DR) | AH/GL + LNet (conjunto) |

---

## Plano de implementação

1. **Acordo de topologia com a LNet** — definir em conjunto o número de regiões, o
   modelo de alta disponibilidade/DR, o mapeamento hub/spoke por região e os
   requisitos de cluster. Registrar como anexo deste ADR.
2. **Formalizar a divisão de responsabilidades** — validar a tabela acima com a
   LNet (quem entrega vs. quem opera), incluindo o processo de handover de
   artefatos.
3. **Implementar a FASE 4 do toolkit** — substituir os stubs que retornam
   `ErrNotImplemented` (`keyProvider` → KMS real; `certSource` → CA real) e
   habilitar `environment: staging`/`prod` na CLI (`scenario-a/toolkit/`), com
   testes falhando antes (test-first).
4. **Produzir manifestos/charts K8s** — criar charts Helm/manifestos derivados dos
   templates de Docker Compose existentes, parametrizados por região; garantir que
   não dependem de `deploy/local` (que permanece a rede de amostra).
5. **Validar a promoção via manifesto** — comprovar que promover de `local` para
   `staging` é troca de valores no manifesto, conforme o desenho declarado, sem
   mudanças no engine.
6. **Piloto de staging multi-região** — executar um deploy piloto (hub + spokes em
   regiões distintas) coordenado com a LNet, validando join cross-região, relay e
   sincronização QBFT.
7. **Caminho para `environment: prod`** — documentar os requisitos remanescentes
   (custódia de chaves, CAs dos bancos centrais, hardening) e registrar o
   fechamento do finding 12.5.

---

## Escopo desta decisão e relação com T-P1-28

Este ADR decide a **topologia e a divisão de responsabilidades** de staging — é o
precursor de **go/no-go de topologia (Day-5)**, não a entrega de T-P1-28. A tarefa
T-P1-28 (K8s multi-região persistente com reconciliação, mTLS e onboarding CEMLA
operantes, mais runbook de staging) é **implementação** subsequente a esta decisão e
**permanece aberta**. As CRDs do operador Paladin em
`scenario-b/deploy/local/paladin/contracts/` são um ativo para o trabalho de charts
(passo 4).

---

## Esforço e cronograma (estimativa preliminar — a confirmar pelo time)

| Fase | Escopo | Esforço estimado |
|------|--------|------------------|
| Acordo de topologia + responsabilidades (passos 1–2) | Coordenação com LNet | ~1 semana + agenda LNet (externo) |
| FASE 4 do toolkit (passo 3) | KMS/CA reais + `environment: staging`/`prod` | ~4–6 semanas-dev |
| Charts/manifestos K8s (passo 4) | Helm/manifestos parametrizados por região | ~3–4 semanas-dev |
| Promoção via manifesto + piloto (passos 5–6) | Validação + deploy piloto multi-região | ~2–3 semanas-dev + agenda LNet |
| Caminho para prod (passo 7) | Documentação de hardening/custódia | ~1 semana-dev |

Estimativa total: **~10–14 semanas-dev**, dependentes do acordo de topologia com a
LNet. Datas-alvo a definir em conjunto com a LNet.

---

## Status / Sign-off

| Parte | Papel | Decisão | Data |
|-------|-------|---------|------|
| Time de arquitetura CBWeb3 (AH/GL) | Autor | Proposto | 2026-07-23 |
| LNet | Acordo de topologia e operação de staging | Pendente | — |
| IDB | Ciência da divisão de responsabilidades | Pendente | — |

O Status permanece **Proposto** até que o acordo de topologia com a LNet esteja
registrado.
