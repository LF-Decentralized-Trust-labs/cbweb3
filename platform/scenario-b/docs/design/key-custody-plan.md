# Plano — custódia de chaves e autenticação entre serviços (Cenário B)

**Data**: 2026-07-31 · **Escopo**: apenas `scenario-b/` · **Status**: proposta

## 1. O problema

Nenhuma chave privada do deployment é secreta hoje.

| Chave | Origem | Consequência |
|---|---|---|
| `CB_PRIVATE_KEY` | conta de desenvolvimento conhecida do Besu, publicada no repositório, **idêntica em todas as entidades** | qualquer entidade assina como qualquer outra no spoke |
| `HUB_SIGNER_PRIVATE_KEY` | `keccak256(salt + spokeID)` | salt é constante em `toolkit/engine/orchestrator/localdev.go`, spokeID está no manifesto — ambos públicos |
| `HUB_RELAYER_PRIVATE_KEY` | `keccak256(outro salt + spokeID)` | idem |

No modelo on-chain, quem detém a chave **é** o banco central: `registerCurrency` exige
`msg.sender == getCentralBankOf(token)` e `CENTRAL_BANK_ROLE` autoriza emissão. Logo isto não é
"criptografia fraca", é ausência de fronteira criptográfica — nenhum controle de aplicação substitui.

A autenticação entre serviços tem o mesmo caráter: os endpoints `/internal/...` aceitam um segredo
simétrico igual em todas as entidades. Já existe mecanismo melhor implementado
(`internal/relayauth`, assinatura por requisição, com pin de certificado), **desligado por default**.

Existe ainda uma armadilha de configuração: o manifesto declara `spec.keyProvider`, o schema
**valida** o campo, e **nada o lê** — as únicas referências a `.KeyProvider` estão na validação.
Um manifesto com `kms://aws-prod` valida com sucesso e o deployment usa chaves derivadas de
desenvolvimento em silêncio.

**Situação aceita:** para pesquisa, laboratório e demonstração isto é adequado. O teto é colocar
valor real ou terceiros no ambiente.

## 2. Fora de escopo

- **Cenário A.** Tem toolkit próprio com o mesmo padrão. Não será tocado.
- **mTLS.** Trabalho de outra branch e de outra camada: autentica a *conexão* gRPC intra-entidade,
  não a *requisição* entre entidades. Aqui há apenas coordenação de merge (ver §7).
- **PKI e CA de produção.** Adiadas. Ficam registradas em §6 as decisões que elas destravam.
- **KMS real.** A implementação fica na Fase 7, que só começa com as decisões D1–D3. As fases 1 a 6
  não dependem dele.
- **Travamento do relayer com RPC do hub pendurado.** Achado desta investigação, já documentado na
  seção de retentativas do runbook. Pertence ao executor do relayer.

## 3. Fases

Cada fase é entregável e verificável isoladamente.

### Fase 1 — Fail-safe e honestidade da configuração · **IMPLEMENTADA**

*Nenhuma decisão pendente. Faria primeiro.*

**a)** O gateway detecta na inicialização a combinação `RELAY_REQUIRE_SIGNATURE=true` com registro de
chaves vazio, e reporta de forma inequívoca em vez de servir tráfego.

> Por quê: em `RequireRelayAuthMigrating`, com `hasRegistry` falso o middleware nunca tenta
> verificar e cai no ramo "sem assinatura verificável", devolvendo **401 para tudo** — inclusive para
> uma requisição corretamente assinada. Como o `PKI_DIR` do gateway do CB está vazio hoje, ligar o
> booleano derrubaria bridge-in, swap no hub e devolução de resíduo. Ou seja: pagamentos param.

**b)** Um `spec.keyProvider` não resolvível falha alto no toolkit, em vez de ser ignorado.

**Verificação**: teste para cada caso, incluindo um que garanta que a combinação perigosa não passa
em silêncio.

### Fase 2 — Costura do KeyProvider no toolkit · **IMPLEMENTADA**

*Nenhuma decisão pendente.*

Honrar `spec.keyProvider` e rotear a derivação atual **através** do provedor, trocando
`deriveCBHubKey`, `deriveCBRelayerKey` e `deriveBesuAccountHex` por `GenerateKey`/`GetPublicKey`.

**Sem mudar nenhum endereço.** As duas derivações são o mesmo cálculo: a inline é
`keccak256(salt + id)` e a do provedor local é `keccak256(seed || id)`. Com `seed` igual ao salt
atual, a chave é idêntica. Custa uma instância de provedor por família de salt (são três: banco,
hub do CB, relayer do CB). Isso evita re-concessão de papéis on-chain e qualquer migração.

Enquanto o provedor for o local, o toolkit continua escrevendo hex no ambiente através do
`LocalKeyExporter` — é o que mantém o caminho local idêntico.

**Verificação**: teste que compara os endereços derivados via provedor com os valores atuais
literais (`spoke-brl` gateway `0xaE3467da888C4Af6F171993Fe9D183033FB71E74`, relayer
`0x9f4fafEEF19E4Dd2472E13d0E1869f9f228B14e2`, e os equivalentes de `spoke-ars`). Qualquer divergência
reprova.

### Fase 3 — Costura do Signer no backend

*Depende da decisão D5.*

`backend/shared/blockchain/scenariob/evm/evm.go` é a costura central: converte hex em chave
(`HexToECDSA`) e monta o transactor (`NewKeyedTransactorWithChainID`), servindo 12 consumidores.
Três clientes do payment-orchestrator repetem isso por conta própria
(`fx_agreement_client.go`, `fiat_token_client.go`, `fiat_client.go`). São 4 pontos, não código
espalhado.

Introduzir um `Signer` com duas implementações — local (hex, o caminho de hoje) e remota (pede
assinatura ao provedor) — e permitir que o ambiente receba **apenas o endereço**. Nesta fase entra
só a implementação local: nenhuma chamada a KMS, nenhuma mudança de comportamento.

**Fronteira estrutural que exige decisão:** `keyprovider` está no módulo
`scenario-b/toolkit`; `evm.go` está em `scenario-b/backend/shared/blockchain`. São módulos Go
distintos, então o backend **não pode importar** o pacote do toolkit. E os dois consumidores querem
coisas diferentes: o toolkit quer "crie a chave e me diga o endereço", uma vez; o backend quer
"assine este digest", a cada transação.

**Verificação**: as suítes atuais continuam passando (558 no gateway, 127 no orquestrador), mais um
teste de que o `Signer` local produz a mesma assinatura que o caminho anterior.

### Fase 4 — Separar administrar de emitir · **IMPLEMENTADA**

*Nenhuma decisão pendente.*

> **Correção de escopo, registrada após a implementação.** Este plano afirmava que a Fase 4 seria
> menor que a Fase 3 e recomendava fazê-la antes por isso. A estimativa estava errada: o caminho
> óbvio — passar os endereços novos por `RegisterCurrency` — cruza HTTP e **gRPC** até o compliance do
> hub, o que exigiria mudança de proto e `proto-gen`. O caminho adotado evita todas essas camadas:
> a separação acontece **depois** do handover, assinada pela chave do gateway, que o toolkit já
> possui e que naquele momento ainda administra o token. Nada em `RegisterCurrency`, no handler HTTP
> ou no proto foi alterado.

A chave do gateway acumula `DEFAULT_ADMIN_ROLE` (concede e revoga emissão) e `CENTRAL_BANK_ROLE`
(emite e queima). Separar em duas identidades, com a de governança usada raramente.

**Como ficou.** Três atos on-chain, todos assinados pela chave do gateway porque é ela que administra
no momento — e esse fato fixa a ordem, já que a revogação da própria administração desabilita o
assinante:

1. conceder `CENTRAL_BANK_ROLE` ao relayer (sai do boot do gateway, que depois não poderá mais conceder)
2. conceder `DEFAULT_ADMIN_ROLE` à identidade de administração, cuja chave não vai para container algum
3. revogar `DEFAULT_ADMIN_ROLE` do gateway — por último

O gateway **mantém** `CENTRAL_BANK_ROLE`: ele mina em `MintAndApproveForAMM` a cada provisionamento de
liquidez. Há teste asseverando que nenhum plano jamais revoga essa emissão.

Idempotência lida da cadeia, não de arquivo de estado — a cadeia é a autoridade sobre quem administra.
E o bootstrap de boot do gateway virou `verifyRelayerIssuanceRole`: verifica e reporta, com log que
diz para re-executar `apply` e que o gateway deliberadamente não pode consertar.

**Verificação**: unitária na função pura (oito combinações de estado, ordem e invariante de nunca
deixar o token sem administrador) e na execução (ordem dos `cast send`, idempotência, e o caso em que
a concessão falha e a revogação **não** é tentada). Ao vivo, `tmp/abc/verify-sovereign-hub-identity.sh`
deve ganhar asserção de que `DEFAULT_ADMIN_ROLE` e `CENTRAL_BANK_ROLE` respondem para endereços
**diferentes** — pendente.

**Ressalva honesta**: enquanto as chaves vierem de entradas públicas, esta separação é
**estrutural** — limita qual container faz o quê e fixa a topologia — e não fronteira de sigilo.
O valor é que, quando a Fase 7 entrar, muda apenas onde a chave mora.

### Fase 5 — relayauth: certificados e identidade única · **IMPLEMENTADA**

*Depende da decisão D4.*

Quatro bloqueios mecânicos, nenhum deles a CA:

1. **Distribuição.** O CB precisa ter o certificado de cada banco que atende, como `<bankCode>.crt`,
   no diretório dele. Hoje o certificado do banco vive no diretório do próprio banco, com nome
   `<bankCode>-participant.crt`. Nada copia para o CB. O compliance do CB é quem assina o CSR no
   onboarding, então o material existe no momento da emissão — falta persistir.
2. **`PKI_DIR` vazio no gateway do CB.** Sem isso não há registro nem assinador.
3. **Colisão de identidade.** Os dois bancos centrais usam `BANK_CODE=central-bank` (verificado ao
   vivo em Brasil e Argentina). Como a key-id é o `BANK_CODE`, o registro só consegue fixar uma
   chave por id.
4. **Convenção de nome** dos certificados (`<entidade>-participant.crt` versus o `<entidade>.crt`
   que o carregador espera).

O lado que **assina** já está pronto: os três relés (bridge-in, swap no hub, resíduo) aceitam um
`relayauth.Signer`, carregado de `PKI_DIR/<BANK_CODE>.key`.

Esta fase **não liga** a exigência de assinatura.

**Verificação**: o registro carrega N chaves; uma requisição assinada por um banco verifica no CB;
teste que reprova key-ids colidentes.

### Fase 6 — Cacti assina, então exigir assinatura · **IMPLEMENTADA**

*Depende da decisão D6.*

O relay assina com **identidade própria**, não repassando a do banco originador. Este plano
recomendava o repasse; a investigação mostrou que estava errado em dois pontos:

- **inviável como está** — o relay faz `JSON.stringify(payload)`, re-serializando o corpo, e a
  assinatura cobre exatamente esses bytes; uma assinatura repassada nunca verificaria;
- **desnecessário** — o handler de bridge-out do CB de destino **verifica o swap on-chain** pelo
  recibo (`LogSwap` da AMM do par, montantes lidos do evento, cada `swap_tx_hash` consumido uma vez),
  e o emissor daquele evento é o CB de origem. A atribuição do ato já é criptográfica na cadeia, então
  a credencial de transporte só precisa autenticar o **salto**.

A string canônica é o único ponto que precisa concordar byte a byte com o verificador em Go, e
divergência rejeita tudo — travada por teste em ambos os lados contra um vetor comum.

Enforcement ligada com quatro ajustes que a flag sozinha não cobria: recarga sob demanda para uma
key-id desconhecida (fecha a janela entre onboarding e a próxima varredura), `pin-relay-cert` como
dependência de `start-spoke-backend`, restart do relay após o `found-hub` que gera sua chave, e a flag
forçada vazia em `join` — sem isso, ligá-la para os CBs derrubaria todo gateway de banco.

**Verificação**: E2E com exigência ligada, provando também que o segredo compartilhado deixa de ser
aceito.

### Fase 7 — Provedor de produção (KMS ou HSM)

*Depende das decisões D1, D2 e D3.*

- Implementar `prod.go`, hoje um stub que recusa tudo.
- Implementar o `Signer` remoto da Fase 3.
- **Ponto de maior risco de erro sutil:** o KMS devolve assinatura em DER; a Ethereum precisa de
  `[R||S||V]` com S normalizado e V recuperado.
- Toolkit passa a escrever apenas endereços: `*_PRIVATE_KEY` dos templates viram `*_ADDRESS`.
- Cobrir os caminhos que hoje usam a conta de teste: deploy de contratos e a concessão de papel ao
  relayer no boot do gateway.
- Tornar o provedor local inalcançável fora de desenvolvimento; URI não configurado falha fechado.
- Procedimento de rotação documentado — o on-chain amarra o *endereço*, então rotacionar exige
  handover (mecânica da Fase 4).
- **Auditoria das chamadas de assinatura.** Na prática é o maior ganho da mudança e o menos citado:
  hoje não existe registro de quem pediu uma assinatura.

**Verificação**: estender `toolkit/engine/keyprovider/nosecrets_test.go` — que já garante que o
provedor de produção não expõe exportador de chave — com asserção de que nenhum `*_PRIVATE_KEY`
aparece nos compose renderizados em ambiente de produção.

## 4. Ordem recomendada

```
1 (fail-safe) → 2 (provider no toolkit) → 4 (separar papéis) → 3 (signer no backend) → 5 (certificados) → 6 (Cacti) → 7 (KMS)
```

As fases 1, 2 e 4 não dependem de nenhuma decisão aberta e podem começar imediatamente. A 4 vem
antes da 3 por ser menor e por já ter mecânica pronta.

## 5. Alternativas rejeitadas

| Alternativa | Por que foi rejeitada |
|---|---|
| Mudar o `BANK_CODE` dos CBs para dar unicidade à key-id | `BANK_CODE` alimenta `owner_bank_id` nas posições de bridge e a auto-exclusão da reconciliação. Exigiria migração de dados. Preferido: variável de key-id separada. |
| Aceitar endereços novos na Fase 2 | Orfanaria as concessões de papel on-chain e exigiria handover em toda stack existente. Preservar os endereços custa apenas três instâncias de provedor. |
| Backend importar `keyprovider` do toolkit | Impossível: módulos Go distintos. Requer mover a interface para um local compartilhado ou dar ao backend um cliente próprio (decisão D5). |
| Ligar `RELAY_REQUIRE_SIGNATURE` antes da Fase 6 | Devolve 401 nos endpoints atendidos pelo Cacti, que ainda não assina. |
| Corrigir aqui o travamento do relayer com RPC pendurado | Componente distinto (executor do relayer); ampliaria o conjunto de mudanças misturando dois assuntos. |
| Revogar `CENTRAL_BANK_ROLE` do relayer para testar falhas | O mesmo papel gateia o mint do bridge-in, então não pode ser revogado antes de um swap. |

## 6. Decisões abertas

| # | Decisão | Bloqueia |
|---|---|---|
| D1 | Qual KMS ou HSM | Fase 7 |
| D2 | Namespace de ids de chave no KMS | Fase 7 |
| D3 | Quais principals podem pedir assinatura por chave | Fase 7 |
| D4 | Como identificar cada banco central de forma única | Fase 5 |
| D5 | Onde vive a interface de assinatura (módulos distintos) | Fase 3 |
| D6 | O relay Cacti valida assinaturas ou apenas repassa | Fase 6 |

Ao definir o KMS, vale considerar que alguns autenticam o cliente por certificado — o que criaria
dependência da PKI que foi adiada. Um cujo acesso seja por IAM ou token evita amarrar as frentes.

Quando PKI e CA forem definidas, uma decisão adicional aparece: **o modelo de pin ignora
`NotAfter`**, então certificado expirado autentica. Num modelo de pin isso é defensável, mas se a CA
emitir certificados de vida curta contando que a expiração limite comprometimento, o pin anula esse
efeito em silêncio. Também não há caminho de revogação.

## 7. Riscos

- **Fase 3** é a de maior volume; **Fase 7** a de maior risco de erro sutil (formato de assinatura).
- **Fase 6** exige trabalho em TypeScript, fora da stack Go do restante.
- **Conflito de merge com a branch de mTLS**: ambas editam o bloco `environment` de
  `provisioning/templates/entity-backend.compose.yaml`.
- **Risco de leitura**: "mTLS está ligado" não implica "autenticação entre serviços resolvida". São
  hops distintos, e o caminho que atravessa o relay segue com segredo compartilhado até a Fase 6.
