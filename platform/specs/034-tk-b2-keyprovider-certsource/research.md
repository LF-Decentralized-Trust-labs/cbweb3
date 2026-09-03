# Research — TK-B2/B3 (KeyProvider + CertSource)

Fase 0. Decisões que resolvem o Technical Context. Formato: Decisão / Rationale / Alternativas.

## R1 — Curva e lib das chaves blockchain (KeyProvider)

- **Decisão**: **secp256k1** via `github.com/ethereum/go-ethereum` (`crypto`), com endereço EVM
  derivado da pubkey (keccak256 dos últimos 20 bytes).
- **Rationale**: as chaves assinam transações Besu/QBFT (EVM), que usam secp256k1 — fora do
  `crypto/ecdsa` da stdlib. É a via idiomática e já é a dep do toolkit de referência (roadmap §3).
- **Alternativas**: implementar secp256k1 na mão (risco/custo) — rejeitada.

## R2 — Origem das chaves locais: derivação determinística vs. aleatória

- **Decisão**: derivação **determinística por id de entidade** a partir de uma seed base (ex.:
  `keccak256(seed || id)` → chave), com uma **chave dev conhecida semeada** sob um id conhecido
  (emulador). Endereços estáveis entre execuções.
- **Rationale**: estabilidade é útil para genesis alloc, grants e reprodutibilidade local; casa
  com a nota de "chave derivada por banco" do roadmap (§10) e mantém isolamento (ids distintos →
  chaves distintas).
- **Alternativas**: geração aleatória → endereços mudam a cada execução (quebra genesis/grants
  locais) — rejeitada para o local.

## R3 — Export de chave privada (local-only)

- **Decisão**: a impl. **local** expõe um `ExportPrivateKeyHex(id)` **apenas-local**; a impl. de
  **produção NÃO** o expõe.
- **Rationale**: o backend precisa assinar transações na camada Besu em `local`; a fronteira de
  custódia de prod (KMS) nunca exporta chave. Espelha o toolkit de referência.
- **Alternativas**: nunca exportar → o caminho de assinatura Besu local não funcionaria sem KMS —
  rejeitada para o local.

## R4 — CA do CertSource: criação preguiçosa, só em memória

- **Decisão**: a CA self-signed P-256 por spoke é criada **preguiçosamente no primeiro
  `IssueLeafCert`**; `GetTrustAnchor` **não** cria (retorna `ErrTrustAnchorNotFound` se ausente);
  a chave privada da CA **nunca** é serializada.
- **Rationale**: mantém a invariante "sem segredos" e espelha o comportamento do toolkit de
  referência (double-check locking, chave em memória).
- **Alternativas**: CA persistida em disco → violaria "sem segredos"; criar CA no `GetTrustAnchor`
  → efeito colateral inesperado — rejeitadas.

## R5 — Taxonomia de erros

- **Decisão**: erros tipados e distinguíveis: `ErrNotImplemented` (prod), `ErrForbiddenRole`
  (OU ≠ ROLE_COMMERCIAL_BANK), `ErrUnsupportedKeyAlgorithm` (curva ≠ P-256 no CSR),
  `ErrInvalidCSR`, `ErrTrustAnchorNotFound`.
- **Rationale**: consumidores (motor/testes) discriminam por tipo; sem falhas silenciosas
  (Princípio VI). Espelha o toolkit de referência.

**Saída**: nenhuma `NEEDS CLARIFICATION` remanescente.
