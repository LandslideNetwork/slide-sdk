package jsonrpc

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequestUnmarshalingSingleObject(t *testing.T) {
	r := strings.NewReader(`{
			"jsonrpc": "2.0",
			"id": 1,
			"method": "some_name",
			"params": 1
		}
	`)

	var request Request
	require.NoError(t, json.NewDecoder(r).Decode(&request), "can't decode object")
	assert.True(t, request.single)
	assert.Len(t, request.params, 1)

	prm := request.params[0]
	assert.Equal(t, prm.JsonRPC, "2.0")
	assert.Equal(t, prm.ID, json.RawMessage([]byte("1")))
	assert.Equal(t, prm.Method, "some_name")
	assert.Equal(t, prm.Params, json.RawMessage([]byte("1")))
}

func TestRequestUnmarshalingSingleBatch(t *testing.T) {
	r := strings.NewReader(`
		[{
			"jsonrpc": "2.0",
			"id": 1,
			"method": "some_name",
			"params": 1
		},{
			"jsonrpc": "2.0",
			"id": 1,
			"method": "some_name",
			"params": 1
		}]
	`)

	var request Request
	require.NoError(t, json.NewDecoder(r).Decode(&request), "can't decode object")
	assert.False(t, request.single)
	assert.Len(t, request.params, 2)

	assert.Equal(t, request.params[0].JsonRPC, "2.0")
	assert.Equal(t, request.params[0].ID, json.RawMessage([]byte("1")))
	assert.Equal(t, request.params[0].Method, "some_name")
	assert.Equal(t, request.params[0].Params, json.RawMessage([]byte("1")))
	assert.Equal(t, request.params[1].JsonRPC, "2.0")
	assert.Equal(t, request.params[1].ID, json.RawMessage([]byte("1")))
	assert.Equal(t, request.params[1].Method, "some_name")
	assert.Equal(t, request.params[1].Params, json.RawMessage([]byte("1")))
}
