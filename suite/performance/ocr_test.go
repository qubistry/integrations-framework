package performance

import (
	"math/big"
	"time"

	"github.com/smartcontractkit/integrations-framework/hooks"
	"github.com/smartcontractkit/integrations-framework/utils"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/smartcontractkit/integrations-framework/actions"
	"github.com/smartcontractkit/integrations-framework/client"
	"github.com/smartcontractkit/integrations-framework/contracts"
	"github.com/smartcontractkit/integrations-framework/environment"
)

var _ = Describe("OCR soak test @soak-ocr", func() {
	var (
		suiteSetup  actions.SuiteSetup
		networkInfo actions.NetworkInfo
		nodes       []client.Chainlink
		perfTest    Test
		err         error
	)

	BeforeEach(func() {
		By("Deploying the environment", func() {
			suiteSetup, err = actions.SingleNetworkSetup(
				environment.NewChainlinkCluster(5),
				hooks.RskTestNetworkHook,
				hooks.EthereumDeployerHook,
				hooks.EthereumClientHook,
				utils.ProjectRoot,
			)
			Expect(err).ShouldNot(HaveOccurred())
			nodes, err = environment.GetChainlinkClients(suiteSetup.Environment())
			Expect(err).ShouldNot(HaveOccurred())
			networkInfo = suiteSetup.DefaultNetwork()

			networkInfo.Client.ParallelTransactions(false)
		})

		By("Funding the Chainlink nodes", func() {
			err := actions.FundChainlinkNodes(
				nodes,
				networkInfo.Client,
				networkInfo.Wallets.Default(),
				big.NewFloat(0),
				big.NewFloat(0),
			)
			Expect(err).ShouldNot(HaveOccurred())
		})

		By("Setting up the OCR soak test", func() {
			perfTest = NewOCRTest(
				OCRTestOptions{
					TestOptions: TestOptions{
						NumberOfContracts: 1,
					},
					RoundTimeout: 360 * time.Second,
					AdapterValue: 5,
					TestDuration: 10080 * time.Minute,
				},
				contracts.DefaultOffChainAggregatorOptions(),
				suiteSetup.Environment(),
				networkInfo.Client,
				networkInfo.Wallets,
				networkInfo.Deployer,
			)
			err = perfTest.Setup()
			Expect(err).ShouldNot(HaveOccurred())
		})
	})

	Describe("OCR Soak test", func() {
		Measure("Measure OCR rounds", func(_ Benchmarker) {
			err = perfTest.Run()
			Expect(err).ShouldNot(HaveOccurred())
		}, 1)
	})

	AfterEach(func() {
		By("Tearing down the environment", suiteSetup.TearDown())
	})
})
