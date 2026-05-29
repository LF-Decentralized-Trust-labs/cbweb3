# Risk Register — Scenario B Cutover
# FR-054 / FR-047

## Accepted Risks

| Risk ID | Risk | Affected FRs/SCs | Mitigation | Owner |
|---------|------|------------------|------------|-------|
| RISK-001 | Sem rate limiting — API v2 exposta a abuso de requisicoes | FR-053 / FR-054 | Autenticacao Keycloak obrigatoria; segmentacao de rede; rate limiting delegado a feature futura | Engineering |
| RISK-002 | Sem observabilidade estruturada (Prometheus/OpenTelemetry) — diagnostico em producao depende de logs ad-hoc em stdout | FR-051 / FR-052 / SC-014 / SC-019 / SC-023 | Logs stdout ativados para todas as operacoes criticas; stack estruturada delegada a feature futura | Engineering |
| RISK-003 | Frontend incompativel apos cutover — frontend/apps/* nao modificado e nao consome API v2 | SC-025 | Risco aceito explicitamente pelo usuario; frontend rebuilding fora de escopo desta feature | Product |
| RISK-004 | Retencao indefinida sem purge — tabelas operacionais crescem ilimitadamente | FR-045 / FR-047 | Particionamento por tempo implementado em tabelas de alta cardinalidade; purge delegado a feature futura | DBA |
| RISK-005 | Hardening contra DBA — audit triggers sao contornados por acesso privilegiado de sistema | FR-050 | Triggers Postgres protegem contra alteracoes casuais; WORM e anchor on-chain rejeitados como superescopo | Security |
| RISK-006 | Reconciliacao manual necessaria apos exaustao do Relayer — posicoes em RECONCILIATION_REQUIRED requerem intervencao humana | FR-039 / SC-019 | Worker gera alerta visivel em stdout + LiquidityAlert persistido; equipe de operacoes alertada para monitorar | Operations |
