# Feature Specification: Restaurar Auth e Onboarding (Scenario A)

**Feature Branch**: `003-restore-auth-onboarding`
**Created**: 2026-05-06
**Status**: Draft
**Input**: User description: "precisamos adicionar o processo de auth e onboard semelhante ao que temos na branch 'develop-scenario-a'. É possível consultar essa branch 'develop-scenario-a' e ver como está sendo implementado e criar a spec para essa feature nova?"

## Contexto

A branch `develop-scenario-a` implementa um fluxo completo de autenticação de 2 fatores (PKI + Keycloak) e onboarding de bancos comerciais em 3 fases (Credencial → KYC → Registro Blockchain). Quando o Scenario B foi desenvolvido (commit `9831d50`), **todas essas rotas foram removidas** do API Gateway, deixando os 4 bancos comerciais (A, B, C, D) incapazes de se registrar na rede.

Esta feature restaura as capacidades de auth e onboarding — coexistindo com as rotas do Scenario B v2 já existentes — e corrige dois defeitos adicionais identificados no diagnóstico.

---

## Clarifications

### Session 2026-05-06

- Q: Qual o TTL do nonce PKI gerado no login de bancos comerciais? → A: 30 minutos
- Q: O que acontece quando o Central Bank rejeita um pedido de KYC? → A: Rejeição define status `KYC_REJECTED` com campo `rejection_reason`; banco pode corrigir e resubmeter a credencial
- Q: Como tratar cold-start sem a tabela `onboarding_requests` em ambientes Scenario B? → A: Scenario B é 100% novo — quem precisa entrar faz tudo do zero. A tabela DEVE ser criada via GORM `AutoMigrate` no startup; a função `DropScenarioATables` deve ser completamente removida (não há resíduos do Scenario A a limpar)
- Q: As transições de estado do onboarding devem ser auditadas? → A: Cada transição de estado é registrada em tabela interna `onboarding_events`; sem endpoint de consulta nesta fase — acessível apenas via DB diretamente *(implementado via tabela `audit_logs` existente no compliance service — ver D-003 em research.md)*
- Q: O que acontece com o `request_id` quando um banco com KYC_REJECTED resubmete a credencial? → A: A resubmissão cria um novo `request_id`; o registro anterior permanece com status KYC_REJECTED no histórico para rastreabilidade completa
- Q: Qual o comportamento exato de `POST /api/v1/onboarding/initiate` quando o banco já tem um registro existente? → A: `CREDENTIAL_REQUESTED`, `KYC_APPROVED` ou `COMPLETED` → HTTP 409 (sem novo registro); `KYC_REJECTED` → HTTP 201 com novo `request_id` (resubmissão permitida)
- Q: Qual o comportamento do `OnboardingProxyHandler` em caso de timeout na chamada ao Central Bank? → A: Timeout configurável via env var `CENTRAL_BANK_TIMEOUT` (default 10s); após expirar retorna HTTP 504 (Gateway Timeout) com mensagem clara
- Q: O que acontece quando o banco envia assinatura inválida em `POST /api/v1/auth/wallet/bind`? → A: O nonce permanece válido após falha de assinatura; banco pode tentar novamente com assinatura corrigida até o TTL de 30 minutos expirar (one-time use aplica-se apenas ao sucesso)
- Q: Qual o modelo de autenticação das rotas do API Gateway do Central Bank? → A: `POST /credential-request` = pública (sem auth); `GET /status/:requestId` e `POST /complete` = exigem `ROLE_GOVERNANCE`
- Q: Como tratar a entidade de eventos no spec para manter alinhamento com o cenário A? → A: Manter `OnboardingEvent` como entidade conceitual no spec (compatibilidade de domínio), com implementação física via `audit_logs` no compliance service

---

## User Scenarios & Testing *(mandatory)*

### User Story 1 — Banco Comercial completa onboarding no Central Bank (Priority: P1)

Um operador de banco comercial (Bank-A, B, C ou D) precisa registrar sua instituição no Central Bank de seu spoke antes de poder participar de qualquer operação de pagamento ou liquidez. O fluxo cobre 3 fases: submissão de credencial, aprovação KYC pelo Central Bank, e conclusão com prova de posse de carteira blockchain.

**Why this priority**: Sem onboarding, nenhuma operação de negócio é possível. É o pré-requisito para tudo que o sistema faz.

**Independent Test**: Pode ser testado de forma independente executando o script `tryout-spoke-a-bank-a.sh` e `tryout-spoke-b-bank-b.sh` com os containers do spoke correspondente no ar, verificando que o fluxo de 7 passos completa sem erros HTTP 404.

**Acceptance Scenarios**:

1. **Given** o banco comercial está autenticado via Keycloak, **When** submete uma requisição de credencial (`POST /api/v1/onboarding/initiate`), **Then** o sistema cria um registro de onboarding com status `CREDENTIAL_REQUESTED` e retorna `request_id` + `user_id` com HTTP 201.
2. **Given** o banco já tem um registro com status `CREDENTIAL_REQUESTED`, `KYC_APPROVED` ou `COMPLETED`, **When** o operador do banco tenta submeter novamente via `POST /api/v1/onboarding/initiate`, **Then** o sistema retorna HTTP 409 (conflito) sem criar registro duplicado. **Given** o status é `KYC_REJECTED`, **When** resubmete, **Then** retorna HTTP 201 com novo `request_id` (o registro anterior permanece imutável).
3. **Given** o Central Bank aprovou o KYC do banco, **When** o banco consulta o status (`GET /api/v1/onboarding/status/:requestId`), **Then** o status é `KYC_APPROVED` e o campo `pop_nonce` está presente.
4. **Given** o banco tem o `pop_nonce` disponível, **When** envia a prova de posse (`POST /api/v1/onboarding/complete`) com a assinatura da carteira blockchain, **Then** o status muda para `COMPLETED`, o banco recebe um certificado X.509 e pode fazer login PKI.
5. **Given** o banco completou o onboarding com sucesso, **When** tenta fazer login PKI via `POST /api/v1/auth/pki-login`, **Then** recebe um token de sessão válido.

---

### User Story 2 — Banco recupera status de onboarding sem o request_id original (Priority: P2)

Após recarregar a página ou reiniciar o cliente, o operador do banco não tem o `request_id` original mas precisa descobrir em qual fase do onboarding se encontra.

**Why this priority**: Funcionalidade de resiliência importante para UX — evita que o operador precise reiniciar o processo do zero.

**Independent Test**: Autenticar como um banco que já iniciou o onboarding e chamar `GET /api/v1/onboarding/my-status` sem parâmetros — o sistema deve resolver o `bank_code` a partir do JWT e retornar o status atual.

**Acceptance Scenarios**:

1. **Given** o banco está autenticado, **When** chama `GET /api/v1/onboarding/my-status`, **Then** recebe o status atual do onboarding sem precisar informar `request_id`, com HTTP 200.
2. **Given** o banco nunca iniciou onboarding, **When** chama `GET /api/v1/onboarding/my-status`, **Then** o sistema retorna `{"status": "NONE"}` com HTTP 200 (não HTTP 404).

---

### User Story 3 — Central Bank aprova KYC de banco comercial (Priority: P2)

O operador de governança do Central Bank revisa os pedidos de credencial pendentes e aprova ou rejeita o KYC de cada banco comercial.

**Why this priority**: Bloqueia a progressão da Phase 1 para Phase 3 do onboarding. Sem isso, nenhum banco conclui o registro.

**Independent Test**: Autenticar como operador de governança do Central Bank e chamar `POST /api/v1/compliance/approve-kyc` com o `user_id` do banco, depois verificar que `GET /api/v1/onboarding/status/:requestId` retorna `pop_nonce`.

**Acceptance Scenarios**:

1. **Given** o banco submeteu credencial (status `CREDENTIAL_REQUESTED`), **When** o operador do Central Bank aprova o KYC via `POST /api/v1/compliance/approve-kyc`, **Then** o status do onboarding muda para `KYC_APPROVED` e um `pop_nonce` é gerado.
2. **Given** o banco submeteu credencial, **When** o operador do Central Bank rejeita o KYC com um motivo, **Then** o status muda para `KYC_REJECTED` e o campo `rejection_reason` é preenchido; o banco pode resubmeter corrigindo os dados.
3. **Given** usuário sem role `ROLE_GOVERNANCE` tenta aprovar ou rejeitar KYC, **When** chama o endpoint, **Then** recebe HTTP 403 (acesso negado). **Given** usuário sem `ROLE_GOVERNANCE` tenta chamar `GET /api/v1/onboarding/status/:requestId` ou `POST /api/v1/onboarding/complete` no Central Bank, **Then** também recebe HTTP 403.

---

### User Story 4 — Login de dois fatores PKI para bancos comerciais (Priority: P3)

Bancos comerciais com role `ROLE_COMMERCIAL_BANK` não usam senha direta: o login gera um nonce que deve ser assinado com a chave privada X.509 do banco.

**Why this priority**: Modelo de segurança exigido pela arquitetura para bancos — evita que credenciais Keycloak comprometidas sejam suficientes para acesso.

**Independent Test**: Chamar `POST /api/v1/auth/login` com `clientId` + `clientSecret` de um banco registrado e verificar que a resposta contém `{"nonce": "..."}` em vez de um token direto; depois completar via `POST /api/v1/auth/wallet/bind`.

**Acceptance Scenarios**:

1. **Given** um banco comercial envia `clientId` + `clientSecret` para `POST /api/v1/auth/login`, **When** as credenciais são válidas, **Then** o sistema retorna um `nonce` de curta duração (sem token JWT) que deve ser assinado.
2. **Given** o banco assinou o `nonce` com sua chave X.509, **When** envia a assinatura + certificado para `POST /api/v1/auth/wallet/bind`, **Then** recebe o token de sessão JWT com as roles corretas.
3. **Given** operadores de governança (ROLE_GOVERNANCE, ROLE_SUPERVISOR) fazem login, **When** chamam `POST /api/v1/auth/login`, **Then** recebem o token JWT diretamente (fluxo sem nonce PKI).

---

### User Story 5 — Scripts de tryout funcionam para todos os bancos (Priority: P3)

Os scripts de teste (`tryout-my-onboarding-status.sh`, `tryout-spoke-a-bank-a.sh`, `tryout-spoke-a-bank-c.sh`, `tryout-spoke-b-bank-b.sh`, `tryout-spoke-b-bank-d.sh`) são usados pela equipe para validar o ambiente após deploy.

**Why this priority**: Qualidade operacional — permite verificação rápida do estado do onboarding sem setup manual.

**Independent Test**: Executar `tryout-my-onboarding-status.sh` com `BANK_URL` e `BANK_ENV` apontando para Bank-B, Bank-C ou Bank-D e verificar que o script roda com 0 falhas.

**Acceptance Scenarios**:

1. **Given** qualquer banco (A, B, C ou D) com stack rodando, **When** o script de tryout correspondente é executado, **Then** completa sem erros com `FAIL=0`.
2. **Given** `tryout-my-onboarding-status.sh` é executado com `BANK_ENV` de Bank-B, **When** tenta fazer login, **Then** usa `KC_CLIENT_ID` do arquivo `.env` (não `bank-a-client` hardcoded) e autentica com sucesso.

---

### Edge Cases

- O que acontece quando o Central Bank **rejeita** o KYC do banco? O status muda para `KYC_REJECTED` com campo `rejection_reason` preenchido; o banco pode corrigir os dados e resubmeter via `POST /api/v1/onboarding/initiate`, criando um **novo `request_id`** — o registro anterior permanece com status `KYC_REJECTED` no histórico de auditoria.
- O que retorna `GET /api/v1/onboarding/my-status` quando um banco tem múltiplos registros (ex.: um `KYC_REJECTED` antigo e um `CREDENTIAL_REQUESTED` novo)? O endpoint sempre retorna o estado do registro mais recente para o `bank_code`.
- O que acontece quando o container do Central Bank está fora do ar e o banco tenta iniciar onboarding? O proxy deve retornar HTTP 502 (Bad Gateway) com mensagem clara.
- O que acontece quando o Central Bank está online mas não responde dentro do prazo? O `OnboardingProxyHandler` aguarda até o timeout definido em `CENTRAL_BANK_TIMEOUT` (default 10s) e retorna HTTP 504 (Gateway Timeout) com mensagem indicando o timeout — distinto do 502 que indica o CB inacessível.
- O que acontece quando o banco envia uma assinatura inválida para `POST /api/v1/auth/wallet/bind`? O endpoint retorna HTTP 401; o nonce **não é invalidado** — o banco pode tentar novamente com a assinatura corrigida até o TTL de 30 minutos expirar. O nonce só é consumido (invalidado) após uso bem-sucedido.
- O que acontece se o arquivo CSR `PKI_DIR/{bankCode}.csr` não existir no modo smart proxy? O sistema deve retornar HTTP 500 com mensagem indicando o arquivo ausente.
- O que acontece se `DropScenarioATables` remover a tabela `onboarding_requests` no startup? A tabela de onboarding não pode ser dropada — o cleanup do Scenario A deve ser removido ou condicionalizado.
- O que acontece se o banco já completou onboarding e tenta chamar `my-status`? Deve retornar status `COMPLETED` (não `NONE`).

---

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: O API Gateway de bancos comerciais DEVE expor o grupo de rotas `/api/v1/onboarding` com os endpoints: `POST /initiate`, `GET /status/:requestId`, `GET /my-status`, `POST /complete`.
- **FR-002**: O API Gateway do Central Bank DEVE expor o grupo de rotas `/api/v1/onboarding` com os endpoints: `POST /credential-request`, `GET /status/:requestId`, `GET /my-status`, `POST /complete`.
- **FR-003**: O router do API Gateway DEVE re-incluir `OnboardingProxyHandler` e `OnboardingHandler` na struct `Dependencies` e registrá-los condicionalmente (proxy quando `CENTRAL_BANK_API_URL` está configurado; handler direto no Central Bank).
- **FR-004**: O `app.go` DEVE instanciar `OnboardingProxyHandler` quando `CENTRAL_BANK_API_URL` estiver configurado e `OnboardingHandler` quando não estiver, usando as mesmas dependências de `identityManager` já presentes.
- **FR-005**: O endpoint `POST /api/v1/auth/pki-login` DEVE ser registrado no grupo de auth quando `OnboardingProxyHandler` estiver disponível, permitindo re-login PKI após onboarding.
- **FR-006**: O endpoint `GET /api/v1/onboarding/my-status` DEVE resolver o `bank_code` a partir do claim `BankID` do JWT da sessão, sem exigir parâmetro de query explícito do frontend.
- **FR-007**: Quando nenhum registro de onboarding for encontrado para o `bank_code`, `GET /api/v1/onboarding/my-status` DEVE retornar `{"status": "NONE"}` com HTTP 200 (não HTTP 404).
- **FR-008**: O script `tryout-my-onboarding-status.sh` DEVE ler o `clientId` da variável `KC_CLIENT_ID` do arquivo `.env` em vez de usar o valor `"bank-a-client"` hardcoded na linha 199.
- **FR-009**: A função `DropScenarioATables` e toda a lógica de cleanup do Scenario A DEVEM ser **completamente removidas** do startup do API Gateway. O Scenario B é um ambiente 100% novo — não há resíduos do Scenario A a limpar. A tabela `onboarding_requests` DEVE ser criada automaticamente via GORM `AutoMigrate` na inicialização, garantindo que ambientes novos funcionem sem intervenção manual.
- **FR-010**: Modelo de autenticação das rotas de onboarding:
  - **Bancos comerciais** (`OnboardingProxyHandler`): todas as rotas `/api/v1/onboarding/*` DEVEM ser protegidas por `RequireCookieAuth`.
  - **Central Bank** (`OnboardingHandler`): `POST /api/v1/onboarding/credential-request` é **pública** (sem auth — banco comercial ainda não tem token no CB neste momento). `GET /api/v1/onboarding/status/:requestId` e `POST /api/v1/onboarding/complete` DEVEM exigir role `ROLE_GOVERNANCE` (operador do CB que consulta ou conclui o registro).
- **FR-010b**: O fluxo de aprovação KYC DEVE suportar rejeição: quando o operador do Central Bank rejeita, o status muda para `KYC_REJECTED` e o campo `rejection_reason` é persistido. A regra de idempotência de `POST /api/v1/onboarding/initiate` por status é: `CREDENTIAL_REQUESTED` → HTTP 409; `KYC_APPROVED` → HTTP 409; `COMPLETED` → HTTP 409; `KYC_REJECTED` → HTTP 201 com **novo `request_id`** (resubmissão permitida). O registro anterior com status `KYC_REJECTED` permanece imutável para fins de rastreabilidade — não é atualizado nem deletado. O endpoint `GET /api/v1/onboarding/my-status` sempre retorna o estado do registro **mais recente** para o `bank_code` (maior `created_at`).
- **FR-010a**: O nonce PKI gerado em `POST /api/v1/auth/login` para bancos comerciais DEVE ter TTL de **30 minutos**; após esse prazo, o nonce é invalidado e o banco deve iniciar um novo login. O nonce é invalidado imediatamente após **uso bem-sucedido** em `POST /api/v1/auth/wallet/bind` (one-time use). Em caso de assinatura inválida, o endpoint DEVE retornar HTTP 401 com mensagem de erro mas o nonce **permanece válido** — o banco pode corrigir a assinatura e tentar novamente dentro do TTL.
- **FR-011**: As rotas Scenario B v2 DEVEM continuar funcionando sem regressão após a restauração das rotas de onboarding e auth.
- **FR-012**: O modo "smart proxy" do `OnboardingProxyHandler` DEVE continuar funcional: quando `PKI_DIR`, `BANK_CODE` e `keyMgr` estão configurados, o proxy lê o CSR do disco e injeta `csr_pem` e `blockchain_pub_key_hex` no payload automaticamente.
- **FR-013**: Cada transição de estado de uma `OnboardingRequest` (CREDENTIAL_REQUESTED → KYC_APPROVED, → KYC_REJECTED, → COMPLETED) DEVE gerar um registro imutável na tabela `audit_logs` do compliance service (decisão D-003: reutilizar infraestrutura existente em vez de criar nova tabela). Os campos do `AuditLogModel` utilizados são: `action_type` (ex.: `"CREDENTIAL_REQUEST"`, `"KYC_APPROVED"`, `"KYC_REJECTED"`), `actor_subject` (user_id do ator), `target_subject` (user_id do banco), `result` (`"SUCCESS"`), `details` (jsonb com `{"request_id": "...", "bank_code": "...", "from_status": "...", "to_status": "..."}`). Nenhuma nova tabela deve ser criada.
- **FR-014**: O `OnboardingProxyHandler` DEVE respeitar um timeout configurável para chamadas ao Central Bank, lido da variável de ambiente `CENTRAL_BANK_TIMEOUT` (valor em segundos inteiros; default `10`). Quando o Central Bank não responder dentro desse prazo, o proxy DEVE retornar HTTP 504 (Gateway Timeout) com body `{"error": "central bank timeout"}`. Quando inacessível (conexão recusada), deve retornar HTTP 502.

### Key Entities

- **OnboardingRequest**: Representa uma solicitação de registro de banco comercial. Atributos: `request_id`, `user_id`, `bank_code`, `status` (CREDENTIAL_REQUESTED → KYC_APPROVED → COMPLETED | KYC_REJECTED), `pop_nonce`, `wallet_address`, `rejection_reason` (preenchido apenas quando status = KYC_REJECTED).
- **OnboardingEvent**: Registro conceitual imutável de cada transição de estado de uma `OnboardingRequest` (mantido para compatibilidade terminológica com o cenário A). Na implementação desta feature, os eventos são persistidos fisicamente em `audit_logs` (com `action_type`, `actor_subject`, `target_subject`, `result`, `details`) conforme FR-013; sem endpoint de consulta nesta fase.
- **TokenClaims**: Claims do JWT de sessão enriquecidos com `BankID`, roles e wallet address — usados pelo middleware para resolver a identidade do banco sem parâmetros explícitos.
- **CredentialRequest**: Payload da Phase 1: `csr_pem`, `blockchain_pub_key_hex`, `institution_name`, `bank_code`, `country`, `role`, `email`, `username`.
- **OnboardingProxyHandler**: Componente do API Gateway dos bancos comerciais que encaminha as requisições de onboarding ao Central Bank, enriquecendo o payload com CSR e chave blockchain (smart mode).

---

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Todos os 4 bancos comerciais (A, B, C, D) concluem o fluxo de onboarding de 3 fases nos seus spokes correspondentes sem erros HTTP 404 ou 500.
- **SC-002**: Os scripts de tryout (`tryout-spoke-a-bank-a.sh`, `tryout-spoke-a-bank-c.sh`, `tryout-spoke-b-bank-b.sh`, `tryout-spoke-b-bank-d.sh`) completam com `FAIL=0` para todos os bancos após a mudança.
- **SC-003**: O script `tryout-my-onboarding-status.sh` executado com variáveis de qualquer banco (A, B, C ou D) retorna `PASS=4, FAIL=0`.
- **SC-004**: As rotas do Scenario B v2 (`/api/v2/pool`, `/api/v2/swap`, `/api/v2/bridge`, etc.) continuam respondendo corretamente após a restauração — zero regressões detectadas nos testes existentes.
- **SC-005**: Um banco que recarrega a página após iniciar o onboarding consegue recuperar seu status atual em menos de 1 segundo via `GET /api/v1/onboarding/my-status`.
- **SC-006**: O processo de login PKI completo (nonce → assinatura → bind → token) é concluído em menos de 5 segundos em condições normais de rede local.

---

## Assumptions

- Os handlers `OnboardingHandler` e `OnboardingProxyHandler` existentes na branch atual (`backend/services/api-gateway/internal/http/handlers/onboarding.go` e `onboarding_proxy.go`) são funcionalmente equivalentes aos da `develop-scenario-a` e não precisam ser reescritos — apenas re-conectados ao router e ao `app.go`.
- A interface `OnboardingManager` e `OnboardingKeyManager` definidas em `interfaces/kyc_checker.go` na branch atual são compatíveis com as dependências dos handlers existentes.
- O `identityManager` já instanciado em `app.go` implementa tanto `OnboardingManager` quanto `OnboardingKeyManager`, pois os métodos correspondentes (`SubmitCredentialRequest`, `GetOnboardingStatus`, etc.) já existem em `identity_grpc_manager.go`.
- A variável de ambiente `CENTRAL_BANK_API_URL` já está configurada nos arquivos `.env.infra.bank-*` para todos os bancos comerciais — não é necessário alterar a infra.
- O Scenario B e o Scenario A podem coexistir no mesmo `router.go` sem conflito de rotas, pois usam prefixos distintos (`/api/v1/` vs rotas v2 registradas pelo `v2router`).
- O Scenario B é um ambiente 100% novo — nenhum participante herda estado do Scenario A. Quem precisar participar realiza o fluxo de onboarding completo do zero. Por isso, `DropScenarioATables` pode ser completamente removida e a tabela `onboarding_requests` é sempre criada via `AutoMigrate` no startup.
- Suporte mobile está fora do escopo desta feature.
