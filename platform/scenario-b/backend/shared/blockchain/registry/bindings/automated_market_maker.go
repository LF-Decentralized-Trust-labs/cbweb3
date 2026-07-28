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
	ABI: "[{\"type\":\"constructor\",\"inputs\":[{\"name\":\"_tokenA\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"_tokenB\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"_identityRegistry\",\"type\":\"address\",\"internalType\":\"address\"}],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"BURN_ADDRESS\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"address\",\"internalType\":\"address\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"IDENTITY_REGISTRY\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"address\",\"internalType\":\"contractIIdentityRegistry\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"MAX_FEE_BPS\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"MINIMUM_LIQUIDITY\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"RESUME_QUORUM\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"TOKEN_A\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"address\",\"internalType\":\"contractIERC20\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"TOKEN_B\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"address\",\"internalType\":\"contractIERC20\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"addLiquidity\",\"inputs\":[{\"name\":\"amountA\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"amountB\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[{\"name\":\"shares\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"allowance\",\"inputs\":[{\"name\":\"owner\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"spender\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"approve\",\"inputs\":[{\"name\":\"spender\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"value\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[{\"name\":\"\",\"type\":\"bool\",\"internalType\":\"bool\"}],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"balanceOf\",\"inputs\":[{\"name\":\"account\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"cancelCommitDeposit\",\"inputs\":[{\"name\":\"commitId\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"},{\"name\":\"isTokenA\",\"type\":\"bool\",\"internalType\":\"bool\"}],\"outputs\":[{\"name\":\"amount\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"decimals\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"uint8\",\"internalType\":\"uint8\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"depositForCommit\",\"inputs\":[{\"name\":\"commitId\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"},{\"name\":\"isTokenA\",\"type\":\"bool\",\"internalType\":\"bool\"},{\"name\":\"amount\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"shareRecipient\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"feeBps\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"finalizeCommit\",\"inputs\":[{\"name\":\"commitId\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"}],\"outputs\":[{\"name\":\"sharesA\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"sharesB\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"getAmountIn\",\"inputs\":[{\"name\":\"reserveIn\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"reserveOut\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"amountOut\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[{\"name\":\"amountIn\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"pure\"},{\"type\":\"function\",\"name\":\"getAmountOut\",\"inputs\":[{\"name\":\"amountIn\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"reserveIn\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"reserveOut\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"feeBps_\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[{\"name\":\"amountOut\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"pure\"},{\"type\":\"function\",\"name\":\"getEscrow\",\"inputs\":[{\"name\":\"commitId\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"}],\"outputs\":[{\"name\":\"depositorA\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"depositorB\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"recipientA\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"recipientB\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"amountA\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"amountB\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"finalized\",\"type\":\"bool\",\"internalType\":\"bool\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"isPaused\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"bool\",\"internalType\":\"bool\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"name\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"string\",\"internalType\":\"string\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"pause\",\"inputs\":[{\"name\":\"reason\",\"type\":\"string\",\"internalType\":\"string\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"proposeResume\",\"inputs\":[],\"outputs\":[{\"name\":\"proposalId\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"}],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"removeLiquidity\",\"inputs\":[{\"name\":\"shares\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"tokenOut\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"minAmountOut\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[{\"name\":\"amountOut\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"removeLiquidityEmergency\",\"inputs\":[{\"name\":\"shares\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[{\"name\":\"amountA\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"amountB\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"reserveA\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"reserveB\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"resumeQuorum\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"pure\"},{\"type\":\"function\",\"name\":\"resumeSignatures\",\"inputs\":[{\"name\":\"proposalId\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"}],\"outputs\":[{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"setFeeBps\",\"inputs\":[{\"name\":\"newFeeBps\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"setWithdrawalFeeBps\",\"inputs\":[{\"name\":\"newWithdrawalFeeBps\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"signResume\",\"inputs\":[{\"name\":\"proposalId\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"swapTokensForExactTokens\",\"inputs\":[{\"name\":\"tokenIn\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"tokenOut\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"amountOut\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"maxAmountIn\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"to\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[{\"name\":\"amountIn\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"symbol\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"string\",\"internalType\":\"string\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"totalSupply\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"transfer\",\"inputs\":[{\"name\":\"to\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"value\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[{\"name\":\"\",\"type\":\"bool\",\"internalType\":\"bool\"}],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"transferFrom\",\"inputs\":[{\"name\":\"from\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"to\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"value\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[{\"name\":\"\",\"type\":\"bool\",\"internalType\":\"bool\"}],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"withdrawalFeeBps\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"event\",\"name\":\"Approval\",\"inputs\":[{\"name\":\"owner\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"spender\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"value\",\"type\":\"uint256\",\"indexed\":false,\"internalType\":\"uint256\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"LogCircuitBreakerPaused\",\"inputs\":[{\"name\":\"pausedBy\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"timestamp\",\"type\":\"uint256\",\"indexed\":false,\"internalType\":\"uint256\"},{\"name\":\"reason\",\"type\":\"string\",\"indexed\":false,\"internalType\":\"string\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"LogCircuitBreakerResumed\",\"inputs\":[{\"name\":\"proposalId\",\"type\":\"bytes32\",\"indexed\":true,\"internalType\":\"bytes32\"},{\"name\":\"timestamp\",\"type\":\"uint256\",\"indexed\":false,\"internalType\":\"uint256\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"LogCommitDeposit\",\"inputs\":[{\"name\":\"commitId\",\"type\":\"bytes32\",\"indexed\":true,\"internalType\":\"bytes32\"},{\"name\":\"depositor\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"recipient\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"isTokenA\",\"type\":\"bool\",\"indexed\":false,\"internalType\":\"bool\"},{\"name\":\"amount\",\"type\":\"uint256\",\"indexed\":false,\"internalType\":\"uint256\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"LogCommitFinalized\",\"inputs\":[{\"name\":\"commitId\",\"type\":\"bytes32\",\"indexed\":true,\"internalType\":\"bytes32\"},{\"name\":\"recipientA\",\"type\":\"address\",\"indexed\":false,\"internalType\":\"address\"},{\"name\":\"recipientB\",\"type\":\"address\",\"indexed\":false,\"internalType\":\"address\"},{\"name\":\"sharesA\",\"type\":\"uint256\",\"indexed\":false,\"internalType\":\"uint256\"},{\"name\":\"sharesB\",\"type\":\"uint256\",\"indexed\":false,\"internalType\":\"uint256\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"LogCommitRefunded\",\"inputs\":[{\"name\":\"commitId\",\"type\":\"bytes32\",\"indexed\":true,\"internalType\":\"bytes32\"},{\"name\":\"depositor\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"isTokenA\",\"type\":\"bool\",\"indexed\":false,\"internalType\":\"bool\"},{\"name\":\"amount\",\"type\":\"uint256\",\"indexed\":false,\"internalType\":\"uint256\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"LogEmergencyWithdrawal\",\"inputs\":[{\"name\":\"provider\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"sharesBurned\",\"type\":\"uint256\",\"indexed\":false,\"internalType\":\"uint256\"},{\"name\":\"amountTokenA\",\"type\":\"uint256\",\"indexed\":false,\"internalType\":\"uint256\"},{\"name\":\"amountTokenB\",\"type\":\"uint256\",\"indexed\":false,\"internalType\":\"uint256\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"LogFeeRateUpdated\",\"inputs\":[{\"name\":\"oldFeeBps\",\"type\":\"uint256\",\"indexed\":false,\"internalType\":\"uint256\"},{\"name\":\"newFeeBps\",\"type\":\"uint256\",\"indexed\":false,\"internalType\":\"uint256\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"LogLiquidityAdded\",\"inputs\":[{\"name\":\"provider\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"amountTokenA\",\"type\":\"uint256\",\"indexed\":false,\"internalType\":\"uint256\"},{\"name\":\"amountTokenB\",\"type\":\"uint256\",\"indexed\":false,\"internalType\":\"uint256\"},{\"name\":\"sharesMinted\",\"type\":\"uint256\",\"indexed\":false,\"internalType\":\"uint256\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"LogLiquidityRemoved\",\"inputs\":[{\"name\":\"provider\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"sharesBurned\",\"type\":\"uint256\",\"indexed\":false,\"internalType\":\"uint256\"},{\"name\":\"tokenOut\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"amountOut\",\"type\":\"uint256\",\"indexed\":false,\"internalType\":\"uint256\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"LogResumeProposed\",\"inputs\":[{\"name\":\"proposalId\",\"type\":\"bytes32\",\"indexed\":true,\"internalType\":\"bytes32\"},{\"name\":\"proposedBy\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"timestamp\",\"type\":\"uint256\",\"indexed\":false,\"internalType\":\"uint256\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"LogResumeSigned\",\"inputs\":[{\"name\":\"proposalId\",\"type\":\"bytes32\",\"indexed\":true,\"internalType\":\"bytes32\"},{\"name\":\"signer\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"signaturesCount\",\"type\":\"uint256\",\"indexed\":false,\"internalType\":\"uint256\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"LogSwap\",\"inputs\":[{\"name\":\"user\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"tokenIn\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"tokenOut\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"amountIn\",\"type\":\"uint256\",\"indexed\":false,\"internalType\":\"uint256\"},{\"name\":\"amountOut\",\"type\":\"uint256\",\"indexed\":false,\"internalType\":\"uint256\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"LogWithdrawalFeeRateUpdated\",\"inputs\":[{\"name\":\"oldWithdrawalFeeBps\",\"type\":\"uint256\",\"indexed\":false,\"internalType\":\"uint256\"},{\"name\":\"newWithdrawalFeeBps\",\"type\":\"uint256\",\"indexed\":false,\"internalType\":\"uint256\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"Transfer\",\"inputs\":[{\"name\":\"from\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"to\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"value\",\"type\":\"uint256\",\"indexed\":false,\"internalType\":\"uint256\"}],\"anonymous\":false},{\"type\":\"error\",\"name\":\"AMM__AlreadyPaused\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"AMM__AlreadySigned\",\"inputs\":[{\"name\":\"proposalId\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"},{\"name\":\"signer\",\"type\":\"address\",\"internalType\":\"address\"}]},{\"type\":\"error\",\"name\":\"AMM__CommitAlreadyFinalized\",\"inputs\":[{\"name\":\"commitId\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"}]},{\"type\":\"error\",\"name\":\"AMM__CommitIncomplete\",\"inputs\":[{\"name\":\"commitId\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"}]},{\"type\":\"error\",\"name\":\"AMM__FeeBpsTooHigh\",\"inputs\":[{\"name\":\"provided\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"max\",\"type\":\"uint256\",\"internalType\":\"uint256\"}]},{\"type\":\"error\",\"name\":\"AMM__InsufficientLiquidity\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"AMM__InsufficientOutputAmount\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"AMM__InvalidToken\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"AMM__NotDepositor\",\"inputs\":[{\"name\":\"commitId\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"}]},{\"type\":\"error\",\"name\":\"AMM__NotGovernance\",\"inputs\":[{\"name\":\"account\",\"type\":\"address\",\"internalType\":\"address\"}]},{\"type\":\"error\",\"name\":\"AMM__NotPaused\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"AMM__NothingToRefund\",\"inputs\":[{\"name\":\"commitId\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"}]},{\"type\":\"error\",\"name\":\"AMM__ParticipantNotVerified\",\"inputs\":[{\"name\":\"account\",\"type\":\"address\",\"internalType\":\"address\"}]},{\"type\":\"error\",\"name\":\"AMM__ProposalNotFound\",\"inputs\":[{\"name\":\"proposalId\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"}]},{\"type\":\"error\",\"name\":\"AMM__ProposalQuorumIncomplete\",\"inputs\":[{\"name\":\"proposalId\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"},{\"name\":\"signatures\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"required\",\"type\":\"uint256\",\"internalType\":\"uint256\"}]},{\"type\":\"error\",\"name\":\"AMM__SideAlreadyDeposited\",\"inputs\":[{\"name\":\"commitId\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"},{\"name\":\"isTokenA\",\"type\":\"bool\",\"internalType\":\"bool\"}]},{\"type\":\"error\",\"name\":\"AMM__SlippageExceeded\",\"inputs\":[{\"name\":\"requiredAmountIn\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"maxAmountIn\",\"type\":\"uint256\",\"internalType\":\"uint256\"}]},{\"type\":\"error\",\"name\":\"AMM__ZeroAddress\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"AMM__ZeroAmount\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"AddressEmptyCode\",\"inputs\":[{\"name\":\"target\",\"type\":\"address\",\"internalType\":\"address\"}]},{\"type\":\"error\",\"name\":\"AddressInsufficientBalance\",\"inputs\":[{\"name\":\"account\",\"type\":\"address\",\"internalType\":\"address\"}]},{\"type\":\"error\",\"name\":\"ERC20InsufficientAllowance\",\"inputs\":[{\"name\":\"spender\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"allowance\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"needed\",\"type\":\"uint256\",\"internalType\":\"uint256\"}]},{\"type\":\"error\",\"name\":\"ERC20InsufficientBalance\",\"inputs\":[{\"name\":\"sender\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"balance\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"needed\",\"type\":\"uint256\",\"internalType\":\"uint256\"}]},{\"type\":\"error\",\"name\":\"ERC20InvalidApprover\",\"inputs\":[{\"name\":\"approver\",\"type\":\"address\",\"internalType\":\"address\"}]},{\"type\":\"error\",\"name\":\"ERC20InvalidReceiver\",\"inputs\":[{\"name\":\"receiver\",\"type\":\"address\",\"internalType\":\"address\"}]},{\"type\":\"error\",\"name\":\"ERC20InvalidSender\",\"inputs\":[{\"name\":\"sender\",\"type\":\"address\",\"internalType\":\"address\"}]},{\"type\":\"error\",\"name\":\"ERC20InvalidSpender\",\"inputs\":[{\"name\":\"spender\",\"type\":\"address\",\"internalType\":\"address\"}]},{\"type\":\"error\",\"name\":\"FailedInnerCall\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"ReentrancyGuardReentrantCall\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"SafeERC20FailedOperation\",\"inputs\":[{\"name\":\"token\",\"type\":\"address\",\"internalType\":\"address\"}]}]",
	Bin: "0x60e034620004a257601f906001600160401b0390601f1962002e4f3881900385810183168401919085831185841017620003a657808592606094604052833981010312620004a2576200005282620004c7565b9360209362000071604062000069878701620004c7565b9501620004c7565b946200007c620004a7565b92601184527004342576562332048756220414d4d204c5607c1b82850152620000a4620004a7565b60078152660434257332d4c560cc1b83820152845192848411620003a65760039384546001978882811c9216801562000497575b84831014620004815781868493116200042b575b508390868311600114620003c857600092620003bc575b505060001982871b1c191690871b1784555b8151948511620003a65760049687548781811c911680156200039b575b8382101462000386579081858897969594931162000328575b5081938611600114620002bd575050600093620002b1575b505082841b92600019911b1c19161782555b6005556001600160a01b039384169081158015620002a6575b80156200029b575b6200028c5750608052821660a0521660c05260006009819055600a556040516129729081620004dd82396080518181816101d8015281816103270152818161084d0152818161096801528181610c7201528181610fe70152611e9b015260a051818181610353015281816104f90152818161081d01528181610ab501528181610af201528181610d4f015281816110130152818161176f0152611f63015260c05181818161029a015281816107280152818161092701528181610deb01528181610f910152818161129201528181611480015281816115ea015281816116ff0152818161180801528181611ddb01526121930152f35b604051633eff773f60e21b8152fd5b508484161562000196565b50848316156200018e565b01519150388062000163565b87969493929194168860005284600020946000905b8282106200030e5750508511620002f3575b50505050811b01825562000175565b01519060f884600019921b161c1916905538808080620002e4565b8484015187558998909601959384019390810190620002d2565b9091929394955088600052826000208580890160051c820192858a106200037c575b918a918a999897969594930160051c01915b8281106200036c5750506200014b565b600081558998508a91016200035c565b925081926200034a565b602289634e487b7160e01b6000525260246000fd5b90607f169062000132565b634e487b7160e01b600052604160045260246000fd5b01519050388062000103565b908a8a94169188600052856000209260005b87828210620004145750508411620003fb575b505050811b01845562000115565b015160001983891b60f8161c19169055388080620003ed565b8385015186558d97909501949384019301620003da565b90915086600052836000208680850160051c82019286861062000477575b918b91869594930160051c01915b82811062000467575050620000ec565b600081558594508b910162000457565b9250819262000449565b634e487b7160e01b600052602260045260246000fd5b91607f1691620000d8565b600080fd5b60408051919082016001600160401b03811183821017620003a657604052565b51906001600160a01b0382168203620004a25756fe6040608081526004908136101561001557600080fd5b600091823560e01c90816301c3213114611d7757816304336bb314611d5857816306fdde0314611c63578163095ea7b314611bb957816318160ddd14611b9a57816319e36f3b14611b7b5781631e9b551d1461199c57816323b872dd146118a857816324167524146117d957816324a9d853146117ba578163313ce5671461179e578163499f712e1461175a57816352707d8c1461172e57816365fad027146116ea5781636da663551461158857816370a082311461155157816372c27b6214611451578163733ffb2b1461126257816385f8c2591461123057816395d89b411461112d5781639c722cde146111025781639cd441da14610f4e5781639e3f8fd714610dbd578163a054816f14610bea578163a9059cbb14610bb9578163abdfb5b214610b79578163b187bd2614610b95578163ba9a7a56146106e0578163bb6fc5e714610b79578163c4ccdeea146108d3578163c9754c38146106e5578163d55be8c6146106e0578163dc5fa6c5146106c1578163dd62ed3e14610678578163f023b811146105f7578163f7d318091461022757508063fccc28131461020b5763ff709ee8146101c557600080fd5b34610207578160031936011261020757517f00000000000000000000000000000000000000000000000000000000000000006001600160a01b03168152602090f35b5080fd5b50346102075781600319360112610207576020905161dead8152f35b8383346102075760a03660031901126102075761024261208e565b61024a6120a4565b936064359060443590608435906001600160a01b03808316918284036105f357610272612532565b60ff600854166105e557875163acf0279f60e01b808252338383015260209b602495909290917f00000000000000000000000000000000000000000000000000000000000000008616918e818981865afa9081156105d9578f8f926105bc575b5050156105a7578d90878d5180948193825286898301525afa90811561059d578c91610570575b501561055b5750851561054c5782169682169586881461053d576006549384158015610533575b61052357837f00000000000000000000000000000000000000000000000000000000000000001680891494856104f7575b7f000000000000000000000000000000000000000000000000000000000000000016891490816104ed575b50841590816104e4575b506104d45783156104c8576103a687865b86156104c1576007549061287b565b61271090818102908082048314901517156104af5760095482039182116104af57906103d191612568565b9a60018c01809c1161049f5750818b11610484575050509183918261042a946000146104645750506104058860065461233f565b60065561041482600754612588565b6007555b610424883033886123ae565b8561279e565b835190858252868201527f499f47d29fe8ad39124b5e7e7864cb954b8c73bb602f3853cac827a3128076d3843392a4600160055551908152f35b61047c916104748b60075461233f565b600755612588565b600655610418565b895163657b126560e11b81529283018b905282015260449150fd5b634e487b7160e01b815260118452fd5b634e487b7160e01b8d5260118552828dfd5b879061287b565b6103a687600754610397565b8951635875f49f60e01b81528390fd5b9050158d610386565b905089148d61037c565b7f000000000000000000000000000000000000000000000000000000000000000081168b149550610351565b8951631ed5ab6160e31b81528390fd5b5060075415610320565b508751635875f49f60e01b8152fd5b50875163250d596f60e21b8152fd5b8285918b5191631c52846560e01b8352820152fd5b61059091508d803d10610596575b6105888183612120565b810190612327565b8d6102f9565b503d61057e565b8b513d8e823e3d90fd5b8b51631c52846560e01b815233818701528790fd5b6105d29250803d10610596576105888183612120565b8f8f6102d2565b8e8e51903d90823e3d90fd5b8751633f01e74960e11b8152fd5b8880fd5b91905034610674576020366003190112610674578060e09383358152600c6020522060018060a01b0391828254169383600184015416938060028501541690600385015416918401549260ff600660058701549601541695815197885260208801528601526060850152608084015260a0830152151560c0820152f35b8280fd5b5050346102075780600319360112610207578060209261069661208e565b61069e6120a4565b6001600160a01b0391821683526001865283832091168252845220549051908152f35b5050346102075781600319360112610207576020906006549051908152f35b612103565b8391503461020757602036600319011261020757803591610704612532565b60ff60085416156108c457835163acf0279f60e01b815233838201526020816024817f00000000000000000000000000000000000000000000000000000000000000006001600160a01b03165afa9182156108b9579161089b575b50156108845781156108765750600254906107956107888361078360065485612555565b612568565b9261078360075484612555565b906107a081336127ee565b6107ac83600654612588565b6006556107bb82600754612588565b60075582610846575b81610816575b8351908152602081018390526040810182905233907f945f4cfb97c69b809044088a94cd4f9a9bde62be502babb58bb8ebe059fcd29d90606090a2600160055582519182526020820152f35b61084182337f000000000000000000000000000000000000000000000000000000000000000061279e565b6107ca565b61087183337f000000000000000000000000000000000000000000000000000000000000000061279e565b6107c4565b825163250d596f60e21b8152fd5b602490835190631c52846560e01b82523390820152fd5b6108b3915060203d8111610596576105888183612120565b8461075f565b8551903d90823e3d90fd5b50825163f36ac62d60e01b8152fd5b838334610207576060366003190112610207578235916108f16120a4565b926108fa612532565b60ff60085416610b6957825163acf0279f60e01b815233868201526001600160a01b0392906020816024817f000000000000000000000000000000000000000000000000000000000000000088165afa908115610b5f578291610b41575b5015610b2b57508015610b1b57817f000000000000000000000000000000000000000000000000000000000000000094169180851683149081159081610aee575b50610ade5760025460066109b282610783835487612555565b916109c4600791610783835488612555565b936109cf86336127ee565b6109da848454612588565b908184556109e9868454612588565b8084559015610a8e57505091610a379391610a2784610a2e955490835490610a20610a18600a5484868b6128a8565b97889461233f565b9055612588565b905561233f565b8095339061279e565b6044358410610a7e5760209450825190815283858201527f896db5cbb8fec9d3fcb0b08630c8ef5cfd04e4424feb2a6a21d42ddef5d67736833392a3600160055551908152f35b8251634b27f19f60e01b81528590fd5b610aad969950610a279293610a20610a18600a9897985484868b6128a8565b92610ad984337f000000000000000000000000000000000000000000000000000000000000000061279e565b610a37565b8351635875f49f60e01b81528690fd5b90507f00000000000000000000000000000000000000000000000000000000000000001683141587610999565b825163250d596f60e21b81528590fd5b8351631c52846560e01b81523381880152602490fd5b610b59915060203d8111610596576105888183612120565b87610958565b85513d84823e3d90fd5b8251633f01e74960e11b81528590fd5b5050346102075781600319360112610207576020905160028152f35b50503461020757816003193601126102075760209060ff6008541690519015158152f35b505034610207578060031936011261020757602090610be3610bd961208e565b6024359033612158565b5160018152f35b919050346106745780600319360112610674578135610c0761207a565b90610c10612532565b808552600c6020528285209460ff600687015416610da6578215610d05578554336001600160a01b03821603610cee57858701908154968715610cd7575060026020986001600160601b0360a01b80931681550190815416905555610c9684337f000000000000000000000000000000000000000000000000000000000000000061279e565b825191151582526020820184905233917ff6c2017497779a6f9357b3771c9ee63b3310a27bbf53b8293a8a215ba4a701c490604090a3600160055551908152f35b8651639ecf085760e01b8152908101859052602490fd5b8451639e1c29d760e01b8152808701849052602490fd5b600186018054336001600160a01b03821603610d8f5760058801918254978815610d7857506020986003916001600160601b0360a01b80941690550190815416905555610d7384337f000000000000000000000000000000000000000000000000000000000000000061279e565b610c96565b8751639ecf085760e01b8152908101869052602490fd5b8551639e1c29d760e01b8152808801859052602490fd5b8351630bbf191d60e31b8152808601839052602490fd5b83833461020757816003193601126102075780516353aa430760e01b815233848201526020939084816024817f00000000000000000000000000000000000000000000000000000000000000006001600160a01b03165afa908115610f41578491610f24575b5015610f0e5760ff6008541615610f0057508051338482019081524360208201524260408201528291610e62816060840103601f198101835282612120565b51902092838152600b85526003828220336001600160601b0360a01b8254161781554260018201556001600282015533835201855220600160ff198254161790558051428152827f3bbaac345bb995e6e6d4dd57822687a8426ea5eac92cc095b6d0a6ca763e8323853393a3805160018152827f81239dd1c0dbda2f6dc7c61c52c24f4f1f8540214d4ffd61ada9c51655aa15f7853393a351908152f35b905163f36ac62d60e01b8152fd5b602491519063f829dd5160e01b82523390820152fd5b610f3b9150853d8711610596576105888183612120565b85610e23565b50505051903d90823e3d90fd5b838334610207578060031936011261020757823560243592610f6e612532565b60ff60085416610b6957825163acf0279f60e01b815233868201526020816024817f00000000000000000000000000000000000000000000000000000000000000006001600160a01b03165afa9182156110f757916110d9575b50156110c357801580156110bb575b6110ab576020935061100b8130337f00000000000000000000000000000000000000000000000000000000000000006123ae565b6110378330337f00000000000000000000000000000000000000000000000000000000000000006123ae565b7ef00467526ab75114da7a5ec65bb1b6755a6a23d97b65fe8fc1662c920c60446110618483612595565b809461106f8460065461233f565b60065561107e8160075461233f565b60075561108b8233612362565b8451938452602084015260408301523391606090a2600160055551908152f35b815163250d596f60e21b81528490fd5b508215610fd7565b8151631c52846560e01b81523381860152602490fd5b6110f1915060203d8111610596576105888183612120565b85610fc8565b8451903d90823e3d90fd5b9050346106745760203660031901126106745781602093600292358152600b85522001549051908152f35b838334610207578160031936011261020757805191809380549160019083821c92828516948515611226575b6020958686108114611213578589529081156111ef5750600114611197575b6111938787611189828c0383612120565b51918291826120ba565b0390f35b81529295507f8a35acfbc15ff81a39ae7d344fd709f28e8600b4aa8c65c6b64bfe7fe36bd19b5b8284106111dc57505050826111939461118992820101948680611178565b80548685018801529286019281016111be565b60ff19168887015250505050151560051b8301019250611189826111938680611178565b634e487b7160e01b845260228352602484fd5b93607f1693611159565b82843461125f57606036600319011261125f575061125860209260443590602435903561287b565b9051908152f35b80fd5b9050346106745760208060031936011261144d5782516353aa430760e01b815233818401528235939082816024817f00000000000000000000000000000000000000000000000000000000000000006001600160a01b03165afa908115611443578691611426575b50156114115760ff600854161561140257838552600b8252808520926001840154156113ed576003840133875280845260ff83882054166113d15733875283528186209060ff199160018382541617905560028501805490600182018092116113be579080600292558451818152887f81239dd1c0dbda2f6dc7c61c52c24f4f1f8540214d4ffd61ada9c51655aa15f7883393a31015806113b0575b61136e578680f35b7f191e4e50c96c12749e054ea243f820456a9ce7d59b4695a95d1465b26e21c90394016001828254161790556008541660085551428152a23880808080808680f35b5060ff818601541615611366565b634e487b7160e01b895260118352602489fd5b50846044925191630bd2d04d60e11b8352820152336024820152fd5b846024925191632eea7e6f60e11b8352820152fd5b5163f36ac62d60e01b81529050fd5b5163f829dd5160e01b81523381840152602490fd5b61143d9150833d8511610596576105888183612120565b386112ca565b82513d88823e3d90fd5b8380fd5b8383346102075760203660031901126102075780516353aa430760e01b815233818501528335906020816024817f00000000000000000000000000000000000000000000000000000000000000006001600160a01b03165afa908115610f41578491611533575b501561151d576103e88082116115025750907f79c496f1c6c3df4a0a1cabbe2f47ff408d8d95d717af8a61c19b5c5bbfc67a5391600954908060095582519182526020820152a180f35b8491604493519263efd933ff60e01b84528301526024820152fd5b815163f829dd5160e01b81523381860152602490fd5b61154b915060203d8111610596576105888183612120565b856114b8565b5050346102075760203660031901126102075760209181906001600160a01b0361157961208e565b16815280845220549051908152f35b9190503461067457602036600319011261067457813567ffffffffffffffff928382116116e657366023830112156116e657818101359384116116e65736602485840101116116e65782516353aa430760e01b815233828201526020816024817f00000000000000000000000000000000000000000000000000000000000000006001600160a01b03165afa9081156116dc5786916116be575b50156116a7576008549060ff8216611699575060247f48bf5813314de41e6ad29b69491af3cc17d544ff8e92c7fa15548dea9e5ea8a293926001869360ff1916176008558284519442865280602087015285015201606083013783606084830101526060813394601f80199101168101030190a280f35b8351633f01e74960e11b8152fd5b60249083519063f829dd5160e01b82523390820152fd5b6116d6915060203d8111610596576105888183612120565b38611622565b84513d88823e3d90fd5b8480fd5b505034610207578160031936011261020757517f00000000000000000000000000000000000000000000000000000000000000006001600160a01b03168152602090f35b82843461125f57608036600319011261125f5750611258602092606435906044359060243590356128a8565b505034610207578160031936011261020757517f00000000000000000000000000000000000000000000000000000000000000006001600160a01b03168152602090f35b5050346102075781600319360112610207576020905160128152f35b5050346102075781600319360112610207576020906009549051908152f35b8383346102075760203660031901126102075780516353aa430760e01b815233818501528335906020816024817f00000000000000000000000000000000000000000000000000000000000000006001600160a01b03165afa908115610f4157849161188a575b501561151d576103e88082116115025750907f6859e91391e76973b032c6eca264badbd6db550ac3c40bca2db1c4c5e22c7aa891600a549080600a5582519182526020820152a180f35b6118a2915060203d8111610596576105888183612120565b85611840565b9050823461125f57606036600319011261125f576118c461208e565b6118cc6120a4565b916044359360018060a01b038316808352600160205286832033845260205286832054916000198303611908575b602088610be3898989612158565b86831061197057811561195957331561194257508252600160209081528683203384528152918690209085900390558290610be3876118fa565b8751634a1406b160e11b8152908101849052602490fd5b875163e602df0560e01b8152908101849052602490fd5b8751637dc7a0d960e11b8152339181019182526020820193909352604081018790528291506060010390fd5b9050823461125f57602036600319011261125f5781356119ba612532565b60ff60085416611b6b57808252600c602052838220600681019384549360ff8516611b545782546001600160a01b0392908316158015611b46575b611b2f57611aa06080938593611aa7937f9729558c6c01f8505a12c658949d638f038ca67a142de4834303f69ae11d78f497015499600160058701549360038160028a015416980154169a60ff1916179055506006548981158015611b25575b15611af957611a9a9150611a87600193611a7c611a73828795612595565b9d60065461233f565b60065560075461233f565b600755611a94838c612555565b9261233f565b90612568565b8097612588565b9486611aea575b85611adb575b875191825260208201528587820152846060820152a2600160055582519182526020820152f35b611ae58682612362565b611ab4565b611af48783612362565b611aae565b611a8783611a7c611a73611a9a9596611b1f611b16879983612555565b60075490612568565b95612595565b5060075415611a55565b875163158a49eb60e01b8152908101859052602490fd5b5082600185015416156119f5565b8651630bbf191d60e31b8152808301859052602490fd5b8351633f01e74960e11b81528390fd5b5050346102075781600319360112610207576020906007549051908152f35b5050346102075781600319360112610207576020906002549051908152f35b905034610674578160031936011261067457611bd361208e565b602435903315611c4c576001600160a01b0316918215611c3557508083602095338152600187528181208582528752205582519081527f8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b925843392a35160018152f35b8351634a1406b160e11b8152908101859052602490fd5b835163e602df0560e01b8152808401869052602490fd5b91905034610674578260031936011261067457805191836003549060019082821c928281168015611d4e575b6020958686108214611d3b5750848852908115611d195750600114611cc0575b6111938686611189828b0383612120565b929550600383527fc2575a0e9e593c00f959f8c92f12db2869c3395a3b0502d05e2516446f71f85b5b828410611d06575050508261119394611189928201019438611caf565b8054868501880152928601928101611ce9565b60ff191687860152505050151560051b83010192506111898261119338611caf565b634e487b7160e01b845260229052602483fd5b93607f1693611c8f565b505034610207578160031936011261020757602090600a549051908152f35b91905034610674576080366003190112610674578135611d9561207a565b91604435906064359460018060a01b0380871680970361207657611db7612532565b60ff6008541661206757825163acf0279f60e01b80825233848301526024916020917f000000000000000000000000000000000000000000000000000000000000000085169183818681865afa90811561205d578d91612040575b501561202b57871561201b578a8484928951948593849283528a8301525afa908115612011578b91611ff4575b5015611fdf57600c90878b52528389209160ff600684015416611fca578715611f1f57825416611f03575091838092611ef894611ebf7f9b51b2050826a9699829e99805cd54d7c243f4274134a53fe51eab09f2af8e529730337f00000000000000000000000000000000000000000000000000000000000000006123ae565b6001600160601b0360a01b33818354161782558a600283019182541617905501555b519415158552602085015233939081906040820190565b0390a4600160055580f35b6001915085604494519363e97e29b360e01b8552840152820152fd5b9192906001840192835416611fae575050918360058193611ef895611f877f9b51b2050826a9699829e99805cd54d7c243f4274134a53fe51eab09f2af8e529830337f00000000000000000000000000000000000000000000000000000000000000006123ae565b6001600160601b0360a01b9033828254161790558a60038301918254161790550155611ee1565b845163e97e29b360e01b81529182018790528101899052604490fd5b508351630bbf191d60e31b8152808401879052fd5b508351631c52846560e01b8152808401899052fd5b61200b9150823d8411610596576105888183612120565b38611e3f565b86513d8d823e3d90fd5b865163250d596f60e21b81528690fd5b8651631c52846560e01b815233818801528490fd5b6120579150843d8611610596576105888183612120565b38611e12565b88513d8f823e3d90fd5b509051633f01e74960e11b8152fd5b8780fd5b60243590811515820361208957565b600080fd5b600435906001600160a01b038216820361208957565b602435906001600160a01b038216820361208957565b6020808252825181830181905290939260005b8281106120ef57505060409293506000838284010152601f8019910116010190565b8181018601518482016040015285016120cd565b346120895760003660031901126120895760206040516103e88152f35b90601f8019910116810190811067ffffffffffffffff82111761214257604052565b634e487b7160e01b600052604160045260246000fd5b92916001600160a01b03918285169190821561230e5783169283156122f5576040805163acf0279f60e01b80825260048201869052602094937f000000000000000000000000000000000000000000000000000000000000000016918581602481865afa9081156122ea576000916122cd575b50156122b5578490602484518094819382528a60048301525afa9081156122aa5760009161228d575b50156122765783600052600083528060002054968288106122475750818495969760008051602061291d8339815191529560005260008552038160002055856000528060002082815401905551908152a3565b905163391434e360e21b81526001600160a01b0390911660048201526024810187905260448101829052606490fd5b51631c52846560e01b815260048101859052602490fd5b6122a49150843d8611610596576105888183612120565b386121f4565b82513d6000823e3d90fd5b8251631c52846560e01b815260048101879052602490fd5b6122e49150863d8811610596576105888183612120565b386121cb565b84513d6000823e3d90fd5b60405163ec442f0560e01b815260006004820152602490fd5b604051634b637e8f60e11b815260006004820152602490fd5b90816020910312612089575180151581036120895790565b9190820180921161234c57565b634e487b7160e01b600052601160045260246000fd5b6001600160a01b03169081156122f55760008051602061291d83398151915260208261239260009460025461233f565b60025584845283825260408420818154019055604051908152a3565b6040516323b872dd60e01b60208201526001600160a01b03928316602482015292909116604483015260648083019390935291815260a081019181831067ffffffffffffffff8411176121425761240792604052612409565b565b60018060a01b031690600080826020829451910182865af13d156124c2573d67ffffffffffffffff81116124ae57604051612465939291612454601f8201601f191660200183612120565b8152809260203d92013e5b836124cf565b8051908115159182612493575b505061247b5750565b60249060405190635274afe760e01b82526004820152fd5b6124a69250602080918301019101612327565b153880612472565b634e487b7160e01b83526041600452602483fd5b612465915060609061245f565b906124f657508051156124e457805190602001fd5b604051630a12f52160e11b8152600490fd5b81511580612529575b612507575090565b604051639996b31560e01b81526001600160a01b039091166004820152602490fd5b50803b156124ff565b600260055414612543576002600555565b604051633ee5aeb560e01b8152600490fd5b8181029291811591840414171561234c57565b8115612572570490565b634e487b7160e01b600052601260045260246000fd5b9190820391821161234c57565b6002549291908361261b576125b2916125ad91612555565b61265b565b6103e89081811115612609576103e719810190811161234c579281810180911161234c57600255600060008051602061291d833981519152602061dead9384845283825260408420818154019055604051908152a3565b604051631ed5ab6160e31b8152600490fd5b8361263a612631612640949596611b1694612555565b60065490612568565b93612555565b808210156126535750905b811561260957565b90509061264b565b801561279857612726816000908360801c8061278c575b508060401c8061277f575b508060201c80612772575b508060101c80612765575b508060081c80612758575b508060041c8061274b575b508060021c8061273e575b50600191828092811c612737575b1c1b6126ce8185612568565b01811c6126db8185612568565b01811c6126e88185612568565b01811c6126f58185612568565b01811c6127028185612568565b01811c61270f8185612568565b01811c61271c8185612568565b01901c8092612568565b80821015612732575090565b905090565b01816126c2565b60029150910190386126b4565b60049150910190386126a9565b600891509101903861269e565b6010915091019038612693565b6020915091019038612688565b604091509101903861267d565b91505060809038612672565b50600090565b60405163a9059cbb60e01b60208201526001600160a01b039092166024830152604480830193909352918152608081019167ffffffffffffffff8311828410176121425761240792604052612409565b906001600160a01b03821690811561230e576000928284528360205260408420549082821061284957508160008051602061291d833981519152926020928587528684520360408620558060025403600255604051908152a3565b60405163391434e360e21b81526001600160a01b03919091166004820152602481019190915260448101829052606490fd5b81831015612609578261289461289a94611a9a93612555565b92612588565b6001810180911161234c5790565b9290919280158015612914575b801561290c575b612903576127109182039082821161234c576128e2916128db91612555565b9384612555565b9181810291818304149015171561234c5761290092611a9a9161233f565b90565b50505050600090565b5083156128bc565b5082156128b556feddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3efa26469706673582212206facc42275982a919e1d0dd2d1a2da388be2d44c194a35721275aaeae44ff96764736f6c63430008140033",
}

// AutomatedMarketMakerABI is the input ABI used to generate the binding from.
// Deprecated: Use AutomatedMarketMakerMetaData.ABI instead.
var AutomatedMarketMakerABI = AutomatedMarketMakerMetaData.ABI

// AutomatedMarketMakerBin is the compiled bytecode used for deploying new contracts.
// Deprecated: Use AutomatedMarketMakerMetaData.Bin instead.
var AutomatedMarketMakerBin = AutomatedMarketMakerMetaData.Bin

// DeployAutomatedMarketMaker deploys a new Ethereum contract, binding an instance of AutomatedMarketMaker to it.
func DeployAutomatedMarketMaker(auth *bind.TransactOpts, backend bind.ContractBackend, _tokenA common.Address, _tokenB common.Address, _identityRegistry common.Address) (common.Address, *types.Transaction, *AutomatedMarketMaker, error) {
	parsed, err := AutomatedMarketMakerMetaData.GetAbi()
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	if parsed == nil {
		return common.Address{}, nil, nil, errors.New("GetABI returned nil")
	}

	address, tx, contract, err := bind.DeployContract(auth, *parsed, common.FromHex(AutomatedMarketMakerBin), backend, _tokenA, _tokenB, _identityRegistry)
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	return address, tx, &AutomatedMarketMaker{AutomatedMarketMakerCaller: AutomatedMarketMakerCaller{contract: contract}, AutomatedMarketMakerTransactor: AutomatedMarketMakerTransactor{contract: contract}, AutomatedMarketMakerFilterer: AutomatedMarketMakerFilterer{contract: contract}}, nil
}

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

// BURNADDRESS is a free data retrieval call binding the contract method 0xfccc2813.
//
// Solidity: function BURN_ADDRESS() view returns(address)
func (_AutomatedMarketMaker *AutomatedMarketMakerCaller) BURNADDRESS(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _AutomatedMarketMaker.contract.Call(opts, &out, "BURN_ADDRESS")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// BURNADDRESS is a free data retrieval call binding the contract method 0xfccc2813.
//
// Solidity: function BURN_ADDRESS() view returns(address)
func (_AutomatedMarketMaker *AutomatedMarketMakerSession) BURNADDRESS() (common.Address, error) {
	return _AutomatedMarketMaker.Contract.BURNADDRESS(&_AutomatedMarketMaker.CallOpts)
}

// BURNADDRESS is a free data retrieval call binding the contract method 0xfccc2813.
//
// Solidity: function BURN_ADDRESS() view returns(address)
func (_AutomatedMarketMaker *AutomatedMarketMakerCallerSession) BURNADDRESS() (common.Address, error) {
	return _AutomatedMarketMaker.Contract.BURNADDRESS(&_AutomatedMarketMaker.CallOpts)
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

// MINIMUMLIQUIDITY is a free data retrieval call binding the contract method 0xba9a7a56.
//
// Solidity: function MINIMUM_LIQUIDITY() view returns(uint256)
func (_AutomatedMarketMaker *AutomatedMarketMakerCaller) MINIMUMLIQUIDITY(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _AutomatedMarketMaker.contract.Call(opts, &out, "MINIMUM_LIQUIDITY")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// MINIMUMLIQUIDITY is a free data retrieval call binding the contract method 0xba9a7a56.
//
// Solidity: function MINIMUM_LIQUIDITY() view returns(uint256)
func (_AutomatedMarketMaker *AutomatedMarketMakerSession) MINIMUMLIQUIDITY() (*big.Int, error) {
	return _AutomatedMarketMaker.Contract.MINIMUMLIQUIDITY(&_AutomatedMarketMaker.CallOpts)
}

// MINIMUMLIQUIDITY is a free data retrieval call binding the contract method 0xba9a7a56.
//
// Solidity: function MINIMUM_LIQUIDITY() view returns(uint256)
func (_AutomatedMarketMaker *AutomatedMarketMakerCallerSession) MINIMUMLIQUIDITY() (*big.Int, error) {
	return _AutomatedMarketMaker.Contract.MINIMUMLIQUIDITY(&_AutomatedMarketMaker.CallOpts)
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

// Allowance is a free data retrieval call binding the contract method 0xdd62ed3e.
//
// Solidity: function allowance(address owner, address spender) view returns(uint256)
func (_AutomatedMarketMaker *AutomatedMarketMakerCaller) Allowance(opts *bind.CallOpts, owner common.Address, spender common.Address) (*big.Int, error) {
	var out []interface{}
	err := _AutomatedMarketMaker.contract.Call(opts, &out, "allowance", owner, spender)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// Allowance is a free data retrieval call binding the contract method 0xdd62ed3e.
//
// Solidity: function allowance(address owner, address spender) view returns(uint256)
func (_AutomatedMarketMaker *AutomatedMarketMakerSession) Allowance(owner common.Address, spender common.Address) (*big.Int, error) {
	return _AutomatedMarketMaker.Contract.Allowance(&_AutomatedMarketMaker.CallOpts, owner, spender)
}

// Allowance is a free data retrieval call binding the contract method 0xdd62ed3e.
//
// Solidity: function allowance(address owner, address spender) view returns(uint256)
func (_AutomatedMarketMaker *AutomatedMarketMakerCallerSession) Allowance(owner common.Address, spender common.Address) (*big.Int, error) {
	return _AutomatedMarketMaker.Contract.Allowance(&_AutomatedMarketMaker.CallOpts, owner, spender)
}

// BalanceOf is a free data retrieval call binding the contract method 0x70a08231.
//
// Solidity: function balanceOf(address account) view returns(uint256)
func (_AutomatedMarketMaker *AutomatedMarketMakerCaller) BalanceOf(opts *bind.CallOpts, account common.Address) (*big.Int, error) {
	var out []interface{}
	err := _AutomatedMarketMaker.contract.Call(opts, &out, "balanceOf", account)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// BalanceOf is a free data retrieval call binding the contract method 0x70a08231.
//
// Solidity: function balanceOf(address account) view returns(uint256)
func (_AutomatedMarketMaker *AutomatedMarketMakerSession) BalanceOf(account common.Address) (*big.Int, error) {
	return _AutomatedMarketMaker.Contract.BalanceOf(&_AutomatedMarketMaker.CallOpts, account)
}

// BalanceOf is a free data retrieval call binding the contract method 0x70a08231.
//
// Solidity: function balanceOf(address account) view returns(uint256)
func (_AutomatedMarketMaker *AutomatedMarketMakerCallerSession) BalanceOf(account common.Address) (*big.Int, error) {
	return _AutomatedMarketMaker.Contract.BalanceOf(&_AutomatedMarketMaker.CallOpts, account)
}

// Decimals is a free data retrieval call binding the contract method 0x313ce567.
//
// Solidity: function decimals() view returns(uint8)
func (_AutomatedMarketMaker *AutomatedMarketMakerCaller) Decimals(opts *bind.CallOpts) (uint8, error) {
	var out []interface{}
	err := _AutomatedMarketMaker.contract.Call(opts, &out, "decimals")

	if err != nil {
		return *new(uint8), err
	}

	out0 := *abi.ConvertType(out[0], new(uint8)).(*uint8)

	return out0, err

}

// Decimals is a free data retrieval call binding the contract method 0x313ce567.
//
// Solidity: function decimals() view returns(uint8)
func (_AutomatedMarketMaker *AutomatedMarketMakerSession) Decimals() (uint8, error) {
	return _AutomatedMarketMaker.Contract.Decimals(&_AutomatedMarketMaker.CallOpts)
}

// Decimals is a free data retrieval call binding the contract method 0x313ce567.
//
// Solidity: function decimals() view returns(uint8)
func (_AutomatedMarketMaker *AutomatedMarketMakerCallerSession) Decimals() (uint8, error) {
	return _AutomatedMarketMaker.Contract.Decimals(&_AutomatedMarketMaker.CallOpts)
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

// GetAmountOut is a free data retrieval call binding the contract method 0x52707d8c.
//
// Solidity: function getAmountOut(uint256 amountIn, uint256 reserveIn, uint256 reserveOut, uint256 feeBps_) pure returns(uint256 amountOut)
func (_AutomatedMarketMaker *AutomatedMarketMakerCaller) GetAmountOut(opts *bind.CallOpts, amountIn *big.Int, reserveIn *big.Int, reserveOut *big.Int, feeBps_ *big.Int) (*big.Int, error) {
	var out []interface{}
	err := _AutomatedMarketMaker.contract.Call(opts, &out, "getAmountOut", amountIn, reserveIn, reserveOut, feeBps_)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// GetAmountOut is a free data retrieval call binding the contract method 0x52707d8c.
//
// Solidity: function getAmountOut(uint256 amountIn, uint256 reserveIn, uint256 reserveOut, uint256 feeBps_) pure returns(uint256 amountOut)
func (_AutomatedMarketMaker *AutomatedMarketMakerSession) GetAmountOut(amountIn *big.Int, reserveIn *big.Int, reserveOut *big.Int, feeBps_ *big.Int) (*big.Int, error) {
	return _AutomatedMarketMaker.Contract.GetAmountOut(&_AutomatedMarketMaker.CallOpts, amountIn, reserveIn, reserveOut, feeBps_)
}

// GetAmountOut is a free data retrieval call binding the contract method 0x52707d8c.
//
// Solidity: function getAmountOut(uint256 amountIn, uint256 reserveIn, uint256 reserveOut, uint256 feeBps_) pure returns(uint256 amountOut)
func (_AutomatedMarketMaker *AutomatedMarketMakerCallerSession) GetAmountOut(amountIn *big.Int, reserveIn *big.Int, reserveOut *big.Int, feeBps_ *big.Int) (*big.Int, error) {
	return _AutomatedMarketMaker.Contract.GetAmountOut(&_AutomatedMarketMaker.CallOpts, amountIn, reserveIn, reserveOut, feeBps_)
}

// GetEscrow is a free data retrieval call binding the contract method 0xf023b811.
//
// Solidity: function getEscrow(bytes32 commitId) view returns(address depositorA, address depositorB, address recipientA, address recipientB, uint256 amountA, uint256 amountB, bool finalized)
func (_AutomatedMarketMaker *AutomatedMarketMakerCaller) GetEscrow(opts *bind.CallOpts, commitId [32]byte) (struct {
	DepositorA common.Address
	DepositorB common.Address
	RecipientA common.Address
	RecipientB common.Address
	AmountA    *big.Int
	AmountB    *big.Int
	Finalized  bool
}, error) {
	var out []interface{}
	err := _AutomatedMarketMaker.contract.Call(opts, &out, "getEscrow", commitId)

	outstruct := new(struct {
		DepositorA common.Address
		DepositorB common.Address
		RecipientA common.Address
		RecipientB common.Address
		AmountA    *big.Int
		AmountB    *big.Int
		Finalized  bool
	})
	if err != nil {
		return *outstruct, err
	}

	outstruct.DepositorA = *abi.ConvertType(out[0], new(common.Address)).(*common.Address)
	outstruct.DepositorB = *abi.ConvertType(out[1], new(common.Address)).(*common.Address)
	outstruct.RecipientA = *abi.ConvertType(out[2], new(common.Address)).(*common.Address)
	outstruct.RecipientB = *abi.ConvertType(out[3], new(common.Address)).(*common.Address)
	outstruct.AmountA = *abi.ConvertType(out[4], new(*big.Int)).(**big.Int)
	outstruct.AmountB = *abi.ConvertType(out[5], new(*big.Int)).(**big.Int)
	outstruct.Finalized = *abi.ConvertType(out[6], new(bool)).(*bool)

	return *outstruct, err

}

// GetEscrow is a free data retrieval call binding the contract method 0xf023b811.
//
// Solidity: function getEscrow(bytes32 commitId) view returns(address depositorA, address depositorB, address recipientA, address recipientB, uint256 amountA, uint256 amountB, bool finalized)
func (_AutomatedMarketMaker *AutomatedMarketMakerSession) GetEscrow(commitId [32]byte) (struct {
	DepositorA common.Address
	DepositorB common.Address
	RecipientA common.Address
	RecipientB common.Address
	AmountA    *big.Int
	AmountB    *big.Int
	Finalized  bool
}, error) {
	return _AutomatedMarketMaker.Contract.GetEscrow(&_AutomatedMarketMaker.CallOpts, commitId)
}

// GetEscrow is a free data retrieval call binding the contract method 0xf023b811.
//
// Solidity: function getEscrow(bytes32 commitId) view returns(address depositorA, address depositorB, address recipientA, address recipientB, uint256 amountA, uint256 amountB, bool finalized)
func (_AutomatedMarketMaker *AutomatedMarketMakerCallerSession) GetEscrow(commitId [32]byte) (struct {
	DepositorA common.Address
	DepositorB common.Address
	RecipientA common.Address
	RecipientB common.Address
	AmountA    *big.Int
	AmountB    *big.Int
	Finalized  bool
}, error) {
	return _AutomatedMarketMaker.Contract.GetEscrow(&_AutomatedMarketMaker.CallOpts, commitId)
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

// Name is a free data retrieval call binding the contract method 0x06fdde03.
//
// Solidity: function name() view returns(string)
func (_AutomatedMarketMaker *AutomatedMarketMakerCaller) Name(opts *bind.CallOpts) (string, error) {
	var out []interface{}
	err := _AutomatedMarketMaker.contract.Call(opts, &out, "name")

	if err != nil {
		return *new(string), err
	}

	out0 := *abi.ConvertType(out[0], new(string)).(*string)

	return out0, err

}

// Name is a free data retrieval call binding the contract method 0x06fdde03.
//
// Solidity: function name() view returns(string)
func (_AutomatedMarketMaker *AutomatedMarketMakerSession) Name() (string, error) {
	return _AutomatedMarketMaker.Contract.Name(&_AutomatedMarketMaker.CallOpts)
}

// Name is a free data retrieval call binding the contract method 0x06fdde03.
//
// Solidity: function name() view returns(string)
func (_AutomatedMarketMaker *AutomatedMarketMakerCallerSession) Name() (string, error) {
	return _AutomatedMarketMaker.Contract.Name(&_AutomatedMarketMaker.CallOpts)
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

// Symbol is a free data retrieval call binding the contract method 0x95d89b41.
//
// Solidity: function symbol() view returns(string)
func (_AutomatedMarketMaker *AutomatedMarketMakerCaller) Symbol(opts *bind.CallOpts) (string, error) {
	var out []interface{}
	err := _AutomatedMarketMaker.contract.Call(opts, &out, "symbol")

	if err != nil {
		return *new(string), err
	}

	out0 := *abi.ConvertType(out[0], new(string)).(*string)

	return out0, err

}

// Symbol is a free data retrieval call binding the contract method 0x95d89b41.
//
// Solidity: function symbol() view returns(string)
func (_AutomatedMarketMaker *AutomatedMarketMakerSession) Symbol() (string, error) {
	return _AutomatedMarketMaker.Contract.Symbol(&_AutomatedMarketMaker.CallOpts)
}

// Symbol is a free data retrieval call binding the contract method 0x95d89b41.
//
// Solidity: function symbol() view returns(string)
func (_AutomatedMarketMaker *AutomatedMarketMakerCallerSession) Symbol() (string, error) {
	return _AutomatedMarketMaker.Contract.Symbol(&_AutomatedMarketMaker.CallOpts)
}

// TotalSupply is a free data retrieval call binding the contract method 0x18160ddd.
//
// Solidity: function totalSupply() view returns(uint256)
func (_AutomatedMarketMaker *AutomatedMarketMakerCaller) TotalSupply(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _AutomatedMarketMaker.contract.Call(opts, &out, "totalSupply")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// TotalSupply is a free data retrieval call binding the contract method 0x18160ddd.
//
// Solidity: function totalSupply() view returns(uint256)
func (_AutomatedMarketMaker *AutomatedMarketMakerSession) TotalSupply() (*big.Int, error) {
	return _AutomatedMarketMaker.Contract.TotalSupply(&_AutomatedMarketMaker.CallOpts)
}

// TotalSupply is a free data retrieval call binding the contract method 0x18160ddd.
//
// Solidity: function totalSupply() view returns(uint256)
func (_AutomatedMarketMaker *AutomatedMarketMakerCallerSession) TotalSupply() (*big.Int, error) {
	return _AutomatedMarketMaker.Contract.TotalSupply(&_AutomatedMarketMaker.CallOpts)
}

// WithdrawalFeeBps is a free data retrieval call binding the contract method 0x04336bb3.
//
// Solidity: function withdrawalFeeBps() view returns(uint256)
func (_AutomatedMarketMaker *AutomatedMarketMakerCaller) WithdrawalFeeBps(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _AutomatedMarketMaker.contract.Call(opts, &out, "withdrawalFeeBps")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// WithdrawalFeeBps is a free data retrieval call binding the contract method 0x04336bb3.
//
// Solidity: function withdrawalFeeBps() view returns(uint256)
func (_AutomatedMarketMaker *AutomatedMarketMakerSession) WithdrawalFeeBps() (*big.Int, error) {
	return _AutomatedMarketMaker.Contract.WithdrawalFeeBps(&_AutomatedMarketMaker.CallOpts)
}

// WithdrawalFeeBps is a free data retrieval call binding the contract method 0x04336bb3.
//
// Solidity: function withdrawalFeeBps() view returns(uint256)
func (_AutomatedMarketMaker *AutomatedMarketMakerCallerSession) WithdrawalFeeBps() (*big.Int, error) {
	return _AutomatedMarketMaker.Contract.WithdrawalFeeBps(&_AutomatedMarketMaker.CallOpts)
}

// AddLiquidity is a paid mutator transaction binding the contract method 0x9cd441da.
//
// Solidity: function addLiquidity(uint256 amountA, uint256 amountB) returns(uint256 shares)
func (_AutomatedMarketMaker *AutomatedMarketMakerTransactor) AddLiquidity(opts *bind.TransactOpts, amountA *big.Int, amountB *big.Int) (*types.Transaction, error) {
	return _AutomatedMarketMaker.contract.Transact(opts, "addLiquidity", amountA, amountB)
}

// AddLiquidity is a paid mutator transaction binding the contract method 0x9cd441da.
//
// Solidity: function addLiquidity(uint256 amountA, uint256 amountB) returns(uint256 shares)
func (_AutomatedMarketMaker *AutomatedMarketMakerSession) AddLiquidity(amountA *big.Int, amountB *big.Int) (*types.Transaction, error) {
	return _AutomatedMarketMaker.Contract.AddLiquidity(&_AutomatedMarketMaker.TransactOpts, amountA, amountB)
}

// AddLiquidity is a paid mutator transaction binding the contract method 0x9cd441da.
//
// Solidity: function addLiquidity(uint256 amountA, uint256 amountB) returns(uint256 shares)
func (_AutomatedMarketMaker *AutomatedMarketMakerTransactorSession) AddLiquidity(amountA *big.Int, amountB *big.Int) (*types.Transaction, error) {
	return _AutomatedMarketMaker.Contract.AddLiquidity(&_AutomatedMarketMaker.TransactOpts, amountA, amountB)
}

// Approve is a paid mutator transaction binding the contract method 0x095ea7b3.
//
// Solidity: function approve(address spender, uint256 value) returns(bool)
func (_AutomatedMarketMaker *AutomatedMarketMakerTransactor) Approve(opts *bind.TransactOpts, spender common.Address, value *big.Int) (*types.Transaction, error) {
	return _AutomatedMarketMaker.contract.Transact(opts, "approve", spender, value)
}

// Approve is a paid mutator transaction binding the contract method 0x095ea7b3.
//
// Solidity: function approve(address spender, uint256 value) returns(bool)
func (_AutomatedMarketMaker *AutomatedMarketMakerSession) Approve(spender common.Address, value *big.Int) (*types.Transaction, error) {
	return _AutomatedMarketMaker.Contract.Approve(&_AutomatedMarketMaker.TransactOpts, spender, value)
}

// Approve is a paid mutator transaction binding the contract method 0x095ea7b3.
//
// Solidity: function approve(address spender, uint256 value) returns(bool)
func (_AutomatedMarketMaker *AutomatedMarketMakerTransactorSession) Approve(spender common.Address, value *big.Int) (*types.Transaction, error) {
	return _AutomatedMarketMaker.Contract.Approve(&_AutomatedMarketMaker.TransactOpts, spender, value)
}

// CancelCommitDeposit is a paid mutator transaction binding the contract method 0xa054816f.
//
// Solidity: function cancelCommitDeposit(bytes32 commitId, bool isTokenA) returns(uint256 amount)
func (_AutomatedMarketMaker *AutomatedMarketMakerTransactor) CancelCommitDeposit(opts *bind.TransactOpts, commitId [32]byte, isTokenA bool) (*types.Transaction, error) {
	return _AutomatedMarketMaker.contract.Transact(opts, "cancelCommitDeposit", commitId, isTokenA)
}

// CancelCommitDeposit is a paid mutator transaction binding the contract method 0xa054816f.
//
// Solidity: function cancelCommitDeposit(bytes32 commitId, bool isTokenA) returns(uint256 amount)
func (_AutomatedMarketMaker *AutomatedMarketMakerSession) CancelCommitDeposit(commitId [32]byte, isTokenA bool) (*types.Transaction, error) {
	return _AutomatedMarketMaker.Contract.CancelCommitDeposit(&_AutomatedMarketMaker.TransactOpts, commitId, isTokenA)
}

// CancelCommitDeposit is a paid mutator transaction binding the contract method 0xa054816f.
//
// Solidity: function cancelCommitDeposit(bytes32 commitId, bool isTokenA) returns(uint256 amount)
func (_AutomatedMarketMaker *AutomatedMarketMakerTransactorSession) CancelCommitDeposit(commitId [32]byte, isTokenA bool) (*types.Transaction, error) {
	return _AutomatedMarketMaker.Contract.CancelCommitDeposit(&_AutomatedMarketMaker.TransactOpts, commitId, isTokenA)
}

// DepositForCommit is a paid mutator transaction binding the contract method 0x01c32131.
//
// Solidity: function depositForCommit(bytes32 commitId, bool isTokenA, uint256 amount, address shareRecipient) returns()
func (_AutomatedMarketMaker *AutomatedMarketMakerTransactor) DepositForCommit(opts *bind.TransactOpts, commitId [32]byte, isTokenA bool, amount *big.Int, shareRecipient common.Address) (*types.Transaction, error) {
	return _AutomatedMarketMaker.contract.Transact(opts, "depositForCommit", commitId, isTokenA, amount, shareRecipient)
}

// DepositForCommit is a paid mutator transaction binding the contract method 0x01c32131.
//
// Solidity: function depositForCommit(bytes32 commitId, bool isTokenA, uint256 amount, address shareRecipient) returns()
func (_AutomatedMarketMaker *AutomatedMarketMakerSession) DepositForCommit(commitId [32]byte, isTokenA bool, amount *big.Int, shareRecipient common.Address) (*types.Transaction, error) {
	return _AutomatedMarketMaker.Contract.DepositForCommit(&_AutomatedMarketMaker.TransactOpts, commitId, isTokenA, amount, shareRecipient)
}

// DepositForCommit is a paid mutator transaction binding the contract method 0x01c32131.
//
// Solidity: function depositForCommit(bytes32 commitId, bool isTokenA, uint256 amount, address shareRecipient) returns()
func (_AutomatedMarketMaker *AutomatedMarketMakerTransactorSession) DepositForCommit(commitId [32]byte, isTokenA bool, amount *big.Int, shareRecipient common.Address) (*types.Transaction, error) {
	return _AutomatedMarketMaker.Contract.DepositForCommit(&_AutomatedMarketMaker.TransactOpts, commitId, isTokenA, amount, shareRecipient)
}

// FinalizeCommit is a paid mutator transaction binding the contract method 0x1e9b551d.
//
// Solidity: function finalizeCommit(bytes32 commitId) returns(uint256 sharesA, uint256 sharesB)
func (_AutomatedMarketMaker *AutomatedMarketMakerTransactor) FinalizeCommit(opts *bind.TransactOpts, commitId [32]byte) (*types.Transaction, error) {
	return _AutomatedMarketMaker.contract.Transact(opts, "finalizeCommit", commitId)
}

// FinalizeCommit is a paid mutator transaction binding the contract method 0x1e9b551d.
//
// Solidity: function finalizeCommit(bytes32 commitId) returns(uint256 sharesA, uint256 sharesB)
func (_AutomatedMarketMaker *AutomatedMarketMakerSession) FinalizeCommit(commitId [32]byte) (*types.Transaction, error) {
	return _AutomatedMarketMaker.Contract.FinalizeCommit(&_AutomatedMarketMaker.TransactOpts, commitId)
}

// FinalizeCommit is a paid mutator transaction binding the contract method 0x1e9b551d.
//
// Solidity: function finalizeCommit(bytes32 commitId) returns(uint256 sharesA, uint256 sharesB)
func (_AutomatedMarketMaker *AutomatedMarketMakerTransactorSession) FinalizeCommit(commitId [32]byte) (*types.Transaction, error) {
	return _AutomatedMarketMaker.Contract.FinalizeCommit(&_AutomatedMarketMaker.TransactOpts, commitId)
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

// RemoveLiquidity is a paid mutator transaction binding the contract method 0xc4ccdeea.
//
// Solidity: function removeLiquidity(uint256 shares, address tokenOut, uint256 minAmountOut) returns(uint256 amountOut)
func (_AutomatedMarketMaker *AutomatedMarketMakerTransactor) RemoveLiquidity(opts *bind.TransactOpts, shares *big.Int, tokenOut common.Address, minAmountOut *big.Int) (*types.Transaction, error) {
	return _AutomatedMarketMaker.contract.Transact(opts, "removeLiquidity", shares, tokenOut, minAmountOut)
}

// RemoveLiquidity is a paid mutator transaction binding the contract method 0xc4ccdeea.
//
// Solidity: function removeLiquidity(uint256 shares, address tokenOut, uint256 minAmountOut) returns(uint256 amountOut)
func (_AutomatedMarketMaker *AutomatedMarketMakerSession) RemoveLiquidity(shares *big.Int, tokenOut common.Address, minAmountOut *big.Int) (*types.Transaction, error) {
	return _AutomatedMarketMaker.Contract.RemoveLiquidity(&_AutomatedMarketMaker.TransactOpts, shares, tokenOut, minAmountOut)
}

// RemoveLiquidity is a paid mutator transaction binding the contract method 0xc4ccdeea.
//
// Solidity: function removeLiquidity(uint256 shares, address tokenOut, uint256 minAmountOut) returns(uint256 amountOut)
func (_AutomatedMarketMaker *AutomatedMarketMakerTransactorSession) RemoveLiquidity(shares *big.Int, tokenOut common.Address, minAmountOut *big.Int) (*types.Transaction, error) {
	return _AutomatedMarketMaker.Contract.RemoveLiquidity(&_AutomatedMarketMaker.TransactOpts, shares, tokenOut, minAmountOut)
}

// RemoveLiquidityEmergency is a paid mutator transaction binding the contract method 0xc9754c38.
//
// Solidity: function removeLiquidityEmergency(uint256 shares) returns(uint256 amountA, uint256 amountB)
func (_AutomatedMarketMaker *AutomatedMarketMakerTransactor) RemoveLiquidityEmergency(opts *bind.TransactOpts, shares *big.Int) (*types.Transaction, error) {
	return _AutomatedMarketMaker.contract.Transact(opts, "removeLiquidityEmergency", shares)
}

// RemoveLiquidityEmergency is a paid mutator transaction binding the contract method 0xc9754c38.
//
// Solidity: function removeLiquidityEmergency(uint256 shares) returns(uint256 amountA, uint256 amountB)
func (_AutomatedMarketMaker *AutomatedMarketMakerSession) RemoveLiquidityEmergency(shares *big.Int) (*types.Transaction, error) {
	return _AutomatedMarketMaker.Contract.RemoveLiquidityEmergency(&_AutomatedMarketMaker.TransactOpts, shares)
}

// RemoveLiquidityEmergency is a paid mutator transaction binding the contract method 0xc9754c38.
//
// Solidity: function removeLiquidityEmergency(uint256 shares) returns(uint256 amountA, uint256 amountB)
func (_AutomatedMarketMaker *AutomatedMarketMakerTransactorSession) RemoveLiquidityEmergency(shares *big.Int) (*types.Transaction, error) {
	return _AutomatedMarketMaker.Contract.RemoveLiquidityEmergency(&_AutomatedMarketMaker.TransactOpts, shares)
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

// SetWithdrawalFeeBps is a paid mutator transaction binding the contract method 0x24167524.
//
// Solidity: function setWithdrawalFeeBps(uint256 newWithdrawalFeeBps) returns()
func (_AutomatedMarketMaker *AutomatedMarketMakerTransactor) SetWithdrawalFeeBps(opts *bind.TransactOpts, newWithdrawalFeeBps *big.Int) (*types.Transaction, error) {
	return _AutomatedMarketMaker.contract.Transact(opts, "setWithdrawalFeeBps", newWithdrawalFeeBps)
}

// SetWithdrawalFeeBps is a paid mutator transaction binding the contract method 0x24167524.
//
// Solidity: function setWithdrawalFeeBps(uint256 newWithdrawalFeeBps) returns()
func (_AutomatedMarketMaker *AutomatedMarketMakerSession) SetWithdrawalFeeBps(newWithdrawalFeeBps *big.Int) (*types.Transaction, error) {
	return _AutomatedMarketMaker.Contract.SetWithdrawalFeeBps(&_AutomatedMarketMaker.TransactOpts, newWithdrawalFeeBps)
}

// SetWithdrawalFeeBps is a paid mutator transaction binding the contract method 0x24167524.
//
// Solidity: function setWithdrawalFeeBps(uint256 newWithdrawalFeeBps) returns()
func (_AutomatedMarketMaker *AutomatedMarketMakerTransactorSession) SetWithdrawalFeeBps(newWithdrawalFeeBps *big.Int) (*types.Transaction, error) {
	return _AutomatedMarketMaker.Contract.SetWithdrawalFeeBps(&_AutomatedMarketMaker.TransactOpts, newWithdrawalFeeBps)
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

// Transfer is a paid mutator transaction binding the contract method 0xa9059cbb.
//
// Solidity: function transfer(address to, uint256 value) returns(bool)
func (_AutomatedMarketMaker *AutomatedMarketMakerTransactor) Transfer(opts *bind.TransactOpts, to common.Address, value *big.Int) (*types.Transaction, error) {
	return _AutomatedMarketMaker.contract.Transact(opts, "transfer", to, value)
}

// Transfer is a paid mutator transaction binding the contract method 0xa9059cbb.
//
// Solidity: function transfer(address to, uint256 value) returns(bool)
func (_AutomatedMarketMaker *AutomatedMarketMakerSession) Transfer(to common.Address, value *big.Int) (*types.Transaction, error) {
	return _AutomatedMarketMaker.Contract.Transfer(&_AutomatedMarketMaker.TransactOpts, to, value)
}

// Transfer is a paid mutator transaction binding the contract method 0xa9059cbb.
//
// Solidity: function transfer(address to, uint256 value) returns(bool)
func (_AutomatedMarketMaker *AutomatedMarketMakerTransactorSession) Transfer(to common.Address, value *big.Int) (*types.Transaction, error) {
	return _AutomatedMarketMaker.Contract.Transfer(&_AutomatedMarketMaker.TransactOpts, to, value)
}

// TransferFrom is a paid mutator transaction binding the contract method 0x23b872dd.
//
// Solidity: function transferFrom(address from, address to, uint256 value) returns(bool)
func (_AutomatedMarketMaker *AutomatedMarketMakerTransactor) TransferFrom(opts *bind.TransactOpts, from common.Address, to common.Address, value *big.Int) (*types.Transaction, error) {
	return _AutomatedMarketMaker.contract.Transact(opts, "transferFrom", from, to, value)
}

// TransferFrom is a paid mutator transaction binding the contract method 0x23b872dd.
//
// Solidity: function transferFrom(address from, address to, uint256 value) returns(bool)
func (_AutomatedMarketMaker *AutomatedMarketMakerSession) TransferFrom(from common.Address, to common.Address, value *big.Int) (*types.Transaction, error) {
	return _AutomatedMarketMaker.Contract.TransferFrom(&_AutomatedMarketMaker.TransactOpts, from, to, value)
}

// TransferFrom is a paid mutator transaction binding the contract method 0x23b872dd.
//
// Solidity: function transferFrom(address from, address to, uint256 value) returns(bool)
func (_AutomatedMarketMaker *AutomatedMarketMakerTransactorSession) TransferFrom(from common.Address, to common.Address, value *big.Int) (*types.Transaction, error) {
	return _AutomatedMarketMaker.Contract.TransferFrom(&_AutomatedMarketMaker.TransactOpts, from, to, value)
}

// AutomatedMarketMakerApprovalIterator is returned from FilterApproval and is used to iterate over the raw logs and unpacked data for Approval events raised by the AutomatedMarketMaker contract.
type AutomatedMarketMakerApprovalIterator struct {
	Event *AutomatedMarketMakerApproval // Event containing the contract specifics and raw log

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
func (it *AutomatedMarketMakerApprovalIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(AutomatedMarketMakerApproval)
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
		it.Event = new(AutomatedMarketMakerApproval)
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
func (it *AutomatedMarketMakerApprovalIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *AutomatedMarketMakerApprovalIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// AutomatedMarketMakerApproval represents a Approval event raised by the AutomatedMarketMaker contract.
type AutomatedMarketMakerApproval struct {
	Owner   common.Address
	Spender common.Address
	Value   *big.Int
	Raw     types.Log // Blockchain specific contextual infos
}

// FilterApproval is a free log retrieval operation binding the contract event 0x8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b925.
//
// Solidity: event Approval(address indexed owner, address indexed spender, uint256 value)
func (_AutomatedMarketMaker *AutomatedMarketMakerFilterer) FilterApproval(opts *bind.FilterOpts, owner []common.Address, spender []common.Address) (*AutomatedMarketMakerApprovalIterator, error) {

	var ownerRule []interface{}
	for _, ownerItem := range owner {
		ownerRule = append(ownerRule, ownerItem)
	}
	var spenderRule []interface{}
	for _, spenderItem := range spender {
		spenderRule = append(spenderRule, spenderItem)
	}

	logs, sub, err := _AutomatedMarketMaker.contract.FilterLogs(opts, "Approval", ownerRule, spenderRule)
	if err != nil {
		return nil, err
	}
	return &AutomatedMarketMakerApprovalIterator{contract: _AutomatedMarketMaker.contract, event: "Approval", logs: logs, sub: sub}, nil
}

// WatchApproval is a free log subscription operation binding the contract event 0x8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b925.
//
// Solidity: event Approval(address indexed owner, address indexed spender, uint256 value)
func (_AutomatedMarketMaker *AutomatedMarketMakerFilterer) WatchApproval(opts *bind.WatchOpts, sink chan<- *AutomatedMarketMakerApproval, owner []common.Address, spender []common.Address) (event.Subscription, error) {

	var ownerRule []interface{}
	for _, ownerItem := range owner {
		ownerRule = append(ownerRule, ownerItem)
	}
	var spenderRule []interface{}
	for _, spenderItem := range spender {
		spenderRule = append(spenderRule, spenderItem)
	}

	logs, sub, err := _AutomatedMarketMaker.contract.WatchLogs(opts, "Approval", ownerRule, spenderRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(AutomatedMarketMakerApproval)
				if err := _AutomatedMarketMaker.contract.UnpackLog(event, "Approval", log); err != nil {
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
func (_AutomatedMarketMaker *AutomatedMarketMakerFilterer) ParseApproval(log types.Log) (*AutomatedMarketMakerApproval, error) {
	event := new(AutomatedMarketMakerApproval)
	if err := _AutomatedMarketMaker.contract.UnpackLog(event, "Approval", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
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

// AutomatedMarketMakerLogCommitDepositIterator is returned from FilterLogCommitDeposit and is used to iterate over the raw logs and unpacked data for LogCommitDeposit events raised by the AutomatedMarketMaker contract.
type AutomatedMarketMakerLogCommitDepositIterator struct {
	Event *AutomatedMarketMakerLogCommitDeposit // Event containing the contract specifics and raw log

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
func (it *AutomatedMarketMakerLogCommitDepositIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(AutomatedMarketMakerLogCommitDeposit)
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
		it.Event = new(AutomatedMarketMakerLogCommitDeposit)
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
func (it *AutomatedMarketMakerLogCommitDepositIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *AutomatedMarketMakerLogCommitDepositIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// AutomatedMarketMakerLogCommitDeposit represents a LogCommitDeposit event raised by the AutomatedMarketMaker contract.
type AutomatedMarketMakerLogCommitDeposit struct {
	CommitId  [32]byte
	Depositor common.Address
	Recipient common.Address
	IsTokenA  bool
	Amount    *big.Int
	Raw       types.Log // Blockchain specific contextual infos
}

// FilterLogCommitDeposit is a free log retrieval operation binding the contract event 0x9b51b2050826a9699829e99805cd54d7c243f4274134a53fe51eab09f2af8e52.
//
// Solidity: event LogCommitDeposit(bytes32 indexed commitId, address indexed depositor, address indexed recipient, bool isTokenA, uint256 amount)
func (_AutomatedMarketMaker *AutomatedMarketMakerFilterer) FilterLogCommitDeposit(opts *bind.FilterOpts, commitId [][32]byte, depositor []common.Address, recipient []common.Address) (*AutomatedMarketMakerLogCommitDepositIterator, error) {

	var commitIdRule []interface{}
	for _, commitIdItem := range commitId {
		commitIdRule = append(commitIdRule, commitIdItem)
	}
	var depositorRule []interface{}
	for _, depositorItem := range depositor {
		depositorRule = append(depositorRule, depositorItem)
	}
	var recipientRule []interface{}
	for _, recipientItem := range recipient {
		recipientRule = append(recipientRule, recipientItem)
	}

	logs, sub, err := _AutomatedMarketMaker.contract.FilterLogs(opts, "LogCommitDeposit", commitIdRule, depositorRule, recipientRule)
	if err != nil {
		return nil, err
	}
	return &AutomatedMarketMakerLogCommitDepositIterator{contract: _AutomatedMarketMaker.contract, event: "LogCommitDeposit", logs: logs, sub: sub}, nil
}

// WatchLogCommitDeposit is a free log subscription operation binding the contract event 0x9b51b2050826a9699829e99805cd54d7c243f4274134a53fe51eab09f2af8e52.
//
// Solidity: event LogCommitDeposit(bytes32 indexed commitId, address indexed depositor, address indexed recipient, bool isTokenA, uint256 amount)
func (_AutomatedMarketMaker *AutomatedMarketMakerFilterer) WatchLogCommitDeposit(opts *bind.WatchOpts, sink chan<- *AutomatedMarketMakerLogCommitDeposit, commitId [][32]byte, depositor []common.Address, recipient []common.Address) (event.Subscription, error) {

	var commitIdRule []interface{}
	for _, commitIdItem := range commitId {
		commitIdRule = append(commitIdRule, commitIdItem)
	}
	var depositorRule []interface{}
	for _, depositorItem := range depositor {
		depositorRule = append(depositorRule, depositorItem)
	}
	var recipientRule []interface{}
	for _, recipientItem := range recipient {
		recipientRule = append(recipientRule, recipientItem)
	}

	logs, sub, err := _AutomatedMarketMaker.contract.WatchLogs(opts, "LogCommitDeposit", commitIdRule, depositorRule, recipientRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(AutomatedMarketMakerLogCommitDeposit)
				if err := _AutomatedMarketMaker.contract.UnpackLog(event, "LogCommitDeposit", log); err != nil {
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

// ParseLogCommitDeposit is a log parse operation binding the contract event 0x9b51b2050826a9699829e99805cd54d7c243f4274134a53fe51eab09f2af8e52.
//
// Solidity: event LogCommitDeposit(bytes32 indexed commitId, address indexed depositor, address indexed recipient, bool isTokenA, uint256 amount)
func (_AutomatedMarketMaker *AutomatedMarketMakerFilterer) ParseLogCommitDeposit(log types.Log) (*AutomatedMarketMakerLogCommitDeposit, error) {
	event := new(AutomatedMarketMakerLogCommitDeposit)
	if err := _AutomatedMarketMaker.contract.UnpackLog(event, "LogCommitDeposit", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// AutomatedMarketMakerLogCommitFinalizedIterator is returned from FilterLogCommitFinalized and is used to iterate over the raw logs and unpacked data for LogCommitFinalized events raised by the AutomatedMarketMaker contract.
type AutomatedMarketMakerLogCommitFinalizedIterator struct {
	Event *AutomatedMarketMakerLogCommitFinalized // Event containing the contract specifics and raw log

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
func (it *AutomatedMarketMakerLogCommitFinalizedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(AutomatedMarketMakerLogCommitFinalized)
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
		it.Event = new(AutomatedMarketMakerLogCommitFinalized)
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
func (it *AutomatedMarketMakerLogCommitFinalizedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *AutomatedMarketMakerLogCommitFinalizedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// AutomatedMarketMakerLogCommitFinalized represents a LogCommitFinalized event raised by the AutomatedMarketMaker contract.
type AutomatedMarketMakerLogCommitFinalized struct {
	CommitId   [32]byte
	RecipientA common.Address
	RecipientB common.Address
	SharesA    *big.Int
	SharesB    *big.Int
	Raw        types.Log // Blockchain specific contextual infos
}

// FilterLogCommitFinalized is a free log retrieval operation binding the contract event 0x9729558c6c01f8505a12c658949d638f038ca67a142de4834303f69ae11d78f4.
//
// Solidity: event LogCommitFinalized(bytes32 indexed commitId, address recipientA, address recipientB, uint256 sharesA, uint256 sharesB)
func (_AutomatedMarketMaker *AutomatedMarketMakerFilterer) FilterLogCommitFinalized(opts *bind.FilterOpts, commitId [][32]byte) (*AutomatedMarketMakerLogCommitFinalizedIterator, error) {

	var commitIdRule []interface{}
	for _, commitIdItem := range commitId {
		commitIdRule = append(commitIdRule, commitIdItem)
	}

	logs, sub, err := _AutomatedMarketMaker.contract.FilterLogs(opts, "LogCommitFinalized", commitIdRule)
	if err != nil {
		return nil, err
	}
	return &AutomatedMarketMakerLogCommitFinalizedIterator{contract: _AutomatedMarketMaker.contract, event: "LogCommitFinalized", logs: logs, sub: sub}, nil
}

// WatchLogCommitFinalized is a free log subscription operation binding the contract event 0x9729558c6c01f8505a12c658949d638f038ca67a142de4834303f69ae11d78f4.
//
// Solidity: event LogCommitFinalized(bytes32 indexed commitId, address recipientA, address recipientB, uint256 sharesA, uint256 sharesB)
func (_AutomatedMarketMaker *AutomatedMarketMakerFilterer) WatchLogCommitFinalized(opts *bind.WatchOpts, sink chan<- *AutomatedMarketMakerLogCommitFinalized, commitId [][32]byte) (event.Subscription, error) {

	var commitIdRule []interface{}
	for _, commitIdItem := range commitId {
		commitIdRule = append(commitIdRule, commitIdItem)
	}

	logs, sub, err := _AutomatedMarketMaker.contract.WatchLogs(opts, "LogCommitFinalized", commitIdRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(AutomatedMarketMakerLogCommitFinalized)
				if err := _AutomatedMarketMaker.contract.UnpackLog(event, "LogCommitFinalized", log); err != nil {
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

// ParseLogCommitFinalized is a log parse operation binding the contract event 0x9729558c6c01f8505a12c658949d638f038ca67a142de4834303f69ae11d78f4.
//
// Solidity: event LogCommitFinalized(bytes32 indexed commitId, address recipientA, address recipientB, uint256 sharesA, uint256 sharesB)
func (_AutomatedMarketMaker *AutomatedMarketMakerFilterer) ParseLogCommitFinalized(log types.Log) (*AutomatedMarketMakerLogCommitFinalized, error) {
	event := new(AutomatedMarketMakerLogCommitFinalized)
	if err := _AutomatedMarketMaker.contract.UnpackLog(event, "LogCommitFinalized", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// AutomatedMarketMakerLogCommitRefundedIterator is returned from FilterLogCommitRefunded and is used to iterate over the raw logs and unpacked data for LogCommitRefunded events raised by the AutomatedMarketMaker contract.
type AutomatedMarketMakerLogCommitRefundedIterator struct {
	Event *AutomatedMarketMakerLogCommitRefunded // Event containing the contract specifics and raw log

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
func (it *AutomatedMarketMakerLogCommitRefundedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(AutomatedMarketMakerLogCommitRefunded)
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
		it.Event = new(AutomatedMarketMakerLogCommitRefunded)
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
func (it *AutomatedMarketMakerLogCommitRefundedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *AutomatedMarketMakerLogCommitRefundedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// AutomatedMarketMakerLogCommitRefunded represents a LogCommitRefunded event raised by the AutomatedMarketMaker contract.
type AutomatedMarketMakerLogCommitRefunded struct {
	CommitId  [32]byte
	Depositor common.Address
	IsTokenA  bool
	Amount    *big.Int
	Raw       types.Log // Blockchain specific contextual infos
}

// FilterLogCommitRefunded is a free log retrieval operation binding the contract event 0xf6c2017497779a6f9357b3771c9ee63b3310a27bbf53b8293a8a215ba4a701c4.
//
// Solidity: event LogCommitRefunded(bytes32 indexed commitId, address indexed depositor, bool isTokenA, uint256 amount)
func (_AutomatedMarketMaker *AutomatedMarketMakerFilterer) FilterLogCommitRefunded(opts *bind.FilterOpts, commitId [][32]byte, depositor []common.Address) (*AutomatedMarketMakerLogCommitRefundedIterator, error) {

	var commitIdRule []interface{}
	for _, commitIdItem := range commitId {
		commitIdRule = append(commitIdRule, commitIdItem)
	}
	var depositorRule []interface{}
	for _, depositorItem := range depositor {
		depositorRule = append(depositorRule, depositorItem)
	}

	logs, sub, err := _AutomatedMarketMaker.contract.FilterLogs(opts, "LogCommitRefunded", commitIdRule, depositorRule)
	if err != nil {
		return nil, err
	}
	return &AutomatedMarketMakerLogCommitRefundedIterator{contract: _AutomatedMarketMaker.contract, event: "LogCommitRefunded", logs: logs, sub: sub}, nil
}

// WatchLogCommitRefunded is a free log subscription operation binding the contract event 0xf6c2017497779a6f9357b3771c9ee63b3310a27bbf53b8293a8a215ba4a701c4.
//
// Solidity: event LogCommitRefunded(bytes32 indexed commitId, address indexed depositor, bool isTokenA, uint256 amount)
func (_AutomatedMarketMaker *AutomatedMarketMakerFilterer) WatchLogCommitRefunded(opts *bind.WatchOpts, sink chan<- *AutomatedMarketMakerLogCommitRefunded, commitId [][32]byte, depositor []common.Address) (event.Subscription, error) {

	var commitIdRule []interface{}
	for _, commitIdItem := range commitId {
		commitIdRule = append(commitIdRule, commitIdItem)
	}
	var depositorRule []interface{}
	for _, depositorItem := range depositor {
		depositorRule = append(depositorRule, depositorItem)
	}

	logs, sub, err := _AutomatedMarketMaker.contract.WatchLogs(opts, "LogCommitRefunded", commitIdRule, depositorRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(AutomatedMarketMakerLogCommitRefunded)
				if err := _AutomatedMarketMaker.contract.UnpackLog(event, "LogCommitRefunded", log); err != nil {
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

// ParseLogCommitRefunded is a log parse operation binding the contract event 0xf6c2017497779a6f9357b3771c9ee63b3310a27bbf53b8293a8a215ba4a701c4.
//
// Solidity: event LogCommitRefunded(bytes32 indexed commitId, address indexed depositor, bool isTokenA, uint256 amount)
func (_AutomatedMarketMaker *AutomatedMarketMakerFilterer) ParseLogCommitRefunded(log types.Log) (*AutomatedMarketMakerLogCommitRefunded, error) {
	event := new(AutomatedMarketMakerLogCommitRefunded)
	if err := _AutomatedMarketMaker.contract.UnpackLog(event, "LogCommitRefunded", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// AutomatedMarketMakerLogEmergencyWithdrawalIterator is returned from FilterLogEmergencyWithdrawal and is used to iterate over the raw logs and unpacked data for LogEmergencyWithdrawal events raised by the AutomatedMarketMaker contract.
type AutomatedMarketMakerLogEmergencyWithdrawalIterator struct {
	Event *AutomatedMarketMakerLogEmergencyWithdrawal // Event containing the contract specifics and raw log

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
func (it *AutomatedMarketMakerLogEmergencyWithdrawalIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(AutomatedMarketMakerLogEmergencyWithdrawal)
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
		it.Event = new(AutomatedMarketMakerLogEmergencyWithdrawal)
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
func (it *AutomatedMarketMakerLogEmergencyWithdrawalIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *AutomatedMarketMakerLogEmergencyWithdrawalIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// AutomatedMarketMakerLogEmergencyWithdrawal represents a LogEmergencyWithdrawal event raised by the AutomatedMarketMaker contract.
type AutomatedMarketMakerLogEmergencyWithdrawal struct {
	Provider     common.Address
	SharesBurned *big.Int
	AmountTokenA *big.Int
	AmountTokenB *big.Int
	Raw          types.Log // Blockchain specific contextual infos
}

// FilterLogEmergencyWithdrawal is a free log retrieval operation binding the contract event 0x945f4cfb97c69b809044088a94cd4f9a9bde62be502babb58bb8ebe059fcd29d.
//
// Solidity: event LogEmergencyWithdrawal(address indexed provider, uint256 sharesBurned, uint256 amountTokenA, uint256 amountTokenB)
func (_AutomatedMarketMaker *AutomatedMarketMakerFilterer) FilterLogEmergencyWithdrawal(opts *bind.FilterOpts, provider []common.Address) (*AutomatedMarketMakerLogEmergencyWithdrawalIterator, error) {

	var providerRule []interface{}
	for _, providerItem := range provider {
		providerRule = append(providerRule, providerItem)
	}

	logs, sub, err := _AutomatedMarketMaker.contract.FilterLogs(opts, "LogEmergencyWithdrawal", providerRule)
	if err != nil {
		return nil, err
	}
	return &AutomatedMarketMakerLogEmergencyWithdrawalIterator{contract: _AutomatedMarketMaker.contract, event: "LogEmergencyWithdrawal", logs: logs, sub: sub}, nil
}

// WatchLogEmergencyWithdrawal is a free log subscription operation binding the contract event 0x945f4cfb97c69b809044088a94cd4f9a9bde62be502babb58bb8ebe059fcd29d.
//
// Solidity: event LogEmergencyWithdrawal(address indexed provider, uint256 sharesBurned, uint256 amountTokenA, uint256 amountTokenB)
func (_AutomatedMarketMaker *AutomatedMarketMakerFilterer) WatchLogEmergencyWithdrawal(opts *bind.WatchOpts, sink chan<- *AutomatedMarketMakerLogEmergencyWithdrawal, provider []common.Address) (event.Subscription, error) {

	var providerRule []interface{}
	for _, providerItem := range provider {
		providerRule = append(providerRule, providerItem)
	}

	logs, sub, err := _AutomatedMarketMaker.contract.WatchLogs(opts, "LogEmergencyWithdrawal", providerRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(AutomatedMarketMakerLogEmergencyWithdrawal)
				if err := _AutomatedMarketMaker.contract.UnpackLog(event, "LogEmergencyWithdrawal", log); err != nil {
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

// ParseLogEmergencyWithdrawal is a log parse operation binding the contract event 0x945f4cfb97c69b809044088a94cd4f9a9bde62be502babb58bb8ebe059fcd29d.
//
// Solidity: event LogEmergencyWithdrawal(address indexed provider, uint256 sharesBurned, uint256 amountTokenA, uint256 amountTokenB)
func (_AutomatedMarketMaker *AutomatedMarketMakerFilterer) ParseLogEmergencyWithdrawal(log types.Log) (*AutomatedMarketMakerLogEmergencyWithdrawal, error) {
	event := new(AutomatedMarketMakerLogEmergencyWithdrawal)
	if err := _AutomatedMarketMaker.contract.UnpackLog(event, "LogEmergencyWithdrawal", log); err != nil {
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
	SharesMinted *big.Int
	Raw          types.Log // Blockchain specific contextual infos
}

// FilterLogLiquidityAdded is a free log retrieval operation binding the contract event 0x00f00467526ab75114da7a5ec65bb1b6755a6a23d97b65fe8fc1662c920c6044.
//
// Solidity: event LogLiquidityAdded(address indexed provider, uint256 amountTokenA, uint256 amountTokenB, uint256 sharesMinted)
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

// WatchLogLiquidityAdded is a free log subscription operation binding the contract event 0x00f00467526ab75114da7a5ec65bb1b6755a6a23d97b65fe8fc1662c920c6044.
//
// Solidity: event LogLiquidityAdded(address indexed provider, uint256 amountTokenA, uint256 amountTokenB, uint256 sharesMinted)
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

// ParseLogLiquidityAdded is a log parse operation binding the contract event 0x00f00467526ab75114da7a5ec65bb1b6755a6a23d97b65fe8fc1662c920c6044.
//
// Solidity: event LogLiquidityAdded(address indexed provider, uint256 amountTokenA, uint256 amountTokenB, uint256 sharesMinted)
func (_AutomatedMarketMaker *AutomatedMarketMakerFilterer) ParseLogLiquidityAdded(log types.Log) (*AutomatedMarketMakerLogLiquidityAdded, error) {
	event := new(AutomatedMarketMakerLogLiquidityAdded)
	if err := _AutomatedMarketMaker.contract.UnpackLog(event, "LogLiquidityAdded", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// AutomatedMarketMakerLogLiquidityRemovedIterator is returned from FilterLogLiquidityRemoved and is used to iterate over the raw logs and unpacked data for LogLiquidityRemoved events raised by the AutomatedMarketMaker contract.
type AutomatedMarketMakerLogLiquidityRemovedIterator struct {
	Event *AutomatedMarketMakerLogLiquidityRemoved // Event containing the contract specifics and raw log

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
func (it *AutomatedMarketMakerLogLiquidityRemovedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(AutomatedMarketMakerLogLiquidityRemoved)
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
		it.Event = new(AutomatedMarketMakerLogLiquidityRemoved)
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
func (it *AutomatedMarketMakerLogLiquidityRemovedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *AutomatedMarketMakerLogLiquidityRemovedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// AutomatedMarketMakerLogLiquidityRemoved represents a LogLiquidityRemoved event raised by the AutomatedMarketMaker contract.
type AutomatedMarketMakerLogLiquidityRemoved struct {
	Provider     common.Address
	SharesBurned *big.Int
	TokenOut     common.Address
	AmountOut    *big.Int
	Raw          types.Log // Blockchain specific contextual infos
}

// FilterLogLiquidityRemoved is a free log retrieval operation binding the contract event 0x896db5cbb8fec9d3fcb0b08630c8ef5cfd04e4424feb2a6a21d42ddef5d67736.
//
// Solidity: event LogLiquidityRemoved(address indexed provider, uint256 sharesBurned, address indexed tokenOut, uint256 amountOut)
func (_AutomatedMarketMaker *AutomatedMarketMakerFilterer) FilterLogLiquidityRemoved(opts *bind.FilterOpts, provider []common.Address, tokenOut []common.Address) (*AutomatedMarketMakerLogLiquidityRemovedIterator, error) {

	var providerRule []interface{}
	for _, providerItem := range provider {
		providerRule = append(providerRule, providerItem)
	}

	var tokenOutRule []interface{}
	for _, tokenOutItem := range tokenOut {
		tokenOutRule = append(tokenOutRule, tokenOutItem)
	}

	logs, sub, err := _AutomatedMarketMaker.contract.FilterLogs(opts, "LogLiquidityRemoved", providerRule, tokenOutRule)
	if err != nil {
		return nil, err
	}
	return &AutomatedMarketMakerLogLiquidityRemovedIterator{contract: _AutomatedMarketMaker.contract, event: "LogLiquidityRemoved", logs: logs, sub: sub}, nil
}

// WatchLogLiquidityRemoved is a free log subscription operation binding the contract event 0x896db5cbb8fec9d3fcb0b08630c8ef5cfd04e4424feb2a6a21d42ddef5d67736.
//
// Solidity: event LogLiquidityRemoved(address indexed provider, uint256 sharesBurned, address indexed tokenOut, uint256 amountOut)
func (_AutomatedMarketMaker *AutomatedMarketMakerFilterer) WatchLogLiquidityRemoved(opts *bind.WatchOpts, sink chan<- *AutomatedMarketMakerLogLiquidityRemoved, provider []common.Address, tokenOut []common.Address) (event.Subscription, error) {

	var providerRule []interface{}
	for _, providerItem := range provider {
		providerRule = append(providerRule, providerItem)
	}

	var tokenOutRule []interface{}
	for _, tokenOutItem := range tokenOut {
		tokenOutRule = append(tokenOutRule, tokenOutItem)
	}

	logs, sub, err := _AutomatedMarketMaker.contract.WatchLogs(opts, "LogLiquidityRemoved", providerRule, tokenOutRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(AutomatedMarketMakerLogLiquidityRemoved)
				if err := _AutomatedMarketMaker.contract.UnpackLog(event, "LogLiquidityRemoved", log); err != nil {
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

// ParseLogLiquidityRemoved is a log parse operation binding the contract event 0x896db5cbb8fec9d3fcb0b08630c8ef5cfd04e4424feb2a6a21d42ddef5d67736.
//
// Solidity: event LogLiquidityRemoved(address indexed provider, uint256 sharesBurned, address indexed tokenOut, uint256 amountOut)
func (_AutomatedMarketMaker *AutomatedMarketMakerFilterer) ParseLogLiquidityRemoved(log types.Log) (*AutomatedMarketMakerLogLiquidityRemoved, error) {
	event := new(AutomatedMarketMakerLogLiquidityRemoved)
	if err := _AutomatedMarketMaker.contract.UnpackLog(event, "LogLiquidityRemoved", log); err != nil {
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

// AutomatedMarketMakerLogWithdrawalFeeRateUpdatedIterator is returned from FilterLogWithdrawalFeeRateUpdated and is used to iterate over the raw logs and unpacked data for LogWithdrawalFeeRateUpdated events raised by the AutomatedMarketMaker contract.
type AutomatedMarketMakerLogWithdrawalFeeRateUpdatedIterator struct {
	Event *AutomatedMarketMakerLogWithdrawalFeeRateUpdated // Event containing the contract specifics and raw log

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
func (it *AutomatedMarketMakerLogWithdrawalFeeRateUpdatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(AutomatedMarketMakerLogWithdrawalFeeRateUpdated)
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
		it.Event = new(AutomatedMarketMakerLogWithdrawalFeeRateUpdated)
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
func (it *AutomatedMarketMakerLogWithdrawalFeeRateUpdatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *AutomatedMarketMakerLogWithdrawalFeeRateUpdatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// AutomatedMarketMakerLogWithdrawalFeeRateUpdated represents a LogWithdrawalFeeRateUpdated event raised by the AutomatedMarketMaker contract.
type AutomatedMarketMakerLogWithdrawalFeeRateUpdated struct {
	OldWithdrawalFeeBps *big.Int
	NewWithdrawalFeeBps *big.Int
	Raw                 types.Log // Blockchain specific contextual infos
}

// FilterLogWithdrawalFeeRateUpdated is a free log retrieval operation binding the contract event 0x6859e91391e76973b032c6eca264badbd6db550ac3c40bca2db1c4c5e22c7aa8.
//
// Solidity: event LogWithdrawalFeeRateUpdated(uint256 oldWithdrawalFeeBps, uint256 newWithdrawalFeeBps)
func (_AutomatedMarketMaker *AutomatedMarketMakerFilterer) FilterLogWithdrawalFeeRateUpdated(opts *bind.FilterOpts) (*AutomatedMarketMakerLogWithdrawalFeeRateUpdatedIterator, error) {

	logs, sub, err := _AutomatedMarketMaker.contract.FilterLogs(opts, "LogWithdrawalFeeRateUpdated")
	if err != nil {
		return nil, err
	}
	return &AutomatedMarketMakerLogWithdrawalFeeRateUpdatedIterator{contract: _AutomatedMarketMaker.contract, event: "LogWithdrawalFeeRateUpdated", logs: logs, sub: sub}, nil
}

// WatchLogWithdrawalFeeRateUpdated is a free log subscription operation binding the contract event 0x6859e91391e76973b032c6eca264badbd6db550ac3c40bca2db1c4c5e22c7aa8.
//
// Solidity: event LogWithdrawalFeeRateUpdated(uint256 oldWithdrawalFeeBps, uint256 newWithdrawalFeeBps)
func (_AutomatedMarketMaker *AutomatedMarketMakerFilterer) WatchLogWithdrawalFeeRateUpdated(opts *bind.WatchOpts, sink chan<- *AutomatedMarketMakerLogWithdrawalFeeRateUpdated) (event.Subscription, error) {

	logs, sub, err := _AutomatedMarketMaker.contract.WatchLogs(opts, "LogWithdrawalFeeRateUpdated")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(AutomatedMarketMakerLogWithdrawalFeeRateUpdated)
				if err := _AutomatedMarketMaker.contract.UnpackLog(event, "LogWithdrawalFeeRateUpdated", log); err != nil {
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

// ParseLogWithdrawalFeeRateUpdated is a log parse operation binding the contract event 0x6859e91391e76973b032c6eca264badbd6db550ac3c40bca2db1c4c5e22c7aa8.
//
// Solidity: event LogWithdrawalFeeRateUpdated(uint256 oldWithdrawalFeeBps, uint256 newWithdrawalFeeBps)
func (_AutomatedMarketMaker *AutomatedMarketMakerFilterer) ParseLogWithdrawalFeeRateUpdated(log types.Log) (*AutomatedMarketMakerLogWithdrawalFeeRateUpdated, error) {
	event := new(AutomatedMarketMakerLogWithdrawalFeeRateUpdated)
	if err := _AutomatedMarketMaker.contract.UnpackLog(event, "LogWithdrawalFeeRateUpdated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// AutomatedMarketMakerTransferIterator is returned from FilterTransfer and is used to iterate over the raw logs and unpacked data for Transfer events raised by the AutomatedMarketMaker contract.
type AutomatedMarketMakerTransferIterator struct {
	Event *AutomatedMarketMakerTransfer // Event containing the contract specifics and raw log

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
func (it *AutomatedMarketMakerTransferIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(AutomatedMarketMakerTransfer)
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
		it.Event = new(AutomatedMarketMakerTransfer)
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
func (it *AutomatedMarketMakerTransferIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *AutomatedMarketMakerTransferIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// AutomatedMarketMakerTransfer represents a Transfer event raised by the AutomatedMarketMaker contract.
type AutomatedMarketMakerTransfer struct {
	From  common.Address
	To    common.Address
	Value *big.Int
	Raw   types.Log // Blockchain specific contextual infos
}

// FilterTransfer is a free log retrieval operation binding the contract event 0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef.
//
// Solidity: event Transfer(address indexed from, address indexed to, uint256 value)
func (_AutomatedMarketMaker *AutomatedMarketMakerFilterer) FilterTransfer(opts *bind.FilterOpts, from []common.Address, to []common.Address) (*AutomatedMarketMakerTransferIterator, error) {

	var fromRule []interface{}
	for _, fromItem := range from {
		fromRule = append(fromRule, fromItem)
	}
	var toRule []interface{}
	for _, toItem := range to {
		toRule = append(toRule, toItem)
	}

	logs, sub, err := _AutomatedMarketMaker.contract.FilterLogs(opts, "Transfer", fromRule, toRule)
	if err != nil {
		return nil, err
	}
	return &AutomatedMarketMakerTransferIterator{contract: _AutomatedMarketMaker.contract, event: "Transfer", logs: logs, sub: sub}, nil
}

// WatchTransfer is a free log subscription operation binding the contract event 0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef.
//
// Solidity: event Transfer(address indexed from, address indexed to, uint256 value)
func (_AutomatedMarketMaker *AutomatedMarketMakerFilterer) WatchTransfer(opts *bind.WatchOpts, sink chan<- *AutomatedMarketMakerTransfer, from []common.Address, to []common.Address) (event.Subscription, error) {

	var fromRule []interface{}
	for _, fromItem := range from {
		fromRule = append(fromRule, fromItem)
	}
	var toRule []interface{}
	for _, toItem := range to {
		toRule = append(toRule, toItem)
	}

	logs, sub, err := _AutomatedMarketMaker.contract.WatchLogs(opts, "Transfer", fromRule, toRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(AutomatedMarketMakerTransfer)
				if err := _AutomatedMarketMaker.contract.UnpackLog(event, "Transfer", log); err != nil {
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
func (_AutomatedMarketMaker *AutomatedMarketMakerFilterer) ParseTransfer(log types.Log) (*AutomatedMarketMakerTransfer, error) {
	event := new(AutomatedMarketMakerTransfer)
	if err := _AutomatedMarketMaker.contract.UnpackLog(event, "Transfer", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}
