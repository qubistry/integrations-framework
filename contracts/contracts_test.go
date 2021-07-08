package contracts

import (
	"context"
	"math/big"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/smartcontractkit/integrations-framework/client"
	"github.com/smartcontractkit/integrations-framework/config"
	"github.com/smartcontractkit/integrations-framework/environment"
	"github.com/smartcontractkit/integrations-framework/tools"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("Chainlink Node", func() {
	var conf *config.Config

	BeforeEach(func() {
		var err error
		conf, err = config.NewWithPath(config.LocalConfig, "../config")
		Expect(err).ShouldNot(HaveOccurred())
	})

	DescribeTable("deploy and use basic functionality", func(
		initFunc client.BlockchainNetworkInit,
		ocrOptions OffchainOptions,
	) {
		// Setup
		networkConfig, err := initFunc(conf)
		Expect(err).ShouldNot(HaveOccurred())
		blockchainClient, err := client.NewBlockchainClient(networkConfig)
		Expect(err).ShouldNot(HaveOccurred())
		contractDeployer, err := NewContractDeployer(blockchainClient)
		Expect(err).ShouldNot(HaveOccurred())
		wallets, err := networkConfig.Wallets()
		Expect(err).ShouldNot(HaveOccurred())
		_, err = contractDeployer.DeployLinkTokenContract(wallets.Default())
		Expect(err).ShouldNot(HaveOccurred())

		// Connect to running chainlink nodes
		chainlinkNodes, err := client.ConnectToTemplateNodes()
		Expect(err).ShouldNot(HaveOccurred())
		Expect(len(chainlinkNodes)).To(Equal(5))
		// Fund each chainlink node
		for _, node := range chainlinkNodes {
			nodeEthKeys, err := node.ReadETHKeys()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(len(nodeEthKeys.Data)).Should(BeNumerically(">=", 1))
			primaryEthKey := nodeEthKeys.Data[0]

			err = blockchainClient.Fund(
				wallets.Default(),
				primaryEthKey.Attributes.Address,
				big.NewInt(2000000000000000000), big.NewInt(2000000000000000000),
			)
			Expect(err).ShouldNot(HaveOccurred())
		}

		// Deploy and config OCR contract
		ocrInstance, err := contractDeployer.DeployOffChainAggregator(wallets.Default(), ocrOptions)
		Expect(err).ShouldNot(HaveOccurred())
		err = ocrInstance.SetConfig(wallets.Default(), chainlinkNodes, DefaultOffChainAggregatorConfig())
		Expect(err).ShouldNot(HaveOccurred())
		err = ocrInstance.Fund(wallets.Default(), big.NewInt(2000000000000000), big.NewInt(2000000000000000))
		Expect(err).ShouldNot(HaveOccurred())

		// Create external adapter, returns 5 every time
		go tools.NewExternalAdapter("6644")

		// Initialize bootstrap node
		bootstrapNode := chainlinkNodes[0]
		bootstrapP2PIds, err := bootstrapNode.ReadP2PKeys()
		Expect(err).ShouldNot(HaveOccurred())
		bootstrapP2PId := bootstrapP2PIds.Data[0].Attributes.PeerID
		bootstrapSpec := &client.OCRBootstrapJobSpec{
			ContractAddress: ocrInstance.Address(),
			P2PPeerID:       bootstrapP2PId,
			IsBootstrapPeer: true,
		}
		_, err = bootstrapNode.CreateJob(bootstrapSpec)
		Expect(err).ShouldNot(HaveOccurred())

		// Send OCR job to other nodes
		for index := 1; index < len(chainlinkNodes); index++ {
			nodeP2PIds, err := chainlinkNodes[index].ReadP2PKeys()
			Expect(err).ShouldNot(HaveOccurred())
			nodeP2PId := nodeP2PIds.Data[0].Attributes.PeerID
			nodeTransmitterAddresses, err := chainlinkNodes[index].ReadETHKeys()
			Expect(err).ShouldNot(HaveOccurred())
			nodeTransmitterAddress := nodeTransmitterAddresses.Data[0].Attributes.Address
			nodeOCRKeys, err := chainlinkNodes[index].ReadOCRKeys()
			Expect(err).ShouldNot(HaveOccurred())
			nodeOCRKeyId := nodeOCRKeys.Data[0].ID

			observationSource := `fetch    [type=http method=POST url="http://host.docker.internal:6644/five" requestData="{}"];
			parse    [type=jsonparse path="data,result"];
			fetch -> parse;`
			ocrSpec := &client.OCRTaskJobSpec{
				ContractAddress:    ocrInstance.Address(),
				P2PPeerID:          nodeP2PId,
				P2PBootstrapPeers:  []string{bootstrapP2PId},
				KeyBundleID:        nodeOCRKeyId,
				TransmitterAddress: nodeTransmitterAddress,
				ObservationSource:  observationSource,
			}
			_, err = chainlinkNodes[index].CreateJob(ocrSpec)
			Expect(err).ShouldNot(HaveOccurred())
		}

		// Request a new round from the OCR
		err = ocrInstance.RequestNewRound(wallets.Default())
		Expect(err).ShouldNot(HaveOccurred())

		// Wait for a round
		for i := 0; i < 60; i++ {
			round, err := ocrInstance.GetLatestRound(context.Background())
			Expect(err).ShouldNot(HaveOccurred())
			log.Info().
				Str("Contract Address", ocrInstance.Address()).
				Str("Answer", round.Answer.String()).
				Str("Round ID", round.RoundId.String()).
				Str("Answered in Round", round.AnsweredInRound.String()).
				Str("Started At", round.StartedAt.String()).
				Str("Updated At", round.UpdatedAt.String()).
				Msg("Latest Round Data")
			if round.RoundId.Cmp(big.NewInt(0)) > 0 {
				break // Break when OCR round processes
			}
			time.Sleep(time.Second)
		}

		// Check answer is as expected
		answer, err := ocrInstance.GetLatestAnswer(context.Background())
		log.Info().Str("Answer", answer.String()).Msg("Final Answer")
		Expect(err).ShouldNot(HaveOccurred())
		Expect(answer.Int64()).Should(Equal(int64(5)))
	},
		Entry("on Ethereum Hardhat", client.NewHardhatNetwork, DefaultOffChainAggregatorOptions()),
	)
})

var _ = Describe("Contracts", func() {
	var conf *config.Config

	BeforeEach(func() {
		var err error
		conf, err = config.NewWithPath(config.LocalConfig, "../config")
		Expect(err).ShouldNot(HaveOccurred())
	})

	DescribeTable("deploy and interact with the storage contract", func(
		initFunc client.BlockchainNetworkInit,
		value *big.Int,
	) {

		// Setup Network
		networkConfig, err := initFunc(conf)
		Expect(err).ShouldNot(HaveOccurred())
		env, err := environment.NewBasicEnvironment("storage-contract", 0, networkConfig)
		Expect(err).ShouldNot(HaveOccurred())
		client, err := client.NewBlockchainClient(networkConfig)
		Expect(err).ShouldNot(HaveOccurred())
		wallets, err := networkConfig.Wallets()
		Expect(err).ShouldNot(HaveOccurred())
		contractDeployer, err := NewContractDeployer(client)
		Expect(err).ShouldNot(HaveOccurred())
		storeInstance, err := contractDeployer.DeployStorageContract(wallets.Default())
		Expect(err).ShouldNot(HaveOccurred())

		// Interact with contract
		err = storeInstance.Set(value)
		Expect(err).ShouldNot(HaveOccurred())
		val, err := storeInstance.Get(context.Background())
		Expect(err).ShouldNot(HaveOccurred())
		Expect(val).To(Equal(value))

		err = env.TearDown()
		Expect(err).ShouldNot(HaveOccurred())
	},
		Entry("on Ethereum Hardhat", client.NewHardhatNetwork, big.NewInt(5)),
	)

	DescribeTable("deploy and interact with the FluxAggregator contract", func(
		initFunc client.BlockchainNetworkInit,
		fluxOptions FluxAggregatorOptions,
	) {
		// Setup network and client
		networkConfig, err := initFunc(conf)
		Expect(err).ShouldNot(HaveOccurred())
		env, err := environment.NewBasicEnvironment("storage-contract", 0, networkConfig)
		Expect(err).ShouldNot(HaveOccurred())
		client, err := client.NewBlockchainClient(networkConfig)
		Expect(err).ShouldNot(HaveOccurred())
		wallets, err := networkConfig.Wallets()
		Expect(err).ShouldNot(HaveOccurred())
		contractDeployer, err := NewContractDeployer(client)
		Expect(err).ShouldNot(HaveOccurred())

		// Deploy LINK contract
		linkInstance, err := contractDeployer.DeployLinkTokenContract(wallets.Default())
		Expect(err).ShouldNot(HaveOccurred())
		name, err := linkInstance.Name(context.Background())
		Expect(err).ShouldNot(HaveOccurred())
		Expect(name).To(Equal("ChainLink Token"))

		// Deploy FluxMonitor contract
		fluxInstance, err := contractDeployer.DeployFluxAggregatorContract(wallets.Default(), fluxOptions)
		Expect(err).ShouldNot(HaveOccurred())
		err = fluxInstance.Fund(wallets.Default(), big.NewInt(0), big.NewInt(50000000000))
		Expect(err).ShouldNot(HaveOccurred())

		// Interact with contract
		desc, err := fluxInstance.Description(context.Background())
		Expect(err).ShouldNot(HaveOccurred())
		Expect(desc).To(Equal(fluxOptions.Description))

		err = env.TearDown()
		Expect(err).ShouldNot(HaveOccurred())
	},
		Entry("on Ethereum Hardhat", client.NewHardhatNetwork, DefaultFluxAggregatorOptions()),
	)

	DescribeTable("deploy and interact with the OffChain Aggregator contract", func(
		initFunc client.BlockchainNetworkInit,
		ocrOptions OffchainOptions,
	) {
		// Setup network and client
		networkConfig, err := initFunc(conf)
		Expect(err).ShouldNot(HaveOccurred())
		env, err := environment.NewBasicEnvironment("storage-contract", 0, networkConfig)
		Expect(err).ShouldNot(HaveOccurred())
		client, err := client.NewEthereumClient(networkConfig)
		Expect(err).ShouldNot(HaveOccurred())
		wallets, err := networkConfig.Wallets()
		Expect(err).ShouldNot(HaveOccurred())
		contractDeployer, err := NewContractDeployer(client)
		Expect(err).ShouldNot(HaveOccurred())

		// Deploy LINK contract
		linkInstance, err := contractDeployer.DeployLinkTokenContract(wallets.Default())
		Expect(err).ShouldNot(HaveOccurred())
		name, err := linkInstance.Name(context.Background())
		Expect(err).ShouldNot(HaveOccurred())
		Expect(name).To(Equal("ChainLink Token"))

		// Deploy Offchain contract
		offChainInstance, err := contractDeployer.DeployOffChainAggregator(wallets.Default(), ocrOptions)
		Expect(err).ShouldNot(HaveOccurred())
		err = offChainInstance.Fund(wallets.Default(), nil, big.NewInt(50000000000))
		Expect(err).ShouldNot(HaveOccurred())

		err = env.TearDown()
		Expect(err).ShouldNot(HaveOccurred())
	},
		Entry("on Ethereum Hardhat", client.NewHardhatNetwork, DefaultOffChainAggregatorOptions()),
	)
})
