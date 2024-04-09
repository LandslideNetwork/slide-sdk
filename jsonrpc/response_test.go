package jsonrpc

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSingleResponse(t *testing.T) {
	r := Response{
		single: true,
		params: []ResponseParams{{
			JsonRPC: "2.0",
			ID:      []byte("1"),
			Result:  []byte(`"1"`),
		}},
	}
	data, err := json.Marshal(r)
	require.NoError(t, err)
	assert.Equal(t, `{"jsonrpc":"2.0","id":1,"result":"1"}`, string(data))
}

func TestSingleErrResponse(t *testing.T) {
	r := Response{
		single: true,
		params: []ResponseParams{{
			JsonRPC: "2.0",
			ID:      []byte("1"),
			Error: &Error{
				Code:    0,
				Message: "1",
			},
		}},
	}
	data, err := json.Marshal(r)
	require.NoError(t, err)
	assert.Equal(t, `{"jsonrpc":"2.0","id":1,"error":{"code":0,"message":"1"}}`, string(data))
}

func TestMultipleResponse(t *testing.T) {
	r := Response{
		single: false,
		params: []ResponseParams{{
			JsonRPC: "2.0",
			ID:      []byte("1"),
			Result:  []byte(`"1"`),
		}},
	}
	data, err := json.Marshal(r)
	require.NoError(t, err)
	assert.Equal(t, `[{"jsonrpc":"2.0","id":1,"result":"1"}]`, string(data))
}
