# FXAgreementLibrary
[Git Source](https://github.com/LACNetNetworks/cbweb3-platform/blob/046ef8ae4f22ae07ddd7d371613d585961088c4e/src/libraries/FXAgreementLibrary.sol)

**Title:**
FXAgreementLibrary

Shared data structures for the on-chain bilateral FX agreement lifecycle.

Centralizes enums and structs to ensure byte-code consistency across the FX module.


## Structs
### FxAgreement
Immutable record of a bilateral FX deal.


```solidity
struct FxAgreement {
    bytes32 tradeId;
    address counterpartyA;
    address counterpartyB;
    address baseToken;
    address quoteToken;
    uint256 notional;
    uint256 rate;
    uint256 expiryDate;
    AgreementState state;
}
```

**Properties**

|Name|Type|Description|
|----|----|-----------|
|`tradeId`|`bytes32`|Unique identifier for the deal.|
|`counterpartyA`|`address`|Address of the deal initiator.|
|`counterpartyB`|`address`|Address of the accepting party.|
|`baseToken`|`address`|Token address of the base currency.|
|`quoteToken`|`address`|Token address of the quote currency.|
|`notional`|`uint256`|Notional amount of the base currency.|
|`rate`|`uint256`|Agreed exchange rate scaled by 1e18.|
|`expiryDate`|`uint256`|Unix timestamp after which the proposal expires.|
|`state`|`AgreementState`|Current lifecycle state of the agreement.|

## Enums
### AgreementState
Represents the finite state machine of an FX agreement.


```solidity
enum AgreementState {
    INVALID,
    PROPOSED,
    ACCEPTED,
    SETTLED,
    REJECTED,
    CANCELLED
}
```

