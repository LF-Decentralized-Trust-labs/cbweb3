# ADR-002: Modelo-alvo de onboarding pelo portal de governança

**Status**: Proposto
**Data**: 2026-07-23
**Decisores**: Time de arquitetura CBWeb3 (AH/GL) — pendente alinhamento IDB/LNet/CEMLA
**Findings relacionados**: A-ARCH-2 / 7.2 (feedback LNet, Rodada 2)

---

## Contexto

O onboarding de participantes evoluiu de forma desalinhada entre duas superfícies:
o **toolkit de provisionamento (CLI)** e o **portal de governança (UI web)**.

- **O bootstrap de banco central por script tornou-se obsoleto.** O toolkit
  cobre esse fluxo de forma declarativa:
  - `mode: found` (fundação da rede pelo banco central) e o passo
    `scenario-a/toolkit/engine/orchestrator/step_onboard_registry.go` registram o
    participante no `IdentityRegistry` on-chain durante o provisionamento, sem
    depender de scripts imperativos de bootstrap.
  - O onboarding é hoje **dirigido por operador** via CLI (`cbweb3 apply`),
    referenciado em `scenario-a/samples/README.md`. Não há passo self-service pelo
    portal.

- **A UI de registro/emissão de credencial/CSR está comentada** em ambos os
  cenários:
  - `scenario-b/frontend/apps/governance/src/pages/RegistryPage.tsx`: o handler
    `onIssue` está comentado (linhas 61-80) e o card "Issue Credential" com o
    formulário de emissão e o fluxo de confirmação está inteiramente comentado
    (linhas 95-176, bloco `{/* <Card> ... </Card> */}`).
  - `scenario-a/frontend/apps/governance/src/pages/RegistryPage.tsx` apresenta o
    mesmo padrão (handler `onIssue` comentado na linha 61; card de emissão
    comentado a partir da linha 95).
  - O que permanece **ativo** na página é a listagem read-only ("Compliance
    Registry") e o fluxo de **"Pending KYC Approvals"** (`onApproveKyc`,
    `scenario-b/.../RegistryPage.tsx:82-94`), que aprova KYC com motivo obrigatório
    e gera entrada de auditoria.

**Conclusão do contexto**: existem hoje dois caminhos parcialmente sobrepostos —
o toolkit (operador, CLI, declarativo) faz o registro on-chain; o portal tem um
fluxo de aprovação de KYC ativo, mas a emissão de credencial/CSR pela UI foi
desativada (comentada). Falta uma decisão sobre qual é o **modelo-alvo de
onboarding pelo portal** e a matriz de autorização por passo — questão levantada
pela LNet no finding 7.2 e relevante ao modelo de governança discutido com a CEMLA.

---

## Opções

### Opção A — Portal como orquestrador de política; toolkit como executor

O portal de governança conduz o fluxo de **decisão** (KYC, autorização, emissão
de credencial) e delega a **execução** on-chain ao toolkit, invocando o mesmo
caminho de `step_onboard_registry` por trás de uma API de gateway. A UI comentada
de "Issue Credential" é reativada, mas apenas como frontend do fluxo já
implementado no toolkit — sem reimplementar a lógica de registro.

**Prós**
- Fonte única de verdade para o registro on-chain (o toolkit), evitando duas
  implementações divergentes da mesma escrita no `IdentityRegistry`.
- Separação limpa: portal = política/aprovação humana; toolkit = execução
  determinística e auditável.
- Aproveita o fluxo de KYC já ativo e a trilha de auditoria existente.
- Modelo compatível com governança CEMLA (aprovação humana explícita antes da
  emissão).

**Contras**
- Requer expor o caminho do toolkit por trás de uma API de gateway (o toolkit foi
  desenhado como CLI de operador), com autenticação/autorização adequadas.
- Reativar a UI exige reconciliar o contrato de `issueCredential` com a assinatura
  atual do backend/toolkit.

### Opção B — Reativar a UI de emissão como caminho autônomo (portal self-service completo)

Reativar `onIssue` e o card de emissão para que o portal execute emissão de
credencial/CSR de forma autônoma, independente do toolkit.

**Prós**
- Self-service completo pela UI; onboarding sem operador de CLI.
- Menor atrito para participantes que já passaram no KYC.

**Contras**
- **Duplica a lógica de registro** já implementada no toolkit
  (`step_onboard_registry.go`), criando risco de divergência de comportamento
  entre o caminho CLI (`cbweb3 apply`) e o caminho UI.
- Emissão de credencial de participante é um ato de governança sensível; permitir
  execução autônoma pela UI amplia a superfície de autorização e exige controles
  fortes (aprovação de quórum, MFA, trilha) que hoje não existem no card comentado.
- Vai contra a direção de tornar o toolkit a fonte declarativa de onboarding.

### Opção C — Manter apenas o toolkit (portal permanece read-only + KYC)

Formalizar o toolkit CLI como único caminho de onboarding e remover
definitivamente a UI comentada de emissão, mantendo no portal apenas a listagem e
a aprovação de KYC.

**Prós**
- Menor esforço; consolida o que já funciona.
- Onboarding 100% declarativo e auditável via manifesto.

**Contras**
- Onboarding continua dependente de operador de CLI — não atende à expectativa de
  self-service pelo portal levantada no finding 7.2.
- Deixa código morto comentado no repositório (dívida técnica de UI) a menos que
  seja efetivamente removido.

---

## Recomendação

**Adotar a Opção A — portal como orquestrador de política, toolkit como executor.**

Justificativa: preserva o toolkit como fonte única de verdade do registro on-chain
(evitando a divergência que a Opção B introduziria), reaproveita o fluxo de KYC já
ativo e entrega a experiência de portal que o finding 7.2 pede, sob controle de
governança compatível com a CEMLA. A UI comentada de "Issue Credential" é
reativada como frontend do fluxo do toolkit, não como reimplementação.

**Matriz de autorização recomendada (a validar com IDB/LNet/CEMLA):**

| Passo | Autorizado | Superfície |
|-------|-----------|------------|
| Submissão de solicitação de participação / CSR | Instituição candidata | Portal (self-service) |
| Revisão e aprovação de KYC | Operador de compliance | Portal (fluxo já ativo) |
| Emissão de credencial / registro on-chain | Governança (quórum banco central) | Portal aciona → toolkit executa |
| Fundação da rede (`mode: found`) | Operador do banco central | CLI (`cbweb3 apply`) |

---

## Plano de implementação

1. **Definir a matriz de autorização** — validar com IDB/LNet/CEMLA quem autoriza
   cada passo (tabela acima), formalizando o requisito de quórum para a emissão de
   credencial. Registrar como anexo deste ADR.
2. **Expor o caminho do toolkit via API de gateway** — publicar uma operação
   autenticada (Keycloak OIDC) que aciona a lógica de `step_onboard_registry`,
   preservando a passagem obrigatória pelo compliance gate do API gateway (sem
   bypass em chamadas serviço-a-serviço).
3. **Reconciliar o contrato `issueCredential`** — alinhar a assinatura esperada
   pela UI comentada (`entityName`, `legalEntityId`, `scopes`, `reason`) com a API
   do passo 2.
4. **Reativar a UI de emissão** — descomentar e adaptar `onIssue` e o card "Issue
   Credential" em ambos os `RegistryPage.tsx` (Scenario A e Scenario B),
   condicionando a ação ao papel de governança e ao motivo obrigatório já usado no
   fluxo de KYC.
5. **Encadear KYC → emissão** — garantir que a emissão de credencial só fique
   disponível para participantes com KYC aprovado, ligando o fluxo `onApproveKyc`
   existente à nova ação de emissão.
6. **Trilha de auditoria** — assegurar entrada imutável de auditoria em cada
   emissão (o próprio comentário da UI já promete "immutable audit entry").
7. **Testes e documentação** — cobrir o fluxo com testes de frontend e de gateway,
   atualizar o README de governança e o `samples/README.md` para descrever o
   modelo portal + toolkit, e registrar o fechamento do finding 7.2.
