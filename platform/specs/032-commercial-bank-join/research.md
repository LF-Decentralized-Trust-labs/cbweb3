# Research: TK-8 e TK-9 — Commercial Bank Join

**Feature**: `032-commercial-bank-join`
**Date**: 2026-06-27
**Status**: Complete — todos os NEEDS CLARIFICATION resolvidos

---

## D1 — PKI do banco comercial: geração de CSR e submissão ao CB

**Decisão**: Implementar os stubs do pacote `pki` (`GenerateBankCSR`, `SubmitCSRToCB`, `StoreCertificate`) que já existem em `scenario-a/toolkit/engine/pki/csr.go` como `not implemented: ... — see TK-9 for implementation`. O contrato já está especificado; TK-9 é a implementação designada.

**Rationale**: O arquivo `pki/csr.go` documenta explicitamente que "implementation is provided by the TK-9 commercial-bank join flow PR". As assinaturas de função estão corretas e alinhadas com `onboarding_proxy.go`.

**Alternativas consideradas**:
- Criar novo pacote `scenario-a/toolkit/engine/bank/csr.go` — rejeitado; o contrato já está no lugar correto em `pki/`.
- Reutilizar `step_gen_tls.go` com auto-assinatura — rejeitado; viola a constituição (Princípio IV: compliance gate requer que o CB assine o cert).

**Detalhes de implementação**:
- `GenerateBankCSR(bankCode, institution, outputDir)` → gera ECDSA P-256 + PKCS#10 CSR, salva `{bankCode}.key` (0600) e `{bankCode}.csr` em `outputDir`
- `SubmitCSRToCB(csrPath, cbURL, timeout)` → POST JSON `{"csr_pem": "...", "bank_code": "...", "blockchain_pubkey": "..."}` ao endpoint; retorna cert PEM assinado
- `StoreCertificate(certPEM, bankCode, outputDir)` → salva `{bankCode}.crt` (0600)
- **NUNCA** cria arquivos `*-ca.key` ou `*-ca.crt` (constraint do package)

---

## D2 — Votação QBFT: API e sequência de espera

**Decisão**: Usar JSON-RPC `qbft_proposeValidatorVote(address, bool)` via HTTP POST em cada validador listado em `bundle.spec.validators`. Aguardar ativação via polling de `qbft_getValidatorsByBlockNumber` até o endereço do joiner aparecer no validator set.

**Rationale**: SP-02 validou esta sequência (ADR-002 D1, D4). O script `vote-in-validator.sh` do spike usou exatamente `qbft_proposeValidatorVote`. A ativação ocorre na próxima fronteira de epoch após o quórum ser atingido (⌊N/2⌋+1 votos).

**Alternativas consideradas**:
- Smart-contract-based validator selection — rejeitado; QBFT usa block-header voting (ADR-002).
- Aguardar número fixo de blocos — rejeitado; o epoch length varia por ambiente.

**Detalhes de implementação**:
```
Para cada validador em bundle.spec.validators:
  POST http://<rpcUrl>/
    body: { "jsonrpc":"2.0", "method":"qbft_proposeValidatorVote",
            "params":["<joiner_address>", true], "id":1 }
  Registrar resultado (sucesso ou falha) individualmente no log

Contar votos com sucesso.
Se sucessos < ⌊N/2⌋+1 → falhar com erro descritivo (N votos obtidos, Q necessários).

Polling de ativação (em qualquer validador acessível):
  GET qbft_getValidatorsByBlockNumber("latest")
  Repetir até joiner_address ∈ resultado OU timeout (configurable, default 5min)
```

**Epoch boundary**: O motor não precisa conhecer o epoch length — o polling detecta a ativação independentemente do tempo.

---

## D3 — Extensão do join bundle: campos `validators` e `cbEndpoint`

**Decisão**: Adicionar dois campos ao `BundleSpec` em `scenario-a/toolkit/engine/bundle/types.go`:
- `Validators []ValidatorSpec` — lista de validadores existentes do spoke
- `CBEndpoint string` — URL do endpoint credential-request do banco central

**Rationale**: O bundle é a única fonte de verdade que o banco comercial recebe do CB. O CB conhece seus próprios validadores e a URL do próprio gateway — é o produtor natural desses dados. O operador do banco comercial não deve precisar configurá-los manualmente.

**Alternativas consideradas**:
- Campos no manifesto do banco comercial — rejeitado; viola o princípio de que o banco só configura sua própria identidade, não a topologia do spoke.
- Campo separado fora do bundle (ex: env var) — rejeitado; quebraria o princípio de "declarativo sobre imperativo" (concat.md §4.1).

**ValidatorSpec**:
```go
type ValidatorSpec struct {
    Address string `yaml:"address"` // Ethereum address (0x...)
    RPCURL  string `yaml:"rpcUrl"`  // JSON-RPC HTTP endpoint
}
```

**Retrocompatibilidade**: Campos novos em `BundleSpec` são omitempty-compatíveis; bundles antigos sem `validators` resultam em slice vazio → step `vote-qbft` falha com erro claro "0 validators in bundle".

---

## D4 — Template Compose TK-8: diferenças em relação ao TK-4

**Decisão**: O template commercial-bank é estruturalmente idêntico ao TK-4 (central-bank) com três diferenças:

| Aspecto | TK-4 (central-bank) | TK-8 (commercial-bank) |
|---------|---------------------|------------------------|
| `genesis-init` | Presente (gera genesis se não existe) | **Ausente** — genesis vem do bundle, gravado pelo motor no volume nomeado `genesis` |
| `BOOTNODE_ENODE` | Opcional (vazio = este nó é o bootnode) | **Obrigatório** (falha explícita se ausente) |
| Node data path | volume nomeado `${SPOKE_ID}_cb_besu_data` | volume nomeado `${SPOKE_ID}_${BANK_ID}_besu_data` |
| Container name | `cbweb3-${SPOKE_ID}-besu.central-bank` | `cbweb3-${SPOKE_ID}-besu.${BANK_ID}` |

`entry.sh` é reutilizado sem alteração (mesma lógica DOCKER/NONE NAT profile, ADR-001).

**Rationale**: Preserva o invariante central do toolkit: genesis nunca é regenerado em um spoke existente. O banco comercial recebe o genesis pré-validado do bundle (com hash SHA-256 verificado pelo motor antes do `docker compose up`).

> **Addendum (2026-07-07) — desvio de "Node data path" para volume Docker nomeado**:
> Originalmente `${SPOKE_DATA_DIR}/nodes/commercial-bank/data` (bind mount, espelhando
> o TK-4). Passou a ser o volume nomeado `${SPOKE_ID}_${BANK_ID}_besu_data`, montado em
> `/opt/besu/data`. Mesmo racional do desvio equivalente em
> `specs/026-tk4-compose-central-bank/plan.md`: paridade com Paladin (`bank_data`,
> `paladin-compose.yaml`) e Postgres (`pg_data`), sem necessidade de inspeção do chain
> data pelo host. Diferente do TK-4, aqui **não** é necessário um `besu-data-init`:
> não há `genesis-init` (nem qualquer container rodando como usuário não-root) escrevendo
> nesse caminho — o serviço `besu` não define `user:`, então um volume novo (root-owned)
> funciona da mesma forma que o diretório bind-mount root-owned que o Docker criava
> antes. Na época deste addendum, `genesis/` ainda era bind mount — ver o addendum
> seguinte, que reverte também essa parte. Ver
> `scenario-a/provisioning/templates/commercial-bank/docker-compose.yaml`.
>
> **Addendum 2 (2026-07-07) — genesis/genesis.json também migrado para volume**:
> `${SPOKE_DATA_DIR}/genesis/genesis.json` (bind mount) passou a ser o volume
> nomeado `${SPOKE_ID}_${BANK_ID}_genesis` (chave `genesis` no compose). O
> `step_write_genesis.go` decodifica o conteúdo base64 do bundle e grava
> diretamente no volume via `engine/dockervolume.WriteFile` — o conteúdo nunca
> toca o filesystem do host. O invariante "genesis-once" (hash não pode divergir
> de uma gravação anterior) passou a ser verificado lendo o volume
> (`engine/dockervolume.ReadFile`) em vez do arquivo no host. Sem necessidade de
> init container: nem o `step_write_genesis.go` (roda `docker run` como root) nem
> o serviço `besu` (sem `user:` override) precisam de um volume mundialmente
> gravável — ambos operam como root. Resultado: o template
> `commercial-bank/docker-compose.yaml` não referencia mais `SPOKE_DATA_DIR` em
> nenhum mount. Ver `scenario-a/toolkit/engine/orchestrator/step_write_genesis.go`
> e `scenario-a/toolkit/engine/dockervolume/`.

---

## D5 — Paladin no template TK-8

**Decisão**: Incluir um `paladin-compose.yaml` no template TK-8, parametrizado por `BANK_ID`. O motor TK-9 sobe o Paladin como parte do passo `start-backend` (passo 9), após o registro no IdentityRegistry.

**Rationale**: SP-02 (`stack-join.yml`) validou que o Paladin do banco comercial pode ser iniciado sem restart dos Paladins existentes (ADR-002 D2/D3). A descoberta é reativa via eventos de bloco. A estrutura `paladin-bank-x` do spike é o modelo.

**Alternativas consideradas**:
- Paladin como compose separado (fora do TK-8) — rejeitado; o template deve ser auto-suficiente para o operador do banco.
- Paladin sem `paladin-bx-data-init` — rejeitado; o init container de permissões é necessário conforme validado no SP-02.

---

## D6 — Roteamento em `apply.go` para `mode: join`

**Decisão**: Estender `apply.go` (TK-7) com roteamento baseado em `spec.mode`:
```go
switch m.Spec.Mode {
case "found":
    return orchestrator.RunFound(ctx, m, deps)
case "join":
    return orchestrator.RunJoin(ctx, m, joinDeps)
default:
    return fmt.Errorf("apply: unknown mode %q", m.Spec.Mode)
}
```

`JoinDeps` é uma nova struct em `deps.go` que carrega o join bundle já parseado + timeout configs específicos do join.

**Rationale**: O TK-7 (`apply.go`) já suporta o dispatch; adicionar `case "join"` é mínimo e não altera o comportamento do `mode: found`.

---

## D7 — Passo `step_receive_cert.go` vs polling inline em `step_request_cert.go`

**Decisão**: Dois passos separados: `request-cert` (POST CSR) e `receive-cert` (polling para obter o cert assinado).

**Rationale**: O CB pode levar tempo variável para assinar (processo humano ou automatizado). Separar os passos permite persistir `request-cert = done` antes do polling. Se o motor é reiniciado durante o polling, não reenvia o CSR — vai direto para `receive-cert`. Isso é critical para idempotência quando o endpoint CB registra a requisição.

**Alternativas consideradas**:
- Polling inline em `request-cert` — rejeitado; se o motor cair durante o polling, não sabe se o POST foi recebido e reenviaria o CSR.
- Async webhook do CB → banco — rejeitado; requer infraestrutura de networking inbound no banco comercial, que pode não estar disponível.

---

## D8 — Step names para `.provisioning-state.yaml`

Os nomes dos passos são strings canônicas (constantes Go) para o estado persistido:

```go
const (
    StepWriteGenesis     = "write-genesis"
    StepStartBesuJoin    = "start-besu-join"
    StepWaitSync         = "wait-sync"
    StepVoteQBFT         = "vote-qbft"
    StepGenCSR           = "gen-csr"
    StepRequestCert      = "request-cert"
    StepReceiveCert      = "receive-cert"
    StepProofPossession  = "proof-of-possession"
    StepStartBackend     = "start-backend"
)
```

Estes nomes são separados do espaço de nomes do `mode: found` (ex: `StepGenTLS = "gen-tls"`), evitando colisão no mesmo `SPOKE_DATA_DIR`.
