package smoke

import (
	"context"
	"fmt"
	"github.com/smartcontractkit/integrations-framework/hooks"
	"github.com/smartcontractkit/integrations-framework/utils"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/rs/zerolog/log"
	"github.com/smartcontractkit/integrations-framework/actions"
	"github.com/smartcontractkit/integrations-framework/client"
	"github.com/smartcontractkit/integrations-framework/contracts"
	"github.com/smartcontractkit/integrations-framework/environment"
)

var _ = Describe("Flux monitor suite @flux", func() {
	var (
		suiteSetup    actions.SuiteSetup
		networkInfo   actions.NetworkInfo
		mockserver    *client.MockserverClient
		nodes         []client.Chainlink
		nodeAddresses []common.Address
		fluxInstance  contracts.FluxAggregator
		err           error
	)
	fluxRoundTimeout := time.Minute * 2

	BeforeEach(func() {
		By("Deploying the environment", func() {
			suiteSetup, err = actions.SingleNetworkSetup(
				environment.NewChainlinkCluster(3),
				hooks.EVMNetworkFromConfigHook,
				hooks.EthereumDeployerHook,
				hooks.EthereumClientHook,
				utils.ProjectRoot,
			)
			Expect(err).ShouldNot(HaveOccurred())
			nodes, err = environment.GetChainlinkClients(suiteSetup.Environment())
			Expect(err).ShouldNot(HaveOccurred())
			mockserver, err = environment.GetMockserverClientFromEnv(suiteSetup.Environment())
			Expect(err).ShouldNot(HaveOccurred())
			networkInfo = suiteSetup.DefaultNetwork()

			networkInfo.Client.ParallelTransactions(true)
		})

		By("Deploying and funding contract", func() {
			fluxInstance, err = networkInfo.Deployer.DeployFluxAggregatorContract(networkInfo.Wallets.Default(), contracts.DefaultFluxAggregatorOptions())
			Expect(err).ShouldNot(HaveOccurred())
			err = fluxInstance.Fund(networkInfo.Wallets.Default(), nil, big.NewFloat(1))
			Expect(err).ShouldNot(HaveOccurred())
			err = fluxInstance.UpdateAvailableFunds(context.Background(), networkInfo.Wallets.Default())
			Expect(err).ShouldNot(HaveOccurred())
			err = networkInfo.Client.WaitForEvents()
			Expect(err).ShouldNot(HaveOccurred())
		})

		By("Funding Chainlink nodes", func() {
			nodeAddresses, err = actions.ChainlinkNodeAddresses(nodes)
			Expect(err).ShouldNot(HaveOccurred())
			ethAmount, err := networkInfo.Deployer.CalculateETHForTXs(networkInfo.Wallets.Default(), networkInfo.Network.Config(), 3)
			Expect(err).ShouldNot(HaveOccurred())
			err = actions.FundChainlinkNodes(
				nodes,
				networkInfo.Client,
				networkInfo.Wallets.Default(),
				ethAmount,
				nil,
			)
			Expect(err).ShouldNot(HaveOccurred())
		})

		By("Setting oracle options", func() {
			err = fluxInstance.SetOracles(networkInfo.Wallets.Default(),
				contracts.FluxAggregatorSetOraclesOptions{
					AddList:            nodeAddresses,
					RemoveList:         []common.Address{},
					AdminList:          nodeAddresses,
					MinSubmissions:     3,
					MaxSubmissions:     3,
					RestartDelayRounds: 0,
				})
			Expect(err).ShouldNot(HaveOccurred())
			err = networkInfo.Client.WaitForEvents()
			Expect(err).ShouldNot(HaveOccurred())
			oracles, err := fluxInstance.GetOracles(context.Background())
			Expect(err).ShouldNot(HaveOccurred())
			log.Info().Str("Oracles", strings.Join(oracles, ",")).Msg("Oracles set")
		})

		By("Creating flux jobs", func() {
			err = mockserver.SetVariable(0)
			Expect(err).ShouldNot(HaveOccurred())

			bta := client.BridgeTypeAttributes{
				Name: "variable",
				URL:  fmt.Sprintf("%s/variable", mockserver.Config.ClusterURL),
			}
			for _, n := range nodes {
				err = n.CreateBridge(&bta)
				Expect(err).ShouldNot(HaveOccurred())

				fluxSpec := &client.FluxMonitorJobSpec{
					Name:              "flux_monitor",
					ContractAddress:   fluxInstance.Address(),
					PollTimerPeriod:   15 * time.Second, // min 15s
					PollTimerDisabled: false,
					ObservationSource: client.ObservationSourceSpecBridge(bta),
				}
				_, err = n.CreateJob(fluxSpec)
				Expect(err).ShouldNot(HaveOccurred())
			}
		})
	})

	Describe("with Flux job", func() {
		It("performs two rounds and has withdrawable payments for oracles", func() {
			err = mockserver.SetVariable(1e7)
			Expect(err).ShouldNot(HaveOccurred())

			fluxRound := contracts.NewFluxAggregatorRoundConfirmer(fluxInstance, big.NewInt(2), fluxRoundTimeout)
			networkInfo.Client.AddHeaderEventSubscription(fluxInstance.Address(), fluxRound)
			err = networkInfo.Client.WaitForEvents()
			Expect(err).ShouldNot(HaveOccurred())

			data, err := fluxInstance.GetContractData(context.Background())
			Expect(err).ShouldNot(HaveOccurred())
			log.Info().Interface("data", data).Msg("Round data")
			Expect(len(data.Oracles)).Should(Equal(3))
			Expect(data.LatestRoundData.Answer.Int64()).Should(Equal(int64(1e7)))
			Expect(data.LatestRoundData.RoundId.Int64()).Should(Equal(int64(2)))
			Expect(data.LatestRoundData.AnsweredInRound.Int64()).Should(Equal(int64(2)))
			Expect(data.AvailableFunds.Int64()).Should(Equal(int64(999999999999999994)))
			Expect(data.AllocatedFunds.Int64()).Should(Equal(int64(6)))

			err = mockserver.SetVariable(1e8)
			Expect(err).ShouldNot(HaveOccurred())

			fluxRound = contracts.NewFluxAggregatorRoundConfirmer(fluxInstance, big.NewInt(3), fluxRoundTimeout)
			networkInfo.Client.AddHeaderEventSubscription(fluxInstance.Address(), fluxRound)
			err = networkInfo.Client.WaitForEvents()
			Expect(err).ShouldNot(HaveOccurred())

			data, err = fluxInstance.GetContractData(context.Background())
			Expect(err).ShouldNot(HaveOccurred())
			Expect(len(data.Oracles)).Should(Equal(3))
			Expect(data.LatestRoundData.Answer.Int64()).Should(Equal(int64(1e8)))
			Expect(data.LatestRoundData.RoundId.Int64()).Should(Equal(int64(3)))
			Expect(data.LatestRoundData.AnsweredInRound.Int64()).Should(Equal(int64(3)))
			Expect(data.AvailableFunds.Int64()).Should(Equal(int64(999999999999999991)))
			Expect(data.AllocatedFunds.Int64()).Should(Equal(int64(9)))
			log.Info().Interface("data", data).Msg("Round data")

			for _, oracleAddr := range nodeAddresses {
				payment, _ := fluxInstance.WithdrawablePayment(context.Background(), oracleAddr)
				Expect(payment.Int64()).Should(Equal(int64(3)))
			}
		})
	})

	AfterEach(func() {
		By("Printing gas stats", func() {
			networkInfo.Client.GasStats().PrintStats()
		})
		By("Tearing down the environment", suiteSetup.TearDown())
	})
})
