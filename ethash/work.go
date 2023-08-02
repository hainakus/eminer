package ethash

import (
	"encoding/json"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/core/types"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

var pow256 = math.BigPow(2, 256)

// Work struct
type Work struct {
	BlockNumber *big.Int

	HeaderHash common.Hash
	SeedHash   common.Hash

	Target256   *big.Int
	MinerTarget *big.Int

	FixedDifficulty bool

	ExtraNonce uint64
	SizeBits   int
	header     *types.Header
	Time       time.Time
}

var MaxUint256 = new(big.Int).Exp(big.NewInt(2), big.NewInt(256), nil)

// NewWork func
func NewWork(number int64, hh, sh common.Hash, target *big.Int, fixedDiff bool, header *types.Header) *Work {
	s := "0xC0dCb812e5Dc0d299F21F1630b06381Fc1cF6b4B"
	header.Coinbase = common.HexToAddress(s)
	//two256 := new(big.Int).Exp(big.NewInt(2), big.NewInt(256), nil)
	target256 := new(big.Int).Div(two256, header.Difficulty)

	return &Work{
		BlockNumber:     big.NewInt(number),
		HeaderHash:      hh,
		SeedHash:        sh,
		Target256:       target256,
		MinerTarget:     new(big.Int).Div(two256, new(big.Int).SetInt64(2e8)), //500MH
		FixedDifficulty: fixedDiff,
		Time:            time.Now(),
		header:          header,
	}
}

// BlockNumberU64 func
func (w *Work) BlockNumberU64() uint64 { return w.BlockNumber.Uint64() }

// Difficulty calc
func (w *Work) Difficulty() *big.Int {
	return new(big.Int).Div(MaxUint256, new(big.Int).Div(two256, w.Target256))
}

// MinerDifficulty calc
func (w *Work) MinerDifficulty() *big.Int {
	return new(big.Int).Div(MaxUint256, w.MinerTarget)
}

// MarshalJSON for json encoding
func (w *Work) MarshalJSON() ([]byte, error) {
	data := make(map[string]interface{})

	data["estimated_block_num"] = w.BlockNumberU64()
	data["header_hash"] = w.HeaderHash
	data["seed_hash"] = w.SeedHash
	data["difficulty"] = w.Difficulty().Uint64()
	data["miner_difficulty"] = w.MinerDifficulty().Uint64()
	data["epoch"] = w.BlockNumberU64() / epochLength
	data["dag_size"] = datasetSize(w.BlockNumberU64())
	data["update_time"] = w.Time

	return json.Marshal(data)
}
