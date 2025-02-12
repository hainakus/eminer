package main

import (
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/hainakus/eminer/ethash"
	"github.com/hainakus/go-rethereum/common"
	"github.com/hainakus/go-rethereum/common/hexutil"
	"github.com/hainakus/go-rethereum/log"
)

// Benchmark mode
func Benchmark(stopChan chan struct{}) {
	deviceID := *flagbenchmark

	miner := ethash.NewCL([]int{int(deviceID)}, *flagworkername, *flaggcn, version)

	hh := common.BytesToHash(common.FromHex(randomHash()))
	sh := common.BytesToHash(common.FromHex("0x0000000000000000000000000000000000000000000000000000000000000000"))
	diff := new(big.Int).SetUint64(5e8) // 500 MH
	work := ethash.NewWork(45, hh, sh, new(big.Int).Div(ethash.MaxUint256, diff), *flagfixediff)

	miner.Work = work

	if *flagkernel != "" {
		miner.SetKernel(argToIntSlice(*flagkernel))
	}

	if *flagintensity != "" {
		miner.SetIntensity(argToIntSlice(*flagintensity))
	}

	err := miner.InitCL()
	if err != nil {
		log.Crit(fmt.Sprintf("OpenCL init error: %s", err.Error()))
		return
	}

	if *flagfan != "" {
		miner.SetFanPercent(argToIntSlice(*flagfan))
	}

	/*if *flagengineclock != "" {
		miner.SetEngineClock(argToIntSlice(*flagengineclock))
	}

	if *flagmemoryclock != "" {
		miner.SetMemoryClock(argToIntSlice(*flagmemoryclock))
	}*/

	stopReportHashRate := make(chan struct{})
	reportHashRate := func() {
		for {
			select {
			case <-stopReportHashRate:
				return
			case <-time.After(5 * time.Second):
				miner.Poll()
				log.Info("GPU device information", "device", deviceID,
					"hashrate", fmt.Sprintf("%.3f Mh/s", miner.GetHashrate(deviceID)/1e6),
					"temperature", fmt.Sprintf("%.2f C", miner.GetTemperature(deviceID)),
					"fan", fmt.Sprintf("%.2f%%", miner.GetFanPercent(deviceID)))
				log.Info("Mining global report", "solutions", miner.FoundSolutions.Count(), "rejected", miner.RejectedSolutions.Count(),
					"hashrate", fmt.Sprintf("%.3f Mh/s", miner.TotalHashRate()/1e6), "effectivehashrate", fmt.Sprintf("%.3f Mh/s", miner.SolutionsHashRate.RateMean()/1e6),
					"efficiency", fmt.Sprintf("%.2f%%", 100+(100*(miner.SolutionsHashRate.RateMean()-miner.TotalHashRate())/miner.TotalHashRate())))

			}
		}
	}

	var wg sync.WaitGroup

	stopSeal := make(chan struct{})
	seal := func() {
		wg.Add(1)
		defer wg.Done()

		onSolutionFound := func(hh common.Hash, nonce uint64, digest []byte, roundVariance uint64) {
			if nonce != 0 {
				log.Info("Solution accepted", "hash", hh.TerminalString(), "digest", common.Bytes2Hex(digest), "nonce", hexutil.Uint64(nonce).String())

				miner.FoundSolutions.Update(int64(roundVariance))
				if *flagfixediff {
					formatter := func(x int64) string {
						return fmt.Sprintf("%d%%", x)
					}

					log.Info("Solutions round variance", "count", miner.FoundSolutions.Count(), "last", formatter(int64(roundVariance)),
						"mean", formatter(int64(miner.FoundSolutions.Mean())), "min", formatter(miner.FoundSolutions.Min()),
						"max", formatter(miner.FoundSolutions.Max()))
				}
			}
		}

		go func() {
			for {
				time.Sleep(time.Second)
				miner.Work.HeaderHash = common.BytesToHash(common.FromHex(randomHash()))
				miner.WorkChanged()

				log.Info("Work changed, new work", "hash", miner.Work.HeaderHash.TerminalString(), "difficulty", fmt.Sprintf("%.3f GH", float64(miner.Work.Difficulty().Uint64())/1e9))
			}
		}()

		miner.Seal(stopSeal, 1, onSolutionFound)
	}

	log.Info("Starting benchmark", "seconds for", 600)

	go seal()
	go reportHashRate()

	miner.WorkChanged()

	if *flaghttp != "" && *flaghttp != "no" {
		httpServer.SetMiner(miner)
	}

	select {
	case <-stopChan:
		stopReportHashRate <- struct{}{}
		stopSeal <- struct{}{}

		wg.Wait()
		miner.Release(deviceID)

		log.Info("Benchmark aborted", "device", deviceID, "hashrate", fmt.Sprintf("%.3f Mh/s", miner.GetHashrate(deviceID)/1e6))

		return
	case <-time.After(600 * time.Second):
		stopReportHashRate <- struct{}{}
		stopSeal <- struct{}{}

		wg.Wait()
		miner.Release(deviceID)

		log.Info("Benchmark completed", "device", deviceID, "hashrate", fmt.Sprintf("%.3f Mh/s", miner.GetHashrate(deviceID)/1e6))

		stopChan <- struct{}{}
		return
	}
}
