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

// ICurrencyRegistryCurrencyEntry is an auto generated low-level Go binding around an user-defined struct.
type ICurrencyRegistryCurrencyEntry struct {
	Symbol       string
	CountryName  string
	TokenAddress common.Address
	ProposerCB   string
}

// CurrencyRegistryMetaData contains all meta data concerning the CurrencyRegistry contract.
var CurrencyRegistryMetaData = &bind.MetaData{
	ABI: "[{\"type\":\"constructor\",\"inputs\":[{\"name\":\"registry\",\"type\":\"address\",\"internalType\":\"address\"}],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"REGISTRY\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"address\",\"internalType\":\"contractIIdentityRegistry\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"getAllCurrencies\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"tuple[]\",\"internalType\":\"structICurrencyRegistry.CurrencyEntry[]\",\"components\":[{\"name\":\"symbol\",\"type\":\"string\",\"internalType\":\"string\"},{\"name\":\"countryName\",\"type\":\"string\",\"internalType\":\"string\"},{\"name\":\"tokenAddress\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"proposerCB\",\"type\":\"string\",\"internalType\":\"string\"}]}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"getCurrency\",\"inputs\":[{\"name\":\"symbol\",\"type\":\"string\",\"internalType\":\"string\"}],\"outputs\":[{\"name\":\"\",\"type\":\"tuple\",\"internalType\":\"structICurrencyRegistry.CurrencyEntry\",\"components\":[{\"name\":\"symbol\",\"type\":\"string\",\"internalType\":\"string\"},{\"name\":\"countryName\",\"type\":\"string\",\"internalType\":\"string\"},{\"name\":\"tokenAddress\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"proposerCB\",\"type\":\"string\",\"internalType\":\"string\"}]}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"registerCurrency\",\"inputs\":[{\"name\":\"symbol\",\"type\":\"string\",\"internalType\":\"string\"},{\"name\":\"countryName\",\"type\":\"string\",\"internalType\":\"string\"},{\"name\":\"tokenAddress\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"proposerCB\",\"type\":\"string\",\"internalType\":\"string\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"removeCurrency\",\"inputs\":[{\"name\":\"symbol\",\"type\":\"string\",\"internalType\":\"string\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"event\",\"name\":\"CurrencyRegistered\",\"inputs\":[{\"name\":\"symbol\",\"type\":\"string\",\"indexed\":true,\"internalType\":\"string\"},{\"name\":\"tokenAddress\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"countryName\",\"type\":\"string\",\"indexed\":false,\"internalType\":\"string\"},{\"name\":\"proposerCB\",\"type\":\"string\",\"indexed\":false,\"internalType\":\"string\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"CurrencyRemoved\",\"inputs\":[{\"name\":\"symbol\",\"type\":\"string\",\"indexed\":true,\"internalType\":\"string\"},{\"name\":\"tokenAddress\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"}],\"anonymous\":false},{\"type\":\"error\",\"name\":\"CurrencyRegistry__AlreadyExists\",\"inputs\":[{\"name\":\"symbol\",\"type\":\"string\",\"internalType\":\"string\"}]},{\"type\":\"error\",\"name\":\"CurrencyRegistry__EmptyString\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"CurrencyRegistry__NotFound\",\"inputs\":[{\"name\":\"symbol\",\"type\":\"string\",\"internalType\":\"string\"}]},{\"type\":\"error\",\"name\":\"CurrencyRegistry__TokenAlreadyRegistered\",\"inputs\":[{\"name\":\"token\",\"type\":\"address\",\"internalType\":\"address\"}]},{\"type\":\"error\",\"name\":\"CurrencyRegistry__Unauthorized\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"CurrencyRegistry__ZeroAddress\",\"inputs\":[]}]",
}

// CurrencyRegistryABI is the input ABI used to generate the binding from.
// Deprecated: Use CurrencyRegistryMetaData.ABI instead.
var CurrencyRegistryABI = CurrencyRegistryMetaData.ABI

// CurrencyRegistry is an auto generated Go binding around an Ethereum contract.
type CurrencyRegistry struct {
	CurrencyRegistryCaller     // Read-only binding to the contract
	CurrencyRegistryTransactor // Write-only binding to the contract
	CurrencyRegistryFilterer   // Log filterer for contract events
}

// CurrencyRegistryCaller is an auto generated read-only Go binding around an Ethereum contract.
type CurrencyRegistryCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// CurrencyRegistryTransactor is an auto generated write-only Go binding around an Ethereum contract.
type CurrencyRegistryTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// CurrencyRegistryFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type CurrencyRegistryFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// CurrencyRegistrySession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type CurrencyRegistrySession struct {
	Contract     *CurrencyRegistry // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// CurrencyRegistryCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type CurrencyRegistryCallerSession struct {
	Contract *CurrencyRegistryCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts           // Call options to use throughout this session
}

// CurrencyRegistryTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type CurrencyRegistryTransactorSession struct {
	Contract     *CurrencyRegistryTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts           // Transaction auth options to use throughout this session
}

// CurrencyRegistryRaw is an auto generated low-level Go binding around an Ethereum contract.
type CurrencyRegistryRaw struct {
	Contract *CurrencyRegistry // Generic contract binding to access the raw methods on
}

// CurrencyRegistryCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type CurrencyRegistryCallerRaw struct {
	Contract *CurrencyRegistryCaller // Generic read-only contract binding to access the raw methods on
}

// CurrencyRegistryTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type CurrencyRegistryTransactorRaw struct {
	Contract *CurrencyRegistryTransactor // Generic write-only contract binding to access the raw methods on
}

// NewCurrencyRegistry creates a new instance of CurrencyRegistry, bound to a specific deployed contract.
func NewCurrencyRegistry(address common.Address, backend bind.ContractBackend) (*CurrencyRegistry, error) {
	contract, err := bindCurrencyRegistry(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &CurrencyRegistry{CurrencyRegistryCaller: CurrencyRegistryCaller{contract: contract}, CurrencyRegistryTransactor: CurrencyRegistryTransactor{contract: contract}, CurrencyRegistryFilterer: CurrencyRegistryFilterer{contract: contract}}, nil
}

// NewCurrencyRegistryCaller creates a new read-only instance of CurrencyRegistry, bound to a specific deployed contract.
func NewCurrencyRegistryCaller(address common.Address, caller bind.ContractCaller) (*CurrencyRegistryCaller, error) {
	contract, err := bindCurrencyRegistry(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &CurrencyRegistryCaller{contract: contract}, nil
}

// NewCurrencyRegistryTransactor creates a new write-only instance of CurrencyRegistry, bound to a specific deployed contract.
func NewCurrencyRegistryTransactor(address common.Address, transactor bind.ContractTransactor) (*CurrencyRegistryTransactor, error) {
	contract, err := bindCurrencyRegistry(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &CurrencyRegistryTransactor{contract: contract}, nil
}

// NewCurrencyRegistryFilterer creates a new log filterer instance of CurrencyRegistry, bound to a specific deployed contract.
func NewCurrencyRegistryFilterer(address common.Address, filterer bind.ContractFilterer) (*CurrencyRegistryFilterer, error) {
	contract, err := bindCurrencyRegistry(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &CurrencyRegistryFilterer{contract: contract}, nil
}

// bindCurrencyRegistry binds a generic wrapper to an already deployed contract.
func bindCurrencyRegistry(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := CurrencyRegistryMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, *parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_CurrencyRegistry *CurrencyRegistryRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _CurrencyRegistry.Contract.CurrencyRegistryCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_CurrencyRegistry *CurrencyRegistryRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _CurrencyRegistry.Contract.CurrencyRegistryTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_CurrencyRegistry *CurrencyRegistryRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _CurrencyRegistry.Contract.CurrencyRegistryTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_CurrencyRegistry *CurrencyRegistryCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _CurrencyRegistry.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_CurrencyRegistry *CurrencyRegistryTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _CurrencyRegistry.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_CurrencyRegistry *CurrencyRegistryTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _CurrencyRegistry.Contract.contract.Transact(opts, method, params...)
}

// REGISTRY is a free data retrieval call binding the contract method 0x06433b1b.
//
// Solidity: function REGISTRY() view returns(address)
func (_CurrencyRegistry *CurrencyRegistryCaller) REGISTRY(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _CurrencyRegistry.contract.Call(opts, &out, "REGISTRY")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// REGISTRY is a free data retrieval call binding the contract method 0x06433b1b.
//
// Solidity: function REGISTRY() view returns(address)
func (_CurrencyRegistry *CurrencyRegistrySession) REGISTRY() (common.Address, error) {
	return _CurrencyRegistry.Contract.REGISTRY(&_CurrencyRegistry.CallOpts)
}

// REGISTRY is a free data retrieval call binding the contract method 0x06433b1b.
//
// Solidity: function REGISTRY() view returns(address)
func (_CurrencyRegistry *CurrencyRegistryCallerSession) REGISTRY() (common.Address, error) {
	return _CurrencyRegistry.Contract.REGISTRY(&_CurrencyRegistry.CallOpts)
}

// GetAllCurrencies is a free data retrieval call binding the contract method 0x47cb4b72.
//
// Solidity: function getAllCurrencies() view returns((string,string,address,string)[])
func (_CurrencyRegistry *CurrencyRegistryCaller) GetAllCurrencies(opts *bind.CallOpts) ([]ICurrencyRegistryCurrencyEntry, error) {
	var out []interface{}
	err := _CurrencyRegistry.contract.Call(opts, &out, "getAllCurrencies")

	if err != nil {
		return *new([]ICurrencyRegistryCurrencyEntry), err
	}

	out0 := *abi.ConvertType(out[0], new([]ICurrencyRegistryCurrencyEntry)).(*[]ICurrencyRegistryCurrencyEntry)

	return out0, err

}

// GetAllCurrencies is a free data retrieval call binding the contract method 0x47cb4b72.
//
// Solidity: function getAllCurrencies() view returns((string,string,address,string)[])
func (_CurrencyRegistry *CurrencyRegistrySession) GetAllCurrencies() ([]ICurrencyRegistryCurrencyEntry, error) {
	return _CurrencyRegistry.Contract.GetAllCurrencies(&_CurrencyRegistry.CallOpts)
}

// GetAllCurrencies is a free data retrieval call binding the contract method 0x47cb4b72.
//
// Solidity: function getAllCurrencies() view returns((string,string,address,string)[])
func (_CurrencyRegistry *CurrencyRegistryCallerSession) GetAllCurrencies() ([]ICurrencyRegistryCurrencyEntry, error) {
	return _CurrencyRegistry.Contract.GetAllCurrencies(&_CurrencyRegistry.CallOpts)
}

// GetCurrency is a free data retrieval call binding the contract method 0xf8066d6b.
//
// Solidity: function getCurrency(string symbol) view returns((string,string,address,string))
func (_CurrencyRegistry *CurrencyRegistryCaller) GetCurrency(opts *bind.CallOpts, symbol string) (ICurrencyRegistryCurrencyEntry, error) {
	var out []interface{}
	err := _CurrencyRegistry.contract.Call(opts, &out, "getCurrency", symbol)

	if err != nil {
		return *new(ICurrencyRegistryCurrencyEntry), err
	}

	out0 := *abi.ConvertType(out[0], new(ICurrencyRegistryCurrencyEntry)).(*ICurrencyRegistryCurrencyEntry)

	return out0, err

}

// GetCurrency is a free data retrieval call binding the contract method 0xf8066d6b.
//
// Solidity: function getCurrency(string symbol) view returns((string,string,address,string))
func (_CurrencyRegistry *CurrencyRegistrySession) GetCurrency(symbol string) (ICurrencyRegistryCurrencyEntry, error) {
	return _CurrencyRegistry.Contract.GetCurrency(&_CurrencyRegistry.CallOpts, symbol)
}

// GetCurrency is a free data retrieval call binding the contract method 0xf8066d6b.
//
// Solidity: function getCurrency(string symbol) view returns((string,string,address,string))
func (_CurrencyRegistry *CurrencyRegistryCallerSession) GetCurrency(symbol string) (ICurrencyRegistryCurrencyEntry, error) {
	return _CurrencyRegistry.Contract.GetCurrency(&_CurrencyRegistry.CallOpts, symbol)
}

// RegisterCurrency is a paid mutator transaction binding the contract method 0x50f9076b.
//
// Solidity: function registerCurrency(string symbol, string countryName, address tokenAddress, string proposerCB) returns()
func (_CurrencyRegistry *CurrencyRegistryTransactor) RegisterCurrency(opts *bind.TransactOpts, symbol string, countryName string, tokenAddress common.Address, proposerCB string) (*types.Transaction, error) {
	return _CurrencyRegistry.contract.Transact(opts, "registerCurrency", symbol, countryName, tokenAddress, proposerCB)
}

// RegisterCurrency is a paid mutator transaction binding the contract method 0x50f9076b.
//
// Solidity: function registerCurrency(string symbol, string countryName, address tokenAddress, string proposerCB) returns()
func (_CurrencyRegistry *CurrencyRegistrySession) RegisterCurrency(symbol string, countryName string, tokenAddress common.Address, proposerCB string) (*types.Transaction, error) {
	return _CurrencyRegistry.Contract.RegisterCurrency(&_CurrencyRegistry.TransactOpts, symbol, countryName, tokenAddress, proposerCB)
}

// RegisterCurrency is a paid mutator transaction binding the contract method 0x50f9076b.
//
// Solidity: function registerCurrency(string symbol, string countryName, address tokenAddress, string proposerCB) returns()
func (_CurrencyRegistry *CurrencyRegistryTransactorSession) RegisterCurrency(symbol string, countryName string, tokenAddress common.Address, proposerCB string) (*types.Transaction, error) {
	return _CurrencyRegistry.Contract.RegisterCurrency(&_CurrencyRegistry.TransactOpts, symbol, countryName, tokenAddress, proposerCB)
}

// RemoveCurrency is a paid mutator transaction binding the contract method 0x9096c29a.
//
// Solidity: function removeCurrency(string symbol) returns()
func (_CurrencyRegistry *CurrencyRegistryTransactor) RemoveCurrency(opts *bind.TransactOpts, symbol string) (*types.Transaction, error) {
	return _CurrencyRegistry.contract.Transact(opts, "removeCurrency", symbol)
}

// RemoveCurrency is a paid mutator transaction binding the contract method 0x9096c29a.
//
// Solidity: function removeCurrency(string symbol) returns()
func (_CurrencyRegistry *CurrencyRegistrySession) RemoveCurrency(symbol string) (*types.Transaction, error) {
	return _CurrencyRegistry.Contract.RemoveCurrency(&_CurrencyRegistry.TransactOpts, symbol)
}

// RemoveCurrency is a paid mutator transaction binding the contract method 0x9096c29a.
//
// Solidity: function removeCurrency(string symbol) returns()
func (_CurrencyRegistry *CurrencyRegistryTransactorSession) RemoveCurrency(symbol string) (*types.Transaction, error) {
	return _CurrencyRegistry.Contract.RemoveCurrency(&_CurrencyRegistry.TransactOpts, symbol)
}

// CurrencyRegistryCurrencyRegisteredIterator is returned from FilterCurrencyRegistered and is used to iterate over the raw logs and unpacked data for CurrencyRegistered events raised by the CurrencyRegistry contract.
type CurrencyRegistryCurrencyRegisteredIterator struct {
	Event *CurrencyRegistryCurrencyRegistered // Event containing the contract specifics and raw log

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
func (it *CurrencyRegistryCurrencyRegisteredIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(CurrencyRegistryCurrencyRegistered)
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
		it.Event = new(CurrencyRegistryCurrencyRegistered)
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
func (it *CurrencyRegistryCurrencyRegisteredIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *CurrencyRegistryCurrencyRegisteredIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// CurrencyRegistryCurrencyRegistered represents a CurrencyRegistered event raised by the CurrencyRegistry contract.
type CurrencyRegistryCurrencyRegistered struct {
	Symbol       common.Hash
	TokenAddress common.Address
	CountryName  string
	ProposerCB   string
	Raw          types.Log // Blockchain specific contextual infos
}

// FilterCurrencyRegistered is a free log retrieval operation binding the contract event 0x782d60d681f250a36f452231cb1f681565852ec340c6fe4d61d9dfec8e3e92bb.
//
// Solidity: event CurrencyRegistered(string indexed symbol, address indexed tokenAddress, string countryName, string proposerCB)
func (_CurrencyRegistry *CurrencyRegistryFilterer) FilterCurrencyRegistered(opts *bind.FilterOpts, symbol []string, tokenAddress []common.Address) (*CurrencyRegistryCurrencyRegisteredIterator, error) {

	var symbolRule []interface{}
	for _, symbolItem := range symbol {
		symbolRule = append(symbolRule, symbolItem)
	}
	var tokenAddressRule []interface{}
	for _, tokenAddressItem := range tokenAddress {
		tokenAddressRule = append(tokenAddressRule, tokenAddressItem)
	}

	logs, sub, err := _CurrencyRegistry.contract.FilterLogs(opts, "CurrencyRegistered", symbolRule, tokenAddressRule)
	if err != nil {
		return nil, err
	}
	return &CurrencyRegistryCurrencyRegisteredIterator{contract: _CurrencyRegistry.contract, event: "CurrencyRegistered", logs: logs, sub: sub}, nil
}

// WatchCurrencyRegistered is a free log subscription operation binding the contract event 0x782d60d681f250a36f452231cb1f681565852ec340c6fe4d61d9dfec8e3e92bb.
//
// Solidity: event CurrencyRegistered(string indexed symbol, address indexed tokenAddress, string countryName, string proposerCB)
func (_CurrencyRegistry *CurrencyRegistryFilterer) WatchCurrencyRegistered(opts *bind.WatchOpts, sink chan<- *CurrencyRegistryCurrencyRegistered, symbol []string, tokenAddress []common.Address) (event.Subscription, error) {

	var symbolRule []interface{}
	for _, symbolItem := range symbol {
		symbolRule = append(symbolRule, symbolItem)
	}
	var tokenAddressRule []interface{}
	for _, tokenAddressItem := range tokenAddress {
		tokenAddressRule = append(tokenAddressRule, tokenAddressItem)
	}

	logs, sub, err := _CurrencyRegistry.contract.WatchLogs(opts, "CurrencyRegistered", symbolRule, tokenAddressRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(CurrencyRegistryCurrencyRegistered)
				if err := _CurrencyRegistry.contract.UnpackLog(event, "CurrencyRegistered", log); err != nil {
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

// ParseCurrencyRegistered is a log parse operation binding the contract event 0x782d60d681f250a36f452231cb1f681565852ec340c6fe4d61d9dfec8e3e92bb.
//
// Solidity: event CurrencyRegistered(string indexed symbol, address indexed tokenAddress, string countryName, string proposerCB)
func (_CurrencyRegistry *CurrencyRegistryFilterer) ParseCurrencyRegistered(log types.Log) (*CurrencyRegistryCurrencyRegistered, error) {
	event := new(CurrencyRegistryCurrencyRegistered)
	if err := _CurrencyRegistry.contract.UnpackLog(event, "CurrencyRegistered", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// CurrencyRegistryCurrencyRemovedIterator is returned from FilterCurrencyRemoved and is used to iterate over the raw logs and unpacked data for CurrencyRemoved events raised by the CurrencyRegistry contract.
type CurrencyRegistryCurrencyRemovedIterator struct {
	Event *CurrencyRegistryCurrencyRemoved // Event containing the contract specifics and raw log

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
func (it *CurrencyRegistryCurrencyRemovedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(CurrencyRegistryCurrencyRemoved)
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
		it.Event = new(CurrencyRegistryCurrencyRemoved)
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
func (it *CurrencyRegistryCurrencyRemovedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *CurrencyRegistryCurrencyRemovedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// CurrencyRegistryCurrencyRemoved represents a CurrencyRemoved event raised by the CurrencyRegistry contract.
type CurrencyRegistryCurrencyRemoved struct {
	Symbol       common.Hash
	TokenAddress common.Address
	Raw          types.Log // Blockchain specific contextual infos
}

// FilterCurrencyRemoved is a free log retrieval operation binding the contract event 0xe37c7601abec3be323a676514c32d8526a72aa74854fd57bd2418e4c2207d1c6.
//
// Solidity: event CurrencyRemoved(string indexed symbol, address indexed tokenAddress)
func (_CurrencyRegistry *CurrencyRegistryFilterer) FilterCurrencyRemoved(opts *bind.FilterOpts, symbol []string, tokenAddress []common.Address) (*CurrencyRegistryCurrencyRemovedIterator, error) {

	var symbolRule []interface{}
	for _, symbolItem := range symbol {
		symbolRule = append(symbolRule, symbolItem)
	}
	var tokenAddressRule []interface{}
	for _, tokenAddressItem := range tokenAddress {
		tokenAddressRule = append(tokenAddressRule, tokenAddressItem)
	}

	logs, sub, err := _CurrencyRegistry.contract.FilterLogs(opts, "CurrencyRemoved", symbolRule, tokenAddressRule)
	if err != nil {
		return nil, err
	}
	return &CurrencyRegistryCurrencyRemovedIterator{contract: _CurrencyRegistry.contract, event: "CurrencyRemoved", logs: logs, sub: sub}, nil
}

// WatchCurrencyRemoved is a free log subscription operation binding the contract event 0xe37c7601abec3be323a676514c32d8526a72aa74854fd57bd2418e4c2207d1c6.
//
// Solidity: event CurrencyRemoved(string indexed symbol, address indexed tokenAddress)
func (_CurrencyRegistry *CurrencyRegistryFilterer) WatchCurrencyRemoved(opts *bind.WatchOpts, sink chan<- *CurrencyRegistryCurrencyRemoved, symbol []string, tokenAddress []common.Address) (event.Subscription, error) {

	var symbolRule []interface{}
	for _, symbolItem := range symbol {
		symbolRule = append(symbolRule, symbolItem)
	}
	var tokenAddressRule []interface{}
	for _, tokenAddressItem := range tokenAddress {
		tokenAddressRule = append(tokenAddressRule, tokenAddressItem)
	}

	logs, sub, err := _CurrencyRegistry.contract.WatchLogs(opts, "CurrencyRemoved", symbolRule, tokenAddressRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(CurrencyRegistryCurrencyRemoved)
				if err := _CurrencyRegistry.contract.UnpackLog(event, "CurrencyRemoved", log); err != nil {
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

// ParseCurrencyRemoved is a log parse operation binding the contract event 0xe37c7601abec3be323a676514c32d8526a72aa74854fd57bd2418e4c2207d1c6.
//
// Solidity: event CurrencyRemoved(string indexed symbol, address indexed tokenAddress)
func (_CurrencyRegistry *CurrencyRegistryFilterer) ParseCurrencyRemoved(log types.Log) (*CurrencyRegistryCurrencyRemoved, error) {
	event := new(CurrencyRegistryCurrencyRemoved)
	if err := _CurrencyRegistry.contract.UnpackLog(event, "CurrencyRemoved", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}
