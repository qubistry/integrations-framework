package performance

import (
	"github.com/smartcontractkit/integrations-framework/hooks"
	"github.com/smartcontractkit/integrations-framework/utils"
	"math/big"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/smartcontractkit/integrations-framework/actions"
	"github.com/smartcontractkit/integrations-framework/client"
	"github.com/smartcontractkit/integrations-framework/environment"
)

var _ = Describe("Keeper performance test @performance-keeper", func() {
	var (
		suiteSetup     actions.SuiteSetup
		defaultNetwork actions.NetworkInfo
		nodes          []client.Chainlink
		perfTest       Test
		err            error
	)

	BeforeEach(func() {
		By("Deploying the environment", func() {
			suiteSetup, err = actions.SingleNetworkSetup(
				environment.NewChainlinkCluster(5),
				hooks.EthereumPerfNetworkHook,
				hooks.EthereumDeployerHook,
				hooks.EthereumClientHook,
				utils.ProjectRoot,
			)
			Expect(err).ShouldNot(HaveOccurred())
			defaultNetwork = suiteSetup.DefaultNetwork()
			nodes, err = environment.GetChainlinkClients(suiteSetup.Environment())
			Expect(err).ShouldNot(HaveOccurred())
			defaultNetwork.Client.ParallelTransactions(true)
		})

		By("Funding the Chainlink nodes", func() {
			err := actions.FundChainlinkNodes(
				nodes,
				defaultNetwork.Client,
				defaultNetwork.Wallets.Default(),
				big.NewFloat(10),
				big.NewFloat(10),
			)
			Expect(err).ShouldNot(HaveOccurred())
		})

		By("Setting up the Keeper soak test", func() {
			perfTest = NewKeeperTest(
				KeeperTestOptions{
					TestOptions: TestOptions{
						NumberOfContracts: 5,
					},
					RoundTimeout:          4 * time.Minute,
					TestDuration:          10 * time.Minute,
					BlockCountPerTurn:     big.NewInt(1),
					PaymentPremiumPPB:     uint32(200000000),
					RegistryCheckGasLimit: uint32(2500000),
					StalenessSeconds:      big.NewInt(90000),
					GasCeilingMultiplier:  uint16(1),
				},
				suiteSetup.Environment(),
				defaultNetwork.Client,
				defaultNetwork.Wallets,
				defaultNetwork.Deployer,
				defaultNetwork.Link,
			)
			err = perfTest.Setup()
			Expect(err).ShouldNot(HaveOccurred())
		})
	})

	Describe("Keeper soak test", func() {
		Measure("Measure upkeeps duration", func(_ Benchmarker) {
			err = perfTest.Run()
			Expect(err).ShouldNot(HaveOccurred())
		}, 1)
	})

	AfterEach(func() {
		By("Tearing down the environment", suiteSetup.TearDown())
	})
})
