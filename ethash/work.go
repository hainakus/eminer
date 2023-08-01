package ethash

import (
	"encoding/json"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

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

	Time time.Time
}

var MaxUint256 = new(big.Int).Exp(big.NewInt(2), big.NewInt(256), nil)

// NewWork func
func NewWork(number int64, hh, sh common.Hash, target *big.Int, fixedDiff bool) *Work {
	header, _ := GetWorkHead()
	target256 := new(big.Int).Div(two256, header.Difficulty)
	blockNumberMinusOne := new(big.Int).Sub(header.Number, common.Big1)
	parentBlock := GetParentBlock(blockNumberMinusOne)
	minerTarget := new(big.Int).Div(calcDifficultyFrontier(header.Time, parentBlock.Header()), big.NewInt(1000))
	return &Work{
		BlockNumber:     big.NewInt(number),
		HeaderHash:      hh,
		SeedHash:        sh,
		Target256:       target256,
		MinerTarget:     minerTarget, //500MH
		FixedDifficulty: fixedDiff,
		Time:            time.Now(),
	}
}

// BlockNumberU64 func
func (w *Work) BlockNumberU64() uint64 { return w.BlockNumber.Uint64() }

// Difficulty calc
func (w *Work) Difficulty() *big.Int {
	return new(big.Int).Div(MaxUint256, w.Target256)
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
