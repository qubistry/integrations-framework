package contracts

import (
	"context"
	"encoding/hex"
	"errors"
	"integrations-framework/client"
	"integrations-framework/contracts/ethereum"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/rs/zerolog/log"
	ocrConfigHelper "github.com/smartcontractkit/libocr/offchainreporting/confighelper"
	ocrTypes "github.com/smartcontractkit/libocr/offchainreporting/types"
)

// EthereumFluxAggregator represents the basic flux aggregation contract
type EthereumFluxAggregator struct {
	client         *client.EthereumClient
	fluxAggregator *ethereum.FluxAggregator
	callerWallet   client.BlockchainWallet
	address        *common.Address
}

// Fund sends specified currencies to the contract
func (f *EthereumFluxAggregator) Fund(fromWallet client.BlockchainWallet, ethAmount, linkAmount *big.Int) error {
	return f.client.Fund(fromWallet, f.address.Hex(), ethAmount, linkAmount)
}

// GetContractData retrieves basic data for the flux aggregator contract
func (f *EthereumFluxAggregator) GetContractData(ctxt context.Context) (*FluxAggregatorData, error) {
	opts := &bind.CallOpts{
		From:    common.HexToAddress(f.callerWallet.Address()),
		Pending: true,
		Context: ctxt,
	}

	allocated, err := f.fluxAggregator.AllocatedFunds(opts)
	if err != nil {
		return &FluxAggregatorData{}, err
	}

	available, err := f.fluxAggregator.AvailableFunds(opts)
	if err != nil {
		return &FluxAggregatorData{}, err
	}

	lr, err := f.fluxAggregator.LatestRoundData(opts)
	if err != nil {
		return &FluxAggregatorData{}, err
	}
	latestRound := RoundData(lr)

	oracles, err := f.fluxAggregator.GetOracles(opts)
	if err != nil {
		return &FluxAggregatorData{}, err
	}

	return &FluxAggregatorData{
		AllocatedFunds:  allocated,
		AvailableFunds:  available,
		LatestRoundData: latestRound,
		Oracles:         oracles,
	}, nil
}

// SetOracles allows the ability to add and/or remove oracles from the contract, and to set admins
func (f *EthereumFluxAggregator) SetOracles(
	fromWallet client.BlockchainWallet,
	toAdd, toRemove, toAdmin []common.Address,
	minSubmissions, maxSubmissions, restartDelay uint32) error {
	opts, err := f.client.TransactionOpts(fromWallet, *f.address, big.NewInt(0), nil)
	if err != nil {
		return err
	}

	tx, err := f.fluxAggregator.ChangeOracles(opts, toRemove, toAdd, toAdmin, minSubmissions, maxSubmissions, restartDelay)
	if err != nil {
		return err
	}
	return f.client.WaitForTransaction(tx.Hash())
}

// Description returns the description of the flux aggregator contract
func (f *EthereumFluxAggregator) Description(ctxt context.Context) (string, error) {
	opts := &bind.CallOpts{
		From:    common.HexToAddress(f.callerWallet.Address()),
		Pending: true,
		Context: ctxt,
	}
	return f.fluxAggregator.Description(opts)
}

// EthereumLinkToken represents a LinkToken address
type EthereumLinkToken struct {
	client       *client.EthereumClient
	linkToken    *ethereum.LinkToken
	callerWallet client.BlockchainWallet
	address      *common.Address
}

// Fund the LINK Token contract with ETH to distribute the token
func (l *EthereumLinkToken) Fund(fromWallet client.BlockchainWallet, ethAmount *big.Int) error {
	return l.client.Fund(fromWallet, l.address.Hex(), ethAmount, nil)
}

// Name returns the name of the link token
func (l *EthereumLinkToken) Name(ctxt context.Context) (string, error) {
	opts := &bind.CallOpts{
		From:    common.HexToAddress(l.callerWallet.Address()),
		Pending: true,
		Context: ctxt,
	}
	return l.linkToken.Name(opts)
}

func (l *EthereumLinkToken) Address() string {
	return l.address.Hex()
}

// EthereumOffchainAggregator represents the offchain aggregation contract
type EthereumOffchainAggregator struct {
	client       *client.EthereumClient
	ocr          *ethereum.OffchainAggregator
	callerWallet client.BlockchainWallet
	address      *common.Address
}

// Fund sends specified currencies to the contract
func (o *EthereumOffchainAggregator) Fund(fromWallet client.BlockchainWallet, ethAmount, linkAmount *big.Int) error {
	return o.client.Fund(fromWallet, o.address.Hex(), ethAmount, linkAmount)
}

// GetContractData retrieves basic data for the offchain aggregator contract
func (o *EthereumOffchainAggregator) GetContractData(ctxt context.Context) (*OffchainAggregatorData, error) {
	opts := &bind.CallOpts{
		From:    common.HexToAddress(o.callerWallet.Address()),
		Pending: true,
		Context: ctxt,
	}

	lr, err := o.ocr.LatestRoundData(opts)
	if err != nil {
		return &OffchainAggregatorData{}, err
	}
	latestRound := RoundData(lr)

	return &OffchainAggregatorData{
		LatestRoundData: latestRound,
	}, nil
}

// SetPayees sets wallets for the contract to pay out to?
func (o *EthereumOffchainAggregator) SetPayees(
	fromWallet client.BlockchainWallet,
	transmitters, payees []common.Address,
) error {
	opts, err := o.client.TransactionOpts(fromWallet, *o.address, big.NewInt(0), nil)
	if err != nil {
		return err
	}

	tx, err := o.ocr.SetPayees(opts, transmitters, payees)
	if err != nil {
		return err
	}
	return o.client.WaitForTransaction(tx.Hash())
}

// SetConfig sets offchain reporting protocol configuration including participating oracles
func (o *EthereumOffchainAggregator) SetConfig(
	fromWallet client.BlockchainWallet,
	chainlinkNodes []client.Chainlink,
	ocrConfig OffChainAggregatorConfig,
) error {
	// Gather necessary addresses and keys from our chainlink nodes to properly configure the OCR contract
	for _, node := range chainlinkNodes {
		ocrKeys, err := node.ReadOCRKeys()
		if err != nil {
			return err
		}
		primaryOCRKey := ocrKeys.Data[0]
		ethKeys, err := node.ReadETHKeys()
		if err != nil {
			return err
		}
		primaryEthKey := ethKeys.Data[0]
		p2pKeys, err := node.ReadP2PKeys()
		if err != nil {
			return err
		}
		primaryP2PKey := p2pKeys.Data[0]

		// Need to convert the key representations
		var onChainSigningAddress [20]byte
		var configPublicKey [32]byte
		offchainSigningAddress, err := hex.DecodeString(primaryOCRKey.Attributes.OffChainPublicKey)
		if err != nil {
			return err
		}
		decodeConfigKey, err := hex.DecodeString(primaryOCRKey.Attributes.ConfigPublicKey)
		if err != nil {
			return err
		}

		// https://stackoverflow.com/questions/8032170/how-to-assign-string-to-bytes-array
		copy(onChainSigningAddress[:], common.HexToAddress(primaryOCRKey.Attributes.OnChainSigningAddress).Bytes())
		copy(configPublicKey[:], decodeConfigKey)

		oracleIdentity := ocrConfigHelper.OracleIdentity{
			TransmitAddress:       common.HexToAddress(primaryEthKey.Attributes.Address),
			OnChainSigningAddress: onChainSigningAddress,
			PeerID:                primaryP2PKey.Attributes.PeerID,
			OffchainPublicKey:     offchainSigningAddress,
		}
		oracleIdentityExtra := ocrConfigHelper.OracleIdentityExtra{
			OracleIdentity:                  oracleIdentity,
			SharedSecretEncryptionPublicKey: ocrTypes.SharedSecretEncryptionPublicKey(configPublicKey),
		}

		ocrConfig.OracleIdentities = append(ocrConfig.OracleIdentities, oracleIdentityExtra)
	}

	signers, transmitters, threshold, encodedConfigVersion, encodedConfig, err := ocrConfigHelper.ContractSetConfigArgs(
		ocrConfig.DeltaProgress,
		ocrConfig.DeltaResend,
		ocrConfig.DeltaRound,
		ocrConfig.DeltaGrace,
		ocrConfig.DeltaC,
		ocrConfig.AlphaPPB,
		ocrConfig.DeltaStage,
		ocrConfig.RMax,
		ocrConfig.S,
		ocrConfig.OracleIdentities,
		ocrConfig.F,
	)
	if err != nil {
		return err
	}

	// Set Payees
	opts, err := o.client.TransactionOpts(fromWallet, *o.address, big.NewInt(0), nil)
	if err != nil {
		return err
	}

	tx, err := o.ocr.SetPayees(opts, transmitters, transmitters)
	if err != nil {
		return err
	}
	err = o.client.WaitForTransaction(tx.Hash())
	if err != nil {
		return err
	}

	// Set Config
	opts, err = o.client.TransactionOpts(fromWallet, *o.address, big.NewInt(0), nil)
	if err != nil {
		return err
	}

	tx, err = o.ocr.SetConfig(opts, signers, transmitters, threshold, encodedConfigVersion, encodedConfig)
	if err != nil {
		return err
	}
	return o.client.WaitForTransaction(tx.Hash())
}

// RequestNewRound requests the OCR contract to create a new round
func (o *EthereumOffchainAggregator) RequestNewRound(fromWallet client.BlockchainWallet) error {
	opts, err := o.client.TransactionOpts(fromWallet, *o.address, big.NewInt(0), nil)
	if err != nil {
		return err
	}
	tx, err := o.ocr.RequestNewRound(opts)
	if err != nil {
		return err
	}
	log.Info().Str("Contract Address", o.address.Hex()).Msg("New OCR round requested")
	return o.client.WaitForTransaction(tx.Hash())
}

// Link returns the LINK contract address on the EVM chain
func (o *EthereumOffchainAggregator) Link(ctxt context.Context) (common.Address, error) {
	opts := &bind.CallOpts{
		From:    common.HexToAddress(o.callerWallet.Address()),
		Pending: true,
		Context: ctxt,
	}
	return o.ocr.LINK(opts)
}

// GetLatestAnswer returns the latest answer from the OCR contract
func (o *EthereumOffchainAggregator) GetLatestAnswer(ctxt context.Context) (*big.Int, error) {
	opts := &bind.CallOpts{
		From:    common.HexToAddress(o.callerWallet.Address()),
		Pending: true,
		Context: ctxt,
	}
	return o.ocr.LatestAnswer(opts)
}

func (o *EthereumOffchainAggregator) Address() string {
	return o.address.Hex()
}

// GetLatestRound returns data from the latest round
func (o *EthereumOffchainAggregator) GetLatestRound(ctxt context.Context) (*RoundData, error) {
	opts := &bind.CallOpts{
		From:    common.HexToAddress(o.callerWallet.Address()),
		Pending: true,
		Context: ctxt,
	}

	roundData, err := o.ocr.LatestRoundData(opts)
	if err != nil {
		return nil, err
	}

	return &RoundData{
		RoundId:         roundData.RoundId,
		Answer:          roundData.Answer,
		AnsweredInRound: roundData.AnsweredInRound,
		StartedAt:       roundData.StartedAt,
		UpdatedAt:       roundData.UpdatedAt,
	}, err
}

// EthereumStorage acts as a conduit for the ethereum version of the storage contract
type EthereumStorage struct {
	client       *client.EthereumClient
	store        *ethereum.Store
	callerWallet client.BlockchainWallet
}

// Set sets a value in the storage contract
func (e *EthereumStorage) Set(value *big.Int) error {
	opts, err := e.client.TransactionOpts(e.callerWallet, common.Address{}, big.NewInt(0), nil)
	if err != nil {
		return err
	}

	transaction, err := e.store.Set(opts, value)
	if err != nil {
		return err
	}
	return e.client.WaitForTransaction(transaction.Hash())
}

// Get retrieves a set value from the storage contract
func (e *EthereumStorage) Get(ctxt context.Context) (*big.Int, error) {
	opts := &bind.CallOpts{
		From:    common.HexToAddress(e.callerWallet.Address()),
		Pending: true,
		Context: ctxt,
	}
	return e.store.Get(opts)
}

// EthereumVRFCoordinator represents the VRF Coordinator contract
type EthereumVRFCoordinator struct {
	address      *common.Address
	client       *client.EthereumClient
	coordinator  *ethereum.VRFCoordinator
	callerWallet client.BlockchainWallet
}

// Fund sends specified eth and link to the contract
func (e *EthereumVRFCoordinator) Fund(fromWallet client.BlockchainWallet, ethAmount, linkAmount *big.Int) error {
	return e.client.Fund(fromWallet, e.address.Hex(), ethAmount, linkAmount)
}

// Address returns the address where the contract is deployed
func (e *EthereumVRFCoordinator) Address() string {
	return e.address.Hex()
}

func (e *EthereumVRFCoordinator) ProofLength(ctxt context.Context) (*big.Int, error) {
	opts := &bind.CallOpts{
		From:    common.HexToAddress(e.callerWallet.Address()),
		Pending: true,
		Context: ctxt,
	}

	return e.coordinator.PROOFLENGTH(opts)
}

// Registers a proving key with a chainlink node
func (e *EthereumVRFCoordinator) RegisterProvingKey(
	fromWallet client.BlockchainWallet,
	linkAmount *big.Int,
	chainlinkNode client.Chainlink,
	publicProvingKey [2]*big.Int,
	chainlinkJobId string,
) error {
	opts, err := e.client.TransactionOpts(fromWallet, *e.address, big.NewInt(0), nil)
	if err != nil {
		return err
	}

	// Get chainlink address
	ethKeys, err := chainlinkNode.ReadETHKeys()
	if err != nil {
		return err
	}
	if len(ethKeys.Data) < 1 {
		return errors.New("Retrieved 0 ETH keys from the chainlink node")
	}
	primaryEthKey := common.HexToAddress(ethKeys.Data[0].Attributes.Address)

	var jobId [32]byte
	copy(jobId[:], []byte(chainlinkJobId))

	tx, err := e.coordinator.RegisterProvingKey(opts, linkAmount, primaryEthKey, publicProvingKey, jobId)
	if err != nil {
		return err
	}
	return e.client.WaitForTransaction(tx.Hash())
}

// EthereumVRFConsumer represents the VRF consumer contract
type EthereumVRFConsumer struct {
	address      *common.Address
	client       *client.EthereumClient
	consumer     *ethereum.VRFConsumer
	callerWallet client.BlockchainWallet
}

// Fund sends specified eth and link to the contract
func (e *EthereumVRFConsumer) Fund(fromWallet client.BlockchainWallet, ethAmount, linkAmount *big.Int) error {
	return e.client.Fund(fromWallet, e.address.Hex(), ethAmount, linkAmount)
}

// Address returns the address where the contract is deployed
func (e *EthereumVRFConsumer) Address() string {
	return e.address.Hex()
}

// RequestRandomness makes a request to the coordinator for randomness
// keyHash ID of public key against which randomness is generated
// fee The amount of LINK to send with the request
// seed seed mixed into the input of the VRF. Mostly vestigal, can be null
func (e *EthereumVRFConsumer) RequestRandomness(fromWallet client.BlockchainWallet, keyHash [32]byte, fee, seed *big.Int) error {
	opts, err := e.client.TransactionOpts(fromWallet, *e.address, big.NewInt(0), nil)
	if err != nil {
		return err
	}

	tx, err := e.consumer.TestRequestRandomness(opts, keyHash, fee, seed)
	if err != nil {
		return err
	}
	return e.client.WaitForTransaction(tx.Hash())
}

// RequestId returns the unique ID for a request
func (e *EthereumVRFConsumer) RequestId(ctxt context.Context) ([32]byte, error) {
	opts := &bind.CallOpts{
		From:    common.HexToAddress(e.callerWallet.Address()),
		Pending: true,
		Context: ctxt,
	}
	return e.consumer.RequestId(opts)
}

// RandomnessOutput retrieves the random number
func (e *EthereumVRFConsumer) RandomnessOutput(ctxt context.Context) (*big.Int, error) {
	opts := &bind.CallOpts{
		From:    common.HexToAddress(e.callerWallet.Address()),
		Pending: true,
		Context: ctxt,
	}
	return e.consumer.RandomnessOutput(opts)
}
