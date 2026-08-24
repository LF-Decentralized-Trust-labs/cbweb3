// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package bindings

import (
	"errors"
	"math/big"
	"strings"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
)

// Reference imports to suppress errors if they are not otherwise used.
var (
	_ = errors.New
	_ = big.NewInt
	_ = strings.NewReader
	_ = ethereum.NotFound
	_ = bind.Bind
	_ = common.Big1
	_ = types.BloomLookup
	_ = event.NewSubscription
	_ = abi.ConvertType
)

// IdentityRegistryLibraryParticipant is an auto generated low-level Go binding around an user-defined struct.
type IdentityRegistryLibraryParticipant struct {
	LegalName       string
	InstitutionId   [32]byte
	Role            uint8
	Status          uint8
	ZkPointer       [32]byte
	CertFingerprint [32]byte
	LastUpdate      *big.Int
}

// IdentityRegistryMetaData contains all meta data concerning the IdentityRegistry contract.
var IdentityRegistryMetaData = &bind.MetaData{
	ABI: "[{\"type\":\"constructor\",\"inputs\":[{\"name\":\"admin\",\"type\":\"address\",\"internalType\":\"address\"}],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"DEFAULT_ADMIN_ROLE\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"GOVERNANCE_ROLE\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"VERIFIER_ROLE\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"canGovern\",\"inputs\":[{\"name\":\"account\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[{\"name\":\"\",\"type\":\"bool\",\"internalType\":\"bool\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"canTransact\",\"inputs\":[{\"name\":\"account\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[{\"name\":\"\",\"type\":\"bool\",\"internalType\":\"bool\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"getCentralBankOf\",\"inputs\":[{\"name\":\"token\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[{\"name\":\"\",\"type\":\"address\",\"internalType\":\"address\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"getCertFingerprint\",\"inputs\":[{\"name\":\"account\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[{\"name\":\"\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"getInstitutionId\",\"inputs\":[{\"name\":\"account\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[{\"name\":\"\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"getParticipant\",\"inputs\":[{\"name\":\"account\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[{\"name\":\"\",\"type\":\"tuple\",\"internalType\":\"structIdentityRegistryLibrary.Participant\",\"components\":[{\"name\":\"legalName\",\"type\":\"string\",\"internalType\":\"string\"},{\"name\":\"institutionId\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"},{\"name\":\"role\",\"type\":\"uint8\",\"internalType\":\"enumIdentityRegistryLibrary.ParticipantRole\"},{\"name\":\"status\",\"type\":\"uint8\",\"internalType\":\"enumIdentityRegistryLibrary.KycStatus\"},{\"name\":\"zkPointer\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"},{\"name\":\"certFingerprint\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"},{\"name\":\"lastUpdate\",\"type\":\"uint256\",\"internalType\":\"uint256\"}]}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"getRoleAdmin\",\"inputs\":[{\"name\":\"role\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"}],\"outputs\":[{\"name\":\"\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"grantLiquidityProvider\",\"inputs\":[{\"name\":\"account\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"grantRole\",\"inputs\":[{\"name\":\"role\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"},{\"name\":\"account\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"hasRole\",\"inputs\":[{\"name\":\"role\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"},{\"name\":\"account\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[{\"name\":\"\",\"type\":\"bool\",\"internalType\":\"bool\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"isLiquidityProvider\",\"inputs\":[{\"name\":\"account\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[{\"name\":\"\",\"type\":\"bool\",\"internalType\":\"bool\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"isWhitelisted\",\"inputs\":[{\"name\":\"account\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[{\"name\":\"\",\"type\":\"bool\",\"internalType\":\"bool\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"registerParticipant\",\"inputs\":[{\"name\":\"account\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"name\",\"type\":\"string\",\"internalType\":\"string\"},{\"name\":\"role\",\"type\":\"uint8\",\"internalType\":\"enumIdentityRegistryLibrary.ParticipantRole\"},{\"name\":\"zkPointer\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"},{\"name\":\"institutionId\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"renounceRole\",\"inputs\":[{\"name\":\"role\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"},{\"name\":\"callerConfirmation\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"revokeLiquidityProvider\",\"inputs\":[{\"name\":\"account\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"revokeRole\",\"inputs\":[{\"name\":\"role\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"},{\"name\":\"account\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"setCentralBankOf\",\"inputs\":[{\"name\":\"token\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"centralBank\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"setCertFingerprint\",\"inputs\":[{\"name\":\"account\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"fingerprint\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"supportsInterface\",\"inputs\":[{\"name\":\"interfaceId\",\"type\":\"bytes4\",\"internalType\":\"bytes4\"}],\"outputs\":[{\"name\":\"\",\"type\":\"bool\",\"internalType\":\"bool\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"updateStatus\",\"inputs\":[{\"name\":\"account\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"newStatus\",\"type\":\"uint8\",\"internalType\":\"enumIdentityRegistryLibrary.KycStatus\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"verifyParticipant\",\"inputs\":[{\"name\":\"account\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"event\",\"name\":\"CertificateRegistered\",\"inputs\":[{\"name\":\"account\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"certFingerprint\",\"type\":\"bytes32\",\"indexed\":false,\"internalType\":\"bytes32\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"IdentityUpdated\",\"inputs\":[{\"name\":\"account\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"oldStatus\",\"type\":\"uint8\",\"indexed\":false,\"internalType\":\"enumIdentityRegistryLibrary.KycStatus\"},{\"name\":\"newStatus\",\"type\":\"uint8\",\"indexed\":false,\"internalType\":\"enumIdentityRegistryLibrary.KycStatus\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"LogCentralBankOfTokenSet\",\"inputs\":[{\"name\":\"token\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"centralBank\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"LogLiquidityProviderGranted\",\"inputs\":[{\"name\":\"account\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"LogLiquidityProviderRevoked\",\"inputs\":[{\"name\":\"account\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"ParticipantRegistered\",\"inputs\":[{\"name\":\"account\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"role\",\"type\":\"uint8\",\"indexed\":false,\"internalType\":\"enumIdentityRegistryLibrary.ParticipantRole\"},{\"name\":\"name\",\"type\":\"string\",\"indexed\":false,\"internalType\":\"string\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"RoleAdminChanged\",\"inputs\":[{\"name\":\"role\",\"type\":\"bytes32\",\"indexed\":true,\"internalType\":\"bytes32\"},{\"name\":\"previousAdminRole\",\"type\":\"bytes32\",\"indexed\":true,\"internalType\":\"bytes32\"},{\"name\":\"newAdminRole\",\"type\":\"bytes32\",\"indexed\":true,\"internalType\":\"bytes32\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"RoleGranted\",\"inputs\":[{\"name\":\"role\",\"type\":\"bytes32\",\"indexed\":true,\"internalType\":\"bytes32\"},{\"name\":\"account\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"sender\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"RoleRevoked\",\"inputs\":[{\"name\":\"role\",\"type\":\"bytes32\",\"indexed\":true,\"internalType\":\"bytes32\"},{\"name\":\"account\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"sender\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"}],\"anonymous\":false},{\"type\":\"error\",\"name\":\"AccessControlBadConfirmation\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"AccessControlUnauthorizedAccount\",\"inputs\":[{\"name\":\"account\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"neededRole\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"}]},{\"type\":\"error\",\"name\":\"InvalidIdentityData\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"ParticipantNotPending\",\"inputs\":[{\"name\":\"account\",\"type\":\"address\",\"internalType\":\"address\"}]},{\"type\":\"error\",\"name\":\"ParticipantNotVerified\",\"inputs\":[{\"name\":\"account\",\"type\":\"address\",\"internalType\":\"address\"}]}]",
}

// IdentityRegistryABI is the input ABI used to generate the binding from.
// Deprecated: Use IdentityRegistryMetaData.ABI instead.
var IdentityRegistryABI = IdentityRegistryMetaData.ABI

// IdentityRegistry is an auto generated Go binding around an Ethereum contract.
type IdentityRegistry struct {
	IdentityRegistryCaller     // Read-only binding to the contract
	IdentityRegistryTransactor // Write-only binding to the contract
	IdentityRegistryFilterer   // Log filterer for contract events
}

// IdentityRegistryCaller is an auto generated read-only Go binding around an Ethereum contract.
type IdentityRegistryCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// IdentityRegistryTransactor is an auto generated write-only Go binding around an Ethereum contract.
type IdentityRegistryTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// IdentityRegistryFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type IdentityRegistryFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// IdentityRegistrySession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type IdentityRegistrySession struct {
	Contract     *IdentityRegistry // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// IdentityRegistryCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type IdentityRegistryCallerSession struct {
	Contract *IdentityRegistryCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts           // Call options to use throughout this session
}

// IdentityRegistryTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type IdentityRegistryTransactorSession struct {
	Contract     *IdentityRegistryTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts           // Transaction auth options to use throughout this session
}

// IdentityRegistryRaw is an auto generated low-level Go binding around an Ethereum contract.
type IdentityRegistryRaw struct {
	Contract *IdentityRegistry // Generic contract binding to access the raw methods on
}

// IdentityRegistryCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type IdentityRegistryCallerRaw struct {
	Contract *IdentityRegistryCaller // Generic read-only contract binding to access the raw methods on
}

// IdentityRegistryTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type IdentityRegistryTransactorRaw struct {
	Contract *IdentityRegistryTransactor // Generic write-only contract binding to access the raw methods on
}

// NewIdentityRegistry creates a new instance of IdentityRegistry, bound to a specific deployed contract.
func NewIdentityRegistry(address common.Address, backend bind.ContractBackend) (*IdentityRegistry, error) {
	contract, err := bindIdentityRegistry(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &IdentityRegistry{IdentityRegistryCaller: IdentityRegistryCaller{contract: contract}, IdentityRegistryTransactor: IdentityRegistryTransactor{contract: contract}, IdentityRegistryFilterer: IdentityRegistryFilterer{contract: contract}}, nil
}

// NewIdentityRegistryCaller creates a new read-only instance of IdentityRegistry, bound to a specific deployed contract.
func NewIdentityRegistryCaller(address common.Address, caller bind.ContractCaller) (*IdentityRegistryCaller, error) {
	contract, err := bindIdentityRegistry(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &IdentityRegistryCaller{contract: contract}, nil
}

// NewIdentityRegistryTransactor creates a new write-only instance of IdentityRegistry, bound to a specific deployed contract.
func NewIdentityRegistryTransactor(address common.Address, transactor bind.ContractTransactor) (*IdentityRegistryTransactor, error) {
	contract, err := bindIdentityRegistry(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &IdentityRegistryTransactor{contract: contract}, nil
}

// NewIdentityRegistryFilterer creates a new log filterer instance of IdentityRegistry, bound to a specific deployed contract.
func NewIdentityRegistryFilterer(address common.Address, filterer bind.ContractFilterer) (*IdentityRegistryFilterer, error) {
	contract, err := bindIdentityRegistry(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &IdentityRegistryFilterer{contract: contract}, nil
}

// bindIdentityRegistry binds a generic wrapper to an already deployed contract.
func bindIdentityRegistry(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := IdentityRegistryMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, *parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_IdentityRegistry *IdentityRegistryRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _IdentityRegistry.Contract.IdentityRegistryCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_IdentityRegistry *IdentityRegistryRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _IdentityRegistry.Contract.IdentityRegistryTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_IdentityRegistry *IdentityRegistryRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _IdentityRegistry.Contract.IdentityRegistryTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_IdentityRegistry *IdentityRegistryCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _IdentityRegistry.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_IdentityRegistry *IdentityRegistryTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _IdentityRegistry.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_IdentityRegistry *IdentityRegistryTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _IdentityRegistry.Contract.contract.Transact(opts, method, params...)
}

// DEFAULTADMINROLE is a free data retrieval call binding the contract method 0xa217fddf.
//
// Solidity: function DEFAULT_ADMIN_ROLE() view returns(bytes32)
func (_IdentityRegistry *IdentityRegistryCaller) DEFAULTADMINROLE(opts *bind.CallOpts) ([32]byte, error) {
	var out []interface{}
	err := _IdentityRegistry.contract.Call(opts, &out, "DEFAULT_ADMIN_ROLE")

	if err != nil {
		return *new([32]byte), err
	}

	out0 := *abi.ConvertType(out[0], new([32]byte)).(*[32]byte)

	return out0, err

}

// DEFAULTADMINROLE is a free data retrieval call binding the contract method 0xa217fddf.
//
// Solidity: function DEFAULT_ADMIN_ROLE() view returns(bytes32)
func (_IdentityRegistry *IdentityRegistrySession) DEFAULTADMINROLE() ([32]byte, error) {
	return _IdentityRegistry.Contract.DEFAULTADMINROLE(&_IdentityRegistry.CallOpts)
}

// DEFAULTADMINROLE is a free data retrieval call binding the contract method 0xa217fddf.
//
// Solidity: function DEFAULT_ADMIN_ROLE() view returns(bytes32)
func (_IdentityRegistry *IdentityRegistryCallerSession) DEFAULTADMINROLE() ([32]byte, error) {
	return _IdentityRegistry.Contract.DEFAULTADMINROLE(&_IdentityRegistry.CallOpts)
}

// GOVERNANCEROLE is a free data retrieval call binding the contract method 0xf36c8f5c.
//
// Solidity: function GOVERNANCE_ROLE() view returns(bytes32)
func (_IdentityRegistry *IdentityRegistryCaller) GOVERNANCEROLE(opts *bind.CallOpts) ([32]byte, error) {
	var out []interface{}
	err := _IdentityRegistry.contract.Call(opts, &out, "GOVERNANCE_ROLE")

	if err != nil {
		return *new([32]byte), err
	}

	out0 := *abi.ConvertType(out[0], new([32]byte)).(*[32]byte)

	return out0, err

}

// GOVERNANCEROLE is a free data retrieval call binding the contract method 0xf36c8f5c.
//
// Solidity: function GOVERNANCE_ROLE() view returns(bytes32)
func (_IdentityRegistry *IdentityRegistrySession) GOVERNANCEROLE() ([32]byte, error) {
	return _IdentityRegistry.Contract.GOVERNANCEROLE(&_IdentityRegistry.CallOpts)
}

// GOVERNANCEROLE is a free data retrieval call binding the contract method 0xf36c8f5c.
//
// Solidity: function GOVERNANCE_ROLE() view returns(bytes32)
func (_IdentityRegistry *IdentityRegistryCallerSession) GOVERNANCEROLE() ([32]byte, error) {
	return _IdentityRegistry.Contract.GOVERNANCEROLE(&_IdentityRegistry.CallOpts)
}

// VERIFIERROLE is a free data retrieval call binding the contract method 0xe7705db6.
//
// Solidity: function VERIFIER_ROLE() view returns(bytes32)
func (_IdentityRegistry *IdentityRegistryCaller) VERIFIERROLE(opts *bind.CallOpts) ([32]byte, error) {
	var out []interface{}
	err := _IdentityRegistry.contract.Call(opts, &out, "VERIFIER_ROLE")

	if err != nil {
		return *new([32]byte), err
	}

	out0 := *abi.ConvertType(out[0], new([32]byte)).(*[32]byte)

	return out0, err

}

// VERIFIERROLE is a free data retrieval call binding the contract method 0xe7705db6.
//
// Solidity: function VERIFIER_ROLE() view returns(bytes32)
func (_IdentityRegistry *IdentityRegistrySession) VERIFIERROLE() ([32]byte, error) {
	return _IdentityRegistry.Contract.VERIFIERROLE(&_IdentityRegistry.CallOpts)
}

// VERIFIERROLE is a free data retrieval call binding the contract method 0xe7705db6.
//
// Solidity: function VERIFIER_ROLE() view returns(bytes32)
func (_IdentityRegistry *IdentityRegistryCallerSession) VERIFIERROLE() ([32]byte, error) {
	return _IdentityRegistry.Contract.VERIFIERROLE(&_IdentityRegistry.CallOpts)
}

// CanGovern is a free data retrieval call binding the contract method 0x53aa4307.
//
// Solidity: function canGovern(address account) view returns(bool)
func (_IdentityRegistry *IdentityRegistryCaller) CanGovern(opts *bind.CallOpts, account common.Address) (bool, error) {
	var out []interface{}
	err := _IdentityRegistry.contract.Call(opts, &out, "canGovern", account)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// CanGovern is a free data retrieval call binding the contract method 0x53aa4307.
//
// Solidity: function canGovern(address account) view returns(bool)
func (_IdentityRegistry *IdentityRegistrySession) CanGovern(account common.Address) (bool, error) {
	return _IdentityRegistry.Contract.CanGovern(&_IdentityRegistry.CallOpts, account)
}

// CanGovern is a free data retrieval call binding the contract method 0x53aa4307.
//
// Solidity: function canGovern(address account) view returns(bool)
func (_IdentityRegistry *IdentityRegistryCallerSession) CanGovern(account common.Address) (bool, error) {
	return _IdentityRegistry.Contract.CanGovern(&_IdentityRegistry.CallOpts, account)
}

// CanTransact is a free data retrieval call binding the contract method 0xacf0279f.
//
// Solidity: function canTransact(address account) view returns(bool)
func (_IdentityRegistry *IdentityRegistryCaller) CanTransact(opts *bind.CallOpts, account common.Address) (bool, error) {
	var out []interface{}
	err := _IdentityRegistry.contract.Call(opts, &out, "canTransact", account)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// CanTransact is a free data retrieval call binding the contract method 0xacf0279f.
//
// Solidity: function canTransact(address account) view returns(bool)
func (_IdentityRegistry *IdentityRegistrySession) CanTransact(account common.Address) (bool, error) {
	return _IdentityRegistry.Contract.CanTransact(&_IdentityRegistry.CallOpts, account)
}

// CanTransact is a free data retrieval call binding the contract method 0xacf0279f.
//
// Solidity: function canTransact(address account) view returns(bool)
func (_IdentityRegistry *IdentityRegistryCallerSession) CanTransact(account common.Address) (bool, error) {
	return _IdentityRegistry.Contract.CanTransact(&_IdentityRegistry.CallOpts, account)
}

// GetCentralBankOf is a free data retrieval call binding the contract method 0x41db05de.
//
// Solidity: function getCentralBankOf(address token) view returns(address)
func (_IdentityRegistry *IdentityRegistryCaller) GetCentralBankOf(opts *bind.CallOpts, token common.Address) (common.Address, error) {
	var out []interface{}
	err := _IdentityRegistry.contract.Call(opts, &out, "getCentralBankOf", token)

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// GetCentralBankOf is a free data retrieval call binding the contract method 0x41db05de.
//
// Solidity: function getCentralBankOf(address token) view returns(address)
func (_IdentityRegistry *IdentityRegistrySession) GetCentralBankOf(token common.Address) (common.Address, error) {
	return _IdentityRegistry.Contract.GetCentralBankOf(&_IdentityRegistry.CallOpts, token)
}

// GetCentralBankOf is a free data retrieval call binding the contract method 0x41db05de.
//
// Solidity: function getCentralBankOf(address token) view returns(address)
func (_IdentityRegistry *IdentityRegistryCallerSession) GetCentralBankOf(token common.Address) (common.Address, error) {
	return _IdentityRegistry.Contract.GetCentralBankOf(&_IdentityRegistry.CallOpts, token)
}

// GetCertFingerprint is a free data retrieval call binding the contract method 0x41489f5c.
//
// Solidity: function getCertFingerprint(address account) view returns(bytes32)
func (_IdentityRegistry *IdentityRegistryCaller) GetCertFingerprint(opts *bind.CallOpts, account common.Address) ([32]byte, error) {
	var out []interface{}
	err := _IdentityRegistry.contract.Call(opts, &out, "getCertFingerprint", account)

	if err != nil {
		return *new([32]byte), err
	}

	out0 := *abi.ConvertType(out[0], new([32]byte)).(*[32]byte)

	return out0, err

}

// GetCertFingerprint is a free data retrieval call binding the contract method 0x41489f5c.
//
// Solidity: function getCertFingerprint(address account) view returns(bytes32)
func (_IdentityRegistry *IdentityRegistrySession) GetCertFingerprint(account common.Address) ([32]byte, error) {
	return _IdentityRegistry.Contract.GetCertFingerprint(&_IdentityRegistry.CallOpts, account)
}

// GetCertFingerprint is a free data retrieval call binding the contract method 0x41489f5c.
//
// Solidity: function getCertFingerprint(address account) view returns(bytes32)
func (_IdentityRegistry *IdentityRegistryCallerSession) GetCertFingerprint(account common.Address) ([32]byte, error) {
	return _IdentityRegistry.Contract.GetCertFingerprint(&_IdentityRegistry.CallOpts, account)
}

// GetInstitutionId is a free data retrieval call binding the contract method 0x6fb8224b.
//
// Solidity: function getInstitutionId(address account) view returns(bytes32)
func (_IdentityRegistry *IdentityRegistryCaller) GetInstitutionId(opts *bind.CallOpts, account common.Address) ([32]byte, error) {
	var out []interface{}
	err := _IdentityRegistry.contract.Call(opts, &out, "getInstitutionId", account)

	if err != nil {
		return *new([32]byte), err
	}

	out0 := *abi.ConvertType(out[0], new([32]byte)).(*[32]byte)

	return out0, err

}

// GetInstitutionId is a free data retrieval call binding the contract method 0x6fb8224b.
//
// Solidity: function getInstitutionId(address account) view returns(bytes32)
func (_IdentityRegistry *IdentityRegistrySession) GetInstitutionId(account common.Address) ([32]byte, error) {
	return _IdentityRegistry.Contract.GetInstitutionId(&_IdentityRegistry.CallOpts, account)
}

// GetInstitutionId is a free data retrieval call binding the contract method 0x6fb8224b.
//
// Solidity: function getInstitutionId(address account) view returns(bytes32)
func (_IdentityRegistry *IdentityRegistryCallerSession) GetInstitutionId(account common.Address) ([32]byte, error) {
	return _IdentityRegistry.Contract.GetInstitutionId(&_IdentityRegistry.CallOpts, account)
}

// GetParticipant is a free data retrieval call binding the contract method 0x7143059f.
//
// Solidity: function getParticipant(address account) view returns((string,bytes32,uint8,uint8,bytes32,bytes32,uint256))
func (_IdentityRegistry *IdentityRegistryCaller) GetParticipant(opts *bind.CallOpts, account common.Address) (IdentityRegistryLibraryParticipant, error) {
	var out []interface{}
	err := _IdentityRegistry.contract.Call(opts, &out, "getParticipant", account)

	if err != nil {
		return *new(IdentityRegistryLibraryParticipant), err
	}

	out0 := *abi.ConvertType(out[0], new(IdentityRegistryLibraryParticipant)).(*IdentityRegistryLibraryParticipant)

	return out0, err

}

// GetParticipant is a free data retrieval call binding the contract method 0x7143059f.
//
// Solidity: function getParticipant(address account) view returns((string,bytes32,uint8,uint8,bytes32,bytes32,uint256))
func (_IdentityRegistry *IdentityRegistrySession) GetParticipant(account common.Address) (IdentityRegistryLibraryParticipant, error) {
	return _IdentityRegistry.Contract.GetParticipant(&_IdentityRegistry.CallOpts, account)
}

// GetParticipant is a free data retrieval call binding the contract method 0x7143059f.
//
// Solidity: function getParticipant(address account) view returns((string,bytes32,uint8,uint8,bytes32,bytes32,uint256))
func (_IdentityRegistry *IdentityRegistryCallerSession) GetParticipant(account common.Address) (IdentityRegistryLibraryParticipant, error) {
	return _IdentityRegistry.Contract.GetParticipant(&_IdentityRegistry.CallOpts, account)
}

// GetRoleAdmin is a free data retrieval call binding the contract method 0x248a9ca3.
//
// Solidity: function getRoleAdmin(bytes32 role) view returns(bytes32)
func (_IdentityRegistry *IdentityRegistryCaller) GetRoleAdmin(opts *bind.CallOpts, role [32]byte) ([32]byte, error) {
	var out []interface{}
	err := _IdentityRegistry.contract.Call(opts, &out, "getRoleAdmin", role)

	if err != nil {
		return *new([32]byte), err
	}

	out0 := *abi.ConvertType(out[0], new([32]byte)).(*[32]byte)

	return out0, err

}

// GetRoleAdmin is a free data retrieval call binding the contract method 0x248a9ca3.
//
// Solidity: function getRoleAdmin(bytes32 role) view returns(bytes32)
func (_IdentityRegistry *IdentityRegistrySession) GetRoleAdmin(role [32]byte) ([32]byte, error) {
	return _IdentityRegistry.Contract.GetRoleAdmin(&_IdentityRegistry.CallOpts, role)
}

// GetRoleAdmin is a free data retrieval call binding the contract method 0x248a9ca3.
//
// Solidity: function getRoleAdmin(bytes32 role) view returns(bytes32)
func (_IdentityRegistry *IdentityRegistryCallerSession) GetRoleAdmin(role [32]byte) ([32]byte, error) {
	return _IdentityRegistry.Contract.GetRoleAdmin(&_IdentityRegistry.CallOpts, role)
}

// HasRole is a free data retrieval call binding the contract method 0x91d14854.
//
// Solidity: function hasRole(bytes32 role, address account) view returns(bool)
func (_IdentityRegistry *IdentityRegistryCaller) HasRole(opts *bind.CallOpts, role [32]byte, account common.Address) (bool, error) {
	var out []interface{}
	err := _IdentityRegistry.contract.Call(opts, &out, "hasRole", role, account)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// HasRole is a free data retrieval call binding the contract method 0x91d14854.
//
// Solidity: function hasRole(bytes32 role, address account) view returns(bool)
func (_IdentityRegistry *IdentityRegistrySession) HasRole(role [32]byte, account common.Address) (bool, error) {
	return _IdentityRegistry.Contract.HasRole(&_IdentityRegistry.CallOpts, role, account)
}

// HasRole is a free data retrieval call binding the contract method 0x91d14854.
//
// Solidity: function hasRole(bytes32 role, address account) view returns(bool)
func (_IdentityRegistry *IdentityRegistryCallerSession) HasRole(role [32]byte, account common.Address) (bool, error) {
	return _IdentityRegistry.Contract.HasRole(&_IdentityRegistry.CallOpts, role, account)
}

// IsLiquidityProvider is a free data retrieval call binding the contract method 0x99f7854a.
//
// Solidity: function isLiquidityProvider(address account) view returns(bool)
func (_IdentityRegistry *IdentityRegistryCaller) IsLiquidityProvider(opts *bind.CallOpts, account common.Address) (bool, error) {
	var out []interface{}
	err := _IdentityRegistry.contract.Call(opts, &out, "isLiquidityProvider", account)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// IsLiquidityProvider is a free data retrieval call binding the contract method 0x99f7854a.
//
// Solidity: function isLiquidityProvider(address account) view returns(bool)
func (_IdentityRegistry *IdentityRegistrySession) IsLiquidityProvider(account common.Address) (bool, error) {
	return _IdentityRegistry.Contract.IsLiquidityProvider(&_IdentityRegistry.CallOpts, account)
}

// IsLiquidityProvider is a free data retrieval call binding the contract method 0x99f7854a.
//
// Solidity: function isLiquidityProvider(address account) view returns(bool)
func (_IdentityRegistry *IdentityRegistryCallerSession) IsLiquidityProvider(account common.Address) (bool, error) {
	return _IdentityRegistry.Contract.IsLiquidityProvider(&_IdentityRegistry.CallOpts, account)
}

// IsWhitelisted is a free data retrieval call binding the contract method 0x3af32abf.
//
// Solidity: function isWhitelisted(address account) view returns(bool)
func (_IdentityRegistry *IdentityRegistryCaller) IsWhitelisted(opts *bind.CallOpts, account common.Address) (bool, error) {
	var out []interface{}
	err := _IdentityRegistry.contract.Call(opts, &out, "isWhitelisted", account)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// IsWhitelisted is a free data retrieval call binding the contract method 0x3af32abf.
//
// Solidity: function isWhitelisted(address account) view returns(bool)
func (_IdentityRegistry *IdentityRegistrySession) IsWhitelisted(account common.Address) (bool, error) {
	return _IdentityRegistry.Contract.IsWhitelisted(&_IdentityRegistry.CallOpts, account)
}

// IsWhitelisted is a free data retrieval call binding the contract method 0x3af32abf.
//
// Solidity: function isWhitelisted(address account) view returns(bool)
func (_IdentityRegistry *IdentityRegistryCallerSession) IsWhitelisted(account common.Address) (bool, error) {
	return _IdentityRegistry.Contract.IsWhitelisted(&_IdentityRegistry.CallOpts, account)
}

// SupportsInterface is a free data retrieval call binding the contract method 0x01ffc9a7.
//
// Solidity: function supportsInterface(bytes4 interfaceId) view returns(bool)
func (_IdentityRegistry *IdentityRegistryCaller) SupportsInterface(opts *bind.CallOpts, interfaceId [4]byte) (bool, error) {
	var out []interface{}
	err := _IdentityRegistry.contract.Call(opts, &out, "supportsInterface", interfaceId)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// SupportsInterface is a free data retrieval call binding the contract method 0x01ffc9a7.
//
// Solidity: function supportsInterface(bytes4 interfaceId) view returns(bool)
func (_IdentityRegistry *IdentityRegistrySession) SupportsInterface(interfaceId [4]byte) (bool, error) {
	return _IdentityRegistry.Contract.SupportsInterface(&_IdentityRegistry.CallOpts, interfaceId)
}

// SupportsInterface is a free data retrieval call binding the contract method 0x01ffc9a7.
//
// Solidity: function supportsInterface(bytes4 interfaceId) view returns(bool)
func (_IdentityRegistry *IdentityRegistryCallerSession) SupportsInterface(interfaceId [4]byte) (bool, error) {
	return _IdentityRegistry.Contract.SupportsInterface(&_IdentityRegistry.CallOpts, interfaceId)
}

// GrantLiquidityProvider is a paid mutator transaction binding the contract method 0xbfc919cc.
//
// Solidity: function grantLiquidityProvider(address account) returns()
func (_IdentityRegistry *IdentityRegistryTransactor) GrantLiquidityProvider(opts *bind.TransactOpts, account common.Address) (*types.Transaction, error) {
	return _IdentityRegistry.contract.Transact(opts, "grantLiquidityProvider", account)
}

// GrantLiquidityProvider is a paid mutator transaction binding the contract method 0xbfc919cc.
//
// Solidity: function grantLiquidityProvider(address account) returns()
func (_IdentityRegistry *IdentityRegistrySession) GrantLiquidityProvider(account common.Address) (*types.Transaction, error) {
	return _IdentityRegistry.Contract.GrantLiquidityProvider(&_IdentityRegistry.TransactOpts, account)
}

// GrantLiquidityProvider is a paid mutator transaction binding the contract method 0xbfc919cc.
//
// Solidity: function grantLiquidityProvider(address account) returns()
func (_IdentityRegistry *IdentityRegistryTransactorSession) GrantLiquidityProvider(account common.Address) (*types.Transaction, error) {
	return _IdentityRegistry.Contract.GrantLiquidityProvider(&_IdentityRegistry.TransactOpts, account)
}

// GrantRole is a paid mutator transaction binding the contract method 0x2f2ff15d.
//
// Solidity: function grantRole(bytes32 role, address account) returns()
func (_IdentityRegistry *IdentityRegistryTransactor) GrantRole(opts *bind.TransactOpts, role [32]byte, account common.Address) (*types.Transaction, error) {
	return _IdentityRegistry.contract.Transact(opts, "grantRole", role, account)
}

// GrantRole is a paid mutator transaction binding the contract method 0x2f2ff15d.
//
// Solidity: function grantRole(bytes32 role, address account) returns()
func (_IdentityRegistry *IdentityRegistrySession) GrantRole(role [32]byte, account common.Address) (*types.Transaction, error) {
	return _IdentityRegistry.Contract.GrantRole(&_IdentityRegistry.TransactOpts, role, account)
}

// GrantRole is a paid mutator transaction binding the contract method 0x2f2ff15d.
//
// Solidity: function grantRole(bytes32 role, address account) returns()
func (_IdentityRegistry *IdentityRegistryTransactorSession) GrantRole(role [32]byte, account common.Address) (*types.Transaction, error) {
	return _IdentityRegistry.Contract.GrantRole(&_IdentityRegistry.TransactOpts, role, account)
}

// RegisterParticipant is a paid mutator transaction binding the contract method 0x3bfef0f0.
//
// Solidity: function registerParticipant(address account, string name, uint8 role, bytes32 zkPointer, bytes32 institutionId) returns()
func (_IdentityRegistry *IdentityRegistryTransactor) RegisterParticipant(opts *bind.TransactOpts, account common.Address, name string, role uint8, zkPointer [32]byte, institutionId [32]byte) (*types.Transaction, error) {
	return _IdentityRegistry.contract.Transact(opts, "registerParticipant", account, name, role, zkPointer, institutionId)
}

// RegisterParticipant is a paid mutator transaction binding the contract method 0x3bfef0f0.
//
// Solidity: function registerParticipant(address account, string name, uint8 role, bytes32 zkPointer, bytes32 institutionId) returns()
func (_IdentityRegistry *IdentityRegistrySession) RegisterParticipant(account common.Address, name string, role uint8, zkPointer [32]byte, institutionId [32]byte) (*types.Transaction, error) {
	return _IdentityRegistry.Contract.RegisterParticipant(&_IdentityRegistry.TransactOpts, account, name, role, zkPointer, institutionId)
}

// RegisterParticipant is a paid mutator transaction binding the contract method 0x3bfef0f0.
//
// Solidity: function registerParticipant(address account, string name, uint8 role, bytes32 zkPointer, bytes32 institutionId) returns()
func (_IdentityRegistry *IdentityRegistryTransactorSession) RegisterParticipant(account common.Address, name string, role uint8, zkPointer [32]byte, institutionId [32]byte) (*types.Transaction, error) {
	return _IdentityRegistry.Contract.RegisterParticipant(&_IdentityRegistry.TransactOpts, account, name, role, zkPointer, institutionId)
}

// RenounceRole is a paid mutator transaction binding the contract method 0x36568abe.
//
// Solidity: function renounceRole(bytes32 role, address callerConfirmation) returns()
func (_IdentityRegistry *IdentityRegistryTransactor) RenounceRole(opts *bind.TransactOpts, role [32]byte, callerConfirmation common.Address) (*types.Transaction, error) {
	return _IdentityRegistry.contract.Transact(opts, "renounceRole", role, callerConfirmation)
}

// RenounceRole is a paid mutator transaction binding the contract method 0x36568abe.
//
// Solidity: function renounceRole(bytes32 role, address callerConfirmation) returns()
func (_IdentityRegistry *IdentityRegistrySession) RenounceRole(role [32]byte, callerConfirmation common.Address) (*types.Transaction, error) {
	return _IdentityRegistry.Contract.RenounceRole(&_IdentityRegistry.TransactOpts, role, callerConfirmation)
}

// RenounceRole is a paid mutator transaction binding the contract method 0x36568abe.
//
// Solidity: function renounceRole(bytes32 role, address callerConfirmation) returns()
func (_IdentityRegistry *IdentityRegistryTransactorSession) RenounceRole(role [32]byte, callerConfirmation common.Address) (*types.Transaction, error) {
	return _IdentityRegistry.Contract.RenounceRole(&_IdentityRegistry.TransactOpts, role, callerConfirmation)
}

// RevokeLiquidityProvider is a paid mutator transaction binding the contract method 0x5913a644.
//
// Solidity: function revokeLiquidityProvider(address account) returns()
func (_IdentityRegistry *IdentityRegistryTransactor) RevokeLiquidityProvider(opts *bind.TransactOpts, account common.Address) (*types.Transaction, error) {
	return _IdentityRegistry.contract.Transact(opts, "revokeLiquidityProvider", account)
}

// RevokeLiquidityProvider is a paid mutator transaction binding the contract method 0x5913a644.
//
// Solidity: function revokeLiquidityProvider(address account) returns()
func (_IdentityRegistry *IdentityRegistrySession) RevokeLiquidityProvider(account common.Address) (*types.Transaction, error) {
	return _IdentityRegistry.Contract.RevokeLiquidityProvider(&_IdentityRegistry.TransactOpts, account)
}

// RevokeLiquidityProvider is a paid mutator transaction binding the contract method 0x5913a644.
//
// Solidity: function revokeLiquidityProvider(address account) returns()
func (_IdentityRegistry *IdentityRegistryTransactorSession) RevokeLiquidityProvider(account common.Address) (*types.Transaction, error) {
	return _IdentityRegistry.Contract.RevokeLiquidityProvider(&_IdentityRegistry.TransactOpts, account)
}

// RevokeRole is a paid mutator transaction binding the contract method 0xd547741f.
//
// Solidity: function revokeRole(bytes32 role, address account) returns()
func (_IdentityRegistry *IdentityRegistryTransactor) RevokeRole(opts *bind.TransactOpts, role [32]byte, account common.Address) (*types.Transaction, error) {
	return _IdentityRegistry.contract.Transact(opts, "revokeRole", role, account)
}

// RevokeRole is a paid mutator transaction binding the contract method 0xd547741f.
//
// Solidity: function revokeRole(bytes32 role, address account) returns()
func (_IdentityRegistry *IdentityRegistrySession) RevokeRole(role [32]byte, account common.Address) (*types.Transaction, error) {
	return _IdentityRegistry.Contract.RevokeRole(&_IdentityRegistry.TransactOpts, role, account)
}

// RevokeRole is a paid mutator transaction binding the contract method 0xd547741f.
//
// Solidity: function revokeRole(bytes32 role, address account) returns()
func (_IdentityRegistry *IdentityRegistryTransactorSession) RevokeRole(role [32]byte, account common.Address) (*types.Transaction, error) {
	return _IdentityRegistry.Contract.RevokeRole(&_IdentityRegistry.TransactOpts, role, account)
}

// SetCentralBankOf is a paid mutator transaction binding the contract method 0xf797b54c.
//
// Solidity: function setCentralBankOf(address token, address centralBank) returns()
func (_IdentityRegistry *IdentityRegistryTransactor) SetCentralBankOf(opts *bind.TransactOpts, token common.Address, centralBank common.Address) (*types.Transaction, error) {
	return _IdentityRegistry.contract.Transact(opts, "setCentralBankOf", token, centralBank)
}

// SetCentralBankOf is a paid mutator transaction binding the contract method 0xf797b54c.
//
// Solidity: function setCentralBankOf(address token, address centralBank) returns()
func (_IdentityRegistry *IdentityRegistrySession) SetCentralBankOf(token common.Address, centralBank common.Address) (*types.Transaction, error) {
	return _IdentityRegistry.Contract.SetCentralBankOf(&_IdentityRegistry.TransactOpts, token, centralBank)
}

// SetCentralBankOf is a paid mutator transaction binding the contract method 0xf797b54c.
//
// Solidity: function setCentralBankOf(address token, address centralBank) returns()
func (_IdentityRegistry *IdentityRegistryTransactorSession) SetCentralBankOf(token common.Address, centralBank common.Address) (*types.Transaction, error) {
	return _IdentityRegistry.Contract.SetCentralBankOf(&_IdentityRegistry.TransactOpts, token, centralBank)
}

// SetCertFingerprint is a paid mutator transaction binding the contract method 0xd9d1e737.
//
// Solidity: function setCertFingerprint(address account, bytes32 fingerprint) returns()
func (_IdentityRegistry *IdentityRegistryTransactor) SetCertFingerprint(opts *bind.TransactOpts, account common.Address, fingerprint [32]byte) (*types.Transaction, error) {
	return _IdentityRegistry.contract.Transact(opts, "setCertFingerprint", account, fingerprint)
}

// SetCertFingerprint is a paid mutator transaction binding the contract method 0xd9d1e737.
//
// Solidity: function setCertFingerprint(address account, bytes32 fingerprint) returns()
func (_IdentityRegistry *IdentityRegistrySession) SetCertFingerprint(account common.Address, fingerprint [32]byte) (*types.Transaction, error) {
	return _IdentityRegistry.Contract.SetCertFingerprint(&_IdentityRegistry.TransactOpts, account, fingerprint)
}

// SetCertFingerprint is a paid mutator transaction binding the contract method 0xd9d1e737.
//
// Solidity: function setCertFingerprint(address account, bytes32 fingerprint) returns()
func (_IdentityRegistry *IdentityRegistryTransactorSession) SetCertFingerprint(account common.Address, fingerprint [32]byte) (*types.Transaction, error) {
	return _IdentityRegistry.Contract.SetCertFingerprint(&_IdentityRegistry.TransactOpts, account, fingerprint)
}

// UpdateStatus is a paid mutator transaction binding the contract method 0x44c5bbf8.
//
// Solidity: function updateStatus(address account, uint8 newStatus) returns()
func (_IdentityRegistry *IdentityRegistryTransactor) UpdateStatus(opts *bind.TransactOpts, account common.Address, newStatus uint8) (*types.Transaction, error) {
	return _IdentityRegistry.contract.Transact(opts, "updateStatus", account, newStatus)
}

// UpdateStatus is a paid mutator transaction binding the contract method 0x44c5bbf8.
//
// Solidity: function updateStatus(address account, uint8 newStatus) returns()
func (_IdentityRegistry *IdentityRegistrySession) UpdateStatus(account common.Address, newStatus uint8) (*types.Transaction, error) {
	return _IdentityRegistry.Contract.UpdateStatus(&_IdentityRegistry.TransactOpts, account, newStatus)
}

// UpdateStatus is a paid mutator transaction binding the contract method 0x44c5bbf8.
//
// Solidity: function updateStatus(address account, uint8 newStatus) returns()
func (_IdentityRegistry *IdentityRegistryTransactorSession) UpdateStatus(account common.Address, newStatus uint8) (*types.Transaction, error) {
	return _IdentityRegistry.Contract.UpdateStatus(&_IdentityRegistry.TransactOpts, account, newStatus)
}

// VerifyParticipant is a paid mutator transaction binding the contract method 0x643a7695.
//
// Solidity: function verifyParticipant(address account) returns()
func (_IdentityRegistry *IdentityRegistryTransactor) VerifyParticipant(opts *bind.TransactOpts, account common.Address) (*types.Transaction, error) {
	return _IdentityRegistry.contract.Transact(opts, "verifyParticipant", account)
}

// VerifyParticipant is a paid mutator transaction binding the contract method 0x643a7695.
//
// Solidity: function verifyParticipant(address account) returns()
func (_IdentityRegistry *IdentityRegistrySession) VerifyParticipant(account common.Address) (*types.Transaction, error) {
	return _IdentityRegistry.Contract.VerifyParticipant(&_IdentityRegistry.TransactOpts, account)
}

// VerifyParticipant is a paid mutator transaction binding the contract method 0x643a7695.
//
// Solidity: function verifyParticipant(address account) returns()
func (_IdentityRegistry *IdentityRegistryTransactorSession) VerifyParticipant(account common.Address) (*types.Transaction, error) {
	return _IdentityRegistry.Contract.VerifyParticipant(&_IdentityRegistry.TransactOpts, account)
}

// IdentityRegistryCertificateRegisteredIterator is returned from FilterCertificateRegistered and is used to iterate over the raw logs and unpacked data for CertificateRegistered events raised by the IdentityRegistry contract.
type IdentityRegistryCertificateRegisteredIterator struct {
	Event *IdentityRegistryCertificateRegistered // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *IdentityRegistryCertificateRegisteredIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(IdentityRegistryCertificateRegistered)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(IdentityRegistryCertificateRegistered)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *IdentityRegistryCertificateRegisteredIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *IdentityRegistryCertificateRegisteredIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// IdentityRegistryCertificateRegistered represents a CertificateRegistered event raised by the IdentityRegistry contract.
type IdentityRegistryCertificateRegistered struct {
	Account         common.Address
	CertFingerprint [32]byte
	Raw             types.Log // Blockchain specific contextual infos
}

// FilterCertificateRegistered is a free log retrieval operation binding the contract event 0xb31a4bfd64a264737ab92a586033af794ce521bd393418cf0e31c8c587f10eb4.
//
// Solidity: event CertificateRegistered(address indexed account, bytes32 certFingerprint)
func (_IdentityRegistry *IdentityRegistryFilterer) FilterCertificateRegistered(opts *bind.FilterOpts, account []common.Address) (*IdentityRegistryCertificateRegisteredIterator, error) {

	var accountRule []interface{}
	for _, accountItem := range account {
		accountRule = append(accountRule, accountItem)
	}

	logs, sub, err := _IdentityRegistry.contract.FilterLogs(opts, "CertificateRegistered", accountRule)
	if err != nil {
		return nil, err
	}
	return &IdentityRegistryCertificateRegisteredIterator{contract: _IdentityRegistry.contract, event: "CertificateRegistered", logs: logs, sub: sub}, nil
}

// WatchCertificateRegistered is a free log subscription operation binding the contract event 0xb31a4bfd64a264737ab92a586033af794ce521bd393418cf0e31c8c587f10eb4.
//
// Solidity: event CertificateRegistered(address indexed account, bytes32 certFingerprint)
func (_IdentityRegistry *IdentityRegistryFilterer) WatchCertificateRegistered(opts *bind.WatchOpts, sink chan<- *IdentityRegistryCertificateRegistered, account []common.Address) (event.Subscription, error) {

	var accountRule []interface{}
	for _, accountItem := range account {
		accountRule = append(accountRule, accountItem)
	}

	logs, sub, err := _IdentityRegistry.contract.WatchLogs(opts, "CertificateRegistered", accountRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(IdentityRegistryCertificateRegistered)
				if err := _IdentityRegistry.contract.UnpackLog(event, "CertificateRegistered", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseCertificateRegistered is a log parse operation binding the contract event 0xb31a4bfd64a264737ab92a586033af794ce521bd393418cf0e31c8c587f10eb4.
//
// Solidity: event CertificateRegistered(address indexed account, bytes32 certFingerprint)
func (_IdentityRegistry *IdentityRegistryFilterer) ParseCertificateRegistered(log types.Log) (*IdentityRegistryCertificateRegistered, error) {
	event := new(IdentityRegistryCertificateRegistered)
	if err := _IdentityRegistry.contract.UnpackLog(event, "CertificateRegistered", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// IdentityRegistryIdentityUpdatedIterator is returned from FilterIdentityUpdated and is used to iterate over the raw logs and unpacked data for IdentityUpdated events raised by the IdentityRegistry contract.
type IdentityRegistryIdentityUpdatedIterator struct {
	Event *IdentityRegistryIdentityUpdated // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *IdentityRegistryIdentityUpdatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(IdentityRegistryIdentityUpdated)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(IdentityRegistryIdentityUpdated)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *IdentityRegistryIdentityUpdatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *IdentityRegistryIdentityUpdatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// IdentityRegistryIdentityUpdated represents a IdentityUpdated event raised by the IdentityRegistry contract.
type IdentityRegistryIdentityUpdated struct {
	Account   common.Address
	OldStatus uint8
	NewStatus uint8
	Raw       types.Log // Blockchain specific contextual infos
}

// FilterIdentityUpdated is a free log retrieval operation binding the contract event 0x9d46c17d71fcfb346dacb44bc99f4c6fde18f5fca70644e69a2177dcda928ca0.
//
// Solidity: event IdentityUpdated(address indexed account, uint8 oldStatus, uint8 newStatus)
func (_IdentityRegistry *IdentityRegistryFilterer) FilterIdentityUpdated(opts *bind.FilterOpts, account []common.Address) (*IdentityRegistryIdentityUpdatedIterator, error) {

	var accountRule []interface{}
	for _, accountItem := range account {
		accountRule = append(accountRule, accountItem)
	}

	logs, sub, err := _IdentityRegistry.contract.FilterLogs(opts, "IdentityUpdated", accountRule)
	if err != nil {
		return nil, err
	}
	return &IdentityRegistryIdentityUpdatedIterator{contract: _IdentityRegistry.contract, event: "IdentityUpdated", logs: logs, sub: sub}, nil
}

// WatchIdentityUpdated is a free log subscription operation binding the contract event 0x9d46c17d71fcfb346dacb44bc99f4c6fde18f5fca70644e69a2177dcda928ca0.
//
// Solidity: event IdentityUpdated(address indexed account, uint8 oldStatus, uint8 newStatus)
func (_IdentityRegistry *IdentityRegistryFilterer) WatchIdentityUpdated(opts *bind.WatchOpts, sink chan<- *IdentityRegistryIdentityUpdated, account []common.Address) (event.Subscription, error) {

	var accountRule []interface{}
	for _, accountItem := range account {
		accountRule = append(accountRule, accountItem)
	}

	logs, sub, err := _IdentityRegistry.contract.WatchLogs(opts, "IdentityUpdated", accountRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(IdentityRegistryIdentityUpdated)
				if err := _IdentityRegistry.contract.UnpackLog(event, "IdentityUpdated", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseIdentityUpdated is a log parse operation binding the contract event 0x9d46c17d71fcfb346dacb44bc99f4c6fde18f5fca70644e69a2177dcda928ca0.
//
// Solidity: event IdentityUpdated(address indexed account, uint8 oldStatus, uint8 newStatus)
func (_IdentityRegistry *IdentityRegistryFilterer) ParseIdentityUpdated(log types.Log) (*IdentityRegistryIdentityUpdated, error) {
	event := new(IdentityRegistryIdentityUpdated)
	if err := _IdentityRegistry.contract.UnpackLog(event, "IdentityUpdated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// IdentityRegistryLogCentralBankOfTokenSetIterator is returned from FilterLogCentralBankOfTokenSet and is used to iterate over the raw logs and unpacked data for LogCentralBankOfTokenSet events raised by the IdentityRegistry contract.
type IdentityRegistryLogCentralBankOfTokenSetIterator struct {
	Event *IdentityRegistryLogCentralBankOfTokenSet // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *IdentityRegistryLogCentralBankOfTokenSetIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(IdentityRegistryLogCentralBankOfTokenSet)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(IdentityRegistryLogCentralBankOfTokenSet)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *IdentityRegistryLogCentralBankOfTokenSetIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *IdentityRegistryLogCentralBankOfTokenSetIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// IdentityRegistryLogCentralBankOfTokenSet represents a LogCentralBankOfTokenSet event raised by the IdentityRegistry contract.
type IdentityRegistryLogCentralBankOfTokenSet struct {
	Token       common.Address
	CentralBank common.Address
	Raw         types.Log // Blockchain specific contextual infos
}

// FilterLogCentralBankOfTokenSet is a free log retrieval operation binding the contract event 0x37abf8fcdf4a6901160322939355317c9dfc995ee6dfb43e8196a42bb252b41a.
//
// Solidity: event LogCentralBankOfTokenSet(address indexed token, address indexed centralBank)
func (_IdentityRegistry *IdentityRegistryFilterer) FilterLogCentralBankOfTokenSet(opts *bind.FilterOpts, token []common.Address, centralBank []common.Address) (*IdentityRegistryLogCentralBankOfTokenSetIterator, error) {

	var tokenRule []interface{}
	for _, tokenItem := range token {
		tokenRule = append(tokenRule, tokenItem)
	}
	var centralBankRule []interface{}
	for _, centralBankItem := range centralBank {
		centralBankRule = append(centralBankRule, centralBankItem)
	}

	logs, sub, err := _IdentityRegistry.contract.FilterLogs(opts, "LogCentralBankOfTokenSet", tokenRule, centralBankRule)
	if err != nil {
		return nil, err
	}
	return &IdentityRegistryLogCentralBankOfTokenSetIterator{contract: _IdentityRegistry.contract, event: "LogCentralBankOfTokenSet", logs: logs, sub: sub}, nil
}

// WatchLogCentralBankOfTokenSet is a free log subscription operation binding the contract event 0x37abf8fcdf4a6901160322939355317c9dfc995ee6dfb43e8196a42bb252b41a.
//
// Solidity: event LogCentralBankOfTokenSet(address indexed token, address indexed centralBank)
func (_IdentityRegistry *IdentityRegistryFilterer) WatchLogCentralBankOfTokenSet(opts *bind.WatchOpts, sink chan<- *IdentityRegistryLogCentralBankOfTokenSet, token []common.Address, centralBank []common.Address) (event.Subscription, error) {

	var tokenRule []interface{}
	for _, tokenItem := range token {
		tokenRule = append(tokenRule, tokenItem)
	}
	var centralBankRule []interface{}
	for _, centralBankItem := range centralBank {
		centralBankRule = append(centralBankRule, centralBankItem)
	}

	logs, sub, err := _IdentityRegistry.contract.WatchLogs(opts, "LogCentralBankOfTokenSet", tokenRule, centralBankRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(IdentityRegistryLogCentralBankOfTokenSet)
				if err := _IdentityRegistry.contract.UnpackLog(event, "LogCentralBankOfTokenSet", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseLogCentralBankOfTokenSet is a log parse operation binding the contract event 0x37abf8fcdf4a6901160322939355317c9dfc995ee6dfb43e8196a42bb252b41a.
//
// Solidity: event LogCentralBankOfTokenSet(address indexed token, address indexed centralBank)
func (_IdentityRegistry *IdentityRegistryFilterer) ParseLogCentralBankOfTokenSet(log types.Log) (*IdentityRegistryLogCentralBankOfTokenSet, error) {
	event := new(IdentityRegistryLogCentralBankOfTokenSet)
	if err := _IdentityRegistry.contract.UnpackLog(event, "LogCentralBankOfTokenSet", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// IdentityRegistryLogLiquidityProviderGrantedIterator is returned from FilterLogLiquidityProviderGranted and is used to iterate over the raw logs and unpacked data for LogLiquidityProviderGranted events raised by the IdentityRegistry contract.
type IdentityRegistryLogLiquidityProviderGrantedIterator struct {
	Event *IdentityRegistryLogLiquidityProviderGranted // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *IdentityRegistryLogLiquidityProviderGrantedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(IdentityRegistryLogLiquidityProviderGranted)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(IdentityRegistryLogLiquidityProviderGranted)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *IdentityRegistryLogLiquidityProviderGrantedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *IdentityRegistryLogLiquidityProviderGrantedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// IdentityRegistryLogLiquidityProviderGranted represents a LogLiquidityProviderGranted event raised by the IdentityRegistry contract.
type IdentityRegistryLogLiquidityProviderGranted struct {
	Account common.Address
	Raw     types.Log // Blockchain specific contextual infos
}

// FilterLogLiquidityProviderGranted is a free log retrieval operation binding the contract event 0x6575ca56ccddde4cd0dc0bebde475e383fac0713d1d87332bbcd254112adffda.
//
// Solidity: event LogLiquidityProviderGranted(address indexed account)
func (_IdentityRegistry *IdentityRegistryFilterer) FilterLogLiquidityProviderGranted(opts *bind.FilterOpts, account []common.Address) (*IdentityRegistryLogLiquidityProviderGrantedIterator, error) {

	var accountRule []interface{}
	for _, accountItem := range account {
		accountRule = append(accountRule, accountItem)
	}

	logs, sub, err := _IdentityRegistry.contract.FilterLogs(opts, "LogLiquidityProviderGranted", accountRule)
	if err != nil {
		return nil, err
	}
	return &IdentityRegistryLogLiquidityProviderGrantedIterator{contract: _IdentityRegistry.contract, event: "LogLiquidityProviderGranted", logs: logs, sub: sub}, nil
}

// WatchLogLiquidityProviderGranted is a free log subscription operation binding the contract event 0x6575ca56ccddde4cd0dc0bebde475e383fac0713d1d87332bbcd254112adffda.
//
// Solidity: event LogLiquidityProviderGranted(address indexed account)
func (_IdentityRegistry *IdentityRegistryFilterer) WatchLogLiquidityProviderGranted(opts *bind.WatchOpts, sink chan<- *IdentityRegistryLogLiquidityProviderGranted, account []common.Address) (event.Subscription, error) {

	var accountRule []interface{}
	for _, accountItem := range account {
		accountRule = append(accountRule, accountItem)
	}

	logs, sub, err := _IdentityRegistry.contract.WatchLogs(opts, "LogLiquidityProviderGranted", accountRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(IdentityRegistryLogLiquidityProviderGranted)
				if err := _IdentityRegistry.contract.UnpackLog(event, "LogLiquidityProviderGranted", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseLogLiquidityProviderGranted is a log parse operation binding the contract event 0x6575ca56ccddde4cd0dc0bebde475e383fac0713d1d87332bbcd254112adffda.
//
// Solidity: event LogLiquidityProviderGranted(address indexed account)
func (_IdentityRegistry *IdentityRegistryFilterer) ParseLogLiquidityProviderGranted(log types.Log) (*IdentityRegistryLogLiquidityProviderGranted, error) {
	event := new(IdentityRegistryLogLiquidityProviderGranted)
	if err := _IdentityRegistry.contract.UnpackLog(event, "LogLiquidityProviderGranted", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// IdentityRegistryLogLiquidityProviderRevokedIterator is returned from FilterLogLiquidityProviderRevoked and is used to iterate over the raw logs and unpacked data for LogLiquidityProviderRevoked events raised by the IdentityRegistry contract.
type IdentityRegistryLogLiquidityProviderRevokedIterator struct {
	Event *IdentityRegistryLogLiquidityProviderRevoked // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *IdentityRegistryLogLiquidityProviderRevokedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(IdentityRegistryLogLiquidityProviderRevoked)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(IdentityRegistryLogLiquidityProviderRevoked)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *IdentityRegistryLogLiquidityProviderRevokedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *IdentityRegistryLogLiquidityProviderRevokedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// IdentityRegistryLogLiquidityProviderRevoked represents a LogLiquidityProviderRevoked event raised by the IdentityRegistry contract.
type IdentityRegistryLogLiquidityProviderRevoked struct {
	Account common.Address
	Raw     types.Log // Blockchain specific contextual infos
}

// FilterLogLiquidityProviderRevoked is a free log retrieval operation binding the contract event 0x69c93a581704e7fbbbf4f46952f8c65e0c091ae422af84bd0bbcc8eaf8584493.
//
// Solidity: event LogLiquidityProviderRevoked(address indexed account)
func (_IdentityRegistry *IdentityRegistryFilterer) FilterLogLiquidityProviderRevoked(opts *bind.FilterOpts, account []common.Address) (*IdentityRegistryLogLiquidityProviderRevokedIterator, error) {

	var accountRule []interface{}
	for _, accountItem := range account {
		accountRule = append(accountRule, accountItem)
	}

	logs, sub, err := _IdentityRegistry.contract.FilterLogs(opts, "LogLiquidityProviderRevoked", accountRule)
	if err != nil {
		return nil, err
	}
	return &IdentityRegistryLogLiquidityProviderRevokedIterator{contract: _IdentityRegistry.contract, event: "LogLiquidityProviderRevoked", logs: logs, sub: sub}, nil
}

// WatchLogLiquidityProviderRevoked is a free log subscription operation binding the contract event 0x69c93a581704e7fbbbf4f46952f8c65e0c091ae422af84bd0bbcc8eaf8584493.
//
// Solidity: event LogLiquidityProviderRevoked(address indexed account)
func (_IdentityRegistry *IdentityRegistryFilterer) WatchLogLiquidityProviderRevoked(opts *bind.WatchOpts, sink chan<- *IdentityRegistryLogLiquidityProviderRevoked, account []common.Address) (event.Subscription, error) {

	var accountRule []interface{}
	for _, accountItem := range account {
		accountRule = append(accountRule, accountItem)
	}

	logs, sub, err := _IdentityRegistry.contract.WatchLogs(opts, "LogLiquidityProviderRevoked", accountRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(IdentityRegistryLogLiquidityProviderRevoked)
				if err := _IdentityRegistry.contract.UnpackLog(event, "LogLiquidityProviderRevoked", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseLogLiquidityProviderRevoked is a log parse operation binding the contract event 0x69c93a581704e7fbbbf4f46952f8c65e0c091ae422af84bd0bbcc8eaf8584493.
//
// Solidity: event LogLiquidityProviderRevoked(address indexed account)
func (_IdentityRegistry *IdentityRegistryFilterer) ParseLogLiquidityProviderRevoked(log types.Log) (*IdentityRegistryLogLiquidityProviderRevoked, error) {
	event := new(IdentityRegistryLogLiquidityProviderRevoked)
	if err := _IdentityRegistry.contract.UnpackLog(event, "LogLiquidityProviderRevoked", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// IdentityRegistryParticipantRegisteredIterator is returned from FilterParticipantRegistered and is used to iterate over the raw logs and unpacked data for ParticipantRegistered events raised by the IdentityRegistry contract.
type IdentityRegistryParticipantRegisteredIterator struct {
	Event *IdentityRegistryParticipantRegistered // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *IdentityRegistryParticipantRegisteredIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(IdentityRegistryParticipantRegistered)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(IdentityRegistryParticipantRegistered)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *IdentityRegistryParticipantRegisteredIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *IdentityRegistryParticipantRegisteredIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// IdentityRegistryParticipantRegistered represents a ParticipantRegistered event raised by the IdentityRegistry contract.
type IdentityRegistryParticipantRegistered struct {
	Account common.Address
	Role    uint8
	Name    string
	Raw     types.Log // Blockchain specific contextual infos
}

// FilterParticipantRegistered is a free log retrieval operation binding the contract event 0xe3564c470dc79d88d220de1a64bc335ec0a928450ce631293691bba84dd9a611.
//
// Solidity: event ParticipantRegistered(address indexed account, uint8 role, string name)
func (_IdentityRegistry *IdentityRegistryFilterer) FilterParticipantRegistered(opts *bind.FilterOpts, account []common.Address) (*IdentityRegistryParticipantRegisteredIterator, error) {

	var accountRule []interface{}
	for _, accountItem := range account {
		accountRule = append(accountRule, accountItem)
	}

	logs, sub, err := _IdentityRegistry.contract.FilterLogs(opts, "ParticipantRegistered", accountRule)
	if err != nil {
		return nil, err
	}
	return &IdentityRegistryParticipantRegisteredIterator{contract: _IdentityRegistry.contract, event: "ParticipantRegistered", logs: logs, sub: sub}, nil
}

// WatchParticipantRegistered is a free log subscription operation binding the contract event 0xe3564c470dc79d88d220de1a64bc335ec0a928450ce631293691bba84dd9a611.
//
// Solidity: event ParticipantRegistered(address indexed account, uint8 role, string name)
func (_IdentityRegistry *IdentityRegistryFilterer) WatchParticipantRegistered(opts *bind.WatchOpts, sink chan<- *IdentityRegistryParticipantRegistered, account []common.Address) (event.Subscription, error) {

	var accountRule []interface{}
	for _, accountItem := range account {
		accountRule = append(accountRule, accountItem)
	}

	logs, sub, err := _IdentityRegistry.contract.WatchLogs(opts, "ParticipantRegistered", accountRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(IdentityRegistryParticipantRegistered)
				if err := _IdentityRegistry.contract.UnpackLog(event, "ParticipantRegistered", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseParticipantRegistered is a log parse operation binding the contract event 0xe3564c470dc79d88d220de1a64bc335ec0a928450ce631293691bba84dd9a611.
//
// Solidity: event ParticipantRegistered(address indexed account, uint8 role, string name)
func (_IdentityRegistry *IdentityRegistryFilterer) ParseParticipantRegistered(log types.Log) (*IdentityRegistryParticipantRegistered, error) {
	event := new(IdentityRegistryParticipantRegistered)
	if err := _IdentityRegistry.contract.UnpackLog(event, "ParticipantRegistered", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// IdentityRegistryRoleAdminChangedIterator is returned from FilterRoleAdminChanged and is used to iterate over the raw logs and unpacked data for RoleAdminChanged events raised by the IdentityRegistry contract.
type IdentityRegistryRoleAdminChangedIterator struct {
	Event *IdentityRegistryRoleAdminChanged // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *IdentityRegistryRoleAdminChangedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(IdentityRegistryRoleAdminChanged)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(IdentityRegistryRoleAdminChanged)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *IdentityRegistryRoleAdminChangedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *IdentityRegistryRoleAdminChangedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// IdentityRegistryRoleAdminChanged represents a RoleAdminChanged event raised by the IdentityRegistry contract.
type IdentityRegistryRoleAdminChanged struct {
	Role              [32]byte
	PreviousAdminRole [32]byte
	NewAdminRole      [32]byte
	Raw               types.Log // Blockchain specific contextual infos
}

// FilterRoleAdminChanged is a free log retrieval operation binding the contract event 0xbd79b86ffe0ab8e8776151514217cd7cacd52c909f66475c3af44e129f0b00ff.
//
// Solidity: event RoleAdminChanged(bytes32 indexed role, bytes32 indexed previousAdminRole, bytes32 indexed newAdminRole)
func (_IdentityRegistry *IdentityRegistryFilterer) FilterRoleAdminChanged(opts *bind.FilterOpts, role [][32]byte, previousAdminRole [][32]byte, newAdminRole [][32]byte) (*IdentityRegistryRoleAdminChangedIterator, error) {

	var roleRule []interface{}
	for _, roleItem := range role {
		roleRule = append(roleRule, roleItem)
	}
	var previousAdminRoleRule []interface{}
	for _, previousAdminRoleItem := range previousAdminRole {
		previousAdminRoleRule = append(previousAdminRoleRule, previousAdminRoleItem)
	}
	var newAdminRoleRule []interface{}
	for _, newAdminRoleItem := range newAdminRole {
		newAdminRoleRule = append(newAdminRoleRule, newAdminRoleItem)
	}

	logs, sub, err := _IdentityRegistry.contract.FilterLogs(opts, "RoleAdminChanged", roleRule, previousAdminRoleRule, newAdminRoleRule)
	if err != nil {
		return nil, err
	}
	return &IdentityRegistryRoleAdminChangedIterator{contract: _IdentityRegistry.contract, event: "RoleAdminChanged", logs: logs, sub: sub}, nil
}

// WatchRoleAdminChanged is a free log subscription operation binding the contract event 0xbd79b86ffe0ab8e8776151514217cd7cacd52c909f66475c3af44e129f0b00ff.
//
// Solidity: event RoleAdminChanged(bytes32 indexed role, bytes32 indexed previousAdminRole, bytes32 indexed newAdminRole)
func (_IdentityRegistry *IdentityRegistryFilterer) WatchRoleAdminChanged(opts *bind.WatchOpts, sink chan<- *IdentityRegistryRoleAdminChanged, role [][32]byte, previousAdminRole [][32]byte, newAdminRole [][32]byte) (event.Subscription, error) {

	var roleRule []interface{}
	for _, roleItem := range role {
		roleRule = append(roleRule, roleItem)
	}
	var previousAdminRoleRule []interface{}
	for _, previousAdminRoleItem := range previousAdminRole {
		previousAdminRoleRule = append(previousAdminRoleRule, previousAdminRoleItem)
	}
	var newAdminRoleRule []interface{}
	for _, newAdminRoleItem := range newAdminRole {
		newAdminRoleRule = append(newAdminRoleRule, newAdminRoleItem)
	}

	logs, sub, err := _IdentityRegistry.contract.WatchLogs(opts, "RoleAdminChanged", roleRule, previousAdminRoleRule, newAdminRoleRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(IdentityRegistryRoleAdminChanged)
				if err := _IdentityRegistry.contract.UnpackLog(event, "RoleAdminChanged", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseRoleAdminChanged is a log parse operation binding the contract event 0xbd79b86ffe0ab8e8776151514217cd7cacd52c909f66475c3af44e129f0b00ff.
//
// Solidity: event RoleAdminChanged(bytes32 indexed role, bytes32 indexed previousAdminRole, bytes32 indexed newAdminRole)
func (_IdentityRegistry *IdentityRegistryFilterer) ParseRoleAdminChanged(log types.Log) (*IdentityRegistryRoleAdminChanged, error) {
	event := new(IdentityRegistryRoleAdminChanged)
	if err := _IdentityRegistry.contract.UnpackLog(event, "RoleAdminChanged", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// IdentityRegistryRoleGrantedIterator is returned from FilterRoleGranted and is used to iterate over the raw logs and unpacked data for RoleGranted events raised by the IdentityRegistry contract.
type IdentityRegistryRoleGrantedIterator struct {
	Event *IdentityRegistryRoleGranted // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *IdentityRegistryRoleGrantedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(IdentityRegistryRoleGranted)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(IdentityRegistryRoleGranted)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *IdentityRegistryRoleGrantedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *IdentityRegistryRoleGrantedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// IdentityRegistryRoleGranted represents a RoleGranted event raised by the IdentityRegistry contract.
type IdentityRegistryRoleGranted struct {
	Role    [32]byte
	Account common.Address
	Sender  common.Address
	Raw     types.Log // Blockchain specific contextual infos
}

// FilterRoleGranted is a free log retrieval operation binding the contract event 0x2f8788117e7eff1d82e926ec794901d17c78024a50270940304540a733656f0d.
//
// Solidity: event RoleGranted(bytes32 indexed role, address indexed account, address indexed sender)
func (_IdentityRegistry *IdentityRegistryFilterer) FilterRoleGranted(opts *bind.FilterOpts, role [][32]byte, account []common.Address, sender []common.Address) (*IdentityRegistryRoleGrantedIterator, error) {

	var roleRule []interface{}
	for _, roleItem := range role {
		roleRule = append(roleRule, roleItem)
	}
	var accountRule []interface{}
	for _, accountItem := range account {
		accountRule = append(accountRule, accountItem)
	}
	var senderRule []interface{}
	for _, senderItem := range sender {
		senderRule = append(senderRule, senderItem)
	}

	logs, sub, err := _IdentityRegistry.contract.FilterLogs(opts, "RoleGranted", roleRule, accountRule, senderRule)
	if err != nil {
		return nil, err
	}
	return &IdentityRegistryRoleGrantedIterator{contract: _IdentityRegistry.contract, event: "RoleGranted", logs: logs, sub: sub}, nil
}

// WatchRoleGranted is a free log subscription operation binding the contract event 0x2f8788117e7eff1d82e926ec794901d17c78024a50270940304540a733656f0d.
//
// Solidity: event RoleGranted(bytes32 indexed role, address indexed account, address indexed sender)
func (_IdentityRegistry *IdentityRegistryFilterer) WatchRoleGranted(opts *bind.WatchOpts, sink chan<- *IdentityRegistryRoleGranted, role [][32]byte, account []common.Address, sender []common.Address) (event.Subscription, error) {

	var roleRule []interface{}
	for _, roleItem := range role {
		roleRule = append(roleRule, roleItem)
	}
	var accountRule []interface{}
	for _, accountItem := range account {
		accountRule = append(accountRule, accountItem)
	}
	var senderRule []interface{}
	for _, senderItem := range sender {
		senderRule = append(senderRule, senderItem)
	}

	logs, sub, err := _IdentityRegistry.contract.WatchLogs(opts, "RoleGranted", roleRule, accountRule, senderRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(IdentityRegistryRoleGranted)
				if err := _IdentityRegistry.contract.UnpackLog(event, "RoleGranted", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseRoleGranted is a log parse operation binding the contract event 0x2f8788117e7eff1d82e926ec794901d17c78024a50270940304540a733656f0d.
//
// Solidity: event RoleGranted(bytes32 indexed role, address indexed account, address indexed sender)
func (_IdentityRegistry *IdentityRegistryFilterer) ParseRoleGranted(log types.Log) (*IdentityRegistryRoleGranted, error) {
	event := new(IdentityRegistryRoleGranted)
	if err := _IdentityRegistry.contract.UnpackLog(event, "RoleGranted", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// IdentityRegistryRoleRevokedIterator is returned from FilterRoleRevoked and is used to iterate over the raw logs and unpacked data for RoleRevoked events raised by the IdentityRegistry contract.
type IdentityRegistryRoleRevokedIterator struct {
	Event *IdentityRegistryRoleRevoked // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *IdentityRegistryRoleRevokedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(IdentityRegistryRoleRevoked)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(IdentityRegistryRoleRevoked)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *IdentityRegistryRoleRevokedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *IdentityRegistryRoleRevokedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// IdentityRegistryRoleRevoked represents a RoleRevoked event raised by the IdentityRegistry contract.
type IdentityRegistryRoleRevoked struct {
	Role    [32]byte
	Account common.Address
	Sender  common.Address
	Raw     types.Log // Blockchain specific contextual infos
}

// FilterRoleRevoked is a free log retrieval operation binding the contract event 0xf6391f5c32d9c69d2a47ea670b442974b53935d1edc7fd64eb21e047a839171b.
//
// Solidity: event RoleRevoked(bytes32 indexed role, address indexed account, address indexed sender)
func (_IdentityRegistry *IdentityRegistryFilterer) FilterRoleRevoked(opts *bind.FilterOpts, role [][32]byte, account []common.Address, sender []common.Address) (*IdentityRegistryRoleRevokedIterator, error) {

	var roleRule []interface{}
	for _, roleItem := range role {
		roleRule = append(roleRule, roleItem)
	}
	var accountRule []interface{}
	for _, accountItem := range account {
		accountRule = append(accountRule, accountItem)
	}
	var senderRule []interface{}
	for _, senderItem := range sender {
		senderRule = append(senderRule, senderItem)
	}

	logs, sub, err := _IdentityRegistry.contract.FilterLogs(opts, "RoleRevoked", roleRule, accountRule, senderRule)
	if err != nil {
		return nil, err
	}
	return &IdentityRegistryRoleRevokedIterator{contract: _IdentityRegistry.contract, event: "RoleRevoked", logs: logs, sub: sub}, nil
}

// WatchRoleRevoked is a free log subscription operation binding the contract event 0xf6391f5c32d9c69d2a47ea670b442974b53935d1edc7fd64eb21e047a839171b.
//
// Solidity: event RoleRevoked(bytes32 indexed role, address indexed account, address indexed sender)
func (_IdentityRegistry *IdentityRegistryFilterer) WatchRoleRevoked(opts *bind.WatchOpts, sink chan<- *IdentityRegistryRoleRevoked, role [][32]byte, account []common.Address, sender []common.Address) (event.Subscription, error) {

	var roleRule []interface{}
	for _, roleItem := range role {
		roleRule = append(roleRule, roleItem)
	}
	var accountRule []interface{}
	for _, accountItem := range account {
		accountRule = append(accountRule, accountItem)
	}
	var senderRule []interface{}
	for _, senderItem := range sender {
		senderRule = append(senderRule, senderItem)
	}

	logs, sub, err := _IdentityRegistry.contract.WatchLogs(opts, "RoleRevoked", roleRule, accountRule, senderRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(IdentityRegistryRoleRevoked)
				if err := _IdentityRegistry.contract.UnpackLog(event, "RoleRevoked", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseRoleRevoked is a log parse operation binding the contract event 0xf6391f5c32d9c69d2a47ea670b442974b53935d1edc7fd64eb21e047a839171b.
//
// Solidity: event RoleRevoked(bytes32 indexed role, address indexed account, address indexed sender)
func (_IdentityRegistry *IdentityRegistryFilterer) ParseRoleRevoked(log types.Log) (*IdentityRegistryRoleRevoked, error) {
	event := new(IdentityRegistryRoleRevoked)
	if err := _IdentityRegistry.contract.UnpackLog(event, "RoleRevoked", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}
