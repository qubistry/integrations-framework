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

var _ = Describe("VRF soak test @soak-vrf", func() {
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
				// more than one node is useless for VRF, because nodes are not cooperating for randomness
				environment.NewChainlinkCluster(1),
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

		By("Setting up the VRF soak test", func() {
			perfTest = NewVRFTest(
				VRFTestOptions{
					TestOptions: TestOptions{
						NumberOfContracts:    5,
						RoundTimeout:         60 * time.Second,
						TestDuration:         1 * time.Minute,
						GracefulStopDuration: 10 * time.Second,
					},
				},
				suiteSetup.Environment(),
				defaultNetwork.Link,
				defaultNetwork.Client,
				defaultNetwork.Wallets,
				defaultNetwork.Deployer,
			)
			err = perfTest.Setup()
			Expect(err).ShouldNot(HaveOccurred())
		})
	})

	Describe("VRF soak test", func() {
		Measure("Measure VRF request latency", func(b Benchmarker) {
			err = perfTest.Run()
			Expect(err).ShouldNot(HaveOccurred())
		}, 1)
	})

	AfterEach(func() {
		By("Tearing down the environment", suiteSetup.TearDown())
	})
})
