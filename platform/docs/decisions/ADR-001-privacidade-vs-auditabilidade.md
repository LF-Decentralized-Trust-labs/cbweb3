# ADR-001: Privacidade versus auditabilidade nos tokens de privacidade

**Status**: Proposto
**Data**: 2026-07-23
**Decisores**: Time de arquitetura CBWeb3 (AH/GL) — pendente sign-off IDB/LNet
**Findings relacionados**: A-ARCH-3 / 7.3 (feedback LNet, Rodada 2)
**Requisito impactado**: REQ-COM-005 (auditabilidade de transações de valor)

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
  - `scenario-a/provisioning/templates/central-bank/paladin-config/bank/config.yaml.tmpl:53`
    (`- name: "Zeto_Anon"`)
  - Mesmo padrão nos demais templates: `.../central-bank/config.yaml.tmpl` e
    `.../commercial-bank/paladin-config/commercial-bank/config.yaml.tmpl`.
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

Adotar uma variante Zeto que cifra o payload da transação para uma autoridade
(por exemplo `Zeto_AnonNullifierEnc` ou `Zeto_AnonEnc`), na qual cada transação
carrega um ciphertext endereçado à chave pública de uma autoridade de auditoria,
e construir o caminho de disclosure/decrypt correspondente.

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

**Adotar a Opção A — migrar para Zeto com encryption-to-authority
(`Zeto_AnonNullifierEnc`) e construir o caminho de disclosure/decrypt.**

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

1. **Prova de conceito de domínio** — validar `Zeto_AnonNullifierEnc` em ambiente
   local isolado: implantar o domínio, executar deposit/transfer/withdraw e
   confirmar a emissão do ciphertext endereçado à autoridade.
2. **Definição da chave de autoridade** — especificar o modelo de custódia e
   quórum da chave de auditoria (documento anexo a este ADR, com sign-off
   IDB/LNet). Decidir se a chave é única do banco central ou de quórum n-de-m.
3. **Atualizar templates do toolkit** — trocar `Zeto_Anon` por
   `Zeto_AnonNullifierEnc` nos três `config.yaml.tmpl`
   (`scenario-a/provisioning/templates/*/paladin-config/*/`) e parametrizar a
   chave pública de autoridade via manifesto.
4. **Substituir os stubs `PaladinBypass`** — implementar `PaladinClient` real em
   `scenario-a/backend/shared/blockchain/privacy/`, cobrindo `TransferZeto` (com
   emissão do ciphertext) e os métodos HTLC; escrever teste falhando antes
   (test-first) e validar hash real (não zero).
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
