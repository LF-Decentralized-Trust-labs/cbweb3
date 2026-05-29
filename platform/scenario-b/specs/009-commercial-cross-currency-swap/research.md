# Research: Commercial Cross-Currency Swap

**Feature**: 009-commercial-cross-currency-swap  
**Phase**: Phase 0 - Architectural Research  
**Date**: 2026-05-26

## Research Questions

This document resolves architectural unknowns identified in plan.md before proceeding to detailed design (Phase 1).

---

## Q1: Padrão de Orquestração para 3-Step Flow

**Question**: Como coordenar bridge-in, swap, bridge-out com rollback parcial em caso de falha?

### Options Evaluated

**Option A: Saga Pattern com Compensating Transactions**
- Cada step é uma transação independente com compensação definida
- Se swap falhar → executar compensação (bridge reverso Hub→Spoke-A)
- Complexidade: alta (requer state machine robusto)
- Benefício: padrão bem estabelecido para distributed transactions

**Option B: Simple Transaction Coordinator**
- Função orquestradora sequencial com try-catch em cada step
- Se falhar → chamar rollback explícito
- Complexidade: baixa (código procedural)
- Benefício: simples de entender e debugar

**Option C: Event-Driven com Message Queue**
- Cada step emite evento para próximo step
- Workers processam queue
- Complexidade: muito alta (requer infraestrutura adicional)
- Benefício: resilience alta, retry automático

### Decision

**Selected**: Option B (Simple Transaction Coordinator)

**Rationale**:
- Feature scope é bounded (3 steps sequenciais bem definidos)
- Saga pattern (Option A) é overkill para operação síncrona esperada pelo usuário (latency target 60-90s)
- Event-driven (Option C) adiciona complexidade de infraestrutura desnecessária (Kafka, workers) para escala atual
- Option B permite implementação rápida reutilizando services existentes (bridge_service.go, swap_service.go)

**Implementation Approach**:
```go
func (orch *CrossCurrencySwapOrchestrator) Execute(ctx context.Context, req CrossCurrencySwapRequest) (*CrossCurrencySwapResult, error) {
    // Step 1: Bridge Spoke-A → Hub
    bridgeIn, err := orch.bridgeService.LockAndEnqueue(...)
    if err != nil { return nil, err }
    
    // Step 2: Swap Hub W-BRL → W-ARS
    swap, err := orch.swapService.Execute(...)
    if err != nil {
        // Rollback: bridge reverso Hub → Spoke-A
        orch.rollbackCoordinator.ReverseBridge(bridgeIn.PositionID)
        return nil, err
    }
    
    // Step 3: Bridge Hub → Spoke-B
    bridgeOut, err := orch.bridgeService.BurnAndEnqueue(...)
    if err != nil {
        // Partial success — swap succeeded but bridge-out failed
        // Log for manual intervention (cannot undo on-chain swap)
        return nil, err
    }
    
    return &CrossCurrencySwapResult{...}, nil
}
```

**Alternatives Rejected**:
- Saga: overhead não justificado para operação síncrona de curta duração
- Event-driven: adiciona dependências de infraestrutura (message broker) sem benefício claro para escala MVP

---

## Q2: Quote Expiry Strategy

**Question**: Como validar expiry de quote (15s) e comportamento quando expirada?

### Options Evaluated

**Option A: Server-Side Timestamp Validation**
- Quote inclui `created_at` e `valid_until` timestamps
- Backend valida `time.Now() <= valid_until` antes de executar swap
- Rejeita com HTTP 422 QUOTE_EXPIRED se expirado
- Benefício: segurança (cliente não pode manipular tempo)
- Trade-off: requer sincronização de relógio server-client

**Option B: Client-Side Countdown Only**
- Backend não valida expiry, apenas calcula amount_in
- Frontend mostra countdown mas permite submit após expiração
- Backend valida apenas slippage on-chain
- Benefício: simplicidade
- Trade-off: vulnerabilidade a replay attacks com quotes antigas

**Option C: Signed Quote Token (JWT-like)**
- Backend gera JWT assinado com quote params + expiry
- Cliente envia token com swap request
- Backend valida assinatura + expiry
- Benefício: máxima segurança
- Trade-off: complexidade criptográfica desnecessária

### Decision

**Selected**: Option A (Server-Side Timestamp Validation)

**Rationale**:
- Protege contra replay attacks (cliente reutilizar quote antiga quando preço mudou adversamente)
- Validação server-side é standard practice para operações financeiras sensíveis
- Trade-off de sincronização é aceitável (servidores NTP-sync'd em prod)
- Option B (client-only) é inseguro para operações monetárias
- Option C (signed token) é over-engineering sem ganho significativo vs Option A

**Implementation Details**:
```go
type SwapQuote struct {
    QuoteID      string
    PoolPair     string
    AmountOut    string
    AmountIn     string  // Calculated via x·y=k formula
    EffectiveRate float64
    CreatedAt    time.Time
    ValidUntil   time.Time  // CreatedAt + 15s
}

func (svc *SwapQuoteGenerator) GenerateQuote(...) (*SwapQuote, error) {
    now := time.Now()
    return &SwapQuote{
        CreatedAt:  now,
        ValidUntil: now.Add(15 * time.Second),
        ...
    }, nil
}

func (orch *CrossCurrencySwapOrchestrator) Execute(ctx, req) error {
    if time.Now().After(req.Quote.ValidUntil) {
        return &QuoteExpiredError{...}  // HTTP 422
    }
    // Proceed with swap...
}
```

**Frontend Behavior**:
- Mostrar countdown timer visual (15s → 0s)
- Desabilitar botão "Confirmar Swap" quando countdown chegar a 0
- Exibir mensagem "Quote expirada - obtenha nova cotação"
- Auto-refresh opcional (UX decision): perguntar usuário "Renovar automaticamente?"

---

## Q3: Rollback Mechanics para Bridge Reverso

**Question**: Como executar bridge reverso (Hub→Spoke-A) se swap falhar após bridge-in bem-sucedido?

### Options Evaluated

**Option A: Automatic Rollback via Same Orchestrator**
- Orquestrador chama `bridgeService.BurnAndEnqueue(positionID)` com target=Spoke-A (origem)
- Relayer detecta burn no Hub e faz unlock no Spoke-A
- Benefício: automático, sem intervenção manual
- Trade-off: requer gas reserve no Hub para burn tx

**Option B: Async Rollback Worker**
- Orquestrador marca operação como NEEDS_ROLLBACK no DB
- Background worker processa rollback queue
- Benefício: não bloqueia response do usuário
- Trade-off: maior latência para devolver fundos

**Option C: Manual Intervention via Admin Tool**
- Swap failure registra posição stuck no DB
- Admin executa rollback via ferramenta interna
- Benefício: controle máximo
- Trade-off: bad UX (usuário aguarda intervenção manual)

### Decision

**Selected**: Option A (Automatic Rollback via Same Orchestrator)

**Rationale**:
- Melhor UX: usuário recebe fundos de volta imediatamente (dentro de ~30s de latency do bridge)
- Reutiliza infraestrutura de bridge existente (BurnAndEnqueue já implementado)
- Option B (async worker) adiciona complexidade de queue processing sem benefício claro (rollback é raro, não precisa otimizar)
- Option C (manual) é inaceitável para produção (violar expectativa de self-service)

**Implementation**:
```go
func (coord *SwapRollbackCoordinator) ReverseBridge(ctx context.Context, bridgeInPositionID string) error {
    // Query bridge position to get original owner_bank_id
    pos, err := coord.bridgeRepo.FindByID(ctx, bridgeInPositionID)
    if err != nil { return err }
    
    // Execute burn on Hub → unlock on original spoke (Spoke-A)
    _, err = coord.bridgeService.BurnAndEnqueue(ctx, pos.PositionID, pos.OwnerBankID, pos.SpokeNetwork)
    if err != nil {
        // Log critical error — requires manual intervention
        log.Error("rollback bridge failed", "position_id", bridgeInPositionID, "error", err)
        return err
    }
    
    return nil
}
```

**Edge Cases**:
- **Gas insuficiente no signer Hub**: Rollback tx falha → registrar em audit log para retry manual
- **Relayer offline**: Burn tx sucede mas unlock no Spoke-A trava → posição fica stuck, requer restart do Relayer (mesma failure mode que bridge normal)
- **Double rollback attempt**: Prevenir com state check (`if pos.State != ACTIVE { return ErrAlreadyProcessed }`)

---

## Q4: Rate Limiting Approach

**Question**: Como implementar 10 swaps/min, 100 swaps/hora por banco comercial?

### Options Evaluated

**Option A: In-Memory Rate Limiter (Redis)**
- Usar Redis INCR + EXPIRE para contadores por bank_id
- Chave: `rate_limit:swap:{bank_id}:minute` TTL 60s
- Chave: `rate_limit:swap:{bank_id}:hour` TTL 3600s
- Benefício: performance alta (memória)
- Trade-off: requer Redis cluster para HA

**Option B: Database-Backed Rate Limiter (PostgreSQL)**
- Tabela `swap_rate_limits` com bank_id, window_start, swap_count
- Query: `SELECT COUNT(*) FROM cross_currency_swap_operations WHERE bank_id=? AND created_at > NOW() - INTERVAL '1 minute'`
- Benefício: sem dependência externa
- Trade-off: performance inferior (disk I/O)

**Option C: Token Bucket Algorithm (in-process)**
- Manter bucket in-memory por bank_id
- Refill 10 tokens/min, capacity 100
- Benefício: algoritmo clássico, fairness garantido
- Trade-off: state perdido em restart (aceitável para rate limiting)

### Decision

**Selected**: Option B (Database-Backed Rate Limiter) para MVP, com path para Option A em prod

**Rationale**:
- MVP não requer performance extrema (dezenas de bancos, não milhares)
- PostgreSQL já é dependência existente, evita adicionar Redis (simplicity)
- Query `COUNT(*) WHERE created_at > NOW() - INTERVAL '1 minute'` é suficientemente rápido com index em (bank_id, created_at)
- Option A (Redis) pode ser adicionada depois se profile de performance exigir (migration path claro)
- Option C (in-process) perde state em restart, inapropriado para limite regulatório

**Implementation**:
```go
func (limiter *DatabaseRateLimiter) CheckLimit(ctx context.Context, bankID string) error {
    // Check minute window
    var countMin int64
    err := limiter.db.QueryRow(`
        SELECT COUNT(*) FROM cross_currency_swap_operations 
        WHERE payer_bank_id = $1 AND created_at > NOW() - INTERVAL '1 minute'
    `, bankID).Scan(&countMin)
    if err != nil { return err }
    if countMin >= 10 {
        return &RateLimitExceededError{Window: "minute", Limit: 10}  // HTTP 429
    }
    
    // Check hour window
    var countHour int64
    err = limiter.db.QueryRow(`
        SELECT COUNT(*) FROM cross_currency_swap_operations 
        WHERE payer_bank_id = $1 AND created_at > NOW() - INTERVAL '1 hour'
    `, bankID).Scan(&countHour)
    if err != nil { return err }
    if countHour >= 100 {
        return &RateLimitExceededError{Window: "hour", Limit: 100}  // HTTP 429
    }
    
    return nil
}
```

**Migration Path to Redis**:
Se performance se tornar bottleneck (>100 bancos, >1000 swaps/min):
- Implementar `RedisRateLimiter` com mesma interface `RateLimiterIface`
- Substituir no dependency injection (app.go)
- Manter database backup para auditoria histórica

---

## Q5: Correlation ID Propagation

**Question**: Como propagar correlation_id através de bridge-in, swap, bridge-out para auditoria?

### Options Evaluated

**Option A: UUID Gerado pelo Orquestrador**
- Orquestrador gera `correlation_id` (UUID v4) no início
- Passa para bridge_service, swap_service via context
- Services incluem em logs e DB records
- Benefício: simplicidade, ownership claro
- Trade-off: requer passar explicitamente em todas as calls

**Option B: Context-Based Propagation (Go context.Context)**
- Armazenar correlation_id em `ctx = context.WithValue(ctx, "correlation_id", uuid)`
- Services extraem via `ctx.Value("correlation_id")`
- Benefício: propagação implícita
- Trade-off: pode ser perdido se context não for propagado corretamente

**Option C: Thread-Local Storage (Go goroutine-local)**
- Não existe em Go (diferente de Java ThreadLocal)
- Simulável com goroutine ID mas antipattern
- Rejected imediatamente

### Decision

**Selected**: Hybrid Option A + B (UUID em Context + Explicit Passing)

**Rationale**:
- Context propagation (Option B) é idiomático em Go para valores cross-cutting
- Explicit passing (Option A) garante visibilidade nos function signatures
- Combinar os dois: gerar UUID no orquestrador, armazenar em context, passar explicitamente em structs de request

**Implementation**:
```go
type ContextKey string
const CorrelationIDKey ContextKey = "correlation_id"

func (orch *CrossCurrencySwapOrchestrator) Execute(ctx context.Context, req CrossCurrencySwapRequest) (*CrossCurrencySwapResult, error) {
    // Generate correlation ID
    correlationID := uuid.New().String()
    ctx = context.WithValue(ctx, CorrelationIDKey, correlationID)
    
    // Log start
    log.Info("cross-currency swap started", "correlation_id", correlationID, "payer", req.PayerBankID)
    
    // Pass to services (context já inclui correlation_id)
    bridgeIn, err := orch.bridgeService.LockAndEnqueue(ctx, ...)
    if err != nil {
        log.Error("bridge-in failed", "correlation_id", correlationID, "error", err)
        return nil, err
    }
    
    swap, err := orch.swapService.Execute(ctx, ...)
    // ...
}

// Utility function para extrair de context
func GetCorrelationID(ctx context.Context) string {
    if id, ok := ctx.Value(CorrelationIDKey).(string); ok {
        return id
    }
    return "unknown"
}
```

**Database Schema**:
```sql
CREATE TABLE cross_currency_swap_operations (
    id UUID PRIMARY KEY,
    correlation_id UUID NOT NULL,  -- Links bridge-in, swap, bridge-out
    payer_bank_id TEXT NOT NULL,
    -- ...
    bridge_in_position_id UUID,
    swap_tx_hash TEXT,
    bridge_out_position_id UUID,
    created_at TIMESTAMP NOT NULL,
    INDEX idx_correlation_id (correlation_id)
);
```

**Query Example**:
```sql
-- Audit trail: todas as operações de um swap
SELECT * FROM cross_currency_swap_operations WHERE correlation_id = '<uuid>';
SELECT * FROM bridge_positions WHERE id IN (SELECT bridge_in_position_id FROM cross_currency_swap_operations WHERE correlation_id = '<uuid>');
SELECT * FROM bridge_positions WHERE id IN (SELECT bridge_out_position_id FROM cross_currency_swap_operations WHERE correlation_id = '<uuid>');
```

---

## Summary of Decisions

| Question | Decision | Key Trade-Off |
|----------|----------|---------------|
| Q1: Orchestration Pattern | Simple Transaction Coordinator | Simplicidade vs saga pattern robustness |
| Q2: Quote Expiry | Server-Side Timestamp Validation | Segurança vs clock sync overhead |
| Q3: Rollback Mechanics | Automatic Rollback via Orchestrator | UX imediato vs async worker resilience |
| Q4: Rate Limiting | Database-Backed (MVP) → Redis (Prod) | Simplicidade inicial vs performance final |
| Q5: Correlation ID | UUID em Context + Explicit Passing | Visibility vs propagation elegance |

Todas as decisões priorizaram simplicidade e reutilização de infraestrutura existente (bridge, swap services) sobre patterns complexos, apropriado para escopo MVP desta feature.

---

## Next Steps

Com research completo, prosseguir para **Phase 1** (data-model.md, contracts/, quickstart.md).
