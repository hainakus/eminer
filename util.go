package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	common2 "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
	"golang.org/x/crypto/sha3"
	"hash"
	"lukechampine.com/blake3"
	"math/big"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/ethash/go-opencl/cl"
	"github.com/ethereum/go-ethereum/common"

	"github.com/ethereum/go-ethereum/log"
	"github.com/hainakus/eminer/client"
	"github.com/hainakus/eminer/ethash"
)

const (
	datasetInitBytes   = 1 << 30 // Bytes in dataset at genesis
	datasetGrowthBytes = 1 << 23 // Dataset growth per epoch
	cacheInitBytes     = 1 << 24 // Bytes in cache at genesis
	cacheGrowthBytes   = 1 << 17 // Cache growth per epoch
	epochLength        = 32000   // Blocks per epoch
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
		log.Crit(fmt.Sprintf("Plaform error: %v\nCheck your OpenCL installation and then run unknownminer -L", err))
		return
	}

	platformMap := make(map[string]bool, len(platforms))

	found := 0
	for _, p := range platforms {
		// check duplicate platform, sometimes found duplicate platforms
		if platformMap[p.Vendor()] {
			continue
		}

		ds, err := platforms[0].GetDevices(cl.DeviceTypeGPU)
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

	return common2.Bytes2Hex(token)
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

	keccak256 := makeHasher(sha3.NewLegacyKeccak256())
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
	var blockNumber *big.Int

	header, _ := GetWorkHead()

	blockNumber = header.Number
	seedHash, _ := GetSeedHash(blockNumber.Uint64())
	sealHash := SealHash(header)
	w := ethash.NewWork(blockNumber.Int64(), sealHash,
		common.BytesToHash(seedHash), new(big.Int).Div(new(big.Int).Exp(big.NewInt(2), big.NewInt(256), big.NewInt(0)), header.Difficulty), *flagfixediff, header)

	//log.Info(strconv.FormatInt(blockNumber, 10))
	return w, nil
}

func getWork(c client.Client) (*ethash.Work, error) {
	var blockNumber *big.Int

	header, _ := c.GetWork()

	blockNumber = header.Number
	seedHash, _ := GetSeedHash(blockNumber.Uint64())
	sealHash := SealHash(header)
	w := ethash.NewWork(blockNumber.Int64(), sealHash,
		common.BytesToHash(seedHash), new(big.Int).Div(new(big.Int).Exp(big.NewInt(2), big.NewInt(256), big.NewInt(0)), header.Difficulty), *flagfixediff, header)

	//log.Info(strconv.FormatInt(blockNumber, 10))
	return w, nil
}
func SealHash(header *types.Header) (hash common.Hash) {
	hasher := sha3.NewLegacyKeccak256()

	enc := []interface{}{
		header.ParentHash,
		header.UncleHash,
		header.Coinbase,
		header.Root,
		header.TxHash,
		header.ReceiptHash,
		header.Bloom,
		header.Difficulty,
		header.Number,
		header.GasLimit,
		header.GasUsed,
		header.Time,
		header.Extra,
	}
	if header.BaseFee != nil {
		enc = append(enc, header.BaseFee)
	}
	rlp.Encode(hasher, enc)
	hasher.Sum(hash[:0])
	return hash
}
func GetSeedHash(blockNum uint64) ([]byte, error) {
	if blockNum >= epochLength*2048 {
		return nil, fmt.Errorf("block number too high, limit is %d", epochLength*2048)
	}
	sh := makeSeedHash(blockNum / epochLength)
	return sh[:], nil
}

func makeSeedHash(epoch uint64) (sh common.Hash) {
	for ; epoch > 0; epoch-- {
		b64 := blake3.Sum512(sh[:])
		sh = common.BytesToHash(b64[:])
	}
	return sh
}
