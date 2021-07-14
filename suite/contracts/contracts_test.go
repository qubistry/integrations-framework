package suite

import (
	"context"
	"github.com/smartcontractkit/integrations-framework/contracts"
	"github.com/smartcontractkit/integrations-framework/suite"
	"math/big"
	"time"

	"github.com/smartcontractkit/integrations-framework/client"
	"github.com/smartcontractkit/integrations-framework/tools"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/rs/zerolog/log"
)

var _ = Describe("Chainlink Node", func() {
	DescribeTable("deploy and use basic functionality", func(
		initFunc client.BlockchainNetworkInit,
		ocrOptions contracts.OffchainOptions,
	) {
		s, err := suite.DefaultLocalSetup(initFunc)
		Expect(err).ShouldNot(HaveOccurred())

		// Connect to running chainlink nodes
		chainlinkNodes, _, err := suite.ConnectToTemplateNodes()
		Expect(err).ShouldNot(HaveOccurred())
		Expect(len(chainlinkNodes)).To(Equal(5))
		// Fund each chainlink node
		for _, node := range chainlinkNodes {
			nodeEthKeys, err := node.ReadETHKeys()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(len(nodeEthKeys.Data)).Should(BeNumerically(">=", 1))
			primaryEthKey := nodeEthKeys.Data[0]

			err = s.Client.Fund(
				s.Wallets.Default(),
				primaryEthKey.Attributes.Address,
				big.NewInt(2000000000000000000), big.NewInt(2000000000000000000),
			)
			Expect(err).ShouldNot(HaveOccurred())
		}

		// Deploy and config OCR contract
		ocrInstance, err := s.Deployer.DeployOffChainAggregator(s.Wallets.Default(), ocrOptions)
		Expect(err).ShouldNot(HaveOccurred())
		err = ocrInstance.SetConfig(s.Wallets.Default(), chainlinkNodes, contracts.DefaultOffChainAggregatorConfig())
		Expect(err).ShouldNot(HaveOccurred())
		err = ocrInstance.Fund(s.Wallets.Default(), big.NewInt(2000000000000000), big.NewInt(2000000000000000))
		Expect(err).ShouldNot(HaveOccurred())

		// Create external adapter, returns 5 every time
		adapter := tools.NewExternalAdapter()

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

			ocrSpec := &client.OCRTaskJobSpec{
				ContractAddress:    ocrInstance.Address(),
				P2PPeerID:          nodeP2PId,
				P2PBootstrapPeers:  []string{bootstrapP2PId},
				KeyBundleID:        nodeOCRKeyId,
				TransmitterAddress: nodeTransmitterAddress,
				ObservationSource:  client.ObservationSourceSpec(adapter.InsideDockerAddr + "/five"),
			}
			_, err = chainlinkNodes[index].CreateJob(ocrSpec)
			Expect(err).ShouldNot(HaveOccurred())
		}

		// Request a new round from the OCR
		err = ocrInstance.RequestNewRound(s.Wallets.Default())
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
		Entry("on Ethereum Hardhat", client.NewHardhatNetwork, contracts.DefaultOffChainAggregatorOptions()),
	)
})

var _ = Describe("Contracts", func() {
	DescribeTable("deploy and interact with the storage contract", func(
		initFunc client.BlockchainNetworkInit,
		value *big.Int,
	) {
		s, err := suite.DefaultLocalSetup(initFunc)
		Expect(err).ShouldNot(HaveOccurred())

		storeInstance, err := s.Deployer.DeployStorageContract(s.Wallets.Default())
		Expect(err).ShouldNot(HaveOccurred())

		// Interact with contract
		err = storeInstance.Set(value)
		Expect(err).ShouldNot(HaveOccurred())
		val, err := storeInstance.Get(context.Background())
		Expect(err).ShouldNot(HaveOccurred())
		Expect(val).To(Equal(value))
	},
		Entry("on Ethereum Hardhat", client.NewHardhatNetwork, big.NewInt(5)),
	)

	DescribeTable("deploy and interact with the FluxAggregator contract", func(
		initFunc client.BlockchainNetworkInit,
		fluxOptions contracts.FluxAggregatorOptions,
	) {
		s, err := suite.DefaultLocalSetup(initFunc)
		Expect(err).ShouldNot(HaveOccurred())

		// Deploy LINK contract
		linkInstance, err := s.Deployer.DeployLinkTokenContract(s.Wallets.Default())
		Expect(err).ShouldNot(HaveOccurred())
		name, err := linkInstance.Name(context.Background())
		Expect(err).ShouldNot(HaveOccurred())
		Expect(name).To(Equal("ChainLink Token"))

		// Deploy FluxMonitor contract
		fluxInstance, err := s.Deployer.DeployFluxAggregatorContract(s.Wallets.Default(), fluxOptions)
		Expect(err).ShouldNot(HaveOccurred())
		err = fluxInstance.Fund(s.Wallets.Default(), big.NewInt(0), big.NewInt(50000000000))
		Expect(err).ShouldNot(HaveOccurred())

		// Interact with contract
		desc, err := fluxInstance.Description(context.Background())
		Expect(err).ShouldNot(HaveOccurred())
		Expect(desc).To(Equal(fluxOptions.Description))
	},
		Entry("on Ethereum Hardhat", client.NewHardhatNetwork, contracts.DefaultFluxAggregatorOptions()),
	)

	DescribeTable("deploy and interact with the OffChain Aggregator contract", func(
		initFunc client.BlockchainNetworkInit,
		ocrOptions contracts.OffchainOptions,
	) {
		s, err := suite.DefaultLocalSetup(initFunc)
		Expect(err).ShouldNot(HaveOccurred())

		// Deploy LINK contract
		linkInstance, err := s.Deployer.DeployLinkTokenContract(s.Wallets.Default())
		Expect(err).ShouldNot(HaveOccurred())
		name, err := linkInstance.Name(context.Background())
		Expect(err).ShouldNot(HaveOccurred())
		Expect(name).To(Equal("ChainLink Token"))

		// Deploy Offchain contract
		offChainInstance, err := s.Deployer.DeployOffChainAggregator(s.Wallets.Default(), ocrOptions)
		Expect(err).ShouldNot(HaveOccurred())
		err = offChainInstance.Fund(s.Wallets.Default(), nil, big.NewInt(50000000000))
		Expect(err).ShouldNot(HaveOccurred())
	},
		Entry("on Ethereum Hardhat", client.NewHardhatNetwork, contracts.DefaultOffChainAggregatorOptions()),
	)
})
