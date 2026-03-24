# IIdentityRegistry
[Git Source](https://github.com/LACNetNetworks/cbweb3-platform/blob/e19c9456a3de8cd6d3345c4656e5c68bf8aefa97/src/interfaces/IIdentityRegistry.sol)

**Title:**
IIdentityRegistry

Interface for the central Identity and Compliance Registry.

Standardizes how external modules (AMM, HTLC) interact with identity data.


## Functions
### isWhitelisted

Verifies if an address is active and whitelisted.


```solidity
function isWhitelisted(address account) external view returns (bool);
```
**Parameters**

|Name|Type|Description|
|----|----|-----------|
|`account`|`address`|The wallet address to check.|

**Returns**

|Name|Type|Description|
|----|----|-----------|
|`<none>`|`bool`|True if the participant is verified, false otherwise.|


### getParticipant

Returns the full identity profile for an institution.


```solidity
function getParticipant(address account) external view returns (IdentityRegistryLibrary.Participant memory);
```
**Parameters**

|Name|Type|Description|
|----|----|-----------|
|`account`|`address`|The wallet address of the participant.|

**Returns**

|Name|Type|Description|
|----|----|-----------|
|`<none>`|`IdentityRegistryLibrary.Participant`|A Participant struct with all registered metadata.|


### canTransact

Gatekeeper function to authorize or reject financial transactions.


```solidity
function canTransact(address account) external view returns (bool);
```
**Parameters**

|Name|Type|Description|
|----|----|-----------|
|`account`|`address`|The address initiating a transfer or swap.|

**Returns**

|Name|Type|Description|
|----|----|-----------|
|`<none>`|`bool`|bool True if authorized, false if the account is blocked or unknown.|


### canGovern

Gatekeeper function to verify governance-level authorization.


```solidity
function canGovern(address account) external view returns (bool);
```
**Parameters**

|Name|Type|Description|
|----|----|-----------|
|`account`|`address`|The address to verify.|

**Returns**

|Name|Type|Description|
|----|----|-----------|
|`<none>`|`bool`|bool True if the account is Verified with a governance-capable role (CENTRAL_BANK or GOVERNANCE).|


### registerParticipant

Registers a new participant. Restricted to governance authorities.


```solidity
function registerParticipant(
    address account,
    string calldata name,
    IdentityRegistryLibrary.ParticipantRole role,
    bytes32 zkPointer
) external;
```
**Parameters**

|Name|Type|Description|
|----|----|-----------|
|`account`|`address`|Target wallet address.|
|`name`|`string`|Legal name of the entity.|
|`role`|`IdentityRegistryLibrary.ParticipantRole`|Assigned functional role in the network.|
|`zkPointer`|`bytes32`|Link to the entity's private compliance proofs.|


### updateStatus

Modifies the status of a participant (e.g., suspension).


```solidity
function updateStatus(address account, IdentityRegistryLibrary.KycStatus newStatus) external;
```
**Parameters**

|Name|Type|Description|
|----|----|-----------|
|`account`|`address`|The address to be updated.|
|`newStatus`|`IdentityRegistryLibrary.KycStatus`|The target KycStatus.|


## Events
### ParticipantRegistered
Emitted when a new institution is onboarded to the network.


```solidity
event ParticipantRegistered(address indexed account, IdentityRegistryLibrary.ParticipantRole role, string name);
```

### IdentityUpdated
Emitted when an institution's status is modified by governance.


```solidity
event IdentityUpdated(
    address indexed account,
    IdentityRegistryLibrary.KycStatus oldStatus,
    IdentityRegistryLibrary.KycStatus newStatus
);
```

## Errors
### ParticipantNotVerified
Error thrown when an unverified account attempts a restricted operation.


```solidity
error ParticipantNotVerified(address account);
```

### InvalidIdentityData
Error thrown when registration data (e.g., address zero) is invalid.


```solidity
error InvalidIdentityData();
```

