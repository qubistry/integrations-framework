// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package ethereum

import (
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
	_ = big.NewInt
	_ = strings.NewReader
	_ = ethereum.NotFound
	_ = bind.Bind
	_ = common.Big1
	_ = types.BloomLookup
	_ = event.NewSubscription
)

// VRFConsumerABI is the input ABI used to generate the binding from.
const VRFConsumerABI = "[{\"inputs\":[{\"internalType\":\"address\",\"name\":\"_vrfCoordinator\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"_link\",\"type\":\"address\"}],\"stateMutability\":\"nonpayable\",\"type\":\"constructor\"},{\"inputs\":[],\"name\":\"randomnessOutput\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"requestId\",\"type\":\"bytes32\"},{\"internalType\":\"uint256\",\"name\":\"randomness\",\"type\":\"uint256\"}],\"name\":\"rawFulfillRandomness\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"requestId\",\"outputs\":[{\"internalType\":\"bytes32\",\"name\":\"\",\"type\":\"bytes32\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"_keyHash\",\"type\":\"bytes32\"},{\"internalType\":\"uint256\",\"name\":\"_fee\",\"type\":\"uint256\"}],\"name\":\"testRequestRandomness\",\"outputs\":[{\"internalType\":\"bytes32\",\"name\":\"requestId\",\"type\":\"bytes32\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"}]"

// VRFConsumerBin is the compiled bytecode used for deploying new contracts.
var VRFConsumerBin = "0x60c060405234801561001057600080fd5b506040516104b33803806104b38339818101604052604081101561003357600080fd5b5080516020909101516001600160601b0319606092831b811660a052911b1660805260805160601c60a05160601c6104316100826000398060e4528061019752508061016852506104316000f3fe608060405234801561001057600080fd5b506004361061004b5760003560e01c80626d6cae146100505780632f47fd861461006a578063866ee7481461007257806394985ddd14610095575b600080fd5b6100586100ba565b60408051918252519081900360200190f35b6100586100c0565b6100586004803603604081101561008857600080fd5b50803590602001356100c6565b6100b8600480360360408110156100ab57600080fd5b50803590602001356100d9565b005b60025481565b60015481565b60006100d28383610164565b9392505050565b336001600160a01b037f00000000000000000000000000000000000000000000000000000000000000001614610156576040805162461bcd60e51b815260206004820152601f60248201527f4f6e6c7920565246436f6f7264696e61746f722063616e2066756c66696c6c00604482015290519081900360640190fd5b6101608282610326565b5050565b60007f00000000000000000000000000000000000000000000000000000000000000006001600160a01b0316634000aea07f00000000000000000000000000000000000000000000000000000000000000008486600060405160200180838152602001828152602001925050506040516020818303038152906040526040518463ffffffff1660e01b815260040180846001600160a01b03166001600160a01b0316815260200183815260200180602001828103825283818151815260200191508051906020019080838360005b8381101561024a578181015183820152602001610232565b50505050905090810190601f1680156102775780820380516001836020036101000a031916815260200191505b50945050505050602060405180830381600087803b15801561029857600080fd5b505af11580156102ac573d6000803e3d6000fd5b505050506040513d60208110156102c257600080fd5b50506000838152602081905260408120546102e29085908390309061032e565b60008581526020819052604090205490915061030590600163ffffffff61037516565b60008581526020819052604090205561031e84826103cf565b949350505050565b600155600255565b60408051602080820196909652808201949094526001600160a01b039290921660608401526080808401919091528151808403909101815260a09092019052805191012090565b6000828201838110156100d2576040805162461bcd60e51b815260206004820152601b60248201527f536166654d6174683a206164646974696f6e206f766572666c6f770000000000604482015290519081900360640190fd5b60408051602080820194909452808201929092528051808303820181526060909201905280519101209056fea26469706673582212200efdc6883e84f1727409e1be45ef60ca7bfd91860c3027cba16a6a4f92422ef764736f6c63430006060033"

// DeployVRFConsumer deploys a new Ethereum contract, binding an instance of VRFConsumer to it.
func DeployVRFConsumer(auth *bind.TransactOpts, backend bind.ContractBackend, _vrfCoordinator common.Address, _link common.Address) (common.Address, *types.Transaction, *VRFConsumer, error) {
	parsed, err := abi.JSON(strings.NewReader(VRFConsumerABI))
	if err != nil {
		return common.Address{}, nil, nil, err
	}

	address, tx, contract, err := bind.DeployContract(auth, parsed, common.FromHex(VRFConsumerBin), backend, _vrfCoordinator, _link)
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	return address, tx, &VRFConsumer{VRFConsumerCaller: VRFConsumerCaller{contract: contract}, VRFConsumerTransactor: VRFConsumerTransactor{contract: contract}, VRFConsumerFilterer: VRFConsumerFilterer{contract: contract}}, nil
}

// VRFConsumer is an auto generated Go binding around an Ethereum contract.
type VRFConsumer struct {
	VRFConsumerCaller     // Read-only binding to the contract
	VRFConsumerTransactor // Write-only binding to the contract
	VRFConsumerFilterer   // Log filterer for contract events
}

// VRFConsumerCaller is an auto generated read-only Go binding around an Ethereum contract.
type VRFConsumerCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// VRFConsumerTransactor is an auto generated write-only Go binding around an Ethereum contract.
type VRFConsumerTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// VRFConsumerFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type VRFConsumerFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// VRFConsumerSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type VRFConsumerSession struct {
	Contract     *VRFConsumer      // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// VRFConsumerCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type VRFConsumerCallerSession struct {
	Contract *VRFConsumerCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts      // Call options to use throughout this session
}

// VRFConsumerTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type VRFConsumerTransactorSession struct {
	Contract     *VRFConsumerTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts      // Transaction auth options to use throughout this session
}

// VRFConsumerRaw is an auto generated low-level Go binding around an Ethereum contract.
type VRFConsumerRaw struct {
	Contract *VRFConsumer // Generic contract binding to access the raw methods on
}

// VRFConsumerCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type VRFConsumerCallerRaw struct {
	Contract *VRFConsumerCaller // Generic read-only contract binding to access the raw methods on
}

// VRFConsumerTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type VRFConsumerTransactorRaw struct {
	Contract *VRFConsumerTransactor // Generic write-only contract binding to access the raw methods on
}

// NewVRFConsumer creates a new instance of VRFConsumer, bound to a specific deployed contract.
func NewVRFConsumer(address common.Address, backend bind.ContractBackend) (*VRFConsumer, error) {
	contract, err := bindVRFConsumer(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &VRFConsumer{VRFConsumerCaller: VRFConsumerCaller{contract: contract}, VRFConsumerTransactor: VRFConsumerTransactor{contract: contract}, VRFConsumerFilterer: VRFConsumerFilterer{contract: contract}}, nil
}

// NewVRFConsumerCaller creates a new read-only instance of VRFConsumer, bound to a specific deployed contract.
func NewVRFConsumerCaller(address common.Address, caller bind.ContractCaller) (*VRFConsumerCaller, error) {
	contract, err := bindVRFConsumer(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &VRFConsumerCaller{contract: contract}, nil
}

// NewVRFConsumerTransactor creates a new write-only instance of VRFConsumer, bound to a specific deployed contract.
func NewVRFConsumerTransactor(address common.Address, transactor bind.ContractTransactor) (*VRFConsumerTransactor, error) {
	contract, err := bindVRFConsumer(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &VRFConsumerTransactor{contract: contract}, nil
}

// NewVRFConsumerFilterer creates a new log filterer instance of VRFConsumer, bound to a specific deployed contract.
func NewVRFConsumerFilterer(address common.Address, filterer bind.ContractFilterer) (*VRFConsumerFilterer, error) {
	contract, err := bindVRFConsumer(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &VRFConsumerFilterer{contract: contract}, nil
}

// bindVRFConsumer binds a generic wrapper to an already deployed contract.
func bindVRFConsumer(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := abi.JSON(strings.NewReader(VRFConsumerABI))
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_VRFConsumer *VRFConsumerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _VRFConsumer.Contract.VRFConsumerCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_VRFConsumer *VRFConsumerRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _VRFConsumer.Contract.VRFConsumerTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_VRFConsumer *VRFConsumerRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _VRFConsumer.Contract.VRFConsumerTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_VRFConsumer *VRFConsumerCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _VRFConsumer.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_VRFConsumer *VRFConsumerTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _VRFConsumer.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_VRFConsumer *VRFConsumerTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _VRFConsumer.Contract.contract.Transact(opts, method, params...)
}

// RandomnessOutput is a free data retrieval call binding the contract method 0x2f47fd86.
//
// Solidity: function randomnessOutput() view returns(uint256)
func (_VRFConsumer *VRFConsumerCaller) RandomnessOutput(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _VRFConsumer.contract.Call(opts, &out, "randomnessOutput")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// RandomnessOutput is a free data retrieval call binding the contract method 0x2f47fd86.
//
// Solidity: function randomnessOutput() view returns(uint256)
func (_VRFConsumer *VRFConsumerSession) RandomnessOutput() (*big.Int, error) {
	return _VRFConsumer.Contract.RandomnessOutput(&_VRFConsumer.CallOpts)
}

// RandomnessOutput is a free data retrieval call binding the contract method 0x2f47fd86.
//
// Solidity: function randomnessOutput() view returns(uint256)
func (_VRFConsumer *VRFConsumerCallerSession) RandomnessOutput() (*big.Int, error) {
	return _VRFConsumer.Contract.RandomnessOutput(&_VRFConsumer.CallOpts)
}

// RequestId is a free data retrieval call binding the contract method 0x006d6cae.
//
// Solidity: function requestId() view returns(bytes32)
func (_VRFConsumer *VRFConsumerCaller) RequestId(opts *bind.CallOpts) ([32]byte, error) {
	var out []interface{}
	err := _VRFConsumer.contract.Call(opts, &out, "requestId")

	if err != nil {
		return *new([32]byte), err
	}

	out0 := *abi.ConvertType(out[0], new([32]byte)).(*[32]byte)

	return out0, err

}

// RequestId is a free data retrieval call binding the contract method 0x006d6cae.
//
// Solidity: function requestId() view returns(bytes32)
func (_VRFConsumer *VRFConsumerSession) RequestId() ([32]byte, error) {
	return _VRFConsumer.Contract.RequestId(&_VRFConsumer.CallOpts)
}

// RequestId is a free data retrieval call binding the contract method 0x006d6cae.
//
// Solidity: function requestId() view returns(bytes32)
func (_VRFConsumer *VRFConsumerCallerSession) RequestId() ([32]byte, error) {
	return _VRFConsumer.Contract.RequestId(&_VRFConsumer.CallOpts)
}

// RawFulfillRandomness is a paid mutator transaction binding the contract method 0x94985ddd.
//
// Solidity: function rawFulfillRandomness(bytes32 requestId, uint256 randomness) returns()
func (_VRFConsumer *VRFConsumerTransactor) RawFulfillRandomness(opts *bind.TransactOpts, requestId [32]byte, randomness *big.Int) (*types.Transaction, error) {
	return _VRFConsumer.contract.Transact(opts, "rawFulfillRandomness", requestId, randomness)
}

// RawFulfillRandomness is a paid mutator transaction binding the contract method 0x94985ddd.
//
// Solidity: function rawFulfillRandomness(bytes32 requestId, uint256 randomness) returns()
func (_VRFConsumer *VRFConsumerSession) RawFulfillRandomness(requestId [32]byte, randomness *big.Int) (*types.Transaction, error) {
	return _VRFConsumer.Contract.RawFulfillRandomness(&_VRFConsumer.TransactOpts, requestId, randomness)
}

// RawFulfillRandomness is a paid mutator transaction binding the contract method 0x94985ddd.
//
// Solidity: function rawFulfillRandomness(bytes32 requestId, uint256 randomness) returns()
func (_VRFConsumer *VRFConsumerTransactorSession) RawFulfillRandomness(requestId [32]byte, randomness *big.Int) (*types.Transaction, error) {
	return _VRFConsumer.Contract.RawFulfillRandomness(&_VRFConsumer.TransactOpts, requestId, randomness)
}

// TestRequestRandomness is a paid mutator transaction binding the contract method 0x866ee748.
//
// Solidity: function testRequestRandomness(bytes32 _keyHash, uint256 _fee) returns(bytes32 requestId)
func (_VRFConsumer *VRFConsumerTransactor) TestRequestRandomness(opts *bind.TransactOpts, _keyHash [32]byte, _fee *big.Int) (*types.Transaction, error) {
	return _VRFConsumer.contract.Transact(opts, "testRequestRandomness", _keyHash, _fee)
}

// TestRequestRandomness is a paid mutator transaction binding the contract method 0x866ee748.
//
// Solidity: function testRequestRandomness(bytes32 _keyHash, uint256 _fee) returns(bytes32 requestId)
func (_VRFConsumer *VRFConsumerSession) TestRequestRandomness(_keyHash [32]byte, _fee *big.Int) (*types.Transaction, error) {
	return _VRFConsumer.Contract.TestRequestRandomness(&_VRFConsumer.TransactOpts, _keyHash, _fee)
}

// TestRequestRandomness is a paid mutator transaction binding the contract method 0x866ee748.
//
// Solidity: function testRequestRandomness(bytes32 _keyHash, uint256 _fee) returns(bytes32 requestId)
func (_VRFConsumer *VRFConsumerTransactorSession) TestRequestRandomness(_keyHash [32]byte, _fee *big.Int) (*types.Transaction, error) {
	return _VRFConsumer.Contract.TestRequestRandomness(&_VRFConsumer.TransactOpts, _keyHash, _fee)
}
