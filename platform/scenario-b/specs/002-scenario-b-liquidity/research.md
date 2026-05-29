# Research - Scenario B Backend Rebuild

## Decision 1: Estrategia de migracao de backend
- **Decision**: Executar corte unico (big bang) com descontinuidade completa da API atual.
- **Rationale**: O spec exige eliminacao total do Cenario A ativo e veda coexistencia operacional da API antiga.
- **Alternatives considered**:
  - Migracao faseada com compatibilidade temporaria.
  - Operacao paralela com proxy de rotas.

## Decision 2: Limite de reaproveitamento
- **Decision**: Reaproveitar apenas infraestrutura transversal (identidade, persistencia base, runtime e observabilidade base), sem reaproveitar contratos funcionais do Cenario A.
- **Rationale**: Mantem continuidade operacional de plataforma sem carregar semantica de dominio legada.
- **Alternatives considered**:
  - Reaproveitamento parcial de handlers/rotas antigas.
  - Reescrita total incluindo camada de infraestrutura.

## Decision 3: Tratamento de dados e historico legado
- **Decision**: Descontinuar historico e dados operacionais ligados ao contrato antigo na mesma janela de corte.
- **Rationale**: A diretriz aprovada prioriza ruptura completa do Cenario A e simplificacao da transicao.
- **Alternatives considered**:
  - Congelar historico para consulta read-only.
  - Migrar acervo completo para o novo modelo.

## Decision 4: Contrato da nova API B
- **Decision**: Definir contrato explicito orientado a quote/swap/estado de pool/governanca de risco, sem endpoints herdados do Cenario A.
- **Rationale**: Evita drift semantico e permite testes de contrato objetivos para o novo escopo.
- **Alternatives considered**:
  - Extensao incremental do OpenAPI atual.
  - Mapeamento de compatibilidade retroativa com endpoints legados.

## Decision 5: Cobertura de artefatos backend
- **Decision**: Cobrir substituicao de rotas, handlers, services, contratos de interface, jobs, testes e docs operacionais.
- **Rationale**: O spec exige substituicao total da esteira funcional para garantir "zero artefatos ativos" do Cenario A.
- **Alternatives considered**:
  - Troca apenas da borda HTTP.
  - Troca de API + servicos de pagamento, mantendo demais fluxos funcionais legados.

## Decision 6: Topologia Hub-and-Spoke e bridging via Relayer
- **Decision**: Adotar topologia Hub-and-Spoke com settlement centralizado no Hub e bridging por Lock&Mint / Burn&Unlock mediado pelo Relayer (Hyperledger Cacti); o AMM reside exclusivamente no Hub e opera sobre ativos espelhados (`W-tCeBM*`).
- **Rationale**: Concentrar a troca algoritmica num ledger neutro elimina o acoplamento bilateral entre Spokes e habilita o modelo AMM com pools compartilhados; o padrao Lock&Mint preserva soberania domestica dos ativos originais enquanto permite troca no Hub.
- **Alternatives considered**:
  - AMM replicado em cada Spoke com sincronizacao best-effort (rejeitado: consistencia fraca, risco de arbitragem).
  - Settlement bilateral P2P sem Hub (rejeitado: reintroduz dinamica de Cenario A, fora do escopo).
  - Bridging sincrono com custodia no Hub (rejeitado: conflita com soberania de CBDC e governanca dos Bancos Centrais).

## Decision 7: Modelo Exact-Output com maxAmountIn
- **Decision**: Padronizar o modelo Exact-Output nas APIs e contratos de swap; exigir parametro obrigatorio `maxAmountIn` para transferir o risco de cambio e slippage ao pagador.
- **Rationale**: Garante previsibilidade do valor recebido pelo beneficiario (obrigacao fixa), mantem a protecao do pagador contra volatilidade, e alinha o backend ao comportamento do contrato AMM on-chain.
- **Alternatives considered**:
  - Exact-Input (rejeitado: transfere incerteza ao beneficiario).
  - Swap sem limite superior de entrada (rejeitado: expoe o pagador a slippage irrestrito).
  - Reserva previa de preco com cotacao estatica (rejeitado: incompativel com produto constante x*y=k).

## Decision 8: Compliance por ZK-Pointers + Master Viewing Key
- **Decision**: Compliance em dois niveis: (i) `ZK-Pointer` por transacao, verificado pelo AMM para atestar "fit to transact" sem expor dados de KYC/AML no ledger do Hub; (ii) Master Viewing Key (Paladin) operada pelo Banco Central via governanca multi-assinatura para desanonimizacao em investigacoes AML/CFT.
- **Rationale**: Preserva confidencialidade no ledger internacional e atende simultaneamente requisito de supervisao institucional sem vazar dado sensivel por padrao.
- **Alternatives considered**:
  - KYC on-chain em claro (rejeitado: viola confidencialidade e LGPD-like).
  - Off-chain attestations assinadas sem prova ZK (rejeitado: dificulta verificacao no contrato AMM).
  - Apenas Master Viewing Key sem validacao por transacao (rejeitado: remove enforcement de compliance no momento do swap).

## Decision 9: Liquidity Monitor com threshold 70/30 (REQ-FX-008)
- **Decision**: Implementar `Liquidity Monitor` como microservico de backend que consulta/subscreve eventos do AMM, calcula razao entre reservas e emite alertas quando o imbalance ultrapassa 70/30; alertas sao enderecados a governanca/observabilidade.
- **Rationale**: Permite resposta operacional antes que a curva do AMM penalize excessivamente os swaps e gera trilha auditavel do estado do pool.
- **Alternatives considered**:
  - Monitoramento embutido na UI (rejeitado: nao garante cobertura continua nem persistencia).
  - Logica de alerta on-chain (rejeitado: custo de execucao e inflexibilidade para ajustar thresholds operacionais).
  - Observabilidade generica sem threshold dedicado (rejeitado: nao atende REQ-FX-008).

## Decision 10: Governanca do Circuit Breaker (modelo assimetrico)
- **Decision**: Expor acoes `pause`/`resume` do Circuit Breaker no backend com autorizacao **assimetrica**: `pause` exige 1-of-N assinaturas de Bancos Centrais autorizados (fail-safe, qualquer supervisor pausa); `resume` exige quorum 2-of-N assinaturas distintas. Contrato `AutomatedMarketMaker.sol` agrega assinaturas on-chain e rejeita `resume` sem quorum, emitindo evento de disputa auditavel. Bloqueio se aplica somente a novos swaps — transacoes atomicas ja confirmadas preservam seu estado.
- **Rationale**: Em infra financeira critica, pause emergencial nao pode depender de coordenacao multi-assinatura (janela de ataque e curta); resume, por outro lado, exige consenso minimo para impedir liberacao unilateral apos incidente. Padrao analogo em Fedwire e TARGET2.
- **Alternatives considered**:
  - Simetrico 1-of-N para ambos (rejeitado: risco de guerra de resume/pause).
  - Simetrico 2-of-3 para ambos, alinhado a Master Viewing Key (rejeitado: adiciona latencia em emergencia).
  - Hierarquico com BC-host do Hub com poderes diferentes (rejeitado: assimetria politica sensivel).

## Decision 11: Reconciliacao de falha assimetrica em bridging/unbridging
- **Decision**: Relayer Cacti executa ate **5 tentativas idempotentes** com backoff exponencial alvo (2s, 4s, 8s, 16s, 32s; cap de 60s). Fila de trabalho persistida em Postgres (tabela `relayer_queue_item` com estados `PENDING`, `IN_FLIGHT`, `FAILED`, `ESCALATED`). Cada item carrega identificador de idempotencia derivado do evento origem (hash de trava/burn) para impedir efeitos on-chain duplicados mesmo em retry sobreposto. Apos exaustao, a posicao afetada transita para `RECONCILIATION_REQUIRED` e e escalada para operador via alerta (sem reversao automatica on-chain).
- **Rationale**: Padrao operacional de relayers em producao (similar ao LayerZero Executor). 5 tentativas cobrem falhas transientes tipicas; persistencia em Postgres reaproveita a camada `DATASTORE` existente e garante sobrevivencia a restart do Relayer; ausencia de compensacao on-chain preserva invariantes contabeis do Hub.
- **Alternatives considered**:
  - Rollback atomico on-chain (rejeitado: complexidade elevada de contrato; risco de double-spend em edge cases).
  - Intervencao manual imediata sem retry (rejeitado: exige plantao 24/7 e alonga SLA).
  - Time-lock + refund (rejeitado: cria caminho de saida que complica auditoria e exige janela fixa).
  - Fila em Redis Streams (rejeitado: introduz dependencia operacional adicional; Postgres e suficiente).

## Decision 12: Master Viewing Key multi-sig (2-of-3 com timeout 72h)
- **Decision**: Disclosure do Master Viewing Key exige quorum **fixo 2-of-3** entre Bancos Centrais participantes, com timeout de **72h** para atingir o quorum; expiracao automatica da solicitacao apos o prazo. Assinaturas parciais ficam registradas em audit log enquanto a solicitacao esta aberta.
- **Rationale**: Alinha com padrao BIS para cooperacao AML/CFT multi-jurisdicao; 72h cobre prazos tipicos de Unidades de Inteligencia Financeira; 2-of-3 equilibra agilidade investigativa e controle institucional.
- **Alternatives considered**:
  - Unanimidade N-of-N com timeout 7 dias (rejeitado: baixa agilidade investigativa).
  - M-of-N configuravel por ambiente (rejeitado: complexidade de governanca de limiares).
  - 1-of-N emergencial com ratificacao posterior (rejeitado: aumenta risco de abuso).

## Decision 13: Performance targets quantificados
- **Decision**: API v2 do Cenario B observa os seguintes alvos em ambiente de homologacao representativo: `GET /api/v2/amm/quote/exact-output` p95 <= 300ms; `POST /api/v2/amm/swap/exact-output` p95 <= 6s (incluindo confirmacao on-chain em 1 bloco do Hub Besu); cadencia do Liquidity Monitor p95 <= 15s entre leituras. Baseline medido ao final de T106; desvios > 20% sinalizados como risco para cutover.
- **Rationale**: Valores realistas para Besu local com bloco de ~2s e overhead de ZK verification + bridging; deixam margem para detectar regressoes sem engessar a implementacao.
- **Alternatives considered**:
  - Alvos relaxados (quote 800ms, swap 15s, monitor 60s) rejeitados por mascararem regressoes.
  - Alvos agressivos (quote 150ms, swap 3s, monitor 5s) rejeitados por exigirem otimizacao antes de baseline.
  - Nao declarar SC para performance rejeitado para manter T106 com criterio objetivo.

## Decision 14: Retencao de dados operacionais (indefinida)
- **Decision**: Retencao indefinida para TODAS as entidades operacionais do Cenario B (`SwapOrderScenarioB`, `BridgedAssetPosition`, `ComplianceZKPointer`, `DisclosureRequest`, audit logs, eventos do Liquidity Monitor, leituras de pool state). Nenhum job de purga/TTL e implementado nesta feature. Absorcao de crescimento perpetuo ocorre via **particionamento por tempo** (mensal ou anual) nas tabelas de alta cardinalidade.
- **Rationale**: Escolha explicita do usuario (opcao A da sessao 2 de clarificacao) priorizando defesa regulatoria maxima. Particionamento isola custos de I/O por janela e mantem performance de escrita mesmo com crescimento irrestrito.
- **Alternatives considered**:
  - Retencao segmentada (7 anos AML/CFT + 2 anos operacional) rejeitada.
  - Retencao uniforme de 5 anos rejeitada.
  - Retencao configuravel por jurisdicao rejeitada por complexidade de governanca.

## Decision 15: Imutabilidade de audit logs via Postgres append-only
- **Decision**: Tabelas classificadas como `audit_log` (acionamentos de Circuit Breaker, disclosure lifecycle, transicoes de `BridgedAssetPosition`, alertas do Liquidity Monitor) sao implementadas append-only no Postgres por meio de triggers `BEFORE UPDATE` e `BEFORE DELETE` que levantam excecao. Sem replicacao WORM e sem anchor Merkle on-chain nesta feature.
- **Rationale**: Escolha explicita do usuario (opcao A da sessao 2). Protege contra alteracao casual ou bug aplicacional a custo operacional minimo. Hardening contra privilegio de sistema (DBA, superuser) e delegado a feature futura de producao.
- **Alternatives considered**:
  - Append-only + replicacao WORM em S3 Object Lock rejeitada (dependencia externa adicional).
  - Append-only + anchor Merkle on-chain rejeitada (custo de gas e complexidade de job de anchoring).
  - Eventos auditaveis diretamente on-chain rejeitada (pressao no Hub e atraso de hot path).

## Decision 16: Observabilidade estruturada delegada
- **Decision**: Observabilidade estruturada (logs JSON, metricas Prometheus, traces OpenTelemetry) **NAO e entregue** nesta feature. Servicos emitem logs ad-hoc em stdout, suficientes para triagem manual durante tryout E2E. SCs que dependem de observabilidade estruturada (SC-014 alerta de imbalance, SC-019 alerta de reconciliacao, SC-023 cadencia do Liquidity Monitor) sao validados qualitativamente nesta iteracao e serao requalificados objetivamente quando a feature de observabilidade existir.
- **Rationale**: Escolha explicita do usuario (opcao D da sessao 2). Evita lock-in em stack prematura. Trade-off aceito: regressoes de cadencia podem passar despercebidas sem metricas objetivas.
- **Alternatives considered**:
  - Apenas logs JSON com correlation ID rejeitada (sem metricas e tracing, debugging multi-service fica manual).
  - Stack completa em hot paths rejeitada pelo usuario.
  - Stack completa em todos os services rejeitada por peso em tryout local.

## Decision 17: Rate limiting delegado
- **Decision**: A API v2 **NAO implementa rate limiting** nesta feature. Nenhum middleware de throttling e instalado no `api-gateway`. Protecao contra abuso e delegada a (a) autenticacao institucional via Keycloak e (b) segmentacao de rede.
- **Rationale**: Escolha explicita do usuario (opcao D da sessao 2). Evita complexidade operacional antes de feature dedicada a gateway/infra de producao. Risco operacional documentado na matriz de riscos do cutover (FR-054).
- **Alternatives considered**:
  - Rate limit global rejeitado por injusticia entre tenants.
  - Rate limit por tenant + familia de endpoint rejeitado pelo usuario.
  - Rate limit por tenant + usuario + endpoint individual rejeitado por complexidade.

## Decision 18: Frontend out-of-scope absoluto
- **Decision**: Nenhum arquivo sob `frontend/apps/*` e modificado por tasks desta feature. Apos o cutover da API v1, o frontend atual permanecera incompativel com a API v2 ate feature futura dedicada. Risco aceito, sem mitigacao.
- **Rationale**: Escolha explicita do usuario ("nao mexer no frontend"). Alinha com principio big-bang declarado. Evita retrabalho concorrente em UI que sera substituida em feature futura.
- **Alternatives considered**:
  - Adaptar frontend atual a API v2 rejeitado (escopo excessivo).
  - Congelar com pagina de manutencao rejeitado pelo usuario.
  - Manter frontend read-only contra API v2 rejeitado (viola big-bang).

## Decision 19: Seed de liquidez inicial como pre-condicao obrigatoria do tryout (2026-05-04)
- **Decision**: O tryout E2E MUST incluir funcao `step4b_seed_liquidity()` executada pelo Banco Central (central-bank-a) **antes** dos steps de US1/US2. Pool zerado ao inicio de US1 e cenario de erro de configuracao de ambiente, nao cenario de teste valido; invalidaria 100% dos cenarios de quote/swap com `INSUFFICIENT_POOL_LIQUIDITY`.
- **Rationale**: Confirmado pelo output real do tryout de 2026-05-04 onde pool zerado causou falha em toda US1/US2. Separar provisao de liquidez (responsabilidade do Banco Central como Liquidity Provider) dos steps de teste (responsabilidade dos CommBanks como Liquidity Takers) alinha com os papeis do spec e evita ambiguidade de quem provisiona e quando.
- **Alternatives considered**:
  - Incluir Add Liquidity dentro de step5_us1 (rejeitado: mistura responsabilidade de setup com cenario de teste de US1).
  - Manter como esta com provisao em US2 (rejeitado: US1 e pre-requisito de US2; sem liquidez US1 invalida a sequencia inteira).
  - Cast direto no contrato via `cast send` sem passar pela API (rejeitado: bypassaria o endpoint de API v2 que precisa ser validado).

## Decision 20: Timeout de polling LOCKING -> ACTIVE no tryout (2026-05-04)
- **Decision**: O tryout MUST aguardar a transicao `LOCKING -> ACTIVE` de `BridgedAssetPosition` via polling em `GET /api/v2/bridge/positions` com **timeout de 120 segundos** (intervalo de 5s, maximo de 24 tentativas). Esgotado o timeout, o step MUST falhar com mensagem de erro explicita.
- **Rationale**: O Relayer tem backoff de ate 32s por tentativa; 120s cobre 3 ciclos completos de tentativa/resposta sem tornar o tryout excessivamente lento. Confirmado pelo comportamento real: chamada prematura de burn-unlock retornou `active bridged position not found: record not found` porque a posicao ainda estava em `LOCKING`.
- **Alternatives considered**:
  - sleep fixo de 30s (rejeitado: muito curto em ambientes sobrecarregados; desperdicador em ambientes rapidos).
  - Timeout de 300s (rejeitado: torna o tryout lento demais para uso cotidiano).
  - Sem timeout, loop infinito (rejeitado: tryout pode travar indefinidamente em falha do Relayer).

## Decision 21: Chave de lookup para Remove Liquidity (2026-05-04)
- **Decision**: O endpoint `POST /api/v2/amm/liquidity/remove` usa `lp_id` (UUID gerado pelo backend no Add Liquidity) como campo obrigatorio de lookup da posicao; `lp_shares` hex on-chain NAO e aceito como chave de remocao.
- **Rationale**: `lp_id` e o identificador estavel e unico da `LiquidityPosition` no banco; `lp_shares` e um valor on-chain em hex que representa participacao fracionaria no pool e pode variar com rebalanceamentos. Usar `lp_id` e consistente com o padrao REST de recursos ja adotado em `BridgedAssetPosition` (que usa `position_id`). Confirmado pelo erro real: tryout enviava `lp_shares: "500"` (hardcoded) mas o Add Liquidity retornou `lp_shares: "0xac37b006..."` — valores incompativeis.
- **Alternatives considered**:
  - lp_shares hex como chave de lookup (rejeitado: ambiguidade de quantidade parcial vs posicao completa; valor pode mudar com rebalanceamentos).
  - Dupla verificacao: lp_id para lookup + lp_shares para validacao on-chain (rejeitado: complexidade desnecessaria para esta feature; lp_id e suficiente).

## Decision 22: Resume automatico ao atingir quorum em resume-sign (2026-05-04)
- **Decision**: O endpoint `POST /api/v2/governance/circuit-breaker/resume-sign` executa a transicao `HALTED -> LIVE` automaticamente quando detecta que o quorum 2-of-N foi atingido, retornando `state: "LIVE"` na mesma resposta. Nenhum endpoint `executeResume` separado e exposto na API v2.
- **Rationale**: Comportamento confirmado no output real do tryout (`resume-sign` com 2a assinatura retornou `state: "LIVE"` diretamente). Elimina round-trip adicional sem valor para o cliente e evita inconsistencia entre estado interno (quorum atingido) e estado exposto (HALTED, aguardando chamada explicitamente).
- **Alternatives considered**:
  - executeResume separado (rejeitado: output real do tryout contradiz esse modelo; adiciona round-trip sem valor).
  - Configuravel via `auto_execute: bool` no body (rejeitado: complexidade desnecessaria; comportamento canonico e sempre automatico).

## Decision 23: Schema plano do endpoint de pool status (2026-05-04)
- **Decision**: `GET /api/v2/amm/pool/{pair}/status` retorna body plano (nao aninhado) com os campos canonicos: `reserve_a` (string), `reserve_b` (string), `current_ratio` (number), `imbalance_flag` (bool), `pool_pair` (string), `updated_at` (RFC3339). Nenhum objeto `reserves` aninhado deve existir no schema.
- **Rationale**: Confirmado pelo response real do tryout: `{"current_ratio":0,"imbalance_flag":false,"pool_pair":"BRL-USD","reserve_a":"0","reserve_b":"0","updated_at":"..."}`. O selector jq `.reserves` no `step4_pool()` falhava silenciosamente com `(warn)` porque o campo `reserves` nao existe. Campos planos sao mais simples de validar e serializar.
- **Alternatives considered**:
  - Objeto `reserves` aninhado com `reserves.a` e `reserves.b` (rejeitado: nao corresponde ao schema implementado; exigiria refactoring no backend sem ganho real).
  - Campo `pool_pair` como indicador de disponibilidade do endpoint no jq (alternativa valida mas `reserve_a` e mais especifico sobre o estado do pool).
