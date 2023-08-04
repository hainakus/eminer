package client

import (
	"github.com/ethereum/go-ethereum/core/types"
	"math/big"
)

// Client interface
type Client interface {
	GetWork() ([]string, error)
	SubmitHashrate(params interface{}) (bool, error)
	SubmitWork(nonce string, blockHash string, mixHash string)
	SubmitWorkStr(params interface{}) (bool, error)
	GetDiff(number *big.Int) (string, *types.Header)
	GetDiffParent() (string, *types.Header)
}
