# ADR-001: Privacidade versus auditabilidade nos tokens de privacidade

**Status**: Proposto
**Data**: 2026-07-23
**Decisores**: Time de arquitetura CBWeb3 (AH/GL) — pendente sign-off IDB/LNet
**Findings relacionados**: A-ARCH-3 / 7.3 (feedback LNet, Rodada 2)
**Requisito impactado**: REQ-COM-005 (auditabilidade de transações de valor)

---

## Pedido de decisão

Este bloco existe para que a decisão possa ser tomada sem ler o ADR inteiro. O corpo
abaixo continua sendo a fundamentação.

| | |
|---|---|
| **O que se pede** | Aprovar a migração `Zeto_Anon` → `Zeto_AnonNullifierEnc` (encryption-to-authority) e a construção do caminho de disclosure, **ou** rejeitar em favor da Opção B (domínio Noto), **ou** rejeitar ambas e aceitar formalmente que valores privados não são auditáveis por meio criptográfico nesta fase. |
| **Quem assina** | IDB e LNet — a decisão define quem detém a chave de autoridade e sob que governança ela é usada. |
| **Recomendação a aprovar** | Opção A (ver §Recomendação). |
| **Se aprovado, desbloqueia** | `[R1-§7.3 / R2-A-ARCH-3]` (implementação do modelo de token) e a parte de auditoria de `[R1-9.2]`. |
| **Se não for aprovado** | O REQ-COM-005 continua sem meio criptográfico de cumprimento e isso precisa ser registrado como limitação aceita, com sign-off — não como pendência técnica. A implementação permanece bloqueada em qualquer caso: começar antes da decisão é construir a opção que pode ser rejeitada. |
| **Evidência reverificada** | `develop` @ `bfa89aa1`, 2026-08-22. `Zeto_Anon` e o stub `PaladinBypass` continuam em vigor; duas citações de caminho foram corrigidas após a retirada do caminho legado `deploy/local` (ver notas no corpo). |

---

## Contexto

A constituição do projeto exige que transferências de valor interbancárias usem
tokens de privacidade (`ZetoToken` via Paladin/Zeto com ZKP, ou `NotoToken` via
Paladin/Noto com notário) e que nenhum PII ou valor em texto claro seja gravado
on-chain. Em paralelo, o REQ-COM-005 exige **auditabilidade** das transações de
valor por autoridade competente (banco central / supervisor).

Hoje existe uma tensão não resolvida entre esses dois objetivos:

- **O domínio Zeto implantado é `Zeto_Anon`** (anonimato puro), sem canal de
  auditoria embutido. Isso vale inclusive para os novos templates do toolkit:
  - `scenario-a/provisioning/templates/central-bank/paladin-config/bank/config.yaml.tmpl:62`
    (`- name: "Zeto_Anon"`)
  - Mesmo padrão, na mesma linha, nos demais templates:
    `.../central-bank/paladin-config/central-bank/config.yaml.tmpl:62` e
    `.../commercial-bank/paladin-config/commercial-bank/config.yaml.tmpl:62`.
  - *(Linha corrigida de 53 para 62 na reverificação de 2026-08-22; o conteúdo da
    citação não mudou.)*
  - `Zeto_Anon` não emite ciphertext endereçado a uma autoridade; o supervisor
    não tem meio criptográfico de reconstruir o valor/participantes de uma
    transação a partir da cadeia.

- **Noto é apenas stub.** A implementação de privacidade em Scenario A é um
  no-op que retorna hash zero:
  - `scenario-a/backend/shared/blockchain/privacy/paladin.go:12-28`
    (`PaladinBypass`; `MintNoto`, `TransferZeto`, `CreateNotoHTLC`,
    `ClaimNotoHTLC` retornam
    `0x0000...0000`). O comentário no arquivo (linhas 7-11) declara explicitamente
    que deve ser substituído por um `PaladinClient` real.

- **A "Master Viewing Key" (MVK) é apenas quórum em banco de dados**, sem
  disclosure criptográfico de fato:
  - `scenario-b/backend/services/compliance/internal/services/oversight_service.go:5-7`
    declara na doc do pacote: "Paladin operates exclusively at the spoke level.
    Hub OversightService tracks quorum only (PENDING → QUORUM_REACHED). Actual MVK
    disclosure is a spoke-level operation via Paladin, out of scope here."
  - O fluxo `OpenDisclosure`/`SignDisclosure` (linhas 30-60) gerencia estado e
    quórum de assinaturas de banco central, mas **não decifra nem recupera** o
    conteúdo da transação-alvo. O elo entre "quórum atingido" e "valor revelado"
    não existe.

**Conclusão do contexto**: hoje o sistema oferece privacidade forte
(`Zeto_Anon`) porém **não atende** ao REQ-COM-005 de forma criptograficamente
verificável. A auditoria depende de um caminho de disclosure que ainda não está
implementado em nenhuma das duas pontas (nem geração do material auditável, nem
o decrypt/recuperação).

---

## Opções

### Opção A — Migrar para Zeto com encryption-to-authority (`_Enc`)

Adotar uma variante Zeto da família `_Enc`, na qual cada transação carrega um
ciphertext do payload, e construir o caminho de disclosure/decrypt correspondente.

**Nota de nomenclatura (a validar na PoC).** Os nomes de domínio upstream são
`Zeto_AnonEnc` e `Zeto_AnonEncNullifier` — **não existe** `Zeto_AnonNullifierEnc`.
Além disso, nas variantes `_Enc` "puras" o ciphertext é endereçado ao **receptor**
via segredo compartilhado emissor-receptor, e **não** a uma autoridade de auditoria.
A variante que habilita decifração por uma autoridade (não-repúdio) é
`Zeto_AnonEncNullifierNonRepudiation`. A escolha do domínio precisa ser confirmada
na PoC (passo 1) contra dois requisitos simultâneos: ciphertext endereçado à
autoridade **e** suporte a `lock`/`transferLocked` — este último exigido porque o
caminho HTLC do Scenario A usa o circuito `transferLocked`.

**Prós**
- Mantém a propriedade de privacidade baseada em ZKP já adotada (nulificadores,
  provas de conhecimento zero), sem introduzir um notário centralizado.
- A auditabilidade passa a ser uma propriedade **criptográfica e verificável**:
  o supervisor recupera o conteúdo com a chave de autoridade, ligando o REQ-COM-005
  ao próprio protocolo.
- Alinhado com a variante `Nullifier` que protege contra double-spend, mantendo o
  modelo UTXO privado.
- Mudança concentrada em configuração de domínio + construção do caminho de
  decrypt; não substitui o modelo de token.

**Contras**
- Introduz uma **chave de autoridade** cuja custódia, rotação e governança de
  quórum precisam ser definidas (a MVK deixa de ser "quórum em DB" e passa a ser
  material criptográfico real).
- Overhead de prova/tamanho de transação maior que `Zeto_Anon` puro; exige
  revalidar baseline de performance (`make scenario-b.perf-baseline`).
- O caminho de decrypt precisa ser implementado nas duas pontas (geração do
  ciphertext no fluxo de transferência; recuperação no `OversightService`).

### Opção B — Implementar o domínio Noto (notário com visibilidade)

Substituir os stubs `PaladinBypass` por um `PaladinClient` real usando o domínio
Noto, no qual um notário designado (banco central / autoridade) tem visibilidade
sobre os estados por construção.

**Prós**
- Auditabilidade "nativa": o notário observa os estados, dispensando um caminho
  de decrypt separado.
- Modelo notarial é mais próximo do mental model regulatório tradicional
  (autoridade endossa cada transição).
- Remove a dívida técnica dos stubs `PaladinBypass` que hoje retornam hash zero.

**Contras**
- **Não oferece privacidade baseada em ZKP** entre pares; o notário vê tudo, o
  que concentra confiança e cria um ponto único de exposição de dados sensíveis.
- Diverge da direção já adotada (Zeto em todos os templates) — obriga a manter
  dois domínios ou migrar tudo, aumentando a superfície operacional.
- Exige implementar integração Paladin/Noto do zero (os stubs cobrem apenas a
  assinatura de métodos), incluindo HTLC Noto para o fluxo de atomicidade do
  Scenario A.
- A privacidade fica dependente de política operacional do notário, não de
  garantia criptográfica.

### Opção C — Manter `Zeto_Anon` + disclosure fora da cadeia (status quo estendido)

Formalizar que a auditoria ocorre por dados fora da cadeia (logs/serviços),
mantendo o quórum em DB como gatilho de processo.

**Prós**
- Zero mudança de protocolo; menor esforço imediato.

**Contras**
- **Não satisfaz REQ-COM-005** de forma verificável on-chain — a auditoria passa
  a confiar em dados fora da cadeia que podem divergir do estado real.
- Perpetua a lacuna já apontada pela LNet; risco de reprovação em revisão
  regulatória. Não recomendado como estado final.

---

## Recomendação

**Adotar a Opção A — migrar para uma variante Zeto com encryption-to-authority
(candidato: `Zeto_AnonEncNullifierNonRepudiation`, a confirmar na PoC) e construir o
caminho de disclosure/decrypt.**

Justificativa: preserva o modelo de privacidade por ZKP que já é a direção
arquitetural do projeto (todos os templates usam Zeto), evita concentrar
visibilidade em um notário e transforma o REQ-COM-005 em uma garantia
criptográfica em vez de um controle de processo. A Opção B seria uma reversão da
direção de privacidade e a Opção C não fecha o finding.

Ponto de decisão que exige sign-off IDB/LNet: **governança da chave de
autoridade** (custódia, quórum, rotação) — a MVK passa a ter significado
criptográfico e deve ser desenhada com a autoridade competente.

---

## Plano de implementação

1. **Prova de conceito de domínio** — validar a variante candidata
   (`Zeto_AnonEncNullifierNonRepudiation`) em ambiente local isolado: implantar o
   domínio, executar deposit/transfer/withdraw e **provar dois requisitos**: (a) o
   ciphertext é decifrável pela chave da **autoridade** (não apenas pelo receptor) e
   (b) a variante suporta `lock`/`transferLocked`, exigido pelo caminho HTLC do
   Scenario A. Se a variante escolhida não atender a ambos, a recomendação deve ser
   revista antes de prosseguir.
2. **Definição da chave de autoridade** — especificar o modelo de custódia e
   quórum da chave de auditoria (documento anexo a este ADR, com sign-off
   IDB/LNet). Decidir se a chave é única do banco central ou de quórum n-de-m.
3. **Atualizar a stack de domínio (escopo completo)** — a troca **não** se limita
   ao nome nos templates. É necessário: (a) trocar `Zeto_Anon` pela variante
   escolhida nos três `config.yaml.tmpl`
   (`scenario-a/provisioning/templates/*/paladin-config/*/`, linha 53) e também nos
   14 configs de `scenario-a/deploy/local/paladin/**` e em
   `scenario-b/deploy/local/paladin/spoke-*/config/**`; (b) garantir que o bloco
   `circuits` e os artefatos de prover da variante `_Enc` estão presentes na imagem
   Paladin (hoje **não** há artefato `*_enc` no repositório); (c) implantar o novo
   verifier g16 e os contratos de implementação da variante; (d) re-registrar a
   variante no `ZetoFactory` (hoje
   `scenario-a/provisioning/paladin/scripts/deploy_zeto_factory_test.go` registra
   apenas `Zeto_Anon` — o arquivo saiu de `deploy/local/paladin/scripts/` quando o
   caminho legado foi retirado em `3b14ecaa`, o registro em si não mudou); e (e) parametrizar a chave pública de autoridade via
   manifesto.
4. **Implementar o `PaladinClient` real e conectá-lo a um caminho vivo** — hoje o
   stub `PaladinBypass` (`scenario-a/backend/shared/blockchain/privacy/paladin.go:12-28`)
   é a única implementação de `PrivacyOperator`, mas **não é consumida por nenhum
   serviço vivo** do Scenario A (só pelo próprio teste; ver
   `interfaces.go:12`). Portanto o trabalho não é apenas "trocar o no-op": exige
   implementar `PaladinClient` real cobrindo `TransferZeto` (com emissão do
   ciphertext) e os métodos HTLC **e** cablá-lo em um caminho de transferência real
   (o consumidor precisa ser criado). Escrever teste falhando antes (test-first) e
   validar hash real (não zero).
5. **Construir o caminho de decrypt** — no `OversightService`
   (`scenario-b/.../oversight_service.go`), ligar o evento
   `QUORUM_REACHED` à recuperação criptográfica efetiva do conteúdo da transação,
   substituindo o "out of scope here" por uma operação de disclosure real
   (spoke-level via Paladin).
6. **Revalidar performance e atomicidade** — rodar `make scenario-b.perf-baseline`
   e as suítes E2E (`make scenario-b.test`) para medir o overhead do domínio
   `_Enc` e confirmar que os caminhos de lock/timeout/refund permanecem íntegros.
7. **Atualizar documentação de status** — refletir a mudança nos READMEs de
   cenário e no runbook, e registrar o fechamento do finding 7.3.

---

## Esforço e cronograma (estimativa preliminar — a confirmar pelo time)

| Fase | Escopo | Esforço estimado |
|------|--------|------------------|
| PoC de domínio (passo 1) | Provar ciphertext à autoridade + `transferLocked` | ~1–2 semanas-dev |
| Governança da chave de autoridade (passo 2) | Custódia/quórum/rotação + sign-off IDB/LNet | ~1 semana-dev + sign-off (externo) |
| Stack de domínio (passo 3) | Templates + circuits/prover + verifier/impl + re-registro no factory + deploy/local + Scenario B | ~3–4 semanas-dev |
| `PaladinClient` real + caminho vivo (passo 4) | Implementação + wiring + testes | ~2–3 semanas-dev |
| Decrypt no `OversightService` (passo 5) | Ligar `QUORUM_REACHED` ao disclosure real | ~2 semanas-dev |
| Revalidação de performance/atomicidade (passo 6) | perf-baseline + E2E | ~1 semana-dev |

Estimativa total: **~9–13 semanas-dev**. O passo 2 (governança da chave) é
bloqueante para os passos 3–5. Datas-alvo a definir no planejamento de release e
dependentes do sign-off IDB/LNet.

---

## Status / Sign-off

| Parte | Papel | Decisão | Data |
|-------|-------|---------|------|
| Time de arquitetura CBWeb3 (AH/GL) | Autor | Proposto | 2026-07-23 |
| IDB | Aprovação da governança da chave de autoridade | Pendente | — |
| LNet | Aprovação da governança da chave de autoridade | Pendente | — |

O Status permanece **Proposto** até que as linhas de sign-off acima estejam
preenchidas com decisão e data.
