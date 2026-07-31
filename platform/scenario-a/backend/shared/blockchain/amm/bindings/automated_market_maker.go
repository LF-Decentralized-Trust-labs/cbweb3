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

// AutomatedMarketMakerMetaData contains all meta data concerning the AutomatedMarketMaker contract.
var AutomatedMarketMakerMetaData = &bind.MetaData{
	ABI: "[{\"type\":\"constructor\",\"inputs\":[{\"name\":\"_tokenA\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"_tokenB\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"_identityRegistry\",\"type\":\"address\",\"internalType\":\"address\"}],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"DEFAULT_FEE_BPS\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"IDENTITY_REGISTRY\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"address\",\"internalType\":\"contractIIdentityRegistry\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"MAX_FEE_BPS\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"RESUME_QUORUM\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"TOKEN_A\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"address\",\"internalType\":\"contractIERC20\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"TOKEN_B\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"address\",\"internalType\":\"contractIERC20\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"addLiquidity\",\"inputs\":[{\"name\":\"amountA\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"amountB\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"feeBps\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"getAmountIn\",\"inputs\":[{\"name\":\"reserveIn\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"reserveOut\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"amountOut\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[{\"name\":\"amountIn\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"pure\"},{\"type\":\"function\",\"name\":\"isPaused\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"bool\",\"internalType\":\"bool\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"pause\",\"inputs\":[{\"name\":\"reason\",\"type\":\"string\",\"internalType\":\"string\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"pauseEpoch\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"paused\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"bool\",\"internalType\":\"bool\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"proposeResume\",\"inputs\":[],\"outputs\":[{\"name\":\"proposalId\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"}],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"quoteExactOutput\",\"inputs\":[{\"name\":\"reserveIn\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"reserveOut\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"amountOut\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"feeBps_\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[{\"name\":\"amountIn\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"pure\"},{\"type\":\"function\",\"name\":\"reserveA\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"reserveB\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"resumeQuorum\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"pure\"},{\"type\":\"function\",\"name\":\"resumeSignatures\",\"inputs\":[{\"name\":\"proposalId\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"}],\"outputs\":[{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"setFeeBps\",\"inputs\":[{\"name\":\"newFeeBps\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"signResume\",\"inputs\":[{\"name\":\"proposalId\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"swapTokensForExactTokens\",\"inputs\":[{\"name\":\"tokenIn\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"tokenOut\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"amountOut\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"maxAmountIn\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"to\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[{\"name\":\"amountIn\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"nonpayable\"},{\"type\":\"event\",\"name\":\"LogCircuitBreakerPaused\",\"inputs\":[{\"name\":\"pausedBy\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"timestamp\",\"type\":\"uint256\",\"indexed\":false,\"internalType\":\"uint256\"},{\"name\":\"reason\",\"type\":\"string\",\"indexed\":false,\"internalType\":\"string\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"LogCircuitBreakerResumed\",\"inputs\":[{\"name\":\"proposalId\",\"type\":\"bytes32\",\"indexed\":true,\"internalType\":\"bytes32\"},{\"name\":\"timestamp\",\"type\":\"uint256\",\"indexed\":false,\"internalType\":\"uint256\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"LogFeeRateUpdated\",\"inputs\":[{\"name\":\"oldFeeBps\",\"type\":\"uint256\",\"indexed\":false,\"internalType\":\"uint256\"},{\"name\":\"newFeeBps\",\"type\":\"uint256\",\"indexed\":false,\"internalType\":\"uint256\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"LogLiquidityAdded\",\"inputs\":[{\"name\":\"provider\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"amountTokenA\",\"type\":\"uint256\",\"indexed\":false,\"internalType\":\"uint256\"},{\"name\":\"amountTokenB\",\"type\":\"uint256\",\"indexed\":false,\"internalType\":\"uint256\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"LogResumeProposed\",\"inputs\":[{\"name\":\"proposalId\",\"type\":\"bytes32\",\"indexed\":true,\"internalType\":\"bytes32\"},{\"name\":\"proposedBy\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"timestamp\",\"type\":\"uint256\",\"indexed\":false,\"internalType\":\"uint256\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"LogResumeSigned\",\"inputs\":[{\"name\":\"proposalId\",\"type\":\"bytes32\",\"indexed\":true,\"internalType\":\"bytes32\"},{\"name\":\"signer\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"signaturesCount\",\"type\":\"uint256\",\"indexed\":false,\"internalType\":\"uint256\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"LogSwap\",\"inputs\":[{\"name\":\"user\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"tokenIn\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"tokenOut\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"amountIn\",\"type\":\"uint256\",\"indexed\":false,\"internalType\":\"uint256\"},{\"name\":\"amountOut\",\"type\":\"uint256\",\"indexed\":false,\"internalType\":\"uint256\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"Paused\",\"inputs\":[{\"name\":\"account\",\"type\":\"address\",\"indexed\":false,\"internalType\":\"address\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"Unpaused\",\"inputs\":[{\"name\":\"account\",\"type\":\"address\",\"indexed\":false,\"internalType\":\"address\"}],\"anonymous\":false},{\"type\":\"error\",\"name\":\"AMM__AlreadySigned\",\"inputs\":[{\"name\":\"proposalId\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"},{\"name\":\"signer\",\"type\":\"address\",\"internalType\":\"address\"}]},{\"type\":\"error\",\"name\":\"AMM__FeeBpsTooHigh\",\"inputs\":[{\"name\":\"provided\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"max\",\"type\":\"uint256\",\"internalType\":\"uint256\"}]},{\"type\":\"error\",\"name\":\"AMM__InsufficientLiquidity\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"AMM__InsufficientOutputAmount\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"AMM__InvalidToken\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"AMM__NotGovernance\",\"inputs\":[{\"name\":\"account\",\"type\":\"address\",\"internalType\":\"address\"}]},{\"type\":\"error\",\"name\":\"AMM__ParticipantNotVerified\",\"inputs\":[{\"name\":\"account\",\"type\":\"address\",\"internalType\":\"address\"}]},{\"type\":\"error\",\"name\":\"AMM__ProposalExpired\",\"inputs\":[{\"name\":\"proposalId\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"},{\"name\":\"proposalEpoch\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"currentEpoch\",\"type\":\"uint256\",\"internalType\":\"uint256\"}]},{\"type\":\"error\",\"name\":\"AMM__ProposalNotFound\",\"inputs\":[{\"name\":\"proposalId\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"}]},{\"type\":\"error\",\"name\":\"AMM__SlippageExceeded\",\"inputs\":[{\"name\":\"requiredAmountIn\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"maxAmountIn\",\"type\":\"uint256\",\"internalType\":\"uint256\"}]},{\"type\":\"error\",\"name\":\"AMM__ZeroAddress\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"AMM__ZeroAmount\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"AddressEmptyCode\",\"inputs\":[{\"name\":\"target\",\"type\":\"address\",\"internalType\":\"address\"}]},{\"type\":\"error\",\"name\":\"AddressInsufficientBalance\",\"inputs\":[{\"name\":\"account\",\"type\":\"address\",\"internalType\":\"address\"}]},{\"type\":\"error\",\"name\":\"EnforcedPause\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"ExpectedPause\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"FailedInnerCall\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"MathOverflowedMulDiv\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"ReentrancyGuardReentrantCall\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"SafeERC20FailedOperation\",\"inputs\":[{\"name\":\"token\",\"type\":\"address\",\"internalType\":\"address\"}]}]",
}

// AutomatedMarketMakerABI is the input ABI used to generate the binding from.
// Deprecated: Use AutomatedMarketMakerMetaData.ABI instead.
var AutomatedMarketMakerABI = AutomatedMarketMakerMetaData.ABI

// AutomatedMarketMaker is an auto generated Go binding around an Ethereum contract.
type AutomatedMarketMaker struct {
	AutomatedMarketMakerCaller     // Read-only binding to the contract
	AutomatedMarketMakerTransactor // Write-only binding to the contract
	AutomatedMarketMakerFilterer   // Log filterer for contract events
}

// AutomatedMarketMakerCaller is an auto generated read-only Go binding around an Ethereum contract.
type AutomatedMarketMakerCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// AutomatedMarketMakerTransactor is an auto generated write-only Go binding around an Ethereum contract.
type AutomatedMarketMakerTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// AutomatedMarketMakerFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type AutomatedMarketMakerFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// AutomatedMarketMakerSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type AutomatedMarketMakerSession struct {
	Contract     *AutomatedMarketMaker // Generic contract binding to set the session for
	CallOpts     bind.CallOpts         // Call options to use throughout this session
	TransactOpts bind.TransactOpts     // Transaction auth options to use throughout this session
}

// AutomatedMarketMakerCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type AutomatedMarketMakerCallerSession struct {
	Contract *AutomatedMarketMakerCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts               // Call options to use throughout this session
}

// AutomatedMarketMakerTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type AutomatedMarketMakerTransactorSession struct {
	Contract     *AutomatedMarketMakerTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts               // Transaction auth options to use throughout this session
}

// AutomatedMarketMakerRaw is an auto generated low-level Go binding around an Ethereum contract.
type AutomatedMarketMakerRaw struct {
	Contract *AutomatedMarketMaker // Generic contract binding to access the raw methods on
}

// AutomatedMarketMakerCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type AutomatedMarketMakerCallerRaw struct {
	Contract *AutomatedMarketMakerCaller // Generic read-only contract binding to access the raw methods on
}

// AutomatedMarketMakerTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type AutomatedMarketMakerTransactorRaw struct {
	Contract *AutomatedMarketMakerTransactor // Generic write-only contract binding to access the raw methods on
}

// NewAutomatedMarketMaker creates a new instance of AutomatedMarketMaker, bound to a specific deployed contract.
func NewAutomatedMarketMaker(address common.Address, backend bind.ContractBackend) (*AutomatedMarketMaker, error) {
	contract, err := bindAutomatedMarketMaker(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &AutomatedMarketMaker{AutomatedMarketMakerCaller: AutomatedMarketMakerCaller{contract: contract}, AutomatedMarketMakerTransactor: AutomatedMarketMakerTransactor{contract: contract}, AutomatedMarketMakerFilterer: AutomatedMarketMakerFilterer{contract: contract}}, nil
}

// NewAutomatedMarketMakerCaller creates a new read-only instance of AutomatedMarketMaker, bound to a specific deployed contract.
func NewAutomatedMarketMakerCaller(address common.Address, caller bind.ContractCaller) (*AutomatedMarketMakerCaller, error) {
	contract, err := bindAutomatedMarketMaker(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &AutomatedMarketMakerCaller{contract: contract}, nil
}

// NewAutomatedMarketMakerTransactor creates a new write-only instance of AutomatedMarketMaker, bound to a specific deployed contract.
func NewAutomatedMarketMakerTransactor(address common.Address, transactor bind.ContractTransactor) (*AutomatedMarketMakerTransactor, error) {
	contract, err := bindAutomatedMarketMaker(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &AutomatedMarketMakerTransactor{contract: contract}, nil
}

// NewAutomatedMarketMakerFilterer creates a new log filterer instance of AutomatedMarketMaker, bound to a specific deployed contract.
func NewAutomatedMarketMakerFilterer(address common.Address, filterer bind.ContractFilterer) (*AutomatedMarketMakerFilterer, error) {
	contract, err := bindAutomatedMarketMaker(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &AutomatedMarketMakerFilterer{contract: contract}, nil
}

// bindAutomatedMarketMaker binds a generic wrapper to an already deployed contract.
func bindAutomatedMarketMaker(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := AutomatedMarketMakerMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, *parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_AutomatedMarketMaker *AutomatedMarketMakerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _AutomatedMarketMaker.Contract.AutomatedMarketMakerCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_AutomatedMarketMaker *AutomatedMarketMakerRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _AutomatedMarketMaker.Contract.AutomatedMarketMakerTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_AutomatedMarketMaker *AutomatedMarketMakerRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _AutomatedMarketMaker.Contract.AutomatedMarketMakerTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_AutomatedMarketMaker *AutomatedMarketMakerCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _AutomatedMarketMaker.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_AutomatedMarketMaker *AutomatedMarketMakerTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _AutomatedMarketMaker.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_AutomatedMarketMaker *AutomatedMarketMakerTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _AutomatedMarketMaker.Contract.contract.Transact(opts, method, params...)
}

// DEFAULTFEEBPS is a free data retrieval call binding the contract method 0x33d8262c.
//
// Solidity: function DEFAULT_FEE_BPS() view returns(uint256)
func (_AutomatedMarketMaker *AutomatedMarketMakerCaller) DEFAULTFEEBPS(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _AutomatedMarketMaker.contract.Call(opts, &out, "DEFAULT_FEE_BPS")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// DEFAULTFEEBPS is a free data retrieval call binding the contract method 0x33d8262c.
//
// Solidity: function DEFAULT_FEE_BPS() view returns(uint256)
func (_AutomatedMarketMaker *AutomatedMarketMakerSession) DEFAULTFEEBPS() (*big.Int, error) {
	return _AutomatedMarketMaker.Contract.DEFAULTFEEBPS(&_AutomatedMarketMaker.CallOpts)
}

// DEFAULTFEEBPS is a free data retrieval call binding the contract method 0x33d8262c.
//
// Solidity: function DEFAULT_FEE_BPS() view returns(uint256)
func (_AutomatedMarketMaker *AutomatedMarketMakerCallerSession) DEFAULTFEEBPS() (*big.Int, error) {
	return _AutomatedMarketMaker.Contract.DEFAULTFEEBPS(&_AutomatedMarketMaker.CallOpts)
}

// IDENTITYREGISTRY is a free data retrieval call binding the contract method 0x65fad027.
//
// Solidity: function IDENTITY_REGISTRY() view returns(address)
func (_AutomatedMarketMaker *AutomatedMarketMakerCaller) IDENTITYREGISTRY(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _AutomatedMarketMaker.contract.Call(opts, &out, "IDENTITY_REGISTRY")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// IDENTITYREGISTRY is a free data retrieval call binding the contract method 0x65fad027.
//
// Solidity: function IDENTITY_REGISTRY() view returns(address)
func (_AutomatedMarketMaker *AutomatedMarketMakerSession) IDENTITYREGISTRY() (common.Address, error) {
	return _AutomatedMarketMaker.Contract.IDENTITYREGISTRY(&_AutomatedMarketMaker.CallOpts)
}

// IDENTITYREGISTRY is a free data retrieval call binding the contract method 0x65fad027.
//
// Solidity: function IDENTITY_REGISTRY() view returns(address)
func (_AutomatedMarketMaker *AutomatedMarketMakerCallerSession) IDENTITYREGISTRY() (common.Address, error) {
	return _AutomatedMarketMaker.Contract.IDENTITYREGISTRY(&_AutomatedMarketMaker.CallOpts)
}

// MAXFEEBPS is a free data retrieval call binding the contract method 0xd55be8c6.
//
// Solidity: function MAX_FEE_BPS() view returns(uint256)
func (_AutomatedMarketMaker *AutomatedMarketMakerCaller) MAXFEEBPS(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _AutomatedMarketMaker.contract.Call(opts, &out, "MAX_FEE_BPS")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// MAXFEEBPS is a free data retrieval call binding the contract method 0xd55be8c6.
//
// Solidity: function MAX_FEE_BPS() view returns(uint256)
func (_AutomatedMarketMaker *AutomatedMarketMakerSession) MAXFEEBPS() (*big.Int, error) {
	return _AutomatedMarketMaker.Contract.MAXFEEBPS(&_AutomatedMarketMaker.CallOpts)
}

// MAXFEEBPS is a free data retrieval call binding the contract method 0xd55be8c6.
//
// Solidity: function MAX_FEE_BPS() view returns(uint256)
func (_AutomatedMarketMaker *AutomatedMarketMakerCallerSession) MAXFEEBPS() (*big.Int, error) {
	return _AutomatedMarketMaker.Contract.MAXFEEBPS(&_AutomatedMarketMaker.CallOpts)
}

// RESUMEQUORUM is a free data retrieval call binding the contract method 0xbb6fc5e7.
//
// Solidity: function RESUME_QUORUM() view returns(uint256)
func (_AutomatedMarketMaker *AutomatedMarketMakerCaller) RESUMEQUORUM(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _AutomatedMarketMaker.contract.Call(opts, &out, "RESUME_QUORUM")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// RESUMEQUORUM is a free data retrieval call binding the contract method 0xbb6fc5e7.
//
// Solidity: function RESUME_QUORUM() view returns(uint256)
func (_AutomatedMarketMaker *AutomatedMarketMakerSession) RESUMEQUORUM() (*big.Int, error) {
	return _AutomatedMarketMaker.Contract.RESUMEQUORUM(&_AutomatedMarketMaker.CallOpts)
}

// RESUMEQUORUM is a free data retrieval call binding the contract method 0xbb6fc5e7.
//
// Solidity: function RESUME_QUORUM() view returns(uint256)
func (_AutomatedMarketMaker *AutomatedMarketMakerCallerSession) RESUMEQUORUM() (*big.Int, error) {
	return _AutomatedMarketMaker.Contract.RESUMEQUORUM(&_AutomatedMarketMaker.CallOpts)
}

// TOKENA is a free data retrieval call binding the contract method 0xff709ee8.
//
// Solidity: function TOKEN_A() view returns(address)
func (_AutomatedMarketMaker *AutomatedMarketMakerCaller) TOKENA(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _AutomatedMarketMaker.contract.Call(opts, &out, "TOKEN_A")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// TOKENA is a free data retrieval call binding the contract method 0xff709ee8.
//
// Solidity: function TOKEN_A() view returns(address)
func (_AutomatedMarketMaker *AutomatedMarketMakerSession) TOKENA() (common.Address, error) {
	return _AutomatedMarketMaker.Contract.TOKENA(&_AutomatedMarketMaker.CallOpts)
}

// TOKENA is a free data retrieval call binding the contract method 0xff709ee8.
//
// Solidity: function TOKEN_A() view returns(address)
func (_AutomatedMarketMaker *AutomatedMarketMakerCallerSession) TOKENA() (common.Address, error) {
	return _AutomatedMarketMaker.Contract.TOKENA(&_AutomatedMarketMaker.CallOpts)
}

// TOKENB is a free data retrieval call binding the contract method 0x499f712e.
//
// Solidity: function TOKEN_B() view returns(address)
func (_AutomatedMarketMaker *AutomatedMarketMakerCaller) TOKENB(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _AutomatedMarketMaker.contract.Call(opts, &out, "TOKEN_B")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// TOKENB is a free data retrieval call binding the contract method 0x499f712e.
//
// Solidity: function TOKEN_B() view returns(address)
func (_AutomatedMarketMaker *AutomatedMarketMakerSession) TOKENB() (common.Address, error) {
	return _AutomatedMarketMaker.Contract.TOKENB(&_AutomatedMarketMaker.CallOpts)
}

// TOKENB is a free data retrieval call binding the contract method 0x499f712e.
//
// Solidity: function TOKEN_B() view returns(address)
func (_AutomatedMarketMaker *AutomatedMarketMakerCallerSession) TOKENB() (common.Address, error) {
	return _AutomatedMarketMaker.Contract.TOKENB(&_AutomatedMarketMaker.CallOpts)
}

// FeeBps is a free data retrieval call binding the contract method 0x24a9d853.
//
// Solidity: function feeBps() view returns(uint256)
func (_AutomatedMarketMaker *AutomatedMarketMakerCaller) FeeBps(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _AutomatedMarketMaker.contract.Call(opts, &out, "feeBps")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// FeeBps is a free data retrieval call binding the contract method 0x24a9d853.
//
// Solidity: function feeBps() view returns(uint256)
func (_AutomatedMarketMaker *AutomatedMarketMakerSession) FeeBps() (*big.Int, error) {
	return _AutomatedMarketMaker.Contract.FeeBps(&_AutomatedMarketMaker.CallOpts)
}

// FeeBps is a free data retrieval call binding the contract method 0x24a9d853.
//
// Solidity: function feeBps() view returns(uint256)
func (_AutomatedMarketMaker *AutomatedMarketMakerCallerSession) FeeBps() (*big.Int, error) {
	return _AutomatedMarketMaker.Contract.FeeBps(&_AutomatedMarketMaker.CallOpts)
}

// GetAmountIn is a free data retrieval call binding the contract method 0x85f8c259.
//
// Solidity: function getAmountIn(uint256 reserveIn, uint256 reserveOut, uint256 amountOut) pure returns(uint256 amountIn)
func (_AutomatedMarketMaker *AutomatedMarketMakerCaller) GetAmountIn(opts *bind.CallOpts, reserveIn *big.Int, reserveOut *big.Int, amountOut *big.Int) (*big.Int, error) {
	var out []interface{}
	err := _AutomatedMarketMaker.contract.Call(opts, &out, "getAmountIn", reserveIn, reserveOut, amountOut)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// GetAmountIn is a free data retrieval call binding the contract method 0x85f8c259.
//
// Solidity: function getAmountIn(uint256 reserveIn, uint256 reserveOut, uint256 amountOut) pure returns(uint256 amountIn)
func (_AutomatedMarketMaker *AutomatedMarketMakerSession) GetAmountIn(reserveIn *big.Int, reserveOut *big.Int, amountOut *big.Int) (*big.Int, error) {
	return _AutomatedMarketMaker.Contract.GetAmountIn(&_AutomatedMarketMaker.CallOpts, reserveIn, reserveOut, amountOut)
}

// GetAmountIn is a free data retrieval call binding the contract method 0x85f8c259.
//
// Solidity: function getAmountIn(uint256 reserveIn, uint256 reserveOut, uint256 amountOut) pure returns(uint256 amountIn)
func (_AutomatedMarketMaker *AutomatedMarketMakerCallerSession) GetAmountIn(reserveIn *big.Int, reserveOut *big.Int, amountOut *big.Int) (*big.Int, error) {
	return _AutomatedMarketMaker.Contract.GetAmountIn(&_AutomatedMarketMaker.CallOpts, reserveIn, reserveOut, amountOut)
}

// IsPaused is a free data retrieval call binding the contract method 0xb187bd26.
//
// Solidity: function isPaused() view returns(bool)
func (_AutomatedMarketMaker *AutomatedMarketMakerCaller) IsPaused(opts *bind.CallOpts) (bool, error) {
	var out []interface{}
	err := _AutomatedMarketMaker.contract.Call(opts, &out, "isPaused")

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// IsPaused is a free data retrieval call binding the contract method 0xb187bd26.
//
// Solidity: function isPaused() view returns(bool)
func (_AutomatedMarketMaker *AutomatedMarketMakerSession) IsPaused() (bool, error) {
	return _AutomatedMarketMaker.Contract.IsPaused(&_AutomatedMarketMaker.CallOpts)
}

// IsPaused is a free data retrieval call binding the contract method 0xb187bd26.
//
// Solidity: function isPaused() view returns(bool)
func (_AutomatedMarketMaker *AutomatedMarketMakerCallerSession) IsPaused() (bool, error) {
	return _AutomatedMarketMaker.Contract.IsPaused(&_AutomatedMarketMaker.CallOpts)
}

// PauseEpoch is a free data retrieval call binding the contract method 0xb139603c.
//
// Solidity: function pauseEpoch() view returns(uint256)
func (_AutomatedMarketMaker *AutomatedMarketMakerCaller) PauseEpoch(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _AutomatedMarketMaker.contract.Call(opts, &out, "pauseEpoch")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// PauseEpoch is a free data retrieval call binding the contract method 0xb139603c.
//
// Solidity: function pauseEpoch() view returns(uint256)
func (_AutomatedMarketMaker *AutomatedMarketMakerSession) PauseEpoch() (*big.Int, error) {
	return _AutomatedMarketMaker.Contract.PauseEpoch(&_AutomatedMarketMaker.CallOpts)
}

// PauseEpoch is a free data retrieval call binding the contract method 0xb139603c.
//
// Solidity: function pauseEpoch() view returns(uint256)
func (_AutomatedMarketMaker *AutomatedMarketMakerCallerSession) PauseEpoch() (*big.Int, error) {
	return _AutomatedMarketMaker.Contract.PauseEpoch(&_AutomatedMarketMaker.CallOpts)
}

// Paused is a free data retrieval call binding the contract method 0x5c975abb.
//
// Solidity: function paused() view returns(bool)
func (_AutomatedMarketMaker *AutomatedMarketMakerCaller) Paused(opts *bind.CallOpts) (bool, error) {
	var out []interface{}
	err := _AutomatedMarketMaker.contract.Call(opts, &out, "paused")

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// Paused is a free data retrieval call binding the contract method 0x5c975abb.
//
// Solidity: function paused() view returns(bool)
func (_AutomatedMarketMaker *AutomatedMarketMakerSession) Paused() (bool, error) {
	return _AutomatedMarketMaker.Contract.Paused(&_AutomatedMarketMaker.CallOpts)
}

// Paused is a free data retrieval call binding the contract method 0x5c975abb.
//
// Solidity: function paused() view returns(bool)
func (_AutomatedMarketMaker *AutomatedMarketMakerCallerSession) Paused() (bool, error) {
	return _AutomatedMarketMaker.Contract.Paused(&_AutomatedMarketMaker.CallOpts)
}

// QuoteExactOutput is a free data retrieval call binding the contract method 0x8fcf399b.
//
// Solidity: function quoteExactOutput(uint256 reserveIn, uint256 reserveOut, uint256 amountOut, uint256 feeBps_) pure returns(uint256 amountIn)
func (_AutomatedMarketMaker *AutomatedMarketMakerCaller) QuoteExactOutput(opts *bind.CallOpts, reserveIn *big.Int, reserveOut *big.Int, amountOut *big.Int, feeBps_ *big.Int) (*big.Int, error) {
	var out []interface{}
	err := _AutomatedMarketMaker.contract.Call(opts, &out, "quoteExactOutput", reserveIn, reserveOut, amountOut, feeBps_)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// QuoteExactOutput is a free data retrieval call binding the contract method 0x8fcf399b.
//
// Solidity: function quoteExactOutput(uint256 reserveIn, uint256 reserveOut, uint256 amountOut, uint256 feeBps_) pure returns(uint256 amountIn)
func (_AutomatedMarketMaker *AutomatedMarketMakerSession) QuoteExactOutput(reserveIn *big.Int, reserveOut *big.Int, amountOut *big.Int, feeBps_ *big.Int) (*big.Int, error) {
	return _AutomatedMarketMaker.Contract.QuoteExactOutput(&_AutomatedMarketMaker.CallOpts, reserveIn, reserveOut, amountOut, feeBps_)
}

// QuoteExactOutput is a free data retrieval call binding the contract method 0x8fcf399b.
//
// Solidity: function quoteExactOutput(uint256 reserveIn, uint256 reserveOut, uint256 amountOut, uint256 feeBps_) pure returns(uint256 amountIn)
func (_AutomatedMarketMaker *AutomatedMarketMakerCallerSession) QuoteExactOutput(reserveIn *big.Int, reserveOut *big.Int, amountOut *big.Int, feeBps_ *big.Int) (*big.Int, error) {
	return _AutomatedMarketMaker.Contract.QuoteExactOutput(&_AutomatedMarketMaker.CallOpts, reserveIn, reserveOut, amountOut, feeBps_)
}

// ReserveA is a free data retrieval call binding the contract method 0xdc5fa6c5.
//
// Solidity: function reserveA() view returns(uint256)
func (_AutomatedMarketMaker *AutomatedMarketMakerCaller) ReserveA(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _AutomatedMarketMaker.contract.Call(opts, &out, "reserveA")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// ReserveA is a free data retrieval call binding the contract method 0xdc5fa6c5.
//
// Solidity: function reserveA() view returns(uint256)
func (_AutomatedMarketMaker *AutomatedMarketMakerSession) ReserveA() (*big.Int, error) {
	return _AutomatedMarketMaker.Contract.ReserveA(&_AutomatedMarketMaker.CallOpts)
}

// ReserveA is a free data retrieval call binding the contract method 0xdc5fa6c5.
//
// Solidity: function reserveA() view returns(uint256)
func (_AutomatedMarketMaker *AutomatedMarketMakerCallerSession) ReserveA() (*big.Int, error) {
	return _AutomatedMarketMaker.Contract.ReserveA(&_AutomatedMarketMaker.CallOpts)
}

// ReserveB is a free data retrieval call binding the contract method 0x19e36f3b.
//
// Solidity: function reserveB() view returns(uint256)
func (_AutomatedMarketMaker *AutomatedMarketMakerCaller) ReserveB(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _AutomatedMarketMaker.contract.Call(opts, &out, "reserveB")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// ReserveB is a free data retrieval call binding the contract method 0x19e36f3b.
//
// Solidity: function reserveB() view returns(uint256)
func (_AutomatedMarketMaker *AutomatedMarketMakerSession) ReserveB() (*big.Int, error) {
	return _AutomatedMarketMaker.Contract.ReserveB(&_AutomatedMarketMaker.CallOpts)
}

// ReserveB is a free data retrieval call binding the contract method 0x19e36f3b.
//
// Solidity: function reserveB() view returns(uint256)
func (_AutomatedMarketMaker *AutomatedMarketMakerCallerSession) ReserveB() (*big.Int, error) {
	return _AutomatedMarketMaker.Contract.ReserveB(&_AutomatedMarketMaker.CallOpts)
}

// ResumeQuorum is a free data retrieval call binding the contract method 0xabdfb5b2.
//
// Solidity: function resumeQuorum() pure returns(uint256)
func (_AutomatedMarketMaker *AutomatedMarketMakerCaller) ResumeQuorum(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _AutomatedMarketMaker.contract.Call(opts, &out, "resumeQuorum")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// ResumeQuorum is a free data retrieval call binding the contract method 0xabdfb5b2.
//
// Solidity: function resumeQuorum() pure returns(uint256)
func (_AutomatedMarketMaker *AutomatedMarketMakerSession) ResumeQuorum() (*big.Int, error) {
	return _AutomatedMarketMaker.Contract.ResumeQuorum(&_AutomatedMarketMaker.CallOpts)
}

// ResumeQuorum is a free data retrieval call binding the contract method 0xabdfb5b2.
//
// Solidity: function resumeQuorum() pure returns(uint256)
func (_AutomatedMarketMaker *AutomatedMarketMakerCallerSession) ResumeQuorum() (*big.Int, error) {
	return _AutomatedMarketMaker.Contract.ResumeQuorum(&_AutomatedMarketMaker.CallOpts)
}

// ResumeSignatures is a free data retrieval call binding the contract method 0x9c722cde.
//
// Solidity: function resumeSignatures(bytes32 proposalId) view returns(uint256)
func (_AutomatedMarketMaker *AutomatedMarketMakerCaller) ResumeSignatures(opts *bind.CallOpts, proposalId [32]byte) (*big.Int, error) {
	var out []interface{}
	err := _AutomatedMarketMaker.contract.Call(opts, &out, "resumeSignatures", proposalId)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// ResumeSignatures is a free data retrieval call binding the contract method 0x9c722cde.
//
// Solidity: function resumeSignatures(bytes32 proposalId) view returns(uint256)
func (_AutomatedMarketMaker *AutomatedMarketMakerSession) ResumeSignatures(proposalId [32]byte) (*big.Int, error) {
	return _AutomatedMarketMaker.Contract.ResumeSignatures(&_AutomatedMarketMaker.CallOpts, proposalId)
}

// ResumeSignatures is a free data retrieval call binding the contract method 0x9c722cde.
//
// Solidity: function resumeSignatures(bytes32 proposalId) view returns(uint256)
func (_AutomatedMarketMaker *AutomatedMarketMakerCallerSession) ResumeSignatures(proposalId [32]byte) (*big.Int, error) {
	return _AutomatedMarketMaker.Contract.ResumeSignatures(&_AutomatedMarketMaker.CallOpts, proposalId)
}

// AddLiquidity is a paid mutator transaction binding the contract method 0x9cd441da.
//
// Solidity: function addLiquidity(uint256 amountA, uint256 amountB) returns()
func (_AutomatedMarketMaker *AutomatedMarketMakerTransactor) AddLiquidity(opts *bind.TransactOpts, amountA *big.Int, amountB *big.Int) (*types.Transaction, error) {
	return _AutomatedMarketMaker.contract.Transact(opts, "addLiquidity", amountA, amountB)
}

// AddLiquidity is a paid mutator transaction binding the contract method 0x9cd441da.
//
// Solidity: function addLiquidity(uint256 amountA, uint256 amountB) returns()
func (_AutomatedMarketMaker *AutomatedMarketMakerSession) AddLiquidity(amountA *big.Int, amountB *big.Int) (*types.Transaction, error) {
	return _AutomatedMarketMaker.Contract.AddLiquidity(&_AutomatedMarketMaker.TransactOpts, amountA, amountB)
}

// AddLiquidity is a paid mutator transaction binding the contract method 0x9cd441da.
//
// Solidity: function addLiquidity(uint256 amountA, uint256 amountB) returns()
func (_AutomatedMarketMaker *AutomatedMarketMakerTransactorSession) AddLiquidity(amountA *big.Int, amountB *big.Int) (*types.Transaction, error) {
	return _AutomatedMarketMaker.Contract.AddLiquidity(&_AutomatedMarketMaker.TransactOpts, amountA, amountB)
}

// Pause is a paid mutator transaction binding the contract method 0x6da66355.
//
// Solidity: function pause(string reason) returns()
func (_AutomatedMarketMaker *AutomatedMarketMakerTransactor) Pause(opts *bind.TransactOpts, reason string) (*types.Transaction, error) {
	return _AutomatedMarketMaker.contract.Transact(opts, "pause", reason)
}

// Pause is a paid mutator transaction binding the contract method 0x6da66355.
//
// Solidity: function pause(string reason) returns()
func (_AutomatedMarketMaker *AutomatedMarketMakerSession) Pause(reason string) (*types.Transaction, error) {
	return _AutomatedMarketMaker.Contract.Pause(&_AutomatedMarketMaker.TransactOpts, reason)
}

// Pause is a paid mutator transaction binding the contract method 0x6da66355.
//
// Solidity: function pause(string reason) returns()
func (_AutomatedMarketMaker *AutomatedMarketMakerTransactorSession) Pause(reason string) (*types.Transaction, error) {
	return _AutomatedMarketMaker.Contract.Pause(&_AutomatedMarketMaker.TransactOpts, reason)
}

// ProposeResume is a paid mutator transaction binding the contract method 0x9e3f8fd7.
//
// Solidity: function proposeResume() returns(bytes32 proposalId)
func (_AutomatedMarketMaker *AutomatedMarketMakerTransactor) ProposeResume(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _AutomatedMarketMaker.contract.Transact(opts, "proposeResume")
}

// ProposeResume is a paid mutator transaction binding the contract method 0x9e3f8fd7.
//
// Solidity: function proposeResume() returns(bytes32 proposalId)
func (_AutomatedMarketMaker *AutomatedMarketMakerSession) ProposeResume() (*types.Transaction, error) {
	return _AutomatedMarketMaker.Contract.ProposeResume(&_AutomatedMarketMaker.TransactOpts)
}

// ProposeResume is a paid mutator transaction binding the contract method 0x9e3f8fd7.
//
// Solidity: function proposeResume() returns(bytes32 proposalId)
func (_AutomatedMarketMaker *AutomatedMarketMakerTransactorSession) ProposeResume() (*types.Transaction, error) {
	return _AutomatedMarketMaker.Contract.ProposeResume(&_AutomatedMarketMaker.TransactOpts)
}

// SetFeeBps is a paid mutator transaction binding the contract method 0x72c27b62.
//
// Solidity: function setFeeBps(uint256 newFeeBps) returns()
func (_AutomatedMarketMaker *AutomatedMarketMakerTransactor) SetFeeBps(opts *bind.TransactOpts, newFeeBps *big.Int) (*types.Transaction, error) {
	return _AutomatedMarketMaker.contract.Transact(opts, "setFeeBps", newFeeBps)
}

// SetFeeBps is a paid mutator transaction binding the contract method 0x72c27b62.
//
// Solidity: function setFeeBps(uint256 newFeeBps) returns()
func (_AutomatedMarketMaker *AutomatedMarketMakerSession) SetFeeBps(newFeeBps *big.Int) (*types.Transaction, error) {
	return _AutomatedMarketMaker.Contract.SetFeeBps(&_AutomatedMarketMaker.TransactOpts, newFeeBps)
}

// SetFeeBps is a paid mutator transaction binding the contract method 0x72c27b62.
//
// Solidity: function setFeeBps(uint256 newFeeBps) returns()
func (_AutomatedMarketMaker *AutomatedMarketMakerTransactorSession) SetFeeBps(newFeeBps *big.Int) (*types.Transaction, error) {
	return _AutomatedMarketMaker.Contract.SetFeeBps(&_AutomatedMarketMaker.TransactOpts, newFeeBps)
}

// SignResume is a paid mutator transaction binding the contract method 0x733ffb2b.
//
// Solidity: function signResume(bytes32 proposalId) returns()
func (_AutomatedMarketMaker *AutomatedMarketMakerTransactor) SignResume(opts *bind.TransactOpts, proposalId [32]byte) (*types.Transaction, error) {
	return _AutomatedMarketMaker.contract.Transact(opts, "signResume", proposalId)
}

// SignResume is a paid mutator transaction binding the contract method 0x733ffb2b.
//
// Solidity: function signResume(bytes32 proposalId) returns()
func (_AutomatedMarketMaker *AutomatedMarketMakerSession) SignResume(proposalId [32]byte) (*types.Transaction, error) {
	return _AutomatedMarketMaker.Contract.SignResume(&_AutomatedMarketMaker.TransactOpts, proposalId)
}

// SignResume is a paid mutator transaction binding the contract method 0x733ffb2b.
//
// Solidity: function signResume(bytes32 proposalId) returns()
func (_AutomatedMarketMaker *AutomatedMarketMakerTransactorSession) SignResume(proposalId [32]byte) (*types.Transaction, error) {
	return _AutomatedMarketMaker.Contract.SignResume(&_AutomatedMarketMaker.TransactOpts, proposalId)
}

// SwapTokensForExactTokens is a paid mutator transaction binding the contract method 0xf7d31809.
//
// Solidity: function swapTokensForExactTokens(address tokenIn, address tokenOut, uint256 amountOut, uint256 maxAmountIn, address to) returns(uint256 amountIn)
func (_AutomatedMarketMaker *AutomatedMarketMakerTransactor) SwapTokensForExactTokens(opts *bind.TransactOpts, tokenIn common.Address, tokenOut common.Address, amountOut *big.Int, maxAmountIn *big.Int, to common.Address) (*types.Transaction, error) {
	return _AutomatedMarketMaker.contract.Transact(opts, "swapTokensForExactTokens", tokenIn, tokenOut, amountOut, maxAmountIn, to)
}

// SwapTokensForExactTokens is a paid mutator transaction binding the contract method 0xf7d31809.
//
// Solidity: function swapTokensForExactTokens(address tokenIn, address tokenOut, uint256 amountOut, uint256 maxAmountIn, address to) returns(uint256 amountIn)
func (_AutomatedMarketMaker *AutomatedMarketMakerSession) SwapTokensForExactTokens(tokenIn common.Address, tokenOut common.Address, amountOut *big.Int, maxAmountIn *big.Int, to common.Address) (*types.Transaction, error) {
	return _AutomatedMarketMaker.Contract.SwapTokensForExactTokens(&_AutomatedMarketMaker.TransactOpts, tokenIn, tokenOut, amountOut, maxAmountIn, to)
}

// SwapTokensForExactTokens is a paid mutator transaction binding the contract method 0xf7d31809.
//
// Solidity: function swapTokensForExactTokens(address tokenIn, address tokenOut, uint256 amountOut, uint256 maxAmountIn, address to) returns(uint256 amountIn)
func (_AutomatedMarketMaker *AutomatedMarketMakerTransactorSession) SwapTokensForExactTokens(tokenIn common.Address, tokenOut common.Address, amountOut *big.Int, maxAmountIn *big.Int, to common.Address) (*types.Transaction, error) {
	return _AutomatedMarketMaker.Contract.SwapTokensForExactTokens(&_AutomatedMarketMaker.TransactOpts, tokenIn, tokenOut, amountOut, maxAmountIn, to)
}

// AutomatedMarketMakerLogCircuitBreakerPausedIterator is returned from FilterLogCircuitBreakerPaused and is used to iterate over the raw logs and unpacked data for LogCircuitBreakerPaused events raised by the AutomatedMarketMaker contract.
type AutomatedMarketMakerLogCircuitBreakerPausedIterator struct {
	Event *AutomatedMarketMakerLogCircuitBreakerPaused // Event containing the contract specifics and raw log

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
func (it *AutomatedMarketMakerLogCircuitBreakerPausedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(AutomatedMarketMakerLogCircuitBreakerPaused)
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
		it.Event = new(AutomatedMarketMakerLogCircuitBreakerPaused)
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
func (it *AutomatedMarketMakerLogCircuitBreakerPausedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *AutomatedMarketMakerLogCircuitBreakerPausedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// AutomatedMarketMakerLogCircuitBreakerPaused represents a LogCircuitBreakerPaused event raised by the AutomatedMarketMaker contract.
type AutomatedMarketMakerLogCircuitBreakerPaused struct {
	PausedBy  common.Address
	Timestamp *big.Int
	Reason    string
	Raw       types.Log // Blockchain specific contextual infos
}

// FilterLogCircuitBreakerPaused is a free log retrieval operation binding the contract event 0x48bf5813314de41e6ad29b69491af3cc17d544ff8e92c7fa15548dea9e5ea8a2.
//
// Solidity: event LogCircuitBreakerPaused(address indexed pausedBy, uint256 timestamp, string reason)
func (_AutomatedMarketMaker *AutomatedMarketMakerFilterer) FilterLogCircuitBreakerPaused(opts *bind.FilterOpts, pausedBy []common.Address) (*AutomatedMarketMakerLogCircuitBreakerPausedIterator, error) {

	var pausedByRule []interface{}
	for _, pausedByItem := range pausedBy {
		pausedByRule = append(pausedByRule, pausedByItem)
	}

	logs, sub, err := _AutomatedMarketMaker.contract.FilterLogs(opts, "LogCircuitBreakerPaused", pausedByRule)
	if err != nil {
		return nil, err
	}
	return &AutomatedMarketMakerLogCircuitBreakerPausedIterator{contract: _AutomatedMarketMaker.contract, event: "LogCircuitBreakerPaused", logs: logs, sub: sub}, nil
}

// WatchLogCircuitBreakerPaused is a free log subscription operation binding the contract event 0x48bf5813314de41e6ad29b69491af3cc17d544ff8e92c7fa15548dea9e5ea8a2.
//
// Solidity: event LogCircuitBreakerPaused(address indexed pausedBy, uint256 timestamp, string reason)
func (_AutomatedMarketMaker *AutomatedMarketMakerFilterer) WatchLogCircuitBreakerPaused(opts *bind.WatchOpts, sink chan<- *AutomatedMarketMakerLogCircuitBreakerPaused, pausedBy []common.Address) (event.Subscription, error) {

	var pausedByRule []interface{}
	for _, pausedByItem := range pausedBy {
		pausedByRule = append(pausedByRule, pausedByItem)
	}

	logs, sub, err := _AutomatedMarketMaker.contract.WatchLogs(opts, "LogCircuitBreakerPaused", pausedByRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(AutomatedMarketMakerLogCircuitBreakerPaused)
				if err := _AutomatedMarketMaker.contract.UnpackLog(event, "LogCircuitBreakerPaused", log); err != nil {
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

// ParseLogCircuitBreakerPaused is a log parse operation binding the contract event 0x48bf5813314de41e6ad29b69491af3cc17d544ff8e92c7fa15548dea9e5ea8a2.
//
// Solidity: event LogCircuitBreakerPaused(address indexed pausedBy, uint256 timestamp, string reason)
func (_AutomatedMarketMaker *AutomatedMarketMakerFilterer) ParseLogCircuitBreakerPaused(log types.Log) (*AutomatedMarketMakerLogCircuitBreakerPaused, error) {
	event := new(AutomatedMarketMakerLogCircuitBreakerPaused)
	if err := _AutomatedMarketMaker.contract.UnpackLog(event, "LogCircuitBreakerPaused", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// AutomatedMarketMakerLogCircuitBreakerResumedIterator is returned from FilterLogCircuitBreakerResumed and is used to iterate over the raw logs and unpacked data for LogCircuitBreakerResumed events raised by the AutomatedMarketMaker contract.
type AutomatedMarketMakerLogCircuitBreakerResumedIterator struct {
	Event *AutomatedMarketMakerLogCircuitBreakerResumed // Event containing the contract specifics and raw log

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
func (it *AutomatedMarketMakerLogCircuitBreakerResumedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(AutomatedMarketMakerLogCircuitBreakerResumed)
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
		it.Event = new(AutomatedMarketMakerLogCircuitBreakerResumed)
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
func (it *AutomatedMarketMakerLogCircuitBreakerResumedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *AutomatedMarketMakerLogCircuitBreakerResumedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// AutomatedMarketMakerLogCircuitBreakerResumed represents a LogCircuitBreakerResumed event raised by the AutomatedMarketMaker contract.
type AutomatedMarketMakerLogCircuitBreakerResumed struct {
	ProposalId [32]byte
	Timestamp  *big.Int
	Raw        types.Log // Blockchain specific contextual infos
}

// FilterLogCircuitBreakerResumed is a free log retrieval operation binding the contract event 0x191e4e50c96c12749e054ea243f820456a9ce7d59b4695a95d1465b26e21c903.
//
// Solidity: event LogCircuitBreakerResumed(bytes32 indexed proposalId, uint256 timestamp)
func (_AutomatedMarketMaker *AutomatedMarketMakerFilterer) FilterLogCircuitBreakerResumed(opts *bind.FilterOpts, proposalId [][32]byte) (*AutomatedMarketMakerLogCircuitBreakerResumedIterator, error) {

	var proposalIdRule []interface{}
	for _, proposalIdItem := range proposalId {
		proposalIdRule = append(proposalIdRule, proposalIdItem)
	}

	logs, sub, err := _AutomatedMarketMaker.contract.FilterLogs(opts, "LogCircuitBreakerResumed", proposalIdRule)
	if err != nil {
		return nil, err
	}
	return &AutomatedMarketMakerLogCircuitBreakerResumedIterator{contract: _AutomatedMarketMaker.contract, event: "LogCircuitBreakerResumed", logs: logs, sub: sub}, nil
}

// WatchLogCircuitBreakerResumed is a free log subscription operation binding the contract event 0x191e4e50c96c12749e054ea243f820456a9ce7d59b4695a95d1465b26e21c903.
//
// Solidity: event LogCircuitBreakerResumed(bytes32 indexed proposalId, uint256 timestamp)
func (_AutomatedMarketMaker *AutomatedMarketMakerFilterer) WatchLogCircuitBreakerResumed(opts *bind.WatchOpts, sink chan<- *AutomatedMarketMakerLogCircuitBreakerResumed, proposalId [][32]byte) (event.Subscription, error) {

	var proposalIdRule []interface{}
	for _, proposalIdItem := range proposalId {
		proposalIdRule = append(proposalIdRule, proposalIdItem)
	}

	logs, sub, err := _AutomatedMarketMaker.contract.WatchLogs(opts, "LogCircuitBreakerResumed", proposalIdRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(AutomatedMarketMakerLogCircuitBreakerResumed)
				if err := _AutomatedMarketMaker.contract.UnpackLog(event, "LogCircuitBreakerResumed", log); err != nil {
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

// ParseLogCircuitBreakerResumed is a log parse operation binding the contract event 0x191e4e50c96c12749e054ea243f820456a9ce7d59b4695a95d1465b26e21c903.
//
// Solidity: event LogCircuitBreakerResumed(bytes32 indexed proposalId, uint256 timestamp)
func (_AutomatedMarketMaker *AutomatedMarketMakerFilterer) ParseLogCircuitBreakerResumed(log types.Log) (*AutomatedMarketMakerLogCircuitBreakerResumed, error) {
	event := new(AutomatedMarketMakerLogCircuitBreakerResumed)
	if err := _AutomatedMarketMaker.contract.UnpackLog(event, "LogCircuitBreakerResumed", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// AutomatedMarketMakerLogFeeRateUpdatedIterator is returned from FilterLogFeeRateUpdated and is used to iterate over the raw logs and unpacked data for LogFeeRateUpdated events raised by the AutomatedMarketMaker contract.
type AutomatedMarketMakerLogFeeRateUpdatedIterator struct {
	Event *AutomatedMarketMakerLogFeeRateUpdated // Event containing the contract specifics and raw log

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
func (it *AutomatedMarketMakerLogFeeRateUpdatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(AutomatedMarketMakerLogFeeRateUpdated)
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
		it.Event = new(AutomatedMarketMakerLogFeeRateUpdated)
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
func (it *AutomatedMarketMakerLogFeeRateUpdatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *AutomatedMarketMakerLogFeeRateUpdatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// AutomatedMarketMakerLogFeeRateUpdated represents a LogFeeRateUpdated event raised by the AutomatedMarketMaker contract.
type AutomatedMarketMakerLogFeeRateUpdated struct {
	OldFeeBps *big.Int
	NewFeeBps *big.Int
	Raw       types.Log // Blockchain specific contextual infos
}

// FilterLogFeeRateUpdated is a free log retrieval operation binding the contract event 0x79c496f1c6c3df4a0a1cabbe2f47ff408d8d95d717af8a61c19b5c5bbfc67a53.
//
// Solidity: event LogFeeRateUpdated(uint256 oldFeeBps, uint256 newFeeBps)
func (_AutomatedMarketMaker *AutomatedMarketMakerFilterer) FilterLogFeeRateUpdated(opts *bind.FilterOpts) (*AutomatedMarketMakerLogFeeRateUpdatedIterator, error) {

	logs, sub, err := _AutomatedMarketMaker.contract.FilterLogs(opts, "LogFeeRateUpdated")
	if err != nil {
		return nil, err
	}
	return &AutomatedMarketMakerLogFeeRateUpdatedIterator{contract: _AutomatedMarketMaker.contract, event: "LogFeeRateUpdated", logs: logs, sub: sub}, nil
}

// WatchLogFeeRateUpdated is a free log subscription operation binding the contract event 0x79c496f1c6c3df4a0a1cabbe2f47ff408d8d95d717af8a61c19b5c5bbfc67a53.
//
// Solidity: event LogFeeRateUpdated(uint256 oldFeeBps, uint256 newFeeBps)
func (_AutomatedMarketMaker *AutomatedMarketMakerFilterer) WatchLogFeeRateUpdated(opts *bind.WatchOpts, sink chan<- *AutomatedMarketMakerLogFeeRateUpdated) (event.Subscription, error) {

	logs, sub, err := _AutomatedMarketMaker.contract.WatchLogs(opts, "LogFeeRateUpdated")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(AutomatedMarketMakerLogFeeRateUpdated)
				if err := _AutomatedMarketMaker.contract.UnpackLog(event, "LogFeeRateUpdated", log); err != nil {
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

// ParseLogFeeRateUpdated is a log parse operation binding the contract event 0x79c496f1c6c3df4a0a1cabbe2f47ff408d8d95d717af8a61c19b5c5bbfc67a53.
//
// Solidity: event LogFeeRateUpdated(uint256 oldFeeBps, uint256 newFeeBps)
func (_AutomatedMarketMaker *AutomatedMarketMakerFilterer) ParseLogFeeRateUpdated(log types.Log) (*AutomatedMarketMakerLogFeeRateUpdated, error) {
	event := new(AutomatedMarketMakerLogFeeRateUpdated)
	if err := _AutomatedMarketMaker.contract.UnpackLog(event, "LogFeeRateUpdated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// AutomatedMarketMakerLogLiquidityAddedIterator is returned from FilterLogLiquidityAdded and is used to iterate over the raw logs and unpacked data for LogLiquidityAdded events raised by the AutomatedMarketMaker contract.
type AutomatedMarketMakerLogLiquidityAddedIterator struct {
	Event *AutomatedMarketMakerLogLiquidityAdded // Event containing the contract specifics and raw log

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
func (it *AutomatedMarketMakerLogLiquidityAddedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(AutomatedMarketMakerLogLiquidityAdded)
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
		it.Event = new(AutomatedMarketMakerLogLiquidityAdded)
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
func (it *AutomatedMarketMakerLogLiquidityAddedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *AutomatedMarketMakerLogLiquidityAddedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// AutomatedMarketMakerLogLiquidityAdded represents a LogLiquidityAdded event raised by the AutomatedMarketMaker contract.
type AutomatedMarketMakerLogLiquidityAdded struct {
	Provider     common.Address
	AmountTokenA *big.Int
	AmountTokenB *big.Int
	Raw          types.Log // Blockchain specific contextual infos
}

// FilterLogLiquidityAdded is a free log retrieval operation binding the contract event 0x08bb63be3d81c7d4e8e394124c5072a7e89026c1621ec00b083a986f22c1311f.
//
// Solidity: event LogLiquidityAdded(address indexed provider, uint256 amountTokenA, uint256 amountTokenB)
func (_AutomatedMarketMaker *AutomatedMarketMakerFilterer) FilterLogLiquidityAdded(opts *bind.FilterOpts, provider []common.Address) (*AutomatedMarketMakerLogLiquidityAddedIterator, error) {

	var providerRule []interface{}
	for _, providerItem := range provider {
		providerRule = append(providerRule, providerItem)
	}

	logs, sub, err := _AutomatedMarketMaker.contract.FilterLogs(opts, "LogLiquidityAdded", providerRule)
	if err != nil {
		return nil, err
	}
	return &AutomatedMarketMakerLogLiquidityAddedIterator{contract: _AutomatedMarketMaker.contract, event: "LogLiquidityAdded", logs: logs, sub: sub}, nil
}

// WatchLogLiquidityAdded is a free log subscription operation binding the contract event 0x08bb63be3d81c7d4e8e394124c5072a7e89026c1621ec00b083a986f22c1311f.
//
// Solidity: event LogLiquidityAdded(address indexed provider, uint256 amountTokenA, uint256 amountTokenB)
func (_AutomatedMarketMaker *AutomatedMarketMakerFilterer) WatchLogLiquidityAdded(opts *bind.WatchOpts, sink chan<- *AutomatedMarketMakerLogLiquidityAdded, provider []common.Address) (event.Subscription, error) {

	var providerRule []interface{}
	for _, providerItem := range provider {
		providerRule = append(providerRule, providerItem)
	}

	logs, sub, err := _AutomatedMarketMaker.contract.WatchLogs(opts, "LogLiquidityAdded", providerRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(AutomatedMarketMakerLogLiquidityAdded)
				if err := _AutomatedMarketMaker.contract.UnpackLog(event, "LogLiquidityAdded", log); err != nil {
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

// ParseLogLiquidityAdded is a log parse operation binding the contract event 0x08bb63be3d81c7d4e8e394124c5072a7e89026c1621ec00b083a986f22c1311f.
//
// Solidity: event LogLiquidityAdded(address indexed provider, uint256 amountTokenA, uint256 amountTokenB)
func (_AutomatedMarketMaker *AutomatedMarketMakerFilterer) ParseLogLiquidityAdded(log types.Log) (*AutomatedMarketMakerLogLiquidityAdded, error) {
	event := new(AutomatedMarketMakerLogLiquidityAdded)
	if err := _AutomatedMarketMaker.contract.UnpackLog(event, "LogLiquidityAdded", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// AutomatedMarketMakerLogResumeProposedIterator is returned from FilterLogResumeProposed and is used to iterate over the raw logs and unpacked data for LogResumeProposed events raised by the AutomatedMarketMaker contract.
type AutomatedMarketMakerLogResumeProposedIterator struct {
	Event *AutomatedMarketMakerLogResumeProposed // Event containing the contract specifics and raw log

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
func (it *AutomatedMarketMakerLogResumeProposedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(AutomatedMarketMakerLogResumeProposed)
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
		it.Event = new(AutomatedMarketMakerLogResumeProposed)
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
func (it *AutomatedMarketMakerLogResumeProposedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *AutomatedMarketMakerLogResumeProposedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// AutomatedMarketMakerLogResumeProposed represents a LogResumeProposed event raised by the AutomatedMarketMaker contract.
type AutomatedMarketMakerLogResumeProposed struct {
	ProposalId [32]byte
	ProposedBy common.Address
	Timestamp  *big.Int
	Raw        types.Log // Blockchain specific contextual infos
}

// FilterLogResumeProposed is a free log retrieval operation binding the contract event 0x3bbaac345bb995e6e6d4dd57822687a8426ea5eac92cc095b6d0a6ca763e8323.
//
// Solidity: event LogResumeProposed(bytes32 indexed proposalId, address indexed proposedBy, uint256 timestamp)
func (_AutomatedMarketMaker *AutomatedMarketMakerFilterer) FilterLogResumeProposed(opts *bind.FilterOpts, proposalId [][32]byte, proposedBy []common.Address) (*AutomatedMarketMakerLogResumeProposedIterator, error) {

	var proposalIdRule []interface{}
	for _, proposalIdItem := range proposalId {
		proposalIdRule = append(proposalIdRule, proposalIdItem)
	}
	var proposedByRule []interface{}
	for _, proposedByItem := range proposedBy {
		proposedByRule = append(proposedByRule, proposedByItem)
	}

	logs, sub, err := _AutomatedMarketMaker.contract.FilterLogs(opts, "LogResumeProposed", proposalIdRule, proposedByRule)
	if err != nil {
		return nil, err
	}
	return &AutomatedMarketMakerLogResumeProposedIterator{contract: _AutomatedMarketMaker.contract, event: "LogResumeProposed", logs: logs, sub: sub}, nil
}

// WatchLogResumeProposed is a free log subscription operation binding the contract event 0x3bbaac345bb995e6e6d4dd57822687a8426ea5eac92cc095b6d0a6ca763e8323.
//
// Solidity: event LogResumeProposed(bytes32 indexed proposalId, address indexed proposedBy, uint256 timestamp)
func (_AutomatedMarketMaker *AutomatedMarketMakerFilterer) WatchLogResumeProposed(opts *bind.WatchOpts, sink chan<- *AutomatedMarketMakerLogResumeProposed, proposalId [][32]byte, proposedBy []common.Address) (event.Subscription, error) {

	var proposalIdRule []interface{}
	for _, proposalIdItem := range proposalId {
		proposalIdRule = append(proposalIdRule, proposalIdItem)
	}
	var proposedByRule []interface{}
	for _, proposedByItem := range proposedBy {
		proposedByRule = append(proposedByRule, proposedByItem)
	}

	logs, sub, err := _AutomatedMarketMaker.contract.WatchLogs(opts, "LogResumeProposed", proposalIdRule, proposedByRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(AutomatedMarketMakerLogResumeProposed)
				if err := _AutomatedMarketMaker.contract.UnpackLog(event, "LogResumeProposed", log); err != nil {
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

// ParseLogResumeProposed is a log parse operation binding the contract event 0x3bbaac345bb995e6e6d4dd57822687a8426ea5eac92cc095b6d0a6ca763e8323.
//
// Solidity: event LogResumeProposed(bytes32 indexed proposalId, address indexed proposedBy, uint256 timestamp)
func (_AutomatedMarketMaker *AutomatedMarketMakerFilterer) ParseLogResumeProposed(log types.Log) (*AutomatedMarketMakerLogResumeProposed, error) {
	event := new(AutomatedMarketMakerLogResumeProposed)
	if err := _AutomatedMarketMaker.contract.UnpackLog(event, "LogResumeProposed", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// AutomatedMarketMakerLogResumeSignedIterator is returned from FilterLogResumeSigned and is used to iterate over the raw logs and unpacked data for LogResumeSigned events raised by the AutomatedMarketMaker contract.
type AutomatedMarketMakerLogResumeSignedIterator struct {
	Event *AutomatedMarketMakerLogResumeSigned // Event containing the contract specifics and raw log

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
func (it *AutomatedMarketMakerLogResumeSignedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(AutomatedMarketMakerLogResumeSigned)
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
		it.Event = new(AutomatedMarketMakerLogResumeSigned)
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
func (it *AutomatedMarketMakerLogResumeSignedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *AutomatedMarketMakerLogResumeSignedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// AutomatedMarketMakerLogResumeSigned represents a LogResumeSigned event raised by the AutomatedMarketMaker contract.
type AutomatedMarketMakerLogResumeSigned struct {
	ProposalId      [32]byte
	Signer          common.Address
	SignaturesCount *big.Int
	Raw             types.Log // Blockchain specific contextual infos
}

// FilterLogResumeSigned is a free log retrieval operation binding the contract event 0x81239dd1c0dbda2f6dc7c61c52c24f4f1f8540214d4ffd61ada9c51655aa15f7.
//
// Solidity: event LogResumeSigned(bytes32 indexed proposalId, address indexed signer, uint256 signaturesCount)
func (_AutomatedMarketMaker *AutomatedMarketMakerFilterer) FilterLogResumeSigned(opts *bind.FilterOpts, proposalId [][32]byte, signer []common.Address) (*AutomatedMarketMakerLogResumeSignedIterator, error) {

	var proposalIdRule []interface{}
	for _, proposalIdItem := range proposalId {
		proposalIdRule = append(proposalIdRule, proposalIdItem)
	}
	var signerRule []interface{}
	for _, signerItem := range signer {
		signerRule = append(signerRule, signerItem)
	}

	logs, sub, err := _AutomatedMarketMaker.contract.FilterLogs(opts, "LogResumeSigned", proposalIdRule, signerRule)
	if err != nil {
		return nil, err
	}
	return &AutomatedMarketMakerLogResumeSignedIterator{contract: _AutomatedMarketMaker.contract, event: "LogResumeSigned", logs: logs, sub: sub}, nil
}

// WatchLogResumeSigned is a free log subscription operation binding the contract event 0x81239dd1c0dbda2f6dc7c61c52c24f4f1f8540214d4ffd61ada9c51655aa15f7.
//
// Solidity: event LogResumeSigned(bytes32 indexed proposalId, address indexed signer, uint256 signaturesCount)
func (_AutomatedMarketMaker *AutomatedMarketMakerFilterer) WatchLogResumeSigned(opts *bind.WatchOpts, sink chan<- *AutomatedMarketMakerLogResumeSigned, proposalId [][32]byte, signer []common.Address) (event.Subscription, error) {

	var proposalIdRule []interface{}
	for _, proposalIdItem := range proposalId {
		proposalIdRule = append(proposalIdRule, proposalIdItem)
	}
	var signerRule []interface{}
	for _, signerItem := range signer {
		signerRule = append(signerRule, signerItem)
	}

	logs, sub, err := _AutomatedMarketMaker.contract.WatchLogs(opts, "LogResumeSigned", proposalIdRule, signerRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(AutomatedMarketMakerLogResumeSigned)
				if err := _AutomatedMarketMaker.contract.UnpackLog(event, "LogResumeSigned", log); err != nil {
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

// ParseLogResumeSigned is a log parse operation binding the contract event 0x81239dd1c0dbda2f6dc7c61c52c24f4f1f8540214d4ffd61ada9c51655aa15f7.
//
// Solidity: event LogResumeSigned(bytes32 indexed proposalId, address indexed signer, uint256 signaturesCount)
func (_AutomatedMarketMaker *AutomatedMarketMakerFilterer) ParseLogResumeSigned(log types.Log) (*AutomatedMarketMakerLogResumeSigned, error) {
	event := new(AutomatedMarketMakerLogResumeSigned)
	if err := _AutomatedMarketMaker.contract.UnpackLog(event, "LogResumeSigned", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// AutomatedMarketMakerLogSwapIterator is returned from FilterLogSwap and is used to iterate over the raw logs and unpacked data for LogSwap events raised by the AutomatedMarketMaker contract.
type AutomatedMarketMakerLogSwapIterator struct {
	Event *AutomatedMarketMakerLogSwap // Event containing the contract specifics and raw log

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
func (it *AutomatedMarketMakerLogSwapIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(AutomatedMarketMakerLogSwap)
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
		it.Event = new(AutomatedMarketMakerLogSwap)
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
func (it *AutomatedMarketMakerLogSwapIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *AutomatedMarketMakerLogSwapIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// AutomatedMarketMakerLogSwap represents a LogSwap event raised by the AutomatedMarketMaker contract.
type AutomatedMarketMakerLogSwap struct {
	User      common.Address
	TokenIn   common.Address
	TokenOut  common.Address
	AmountIn  *big.Int
	AmountOut *big.Int
	Raw       types.Log // Blockchain specific contextual infos
}

// FilterLogSwap is a free log retrieval operation binding the contract event 0x499f47d29fe8ad39124b5e7e7864cb954b8c73bb602f3853cac827a3128076d3.
//
// Solidity: event LogSwap(address indexed user, address indexed tokenIn, address indexed tokenOut, uint256 amountIn, uint256 amountOut)
func (_AutomatedMarketMaker *AutomatedMarketMakerFilterer) FilterLogSwap(opts *bind.FilterOpts, user []common.Address, tokenIn []common.Address, tokenOut []common.Address) (*AutomatedMarketMakerLogSwapIterator, error) {

	var userRule []interface{}
	for _, userItem := range user {
		userRule = append(userRule, userItem)
	}
	var tokenInRule []interface{}
	for _, tokenInItem := range tokenIn {
		tokenInRule = append(tokenInRule, tokenInItem)
	}
	var tokenOutRule []interface{}
	for _, tokenOutItem := range tokenOut {
		tokenOutRule = append(tokenOutRule, tokenOutItem)
	}

	logs, sub, err := _AutomatedMarketMaker.contract.FilterLogs(opts, "LogSwap", userRule, tokenInRule, tokenOutRule)
	if err != nil {
		return nil, err
	}
	return &AutomatedMarketMakerLogSwapIterator{contract: _AutomatedMarketMaker.contract, event: "LogSwap", logs: logs, sub: sub}, nil
}

// WatchLogSwap is a free log subscription operation binding the contract event 0x499f47d29fe8ad39124b5e7e7864cb954b8c73bb602f3853cac827a3128076d3.
//
// Solidity: event LogSwap(address indexed user, address indexed tokenIn, address indexed tokenOut, uint256 amountIn, uint256 amountOut)
func (_AutomatedMarketMaker *AutomatedMarketMakerFilterer) WatchLogSwap(opts *bind.WatchOpts, sink chan<- *AutomatedMarketMakerLogSwap, user []common.Address, tokenIn []common.Address, tokenOut []common.Address) (event.Subscription, error) {

	var userRule []interface{}
	for _, userItem := range user {
		userRule = append(userRule, userItem)
	}
	var tokenInRule []interface{}
	for _, tokenInItem := range tokenIn {
		tokenInRule = append(tokenInRule, tokenInItem)
	}
	var tokenOutRule []interface{}
	for _, tokenOutItem := range tokenOut {
		tokenOutRule = append(tokenOutRule, tokenOutItem)
	}

	logs, sub, err := _AutomatedMarketMaker.contract.WatchLogs(opts, "LogSwap", userRule, tokenInRule, tokenOutRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(AutomatedMarketMakerLogSwap)
				if err := _AutomatedMarketMaker.contract.UnpackLog(event, "LogSwap", log); err != nil {
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

// ParseLogSwap is a log parse operation binding the contract event 0x499f47d29fe8ad39124b5e7e7864cb954b8c73bb602f3853cac827a3128076d3.
//
// Solidity: event LogSwap(address indexed user, address indexed tokenIn, address indexed tokenOut, uint256 amountIn, uint256 amountOut)
func (_AutomatedMarketMaker *AutomatedMarketMakerFilterer) ParseLogSwap(log types.Log) (*AutomatedMarketMakerLogSwap, error) {
	event := new(AutomatedMarketMakerLogSwap)
	if err := _AutomatedMarketMaker.contract.UnpackLog(event, "LogSwap", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// AutomatedMarketMakerPausedIterator is returned from FilterPaused and is used to iterate over the raw logs and unpacked data for Paused events raised by the AutomatedMarketMaker contract.
type AutomatedMarketMakerPausedIterator struct {
	Event *AutomatedMarketMakerPaused // Event containing the contract specifics and raw log

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
func (it *AutomatedMarketMakerPausedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(AutomatedMarketMakerPaused)
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
		it.Event = new(AutomatedMarketMakerPaused)
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
func (it *AutomatedMarketMakerPausedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *AutomatedMarketMakerPausedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// AutomatedMarketMakerPaused represents a Paused event raised by the AutomatedMarketMaker contract.
type AutomatedMarketMakerPaused struct {
	Account common.Address
	Raw     types.Log // Blockchain specific contextual infos
}

// FilterPaused is a free log retrieval operation binding the contract event 0x62e78cea01bee320cd4e420270b5ea74000d11b0c9f74754ebdbfc544b05a258.
//
// Solidity: event Paused(address account)
func (_AutomatedMarketMaker *AutomatedMarketMakerFilterer) FilterPaused(opts *bind.FilterOpts) (*AutomatedMarketMakerPausedIterator, error) {

	logs, sub, err := _AutomatedMarketMaker.contract.FilterLogs(opts, "Paused")
	if err != nil {
		return nil, err
	}
	return &AutomatedMarketMakerPausedIterator{contract: _AutomatedMarketMaker.contract, event: "Paused", logs: logs, sub: sub}, nil
}

// WatchPaused is a free log subscription operation binding the contract event 0x62e78cea01bee320cd4e420270b5ea74000d11b0c9f74754ebdbfc544b05a258.
//
// Solidity: event Paused(address account)
func (_AutomatedMarketMaker *AutomatedMarketMakerFilterer) WatchPaused(opts *bind.WatchOpts, sink chan<- *AutomatedMarketMakerPaused) (event.Subscription, error) {

	logs, sub, err := _AutomatedMarketMaker.contract.WatchLogs(opts, "Paused")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(AutomatedMarketMakerPaused)
				if err := _AutomatedMarketMaker.contract.UnpackLog(event, "Paused", log); err != nil {
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

// ParsePaused is a log parse operation binding the contract event 0x62e78cea01bee320cd4e420270b5ea74000d11b0c9f74754ebdbfc544b05a258.
//
// Solidity: event Paused(address account)
func (_AutomatedMarketMaker *AutomatedMarketMakerFilterer) ParsePaused(log types.Log) (*AutomatedMarketMakerPaused, error) {
	event := new(AutomatedMarketMakerPaused)
	if err := _AutomatedMarketMaker.contract.UnpackLog(event, "Paused", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// AutomatedMarketMakerUnpausedIterator is returned from FilterUnpaused and is used to iterate over the raw logs and unpacked data for Unpaused events raised by the AutomatedMarketMaker contract.
type AutomatedMarketMakerUnpausedIterator struct {
	Event *AutomatedMarketMakerUnpaused // Event containing the contract specifics and raw log

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
func (it *AutomatedMarketMakerUnpausedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(AutomatedMarketMakerUnpaused)
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
		it.Event = new(AutomatedMarketMakerUnpaused)
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
func (it *AutomatedMarketMakerUnpausedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *AutomatedMarketMakerUnpausedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// AutomatedMarketMakerUnpaused represents a Unpaused event raised by the AutomatedMarketMaker contract.
type AutomatedMarketMakerUnpaused struct {
	Account common.Address
	Raw     types.Log // Blockchain specific contextual infos
}

// FilterUnpaused is a free log retrieval operation binding the contract event 0x5db9ee0a495bf2e6ff9c91a7834c1ba4fdd244a5e8aa4e537bd38aeae4b073aa.
//
// Solidity: event Unpaused(address account)
func (_AutomatedMarketMaker *AutomatedMarketMakerFilterer) FilterUnpaused(opts *bind.FilterOpts) (*AutomatedMarketMakerUnpausedIterator, error) {

	logs, sub, err := _AutomatedMarketMaker.contract.FilterLogs(opts, "Unpaused")
	if err != nil {
		return nil, err
	}
	return &AutomatedMarketMakerUnpausedIterator{contract: _AutomatedMarketMaker.contract, event: "Unpaused", logs: logs, sub: sub}, nil
}

// WatchUnpaused is a free log subscription operation binding the contract event 0x5db9ee0a495bf2e6ff9c91a7834c1ba4fdd244a5e8aa4e537bd38aeae4b073aa.
//
// Solidity: event Unpaused(address account)
func (_AutomatedMarketMaker *AutomatedMarketMakerFilterer) WatchUnpaused(opts *bind.WatchOpts, sink chan<- *AutomatedMarketMakerUnpaused) (event.Subscription, error) {

	logs, sub, err := _AutomatedMarketMaker.contract.WatchLogs(opts, "Unpaused")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(AutomatedMarketMakerUnpaused)
				if err := _AutomatedMarketMaker.contract.UnpackLog(event, "Unpaused", log); err != nil {
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

// ParseUnpaused is a log parse operation binding the contract event 0x5db9ee0a495bf2e6ff9c91a7834c1ba4fdd244a5e8aa4e537bd38aeae4b073aa.
//
// Solidity: event Unpaused(address account)
func (_AutomatedMarketMaker *AutomatedMarketMakerFilterer) ParseUnpaused(log types.Log) (*AutomatedMarketMakerUnpaused, error) {
	event := new(AutomatedMarketMakerUnpaused)
	if err := _AutomatedMarketMaker.contract.UnpackLog(event, "Unpaused", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}
