# Data Model — TK-B1 (ParticipantDeployment)

Fase 1. A entidade do manifesto, campos, regras por modo e regras de validação.

## Entidade: ParticipantDeployment

Documento YAML declarativo, um por participante/modo. **Nunca contém segredos.**

| Campo | Tipo | Obrigatório | Notas |
|---|---|---|---|
| `apiVersion` | string | sim | fixo `cbweb3b/v1` |
| `kind` | string | sim | fixo `ParticipantDeployment` |
| `metadata.name` | string | sim | id lógico do participante |
| `spec.scenario` | string | sim | fixo `"b"` |
| `spec.environment` | enum | sim | só `local` nesta fase (`staging`/`prod` rejeitados) |
| `spec.mode` | enum | sim | `found-hub` \| `found-spoke` \| `join` |
| `spec.topology.role` | enum | sim | `hub` \| `central-bank` \| `commercial-bank` |
| `spec.displayName` | string | não | cosmético |
| `spec.hub` | objeto | **found-hub** | `{ chainId:int, currency:[string] }` |
| `spec.spoke` | objeto | **found-spoke/join** | `{ id:string, chainId:int, currency:string }` |
| `spec.hubBundleRef` | path | **found-spoke** | consome o hub bundle |
| `spec.joinBundleRef` | path | **join** | consome o spoke bundle |
| `spec.bankId` | string | **join** | alimenta CSR CN / IdentityRegistry / BANK_ID |
| `spec.pair` | objeto | não (found-spoke) | `{ proposerCB, confirmerCB, symbolA, symbolB }` |
| `spec.node` | objeto | sim | `{ advertisedHost, rpc.port, ws.port, p2p.port, dataDir, validator? }` |
| `spec.image` | string | sim | `build` ou ref de registry |
| `spec.keyProvider` | uri | sim | `kms://…` |
| `spec.certSource` | uri | sim | `self-signed` \| `self-signed://…` \| `ca://…` |
| `spec.relay` | objeto | sim | `{ endpoint, advertisedHost? }` |
| `spec.cbEndpoint` | url | não (found-spoke) | gateway do CB p/ credential-request |
| `spec.frontendHost` | string | sim | host das apps |
| `spec.adminUsers[]` | lista | sim | `{ role, username, password }` — senhas só bootstrap `local` |

## Matriz por modo (obrigatório / proibido)

| Campo | found-hub | found-spoke | join |
|---|---|---|---|
| `hub{}` | **obrigatório** | proibido | proibido |
| `spoke{}` | proibido | **obrigatório** | **obrigatório** |
| `hubBundleRef` | proibido | **obrigatório** | proibido |
| `joinBundleRef` | proibido | proibido | **obrigatório** |
| `bankId` | proibido | proibido | **obrigatório** |
| `pair{}` | proibido | opcional | proibido |
| `cbEndpoint` | opcional | opcional | proibido |

## Regras de validação

**Estruturais (espelhadas no JSON-Schema):**
- `apiVersion`/`kind` fixos; `scenario == "b"`; enums de `mode`/`role`/`environment`.
- Presença/ausência de campos conforme a matriz por modo.
- `node.*.port` inteiros válidos; `advertisedHost` não-vazio.
- `keyProvider` casa `kms://…`; `certSource` casa `self-signed`/`self-signed://…`/`ca://…`.

**Semânticas (só em Go):**
- `environment == local` (rejeita `staging`/`prod`).
- **Sem segredos**: rejeitar qualquer valor contendo `-----BEGIN … PRIVATE KEY-----` ou chave
  privada hex (padrão de 64 hex) em qualquer campo.
- `pair`: `proposerCB != confirmerCB` e `symbolA != symbolB`.
- **Colisão (validação de conjunto, network-aware):** `chainId` colide só entre **redes
  distintas** (um `join` compartilha o `chainId`/`spoke.id` do founder — não é colisão);
  `spoke.id` colide só se **fundado por 2+** manifestos `found-spoke`; **portas de nó declaradas**
  (`rpc/ws/p2p`) globalmente únicas; `metadata.name` único. (Bandas de porta **derivadas por
  offset** ficam para fase futura — não são checadas em TK-B1.)

**Warnings (não bloqueiam):**
- `join` com `node.validator: true` → warning (modelo é full node não-validador; FR-013).

## Resultado da validação

- Estrutura de resultado com **lista de erros** (campo/recurso + mensagem) e **lista de
  warnings**, **coletadas** (não para na primeira — FR-011).
- Exit/param: válido ⇒ sem erros (pode ter warnings); inválido ⇒ ≥1 erro.
- Serializável em JSON e YAML (`-o`).

## Transições de estado

Nenhuma — validação stateless (sem persistência nesta fase).
