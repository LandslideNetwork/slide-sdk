package writer

import (
	"context"
	"errors"
	"io"

	writerpb "github.com/consideritdone/landslidevm/proto/io/writer"
)

var _ io.Writer = (*Client)(nil)

// Client is an io.Writer that talks over RPC.
type Client struct{ client writerpb.WriterClient }

// NewClient returns a writer connected to a remote writer
func NewClient(client writerpb.WriterClient) *Client {
	return &Client{client: client}
}

func (c *Client) Write(p []byte) (int, error) {
	resp, err := c.client.Write(context.Background(), &writerpb.WriteRequest{
		Payload: p,
	})
	if err != nil {
		return 0, err
	}

	if resp.Error != nil {
		err = errors.New(*resp.Error)
	}
	return int(resp.Written), err
}
