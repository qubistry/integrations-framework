package actions

import (
	"context"
	"fmt"
	"github.com/rs/zerolog/log"
	"github.com/smartcontractkit/integrations-framework/client"
	"github.com/smartcontractkit/integrations-framework/contracts"
	"math/big"
	"sort"
	"sync"
)

func GetRoundStartTimesAcrossNodes(
	contractAddr string,
	jobs []contracts.JobByInstance,
	nodesByHostPort map[string]client.Chainlink,
	mu *sync.Mutex,
	contractsStartTimes map[string]int64,
) func() error {
	return func() error {
		startTimesAcrossNodes := make([]int64, 0)
		for _, j := range jobs {
			// get node for a job
			node := nodesByHostPort[j.Instance]
			runs, err := node.ReadRunsByJob(j.ID)
			if err != nil {
				return err
			}
			runsStartTimes := make([]int64, 0)
			for _, r := range runs.Data {
				runsStartTimes = append(runsStartTimes, r.Attributes.CreatedAt.UnixNano()) // milliseconds
			}
			sort.SliceStable(runsStartTimes, func(i, j int) bool {
				return runsStartTimes[i] > runsStartTimes[j]
			})
			log.Debug().Interface("Start times array", runsStartTimes).Send()
			lastRunStartTime := runsStartTimes[0]
			startTimesAcrossNodes = append(startTimesAcrossNodes, lastRunStartTime)
		}
		mu.Lock()
		defer mu.Unlock()
		// earliest start across nodes for contract
		sort.SliceStable(startTimesAcrossNodes, func(i, j int) bool {
			return startTimesAcrossNodes[i] < startTimesAcrossNodes[j]
		})
		contractsStartTimes[contractAddr] = startTimesAcrossNodes[0]
		return nil
	}
}

func GetRoundCompleteTimestamps(
	fi contracts.FluxAggregator,
	roundID int,
	submissions int,
	submissionVal *big.Int,
	mu *sync.Mutex,
	endTimes map[string]int64,
	client client.BlockchainClient,
) func() error {
	return func() error {
		events, err := fi.FilterRoundSubmissions(context.Background(), submissionVal, roundID)
		if err != nil {
			return err
		}
		for _, e := range events {
			log.Debug().Uint64("Block number in event", e.BlockNumber).Send()
			hTime, err := client.HeaderTimestampByNumber(context.Background(), big.NewInt(int64(e.BlockNumber)))
			if err != nil {
				return err
			}
			log.Debug().Uint64("Block time in event", hTime).Send()
		}
		if len(events) == submissions {
			lastEvent := events[len(events)-1]
			hTime, err := client.HeaderTimestampByNumber(context.Background(), big.NewInt(int64(lastEvent.BlockNumber)))
			if err != nil {
				return err
			}
			log.Debug().
				Str("Contract", fi.Address()).
				Uint64("Header timestamp", hTime).
				Msg("All submissions found")
			mu.Lock()
			defer mu.Unlock()
			// nanoseconds
			endTimes[fi.Address()] = int64(hTime) * 1e9
			return nil
		}
		return fmt.Errorf("not all submissions found for contract: %s", fi.Address())
	}
}
