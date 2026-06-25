# Tasks: Fix PKI Local CA Model (FIX-1)

**Input**: Design documents de `/specs/016-fix-pki-local-ca/`
**Branch**: `016-fix-pki-local-ca`
**Spec**: spec.md | **Plan**: plan.md | **Data model**: data-model.md | **Research**: research.md

**Organização**: Tasks agrupadas por user story para implementação e validação independentes.

## Formato: `[ID] [P?] [Story?] Descrição com caminho do arquivo`

- **[P]**: Pode ser executada em paralelo (arquivos diferentes, sem dependências incompletas)
- **[Story]**: A qual user story a task pertence (US1, US2, US3)

---

## Phase 1: Setup (Verificação de pré-condições)

**Objetivo**: Confirmar que o ambiente está estável antes de qualquer mudança. Base de comparação para o teste de regressão (SC-004).

- [x] T001 Executar `make pki.gen-all` no diretório `scenario-a/` e confirmar que termina sem erros — registrar saída como baseline para o teste de regressão pós-fix
- [x] T002 Confirmar que os arquivos `scenario-a/make/05-pki.mk` e `scenario-a/README.md` estão no HEAD da branch `016-fix-pki-local-ca` sem modificações pendentes

**Checkpoint**: Ambiente verificado — implementação pode começar.

---

## Phase 2: Foundational (Pré-requisitos bloqueantes)

**Objetivo**: Não há infraestrutura nova a criar nesta fix. A única fundação necessária é o entendimento confirmado do estado atual — já documentado em `research.md`. Esta fase é marcada como completa pela conclusão da Phase 1.

**⚠️ CRÍTICO**: Nenhuma user story deve ser iniciada antes de T001 e T002 serem concluídas.

---

## Phase 3: User Story 1 — Developer Bootstrap Local PKI com Clareza (Prioridade: P1) 🎯 MVP

**Objetivo**: Adicionar aviso explícito `[DEV ONLY]` ao Makefile e documentar ambos os caminhos PKI (bootstrap local vs. produção CB-signed) no README. Entrega: FR-001, FR-002, FR-003.

**Teste Independente**: Ler `scenario-a/make/05-pki.mk` e encontrar o bloco de comentário `[DEV ONLY]` acima do macro `gen_commercial_bank_cert`; executar `make pki.gen-commercial-bank-bank-a` e ver a linha de aviso no terminal; ler `scenario-a/README.md` e encontrar a seção PKI com ambos os caminhos documentados.

### Implementação para User Story 1

- [x] T003 [US1] Adicionar bloco de comentário `[DEV ONLY]` imediatamente acima do macro `gen_commercial_bank_cert` em `scenario-a/make/05-pki.mk` — o comentário DEVE conter: (1) "LOCAL DEV ONLY — NÃO representa o fluxo de produção", (2) descrição do fluxo correto de produção (CSR → CB assina), (3) referência ao `onboarding_proxy.go` smart mode como implementação de referência

- [x] T004 [US1] Adicionar linha de echo `[DEV ONLY]` ao target `pki.gen-commercial-bank-%` em `scenario-a/make/05-pki.mk` — a linha DEVE ser visível antes da geração do CA e DEVE mencionar que em produção a CB assina o CSR do banco; exemplo: `@echo "[DEV ONLY] Bootstrap local: banco gera CA própria. Em produção, use o fluxo CSR→CB via onboarding_proxy.go"`

- [x] T005 [US1] Adicionar bloco de comentário `[DEV ONLY]` no target `pki.gen-bank-a` e `pki.gen-bank-b` em `scenario-a/make/05-pki.mk` — estes targets geram CAs de banco (não de banco central) e precisam do mesmo aviso

- [x] T006 [US1] Adicionar seção "Modelo de Confiança PKI" ao `scenario-a/README.md` contendo:
  - Subseção "Caminho local (bootstrap de desenvolvimento)": descreve o que `make pki.gen-commercial-bank-*` faz, com aviso explícito de que é atalho de dev, cada banco gera CA própria auto-assinada
  - Subseção "Caminho de produção (CB assina o CSR)": descreve o fluxo correto — banco gera keypair + CSR, submete ao CB via `POST /api/v1/onboarding/credential-request`, CB assina com `central-bank-<x>-ca.key`, banco recebe certificado leaf
  - Referência ao `scenario-a/backend/services/api-gateway/internal/http/handlers/onboarding_proxy.go` (smart mode) como implementação canônica do fluxo de produção
  - Referência ao `scenario-a/backend/shared/identity/ca.go` como biblioteca PKI de referência (`GenerateCSR`, `SignCSR`, `GenerateSelfSignedCA`)

- [x] T007 [US1] Executar `make pki.gen-all` em `scenario-a/` com FORCE=1 após as mudanças em `05-pki.mk` e confirmar: (1) o comando termina sem erros (SC-004 — sem regressão), (2) a linha `[DEV ONLY]` aparece no output para os targets de banco comercial, (3) os certificados são gerados corretamente

**Checkpoint**: User Story 1 completamente funcional — desenvolvedor vê aviso claro, README documenta ambos os caminhos, regressão local não ocorreu.

---

## Phase 4: User Story 2 — Operador Provisiona Banco Comercial com Certificado Assinado pela CB (Prioridade: P2)

**Objetivo**: Escrever testes com falha (fase RED) que definem o contrato do toolkit PKI para TK-9 (commercial-bank join flow). O stub garante que os testes compilam mas falham. Entrega: pre-work para FR-004, FR-005, FR-006, SC-002, SC-003, SC-005.

**Teste Independente**: `go test ./toolkit/engine/pki/... -v` retorna FAIL com mensagem "not implemented" — confirma que a fase RED está completa e o contrato está definido.

> **NOTA**: Esta fase entrega apenas os testes com falha + stub (fase RED da constituição Principle V). A implementação que faz os testes passarem é Phase C do plan.md, em PR separado para TK-9.

### Testes para User Story 2 (Constituição Princípio V — OBRIGATÓRIO: teste ANTES da implementação) ⚠️

- [x] T008 [US2] Criar diretório `scenario-a/toolkit/engine/pki/` e arquivo `scenario-a/toolkit/engine/pki/csr_test.go` com os seguintes casos de teste:
  - `TestGenerateCSR_NoCAKeyCreated`: chama o gerador de CSR do toolkit e verifica que nenhum arquivo `*-ca.key` ou `*-ca.crt` é criado no diretório de saída (SC-003)
  - `TestGenerateCSR_ProducesKeyAndCSR`: verifica que `{bankCode}.key` e `{bankCode}.csr` são criados com os atributos corretos — `CN={bankCode}`, `O={institution}`, `OU=ROLE_COMMERCIAL_BANK`, `C=BR`
  - `TestSubmitCSR_FailsFastWhenCBUnreachable`: submete CSR a um endpoint inexistente e verifica que o toolkit retorna erro não-nil dentro de 30 segundos, sem fallback para auto-assinatura (SC-005)
  - `TestSubmitCSR_FailsFastWhenCBReturnsError`: submete CSR a um test server HTTP que retorna 500 e verifica que o toolkit retorna erro, sem criar `{bankCode}-ca.key` (SC-005)
  - `TestIssuedCert_IssuerIsCBCA`: usando `NewCAFromPEM` de `scenario-a/backend/services/compliance/internal/pki/ca.go` como CA de teste, verifica que o certificado retornado pelo toolkit tem `Issuer` igual ao CN da CA de teste, não ao `{bankCode}` (SC-002)
  - Usar `t.TempDir()` para isolar artefatos de arquivo; usar `net/http/httptest` para simular o endpoint CB

- [x] T009 [P] [US2] Criar `scenario-a/toolkit/engine/pki/csr.go` com assinaturas de função que façam `csr_test.go` compilar, mas com corpo retornando `errors.New("not implemented")`:
  - `GenerateBankCSR(bankCode, institution, outputDir string) error`
  - `SubmitCSRToCB(csrPath, cbCredentialRequestURL string, timeout time.Duration) (certPEM string, err error)`
  - `StoreCertificate(certPEM, bankCode, outputDir string) error`
  - Adicionar import do `shared/identity` e `compliance/pki` para confirmar que as dependências estão acessíveis

- [x] T010 [US2] Criar `scenario-a/toolkit/engine/pki/go.mod` (ou incluir no módulo Go existente do toolkit, se houver) — verificar se existe um `go.mod` em `scenario-a/toolkit/` ou `scenario-a/backend/` que deveria incluir o novo pacote; garantir que `go build ./scenario-a/toolkit/...` funciona

- [x] T011 [US2] Executar `go test ./scenario-a/toolkit/engine/pki/... -v` e confirmar: (1) todos os 5 testes compilam, (2) todos os testes FALHAM com "not implemented" ou similar — fase RED da constituição Principle V confirmada (T008, T009 devem estar completos)

**Checkpoint**: Fase RED completa — testes definem o contrato do toolkit PKI. Nenhum banco comercial pode gerar CA key sem quebrar os testes. Implementação (faz testes ficarem GREEN) é responsabilidade do PR TK-9.

---

## Phase 5: User Story 3 — Auditor Verifica Ausência de CA de Banco em Artefatos de Produção (Prioridade: P3)

**Objetivo**: Garantir que o teste T008 (`TestGenerateCSR_NoCAKeyCreated`) já cobre esta user story. A tarefa aqui é adicionar uma asserção explícita de nomes de arquivo proibidos ao teste, e documentar o procedimento de auditoria no README.

**Teste Independente**: `grep -r "ca.key\|ca.crt" scenario-a/toolkit/engine/pki/csr.go` retorna zero resultados — nenhum caminho no código do toolkit gera arquivo CA para banco comercial.

### Implementação para User Story 3

- [x] T012 [P] [US3] Verificar que `csr_test.go` (criado em T008) inclui asserção explícita da lista de arquivos proibidos: o teste `TestGenerateCSR_NoCAKeyCreated` deve usar `filepath.Glob(outputDir + "/*-ca.*")` e confirmar que o resultado é vazio — se a asserção não estiver presente, adicioná-la

- [x] T013 [P] [US3] Adicionar subseção "Auditoria de Artefatos PKI" ao `scenario-a/README.md` (dentro da seção criada em T006) descrevendo o procedimento de inspeção:
  - Comando para verificar ausência de CA de banco: `find backend/config/pki/ -name "*-ca.key" | grep -v "central-bank"` deve retornar vazio após um join de banco comercial via toolkit
  - Referência ao SC-003: "Zero arquivos correspondendo a `*-ca.key` ou `*-ca.crt` devem aparecer como output de artefato de um join de banco comercial executado via toolkit"
  - Nota sobre o ambiente local (Makefile): explica que `bank-a-ca.key` PODE existir no ambiente local de dev pois o Makefile bootstrap cria (confirmado pelo aviso `[DEV ONLY]`), mas isso NÃO deve aparecer em artefatos do toolkit

**Checkpoint**: US3 coberta — asserção de auditoria presente nos testes, procedimento de inspeção documentado no README.

---

## Phase Final: Polimento e Verificação Cruzada

**Objetivo**: Revisão final de consistência, regressão e qualidade.

- [x] T014 [P] Rever `scenario-a/make/05-pki.mk` inteiro para garantir que nenhum outro target gera CA para banco comercial sem aviso `[DEV ONLY]` — verificar especialmente `pki.gen-all` e quaisquer targets de conveniência compostos

- [x] T015 [P] Rever `scenario-a/README.md` para garantir que a seção PKI adicionada em T006 e T013 é consistente com o conteúdo existente do README — sem duplicação, sem contradição com seções existentes

- [x] T016 Executar `make pki.gen-all` uma última vez em `scenario-a/` após todas as mudanças e confirmar output completo limpo, incluindo linhas `[DEV ONLY]` para os targets de banco comercial (validação final de SC-004)

- [x] T017 [P] Executar `go build ./scenario-a/toolkit/...` e confirmar zero erros de compilação (stub deve compilar mesmo sem implementação)

- [x] T018 Revisar o diff final com `git diff develop` e confirmar que: (1) nenhuma linha funcional foi alterada em `05-pki.mk` além de comentários e echos, (2) nenhum arquivo de contrato ou endpoint foi modificado, (3) todos os novos arquivos pertencem a `scenario-a/`

---

## Dependências e Ordem de Execução

### Dependências entre Phases

- **Phase 1 (Setup)**: Sem dependências — inicia imediatamente
- **Phase 2 (Foundational)**: Depende de Phase 1 — completa ao término de T001/T002
- **Phase 3 (US1)**: Pode iniciar após Phase 2 — independente de US2 e US3
- **Phase 4 (US2)**: Pode iniciar após Phase 2 — independente de US1 (arquivos diferentes)
- **Phase 5 (US3)**: Depende de T008 (teste de US2 deve existir para T012 validar)
- **Phase Final**: Depende de todas as user stories estarem completas

### Dependências dentro de cada User Story

```
US1: T003 → T004 → T005 → T006 → T007 (sequencial: cada step depende do anterior)
         ↘ T004 pode ser feito em paralelo com T003 (targets diferentes do Makefile)
         ↘ T005 pode ser feito em paralelo com T003 e T004
         ↘ T006 pode ser feito em paralelo com T003, T004, T005 (arquivo diferente: README)
         ↘ T007 depende de T003+T004+T005 (regressão test precisa das mudanças)

US2: T008 → T009 → T010 → T011 (T008 e T009 podem ser paralelos; T010 após T009; T011 após tudo)

US3: T012 e T013 são paralelos entre si; ambos dependem de T008 estar completo
```

### Oportunidades de Paralelismo

```bash
# Phase 3 — US1: T003, T004, T005 e T006 podem rodar em paralelo (arquivos diferentes)
Task: "Comentário DEV ONLY no macro gen_commercial_bank_cert em 05-pki.mk"   # T003
Task: "Echo DEV ONLY no target pki.gen-commercial-bank-% em 05-pki.mk"       # T004
Task: "Comentário DEV ONLY nos targets pki.gen-bank-a/b em 05-pki.mk"        # T005
Task: "Seção PKI no scenario-a/README.md"                                     # T006

# Phase 4 — US2: T008 e T009 podem rodar em paralelo (csr_test.go vs csr.go)
Task: "Criar csr_test.go com 5 casos de teste"     # T008
Task: "Criar csr.go com stubs not implemented"      # T009
```

---

## Exemplo de Execução em Paralelo: User Story 1

```bash
# Executar em paralelo (arquivos distintos, sem dependências):
Task T003: "Adicionar comentário [DEV ONLY] ao macro gen_commercial_bank_cert em scenario-a/make/05-pki.mk"
Task T005: "Adicionar comentário [DEV ONLY] aos targets pki.gen-bank-a/b em scenario-a/make/05-pki.mk"
Task T006: "Adicionar seção PKI ao scenario-a/README.md"

# Após T003 e T005 (mesmo arquivo 05-pki.mk — cuidado com conflito):
Task T004: "Adicionar echo [DEV ONLY] ao target pki.gen-commercial-bank-% em scenario-a/make/05-pki.mk"

# Após T003, T004, T005 (validação de regressão):
Task T007: "Executar make pki.gen-all e verificar output"
```

---

## Estratégia de Implementação

### MVP (somente User Story 1)

1. Completar Phase 1: Setup (T001, T002)
2. Completar Phase 3: US1 (T003→T007)
3. **PARAR E VALIDAR**: Ler Makefile + README, executar `make pki.gen-all`
4. Fazer PR com somente Track 1 (documentação Makefile + README) se precisar separar as entregas

### Entrega Incremental

1. Phase 1 → US1 completo → commit Track 1 (documentação)
2. Phase 4 (US2) → testes com falha → commit Track 2 fase RED
3. Phase 5 (US3) → asserção de auditoria + README → commit final
4. Phase Final → polimento → PR pronto

### Track 2 separado do Track 1 (opcional)

O plan.md identifica que Track 2 (toolkit CSR) pode ser entregue em PRs separados:
- **Este PR (016)**: somente Track 1 (US1) + testes RED de Track 2 (US2 fase RED)
- **PR TK-9**: implementação que faz os testes ficarem GREEN (Phase C do plan.md)

---

## Notas

- `[P]` = arquivos diferentes, sem dependências — pode rodar em paralelo
- Label `[USn]` mapeia cada task à user story correspondente do spec.md
- Testes em T008 DEVEM falhar antes de qualquer implementação (constituição Principle V)
- T007 é o único gate de regressão — deve ser executado após cada mudança em `05-pki.mk`
- Nenhuma task modifica código funcional de backend ou contratos gRPC — escopo controlado
- Track 2 (Phase 4, US2) entrega apenas a fase RED; a implementação GREEN é responsabilidade do PR TK-9
