# FiatCentralBankMoneyLibrary
[Git Source](https://github.com/LACNetNetworks/cbweb3-platform/blob/046ef8ae4f22ae07ddd7d371613d585961088c4e/src/libraries/FiatCentralBankMoneyLibrary.sol)

**Title:**
FiatCentralBankMoneyLibrary

Core data structures and custom errors for the FiatCentralBankMoney (fCeBM) contract.

The fCeBM is an ERC-20 representation of fiat central bank money on the Besu ledger.
It serves as the on-chain fiat asset that commercial banks receive upon deposit
approval and that can later be exchanged for TokenizedCentralBankMoney (tCeBM)
via the escrow flow (burn fCeBM on Besu → mint tCeBM on Paladin/Zeto).


## Events
### FiatMinted
Emitted when the Central Bank mints fCeBM tokens to a commercial bank.


```solidity
event FiatMinted(address indexed to, uint256 amount);
```

**Parameters**

|Name|Type|Description|
|----|----|-----------|
|`to`|`address`|The address receiving the minted tokens.|
|`amount`|`uint256`|The amount of tokens minted.|

### FiatBurned
Emitted when the Central Bank burns fCeBM tokens from an address.


```solidity
event FiatBurned(address indexed from, uint256 amount);
```

**Parameters**

|Name|Type|Description|
|----|----|-----------|
|`from`|`address`|The address from which tokens are burned.|
|`amount`|`uint256`|The amount of tokens burned.|

