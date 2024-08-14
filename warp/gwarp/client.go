package gwarp

import (
	"context"

	"github.com/consideritdone/landslidevm/warp"

	pb "github.com/consideritdone/landslidevm/proto/warp"
)

var _ warp.Signer = (*Client)(nil)

type Client struct {
	client pb.SignerClient
}

func NewClient(client pb.SignerClient) *Client {
	return &Client{client: client}
}

func (c *Client) Sign(unsignedMsg *warp.UnsignedMessage) ([]byte, error) {
	resp, err := c.client.Sign(context.Background(), &pb.SignRequest{
		NetworkId:     unsignedMsg.NetworkID,
		SourceChainId: unsignedMsg.SourceChainID[:],
		Payload:       unsignedMsg.Payload,
	})
	if err != nil {
		return nil, err
	}
	return resp.Signature, nil
}
