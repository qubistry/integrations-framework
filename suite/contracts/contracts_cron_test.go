package suite

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/smartcontractkit/integrations-framework/client"
	"github.com/smartcontractkit/integrations-framework/contracts"
	"github.com/smartcontractkit/integrations-framework/suite"
	"github.com/smartcontractkit/integrations-framework/tools"
)

var _ = FDescribe("Flux monitor suite", func() {
	DescribeTable("use cron job", func(
		initFunc client.BlockchainNetworkInit,
		ocrOptions contracts.OffchainOptions,
	) {
		_, err := suite.DefaultLocalSetup(initFunc)
		Expect(err).ShouldNot(HaveOccurred())
		chainlinkNodes, _, err := suite.ConnectToTemplateNodes()
		Expect(err).ShouldNot(HaveOccurred())

		adapter := tools.NewExternalAdapter()

		_, err = chainlinkNodes[0].CreateJob(&client.CronJobSpec{
			Schedule:          "CRON_TZ=UTC * * * * * *",
			ObservationSource: client.ObservationSourceSpec(adapter.InsideDockerAddr + "/five"),
		})
		Expect(err).ShouldNot(HaveOccurred())
		//time.Sleep(999*time.Second)
		//chainlinkNodes[0].ReadJob(job.Data.ID)
	},
		Entry("on Ethereum Hardhat", client.NewHardhatNetwork, contracts.DefaultOffChainAggregatorOptions()),
	)
})
