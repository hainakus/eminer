package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/goccy/go-json"
	"github.com/hainakus/eminer/util"
	"io/ioutil"
	"lukechampine.com/blake3"
	"net/http"
	"strings"
	"sync"
	"time"

	_ "github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/hainakus/eminer/client"
	"github.com/hainakus/eminer/ethash"
	"github.com/hainakus/eminer/rpc"
)

type Work struct {
	Header *types.Header
	Hash   string
}

type BlockData struct {
	BlockNonce uint64 `json:"nonce"`
	// Add other fields of the block if needed
}

const mixRounds = 64 // Number of mix rounds (simplified example)

// Simple mixing function
func mix(headerHash, mixHash []byte, nonce uint64) []byte {
	mixData := append(headerHash, mixHash...)
	nonceBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(nonceBytes, nonce)
	mixData = append(mixData, nonceBytes...)
	hash := blake3.Sum256(mixData)
	return hash[:]
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

				onSolutionFound := func(hh string, nonce string, digest []byte, roundVariance uint64) {

					// Output the final mix digest

					blockNonce := nonce
					mixDigest, _ := common.BytesToHash(digest).MarshalText()
					ri := blockNonce
					h := hh

					log.Error("err", string(ri))

					log.Error("err", string(mixDigest))
					log.Error("err", string(h))
					miner.FoundSolutions.Update(int64(roundVariance))
					if *flagfixediff {
						formatter := func(x int64) string {
							return fmt.Sprintf("%d%%", x)
						}

						log.Info("Solutions round variance", "count", miner.FoundSolutions.Count(), "last", formatter(int64(roundVariance)),
							"mean", formatter(int64(miner.FoundSolutions.Mean())), "min", formatter(miner.FoundSolutions.Min()),
							"max", formatter(miner.FoundSolutions.Max()))
					}

					c.SubmitWork(ri, h, string(mixDigest))

					hashrate := hexutil.Uint64(uint64(miner.TotalHashRate()))
					randomID := randomHash()
					params2 := []interface{}{
						hashrate,
						randomID,
					}
					c.SubmitHashrate(params2)

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

	//rpcUrl := "http://pool.rethereum.org:8888/0xC0dCb812e5Dc0d299F21F1630b06381Fc1cF6b4B/woo"
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

	//rpcUrl := "http://pool.rethereum.org:8888/0xC0dCb812e5Dc0d299F21F1630b06381Fc1cF6b4B/woo"
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
				hashrate := hexutil.Uint64(uint64(miner.TotalHashRate()))
				randomID := randomHash()
				params2 := []interface{}{
					hashrate,
					randomID,
				}
				rc.SubmitHashrate(params2)
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

		}
	}
}
