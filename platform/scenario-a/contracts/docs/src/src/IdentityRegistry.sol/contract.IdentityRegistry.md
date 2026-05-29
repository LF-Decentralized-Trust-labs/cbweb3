# IdentityRegistry
[Git Source](https://github.com/LACNetNetworks/cbweb3-platform/blob/046ef8ae4f22ae07ddd7d371613d585961088c4e/src/IdentityRegistry.sol)

**Inherits:**
[IIdentityRegistry](/src/interfaces/IIdentityRegistry.sol/interface.IIdentityRegistry.md), AccessControl

**Title:**
IdentityRegistry

Implementation of the institutional registry for the CBDC ecosystem.

Uses AccessControl for governance and IdentityRegistryLibrary for data integrity.


## State Variables
### _participants
Internal storage mapping addresses to their institutional profiles.


```solidity
mapping(address => IdentityRegistryLibrary.Participant) private _participants
```


### GOVERNANCE_ROLE
Role definition for governance administrators (Central Banks).


```solidity
bytes32 public constant GOVERNANCE_ROLE = keccak256("GOVERNANCE_ROLE")
```


## Functions
### constructor

Initializes the registry and assigns the primary administrator.


```solidity
constructor(address admin) ;
```
**Parameters**

|Name|Type|Description|
|----|----|-----------|
|`admin`|`address`|The address granted initial governance and admin rights.|


### registerParticipant

Registers a new participant. Restricted to governance authorities.

Registration is restricted to GOVERNANCE_ROLE holders.


```solidity
function registerParticipant(
    address account,
    string calldata name,
    IdentityRegistryLibrary.ParticipantRole role,
    bytes32 zkPointer
) external override onlyRole(GOVERNANCE_ROLE);
```
**Parameters**

|Name|Type|Description|
|----|----|-----------|
|`account`|`address`|Target wallet address.|
|`name`|`string`|Legal name of the entity.|
|`role`|`IdentityRegistryLibrary.ParticipantRole`|Assigned functional role in the network.|
|`zkPointer`|`bytes32`|Link to the entity's private compliance proofs.|


### canTransact

Gatekeeper function to authorize or reject financial transactions.

Returns true only if the status is exactly 'Verified'.


```solidity
function canTransact(address account) external view override returns (bool);
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
function canGovern(address account) external view override returns (bool);
```
**Parameters**

|Name|Type|Description|
|----|----|-----------|
|`account`|`address`|The address to verify.|

**Returns**

|Name|Type|Description|
|----|----|-----------|
|`<none>`|`bool`|bool True if the account is Verified with a governance-capable role (CENTRAL_BANK or GOVERNANCE).|


### isWhitelisted

Verifies if an address is active and whitelisted.


```solidity
function isWhitelisted(address account) external view override returns (bool);
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
function getParticipant(address account)
    external
    view
    override
    returns (IdentityRegistryLibrary.Participant memory);
```
**Parameters**

|Name|Type|Description|
|----|----|-----------|
|`account`|`address`|The wallet address of the participant.|

**Returns**

|Name|Type|Description|
|----|----|-----------|
|`<none>`|`IdentityRegistryLibrary.Participant`|A Participant struct with all registered metadata.|


### updateStatus

Modifies the status of a participant (e.g., suspension).

Critical for regulatory compliance and account freezing.


```solidity
function updateStatus(address account, IdentityRegistryLibrary.KycStatus newStatus)
    external
    override
    onlyRole(GOVERNANCE_ROLE);
```
**Parameters**

|Name|Type|Description|
|----|----|-----------|
|`account`|`address`|The address to be updated.|
|`newStatus`|`IdentityRegistryLibrary.KycStatus`|The target KycStatus.|


### setCertFingerprint

Registers or updates the X.509 certificate fingerprint for a participant.

Binds an X.509 certificate to a participant's on-chain identity.


```solidity
function setCertFingerprint(address account, bytes32 fingerprint) external override onlyRole(GOVERNANCE_ROLE);
```
**Parameters**

|Name|Type|Description|
|----|----|-----------|
|`account`|`address`|The wallet address whose certificate is being bound.|
|`fingerprint`|`bytes32`|SHA-256 hash of the DER-encoded X.509 certificate.|


### getCertFingerprint

Returns the certificate fingerprint for a participant.


```solidity
function getCertFingerprint(address account) external view override returns (bytes32);
```
**Parameters**

|Name|Type|Description|
|----|----|-----------|
|`account`|`address`|The wallet address to query.|

**Returns**

|Name|Type|Description|
|----|----|-----------|
|`<none>`|`bytes32`|The SHA-256 fingerprint, or bytes32(0) if not set.|


