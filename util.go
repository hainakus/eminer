package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"ethashGpu/ethash"
	"fmt"
	common2 "github.com/ethereumproject/go-ethereum/common"
	"github.com/ethereumproject/go-ethereum/crypto/sha3"
	"hash"
	"math/big"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"ethashGpu/client"
	"github.com/ethash/go-opencl/cl"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
)

const (
	datasetInitBytes   = 1 << 30 // Bytes in dataset at genesis
	datasetGrowthBytes = 1 << 23 // Dataset growth per epoch
	cacheInitBytes     = 1 << 24 // Bytes in cache at genesis
	cacheGrowthBytes   = 1 << 17 // Cache growth per epoch
	epochLength        = 30000   // Blocks per epoch
	mixBytes           = 128     // Width of mix
	hashBytes          = 64      // Hash length in bytes
	hashWords          = 16      // Number of 32 bit ints in a hash
	datasetParents     = 256     // Number of parents of each dataset element
	cacheRounds        = 3       // Number of rounds in cache production
	loopAccesses       = 64      // Number of accesses in hashimoto loop
)

func argToIntSlice(arg string) (devices []int) {
	deviceList := strings.Split(arg, ",")

	for _, device := range deviceList {
		deviceID, _ := strconv.Atoi(device)
		devices = append(devices, deviceID)
	}

	return
}

func getAllDevices() (devices []int) {
	platforms, err := cl.GetPlatforms()
	if err != nil {
		log.Crit(fmt.Sprintf("Plaform error: %v\nCheck your OpenCL installation and then run eminer -L", err))
		return
	}

	platformMap := make(map[string]bool, len(platforms))

	found := 0
	for _, p := range platforms {
		// check duplicate platform, sometimes found duplicate platforms
		if platformMap[p.Vendor()] {
			continue
		}

		ds, err := cl.GetDevices(p, cl.DeviceTypeGPU)
		if err != nil {
			continue
		}

		platformMap[p.Vendor()] = true

		for _, d := range ds {
			if !strings.Contains(d.Vendor(), "AMD") &&
				!strings.Contains(d.Vendor(), "Advanced Micro Devices") &&
				!strings.Contains(d.Vendor(), "NVIDIA") {
				continue
			}

			devices = append(devices, found)

			found++
		}
	}

	return
}

func randomHash() string {
	rand.Seed(time.Now().UnixNano())
	token := make([]byte, 32)
	rand.Read(token)

	return common2.ToHex(token)
}

func randomString(n int) string {
	rand.Seed(time.Now().UnixNano())

	var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

type hasher func(dest []byte, data []byte)

func makeHasher(h hash.Hash) hasher {
	return func(dest []byte, data []byte) {
		h.Write(data)
		h.Sum(dest[:0])
		h.Reset()
	}
}

func number(seedHash common.Hash) (int64, error) {
	var epoch uint64
	find := make([]byte, 32)
	seed := seedHash.Bytes()

	if bytes.Equal(find, seed) {
		return 0, nil
	}

	keccak256 := makeHasher(sha3.NewKeccak256())
	for epoch = 1; epoch < 2048; epoch++ {
		keccak256(find, find)
		if bytes.Equal(seed, find) {
			return int64(epoch * epochLength), nil
		}
	}

	if epoch == 2048 {
		return -1, fmt.Errorf("apparent block number for seed %s", seedHash.String())
	}

	return -1, fmt.Errorf("cant find block number in epoch for seed %s", seedHash.String())
}
func Number(seedHash common.Hash) (int64, error) {
	return number(seedHash)
}
func notifyWork(result *json.RawMessage) (*ethash.Work, error) {
	var blockNumber int64
	var getWork []string
	err := json.Unmarshal(*result, &getWork)
	if err != nil {
		return nil, err
	}

	if len(getWork) < 3 {
		return nil, errors.New("result short")
	}

	seedHash := common.BytesToHash(common.FromHex(getWork[1]))

	blockNumber, err = Number(seedHash)
	if err != nil {
		return nil, err
	}

	w := ethash.NewWork(blockNumber, common.Hash(common2.BytesToHash(common.FromHex(getWork[0]))),
		seedHash, new(big.Int).SetBytes(common.FromHex(getWork[2])), *flagfixediff)

	if len(getWork) > 4 { //extraNonce
		w.ExtraNonce = new(big.Int).SetBytes(common.FromHex(getWork[3])).Uint64()
		w.SizeBits, _ = strconv.Atoi(getWork[4])
	}

	return w, nil
}

func getWork(c client.Client) (*ethash.Work, error) {
	var blockNumber int64

	getWork, err := c.GetWork()
	if err != nil {
		return nil, err
	}

	seedHash := common.BytesToHash(common.FromHex(getWork[1]))

	blockNumber, err = Number(seedHash)
	if err != nil {
		return nil, err
	}

	w := ethash.NewWork(blockNumber, common.BytesToHash(common.FromHex(getWork[0])),
		seedHash, new(big.Int).SetBytes(common.FromHex(getWork[2])), *flagfixediff)

	if len(getWork) > 4 { //extraNonce
		w.ExtraNonce = new(big.Int).SetBytes(common.FromHex(getWork[3])).Uint64()
		w.SizeBits, _ = strconv.Atoi(getWork[4])
	}

	return w, nil
}
