# Contrato: Extensão do Join Bundle (TK-6 → TK-9)

**Feature**: `032-commercial-bank-join`
**Arquivo afetado**: `scenario-a/toolkit/engine/bundle/types.go`

## Mudanças no Schema YAML

### Adições a `BundleSpec`

```yaml
# Campos adicionados ao spec do bundle:

validators:            # NOVO — obrigatório para mode:join
  - address: "0xABCD..."   # Ethereum address do validador
    rpcUrl: "http://cb-besu:8545"  # JSON-RPC endpoint

cbEndpoint: "http://api-gateway-cb:8080/api/v1/credential-request"  # NOVO
```

### Exemplo de bundle completo (após extensão)

```yaml
apiVersion: cbweb3/v1
kind: JoinBundle
metadata:
  name: spoke-brl
  generatedAt: "2026-06-27T10:00:00Z"
spec:
  spokeId: spoke-brl
  chainId: 1337
  currency: BRL
  bootnode:
    enode: "enode://abc123...@cbweb3-spoke-brl-besu.central-bank-brazil:31303"
    advertisedHost: cbweb3-spoke-brl-besu.central-bank-brazil
    p2pPort: 31303
  genesis:
    hash: "sha256:abcdef1234..."
    content: "eyJjb25maWci...base64..."
  contracts:
    registryAddress: "0x1234..."
    zetoFactoryAddress: "0x2345..."
    penteFactoryAddress: "0x3456..."
    zetoTokenAddress: "0x4567..."
    penteContextGroupId: "group-brl-01"
    penteContextAddress: "0x5678..."
    fxAgreementAddress: "0x6789..."
  relay:
    endpoint: "http://cacti-relay:4000"
  trust:
    caCertPEM: |
      -----BEGIN CERTIFICATE-----
      ...
      -----END CERTIFICATE-----
  validators:                                    # NOVO
    - address: "0xCB1A..."
      rpcUrl: "http://cbweb3-spoke-brl-besu.central-bank-brazil:8645"
  cbEndpoint: "http://api-gateway-cb:8080/api/v1/credential-request"  # NOVO
```

## Validação

O `bundle.EmitBundle()` (TK-6) deve:
1. Popular `Validators` a partir dos validadores QBFT ativos no momento da emissão (via `qbft_getValidatorsByBlockNumber("latest")` + mapeamento address→rpcUrl do manifest CB)
2. Popular `CBEndpoint` a partir do `manifest.Spec.Relay.Endpoint` ou de uma nova config `spec.cbEndpoint` no manifesto do CB

O `bundle.LoadBundle()` (consumido por TK-9) deve:
1. Rejeitar bundles onde `validators` é vazio com erro: `"bundle: validators list is empty — cannot execute mode:join"`
2. Rejeitar bundles onde `cbEndpoint` é vazio com erro: `"bundle: cbEndpoint is required for mode:join"`

## Retrocompatibilidade

Bundles existentes gerados por versões anteriores do TK-6 não contêm `validators` ou `cbEndpoint`. Eles continuam válidos para fins de documentação/arquivo, mas não podem ser usados com `mode: join` (falham na validação acima com mensagem clara). O TK-6 deve ser atualizado neste mesmo PR para emitir os novos campos.
