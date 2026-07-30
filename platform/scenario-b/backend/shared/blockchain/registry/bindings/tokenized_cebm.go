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

// TokenizedCentralBankMoneyMetaData contains all meta data concerning the TokenizedCentralBankMoney contract.
var TokenizedCentralBankMoneyMetaData = &bind.MetaData{
	ABI: "[{\"type\":\"constructor\",\"inputs\":[{\"name\":\"name_\",\"type\":\"string\",\"internalType\":\"string\"},{\"name\":\"symbol_\",\"type\":\"string\",\"internalType\":\"string\"},{\"name\":\"admin\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"centralBank\",\"type\":\"address\",\"internalType\":\"address\"}],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"CENTRAL_BANK_ROLE\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"DEFAULT_ADMIN_ROLE\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"allowance\",\"inputs\":[{\"name\":\"owner\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"spender\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"approve\",\"inputs\":[{\"name\":\"spender\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"value\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[{\"name\":\"\",\"type\":\"bool\",\"internalType\":\"bool\"}],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"balanceOf\",\"inputs\":[{\"name\":\"account\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"burn\",\"inputs\":[{\"name\":\"from\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"amount\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"decimals\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"uint8\",\"internalType\":\"uint8\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"getRoleAdmin\",\"inputs\":[{\"name\":\"role\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"}],\"outputs\":[{\"name\":\"\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"grantRole\",\"inputs\":[{\"name\":\"role\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"},{\"name\":\"account\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"hasRole\",\"inputs\":[{\"name\":\"role\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"},{\"name\":\"account\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[{\"name\":\"\",\"type\":\"bool\",\"internalType\":\"bool\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"mint\",\"inputs\":[{\"name\":\"to\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"amount\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"name\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"string\",\"internalType\":\"string\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"renounceRole\",\"inputs\":[{\"name\":\"role\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"},{\"name\":\"callerConfirmation\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"revokeRole\",\"inputs\":[{\"name\":\"role\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"},{\"name\":\"account\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"supportsInterface\",\"inputs\":[{\"name\":\"interfaceId\",\"type\":\"bytes4\",\"internalType\":\"bytes4\"}],\"outputs\":[{\"name\":\"\",\"type\":\"bool\",\"internalType\":\"bool\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"symbol\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"string\",\"internalType\":\"string\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"totalSupply\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"transfer\",\"inputs\":[{\"name\":\"to\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"value\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[{\"name\":\"\",\"type\":\"bool\",\"internalType\":\"bool\"}],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"transferFrom\",\"inputs\":[{\"name\":\"from\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"to\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"value\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[{\"name\":\"\",\"type\":\"bool\",\"internalType\":\"bool\"}],\"stateMutability\":\"nonpayable\"},{\"type\":\"event\",\"name\":\"Approval\",\"inputs\":[{\"name\":\"owner\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"spender\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"value\",\"type\":\"uint256\",\"indexed\":false,\"internalType\":\"uint256\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"RoleAdminChanged\",\"inputs\":[{\"name\":\"role\",\"type\":\"bytes32\",\"indexed\":true,\"internalType\":\"bytes32\"},{\"name\":\"previousAdminRole\",\"type\":\"bytes32\",\"indexed\":true,\"internalType\":\"bytes32\"},{\"name\":\"newAdminRole\",\"type\":\"bytes32\",\"indexed\":true,\"internalType\":\"bytes32\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"RoleGranted\",\"inputs\":[{\"name\":\"role\",\"type\":\"bytes32\",\"indexed\":true,\"internalType\":\"bytes32\"},{\"name\":\"account\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"sender\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"RoleRevoked\",\"inputs\":[{\"name\":\"role\",\"type\":\"bytes32\",\"indexed\":true,\"internalType\":\"bytes32\"},{\"name\":\"account\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"sender\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"Transfer\",\"inputs\":[{\"name\":\"from\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"to\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"value\",\"type\":\"uint256\",\"indexed\":false,\"internalType\":\"uint256\"}],\"anonymous\":false},{\"type\":\"error\",\"name\":\"AccessControlBadConfirmation\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"AccessControlUnauthorizedAccount\",\"inputs\":[{\"name\":\"account\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"neededRole\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"}]},{\"type\":\"error\",\"name\":\"ERC20InsufficientAllowance\",\"inputs\":[{\"name\":\"spender\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"allowance\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"needed\",\"type\":\"uint256\",\"internalType\":\"uint256\"}]},{\"type\":\"error\",\"name\":\"ERC20InsufficientBalance\",\"inputs\":[{\"name\":\"sender\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"balance\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"needed\",\"type\":\"uint256\",\"internalType\":\"uint256\"}]},{\"type\":\"error\",\"name\":\"ERC20InvalidApprover\",\"inputs\":[{\"name\":\"approver\",\"type\":\"address\",\"internalType\":\"address\"}]},{\"type\":\"error\",\"name\":\"ERC20InvalidReceiver\",\"inputs\":[{\"name\":\"receiver\",\"type\":\"address\",\"internalType\":\"address\"}]},{\"type\":\"error\",\"name\":\"ERC20InvalidSender\",\"inputs\":[{\"name\":\"sender\",\"type\":\"address\",\"internalType\":\"address\"}]},{\"type\":\"error\",\"name\":\"ERC20InvalidSpender\",\"inputs\":[{\"name\":\"spender\",\"type\":\"address\",\"internalType\":\"address\"}]}]",
	Bin: "0x608060405234620003615762001156803803806200001d8162000366565b9283398101608082820312620003615781516001600160401b0392908381116200036157826200004f9183016200038c565b91602090818301519085821162000361576200006d9184016200038c565b6200008960606200008160408601620003fe565b9401620003fe565b93805186811162000261576003928354916001938484811c9416801562000356575b8785101462000340578190601f94858111620002ea575b508790858311600114620002835760009262000277575b505060001982871b1c191690841b1784555b8051978811620002615760049485548481811c9116801562000256575b828210146200024157838111620001f6575b50809289116001146200017b5750928780936200015e9993620001579897966000956200016f575b50501b92600019911b1c191617905562000413565b5062000494565b50604051610bfe9081620005388239f35b01519350388062000142565b92939190601f1989168660005284600020946000905b828210620001de575050918993916200015e9a62000157999897969410620001c3575b50505050811b01905562000413565b01519060f884600019921b161c1916905538808080620001b4565b80888698829497870151815501970194019062000191565b866000528160002084808c0160051c820192848d1062000237575b0160051c019085905b8281106200022a5750506200011a565b600081550185906200021a565b9250819262000211565b602287634e487b7160e01b6000525260246000fd5b90607f169062000108565b634e487b7160e01b600052604160045260246000fd5b015190503880620000d9565b90869350601f1983169188600052896000209260005b8b828210620002d35750508411620002ba575b505050811b018455620000eb565b015160001983891b60f8161c19169055388080620002ac565b8385015186558a9790950194938401930162000299565b90915086600052876000208580850160051c8201928a861062000336575b918891869594930160051c01915b82811062000326575050620000c2565b6000815585945088910162000316565b9250819262000308565b634e487b7160e01b600052602260045260246000fd5b93607f1693620000ab565b600080fd5b6040519190601f01601f191682016001600160401b038111838210176200026157604052565b919080601f84011215620003615782516001600160401b0381116200026157602090620003c2601f8201601f1916830162000366565b92818452828287010111620003615760005b818110620003ea57508260009394955001015290565b8581018301518482018401528201620003d4565b51906001600160a01b03821682036200036157565b6001600160a01b031660008181527f05b8ccbb9d4d8fb16ea74ce3c29a41f1b461fbdaff4714a0d9a8eb05499746bc602052604081205490919060ff16620004905781805260056020526040822081835260205260408220600160ff198254161790553391600080516020620011368339815191528180a4600190565b5090565b6001600160a01b031660008181527f90855a740dbaa3a21b673ebec4f0e79444a9b4d8cd172f871233f057e3174d3460205260408120549091907fdde1d57e4f95ec67edc5d43d42c6c08511388e9a50979dc576318f76aae844419060ff16620005325780835260056020526040832082845260205260408320600160ff1982541617905560008051602062001136833981519152339380a4600190565b50509056fe608060408181526004918236101561001657600080fd5b600092833560e01c91826301ffc9a7146108865750816306fdde03146107ac578163095ea7b31461070257816318160ddd146106e357816323b872dd146105ec578163248a9ca3146105c15781632f2ff15d14610597578163313ce5671461057b57816336568abe1461053557816340c10f191461048557816370a082311461044e57816391d148541461040757816395d89b41146102e85781639af71aa0146102ad5781639dc29fac146101d1578163a217fddf146101b6578163a9059cbb14610185578163d547741f14610141575063dd62ed3e146100f657600080fd5b3461013d578060031936011261013d5780602092610112610922565b61011a61093d565b6001600160a01b0391821683526001865283832091168252845220549051908152f35b5080fd5b9190503461018157806003193601126101815761017d9135610178600161016661093d565b938387526005602052862001546109cd565b610a73565b5080f35b8280fd5b50503461013d578060031936011261013d576020906101af6101a5610922565b6024359033610aea565b5160018152f35b50503461013d578160031936011261013d5751908152602090f35b8391503461013d578260031936011261013d576101ec610922565b90602435906101f9610953565b6001600160a01b038316928315610296578385528460205285852054918383106102625750508184957fddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef936020938688528785520381872055816002540360025551908152a380f35b865163391434e360e21b81526001600160a01b03909216908201908152602081018390526040810184905281906060010390fd5b8551634b637e8f60e11b8152808301869052602490fd5b50503461013d578160031936011261013d57602090517fdde1d57e4f95ec67edc5d43d42c6c08511388e9a50979dc576318f76aae844418152f35b83833461013d578160031936011261013d57805190828454600181811c908083169283156103fd575b60209384841081146103ea578388529081156103ce5750600114610379575b505050829003601f01601f191682019267ffffffffffffffff84118385101761036657508291826103629252826108d9565b0390f35b634e487b7160e01b815260418552602490fd5b8787529192508591837f8a35acfbc15ff81a39ae7d344fd709f28e8600b4aa8c65c6b64bfe7fe36bd19b5b8385106103ba5750505050830101858080610330565b8054888601830152930192849082016103a4565b60ff1916878501525050151560051b8401019050858080610330565b634e487b7160e01b895260228a52602489fd5b91607f1691610311565b9050346101815781600319360112610181578160209360ff9261042861093d565b90358252600586528282206001600160a01b039091168252855220549151911615158152f35b50503461013d57602036600319011261013d5760209181906001600160a01b03610476610922565b16815280845220549051908152f35b919050346101815780600319360112610181576104a0610922565b90602435916104ad610953565b6001600160a01b0316928315610520576002549083820180921161050d575084927fddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef9260209260025585855284835280852082815401905551908152a380f35b634e487b7160e01b865260119052602485fd5b84602492519163ec442f0560e01b8352820152fd5b83833461013d578060031936011261013d5761054f61093d565b90336001600160a01b0383160361056c575061017d919235610a73565b5163334bd91960e11b81528390fd5b50503461013d578160031936011261013d576020905160128152f35b9190503461018157806003193601126101815761017d91356105bc600161016661093d565b6109f3565b9050346101815760203660031901126101815781602093600192358152600585522001549051908152f35b905082346106e05760603660031901126106e057610608610922565b61061061093d565b916044359360018060a01b03831680835260016020528683203384526020528683205491600019830361064c575b6020886101af898989610aea565b8683106106b457811561069d573315610686575082526001602090815286832033845281529186902090859003905582906101af8761063e565b8751634a1406b160e11b8152908101849052602490fd5b875163e602df0560e01b8152908101849052602490fd5b8751637dc7a0d960e11b8152339181019182526020820193909352604081018790528291506060010390fd5b80fd5b50503461013d578160031936011261013d576020906002549051908152f35b90503461018157816003193601126101815761071c610922565b602435903315610795576001600160a01b031691821561077e57508083602095338152600187528181208582528752205582519081527f8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b925843392a35160018152f35b8351634a1406b160e11b8152908101859052602490fd5b835163e602df0560e01b8152808401869052602490fd5b83833461013d578160031936011261013d5780519082600354600181811c9080831692831561087c575b60209384841081146103ea578388529081156103ce575060011461082657505050829003601f01601f191682019267ffffffffffffffff84118385101761036657508291826103629252826108d9565b600387529192508591837fc2575a0e9e593c00f959f8c92f12db2869c3395a3b0502d05e2516446f71f85b5b8385106108685750505050830101858080610330565b805488860183015293019284908201610852565b91607f16916107d6565b849134610181576020366003190112610181573563ffffffff60e01b81168091036101815760209250637965db0b60e01b81149081156108c8575b5015158152f35b6301ffc9a760e01b149050836108c1565b6020808252825181830181905290939260005b82811061090e57505060409293506000838284010152601f8019910116010190565b8181018601518482016040015285016108ec565b600435906001600160a01b038216820361093857565b600080fd5b602435906001600160a01b038216820361093857565b3360009081527f90855a740dbaa3a21b673ebec4f0e79444a9b4d8cd172f871233f057e3174d3460205260409020547fdde1d57e4f95ec67edc5d43d42c6c08511388e9a50979dc576318f76aae844419060ff16156109af5750565b6044906040519063e2517d3f60e01b82523360048301526024820152fd5b80600052600560205260406000203360005260205260ff60406000205416156109af5750565b906000918083526005602052604083209160018060a01b03169182845260205260ff60408420541615600014610a6e5780835260056020526040832082845260205260408320600160ff198254161790557f2f8788117e7eff1d82e926ec794901d17c78024a50270940304540a733656f0d339380a4600190565b505090565b906000918083526005602052604083209160018060a01b03169182845260205260ff604084205416600014610a6e578083526005602052604083208284526020526040832060ff1981541690557ff6391f5c32d9c69d2a47ea670b442974b53935d1edc7fd64eb21e047a839171b339380a4600190565b916001600160a01b03808416928315610baf5716928315610b965760009083825281602052604082205490838210610b64575091604082827fddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef958760209652828652038282205586815220818154019055604051908152a3565b60405163391434e360e21b81526001600160a01b03919091166004820152602481019190915260448101839052606490fd5b60405163ec442f0560e01b815260006004820152602490fd5b604051634b637e8f60e11b815260006004820152602490fdfea264697066735822122054c50dd7adfbd4067f3d229b92c04116d08c27add527e69a799cd90bd8281d1d64736f6c634300081400332f8788117e7eff1d82e926ec794901d17c78024a50270940304540a733656f0d",
}

// TokenizedCentralBankMoneyABI is the input ABI used to generate the binding from.
// Deprecated: Use TokenizedCentralBankMoneyMetaData.ABI instead.
var TokenizedCentralBankMoneyABI = TokenizedCentralBankMoneyMetaData.ABI

// TokenizedCentralBankMoneyBin is the compiled bytecode used for deploying new contracts.
// Deprecated: Use TokenizedCentralBankMoneyMetaData.Bin instead.
var TokenizedCentralBankMoneyBin = TokenizedCentralBankMoneyMetaData.Bin

// DeployTokenizedCentralBankMoney deploys a new Ethereum contract, binding an instance of TokenizedCentralBankMoney to it.
func DeployTokenizedCentralBankMoney(auth *bind.TransactOpts, backend bind.ContractBackend, name_ string, symbol_ string, admin common.Address, centralBank common.Address) (common.Address, *types.Transaction, *TokenizedCentralBankMoney, error) {
	parsed, err := TokenizedCentralBankMoneyMetaData.GetAbi()
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	if parsed == nil {
		return common.Address{}, nil, nil, errors.New("GetABI returned nil")
	}

	address, tx, contract, err := bind.DeployContract(auth, *parsed, common.FromHex(TokenizedCentralBankMoneyBin), backend, name_, symbol_, admin, centralBank)
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	return address, tx, &TokenizedCentralBankMoney{TokenizedCentralBankMoneyCaller: TokenizedCentralBankMoneyCaller{contract: contract}, TokenizedCentralBankMoneyTransactor: TokenizedCentralBankMoneyTransactor{contract: contract}, TokenizedCentralBankMoneyFilterer: TokenizedCentralBankMoneyFilterer{contract: contract}}, nil
}

// TokenizedCentralBankMoney is an auto generated Go binding around an Ethereum contract.
type TokenizedCentralBankMoney struct {
	TokenizedCentralBankMoneyCaller     // Read-only binding to the contract
	TokenizedCentralBankMoneyTransactor // Write-only binding to the contract
	TokenizedCentralBankMoneyFilterer   // Log filterer for contract events
}

// TokenizedCentralBankMoneyCaller is an auto generated read-only Go binding around an Ethereum contract.
type TokenizedCentralBankMoneyCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// TokenizedCentralBankMoneyTransactor is an auto generated write-only Go binding around an Ethereum contract.
type TokenizedCentralBankMoneyTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// TokenizedCentralBankMoneyFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type TokenizedCentralBankMoneyFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// TokenizedCentralBankMoneySession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type TokenizedCentralBankMoneySession struct {
	Contract     *TokenizedCentralBankMoney // Generic contract binding to set the session for
	CallOpts     bind.CallOpts              // Call options to use throughout this session
	TransactOpts bind.TransactOpts          // Transaction auth options to use throughout this session
}

// TokenizedCentralBankMoneyCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type TokenizedCentralBankMoneyCallerSession struct {
	Contract *TokenizedCentralBankMoneyCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts                    // Call options to use throughout this session
}

// TokenizedCentralBankMoneyTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type TokenizedCentralBankMoneyTransactorSession struct {
	Contract     *TokenizedCentralBankMoneyTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts                    // Transaction auth options to use throughout this session
}

// TokenizedCentralBankMoneyRaw is an auto generated low-level Go binding around an Ethereum contract.
type TokenizedCentralBankMoneyRaw struct {
	Contract *TokenizedCentralBankMoney // Generic contract binding to access the raw methods on
}

// TokenizedCentralBankMoneyCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type TokenizedCentralBankMoneyCallerRaw struct {
	Contract *TokenizedCentralBankMoneyCaller // Generic read-only contract binding to access the raw methods on
}

// TokenizedCentralBankMoneyTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type TokenizedCentralBankMoneyTransactorRaw struct {
	Contract *TokenizedCentralBankMoneyTransactor // Generic write-only contract binding to access the raw methods on
}

// NewTokenizedCentralBankMoney creates a new instance of TokenizedCentralBankMoney, bound to a specific deployed contract.
func NewTokenizedCentralBankMoney(address common.Address, backend bind.ContractBackend) (*TokenizedCentralBankMoney, error) {
	contract, err := bindTokenizedCentralBankMoney(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &TokenizedCentralBankMoney{TokenizedCentralBankMoneyCaller: TokenizedCentralBankMoneyCaller{contract: contract}, TokenizedCentralBankMoneyTransactor: TokenizedCentralBankMoneyTransactor{contract: contract}, TokenizedCentralBankMoneyFilterer: TokenizedCentralBankMoneyFilterer{contract: contract}}, nil
}

// NewTokenizedCentralBankMoneyCaller creates a new read-only instance of TokenizedCentralBankMoney, bound to a specific deployed contract.
func NewTokenizedCentralBankMoneyCaller(address common.Address, caller bind.ContractCaller) (*TokenizedCentralBankMoneyCaller, error) {
	contract, err := bindTokenizedCentralBankMoney(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &TokenizedCentralBankMoneyCaller{contract: contract}, nil
}

// NewTokenizedCentralBankMoneyTransactor creates a new write-only instance of TokenizedCentralBankMoney, bound to a specific deployed contract.
func NewTokenizedCentralBankMoneyTransactor(address common.Address, transactor bind.ContractTransactor) (*TokenizedCentralBankMoneyTransactor, error) {
	contract, err := bindTokenizedCentralBankMoney(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &TokenizedCentralBankMoneyTransactor{contract: contract}, nil
}

// NewTokenizedCentralBankMoneyFilterer creates a new log filterer instance of TokenizedCentralBankMoney, bound to a specific deployed contract.
func NewTokenizedCentralBankMoneyFilterer(address common.Address, filterer bind.ContractFilterer) (*TokenizedCentralBankMoneyFilterer, error) {
	contract, err := bindTokenizedCentralBankMoney(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &TokenizedCentralBankMoneyFilterer{contract: contract}, nil
}

// bindTokenizedCentralBankMoney binds a generic wrapper to an already deployed contract.
func bindTokenizedCentralBankMoney(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := TokenizedCentralBankMoneyMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, *parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_TokenizedCentralBankMoney *TokenizedCentralBankMoneyRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _TokenizedCentralBankMoney.Contract.TokenizedCentralBankMoneyCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_TokenizedCentralBankMoney *TokenizedCentralBankMoneyRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _TokenizedCentralBankMoney.Contract.TokenizedCentralBankMoneyTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_TokenizedCentralBankMoney *TokenizedCentralBankMoneyRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _TokenizedCentralBankMoney.Contract.TokenizedCentralBankMoneyTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_TokenizedCentralBankMoney *TokenizedCentralBankMoneyCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _TokenizedCentralBankMoney.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_TokenizedCentralBankMoney *TokenizedCentralBankMoneyTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _TokenizedCentralBankMoney.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_TokenizedCentralBankMoney *TokenizedCentralBankMoneyTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _TokenizedCentralBankMoney.Contract.contract.Transact(opts, method, params...)
}

// CENTRALBANKROLE is a free data retrieval call binding the contract method 0x9af71aa0.
//
// Solidity: function CENTRAL_BANK_ROLE() view returns(bytes32)
func (_TokenizedCentralBankMoney *TokenizedCentralBankMoneyCaller) CENTRALBANKROLE(opts *bind.CallOpts) ([32]byte, error) {
	var out []interface{}
	err := _TokenizedCentralBankMoney.contract.Call(opts, &out, "CENTRAL_BANK_ROLE")

	if err != nil {
		return *new([32]byte), err
	}

	out0 := *abi.ConvertType(out[0], new([32]byte)).(*[32]byte)

	return out0, err

}

// CENTRALBANKROLE is a free data retrieval call binding the contract method 0x9af71aa0.
//
// Solidity: function CENTRAL_BANK_ROLE() view returns(bytes32)
func (_TokenizedCentralBankMoney *TokenizedCentralBankMoneySession) CENTRALBANKROLE() ([32]byte, error) {
	return _TokenizedCentralBankMoney.Contract.CENTRALBANKROLE(&_TokenizedCentralBankMoney.CallOpts)
}

// CENTRALBANKROLE is a free data retrieval call binding the contract method 0x9af71aa0.
//
// Solidity: function CENTRAL_BANK_ROLE() view returns(bytes32)
func (_TokenizedCentralBankMoney *TokenizedCentralBankMoneyCallerSession) CENTRALBANKROLE() ([32]byte, error) {
	return _TokenizedCentralBankMoney.Contract.CENTRALBANKROLE(&_TokenizedCentralBankMoney.CallOpts)
}

// DEFAULTADMINROLE is a free data retrieval call binding the contract method 0xa217fddf.
//
// Solidity: function DEFAULT_ADMIN_ROLE() view returns(bytes32)
func (_TokenizedCentralBankMoney *TokenizedCentralBankMoneyCaller) DEFAULTADMINROLE(opts *bind.CallOpts) ([32]byte, error) {
	var out []interface{}
	err := _TokenizedCentralBankMoney.contract.Call(opts, &out, "DEFAULT_ADMIN_ROLE")

	if err != nil {
		return *new([32]byte), err
	}

	out0 := *abi.ConvertType(out[0], new([32]byte)).(*[32]byte)

	return out0, err

}

// DEFAULTADMINROLE is a free data retrieval call binding the contract method 0xa217fddf.
//
// Solidity: function DEFAULT_ADMIN_ROLE() view returns(bytes32)
func (_TokenizedCentralBankMoney *TokenizedCentralBankMoneySession) DEFAULTADMINROLE() ([32]byte, error) {
	return _TokenizedCentralBankMoney.Contract.DEFAULTADMINROLE(&_TokenizedCentralBankMoney.CallOpts)
}

// DEFAULTADMINROLE is a free data retrieval call binding the contract method 0xa217fddf.
//
// Solidity: function DEFAULT_ADMIN_ROLE() view returns(bytes32)
func (_TokenizedCentralBankMoney *TokenizedCentralBankMoneyCallerSession) DEFAULTADMINROLE() ([32]byte, error) {
	return _TokenizedCentralBankMoney.Contract.DEFAULTADMINROLE(&_TokenizedCentralBankMoney.CallOpts)
}

// Allowance is a free data retrieval call binding the contract method 0xdd62ed3e.
//
// Solidity: function allowance(address owner, address spender) view returns(uint256)
func (_TokenizedCentralBankMoney *TokenizedCentralBankMoneyCaller) Allowance(opts *bind.CallOpts, owner common.Address, spender common.Address) (*big.Int, error) {
	var out []interface{}
	err := _TokenizedCentralBankMoney.contract.Call(opts, &out, "allowance", owner, spender)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// Allowance is a free data retrieval call binding the contract method 0xdd62ed3e.
//
// Solidity: function allowance(address owner, address spender) view returns(uint256)
func (_TokenizedCentralBankMoney *TokenizedCentralBankMoneySession) Allowance(owner common.Address, spender common.Address) (*big.Int, error) {
	return _TokenizedCentralBankMoney.Contract.Allowance(&_TokenizedCentralBankMoney.CallOpts, owner, spender)
}

// Allowance is a free data retrieval call binding the contract method 0xdd62ed3e.
//
// Solidity: function allowance(address owner, address spender) view returns(uint256)
func (_TokenizedCentralBankMoney *TokenizedCentralBankMoneyCallerSession) Allowance(owner common.Address, spender common.Address) (*big.Int, error) {
	return _TokenizedCentralBankMoney.Contract.Allowance(&_TokenizedCentralBankMoney.CallOpts, owner, spender)
}

// BalanceOf is a free data retrieval call binding the contract method 0x70a08231.
//
// Solidity: function balanceOf(address account) view returns(uint256)
func (_TokenizedCentralBankMoney *TokenizedCentralBankMoneyCaller) BalanceOf(opts *bind.CallOpts, account common.Address) (*big.Int, error) {
	var out []interface{}
	err := _TokenizedCentralBankMoney.contract.Call(opts, &out, "balanceOf", account)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// BalanceOf is a free data retrieval call binding the contract method 0x70a08231.
//
// Solidity: function balanceOf(address account) view returns(uint256)
func (_TokenizedCentralBankMoney *TokenizedCentralBankMoneySession) BalanceOf(account common.Address) (*big.Int, error) {
	return _TokenizedCentralBankMoney.Contract.BalanceOf(&_TokenizedCentralBankMoney.CallOpts, account)
}

// BalanceOf is a free data retrieval call binding the contract method 0x70a08231.
//
// Solidity: function balanceOf(address account) view returns(uint256)
func (_TokenizedCentralBankMoney *TokenizedCentralBankMoneyCallerSession) BalanceOf(account common.Address) (*big.Int, error) {
	return _TokenizedCentralBankMoney.Contract.BalanceOf(&_TokenizedCentralBankMoney.CallOpts, account)
}

// Decimals is a free data retrieval call binding the contract method 0x313ce567.
//
// Solidity: function decimals() view returns(uint8)
func (_TokenizedCentralBankMoney *TokenizedCentralBankMoneyCaller) Decimals(opts *bind.CallOpts) (uint8, error) {
	var out []interface{}
	err := _TokenizedCentralBankMoney.contract.Call(opts, &out, "decimals")

	if err != nil {
		return *new(uint8), err
	}

	out0 := *abi.ConvertType(out[0], new(uint8)).(*uint8)

	return out0, err

}

// Decimals is a free data retrieval call binding the contract method 0x313ce567.
//
// Solidity: function decimals() view returns(uint8)
func (_TokenizedCentralBankMoney *TokenizedCentralBankMoneySession) Decimals() (uint8, error) {
	return _TokenizedCentralBankMoney.Contract.Decimals(&_TokenizedCentralBankMoney.CallOpts)
}

// Decimals is a free data retrieval call binding the contract method 0x313ce567.
//
// Solidity: function decimals() view returns(uint8)
func (_TokenizedCentralBankMoney *TokenizedCentralBankMoneyCallerSession) Decimals() (uint8, error) {
	return _TokenizedCentralBankMoney.Contract.Decimals(&_TokenizedCentralBankMoney.CallOpts)
}

// GetRoleAdmin is a free data retrieval call binding the contract method 0x248a9ca3.
//
// Solidity: function getRoleAdmin(bytes32 role) view returns(bytes32)
func (_TokenizedCentralBankMoney *TokenizedCentralBankMoneyCaller) GetRoleAdmin(opts *bind.CallOpts, role [32]byte) ([32]byte, error) {
	var out []interface{}
	err := _TokenizedCentralBankMoney.contract.Call(opts, &out, "getRoleAdmin", role)

	if err != nil {
		return *new([32]byte), err
	}

	out0 := *abi.ConvertType(out[0], new([32]byte)).(*[32]byte)

	return out0, err

}

// GetRoleAdmin is a free data retrieval call binding the contract method 0x248a9ca3.
//
// Solidity: function getRoleAdmin(bytes32 role) view returns(bytes32)
func (_TokenizedCentralBankMoney *TokenizedCentralBankMoneySession) GetRoleAdmin(role [32]byte) ([32]byte, error) {
	return _TokenizedCentralBankMoney.Contract.GetRoleAdmin(&_TokenizedCentralBankMoney.CallOpts, role)
}

// GetRoleAdmin is a free data retrieval call binding the contract method 0x248a9ca3.
//
// Solidity: function getRoleAdmin(bytes32 role) view returns(bytes32)
func (_TokenizedCentralBankMoney *TokenizedCentralBankMoneyCallerSession) GetRoleAdmin(role [32]byte) ([32]byte, error) {
	return _TokenizedCentralBankMoney.Contract.GetRoleAdmin(&_TokenizedCentralBankMoney.CallOpts, role)
}

// HasRole is a free data retrieval call binding the contract method 0x91d14854.
//
// Solidity: function hasRole(bytes32 role, address account) view returns(bool)
func (_TokenizedCentralBankMoney *TokenizedCentralBankMoneyCaller) HasRole(opts *bind.CallOpts, role [32]byte, account common.Address) (bool, error) {
	var out []interface{}
	err := _TokenizedCentralBankMoney.contract.Call(opts, &out, "hasRole", role, account)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// HasRole is a free data retrieval call binding the contract method 0x91d14854.
//
// Solidity: function hasRole(bytes32 role, address account) view returns(bool)
func (_TokenizedCentralBankMoney *TokenizedCentralBankMoneySession) HasRole(role [32]byte, account common.Address) (bool, error) {
	return _TokenizedCentralBankMoney.Contract.HasRole(&_TokenizedCentralBankMoney.CallOpts, role, account)
}

// HasRole is a free data retrieval call binding the contract method 0x91d14854.
//
// Solidity: function hasRole(bytes32 role, address account) view returns(bool)
func (_TokenizedCentralBankMoney *TokenizedCentralBankMoneyCallerSession) HasRole(role [32]byte, account common.Address) (bool, error) {
	return _TokenizedCentralBankMoney.Contract.HasRole(&_TokenizedCentralBankMoney.CallOpts, role, account)
}

// Name is a free data retrieval call binding the contract method 0x06fdde03.
//
// Solidity: function name() view returns(string)
func (_TokenizedCentralBankMoney *TokenizedCentralBankMoneyCaller) Name(opts *bind.CallOpts) (string, error) {
	var out []interface{}
	err := _TokenizedCentralBankMoney.contract.Call(opts, &out, "name")

	if err != nil {
		return *new(string), err
	}

	out0 := *abi.ConvertType(out[0], new(string)).(*string)

	return out0, err

}

// Name is a free data retrieval call binding the contract method 0x06fdde03.
//
// Solidity: function name() view returns(string)
func (_TokenizedCentralBankMoney *TokenizedCentralBankMoneySession) Name() (string, error) {
	return _TokenizedCentralBankMoney.Contract.Name(&_TokenizedCentralBankMoney.CallOpts)
}

// Name is a free data retrieval call binding the contract method 0x06fdde03.
//
// Solidity: function name() view returns(string)
func (_TokenizedCentralBankMoney *TokenizedCentralBankMoneyCallerSession) Name() (string, error) {
	return _TokenizedCentralBankMoney.Contract.Name(&_TokenizedCentralBankMoney.CallOpts)
}

// SupportsInterface is a free data retrieval call binding the contract method 0x01ffc9a7.
//
// Solidity: function supportsInterface(bytes4 interfaceId) view returns(bool)
func (_TokenizedCentralBankMoney *TokenizedCentralBankMoneyCaller) SupportsInterface(opts *bind.CallOpts, interfaceId [4]byte) (bool, error) {
	var out []interface{}
	err := _TokenizedCentralBankMoney.contract.Call(opts, &out, "supportsInterface", interfaceId)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// SupportsInterface is a free data retrieval call binding the contract method 0x01ffc9a7.
//
// Solidity: function supportsInterface(bytes4 interfaceId) view returns(bool)
func (_TokenizedCentralBankMoney *TokenizedCentralBankMoneySession) SupportsInterface(interfaceId [4]byte) (bool, error) {
	return _TokenizedCentralBankMoney.Contract.SupportsInterface(&_TokenizedCentralBankMoney.CallOpts, interfaceId)
}

// SupportsInterface is a free data retrieval call binding the contract method 0x01ffc9a7.
//
// Solidity: function supportsInterface(bytes4 interfaceId) view returns(bool)
func (_TokenizedCentralBankMoney *TokenizedCentralBankMoneyCallerSession) SupportsInterface(interfaceId [4]byte) (bool, error) {
	return _TokenizedCentralBankMoney.Contract.SupportsInterface(&_TokenizedCentralBankMoney.CallOpts, interfaceId)
}

// Symbol is a free data retrieval call binding the contract method 0x95d89b41.
//
// Solidity: function symbol() view returns(string)
func (_TokenizedCentralBankMoney *TokenizedCentralBankMoneyCaller) Symbol(opts *bind.CallOpts) (string, error) {
	var out []interface{}
	err := _TokenizedCentralBankMoney.contract.Call(opts, &out, "symbol")

	if err != nil {
		return *new(string), err
	}

	out0 := *abi.ConvertType(out[0], new(string)).(*string)

	return out0, err

}

// Symbol is a free data retrieval call binding the contract method 0x95d89b41.
//
// Solidity: function symbol() view returns(string)
func (_TokenizedCentralBankMoney *TokenizedCentralBankMoneySession) Symbol() (string, error) {
	return _TokenizedCentralBankMoney.Contract.Symbol(&_TokenizedCentralBankMoney.CallOpts)
}

// Symbol is a free data retrieval call binding the contract method 0x95d89b41.
//
// Solidity: function symbol() view returns(string)
func (_TokenizedCentralBankMoney *TokenizedCentralBankMoneyCallerSession) Symbol() (string, error) {
	return _TokenizedCentralBankMoney.Contract.Symbol(&_TokenizedCentralBankMoney.CallOpts)
}

// TotalSupply is a free data retrieval call binding the contract method 0x18160ddd.
//
// Solidity: function totalSupply() view returns(uint256)
func (_TokenizedCentralBankMoney *TokenizedCentralBankMoneyCaller) TotalSupply(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _TokenizedCentralBankMoney.contract.Call(opts, &out, "totalSupply")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// TotalSupply is a free data retrieval call binding the contract method 0x18160ddd.
//
// Solidity: function totalSupply() view returns(uint256)
func (_TokenizedCentralBankMoney *TokenizedCentralBankMoneySession) TotalSupply() (*big.Int, error) {
	return _TokenizedCentralBankMoney.Contract.TotalSupply(&_TokenizedCentralBankMoney.CallOpts)
}

// TotalSupply is a free data retrieval call binding the contract method 0x18160ddd.
//
// Solidity: function totalSupply() view returns(uint256)
func (_TokenizedCentralBankMoney *TokenizedCentralBankMoneyCallerSession) TotalSupply() (*big.Int, error) {
	return _TokenizedCentralBankMoney.Contract.TotalSupply(&_TokenizedCentralBankMoney.CallOpts)
}

// Approve is a paid mutator transaction binding the contract method 0x095ea7b3.
//
// Solidity: function approve(address spender, uint256 value) returns(bool)
func (_TokenizedCentralBankMoney *TokenizedCentralBankMoneyTransactor) Approve(opts *bind.TransactOpts, spender common.Address, value *big.Int) (*types.Transaction, error) {
	return _TokenizedCentralBankMoney.contract.Transact(opts, "approve", spender, value)
}

// Approve is a paid mutator transaction binding the contract method 0x095ea7b3.
//
// Solidity: function approve(address spender, uint256 value) returns(bool)
func (_TokenizedCentralBankMoney *TokenizedCentralBankMoneySession) Approve(spender common.Address, value *big.Int) (*types.Transaction, error) {
	return _TokenizedCentralBankMoney.Contract.Approve(&_TokenizedCentralBankMoney.TransactOpts, spender, value)
}

// Approve is a paid mutator transaction binding the contract method 0x095ea7b3.
//
// Solidity: function approve(address spender, uint256 value) returns(bool)
func (_TokenizedCentralBankMoney *TokenizedCentralBankMoneyTransactorSession) Approve(spender common.Address, value *big.Int) (*types.Transaction, error) {
	return _TokenizedCentralBankMoney.Contract.Approve(&_TokenizedCentralBankMoney.TransactOpts, spender, value)
}

// Burn is a paid mutator transaction binding the contract method 0x9dc29fac.
//
// Solidity: function burn(address from, uint256 amount) returns()
func (_TokenizedCentralBankMoney *TokenizedCentralBankMoneyTransactor) Burn(opts *bind.TransactOpts, from common.Address, amount *big.Int) (*types.Transaction, error) {
	return _TokenizedCentralBankMoney.contract.Transact(opts, "burn", from, amount)
}

// Burn is a paid mutator transaction binding the contract method 0x9dc29fac.
//
// Solidity: function burn(address from, uint256 amount) returns()
func (_TokenizedCentralBankMoney *TokenizedCentralBankMoneySession) Burn(from common.Address, amount *big.Int) (*types.Transaction, error) {
	return _TokenizedCentralBankMoney.Contract.Burn(&_TokenizedCentralBankMoney.TransactOpts, from, amount)
}

// Burn is a paid mutator transaction binding the contract method 0x9dc29fac.
//
// Solidity: function burn(address from, uint256 amount) returns()
func (_TokenizedCentralBankMoney *TokenizedCentralBankMoneyTransactorSession) Burn(from common.Address, amount *big.Int) (*types.Transaction, error) {
	return _TokenizedCentralBankMoney.Contract.Burn(&_TokenizedCentralBankMoney.TransactOpts, from, amount)
}

// GrantRole is a paid mutator transaction binding the contract method 0x2f2ff15d.
//
// Solidity: function grantRole(bytes32 role, address account) returns()
func (_TokenizedCentralBankMoney *TokenizedCentralBankMoneyTransactor) GrantRole(opts *bind.TransactOpts, role [32]byte, account common.Address) (*types.Transaction, error) {
	return _TokenizedCentralBankMoney.contract.Transact(opts, "grantRole", role, account)
}

// GrantRole is a paid mutator transaction binding the contract method 0x2f2ff15d.
//
// Solidity: function grantRole(bytes32 role, address account) returns()
func (_TokenizedCentralBankMoney *TokenizedCentralBankMoneySession) GrantRole(role [32]byte, account common.Address) (*types.Transaction, error) {
	return _TokenizedCentralBankMoney.Contract.GrantRole(&_TokenizedCentralBankMoney.TransactOpts, role, account)
}

// GrantRole is a paid mutator transaction binding the contract method 0x2f2ff15d.
//
// Solidity: function grantRole(bytes32 role, address account) returns()
func (_TokenizedCentralBankMoney *TokenizedCentralBankMoneyTransactorSession) GrantRole(role [32]byte, account common.Address) (*types.Transaction, error) {
	return _TokenizedCentralBankMoney.Contract.GrantRole(&_TokenizedCentralBankMoney.TransactOpts, role, account)
}

// Mint is a paid mutator transaction binding the contract method 0x40c10f19.
//
// Solidity: function mint(address to, uint256 amount) returns()
func (_TokenizedCentralBankMoney *TokenizedCentralBankMoneyTransactor) Mint(opts *bind.TransactOpts, to common.Address, amount *big.Int) (*types.Transaction, error) {
	return _TokenizedCentralBankMoney.contract.Transact(opts, "mint", to, amount)
}

// Mint is a paid mutator transaction binding the contract method 0x40c10f19.
//
// Solidity: function mint(address to, uint256 amount) returns()
func (_TokenizedCentralBankMoney *TokenizedCentralBankMoneySession) Mint(to common.Address, amount *big.Int) (*types.Transaction, error) {
	return _TokenizedCentralBankMoney.Contract.Mint(&_TokenizedCentralBankMoney.TransactOpts, to, amount)
}

// Mint is a paid mutator transaction binding the contract method 0x40c10f19.
//
// Solidity: function mint(address to, uint256 amount) returns()
func (_TokenizedCentralBankMoney *TokenizedCentralBankMoneyTransactorSession) Mint(to common.Address, amount *big.Int) (*types.Transaction, error) {
	return _TokenizedCentralBankMoney.Contract.Mint(&_TokenizedCentralBankMoney.TransactOpts, to, amount)
}

// RenounceRole is a paid mutator transaction binding the contract method 0x36568abe.
//
// Solidity: function renounceRole(bytes32 role, address callerConfirmation) returns()
func (_TokenizedCentralBankMoney *TokenizedCentralBankMoneyTransactor) RenounceRole(opts *bind.TransactOpts, role [32]byte, callerConfirmation common.Address) (*types.Transaction, error) {
	return _TokenizedCentralBankMoney.contract.Transact(opts, "renounceRole", role, callerConfirmation)
}

// RenounceRole is a paid mutator transaction binding the contract method 0x36568abe.
//
// Solidity: function renounceRole(bytes32 role, address callerConfirmation) returns()
func (_TokenizedCentralBankMoney *TokenizedCentralBankMoneySession) RenounceRole(role [32]byte, callerConfirmation common.Address) (*types.Transaction, error) {
	return _TokenizedCentralBankMoney.Contract.RenounceRole(&_TokenizedCentralBankMoney.TransactOpts, role, callerConfirmation)
}

// RenounceRole is a paid mutator transaction binding the contract method 0x36568abe.
//
// Solidity: function renounceRole(bytes32 role, address callerConfirmation) returns()
func (_TokenizedCentralBankMoney *TokenizedCentralBankMoneyTransactorSession) RenounceRole(role [32]byte, callerConfirmation common.Address) (*types.Transaction, error) {
	return _TokenizedCentralBankMoney.Contract.RenounceRole(&_TokenizedCentralBankMoney.TransactOpts, role, callerConfirmation)
}

// RevokeRole is a paid mutator transaction binding the contract method 0xd547741f.
//
// Solidity: function revokeRole(bytes32 role, address account) returns()
func (_TokenizedCentralBankMoney *TokenizedCentralBankMoneyTransactor) RevokeRole(opts *bind.TransactOpts, role [32]byte, account common.Address) (*types.Transaction, error) {
	return _TokenizedCentralBankMoney.contract.Transact(opts, "revokeRole", role, account)
}

// RevokeRole is a paid mutator transaction binding the contract method 0xd547741f.
//
// Solidity: function revokeRole(bytes32 role, address account) returns()
func (_TokenizedCentralBankMoney *TokenizedCentralBankMoneySession) RevokeRole(role [32]byte, account common.Address) (*types.Transaction, error) {
	return _TokenizedCentralBankMoney.Contract.RevokeRole(&_TokenizedCentralBankMoney.TransactOpts, role, account)
}

// RevokeRole is a paid mutator transaction binding the contract method 0xd547741f.
//
// Solidity: function revokeRole(bytes32 role, address account) returns()
func (_TokenizedCentralBankMoney *TokenizedCentralBankMoneyTransactorSession) RevokeRole(role [32]byte, account common.Address) (*types.Transaction, error) {
	return _TokenizedCentralBankMoney.Contract.RevokeRole(&_TokenizedCentralBankMoney.TransactOpts, role, account)
}

// Transfer is a paid mutator transaction binding the contract method 0xa9059cbb.
//
// Solidity: function transfer(address to, uint256 value) returns(bool)
func (_TokenizedCentralBankMoney *TokenizedCentralBankMoneyTransactor) Transfer(opts *bind.TransactOpts, to common.Address, value *big.Int) (*types.Transaction, error) {
	return _TokenizedCentralBankMoney.contract.Transact(opts, "transfer", to, value)
}

// Transfer is a paid mutator transaction binding the contract method 0xa9059cbb.
//
// Solidity: function transfer(address to, uint256 value) returns(bool)
func (_TokenizedCentralBankMoney *TokenizedCentralBankMoneySession) Transfer(to common.Address, value *big.Int) (*types.Transaction, error) {
	return _TokenizedCentralBankMoney.Contract.Transfer(&_TokenizedCentralBankMoney.TransactOpts, to, value)
}

// Transfer is a paid mutator transaction binding the contract method 0xa9059cbb.
//
// Solidity: function transfer(address to, uint256 value) returns(bool)
func (_TokenizedCentralBankMoney *TokenizedCentralBankMoneyTransactorSession) Transfer(to common.Address, value *big.Int) (*types.Transaction, error) {
	return _TokenizedCentralBankMoney.Contract.Transfer(&_TokenizedCentralBankMoney.TransactOpts, to, value)
}

// TransferFrom is a paid mutator transaction binding the contract method 0x23b872dd.
//
// Solidity: function transferFrom(address from, address to, uint256 value) returns(bool)
func (_TokenizedCentralBankMoney *TokenizedCentralBankMoneyTransactor) TransferFrom(opts *bind.TransactOpts, from common.Address, to common.Address, value *big.Int) (*types.Transaction, error) {
	return _TokenizedCentralBankMoney.contract.Transact(opts, "transferFrom", from, to, value)
}

// TransferFrom is a paid mutator transaction binding the contract method 0x23b872dd.
//
// Solidity: function transferFrom(address from, address to, uint256 value) returns(bool)
func (_TokenizedCentralBankMoney *TokenizedCentralBankMoneySession) TransferFrom(from common.Address, to common.Address, value *big.Int) (*types.Transaction, error) {
	return _TokenizedCentralBankMoney.Contract.TransferFrom(&_TokenizedCentralBankMoney.TransactOpts, from, to, value)
}

// TransferFrom is a paid mutator transaction binding the contract method 0x23b872dd.
//
// Solidity: function transferFrom(address from, address to, uint256 value) returns(bool)
func (_TokenizedCentralBankMoney *TokenizedCentralBankMoneyTransactorSession) TransferFrom(from common.Address, to common.Address, value *big.Int) (*types.Transaction, error) {
	return _TokenizedCentralBankMoney.Contract.TransferFrom(&_TokenizedCentralBankMoney.TransactOpts, from, to, value)
}

// TokenizedCentralBankMoneyApprovalIterator is returned from FilterApproval and is used to iterate over the raw logs and unpacked data for Approval events raised by the TokenizedCentralBankMoney contract.
type TokenizedCentralBankMoneyApprovalIterator struct {
	Event *TokenizedCentralBankMoneyApproval // Event containing the contract specifics and raw log

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
func (it *TokenizedCentralBankMoneyApprovalIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(TokenizedCentralBankMoneyApproval)
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
		it.Event = new(TokenizedCentralBankMoneyApproval)
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
func (it *TokenizedCentralBankMoneyApprovalIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *TokenizedCentralBankMoneyApprovalIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// TokenizedCentralBankMoneyApproval represents a Approval event raised by the TokenizedCentralBankMoney contract.
type TokenizedCentralBankMoneyApproval struct {
	Owner   common.Address
	Spender common.Address
	Value   *big.Int
	Raw     types.Log // Blockchain specific contextual infos
}

// FilterApproval is a free log retrieval operation binding the contract event 0x8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b925.
//
// Solidity: event Approval(address indexed owner, address indexed spender, uint256 value)
func (_TokenizedCentralBankMoney *TokenizedCentralBankMoneyFilterer) FilterApproval(opts *bind.FilterOpts, owner []common.Address, spender []common.Address) (*TokenizedCentralBankMoneyApprovalIterator, error) {

	var ownerRule []interface{}
	for _, ownerItem := range owner {
		ownerRule = append(ownerRule, ownerItem)
	}
	var spenderRule []interface{}
	for _, spenderItem := range spender {
		spenderRule = append(spenderRule, spenderItem)
	}

	logs, sub, err := _TokenizedCentralBankMoney.contract.FilterLogs(opts, "Approval", ownerRule, spenderRule)
	if err != nil {
		return nil, err
	}
	return &TokenizedCentralBankMoneyApprovalIterator{contract: _TokenizedCentralBankMoney.contract, event: "Approval", logs: logs, sub: sub}, nil
}

// WatchApproval is a free log subscription operation binding the contract event 0x8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b925.
//
// Solidity: event Approval(address indexed owner, address indexed spender, uint256 value)
func (_TokenizedCentralBankMoney *TokenizedCentralBankMoneyFilterer) WatchApproval(opts *bind.WatchOpts, sink chan<- *TokenizedCentralBankMoneyApproval, owner []common.Address, spender []common.Address) (event.Subscription, error) {

	var ownerRule []interface{}
	for _, ownerItem := range owner {
		ownerRule = append(ownerRule, ownerItem)
	}
	var spenderRule []interface{}
	for _, spenderItem := range spender {
		spenderRule = append(spenderRule, spenderItem)
	}

	logs, sub, err := _TokenizedCentralBankMoney.contract.WatchLogs(opts, "Approval", ownerRule, spenderRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(TokenizedCentralBankMoneyApproval)
				if err := _TokenizedCentralBankMoney.contract.UnpackLog(event, "Approval", log); err != nil {
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

// ParseApproval is a log parse operation binding the contract event 0x8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b925.
//
// Solidity: event Approval(address indexed owner, address indexed spender, uint256 value)
func (_TokenizedCentralBankMoney *TokenizedCentralBankMoneyFilterer) ParseApproval(log types.Log) (*TokenizedCentralBankMoneyApproval, error) {
	event := new(TokenizedCentralBankMoneyApproval)
	if err := _TokenizedCentralBankMoney.contract.UnpackLog(event, "Approval", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// TokenizedCentralBankMoneyRoleAdminChangedIterator is returned from FilterRoleAdminChanged and is used to iterate over the raw logs and unpacked data for RoleAdminChanged events raised by the TokenizedCentralBankMoney contract.
type TokenizedCentralBankMoneyRoleAdminChangedIterator struct {
	Event *TokenizedCentralBankMoneyRoleAdminChanged // Event containing the contract specifics and raw log

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
func (it *TokenizedCentralBankMoneyRoleAdminChangedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(TokenizedCentralBankMoneyRoleAdminChanged)
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
		it.Event = new(TokenizedCentralBankMoneyRoleAdminChanged)
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
func (it *TokenizedCentralBankMoneyRoleAdminChangedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *TokenizedCentralBankMoneyRoleAdminChangedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// TokenizedCentralBankMoneyRoleAdminChanged represents a RoleAdminChanged event raised by the TokenizedCentralBankMoney contract.
type TokenizedCentralBankMoneyRoleAdminChanged struct {
	Role              [32]byte
	PreviousAdminRole [32]byte
	NewAdminRole      [32]byte
	Raw               types.Log // Blockchain specific contextual infos
}

// FilterRoleAdminChanged is a free log retrieval operation binding the contract event 0xbd79b86ffe0ab8e8776151514217cd7cacd52c909f66475c3af44e129f0b00ff.
//
// Solidity: event RoleAdminChanged(bytes32 indexed role, bytes32 indexed previousAdminRole, bytes32 indexed newAdminRole)
func (_TokenizedCentralBankMoney *TokenizedCentralBankMoneyFilterer) FilterRoleAdminChanged(opts *bind.FilterOpts, role [][32]byte, previousAdminRole [][32]byte, newAdminRole [][32]byte) (*TokenizedCentralBankMoneyRoleAdminChangedIterator, error) {

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

	logs, sub, err := _TokenizedCentralBankMoney.contract.FilterLogs(opts, "RoleAdminChanged", roleRule, previousAdminRoleRule, newAdminRoleRule)
	if err != nil {
		return nil, err
	}
	return &TokenizedCentralBankMoneyRoleAdminChangedIterator{contract: _TokenizedCentralBankMoney.contract, event: "RoleAdminChanged", logs: logs, sub: sub}, nil
}

// WatchRoleAdminChanged is a free log subscription operation binding the contract event 0xbd79b86ffe0ab8e8776151514217cd7cacd52c909f66475c3af44e129f0b00ff.
//
// Solidity: event RoleAdminChanged(bytes32 indexed role, bytes32 indexed previousAdminRole, bytes32 indexed newAdminRole)
func (_TokenizedCentralBankMoney *TokenizedCentralBankMoneyFilterer) WatchRoleAdminChanged(opts *bind.WatchOpts, sink chan<- *TokenizedCentralBankMoneyRoleAdminChanged, role [][32]byte, previousAdminRole [][32]byte, newAdminRole [][32]byte) (event.Subscription, error) {

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

	logs, sub, err := _TokenizedCentralBankMoney.contract.WatchLogs(opts, "RoleAdminChanged", roleRule, previousAdminRoleRule, newAdminRoleRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(TokenizedCentralBankMoneyRoleAdminChanged)
				if err := _TokenizedCentralBankMoney.contract.UnpackLog(event, "RoleAdminChanged", log); err != nil {
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
func (_TokenizedCentralBankMoney *TokenizedCentralBankMoneyFilterer) ParseRoleAdminChanged(log types.Log) (*TokenizedCentralBankMoneyRoleAdminChanged, error) {
	event := new(TokenizedCentralBankMoneyRoleAdminChanged)
	if err := _TokenizedCentralBankMoney.contract.UnpackLog(event, "RoleAdminChanged", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// TokenizedCentralBankMoneyRoleGrantedIterator is returned from FilterRoleGranted and is used to iterate over the raw logs and unpacked data for RoleGranted events raised by the TokenizedCentralBankMoney contract.
type TokenizedCentralBankMoneyRoleGrantedIterator struct {
	Event *TokenizedCentralBankMoneyRoleGranted // Event containing the contract specifics and raw log

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
func (it *TokenizedCentralBankMoneyRoleGrantedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(TokenizedCentralBankMoneyRoleGranted)
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
		it.Event = new(TokenizedCentralBankMoneyRoleGranted)
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
func (it *TokenizedCentralBankMoneyRoleGrantedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *TokenizedCentralBankMoneyRoleGrantedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// TokenizedCentralBankMoneyRoleGranted represents a RoleGranted event raised by the TokenizedCentralBankMoney contract.
type TokenizedCentralBankMoneyRoleGranted struct {
	Role    [32]byte
	Account common.Address
	Sender  common.Address
	Raw     types.Log // Blockchain specific contextual infos
}

// FilterRoleGranted is a free log retrieval operation binding the contract event 0x2f8788117e7eff1d82e926ec794901d17c78024a50270940304540a733656f0d.
//
// Solidity: event RoleGranted(bytes32 indexed role, address indexed account, address indexed sender)
func (_TokenizedCentralBankMoney *TokenizedCentralBankMoneyFilterer) FilterRoleGranted(opts *bind.FilterOpts, role [][32]byte, account []common.Address, sender []common.Address) (*TokenizedCentralBankMoneyRoleGrantedIterator, error) {

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

	logs, sub, err := _TokenizedCentralBankMoney.contract.FilterLogs(opts, "RoleGranted", roleRule, accountRule, senderRule)
	if err != nil {
		return nil, err
	}
	return &TokenizedCentralBankMoneyRoleGrantedIterator{contract: _TokenizedCentralBankMoney.contract, event: "RoleGranted", logs: logs, sub: sub}, nil
}

// WatchRoleGranted is a free log subscription operation binding the contract event 0x2f8788117e7eff1d82e926ec794901d17c78024a50270940304540a733656f0d.
//
// Solidity: event RoleGranted(bytes32 indexed role, address indexed account, address indexed sender)
func (_TokenizedCentralBankMoney *TokenizedCentralBankMoneyFilterer) WatchRoleGranted(opts *bind.WatchOpts, sink chan<- *TokenizedCentralBankMoneyRoleGranted, role [][32]byte, account []common.Address, sender []common.Address) (event.Subscription, error) {

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

	logs, sub, err := _TokenizedCentralBankMoney.contract.WatchLogs(opts, "RoleGranted", roleRule, accountRule, senderRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(TokenizedCentralBankMoneyRoleGranted)
				if err := _TokenizedCentralBankMoney.contract.UnpackLog(event, "RoleGranted", log); err != nil {
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
func (_TokenizedCentralBankMoney *TokenizedCentralBankMoneyFilterer) ParseRoleGranted(log types.Log) (*TokenizedCentralBankMoneyRoleGranted, error) {
	event := new(TokenizedCentralBankMoneyRoleGranted)
	if err := _TokenizedCentralBankMoney.contract.UnpackLog(event, "RoleGranted", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// TokenizedCentralBankMoneyRoleRevokedIterator is returned from FilterRoleRevoked and is used to iterate over the raw logs and unpacked data for RoleRevoked events raised by the TokenizedCentralBankMoney contract.
type TokenizedCentralBankMoneyRoleRevokedIterator struct {
	Event *TokenizedCentralBankMoneyRoleRevoked // Event containing the contract specifics and raw log

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
func (it *TokenizedCentralBankMoneyRoleRevokedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(TokenizedCentralBankMoneyRoleRevoked)
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
		it.Event = new(TokenizedCentralBankMoneyRoleRevoked)
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
func (it *TokenizedCentralBankMoneyRoleRevokedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *TokenizedCentralBankMoneyRoleRevokedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// TokenizedCentralBankMoneyRoleRevoked represents a RoleRevoked event raised by the TokenizedCentralBankMoney contract.
type TokenizedCentralBankMoneyRoleRevoked struct {
	Role    [32]byte
	Account common.Address
	Sender  common.Address
	Raw     types.Log // Blockchain specific contextual infos
}

// FilterRoleRevoked is a free log retrieval operation binding the contract event 0xf6391f5c32d9c69d2a47ea670b442974b53935d1edc7fd64eb21e047a839171b.
//
// Solidity: event RoleRevoked(bytes32 indexed role, address indexed account, address indexed sender)
func (_TokenizedCentralBankMoney *TokenizedCentralBankMoneyFilterer) FilterRoleRevoked(opts *bind.FilterOpts, role [][32]byte, account []common.Address, sender []common.Address) (*TokenizedCentralBankMoneyRoleRevokedIterator, error) {

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

	logs, sub, err := _TokenizedCentralBankMoney.contract.FilterLogs(opts, "RoleRevoked", roleRule, accountRule, senderRule)
	if err != nil {
		return nil, err
	}
	return &TokenizedCentralBankMoneyRoleRevokedIterator{contract: _TokenizedCentralBankMoney.contract, event: "RoleRevoked", logs: logs, sub: sub}, nil
}

// WatchRoleRevoked is a free log subscription operation binding the contract event 0xf6391f5c32d9c69d2a47ea670b442974b53935d1edc7fd64eb21e047a839171b.
//
// Solidity: event RoleRevoked(bytes32 indexed role, address indexed account, address indexed sender)
func (_TokenizedCentralBankMoney *TokenizedCentralBankMoneyFilterer) WatchRoleRevoked(opts *bind.WatchOpts, sink chan<- *TokenizedCentralBankMoneyRoleRevoked, role [][32]byte, account []common.Address, sender []common.Address) (event.Subscription, error) {

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

	logs, sub, err := _TokenizedCentralBankMoney.contract.WatchLogs(opts, "RoleRevoked", roleRule, accountRule, senderRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(TokenizedCentralBankMoneyRoleRevoked)
				if err := _TokenizedCentralBankMoney.contract.UnpackLog(event, "RoleRevoked", log); err != nil {
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
func (_TokenizedCentralBankMoney *TokenizedCentralBankMoneyFilterer) ParseRoleRevoked(log types.Log) (*TokenizedCentralBankMoneyRoleRevoked, error) {
	event := new(TokenizedCentralBankMoneyRoleRevoked)
	if err := _TokenizedCentralBankMoney.contract.UnpackLog(event, "RoleRevoked", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// TokenizedCentralBankMoneyTransferIterator is returned from FilterTransfer and is used to iterate over the raw logs and unpacked data for Transfer events raised by the TokenizedCentralBankMoney contract.
type TokenizedCentralBankMoneyTransferIterator struct {
	Event *TokenizedCentralBankMoneyTransfer // Event containing the contract specifics and raw log

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
func (it *TokenizedCentralBankMoneyTransferIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(TokenizedCentralBankMoneyTransfer)
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
		it.Event = new(TokenizedCentralBankMoneyTransfer)
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
func (it *TokenizedCentralBankMoneyTransferIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *TokenizedCentralBankMoneyTransferIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// TokenizedCentralBankMoneyTransfer represents a Transfer event raised by the TokenizedCentralBankMoney contract.
type TokenizedCentralBankMoneyTransfer struct {
	From  common.Address
	To    common.Address
	Value *big.Int
	Raw   types.Log // Blockchain specific contextual infos
}

// FilterTransfer is a free log retrieval operation binding the contract event 0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef.
//
// Solidity: event Transfer(address indexed from, address indexed to, uint256 value)
func (_TokenizedCentralBankMoney *TokenizedCentralBankMoneyFilterer) FilterTransfer(opts *bind.FilterOpts, from []common.Address, to []common.Address) (*TokenizedCentralBankMoneyTransferIterator, error) {

	var fromRule []interface{}
	for _, fromItem := range from {
		fromRule = append(fromRule, fromItem)
	}
	var toRule []interface{}
	for _, toItem := range to {
		toRule = append(toRule, toItem)
	}

	logs, sub, err := _TokenizedCentralBankMoney.contract.FilterLogs(opts, "Transfer", fromRule, toRule)
	if err != nil {
		return nil, err
	}
	return &TokenizedCentralBankMoneyTransferIterator{contract: _TokenizedCentralBankMoney.contract, event: "Transfer", logs: logs, sub: sub}, nil
}

// WatchTransfer is a free log subscription operation binding the contract event 0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef.
//
// Solidity: event Transfer(address indexed from, address indexed to, uint256 value)
func (_TokenizedCentralBankMoney *TokenizedCentralBankMoneyFilterer) WatchTransfer(opts *bind.WatchOpts, sink chan<- *TokenizedCentralBankMoneyTransfer, from []common.Address, to []common.Address) (event.Subscription, error) {

	var fromRule []interface{}
	for _, fromItem := range from {
		fromRule = append(fromRule, fromItem)
	}
	var toRule []interface{}
	for _, toItem := range to {
		toRule = append(toRule, toItem)
	}

	logs, sub, err := _TokenizedCentralBankMoney.contract.WatchLogs(opts, "Transfer", fromRule, toRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(TokenizedCentralBankMoneyTransfer)
				if err := _TokenizedCentralBankMoney.contract.UnpackLog(event, "Transfer", log); err != nil {
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

// ParseTransfer is a log parse operation binding the contract event 0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef.
//
// Solidity: event Transfer(address indexed from, address indexed to, uint256 value)
func (_TokenizedCentralBankMoney *TokenizedCentralBankMoneyFilterer) ParseTransfer(log types.Log) (*TokenizedCentralBankMoneyTransfer, error) {
	event := new(TokenizedCentralBankMoneyTransfer)
	if err := _TokenizedCentralBankMoney.contract.UnpackLog(event, "Transfer", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}
