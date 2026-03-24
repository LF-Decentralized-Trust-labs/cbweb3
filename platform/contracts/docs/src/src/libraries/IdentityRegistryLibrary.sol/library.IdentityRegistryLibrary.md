# IdentityRegistryLibrary
[Git Source](https://github.com/LACNetNetworks/cbweb3-platform/blob/e19c9456a3de8cd6d3345c4656e5c68bf8aefa97/src/libraries/IdentityRegistryLibrary.sol)

**Title:**
IdentityLib

Shared data structures and constants for the CBDC network identity management.

This library centralizes enums and structs to ensure byte-code consistency across the ecosystem.


## Structs
### Participant
Core entity representing an institutional identity on the ledger.


```solidity
struct Participant {
    string legalName;
    ParticipantRole role;
    KycStatus status;
    bytes32 zkPointer;
    uint256 lastUpdate;
}
```

**Properties**

|Name|Type|Description|
|----|----|-----------|
|`legalName`|`string`|Registered legal name of the institution.|
|`role`|`ParticipantRole`|Functional role (e.g., Commercial Bank) governing access rights.|
|`status`|`KycStatus`|Current KYC/AML verification state.|
|`zkPointer`|`bytes32`|Hash reference to private credentials handled by the privacy layer.|
|`lastUpdate`|`uint256`|Unix timestamp of the last identity modification.|

## Enums
### KycStatus
Possible compliance states for an institutional participant.


```solidity
enum KycStatus {
    None,
    Pending,
    Verified,
    Suspended,
    Expired
}
```

### ParticipantRole
Functional roles assigned to network participants.


```solidity
enum ParticipantRole {
    NONE,
    TREASURY,
    GOVERNANCE,
    CENTRAL_BANK,
    COMMERCIAL_BANK
}
```

