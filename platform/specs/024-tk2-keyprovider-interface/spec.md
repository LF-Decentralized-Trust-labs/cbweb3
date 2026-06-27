# Feature Specification: TK-2 — Interface keyProvider

**Feature Branch**: `024-tk2-keyprovider-interface`  
**Created**: 2026-06-27  
**Status**: Draft  
**Input**: User description: "TK-2 — CRIAR: interface keyProvider para o provisioning toolkit do Scenario A (Fase 1B)"

## User Scenarios & Testing *(mandatory)*

### User Story 1 — Provisionar participante sem chaves em arquivos (Priority: P1)

Um operador executa o toolkit para criar um novo participante no Scenario A. Nenhum material de chave privada aparece em arquivos, variáveis de ambiente, manifestos ou logs — nem mesmo durante a execução. O toolkit solicita que o provedor de chaves gere e armazene a chave internamente, e recebe de volta apenas a chave pública (endereço EVM) e assinaturas pontuais.

**Why this priority**: É o requisito de segurança central que desbloqueia todo o restante do toolkit. Se chaves privadas vazam para arquivos ou manifestos, o participante não pode ser auditado nem implantado em produção.

**Independent Test**: Pode ser testado executando `GenerateKey` e `Sign` via a interface e verificando que (a) nenhum arquivo de chave privada é criado, (b) o retorno de `GenerateKey` é a chave pública, e (c) a assinatura pode ser verificada com a chave pública correspondente.

**Acceptance Scenarios**:

1. **Given** um manifesto com `keyProvider: kms://local-emulator`, **When** o toolkit provisiona um novo participante, **Then** a chave privada nunca aparece em arquivo, log, variável de ambiente ou manifesto — apenas o endereço EVM derivado da chave pública.
2. **Given** o toolkit solicita uma assinatura para uma operação on-chain, **When** o provedor de chaves assina o payload, **Then** a assinatura retornada é verificável pela chave pública do participante sem que a chave privada tenha sido exposta.
3. **Given** o toolkit solicita `GenerateKey` para um mesmo ID duas vezes, **When** a segunda chamada ocorre, **Then** o provedor retorna a mesma chave pública sem criar uma nova chave (comportamento idempotente).

---

### User Story 2 — Uso local sem dependência de serviço externo (Priority: P2)

Um operador executando o toolkit em ambiente local (desenvolvimento, CI) consegue provisionar participantes sem necessidade de um KMS externo instalado. O toolkit usa um emulador local, em memória, que mantém as chaves durante a sessão sem gravá-las em disco.

**Why this priority**: Viabiliza o desenvolvimento e testes locais do toolkit de forma isolada, sem infraestrutura de KMS.

**Independent Test**: Pode ser testado executando o toolkit localmente com `keyProvider: kms://local-emulator` e confirmando que nenhum processo externo é necessário e que a chave gerada produz uma assinatura válida.

**Acceptance Scenarios**:

1. **Given** um manifesto com `keyProvider: kms://local-emulator`, **When** o toolkit inicia, **Then** o emulador local é instanciado sem conexões de rede e sem criar arquivos de chave.
2. **Given** o emulador local em execução, **When** o operador chama `GetPublicKey` para um ID inexistente, **Then** o sistema retorna um erro claro (`key not found`) sem criar uma chave silenciosamente.
3. **Given** o emulador local com uma chave já gerada, **When** o toolkit é encerrado e reiniciado, **Then** a chave não persiste (cada sessão começa com keystore vazio).

---

### User Story 3 — Extensibilidade para KMS de produção sem alterar o motor (Priority: P3)

Um operador que migra para produção consegue trocar o provedor de chaves apenas alterando o manifesto. O motor de orquestração não precisa de nenhuma modificação de código; a interface permanece a mesma.

**Why this priority**: Garante que a fase de produção (Fase 4 do roadmap) não quebre nenhuma lógica já testada — é o que torna o toolkit "mesmo toolset, local e prod".

**Independent Test**: Pode ser testado confirmando que o stub de produção implementa a mesma interface que o emulador local e que o factory instancia o tipo correto com base na URI do manifesto.

**Acceptance Scenarios**:

1. **Given** um manifesto com URI de KMS de produção, **When** o factory é invocado, **Then** o stub de produção é retornado — todas as operações resultam em erro informativo até ser implementado na Fase 4, sem pânico ou comportamento indefinido.
2. **Given** uma URI de keyProvider malformada (sem prefixo esperado), **When** o factory tenta instanciar o provedor, **Then** um erro descritivo é retornado antes que qualquer operação de chave ocorra.

---

### Edge Cases

- O que acontece quando `Sign` recebe um payload com tamanho diferente do esperado?
- O que acontece quando `GenerateKey` é chamado concorrentemente para o mesmo ID?
- O que acontece quando a URI de keyProvider está presente no manifesto mas é vazia ou malformada?
- Como o sistema se comporta se o emulador local fica sem memória durante uma sessão longa?

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: O sistema DEVE expor uma interface de gerenciamento de chaves com três operações: geração de chave, assinatura de payload, e recuperação de chave pública.
- **FR-002**: A interface DEVE garantir que chaves privadas nunca sejam retornadas a chamadores — somente chaves públicas e assinaturas são acessíveis externamente.
- **FR-003**: A implementação local DEVE gerar e armazenar chaves em memória, sem gravar nenhum material de chave em arquivo, variável de ambiente ou log.
- **FR-004**: A operação de geração de chave DEVE ser idempotente: chamar com o mesmo ID múltiplas vezes retorna a mesma chave pública sem criar duplicatas.
- **FR-005**: A operação de assinatura DEVE rejeitar payloads com tamanho incorreto, retornando um erro claro e descritivo.
- **FR-006**: A operação de assinatura e recuperação de chave pública DEVE retornar um erro descritivo (`key not found`) quando o ID solicitado não possui chave gerada.
- **FR-007**: O componente de factory DEVE instanciar o provedor correto com base na URI do campo `keyProvider` do manifesto.
- **FR-008**: O factory DEVE retornar um erro descritivo para URIs malformadas ou esquemas não reconhecidos.
- **FR-009**: O stub de produção DEVE implementar a mesma interface que o emulador local, retornando erro informativo em todas as operações até ser implementado na Fase 4.
- **FR-010**: Todas as operações da interface DEVEM suportar cancelamento via contexto de execução passado pelo chamador.

### Key Entities

- **KeyProvider**: Abstração que encapsula todo o ciclo de vida de chaves criptográficas do participante. Operações: geração, assinatura, recuperação de chave pública. Nunca expõe chave privada.
- **KeyProviderURI**: Referência ao backend de chaves no manifesto (ex: `kms://local-emulator`). Nunca contém material de chave; apenas identifica o provedor e seu endereço.
- **Digest**: Payload criptográfico de tamanho fixo a ser assinado. Responsabilidade do chamador garantir o conteúdo e tamanho corretos antes de passar para a operação de assinatura.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Um operador consegue provisionar um participante (geração de chave, assinatura de operações on-chain) sem que nenhum arquivo de chave privada seja criado em disco — verificável por inspeção do filesystem antes e depois da execução.
- **SC-002**: Todos os cenários de aceitação das User Stories são cobertos por testes automatizados que passam sem intervenção manual.
- **SC-003**: A troca de provedor (local → produção) exige apenas alteração de URI no manifesto, sem modificação de código no motor de orquestração — verificável por diff de código entre as duas configurações.
- **SC-004**: O emulador local responde a operações individuais de geração e assinatura em tempo imperceptível para o operador em ambiente de desenvolvimento.
- **SC-005**: O factory retorna um erro legível (sem stack trace técnico exposto) para URIs inválidas.

## Assumptions

- Chaves criptográficas utilizadas são do tipo compatível com endereços EVM — padrão do projeto para identidade on-chain no Scenario A.
- O emulador local não precisa de persistência entre sessões: cada execução do toolkit começa com keystore vazio (adequado para desenvolvimento e CI).
- O manifesto YAML já é validado antes de o factory ser invocado (TK-1 garante que `spec.keyProvider` é não vazio e presente).
- A implementação de produção é responsabilidade de uma fase futura (Fase 4); TK-2 entrega apenas o stub com contrato definido.
- O toolkit é executado em ambiente single-process; não há requisito de sincronização de estado de chaves entre instâncias distribuídas.
- O operador que aciona o toolkit tem permissão para criar chaves no backend configurado — controle de acesso ao KMS está fora do escopo desta feature.
