package main

import (
	"bytes"
	"fmt"
	"github.com/goccy/go-json"
	"github.com/hainakus/eminer/util"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/hainakus/eminer/client"
	"github.com/hainakus/eminer/ethash"
	"github.com/hainakus/eminer/rpc"
	"github.com/hainakus/go-rethereum/common"
	_ "github.com/hainakus/go-rethereum/common/hexutil"
	"github.com/hainakus/go-rethereum/core/types"
	"github.com/hainakus/go-rethereum/log"
)

type Work struct {
	Header *types.Header
	Hash   string
}

type BlockData struct {
	BlockNonce uint64 `json:"nonce"`
	// Add other fields of the block if needed
}

func farmMineByDevice(miner *ethash.OpenCLMiner, deviceID int, c client.Client, stopChan <-chan struct{}) {
	stopSealFunc := make(chan struct{})
	stopSeal := make(chan struct{})

	seal := func() {
		for {
			select {
			case <-stopSealFunc:
				return
			case <-time.After(time.Second):

				onSolutionFound := func(hh common.Hash, nonce uint64, digest []byte, roundVariance uint64) {
					blockNonce := types.EncodeNonce(nonce)

					ri, _ := blockNonce.MarshalText()
					h, _ := hh.MarshalText()
					r, _ := common.BytesToHash(digest).MarshalText()
					params := []interface{}{
						string(ri),
						string(h),
						string(r),
					}
					log.Error("err", string(ri))
					log.Error("err", string(h))
					log.Error("err", string(r))
					miner.FoundSolutions.Update(int64(roundVariance))
					if *flagfixediff {
						formatter := func(x int64) string {
							return fmt.Sprintf("%d%%", x)
						}

						log.Info("Solutions round variance", "count", miner.FoundSolutions.Count(), "last", formatter(int64(roundVariance)),
							"mean", formatter(int64(miner.FoundSolutions.Mean())), "min", formatter(miner.FoundSolutions.Min()),
							"max", formatter(miner.FoundSolutions.Max()))
					}

					res, err := c.SubmitWork(params)
					if res && err == nil {
						log.Info("Solution accepted by network", "hash", hh.TerminalString())
					} else {
						miner.RejectedSolutions.Inc(1)
						log.Warn("Solution not accepted by network", "hash", hh.TerminalString(), "error", err.Error())
					}
				}

				miner.Seal(stopSeal, deviceID, onSolutionFound)
			}
		}
	}

	go seal()

	<-stopChan

	stopSeal <- struct{}{}
	stopSealFunc <- struct{}{}
}

type RpcReback struct {
	Jsonrpc string   `json:"jsonrpc"`
	Result  []string `json:"result"`
	Id      int      `json:"id"`
}

func GetWorkHead() (*types.Header, string) {
	getWorkInfo := RpcInfo{Method: "eth_getWork", Params: []string{}, Id: 1, Jsonrpc: "2.0"}
	getWorkInfoBuffs, _ := json.Marshal(getWorkInfo)

	rpcUrl := "http://213.22.47.84:8545"
	req, err := http.NewRequest("POST", rpcUrl, bytes.NewBuffer(getWorkInfoBuffs))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, ""
	}
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)
	workReback := new(RpcReback)

	json.Unmarshal(body, workReback)

	newHeader := new(types.Header)
	newHeader.Number = util.HexToBig(workReback.Result[3])
	newHeader.Difficulty = util.TargetHexToDiff(workReback.Result[2])

	return newHeader, workReback.Result[0]
}

type RpcInfo struct {
	Method  string
	Params  []string
	Id      int
	Jsonrpc string
}

func SubmitWork(nonce string, blockHash string, mixHash string) {
	getWorkInfo := RpcInfo{Method: "eth_submitWork", Params: []string{nonce, blockHash, mixHash}, Id: 1, Jsonrpc: "2.0"}
	log.Info("Submit work:", getWorkInfo.Params)
	getWorkInfoBuffs, _ := json.Marshal(getWorkInfo)

	rpcUrl := "http://213.22.47.84:8545"
	req, err := http.NewRequest("POST", rpcUrl, bytes.NewBuffer(getWorkInfoBuffs))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Error("error", err)
	}
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)

	log.Info("Submit reback", string(body))

}

// Farm mode
func Farm(stopChan <-chan struct{}) {
	defer func() {
		if r := recover(); r != nil {
			stack := stack(3)
			log.Error(fmt.Sprintf("Recovered in farm mode -> %s\n%s\n", r, stack))
			go Farm(stopChan)
		}
	}()

	rc, err := rpc.New(*flagfarm, 2*time.Second)
	if err != nil {
		log.Crit(err.Error())
	}

	w, err := getWork(rc)
	if err != nil {
		if strings.Contains(err.Error(), "No work available") {
			log.Warn("No work available on network, will try 5 sec later")
			time.Sleep(5 * time.Second)
			go Farm(stopChan)
			return
		}
		log.Crit(err.Error())
	}

	var deviceIds []int
	if *flagmine == "all" {
		deviceIds = getAllDevices()
	} else {
		deviceIds = argToIntSlice(*flagmine)
	}

	miner := ethash.NewCL(deviceIds, *flagworkername, *flaggcn, version)

	miner.Lock()
	miner.Work = w
	miner.Unlock()

	if *flagkernel != "" {
		miner.SetKernel(argToIntSlice(*flagkernel))
	}

	if *flagintensity != "" {
		miner.SetIntensity(argToIntSlice(*flagintensity))
	}

	err = miner.InitCL()
	if err != nil {
		log.Crit("OpenCL init not completed", "error", err.Error())
	}

	if *flagfan != "" {
		miner.SetFanPercent(argToIntSlice(*flagfan))
	}

	if *flagengineclock != "" {
		miner.SetEngineClock(argToIntSlice(*flagengineclock))
	}

	if *flagmemoryclock != "" {
		miner.SetMemoryClock(argToIntSlice(*flagmemoryclock))
	}

	changeDAG := make(chan struct{})

	stopCheckNewWork := make(chan struct{})
	checkNewWork := func() {
		for {
			select {
			case <-stopCheckNewWork:
				return
			case <-time.After(250 * time.Millisecond):
				wt, errc := getWork(rc)
				if errc != nil {
					log.Error("Get work error", "error", errc.Error())
					continue
				}

				if !bytes.Equal(wt.HeaderHash.Bytes(), miner.Work.HeaderHash.Bytes()) {
					log.Info("Work changed, new work", "hash", wt.HeaderHash.TerminalString(), "difficulty",
						fmt.Sprintf("%.3f GH", float64(wt.Difficulty().Uint64())/1e9))

					miner.Lock()
					miner.Work = wt
					miner.Unlock()

					miner.WorkChanged()
				}
			}
		}
	}

	stopShareInfo := make(chan struct{})
	shareInfo := func() {
		loops := 0
		for {
			select {
			case <-stopShareInfo:
				return
			case <-time.After(30 * time.Second):
				miner.Poll()
				for _, deviceID := range deviceIds {
					log.Info("GPU device information", "device", deviceID,
						"hashrate", fmt.Sprintf("%.3f Mh/s", miner.GetHashrate(deviceID)/1e6),
						"temperature", fmt.Sprintf("%.2f C", miner.GetTemperature(deviceID)),
						"fan", fmt.Sprintf("%.2f%%", miner.GetFanPercent(deviceID)))
				}

				loops++
				if (loops % 6) == 0 {
					log.Info("Mining global report", "solutions", miner.FoundSolutions.Count(), "rejected", miner.RejectedSolutions.Count(),
						"hashrate", fmt.Sprintf("%.3f Mh/s", miner.TotalHashRate()/1e6))
				}
			}
		}
	}

	stopReportHashRate := make(chan struct{})
	reportHashRate := func() {

		for {
			select {
			case <-stopReportHashRate:
				return
			case <-time.After(30 * time.Second):

			}
		}
	}

	log.Info("New work from network", "hash", w.HeaderHash.TerminalString(), "difficulty", fmt.Sprintf("%.3f GH", float64(w.Difficulty().Uint64())/1e9))
	log.Info("Starting mining process", "hash", miner.Work.HeaderHash.TerminalString())

	var wg sync.WaitGroup
	stopFarmMine := make(chan struct{}, len(deviceIds))
	for _, deviceID := range deviceIds {
		wg.Add(1)
		go func(deviceID int) {
			defer wg.Done()

			farmMineByDevice(miner, deviceID, rc, stopFarmMine)
		}(deviceID)
	}

	go checkNewWork()
	go shareInfo()
	go reportHashRate()

	if *flaghttp != "" && *flaghttp != "no" {
		httpServer.SetMiner(miner)
	}

	miner.WorkChanged()
	miner.Poll()

	for {
		select {
		case <-stopChan:
			stopCheckNewWork <- struct{}{}
			stopShareInfo <- struct{}{}
			stopReportHashRate <- struct{}{}

			for range deviceIds {
				stopFarmMine <- struct{}{}
			}

			wg.Wait()
			miner.Destroy()
			return
		case <-changeDAG:
			for range deviceIds {
				stopFarmMine <- struct{}{}
			}

			wg.Wait()

			err = miner.ChangeDAGOnAllDevices()
			if err != nil {
				if strings.Contains(err.Error(), "Device memory may be insufficient") {
					log.Crit("Generate DAG failed", "error", err.Error())
				}

				log.Error("Generate DAG failed", "error", err.Error())

				miner.Destroy()
				httpServer.ClearStats()

				go Farm(stopChan)
				return
			}

			miner.Resume()

			farmMineByDevice(miner, 1, rc, stopFarmMine)
			farmMineByDevice(miner, 2, rc, stopFarmMine)
			farmMineByDevice(miner, 3, rc, stopFarmMine)
		}
	}
}
