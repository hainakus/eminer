package client

import "github.com/ethereum/go-ethereum/core/types"

// Client interface
type Client interface {
	GetWork() (*types.Header, string)
	SubmitHashrate(params interface{}) (bool, error)
	SubmitWork(params []string) (bool, error)
	SubmitWorkStr(params interface{}) (bool, error)
}
