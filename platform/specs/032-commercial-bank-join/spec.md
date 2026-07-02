# Feature Specification: TK-8 e TK-9 — Template Compose e Motor de Orquestração `mode: join`

**Feature Branch**: `032-commercial-bank-join`
**Created**: 2026-06-27
**Status**: Draft
**Input**: User description: "TK-8 e TK-9: template Compose commercial-bank e motor de orquestração mode:join — permite que um banco comercial entre em um spoke existente a partir de um join bundle, iniciando o nó Besu, votando QBFT, obtendo certificado do banco central e registrando no IdentityRegistry"

## User Scenarios & Testing *(mandatory)*

### User Story 1 — Operador incorpora um banco comercial a um spoke existente (Priority: P1)

Um operador recebe um join bundle emitido pelo banco central do spoke (TK-6) e um manifesto YAML descrevendo o banco comercial. Ele executa `cbweb3 apply -f commercial-bank-brl.yaml` uma única vez. Ao final, o nó Besu do banco está sincronizado com o spoke, foi promovido a validador QBFT, possui certificado TLS assinado pelo banco central, e sua identidade consta no IdentityRegistry on-chain.

**Why this priority**: Este é o fluxo principal de FASE 3 e pré-requisito para qualquer teste de interoperação com banco comercial. Sem ele, nenhum banco comercial pode participar do spoke.

**Independent Test**: Pode ser testado de forma isolada usando um spoke local provisionado por TK-4/TK-5 (`mode: found`) e um bundle emitido por TK-6. O cenário completo é: found → bundle → join. Entrega valor imediato ao permitir que um novo participante entre no spoke sem edição de código.

**Acceptance Scenarios**:

1. **Given** um spoke local rodando com 1 validador QBFT (banco central) e um join bundle válido emitido por TK-6, **When** o operador executa `cbweb3 apply -f commercial-bank.yaml` com `mode: join` e `joinBundleRef` apontando para o bundle, **Then** o comando completa com saída estruturada indicando sucesso em cada um dos 9 passos e o banco comercial aparece no validator set do QBFT.

2. **Given** o join completou com sucesso, **When** o operador consulta o IdentityRegistry on-chain, **Then** o endereço Ethereum do banco comercial está registrado com status ativo.

3. **Given** o join completou com sucesso, **When** o operador verifica os logs do Paladin do banco central, **Then** não há erros de TLS e o banco comercial aparece no registro via descoberta reativa (sem restart dos nós existentes).

4. **Given** um spoke local rodando com bloco height N, **When** o passo de votação QBFT ocorre, **Then** a produção de blocos não é interrompida por mais de 6 segundos (limiar do ADR-002 T2: period=2s + requestTimeout=4s).

---

### User Story 2 — Operador retoma um join interrompido (Priority: P2)

Um join falhou na metade (ex: timeout no passo de sincronização do Besu). O operador corrige a causa-raiz e re-executa `cbweb3 apply -f commercial-bank.yaml`. O motor detecta os passos já concluídos e retoma a partir do ponto de falha.

**Why this priority**: Idempotência é requisito não-funcional crítico da constituição do projeto. Sem ela, uma falha obriga o operador a desfazer manualmente o estado parcial.

**Independent Test**: Pode ser testado injetando uma falha controlada (ex: banco central indisponível no passo de CSR) e verificando que o re-run pula os passos anteriores (deploy-genesis, start-besu, wait-sync, vote-qbft) e retoma a partir de `gen-csr`.

**Acceptance Scenarios**:

1. **Given** um join que falhou no passo `gen-csr`, **When** o operador re-executa `cbweb3 apply`, **Then** os passos `write-genesis`, `start-besu`, `wait-sync` e `vote-qbft` são marcados como "skipped (already done)" na saída e `gen-csr` é o primeiro passo executado.

2. **Given** um join completado em sua totalidade, **When** o operador re-executa `cbweb3 apply`, **Then** todos os 9 passos são pulados e o comando retorna sucesso imediatamente.

---

### User Story 3 — Operador pré-visualiza o plano de join sem executar (Priority: P3)

Antes de iniciar o join em produção, o operador quer ver quais passos serão executados e quais variáveis foram resolvidas, sem alterar nenhum estado.

**Why this priority**: A flag `-dry-run` já existe em TK-7 para `mode: found`. Estendê-la para `mode: join` mantém paridade de UX entre os dois modos.

**Independent Test**: Pode ser testado passando `-dry-run` e verificando que nenhum container é iniciado, nenhum arquivo de estado é escrito e a saída lista todos os 9 passos com suas configurações resolvidas.

**Acceptance Scenarios**:

1. **Given** um manifesto `mode: join` e um join bundle válido, **When** o operador executa `cbweb3 apply -f commercial-bank.yaml -dry-run`, **Then** a saída lista os 9 passos com status "would run" e nenhum efeito colateral (nenhum container, nenhum arquivo de estado, nenhuma transação on-chain).

---

### Edge Cases

- O que acontece se o join bundle tem hash de genesis divergente do arquivo já existente em `SPOKE_DATA_DIR/genesis/genesis.json`? → O motor deve abortar com erro claro e não sobrescrever o genesis.
- O que acontece se a votação QBFT falha porque menos validadores que o quórum estão disponíveis? → O passo falha com erro explícito indicando quantos votos foram coletados versus o quórum necessário (⌊N/2⌋+1).
- O que acontece se o banco central rejeita o CSR (identidade não autorizada)? → O passo `request-cert` falha com o código de erro HTTP da resposta do banco central, sem tentar novamente.
- O que acontece se o join bundle não contém o campo `validators`? → A validação do bundle no passo `write-genesis` falha imediatamente com erro de schema, antes de qualquer efeito.
- O que acontece se a sincronização do Besu excede o timeout configurado? → O passo `wait-sync` falha com erro de timeout, deixando o container ativo para debugging pelo operador.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: O sistema DEVE aceitar um manifesto YAML com `spec.mode: join` e `spec.joinBundleRef` apontando para um arquivo de join bundle produzido pelo TK-6.
- **FR-002**: O sistema DEVE validar o join bundle contra seu schema (incluindo os campos `validators` e `cbEndpoint` adicionados por esta feature) e falhar imediatamente com erro descritivo caso qualquer campo obrigatório esteja ausente ou inválido.
- **FR-003**: O sistema DEVE escrever o conteúdo de `genesis.json` do bundle em `SPOKE_DATA_DIR/genesis/genesis.json` somente se o arquivo não existir; se existir, DEVE verificar que o SHA-256 corresponde ao hash do bundle e abortar caso diverge.
- **FR-004**: O sistema DEVE iniciar o template Compose TK-8 passando `BOOTNODE_ENODE` do bundle como variável de ambiente obrigatória.
- **FR-005**: O template Compose TK-8 NÃO DEVE possuir serviço `genesis-init`; o genesis é provido exclusivamente via bind mount a partir de `SPOKE_DATA_DIR/genesis/`.
- **FR-006**: O sistema DEVE aguardar a sincronização do nó Besu do banco comercial com o spoke, fazendo polling em `eth_blockNumber` até atingir a altura esperada ou um timeout configurável.
- **FR-007**: O sistema DEVE executar a votação QBFT chamando `qbft_proposeValidatorVote(endereço_joiner, true)` em cada validador listado em `bundle.spec.validators`, coletando respostas e verificando que o quórum (⌊N/2⌋+1) foi atingido antes de prosseguir.
- **FR-008**: O sistema DEVE aguardar a ativação do joiner no validator set QBFT (verificando via `qbft_getValidatorsByBlockNumber` após a fronteira do próximo epoch) antes de prosseguir para os passos de PKI.
- **FR-009**: O sistema DEVE gerar um par de chaves secp256k1 e um CSR X.509 (`OU=ROLE_COMMERCIAL_BANK`) via a interface `keyProvider` configurada no manifesto.
- **FR-010**: O sistema DEVE enviar o CSR e a chave pública blockchain ao endpoint `bundle.spec.cbEndpoint` (credential-request do banco central) via HTTP POST, sem armazenar a chave privada em nenhum arquivo ou variável de ambiente.
- **FR-011**: O sistema DEVE receber o certificado TLS assinado pelo banco central e persisti-lo em `SPOKE_DATA_DIR/tls/commercial-bank.crt`.
- **FR-012**: O sistema DEVE executar a prova de posse (nonce assinado pelo par de chaves blockchain do banco comercial) e registrar o banco no IdentityRegistry usando o endereço do contrato fornecido em `bundle.spec.contracts.registryAddress`.
- **FR-013**: O sistema DEVE iniciar o backend do banco comercial (compose) após o registro bem-sucedido no IdentityRegistry.
- **FR-014**: O sistema DEVE persistir o estado de conclusão de cada passo em `.provisioning-state.yaml` de forma atômica; cada passo verifica o estado antes de executar (idempotência).
- **FR-015**: O join bundle (TK-6) DEVE ser estendido com dois novos campos: `spec.validators` (lista de validadores existentes com endereço Ethereum e URL RPC) e `spec.cbEndpoint` (URL do endpoint de credential-request do banco central).
- **FR-016**: O template Compose TK-8 DEVE ser parametrizado pelas mesmas variáveis de ambiente que TK-4 (SPOKE_ID, BESU_RPC_PORT, BESU_WS_PORT, BESU_P2P_PORT, BESU_IMAGE, SPOKE_DATA_DIR, BESU_ADVERTISED_HOST, BESU_NAT_PROFILE) mais `BANK_ID` para identificar o banco comercial dentro do spoke.
- **FR-017**: O motor `mode: join` DEVE ser roteado pelo comando `apply` (TK-7) com base em `spec.mode: join`; nenhuma flag CLI adicional deve ser necessária.

### Key Entities

- **JoinBundle (estendido)**: Documento YAML emitido por TK-6, agora com `spec.validators[]` (endereço + rpcUrl por validador) e `spec.cbEndpoint` (URL credential-request do CB). Nunca contém chaves privadas.
- **CommercialBankManifest**: Manifesto `mode: join` com `spec.joinBundleRef`, `spec.bankId`, `spec.keyProvider`, `spec.certSource`. Estende a estrutura de `Manifest` do TK-1: além de `joinBundleRef` (já existente em `manifest.Spec`), o fluxo de join adicionou o campo `BankID` a `manifest.Spec` (T047) e passou a exigir `spec.joinBundleRef` (T049) e `spec.node.dataDir` (T050) na validação em `mode: join`.
- **ProvisioningState (modo join)**: Mesmo formato `.provisioning-state.yaml` do TK-5, com novos nomes de passos: `write-genesis`, `start-besu`, `wait-sync`, `vote-qbft`, `gen-csr`, `request-cert`, `receive-cert`, `proof-of-possession`, `start-backend`.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Um banco comercial pode se incorporar a um spoke existente a partir de um manifesto YAML e um join bundle, sem edição de código e sem acesso direto ao ambiente do banco central.
- **SC-002**: O fluxo de join completo (9 passos) termina em menos de 5 minutos em hardware local com o spoke já sincronizado.
- **SC-003**: Re-executar `apply` após uma interrupção em qualquer passo retoma a partir do ponto de falha, sem repetir passos já concluídos, em menos de 10 segundos de overhead.
- **SC-004**: O endereço Ethereum do banco comercial aparece no validator set QBFT após o passo de votação, verificável via `qbft_getValidatorsByBlockNumber`.
- **SC-005**: O endereço Ethereum do banco comercial aparece no IdentityRegistry on-chain após o passo de prova-de-posse, verificável via chamada de leitura ao contrato.
- **SC-006**: A produção de blocos do spoke não é interrompida por mais de 6 segundos durante o passo de votação QBFT (limiar documentado no ADR-002 T2).
- **SC-007**: Nenhuma chave privada do banco comercial aparece em arquivos, variáveis de ambiente, logs ou no manifesto em nenhum ponto do fluxo.
- **SC-008**: O template Compose TK-8 e o motor TK-9 passam em todos os testes unitários (`go test ./...`) sem dependências de containers ou redes externas.

## Assumptions

- SP-02 (ADR-002) é autoritativo: nenhum restart dos nós Paladin existentes é necessário; a descoberta de novos pares é reativa via eventos de bloco.
- O epoch length do QBFT é configurado no genesis; em local pode ser curto (30 blocos); em prod é 30.000. O motor aguarda a próxima fronteira de epoch sem hardcodar o valor — lê do genesis.
- A votação requer que ⌊N/2⌋+1 validadores existentes votem; com 1 validador (CB), 1 voto é suficiente. O motor tenta todos os validadores listados no bundle e conta os sucessos.
- O endpoint `cbEndpoint` no bundle aponta para o API gateway do banco central, que expõe `/credential-request` conforme `onboarding_proxy.go`. Esta rota já existe no backend; TK-9 apenas a consome.
- O backend do banco comercial (passo 9) é iniciado via `docker compose up -d` sobre um template existente (fora do escopo desta feature); TK-9 apenas dispara o comando.
- A Paladin do banco comercial segue o padrão `paladin-bank-x` validado no SP-02; o template Compose TK-8 inclui o serviço Paladin parametrizado (analogamente ao `paladin-compose.yaml` do TK-4).
- O fluxo de join exigiu adições ao schema Go do manifesto: além de `spec.joinBundleRef` (já existente em `manifest.Spec`), foi adicionado o campo `BankID` a `manifest.Spec` (T047) para identificar o banco comercial dentro do spoke. A validação em `mode: join` passou a exigir `spec.joinBundleRef` (T049) e `spec.node.dataDir` (T050) como campos obrigatórios.
- Suporte mobile (frontend) está fora do escopo desta feature.
- Os testes do motor `mode: join` usam stubs/mocks para BesuRPC, CB endpoint e IdentityRegistry, seguindo o padrão já estabelecido em TK-5.
