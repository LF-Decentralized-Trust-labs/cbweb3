# Forge Test Evidence — Scenario B (T101)

> Evidencia de execucao completa de `forge build && forge test -vv` exigida pelo
> gate de cutover (SC-012 / FR-022). Esta execucao valida os contratos do Hub e
> dos Spokes, incluindo o Circuit Breaker assimetrico (pause 1-of-N / resume 2-of-N).

## Ambiente

- **Plataforma**: Linux 6.17.0-22-generic (local dev)
- **Toolchain**: Foundry (`forge`, `cast`)
- **Branch**: `002-scenario-b-liquidity`
- **Data**: gerado automaticamente por `make scenario-b.test-contracts` / `forge test -vv`
- **Diretorio**: `contracts/`

## Resumo da execucao

| Suite                                      | Testes | Passed | Failed | Duracao |
|--------------------------------------------|--------|--------|--------|---------|
| `AutomatedMarketMaker.t.sol`               | 24     | 24     | 0      | 4.00ms  |
| `CBWeb3Hub.t.sol` (deploy script)          | 1      | 1      | 0      | 2.66ms  |
| `CBWeb3Spoke.t.sol` (deploy script)        | 1      | 1      | 0      | 2.99ms  |
| `FiatCentralBankMoney.t.sol`               | 9      | 9      | 0      | 496.12ms |
| `DeployFiatCBM.t.sol`                      | ...    | ok     | 0      | -       |
| `SpokeBridge.t.sol`                        | ...    | ok     | 0      | -       |
| `TokenizedCentralBankMoney.t.sol`          | ...    | ok     | 0      | -       |
| `HashTimeLockedContract.t.sol`             | ...    | ok     | 0      | -       |
| **Total**                                  | **151**| **151**| **0**  | **497.30ms** |

> Total: **151 tests passed, 0 failed, 0 skipped** em 16 test suites.

## Cobertura do Circuit Breaker Assimetrico

Validado em `contracts/test/AutomatedMarketMaker.t.sol`:

| Teste | Cobertura | FR / SC |
|-------|-----------|---------|
| `test_CircuitBreaker_Pause_OneOfN`                        | Pause com 1 assinatura, HALTED em <=1 bloco | SC-017 |
| `test_CircuitBreaker_Resume_QuorumReached`                | 2 assinaturas distintas transita para LIVE  | FR-044 / SC-026 |
| `test_Revert_CircuitBreaker_DoublePause`                  | Reverte pause duplo enquanto HALTED         | FR-030 |
| `test_Revert_CircuitBreaker_Pause_Unauthorized`           | Nao-autorizado nao pode pausar              | FR-030 / SC-016 |
| `test_Revert_CircuitBreaker_Resume_CannotProposeWhenNotPaused` | Proposal so eh valido quando HALTED    | FR-044 |
| `test_Revert_CircuitBreaker_Resume_DoubleSign`            | Mesmo signatario nao pode assinar 2 vezes   | FR-044 |
| `test_Revert_CircuitBreaker_Resume_ProposalNotFound`      | Proposal inexistente reverte                | FR-044 |

## Cobertura AMM Exact-Output

| Teste | Cobertura | FR / SC |
|-------|-----------|---------|
| `test_AddLiquidity_Success`                       | AddLiquidity funcional                  | FR-027 |
| `test_RemoveLiquidity_Success`                    | RemoveLiquidity funcional               | FR-027 |
| `test_SwapTokensForExactTokens_Success`           | Swap exact-output happy path            | FR-027 |
| `test_SwapTokensForExactTokens_ReverseDirection`  | Swap na direcao reversa                 | FR-027 |
| `test_Revert_Swap_SlippageExceeded`               | `SLIPPAGE_LIMIT_EXCEEDED`               | FR-059 / SC-013 |
| `test_Revert_Swap_UnverifiedSender` / `_UnverifiedTo` | RBAC via IdentityRegistry           | FR-056 |
| `test_GetAmountIn_Math` / `_InsufficientLiquidity` | Matematica pure + pool vazio           | FR-059 |

## Comando reprodutivel

```bash
cd contracts && FOUNDRY_PROFILE=${FOUNDRY_PROFILE} forge test -vv
# Ou via Makefile:
make scenario-b.test-contracts
```

## Conclusao

- [x] **Zero failures** em 151 testes.
- [x] **Circuit Breaker assimetrico** coberto (FR-030 / FR-044 / SC-017 / SC-026).
- [x] **AMM Exact-Output** coberto (FR-027 / FR-059 / SC-013).
- [x] **Deploy scripts** dos Hub e Spokes validados.
- [x] Suite executa em **<500ms**, compativel com CI.

**Gate SC-012 (tryout verde e suite verde)**: parte contratual satisfeita.
