<!-- SPDX-License-Identifier: Apache-2.0 -->

# Índice de viewpoints — IEEE 1016 / ISO 42010

Este documento é o **índice de viewpoints** da plataforma: quem são os stakeholders,
que preocupações cada um traz, qual viewpoint responde a cada preocupação, e **em que
documento a view correspondente já existe**.

## Por que este índice existe, e o que ele não é

O finding R1-11.4 registrou que a documentação de arquitetura não tinha índice de
viewpoints nem conjunto de ADRs. O segundo item vem sendo endereçado em
[`../decisions/`](../decisions/). Este arquivo fecha o primeiro.

**Ele não cria documentação nova.** A plataforma já tem visões de arquitetura escritas
— diagramas de componentes, runbooks de deploy, especificação de endpoints, catálogo de
testes. O que faltava era a camada que o IEEE 1016 pede: dizer *qual preocupação de
qual stakeholder* cada documento responde, e onde há lacuna. Um índice que aponta para
documento inexistente é pior que ausência de índice, então cada linha abaixo referencia
arquivo que existe hoje, e as lacunas estão marcadas como lacuna em vez de omitidas.

## Stakeholders e preocupações

| Stakeholder | Preocupação principal |
|---|---|
| **IDB / BID** | Cumprimento dos deliverables contratados; auditabilidade das transações de valor; bem público digital (DPG) |
| **LNET** | Operar a rede: topologia, parâmetros de rede, procedimento de deploy e de cutover |
| **CEMLA** | Modelo de onboarding e governança entre bancos centrais |
| **Banco central (governança)** | Soberania sobre a própria spoke; quem pode emitir, pausar e retomar |
| **Banco central (tesouraria)** | Emissão, tokenização e resgate de moeda de banco central |
| **Supervisor** | Visibilidade e auditoria sem quebrar a privacidade dos participantes |
| **Banco comercial** | Liquidar pagamento transfronteiriço sem expor contraparte nem valor |
| **Operador de NOC** | Saúde da rede, alertas, logs e evidência operacional |
| **Time de plataforma** | Manutenibilidade, isolamento entre cenários, testabilidade |

## Viewpoints e onde cada view está documentada

### 1. Viewpoint de contexto
**Preocupação:** o que é a plataforma, que atores existem, que fronteira ela tem.
**Views existentes:** [`../../scenario-a/docs/architecture/architecture-overview.md`](../../scenario-a/docs/architecture/architecture-overview.md) ·
[`../../scenario-b/docs/architecture/architecture-overview.md`](../../scenario-b/docs/architecture/architecture-overview.md) ·
[`../../README.md`](../../README.md)

### 2. Viewpoint de composição (componentes e conectores)
**Preocupação:** que serviços existem, como se comunicam, onde estão as fronteiras de confiança.
**Views existentes:** `CBWeb3-ComponentDiagram.png` e
`cbweb3-component-diagram-with-paladin.excalidraw` em `docs/architecture/` de cada cenário ·
[`../../scenario-b/docs/architecture/cbweb3-component-documentation-en.md`](../../scenario-b/docs/architecture/cbweb3-component-documentation-en.md)

### 3. Viewpoint de interface
**Preocupação:** contrato externo — endpoints, autenticação, versões.
**Views existentes:** [`../deliverables/D5-endpoint-specification.md`](../deliverables/D5-endpoint-specification.md) ·
[`../deliverables/D5-changelog-v1-to-v2.md`](../deliverables/D5-changelog-v1-to-v2.md) · OpenAPI servido por cada
api-gateway em `/openapi.yaml`

### 4. Viewpoint de dependências e topologia de rede
**Preocupação:** quantas chains, quem valida, como uma spoke alcança o hub.
**Views existentes:** [`ADR-006`](../decisions/ADR-006-isolamento-de-rede-do-hub.md) (isolamento do hub) ·
[`ADR-007`](../decisions/ADR-007-rebase-de-chain-ids.md) (alocação de chain IDs) ·
[`../../scenario-b/docs/runbooks/besu-chainid-migration-notes.md`](../../scenario-b/docs/runbooks/besu-chainid-migration-notes.md) ·
[`../scenario-a-n-spoke-scalability-plan.md`](../scenario-a-n-spoke-scalability-plan.md)

### 5. Viewpoint de privacidade e confidencialidade
**Preocupação:** que valor fica visível on-chain, e para quem.
**Views existentes:** [`ADR-001`](../decisions/ADR-001-privacidade-vs-auditabilidade.md) ·
[`ADR-003`](../decisions/ADR-003-privacidade-no-hub.md)

### 6. Viewpoint de identidade, autenticação e autorização
**Preocupação:** quem pode chamar o quê, e como a sessão é sustentada.
**Views existentes:** [`ADR-008`](../decisions/ADR-008-sessao-por-cookie-nos-portais.md) (sessão por cookie) ·
[`ADR-002`](../decisions/ADR-002-onboarding-governanca-portal.md) (onboarding) ·
[`../../scenario-a/docs/runbooks/identity-registry-role-separation.md`](../../scenario-a/docs/runbooks/identity-registry-role-separation.md)

### 7. Viewpoint de implantação
**Preocupação:** como a rede sobe, com que parâmetros, e como se faz cutover.
**Views existentes:** `docs/runbooks/deployment-runbook.md` e `environment-setup.md` de cada cenário ·
[`../TOOLCHAIN.md`](../TOOLCHAIN.md) (autoridade de versões) ·
[`ADR-004`](../decisions/ADR-004-ambiente-de-staging.md) (staging)

### 8. Viewpoint operacional
**Preocupação:** observar, alertar e provar o que aconteceu.
**Views existentes:** `docs/runbooks/NOC-PORTAL-DOCS.md` (cenário A) ·
[`../../scenario-b/docs/user-manuals/`](../user-manuals/) ·
`tests/integration` (evidência on-chain por passo: hash, bloco, gas)

### 9. Viewpoint de verificação
**Preocupação:** o que é testado, em que nível, e o que não é.
**Views existentes:** `scenario-{a,b}/tests/TEST-CATALOG.md` ·
`docs/test-execution-plan.md` de cada cenário · [`../TEST-REPORTS.md`](../TEST-REPORTS.md) ·
`scenario-{a,b}/tests/integration/` (a suíte E2E do cenário A vive aqui, sob a tag
`integration`, e não em `tests/e2e/` — ver PR #155, que acrescenta o ponteiro
explicando isso; a referência direta será incluída quando ela mergear)

### 10. Viewpoint de segredos e material criptográfico
**Preocupação:** onde vivem chaves e senhas, e o que nunca vai para disco.
**Views existentes:** [`../secret-management.md`](../secret-management.md) ·
[`../deliverables/D11-secret-scan-report.md`](../deliverables/D11-secret-scan-report.md)

### 11. Viewpoint de variação entre cenários
**Preocupação:** o que difere entre Scenario A e Scenario B, e se a diferença é deliberada.
**View existente:** [`../scenario-drift.md`](../scenario-drift.md)

## Lacunas conhecidas

Declaradas porque um índice honesto aponta o que falta:

| Lacuna | Situação |
|---|---|
| **Viewpoint de informação/dados** | Não há view dedicada ao modelo de dados persistido (tabelas por serviço, retenção, particionamento). Existe de forma dispersa nas migrations e em specs de feature. |
| **Viewpoint de desempenho** | Existem baselines (`tests/performance/`) mas nenhuma view que declare orçamento de latência por caminho. O ADR de deadlines por RPC mediu p95/p99 de três adapters; o caminho de escrita do swap segue sem medição. |
| **ADR-005 (escalabilidade N-spoke)** | Reservado no índice de decisões; o material existe em `docs/scenario-a-n-spoke-scalability-plan.md` e precisa ser promovido à estrutura de ADR. |
| **Idioma** | Os ADRs e este índice estão em PT-BR; parte de `docs/` está em inglês. Decisão de stakeholder ainda aberta, registrada no README de decisions. |

## Como manter isto verdadeiro

Ao adicionar documento de arquitetura, acrescente a linha no viewpoint correspondente.
Ao tomar decisão que altere uma view, registre ADR em [`../decisions/`](../decisions/) e
referencie-o aqui. Referência para arquivo que não existe é pior que lacuna declarada —
lacuna avisa o leitor, referência morta engana.
