package jsonrpc

import (
	"bytes"
	"encoding/json"
	"net/http"
	"sync"
)

var (
	_ Client      = (*rcpClient)(nil)
	_ BatchClient = (*rpcBatchClient)(nil)
)

type (
	Client interface {
		Call(string, json.RawMessage) (json.RawMessage, error)
		Batch() BatchClient
	}

	BatchClient interface {
		Alloc(method string, params json.RawMessage)
		Call() ([]json.RawMessage, []*Error, error)
	}

	rcpClient struct {
		endpoint string
		client   http.Client
		id       uint64
		mu       *sync.Mutex
	}

	rpcBatchClient struct {
		endpoint string
		client   http.Client
		id       uint64
		mu       *sync.Mutex
		request  Request
	}
)

func NewClient(endpoint string, client http.Client) Client {
	return &rcpClient{
		endpoint: endpoint,
		client:   client,
		id:       0,
		mu:       new(sync.Mutex),
	}
}

func (client rcpClient) Call(method string, params json.RawMessage) (json.RawMessage, error) {
	client.mu.Lock()
	client.id++
	client.mu.Unlock()

	id, err := json.Marshal(client.id)
	if err != nil {
		return nil, err
	}

	req, err := json.Marshal(Request{
		single: true,
		params: []RequestParams{
			{
				JsonRPC: "2.0",
				ID:      id,
				Method:  method,
				Params:  params,
			},
		},
	})
	if err != nil {
		return nil, err
	}

	res, err := client.client.Post(client.endpoint, "application/json", bytes.NewReader(req))
	if err != nil {
		return nil, err
	}

	var resp Response
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		return nil, err
	}
	defer res.Body.Close()

	return resp.params[0].Result, resp.params[0].Error
}

func (client rcpClient) Batch() BatchClient {
	return &rpcBatchClient{
		endpoint: client.endpoint,
		client:   client.client,
		mu:       new(sync.Mutex),
		request:  Request{single: false},
	}
}

func (batch *rpcBatchClient) Alloc(method string, params json.RawMessage) {
	batch.mu.Lock()
	defer batch.mu.Unlock()
	batch.id++

	id, _ := json.Marshal(batch.id)
	batch.request.params = append(batch.request.params, RequestParams{
		JsonRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	})
}

func (batch rpcBatchClient) Call() ([]json.RawMessage, []*Error, error) {
	req, err := json.Marshal(batch.request)
	if err != nil {
		return nil, nil, err
	}

	res, err := batch.client.Post(batch.endpoint, "application/json", bytes.NewReader(req))
	if err != nil {
		return nil, nil, err
	}

	var resp Response
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		return nil, nil, err
	}
	defer res.Body.Close()

	respdata := make([]json.RawMessage, len(batch.request.params))
	resperro := make([]*Error, len(batch.request.params))
	for i := range resp.params {
		respdata[i] = resp.params[i].Result
		resperro[i] = resp.params[i].Error
	}

	return respdata, resperro, nil
}
