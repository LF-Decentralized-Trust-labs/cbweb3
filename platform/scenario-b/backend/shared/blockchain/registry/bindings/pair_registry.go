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

// PairRegistryPairEntry is an auto generated low-level Go binding around an user-defined struct.
type PairRegistryPairEntry struct {
	PairId     string
	AmmAddress common.Address
	TokenA     common.Address
	TokenB     common.Address
	Status     uint8
	Proposer   common.Address
	Confirmer  common.Address
}

// PairRegistryMetaData contains all meta data concerning the PairRegistry contract.
var PairRegistryMetaData = &bind.MetaData{
	ABI: "[{\"type\":\"constructor\",\"inputs\":[{\"name\":\"registry\",\"type\":\"address\",\"internalType\":\"address\"}],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"REGISTRY\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"address\",\"internalType\":\"contractIIdentityRegistry\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"confirmPair\",\"inputs\":[{\"name\":\"pairId\",\"type\":\"string\",\"internalType\":\"string\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"getAllActivePairs\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"tuple[]\",\"internalType\":\"structPairRegistry.PairEntry[]\",\"components\":[{\"name\":\"pairId\",\"type\":\"string\",\"internalType\":\"string\"},{\"name\":\"ammAddress\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"tokenA\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"tokenB\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"status\",\"type\":\"uint8\",\"internalType\":\"enumPairRegistry.PairStatus\"},{\"name\":\"proposer\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"confirmer\",\"type\":\"address\",\"internalType\":\"address\"}]}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"getPair\",\"inputs\":[{\"name\":\"pairId\",\"type\":\"string\",\"internalType\":\"string\"}],\"outputs\":[{\"name\":\"\",\"type\":\"tuple\",\"internalType\":\"structPairRegistry.PairEntry\",\"components\":[{\"name\":\"pairId\",\"type\":\"string\",\"internalType\":\"string\"},{\"name\":\"ammAddress\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"tokenA\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"tokenB\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"status\",\"type\":\"uint8\",\"internalType\":\"enumPairRegistry.PairStatus\"},{\"name\":\"proposer\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"confirmer\",\"type\":\"address\",\"internalType\":\"address\"}]}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"proposePair\",\"inputs\":[{\"name\":\"pairId\",\"type\":\"string\",\"internalType\":\"string\"},{\"name\":\"tokenA\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"tokenB\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"ammAddress\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"event\",\"name\":\"PairProposed\",\"inputs\":[{\"name\":\"pairId\",\"type\":\"string\",\"indexed\":true,\"internalType\":\"string\"},{\"name\":\"proposer\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"tokenA\",\"type\":\"address\",\"indexed\":false,\"internalType\":\"address\"},{\"name\":\"tokenB\",\"type\":\"address\",\"indexed\":false,\"internalType\":\"address\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"PairRegistered\",\"inputs\":[{\"name\":\"pairId\",\"type\":\"string\",\"indexed\":true,\"internalType\":\"string\"},{\"name\":\"ammAddress\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"tokenA\",\"type\":\"address\",\"indexed\":false,\"internalType\":\"address\"},{\"name\":\"tokenB\",\"type\":\"address\",\"indexed\":false,\"internalType\":\"address\"}],\"anonymous\":false},{\"type\":\"error\",\"name\":\"PairRegistry__AlreadyActive\",\"inputs\":[{\"name\":\"pairId\",\"type\":\"string\",\"internalType\":\"string\"}]},{\"type\":\"error\",\"name\":\"PairRegistry__AlreadyExists\",\"inputs\":[{\"name\":\"pairId\",\"type\":\"string\",\"internalType\":\"string\"}]},{\"type\":\"error\",\"name\":\"PairRegistry__NotFound\",\"inputs\":[{\"name\":\"pairId\",\"type\":\"string\",\"internalType\":\"string\"}]},{\"type\":\"error\",\"name\":\"PairRegistry__Unauthorized\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"PairRegistry__ZeroAddress\",\"inputs\":[]}]",
	Bin: "0x60a03461009257601f610f3838819003918201601f19168301916001600160401b038311848410176100975780849260209460405283398101031261009257516001600160a01b0381169081900361009257801561008057608052604051610e8a90816100ae82396080518181816102f4015281816105db0152610a080152f35b6040516312f0c94760e21b8152600490fd5b600080fd5b634e487b7160e01b600052604160045260246000fdfe608080604052600436101561001357600080fd5b60003560e01c90816306433b1b146109f55750806352e9b130146104ff57806374adf539146104515780639d4c1a211461024557639e7c150f1461005657600080fd5b34610240576000366003190112610240576000806001908154905b8181106101dd575061008283610d79565b926100906040519485610b23565b80845261009f601f1991610d79565b0160005b8181106101be5750506000805b82821061011b57505050604051906020808301818452845180915260408401918060408360051b8701019601926000905b8382106100ee5786880387f35b9091929394838061010a839a603f198b82030186528951610a65565b9997019594939190910191016100e1565b600061013361012d8497959697610bd3565b50610cd3565b805160208092012082525260406000209160ff600384015460a01c1660028110156101a8578614610174575b61016a919250610cae565b90939291936100b0565b61019f8261018e61018861016a9592610cae565b95610dde565b6101988288610dca565b5285610dca565b5082915061015f565b634e487b7160e01b600052602160045260246000fd5b6020906101cd95939495610d91565b82828601015201939291936100a3565b60006101ef61012d8396949596610bd3565b805160208092012082525260ff60036040600020015460a01c1660028110156101a857841461022b575b61022290610cae565b92919092610071565b9061023861022291610cae565b919050610219565b600080fd5b34610240576020806003193601126102405760043567ffffffffffffffff811161024057610277903690600401610a37565b610282368284610b45565b838151910120806000526002845260ff60406000205416156104335760005260008352604060002092600384019182549260ff8460a01c1660028110156101a857600114610410576040516320ed82ef60e11b81526001600160a01b03858116600483018190529490929181816024817f000000000000000000000000000000000000000000000000000000000000000088165afa9081156104045784926000926103d7575b50501633036103c5577f1bdd6e3c097fae9959c0e8f7088a7a41aa56ee949b35a46c3dce5e5db2aff67e94600160a01b9060ff60a01b191617905560058601336bffffffffffffffffffffffff60a01b82541617905560028160018801541696015416938160405192839283376000908201908152039020604080516001600160a01b03958616815292909416602083015292819081015b0390a3005b604051631aa7171b60e21b8152600490fd5b6103f69250803d106103fd575b6103ee8183610b23565b810190610bb4565b8980610328565b503d6103e4565b6040513d6000823e3d90fd5b50604051634ebc719d60e01b815290819061042f908660048401610b8c565b0390fd5b5060405163176f542760e11b815291829161042f9160048401610b8c565b34610240576020806003193601126102405760043567ffffffffffffffff811161024057610483903690600401610a37565b61048b610d91565b50610497368284610b45565b83815191012091826000526002845260ff60406000205416156104e3575050600052600081526104ca6040600020610dde565b906104df604051928284938452830190610a65565b0390f35b61042f60405192839263176f542760e11b845260048401610b8c565b346102405760803660031901126102405760043567ffffffffffffffff811161024057610530903690600401610a37565b906024356001600160a01b0381169003610240576044356001600160a01b0381169003610240576064356001600160a01b0381168103610240576024356001600160a01b03161580156109e2575b80156109d1575b6109bf57610594368484610b45565b6020815191012080600052600260205260ff604060002054166109a1576040516320ed82ef60e11b8152602480356001600160a01b039081166004840152602091839182907f0000000000000000000000000000000000000000000000000000000000000000165afa90811561040457600091610982575b506001600160a01b031633036103c5578060005260026020526040600020600160ff19825416179055600154680100000000000000008110156108e6578060016106599201600155610bd3565b61096c5767ffffffffffffffff85116108e6576106808561067a8354610c20565b83610c5a565b846000601f8211600114610907576000916108fc575b508560011b906000198760031b1c19161790555b604051916106b783610b07565b6106c2368686610b45565b835260018060a01b0316602083015260018060a01b0360243516604083015260018060a01b03604435166060830152600060808301523360a0830152600060c083015260005260006020526040600020815180519067ffffffffffffffff82116108e657819061073c826107368654610c20565b86610c5a565b602090601f831160011461088057600092610875575b50508160011b916000199060031b1c19161781555b60208201516001820180546001600160a01b03199081166001600160a01b0393841617909155604084015160028085018054841692851692909217909155606085015160038501805460808801519496949591939092909116908510156101a85760ff60a01b60a095861b166001600160a81b03199290921617179055908301516004820180546001600160a01b0392831690851617905560c09093015160059091018054919093169116179055604051918291819083376000908201908152039020604080516001600160a01b03602435811682526044351660208201523392917f81b6c2eabb42da5f8e9bc4f181b131ca906df3ceb6ea7284dfd80cc6818d01449190819081016103c0565b015190508680610752565b6000858152602081209350601f198516905b8181106108ce57509084600195949392106108b5575b505050811b018155610767565b015160001960f88460031b161c191690558680806108a8565b92936020600181928786015181550195019301610892565b634e487b7160e01b600052604160045260246000fd5b905084013586610696565b60008381526020812092505b601f1988168110610954575086601f1981161061093a575b5050600185811b0190556106aa565b850135600019600388901b60f8161c19169055858061092b565b9091602060018192858a013581550193019101610913565b634e487b7160e01b600052600060045260246000fd5b61099b915060203d6020116103fd576103ee8183610b23565b8561060c565b505061042f604051928392630ebc1d2d60e41b845260048401610b8c565b6040516312f0c94760e21b8152600490fd5b506001600160a01b03811615610585565b506044356001600160a01b03161561057e565b34610240576000366003190112610240577f00000000000000000000000000000000000000000000000000000000000000006001600160a01b03168152602090f35b9181601f840112156102405782359167ffffffffffffffff8311610240576020838186019501011161024057565b90815160e082528051908160e084015260005b828110610af057505061010092600084838501015260018060a01b039081602082015116602085015281604082015116604085015281606082015116606085015260808101519060028210156101a85760c09160808601528260a08201511660a086015201511660c0830152601f8019910116010190565b806020809284010151610100828701015201610a78565b60e0810190811067ffffffffffffffff8211176108e657604052565b90601f8019910116810190811067ffffffffffffffff8211176108e657604052565b92919267ffffffffffffffff82116108e65760405191610b6f601f8201601f191660200184610b23565b829481845281830111610240578281602093846000960137010152565b90918060409360208452816020850152848401376000828201840152601f01601f1916010190565b9081602091031261024057516001600160a01b03811681036102405790565b600154811015610c0a5760016000527fb10e2d527612073b26eecdfd717e6a320cf44b4afac2b0732d9fcbe2b7fa0cf60190600090565b634e487b7160e01b600052603260045260246000fd5b90600182811c92168015610c50575b6020831014610c3a57565b634e487b7160e01b600052602260045260246000fd5b91607f1691610c2f565b90601f8111610c6857505050565b600091825260208220906020601f850160051c83019410610ca4575b601f0160051c01915b828110610c9957505050565b818155600101610c8d565b9092508290610c84565b6000198114610cbd5760010190565b634e487b7160e01b600052601160045260246000fd5b9060405191826000825492610ce784610c20565b908184526001948581169081600014610d565750600114610d13575b5050610d1192500383610b23565b565b9093915060005260209081600020936000915b818310610d3e575050610d1193508201013880610d03565b85548884018501529485019487945091830191610d26565b915050610d1194506020925060ff191682840152151560051b8201013880610d03565b67ffffffffffffffff81116108e65760051b60200190565b60405190610d9e82610b07565b816060815260c06000918260208201528260408201528260608201528260808201528260a08201520152565b8051821015610c0a5760209160051b010190565b90604051610deb81610b07565b8092610df681610cd3565b825260018101546001600160a01b039081166020840152600280830154821660408501526003830154808316606086015260a01c60ff16908110156101a85760c09260059160808601528260048201541660a086015201541691015256fea26469706673582212206e6274f7570948b8230a2d9b0ec7952cfe8797bcb6d3f26d124f898b369f6e5664736f6c63430008140033",
}

// PairRegistryABI is the input ABI used to generate the binding from.
// Deprecated: Use PairRegistryMetaData.ABI instead.
var PairRegistryABI = PairRegistryMetaData.ABI

// PairRegistryBin is the compiled bytecode used for deploying new contracts.
// Deprecated: Use PairRegistryMetaData.Bin instead.
var PairRegistryBin = PairRegistryMetaData.Bin

// DeployPairRegistry deploys a new Ethereum contract, binding an instance of PairRegistry to it.
func DeployPairRegistry(auth *bind.TransactOpts, backend bind.ContractBackend, registry common.Address) (common.Address, *types.Transaction, *PairRegistry, error) {
	parsed, err := PairRegistryMetaData.GetAbi()
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	if parsed == nil {
		return common.Address{}, nil, nil, errors.New("GetABI returned nil")
	}

	address, tx, contract, err := bind.DeployContract(auth, *parsed, common.FromHex(PairRegistryBin), backend, registry)
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	return address, tx, &PairRegistry{PairRegistryCaller: PairRegistryCaller{contract: contract}, PairRegistryTransactor: PairRegistryTransactor{contract: contract}, PairRegistryFilterer: PairRegistryFilterer{contract: contract}}, nil
}

// PairRegistry is an auto generated Go binding around an Ethereum contract.
type PairRegistry struct {
	PairRegistryCaller     // Read-only binding to the contract
	PairRegistryTransactor // Write-only binding to the contract
	PairRegistryFilterer   // Log filterer for contract events
}

// PairRegistryCaller is an auto generated read-only Go binding around an Ethereum contract.
type PairRegistryCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// PairRegistryTransactor is an auto generated write-only Go binding around an Ethereum contract.
type PairRegistryTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// PairRegistryFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type PairRegistryFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// PairRegistrySession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type PairRegistrySession struct {
	Contract     *PairRegistry     // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// PairRegistryCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type PairRegistryCallerSession struct {
	Contract *PairRegistryCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts       // Call options to use throughout this session
}

// PairRegistryTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type PairRegistryTransactorSession struct {
	Contract     *PairRegistryTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts       // Transaction auth options to use throughout this session
}

// PairRegistryRaw is an auto generated low-level Go binding around an Ethereum contract.
type PairRegistryRaw struct {
	Contract *PairRegistry // Generic contract binding to access the raw methods on
}

// PairRegistryCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type PairRegistryCallerRaw struct {
	Contract *PairRegistryCaller // Generic read-only contract binding to access the raw methods on
}

// PairRegistryTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type PairRegistryTransactorRaw struct {
	Contract *PairRegistryTransactor // Generic write-only contract binding to access the raw methods on
}

// NewPairRegistry creates a new instance of PairRegistry, bound to a specific deployed contract.
func NewPairRegistry(address common.Address, backend bind.ContractBackend) (*PairRegistry, error) {
	contract, err := bindPairRegistry(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &PairRegistry{PairRegistryCaller: PairRegistryCaller{contract: contract}, PairRegistryTransactor: PairRegistryTransactor{contract: contract}, PairRegistryFilterer: PairRegistryFilterer{contract: contract}}, nil
}

// NewPairRegistryCaller creates a new read-only instance of PairRegistry, bound to a specific deployed contract.
func NewPairRegistryCaller(address common.Address, caller bind.ContractCaller) (*PairRegistryCaller, error) {
	contract, err := bindPairRegistry(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &PairRegistryCaller{contract: contract}, nil
}

// NewPairRegistryTransactor creates a new write-only instance of PairRegistry, bound to a specific deployed contract.
func NewPairRegistryTransactor(address common.Address, transactor bind.ContractTransactor) (*PairRegistryTransactor, error) {
	contract, err := bindPairRegistry(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &PairRegistryTransactor{contract: contract}, nil
}

// NewPairRegistryFilterer creates a new log filterer instance of PairRegistry, bound to a specific deployed contract.
func NewPairRegistryFilterer(address common.Address, filterer bind.ContractFilterer) (*PairRegistryFilterer, error) {
	contract, err := bindPairRegistry(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &PairRegistryFilterer{contract: contract}, nil
}

// bindPairRegistry binds a generic wrapper to an already deployed contract.
func bindPairRegistry(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := PairRegistryMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, *parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_PairRegistry *PairRegistryRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _PairRegistry.Contract.PairRegistryCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_PairRegistry *PairRegistryRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _PairRegistry.Contract.PairRegistryTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_PairRegistry *PairRegistryRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _PairRegistry.Contract.PairRegistryTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_PairRegistry *PairRegistryCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _PairRegistry.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_PairRegistry *PairRegistryTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _PairRegistry.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_PairRegistry *PairRegistryTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _PairRegistry.Contract.contract.Transact(opts, method, params...)
}

// REGISTRY is a free data retrieval call binding the contract method 0x06433b1b.
//
// Solidity: function REGISTRY() view returns(address)
func (_PairRegistry *PairRegistryCaller) REGISTRY(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _PairRegistry.contract.Call(opts, &out, "REGISTRY")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// REGISTRY is a free data retrieval call binding the contract method 0x06433b1b.
//
// Solidity: function REGISTRY() view returns(address)
func (_PairRegistry *PairRegistrySession) REGISTRY() (common.Address, error) {
	return _PairRegistry.Contract.REGISTRY(&_PairRegistry.CallOpts)
}

// REGISTRY is a free data retrieval call binding the contract method 0x06433b1b.
//
// Solidity: function REGISTRY() view returns(address)
func (_PairRegistry *PairRegistryCallerSession) REGISTRY() (common.Address, error) {
	return _PairRegistry.Contract.REGISTRY(&_PairRegistry.CallOpts)
}

// GetAllActivePairs is a free data retrieval call binding the contract method 0x9e7c150f.
//
// Solidity: function getAllActivePairs() view returns((string,address,address,address,uint8,address,address)[])
func (_PairRegistry *PairRegistryCaller) GetAllActivePairs(opts *bind.CallOpts) ([]PairRegistryPairEntry, error) {
	var out []interface{}
	err := _PairRegistry.contract.Call(opts, &out, "getAllActivePairs")

	if err != nil {
		return *new([]PairRegistryPairEntry), err
	}

	out0 := *abi.ConvertType(out[0], new([]PairRegistryPairEntry)).(*[]PairRegistryPairEntry)

	return out0, err

}

// GetAllActivePairs is a free data retrieval call binding the contract method 0x9e7c150f.
//
// Solidity: function getAllActivePairs() view returns((string,address,address,address,uint8,address,address)[])
func (_PairRegistry *PairRegistrySession) GetAllActivePairs() ([]PairRegistryPairEntry, error) {
	return _PairRegistry.Contract.GetAllActivePairs(&_PairRegistry.CallOpts)
}

// GetAllActivePairs is a free data retrieval call binding the contract method 0x9e7c150f.
//
// Solidity: function getAllActivePairs() view returns((string,address,address,address,uint8,address,address)[])
func (_PairRegistry *PairRegistryCallerSession) GetAllActivePairs() ([]PairRegistryPairEntry, error) {
	return _PairRegistry.Contract.GetAllActivePairs(&_PairRegistry.CallOpts)
}

// GetPair is a free data retrieval call binding the contract method 0x74adf539.
//
// Solidity: function getPair(string pairId) view returns((string,address,address,address,uint8,address,address))
func (_PairRegistry *PairRegistryCaller) GetPair(opts *bind.CallOpts, pairId string) (PairRegistryPairEntry, error) {
	var out []interface{}
	err := _PairRegistry.contract.Call(opts, &out, "getPair", pairId)

	if err != nil {
		return *new(PairRegistryPairEntry), err
	}

	out0 := *abi.ConvertType(out[0], new(PairRegistryPairEntry)).(*PairRegistryPairEntry)

	return out0, err

}

// GetPair is a free data retrieval call binding the contract method 0x74adf539.
//
// Solidity: function getPair(string pairId) view returns((string,address,address,address,uint8,address,address))
func (_PairRegistry *PairRegistrySession) GetPair(pairId string) (PairRegistryPairEntry, error) {
	return _PairRegistry.Contract.GetPair(&_PairRegistry.CallOpts, pairId)
}

// GetPair is a free data retrieval call binding the contract method 0x74adf539.
//
// Solidity: function getPair(string pairId) view returns((string,address,address,address,uint8,address,address))
func (_PairRegistry *PairRegistryCallerSession) GetPair(pairId string) (PairRegistryPairEntry, error) {
	return _PairRegistry.Contract.GetPair(&_PairRegistry.CallOpts, pairId)
}

// ConfirmPair is a paid mutator transaction binding the contract method 0x9d4c1a21.
//
// Solidity: function confirmPair(string pairId) returns()
func (_PairRegistry *PairRegistryTransactor) ConfirmPair(opts *bind.TransactOpts, pairId string) (*types.Transaction, error) {
	return _PairRegistry.contract.Transact(opts, "confirmPair", pairId)
}

// ConfirmPair is a paid mutator transaction binding the contract method 0x9d4c1a21.
//
// Solidity: function confirmPair(string pairId) returns()
func (_PairRegistry *PairRegistrySession) ConfirmPair(pairId string) (*types.Transaction, error) {
	return _PairRegistry.Contract.ConfirmPair(&_PairRegistry.TransactOpts, pairId)
}

// ConfirmPair is a paid mutator transaction binding the contract method 0x9d4c1a21.
//
// Solidity: function confirmPair(string pairId) returns()
func (_PairRegistry *PairRegistryTransactorSession) ConfirmPair(pairId string) (*types.Transaction, error) {
	return _PairRegistry.Contract.ConfirmPair(&_PairRegistry.TransactOpts, pairId)
}

// ProposePair is a paid mutator transaction binding the contract method 0x52e9b130.
//
// Solidity: function proposePair(string pairId, address tokenA, address tokenB, address ammAddress) returns()
func (_PairRegistry *PairRegistryTransactor) ProposePair(opts *bind.TransactOpts, pairId string, tokenA common.Address, tokenB common.Address, ammAddress common.Address) (*types.Transaction, error) {
	return _PairRegistry.contract.Transact(opts, "proposePair", pairId, tokenA, tokenB, ammAddress)
}

// ProposePair is a paid mutator transaction binding the contract method 0x52e9b130.
//
// Solidity: function proposePair(string pairId, address tokenA, address tokenB, address ammAddress) returns()
func (_PairRegistry *PairRegistrySession) ProposePair(pairId string, tokenA common.Address, tokenB common.Address, ammAddress common.Address) (*types.Transaction, error) {
	return _PairRegistry.Contract.ProposePair(&_PairRegistry.TransactOpts, pairId, tokenA, tokenB, ammAddress)
}

// ProposePair is a paid mutator transaction binding the contract method 0x52e9b130.
//
// Solidity: function proposePair(string pairId, address tokenA, address tokenB, address ammAddress) returns()
func (_PairRegistry *PairRegistryTransactorSession) ProposePair(pairId string, tokenA common.Address, tokenB common.Address, ammAddress common.Address) (*types.Transaction, error) {
	return _PairRegistry.Contract.ProposePair(&_PairRegistry.TransactOpts, pairId, tokenA, tokenB, ammAddress)
}

// PairRegistryPairProposedIterator is returned from FilterPairProposed and is used to iterate over the raw logs and unpacked data for PairProposed events raised by the PairRegistry contract.
type PairRegistryPairProposedIterator struct {
	Event *PairRegistryPairProposed // Event containing the contract specifics and raw log

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
func (it *PairRegistryPairProposedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(PairRegistryPairProposed)
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
		it.Event = new(PairRegistryPairProposed)
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
func (it *PairRegistryPairProposedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *PairRegistryPairProposedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// PairRegistryPairProposed represents a PairProposed event raised by the PairRegistry contract.
type PairRegistryPairProposed struct {
	PairId   common.Hash
	Proposer common.Address
	TokenA   common.Address
	TokenB   common.Address
	Raw      types.Log // Blockchain specific contextual infos
}

// FilterPairProposed is a free log retrieval operation binding the contract event 0x81b6c2eabb42da5f8e9bc4f181b131ca906df3ceb6ea7284dfd80cc6818d0144.
//
// Solidity: event PairProposed(string indexed pairId, address indexed proposer, address tokenA, address tokenB)
func (_PairRegistry *PairRegistryFilterer) FilterPairProposed(opts *bind.FilterOpts, pairId []string, proposer []common.Address) (*PairRegistryPairProposedIterator, error) {

	var pairIdRule []interface{}
	for _, pairIdItem := range pairId {
		pairIdRule = append(pairIdRule, pairIdItem)
	}
	var proposerRule []interface{}
	for _, proposerItem := range proposer {
		proposerRule = append(proposerRule, proposerItem)
	}

	logs, sub, err := _PairRegistry.contract.FilterLogs(opts, "PairProposed", pairIdRule, proposerRule)
	if err != nil {
		return nil, err
	}
	return &PairRegistryPairProposedIterator{contract: _PairRegistry.contract, event: "PairProposed", logs: logs, sub: sub}, nil
}

// WatchPairProposed is a free log subscription operation binding the contract event 0x81b6c2eabb42da5f8e9bc4f181b131ca906df3ceb6ea7284dfd80cc6818d0144.
//
// Solidity: event PairProposed(string indexed pairId, address indexed proposer, address tokenA, address tokenB)
func (_PairRegistry *PairRegistryFilterer) WatchPairProposed(opts *bind.WatchOpts, sink chan<- *PairRegistryPairProposed, pairId []string, proposer []common.Address) (event.Subscription, error) {

	var pairIdRule []interface{}
	for _, pairIdItem := range pairId {
		pairIdRule = append(pairIdRule, pairIdItem)
	}
	var proposerRule []interface{}
	for _, proposerItem := range proposer {
		proposerRule = append(proposerRule, proposerItem)
	}

	logs, sub, err := _PairRegistry.contract.WatchLogs(opts, "PairProposed", pairIdRule, proposerRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(PairRegistryPairProposed)
				if err := _PairRegistry.contract.UnpackLog(event, "PairProposed", log); err != nil {
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

// ParsePairProposed is a log parse operation binding the contract event 0x81b6c2eabb42da5f8e9bc4f181b131ca906df3ceb6ea7284dfd80cc6818d0144.
//
// Solidity: event PairProposed(string indexed pairId, address indexed proposer, address tokenA, address tokenB)
func (_PairRegistry *PairRegistryFilterer) ParsePairProposed(log types.Log) (*PairRegistryPairProposed, error) {
	event := new(PairRegistryPairProposed)
	if err := _PairRegistry.contract.UnpackLog(event, "PairProposed", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// PairRegistryPairRegisteredIterator is returned from FilterPairRegistered and is used to iterate over the raw logs and unpacked data for PairRegistered events raised by the PairRegistry contract.
type PairRegistryPairRegisteredIterator struct {
	Event *PairRegistryPairRegistered // Event containing the contract specifics and raw log

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
func (it *PairRegistryPairRegisteredIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(PairRegistryPairRegistered)
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
		it.Event = new(PairRegistryPairRegistered)
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
func (it *PairRegistryPairRegisteredIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *PairRegistryPairRegisteredIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// PairRegistryPairRegistered represents a PairRegistered event raised by the PairRegistry contract.
type PairRegistryPairRegistered struct {
	PairId     common.Hash
	AmmAddress common.Address
	TokenA     common.Address
	TokenB     common.Address
	Raw        types.Log // Blockchain specific contextual infos
}

// FilterPairRegistered is a free log retrieval operation binding the contract event 0x1bdd6e3c097fae9959c0e8f7088a7a41aa56ee949b35a46c3dce5e5db2aff67e.
//
// Solidity: event PairRegistered(string indexed pairId, address indexed ammAddress, address tokenA, address tokenB)
func (_PairRegistry *PairRegistryFilterer) FilterPairRegistered(opts *bind.FilterOpts, pairId []string, ammAddress []common.Address) (*PairRegistryPairRegisteredIterator, error) {

	var pairIdRule []interface{}
	for _, pairIdItem := range pairId {
		pairIdRule = append(pairIdRule, pairIdItem)
	}
	var ammAddressRule []interface{}
	for _, ammAddressItem := range ammAddress {
		ammAddressRule = append(ammAddressRule, ammAddressItem)
	}

	logs, sub, err := _PairRegistry.contract.FilterLogs(opts, "PairRegistered", pairIdRule, ammAddressRule)
	if err != nil {
		return nil, err
	}
	return &PairRegistryPairRegisteredIterator{contract: _PairRegistry.contract, event: "PairRegistered", logs: logs, sub: sub}, nil
}

// WatchPairRegistered is a free log subscription operation binding the contract event 0x1bdd6e3c097fae9959c0e8f7088a7a41aa56ee949b35a46c3dce5e5db2aff67e.
//
// Solidity: event PairRegistered(string indexed pairId, address indexed ammAddress, address tokenA, address tokenB)
func (_PairRegistry *PairRegistryFilterer) WatchPairRegistered(opts *bind.WatchOpts, sink chan<- *PairRegistryPairRegistered, pairId []string, ammAddress []common.Address) (event.Subscription, error) {

	var pairIdRule []interface{}
	for _, pairIdItem := range pairId {
		pairIdRule = append(pairIdRule, pairIdItem)
	}
	var ammAddressRule []interface{}
	for _, ammAddressItem := range ammAddress {
		ammAddressRule = append(ammAddressRule, ammAddressItem)
	}

	logs, sub, err := _PairRegistry.contract.WatchLogs(opts, "PairRegistered", pairIdRule, ammAddressRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(PairRegistryPairRegistered)
				if err := _PairRegistry.contract.UnpackLog(event, "PairRegistered", log); err != nil {
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

// ParsePairRegistered is a log parse operation binding the contract event 0x1bdd6e3c097fae9959c0e8f7088a7a41aa56ee949b35a46c3dce5e5db2aff67e.
//
// Solidity: event PairRegistered(string indexed pairId, address indexed ammAddress, address tokenA, address tokenB)
func (_PairRegistry *PairRegistryFilterer) ParsePairRegistered(log types.Log) (*PairRegistryPairRegistered, error) {
	event := new(PairRegistryPairRegistered)
	if err := _PairRegistry.contract.UnpackLog(event, "PairRegistered", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}
