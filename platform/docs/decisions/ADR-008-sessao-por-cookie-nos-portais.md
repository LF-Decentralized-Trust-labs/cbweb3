<!-- SPDX-License-Identifier: Apache-2.0 -->

# ADR-008 — Sessão por cookie HttpOnly nos portais

| | |
|---|---|
| **Finding** | R1-11.4 (decisão nomeada como "cookie auth", sem ADR) |
| **Status** | **Aceito e implementado** — registro retrospectivo, com uma lacuna aberta |

## Contexto

Os portais (governança, tesouraria, supervisor, banco, NOC) autenticam contra o
api-gateway e mantêm sessão por **cookie HttpOnly**, não por token em `localStorage`
com header `Authorization`.

Estado atual, idêntico nos dois cenários
(`scenario-a/backend/services/api-gateway/internal/http/handlers/auth.go:74-98` e o
arquivo equivalente do `scenario-b`):

| Cookie | Atributos |
|---|---|
| `access_token` | `HttpOnly`, `Secure` (configurável), `SameSite=Strict`, `Path=/`, `MaxAge = expiresIn` |
| `refresh_token` | idem, com `MaxAge` próprio (`refreshExpiresIn`), para permitir refresh silencioso |

Para chamador máquina-a-máquina, a v2 aceita **também** `Authorization: Bearer` nas rotas
soberanas de banco central — `RequireAnyAuth` —, enquanto a v1 documentava apenas cookie
([`../deliverables/D5-changelog-v1-to-v2.md`](../deliverables/D5-changelog-v1-to-v2.md)).

## O que estava em jogo

A escolha entre cookie e token em storage do navegador é um par de riscos, não uma
melhoria unilateral:

- **Token em `localStorage`** é legível por qualquer script na origem. Um XSS em qualquer
  ponto do portal entrega o token, e com ele a sessão inteira.
- **Cookie** não é legível por script quando `HttpOnly`, o que remove essa classe. Em
  troca, passa a ser enviado automaticamente pelo navegador — e é isso que abre CSRF.

Ou seja: trocar storage por cookie **troca** exposição a XSS por exposição a CSRF. Só é
ganho líquido se o CSRF for tratado.

## Opções que existiam

### Opção A — Token no `localStorage` com header `Authorization`
Imune a CSRF por construção (o navegador não anexa nada sozinho). Simples de implementar
em SPA.

Custo: qualquer XSS vira roubo de sessão. Em portal de banco central, é o risco menos
aceitável dos dois.

### Opção B — Cookie HttpOnly com `SameSite=Strict` (escolhida)
Remove a leitura por script. `SameSite=Strict` faz o navegador não anexar o cookie em
requisição originada de outro site, o que cobre a maior parte do CSRF clássico sem token
adicional.

Custo: depende do navegador respeitar `SameSite`, e não cobre todos os cenários que um
token anti-CSRF cobriria. Some-se que `Strict` afeta navegação vinda de link externo, o
que é aceitável em portal operacional mas seria hostil em produto de consumo.

### Opção C — Cookie HttpOnly **mais** token anti-CSRF por requisição
Defesa em profundidade: o padrão para aplicação que aceita cookie e muda estado.

Custo: exige middleware no gateway e cooperação de todos os frontends.

## Decisão

**Opção B**, com `SameSite=Strict` como a mitigação de CSRF, mais `Bearer` aceito nas
rotas máquina-a-máquina da v2.

## Consequências

**Positiva.** Nenhum token de sessão acessível a script no portal. Refresh silencioso
funciona sem que o SPA manipule token.

**Lacuna aberta, e ela é a razão de este ADR existir.** Não há defesa CSRF explícita além
do `SameSite` — nenhum middleware anti-CSRF no gateway dos dois cenários. Isso já estava
registrado em [`../scenario-drift.md`](../scenario-drift.md), que anota que nenhum dos
dois backends usa defesa CSRF, e o registrou como simetria entre cenários — o que é
verdade, e não é o mesmo que estar resolvido.

A leitura honesta: a Opção B **sem** a Opção C é defesa de uma camada. `SameSite=Strict`
é boa camada e provavelmente suficiente para portal de operador em rede fechada; não é o
que se defenderia para exposição pública. Qual dos dois é o alvo é decisão de stakeholder,
e é o gatilho de reabertura deste ADR.

**Sobre `Secure`.** O atributo é configurável (`cookieSecure`), o que existe para o
laboratório local em HTTP. Em qualquer ambiente alcançável por rede, ligado é obrigatório —
cookie de sessão sem `Secure` viaja em claro.

## Relação com outras decisões

O login que preenche esses cookies exige **usuário** Keycloak, não credencial de client,
por decisão registrada em `f55ade5b` (*require user (password grant) login for portals*).
Cookie e password grant são duas metades do mesmo desenho: sessão de **operador humano**.

## Status / Sign-off

| Parte | Decisão | Data |
|---|---|---|
| Time de plataforma | Implementado nos dois cenários | — |
| IDB / BID | A confirmar: se o alvo inclui exposição pública, a Opção C passa a ser exigida | — |
