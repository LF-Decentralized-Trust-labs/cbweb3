# Feature Specification: TK-6 — Emissor do Join Bundle

**Feature Branch**: `028-tk6-join-bundle-emitter`  
**Created**: 2026-06-27  
**Status**: Draft  
**Input**: User description: "TK-6 — CRIAR: emissor do join bundle para o toolkit de provisionamento do Scenario A (Fase 1B)"

## User Scenarios & Testing *(mandatory)*

### User Story 1 — Emitir join bundle completo após `mode: found` completar (Priority: P1)

Um operador de banco central que acabou de provisionar um spoke via TK-5 invoca o emissor. O emissor lê todos os artefatos produzidos pelo engine no `SPOKE_DATA_DIR` — genesis, endereços de contratos, cert CA, e enode obtido do Besu via RPC — e emite `bundles/<spoke-id>.bundle.yaml` completo e público. O bundle contém os seis campos obrigatórios: enode do bootnode (com host anunciado do manifesto), genesis (hash SHA-256 + conteúdo base64), endereços de contratos deployados, endpoint do relay, certificado CA do spoke como âncora de confiança, e porta P2P anunciada. O bundle não contém nenhuma chave privada.

**Why this priority**: O join bundle é a saída pública do CB após o provisionamento e o elo que une os dois fluxos: sem ele, nenhum banco comercial consegue se conectar ao spoke (`mode: join` — TK-9). É o gating item da Fase 1B que transforma um spoke fundado em um spoke conectável.

**Independent Test**: Pode ser testado com `SPOKE_DATA_DIR` populado (genesis.json, .deployed-addrs.env, tls/central-bank.crt) e Besu mockado para `admin_nodeInfo`. Invocar `EmitBundle(ctx, BundleInput{...})` e verificar: (a) `bundles/spoke-brl.bundle.yaml` existe no `outputDir`; (b) todos os campos obrigatórios presentes e não vazios; (c) `spec.trust.caCertPEM` começa com `-----BEGIN CERTIFICATE-----`; (d) nenhum campo contém `PRIVATE KEY`.

**Acceptance Scenarios**:

1. **Given** `SPOKE_DATA_DIR` com genesis.json válido, .deployed-addrs.env com todos os 7 endereços preenchidos, tls/central-bank.crt PEM válido, e Besu respondendo `admin_nodeInfo` com enode `enode://abc@172.17.0.2:30303`, manifesto com `advertisedHost: cbweb3-spoke-brl-besu.central-bank-brazil` e `p2p.port: 31303`, **When** `EmitBundle` é chamado, **Then** `bundles/spoke-brl.bundle.yaml` é criado com `spec.bootnode.enode: enode://abc@cbweb3-spoke-brl-besu.central-bank-brazil:31303`, `spec.genesis.hash` igual ao SHA-256 hex do genesis.json, `spec.genesis.content` igual ao genesis.json base64-encoded, `spec.contracts` com os 7 endereços, `spec.relay.endpoint` do manifesto, `spec.trust.caCertPEM` com o PEM completo do cert CA.
2. **Given** bundle já emitido em `bundles/spoke-brl.bundle.yaml`, **When** `EmitBundle` é chamado novamente com os mesmos artefatos, **Then** o bundle é regravado deterministicamente (mesmo conteúdo, campo `metadata.generatedAt` fixo na primeira emissão), sem erro — idempotente.
3. **Given** bundle emitido, **When** inspeção de todas as strings do YAML, **Then** nenhuma string contém `PRIVATE KEY`, `-----BEGIN EC PRIVATE KEY`, `-----BEGIN RSA PRIVATE KEY`, `-----BEGIN OPENSSH PRIVATE KEY`, ou qualquer variante de bloco PEM de chave privada.

---

### User Story 2 — Falha rápida quando artefatos obrigatórios estão ausentes ou incompletos (Priority: P1)

Se qualquer artefato obrigatório estiver ausente ou incompleto no `SPOKE_DATA_DIR`, o emissor falha imediatamente com um erro tipado que identifica o artefato faltante. Nenhum bundle parcial é criado.

**Why this priority**: Um bundle parcial consumido por TK-9 causa falha silenciosa ou configuração incorreta do nó que entra no spoke. Fail-fast com erro claro é a única resposta segura; obriga o operador a completar o provisionamento TK-5 antes de emitir.

**Independent Test**: Invocar `EmitBundle` com cada artefato ausente individualmente e verificar que retorna o erro tipado correto e que nenhum arquivo de bundle foi criado.

**Acceptance Scenarios**:

1. **Given** `<dataDir>/genesis/genesis.json` ausente, **When** `EmitBundle` é chamado, **Then** retorna `ErrGenesisNotFound` e nenhum arquivo de bundle é criado.
2. **Given** `<dataDir>/tls/central-bank.crt` ausente, **When** `EmitBundle` é chamado, **Then** retorna `ErrCACertNotFound`.
3. **Given** `.deployed-addrs.env` com `REGISTRY_CONTRACT_ADDRESS` vazio, **When** `EmitBundle` é chamado, **Then** retorna `ErrDeployedAddrsIncomplete` com mensagem identificando a chave `REGISTRY_CONTRACT_ADDRESS`.
4. **Given** Besu não respondendo em `BesuRPCURL` (connection refused), **When** `EmitBundle` é chamado, **Then** retorna `ErrEnodeUnavailable` sem criar arquivo de bundle.
5. **Given** `tls/central-bank.crt` contendo bloco PEM de chave privada (misconfiguration), **When** `EmitBundle` é chamado, **Then** retorna `ErrCACertNotFound` com mensagem "cert file contains private key material" — nunca escreve chave no bundle.

---

### User Story 3 — Enode com host anunciado correto (nunca IP interno) (Priority: P1)

O enode no bundle usa sempre `spec.node.advertisedHost` e `spec.node.p2p.port` do manifesto como host e porta — nunca o IP interno do container ou a porta interna do Besu. O enode-id (pubkey secp256k1 hexadecimal) é extraído do Besu via `admin_nodeInfo`; host e porta são substituídos pelos valores do manifesto.

**Why this priority**: É a regra central do design doc (§4, Regra 2): endereçamento é um parâmetro explícito, nunca inferido. Um enode com IP de container é inválido fora da rede Docker, invalidando o bundle para qualquer join externo.

**Independent Test**: Mockar `admin_nodeInfo` retornando enode com host `0.0.0.0:30303`. Manifesto com `advertisedHost: cbweb3-brl.bank.local` e `p2p.port: 31303`. Verificar que o bundle contém `enode://<id>@cbweb3-brl.bank.local:31303`.

**Acceptance Scenarios**:

1. **Given** Besu retornando `enode://deadbeef@0.0.0.0:30303` via `admin_nodeInfo`, manifesto com `advertisedHost: cbweb3-brl.bank.local` e `p2p.port: 31303`, **When** bundle emitido, **Then** `spec.bootnode.enode = enode://deadbeef@cbweb3-brl.bank.local:31303` e `spec.bootnode.advertisedHost = cbweb3-brl.bank.local` e `spec.bootnode.p2pPort = 31303`.
2. **Given** `admin_nodeInfo` retornando string sem `@` (enode mal formado), **When** `EmitBundle` é chamado, **Then** retorna `ErrEnodeUnavailable` com detalhe "invalid enode format".
3. **Given** `spec.node.p2p` nil no manifesto (porta não configurada), **When** `EmitBundle` é chamado, **Then** retorna `ErrInvalidInput` com mensagem "spec.node.p2p.port is required for mode: found".

---

### User Story 4 — Modo exclusivo: apenas `mode: found` emite bundle (Priority: P2)

O emissor rejeita manifestos com `mode != "found"` imediatamente, antes de qualquer leitura de artefato. Apenas o banco central que fundou o spoke tem autoridade para emitir seu bundle.

**Why this priority**: Garante o modelo de confiança do design doc (§2, PKI): a âncora de confiança do spoke é o CB fundador. Um banco comercial não tem nem o genesis nem o cert CA para emitir um bundle válido — e não deveria tentar.

**Acceptance Scenarios**:

1. **Given** manifesto com `mode: join`, **When** `EmitBundle` é chamado, **Then** retorna `ErrInvalidMode` com mensagem `"EmitBundle requires mode: found, got: join"` sem ler nenhum arquivo.
2. **Given** manifesto com `mode: found`, **When** `EmitBundle` é chamado, **Then** validação de modo passa e o emissor prossegue normalmente.

---

### Edge Cases

- O que acontece quando `admin_nodeInfo` retorna enode com host `[::]` (bind-all IPv6)? → substituir pelo `advertisedHost` do manifesto.
- O que acontece quando genesis.json tem mais de 1 MB? → embutir como base64 ainda é aceito (genesis Besu é sempre < 50 KB em prática); documentar o limite de 1 MB como aviso, não bloqueio.
- O que acontece quando `tls/central-bank.crt` existe mas está truncado (PEM incompleto)? → `pem.Decode` retorna nil → `ErrCACertNotFound` com detalhe de parsing.
- O que acontece quando `outputDir` não existe? → `os.MkdirAll(outputDir+"/bundles", 0o755)` antes de escrever; se falhar, propagar o erro do sistema operacional.
- O que acontece quando dois processos chamam `EmitBundle` concorrentemente para o mesmo spoke? → escrita atômica via temp+rename resolve: o último a renomear vence, conteúdo sempre completo.
- O que acontece quando o contexto é cancelado durante a chamada RPC ao Besu? → `net/http` propaga o cancelamento; `EmitBundle` retorna `ctx.Err()`.
- O que acontece quando `spec.relay` é nil no manifesto? → campo `spec.relay` é omitido do bundle (não é erro — relay pode ser adicionado posteriormente por TK-7 ou manualmente).

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: O emissor DEVE expor a função pública `EmitBundle(ctx context.Context, in BundleInput) (*JoinBundle, error)` como único entry point; `BundleInput` encapsula `Manifest *manifest.Manifest`, `DataDir string`, `OutputDir string`, e `EnodeProvider EnodeProvider`.
- **FR-002**: O emissor DEVE verificar `manifest.Spec.Mode == "found"` antes de qualquer operação; retornar `ErrInvalidMode` se diferente.
- **FR-003**: O emissor DEVE verificar que `manifest.Spec.Node.P2P != nil` e `manifest.Spec.Node.P2P.Port > 0`; retornar `ErrInvalidInput` se ausente.
- **FR-004**: O emissor DEVE obter o enode-id via `EnodeProvider.NodeInfo(ctx)`, extrair o enode-id (parte antes do `@`), e compor o enode final como `enode://<id>@<advertisedHost>:<p2pPort>`; retornar `ErrEnodeUnavailable` se a chamada falhar ou o formato for inválido.
- **FR-005**: O emissor DEVE ler `<dataDir>/genesis/genesis.json`, computar SHA-256 (hex lowercase, prefixo `sha256:`), e embutir o conteúdo base64-encoded (RFC 4648, sem quebra de linha) em `spec.genesis`; retornar `ErrGenesisNotFound` se o arquivo não existir.
- **FR-006**: O emissor DEVE ler `<dataDir>/.deployed-addrs.env` via `parseDeployedAddrs` (já implementado em `engine/orchestrator/addrs.go`); os 7 campos obrigatórios são: `RegistryContractAddress`, `ZetoFactoryAddress`, `PenteFactoryAddress`, `ZetoTokenAddress`, `PenteContextGroupID`, `PenteContextAddress`, `FXAgreementDeployedAt`; retornar `ErrDeployedAddrsIncomplete` com o nome da chave ausente se qualquer um estiver vazio.
- **FR-007**: O emissor DEVE ler `<dataDir>/tls/central-bank.crt`, validar que contém pelo menos um bloco PEM com `Type: "CERTIFICATE"` via `encoding/pem`, e embutir o conteúdo completo do arquivo em `spec.trust.caCertPEM`; retornar `ErrCACertNotFound` se ausente, ilegível, ou sem bloco CERTIFICATE válido.
- **FR-008**: O emissor DEVE verificar que o conteúdo de `caCertPEM` não contém a string `PRIVATE KEY`; se contiver, retornar `ErrCACertNotFound` com mensagem `"cert file contains private key material"` — esta é a guarda de segurança final contra leakage de chaves.
- **FR-009**: O emissor DEVE tomar `manifest.Spec.Relay.Endpoint` para `spec.relay.endpoint` do bundle; se `manifest.Spec.Relay` for nil, omitir o campo `spec.relay` no bundle (zero value do YAML).
- **FR-010**: O emissor DEVE escrever o bundle em `<outputDir>/bundles/<spoke-id>.bundle.yaml` via escrita atômica: escrever em temp file no mesmo diretório e `os.Rename`; criar `<outputDir>/bundles/` com `os.MkdirAll` se não existir.
- **FR-011**: O bundle serializado DEVE ter `apiVersion: cbweb3/v1` e `kind: JoinBundle` como primeiros campos do YAML (ordem garantida via struct com `yaml` tags explícitas).
- **FR-012**: O campo `metadata.generatedAt` DEVE ser preenchido com o timestamp ISO-8601 UTC da primeira emissão; em re-emissões (arquivo já existe), o campo é atualizado com o novo timestamp (o bundle é sempre regravado com os artefatos atuais).
- **FR-013**: O emissor DEVE respeitar cancelamento do `context.Context`; `ctx.Err()` deve ser retornado quando o contexto for cancelado durante a chamada `EnodeProvider.NodeInfo`.
- **FR-014**: A interface `EnodeProvider` DEVE ter um único método `NodeInfo(ctx context.Context) (string, error)` que retorna a string bruta do enode (ex: `enode://abc@0.0.0.0:30303`); a implementação padrão `BesuEnodeProvider` faz JSON-RPC `admin_nodeInfo` para `BesuRPCURL`.

### Non-Functional Requirements

- Pacote Go em `scenario-a/toolkit/engine/bundle/` — sem dependências externas além das já em `toolkit/go.mod` (`gopkg.in/yaml.v3`, stdlib).
- Reutilizar `parseDeployedAddrs` de `engine/orchestrator/addrs.go` movendo-o para um pacote compartilhado `engine/addrs/`, ou duplicar minimamente — a decisão é tomada na fase de implementação (ver research.md R-06).
- Nenhum arquivo em `deploy/local/` ou `make/` é modificado.
- Nenhum dado de chave privada (key material) pode aparecer no bundle sob qualquer circunstância.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: `go test -race ./scenario-a/toolkit/engine/bundle/...` passa com 100% dos testes (test-first, per Constitution V) antes de qualquer implementação ser considerada completa.
- **SC-002**: `EmitBundle` invocado sobre um `SPOKE_DATA_DIR` totalmente populado (artefatos de TK-5 completos) produz `bundles/spoke-brl.bundle.yaml` com todos os 6 campos obrigatórios preenchidos e não vazios.
- **SC-003**: O bundle emitido é YAML válido parseable por `gopkg.in/yaml.v3` com `apiVersion: cbweb3/v1` e `kind: JoinBundle`.
- **SC-004**: O campo `spec.bootnode.enode` contém o `advertisedHost` e `p2pPort` do manifesto, não o host retornado pelo Besu.
- **SC-005**: O campo `spec.genesis.hash` é o SHA-256 hex do genesis.json lido do `dataDir` (verificável com `sha256sum`).
- **SC-006**: Nenhuma substring `PRIVATE KEY` aparece no conteúdo do arquivo bundle (verificável com `grep "PRIVATE KEY" bundles/*.bundle.yaml`).
- **SC-007**: Invocar `EmitBundle` com artefato ausente retorna o erro tipado correto (tabela: `ErrGenesisNotFound`, `ErrCACertNotFound`, `ErrDeployedAddrsIncomplete`, `ErrEnodeUnavailable`, `ErrInvalidMode`).
- **SC-008**: Invocar `EmitBundle` duas vezes consecutivas com os mesmos artefatos produz YAML com conteúdo idêntico nos campos estruturais (exceto `metadata.generatedAt`).
- **SC-009**: `go vet ./scenario-a/toolkit/engine/bundle/...` retorna zero warnings.

## Assumptions

- Go 1.26+ com `crypto/sha256`, `encoding/base64`, `encoding/json`, `encoding/pem`, `net/http`, `os`, `path/filepath` da stdlib; `gopkg.in/yaml.v3` já em `toolkit/go.mod`.
- O `SPOKE_DATA_DIR` é populado pelo TK-5 engine antes de `EmitBundle` ser chamado; o emissor não verifica o estado do arquivo `.provisioning-state.yaml` — isso é responsabilidade de TK-7.
- A interface `KeyProvider` (TK-2) **não** é usada pelo emissor — o emissor não assina nada, apenas lê artefatos públicos.
- A interface `CertSource` (TK-3) **não** é usada pelo emissor — o cert CA já está em `<dataDir>/tls/central-bank.crt` (produzido pelo passo 2 de TK-5).
- O Besu do spoke está rodando e acessível em `BesuRPCURL` quando `EmitBundle` for chamado; o emissor não tenta subir o Besu.
- `parseDeployedAddrs` de `engine/orchestrator/addrs.go` será movida para um pacote compartilhado ou re-exportada — decisão de refactoring na implementação (research.md R-06).
- `mode: join` (TK-9) consome o bundle emitido por TK-6; a estrutura do bundle é o contrato entre os dois — qualquer mudança no schema do bundle é uma breaking change para TK-9.
- O emissor reside em `scenario-a/toolkit/engine/bundle/`. É chamado por TK-7 (`cbweb3 apply`) após `RunFound` retornar nil — não é invocado diretamente pelo engine TK-5 (separation of concerns).
