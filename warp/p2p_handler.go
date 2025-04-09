package warp

import (
	"context"
	"github.com/landslidenetwork/slide-sdk/utils/avalanche/common"
	"github.com/landslidenetwork/slide-sdk/utils/avalanche/network/p2p"
	"github.com/landslidenetwork/slide-sdk/utils/avalanche/warp"
	"github.com/landslidenetwork/slide-sdk/utils/ids"
	"time"
)

// NewHandler returns an instance of Handler
func NewHandler(
	//verifier Verifier,
	signer warp.Signer) *Handler {
	//return NewCachedHandler(
	//	&cache.Empty[ids.ID, []byte]{},
	//	verifier,
	//	signer,
	//)
	return &Handler{signer: signer}
}

// Handler signs warp messages
type Handler struct {
	p2p.NoOpHandler

	//signatureCache cache.Cacher[ids.ID, []byte]
	//verifier       Verifier
	signer warp.Signer
}

func (h *Handler) AppRequest(
	ctx context.Context,
	_ ids.NodeID,
	_ time.Time,
	requestBytes []byte,
) ([]byte, *common.AppError) {
	//request := &sdk.SignatureRequest{}
	//if err := proto.Unmarshal(requestBytes, request); err != nil {
	//	return nil, &common.AppError{
	//		Code:    p2p.ErrUnexpected.Code,
	//		Message: fmt.Sprintf("failed to unmarshal request: %s", err),
	//	}
	//}
	//
	//msg, err := warp.ParseUnsignedMessage(request.Message)
	//if err != nil {
	//	return nil, &common.AppError{
	//		Code:    p2p.ErrUnexpected.Code,
	//		Message: fmt.Sprintf("failed to parse warp unsigned message: %s", err),
	//	}
	//}
	//
	//msgID := msg.ID()
	//if signatureBytes, ok := h.signatureCache.Get(msgID); ok {
	//	return signatureToResponse(signatureBytes)
	//}
	//
	//if err := h.verifier.Verify(ctx, msg, request.Justification); err != nil {
	//	return nil, err
	//}
	//
	//signature, err := h.signer.Sign(msg)
	//if err != nil {
	//	return nil, &common.AppError{
	//		Code:    p2p.ErrUnexpected.Code,
	//		Message: fmt.Sprintf("failed to sign message: %s", err),
	//	}
	//}
	//
	//h.signatureCache.Put(msgID, signature)
	//return signatureToResponse(signature)
	return nil, nil
}

func signatureToResponse(signature []byte) ([]byte, *common.AppError) {
	//response := &sdk.SignatureResponse{
	//	Signature: signature,
	//}
	//
	//responseBytes, err := proto.Marshal(response)
	//if err != nil {
	//	return nil, &common.AppError{
	//		Code:    p2p.ErrUnexpected.Code,
	//		Message: fmt.Sprintf("failed to marshal response: %s", err),
	//	}
	//}
	//return responseBytes, nil
	return nil, nil
}
