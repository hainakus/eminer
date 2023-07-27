package rpc

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/hainakus/go-rethereum/core/types"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hainakus/go-rethereum/log"
)

var (
	rpcUrl string = "http://213.22.47.84:8545"
)

type RpcReback struct {
	Jsonrpc string   `json:"jsonrpc"`
	Result  []string `json:"result"`
	Id      int      `json:"id"`
}

type RpcInfo struct {
	Jsonrpc string   `json:"jsonrpc"`
	Method  string   `json:""`
	Params  []string `json:"params"`
	Id      int      `json:"id"`
}

type Work struct {
	Header *types.Header
	Hash   string
}

// Client struct
type Client struct {
	sync.RWMutex

	URL        *url.URL
	backupURLs []*url.URL
	urlIdx     int

	client *http.Client

	sick        bool
	sickRate    int
	successRate int
	FailsCount  uint64

	timeout time.Duration
}

func (r *Client) SubmitWorkStr(params interface{}) (bool, error) {
	//TODO implement me
	panic("implement me")
}

func SubmitWork(params interface{}) (bool, error) {
	//TODO implement me
	panic("implement me")
}

// GetBlockReply struct
type GetBlockReply struct {
	Number     string `json:"number"`
	Difficulty string `json:"difficulty"`
}

// JSONRpcResp struct
type JSONRpcResp struct {
	ID     *json.RawMessage       `json:"id"`
	Result *json.RawMessage       `json:"result"`
	Error  map[string]interface{} `json:"error"`
}

// New func
func New(rawURLs string, timeout time.Duration) (*Client, error) {
	c := new(Client)

	splitURLs := strings.Split(rawURLs, ",")

	if len(splitURLs) > 0 {
		for _, rawURL := range splitURLs {
			url, err := url.Parse(rawURL)
			if err != nil {
				log.Error("Error parse url", "url", url, "error", err.Error())
			}

			c.backupURLs = append(c.backupURLs, url)
		}
	}

	if len(c.backupURLs) == 0 {
		return nil, errors.New("No URL found")
	}

	c.URL = c.backupURLs[c.urlIdx]
	c.timeout = timeout

	c.client = &http.Client{
		Timeout: c.timeout,
	}

	return c, nil
}

// GetWork func

// SubmitWork func
func (r *Client) SubmitWork(params []interface{}) (bool, error) {
	fmt.Printf("Params: %v\n", params)
	rpcResp, err := r.doPost("eth_submitWork", params, 1)
	var result bool
	if err != nil {
		log.Warn("[JSON-RPC] Submit work error", "error", err.Error())
		return true, nil //network error never rejected
	}

	if rpcResp.Error != nil {
		return false, errors.New("[JSON-RPC] " + rpcResp.Error["message"].(string))
	}

	json.Unmarshal(*rpcResp.Result, &result)
	if !result {
		return false, errors.New("[JSON-RPC] " + "rejected without reason")
	}
	return result, nil
	//paramStrings := make([]string, len(params))
	//for i, param := range params {
	//	switch v := param.(type) {
	//	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
	//		paramStrings[i] = fmt.Sprintf("%x", v)
	//	case string:
	//		paramStrings[i] = v
	//	case common.Hash:
	//		paramStrings[i] = v.Hex()
	//	default:
	//		paramStrings[i] = fmt.Sprintf("%x", v)
	//	}
	//}
	//nonce := paramStrings[0]
	//blockHash := paramStrings[1]
	//mixHash := paramStrings[2]
	//getWorkInfo := RpcInfo{Method: "eth_submitWork", Params: []string{nonce, blockHash, mixHash}, Id: 1, Jsonrpc: "2.0"}
	//fmt.Println("Submit work:", getWorkInfo.Params)
	//getWorkInfoBuffs, _ := json.Marshal(getWorkInfo)
	//
	//rpcUrl := "http://213.22.47.84:8545"
	//req, err := http.NewRequest("POST", rpcUrl, bytes.NewBuffer(getWorkInfoBuffs))
	//req.Header.Set("Content-Type", "application/json")
	//
	//client := &http.Client{}
	//resp, err := client.Do(req)
	//if err != nil {
	//	log.Error("error", err)
	//}
	//defer resp.Body.Close()
	//body, _ := ioutil.ReadAll(resp.Body)
	//
	//fmt.Println("Submit reback", string(body))
	//
	//return false, nil
}

// SubmitHashrate func
func (r *Client) SubmitHashrate(params interface{}) (bool, error) {
	rpcResp, err := r.doPost("eth_submitHashrate", params, 1)
	var result bool
	if err != nil {
		return false, nil
	}
	if rpcResp.Error != nil {
		return false, nil
	}

	json.Unmarshal(*rpcResp.Result, &result)

	return result, nil
}

func (r *Client) doPost(method string, params interface{}, id uint64) (JSONRpcResp, error) {
	if r.Sick() && len(r.backupURLs) > r.urlIdx+1 {
		log.Warn("RPC server sick", "url", r.URL.String())

		r.URL = r.backupURLs[r.urlIdx+1]
		r.urlIdx++

		if r.urlIdx+1 == len(r.backupURLs) {
			r.urlIdx = -1
		}

		log.Info("Trying another RPC server", "url", r.URL.String())

		//clear stats
		r.Lock()
		r.sick = false
		r.sickRate = 0
		r.successRate = 0
		r.Unlock()
	}

	var rpcResp JSONRpcResp

	jsonReq := map[string]interface{}{"jsonrpc": "2.0", "id": 1, "method": method, "params": params}

	data, _ := json.Marshal(jsonReq)
	req, err := http.NewRequest("POST", r.URL.String(), bytes.NewBuffer(data))
	if err != nil {
		r.markSick()
		return rpcResp, errors.New("[JSON-RPC] " + err.Error())
	}

	req.Header.Set("Content-Length", (string)(len(data)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := r.client.Do(req)

	if err != nil {
		r.markSick()
		return rpcResp, errors.New("[JSON-RPC] " + err.Error())
	}
	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)
	err = json.Unmarshal(body, &rpcResp)

	if rpcResp.Error != nil {
		r.markSick()
	}

	if err != nil {
		return rpcResp, errors.New("[JSON-RPC] " + err.Error())
	}

	return rpcResp, nil
}

// Check func
func (r *Client) Check() (bool, error) {
	_, err := r.GetWork()
	if err != nil {
		return false, errors.New("[JSON-RPC] " + err.Error())
	}
	r.markAlive()
	return !r.Sick(), nil
}

// Sick func
func (r *Client) Sick() bool {
	r.RLock()
	defer r.RUnlock()
	return r.sick
}

func (r *Client) markSick() {
	r.Lock()
	if !r.sick {
		atomic.AddUint64(&r.FailsCount, 1)
	}
	r.sickRate++
	r.successRate = 0
	if r.sickRate >= 5 {
		r.sick = true
	}
	r.Unlock()
}

func (r *Client) markAlive() {
	r.Lock()
	r.successRate++
	if r.successRate >= 5 {
		r.sick = false
		r.sickRate = 0
		r.successRate = 0
	}
	r.Unlock()
}
func (r *Client) GetWork() ([]string, error) {
	rpcResp, err := r.doPost("eth_getWork", []string{}, 73)
	var reply []string
	if err != nil {
		return reply, err
	}
	if rpcResp.Error != nil {
		return reply, errors.New("[JSON-RPC] " + rpcResp.Error["message"].(string))
	}
	err = json.Unmarshal(*rpcResp.Result, &reply)
	return reply, err
}
