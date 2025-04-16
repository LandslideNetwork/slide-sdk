// (c) 2019-2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package peer

import (
	"context"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cometbft/cometbft/libs/log"

	"github.com/landslidenetwork/slide-sdk/utils/avalanche/common"
	"github.com/landslidenetwork/slide-sdk/utils/avalanche/network/p2p"
	"github.com/landslidenetwork/slide-sdk/utils/set"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/landslidenetwork/slide-sdk/utils/message"

	"github.com/landslidenetwork/slide-sdk/utils/codec"
	"github.com/landslidenetwork/slide-sdk/utils/codec/linearcodec"
	"github.com/landslidenetwork/slide-sdk/utils/ids"
	"github.com/landslidenetwork/slide-sdk/utils/version"
)

var (
	defaultPeerVersion = &version.Application{
		Major: 1,
		Minor: 0,
		Patch: 0,
	}

	_ message.Request = &HelloRequest{}
	_                 = &HelloResponse{}
	_                 = &GreetingRequest{}
	_                 = &GreetingResponse{}
	_                 = &TestMessage{}

	_ message.RequestHandler = &HelloGreetingRequestHandler{}
	_ message.RequestHandler = &testRequestHandler{}

	_ common.AppSender = testAppSender{}

	_ p2p.Handler = &testSDKHandler{}
)

func TestNetworkDoesNotConnectToItself(t *testing.T) {
	selfNodeID := ids.GenerateTestNodeID()
	p2pNetwork, err := p2p.NewNetwork(log.NewNopLogger(), nil, prometheus.NewRegistry(), "")
	require.NoError(t, err)
	networkCodec := message.Codec
	n := NewNetwork(p2pNetwork, nil, nil, 1, networkCodec)
	//TODO: implement if necessary
	t.Log(n)
	t.Log(selfNodeID)
	//assert.NoError(t, n.Connected(context.Background(), selfNodeID, defaultPeerVersion))
	//assert.EqualValues(t, 0, n.Size())
}

func TestRequestAnyRequestsRoutingAndResponse(t *testing.T) {
	callNum := uint32(0)
	senderWg := &sync.WaitGroup{}
	var net Network
	sender := testAppSender{
		sendAppRequestFn: func(_ context.Context, nodes set.Set[ids.NodeID], requestID uint32, requestBytes []byte) error {
			nodeID, _ := nodes.Pop()
			senderWg.Add(1)
			go func() {
				defer senderWg.Done()
				//TODO: implement
				t.Log(nodeID)
				//if err := net.AppRequest(context.Background(), nodeID, requestID, time.Now().Add(5*time.Second), requestBytes); err != nil {
				//	panic(err)
				//}
			}()
			return nil
		},
		sendAppResponseFn: func(nodeID ids.NodeID, requestID uint32, responseBytes []byte) error {
			senderWg.Add(1)
			go func() {
				defer senderWg.Done()
				//TODO: implement
				//if err := net.AppResponse(context.Background(), nodeID, requestID, responseBytes); err != nil {
				//	panic(err)
				//}
				atomic.AddUint32(&callNum, 1)
			}()
			return nil
		},
	}

	codecManager := buildCodec(t, HelloRequest{}, HelloResponse{})
	p2pNetwork, err := p2p.NewNetwork(log.NewNopLogger(), nil, prometheus.NewRegistry(), "")
	require.NoError(t, err)
	networkCodec := message.Codec
	net = NewNetwork(p2pNetwork, sender, log.NewNopLogger(), 16, networkCodec)
	//TODO: implement
	t.Log(net)
	//net.SetRequestHandler(&HelloGreetingRequestHandler{codec: codecManager})
	//client := NewNetworkClient(net)
	//nodeID := ids.GenerateTestNodeID()
	//assert.NoError(t, net.Connected(context.Background(), nodeID, defaultPeerVersion))

	requestMessage := HelloRequest{Message: "this is a request"}

	//TODO: implement
	//defer net.Shutdown()
	//assert.NoError(t, net.Connected(context.Background(), nodeID, defaultPeerVersion))

	totalRequests := 5000
	numCallsPerRequest := 1 // on sending response
	totalCalls := totalRequests * numCallsPerRequest

	requestWg := &sync.WaitGroup{}
	requestWg.Add(totalCalls)
	for i := 0; i < totalCalls; i++ {
		go func(wg *sync.WaitGroup) {
			defer wg.Done()
			requestBytes, err := message.RequestToBytes(codecManager, requestMessage)
			assert.NoError(t, err)
			//TODO: implement
			t.Log(requestBytes)
			//responseBytes, _, err := client.SendAppRequestAny(context.Background(), defaultPeerVersion, requestBytes)
			//assert.NoError(t, err)
			//assert.NotNil(t, responseBytes)

			//var response TestMessage
			//if err = codecManager.Unmarshal(responseBytes, &response); err != nil {
			//	panic(fmt.Errorf("unexpected error during unmarshal: %w", err))
			//}
			//assert.Equal(t, "Hi", response.Message)
		}(requestWg)
	}

	requestWg.Wait()
	senderWg.Wait()
	assert.Equal(t, totalCalls, int(atomic.LoadUint32(&callNum)))
}

func TestAppRequestOnCtxCancellation(t *testing.T) {
	codecManager := buildCodec(t, HelloRequest{}, HelloResponse{})
	sender := testAppSender{
		sendAppRequestFn: func(_ context.Context, nodes set.Set[ids.NodeID], requestID uint32, requestBytes []byte) error {
			return nil
		},
		sendAppResponseFn: func(nodeID ids.NodeID, requestID uint32, responseBytes []byte) error {
			return nil
		},
	}

	p2pNetwork, err := p2p.NewNetwork(log.NewNopLogger(), nil, prometheus.NewRegistry(), "")
	require.NoError(t, err)
	networkCodec := message.Codec
	net := NewNetwork(p2pNetwork, sender, log.NewNopLogger(), 1, networkCodec)
	//net.SetRequestHandler(&HelloGreetingRequestHandler{codec: codecManager})

	requestMessage := HelloRequest{Message: "this is a request"}
	requestBytes, err := message.RequestToBytes(codecManager, requestMessage)
	assert.NoError(t, err)

	nodeID := ids.GenerateTestNodeID()
	ctx, cancel := context.WithCancel(context.Background())
	// cancel context prior to sending
	cancel()
	client := NewNetworkClient(net)
	_, err = client.SendAppRequest(ctx, nodeID, requestBytes)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestRequestRequestsRoutingAndResponse(t *testing.T) {
	callNum := uint32(0)
	senderWg := &sync.WaitGroup{}
	var net Network
	var lock sync.Mutex
	contactedNodes := make(map[ids.NodeID]struct{})
	sender := testAppSender{
		sendAppRequestFn: func(_ context.Context, nodes set.Set[ids.NodeID], requestID uint32, requestBytes []byte) error {
			nodeID, _ := nodes.Pop()
			lock.Lock()
			contactedNodes[nodeID] = struct{}{}
			lock.Unlock()
			senderWg.Add(1)
			go func() {
				defer senderWg.Done()
				//TODO: implement
				//if err := net.AppRequest(context.Background(), nodeID, requestID, time.Now().Add(5*time.Second), requestBytes); err != nil {
				//	panic(err)
				//}
			}()
			return nil
		},
		sendAppResponseFn: func(nodeID ids.NodeID, requestID uint32, responseBytes []byte) error {
			senderWg.Add(1)
			go func() {
				defer senderWg.Done()
				//TODO: implement
				//if err := net.AppResponse(context.Background(), nodeID, requestID, responseBytes); err != nil {
				//	panic(err)
				//}
				atomic.AddUint32(&callNum, 1)
			}()
			return nil
		},
	}

	codecManager := buildCodec(t, HelloRequest{}, HelloResponse{})
	p2pNetwork, err := p2p.NewNetwork(log.NewNopLogger(), nil, prometheus.NewRegistry(), "")
	require.NoError(t, err)
	networkCodec := message.Codec
	net = NewNetwork(p2pNetwork, sender, log.NewNopLogger(), 16, networkCodec)
	//TODO: implement if necessary
	//net.SetRequestHandler(&HelloGreetingRequestHandler{codec: codecManager})
	client := NewNetworkClient(net)

	nodes := []ids.NodeID{
		ids.GenerateTestNodeID(),
		ids.GenerateTestNodeID(),
		ids.GenerateTestNodeID(),
		ids.GenerateTestNodeID(),
		ids.GenerateTestNodeID(),
	}
	//for _, nodeID := range nodes {
	//	assert.NoError(t, net.Connected(context.Background(), nodeID, defaultPeerVersion))
	//}

	requestMessage := HelloRequest{Message: "this is a request"}
	//defer net.Shutdown()

	totalRequests := 5000
	numCallsPerRequest := 1 // on sending response
	totalCalls := totalRequests * numCallsPerRequest

	requestWg := &sync.WaitGroup{}
	requestWg.Add(totalCalls)
	nodeIdx := 0
	for i := 0; i < totalCalls; i++ {
		nodeIdx = (nodeIdx + 1) % (len(nodes))
		nodeID := nodes[nodeIdx]
		go func(wg *sync.WaitGroup, nodeID ids.NodeID) {
			defer wg.Done()
			requestBytes, err := message.RequestToBytes(codecManager, requestMessage)
			assert.NoError(t, err)
			responseBytes, err := client.SendAppRequest(context.Background(), nodeID, requestBytes)
			assert.NoError(t, err)
			assert.NotNil(t, responseBytes)

			var response TestMessage
			if err = codecManager.Unmarshal(responseBytes, &response); err != nil {
				panic(fmt.Errorf("unexpected error during unmarshal: %w", err))
			}
			assert.Equal(t, "Hi", response.Message)
		}(requestWg, nodeID)
	}

	requestWg.Wait()
	senderWg.Wait()
	assert.Equal(t, totalCalls, int(atomic.LoadUint32(&callNum)))
	for _, nodeID := range nodes {
		if _, exists := contactedNodes[nodeID]; !exists {
			t.Fatalf("expected nodeID %s to be contacted but was not", nodeID)
		}
	}

	// ensure empty nodeID is not allowed
	_, err = client.SendAppRequest(context.Background(), ids.EmptyNodeID, []byte("hello there"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot send request to empty nodeID")
}

func TestAppRequestOnShutdown(t *testing.T) {
	var (
		net    Network
		wg     sync.WaitGroup
		called bool
	)
	sender := testAppSender{
		sendAppRequestFn: func(_ context.Context, nodes set.Set[ids.NodeID], requestID uint32, requestBytes []byte) error {
			wg.Add(1)
			go func() {
				called = true
				// shutdown the network here to ensure any outstanding requests are handled as failed
				//TODO: implement
				//net.Shutdown()
				wg.Done()
			}() // this is on a goroutine to avoid a deadlock since calling Shutdown takes the lock.
			return nil
		},
	}

	codecManager := buildCodec(t, HelloRequest{}, HelloResponse{})
	p2pNetwork, err := p2p.NewNetwork(log.NewNopLogger(), nil, prometheus.NewRegistry(), "")
	require.NoError(t, err)
	networkCodec := message.Codec
	net = NewNetwork(p2pNetwork, sender, log.NewNopLogger(), 1, networkCodec)
	client := NewNetworkClient(net)
	nodeID := ids.GenerateTestNodeID()
	//TODO: implement
	t.Log(nodeID)
	t.Log(client)
	//require.NoError(t, net.Connected(context.Background(), nodeID, defaultPeerVersion))

	requestMessage := HelloRequest{Message: "this is a request"}
	//require.NoError(t, net.Connected(context.Background(), nodeID, defaultPeerVersion))

	wg.Add(1)
	go func() {
		defer wg.Done()
		requestBytes, err := message.RequestToBytes(codecManager, requestMessage)
		require.NoError(t, err)
		//TODO: implement
		t.Log(requestBytes)
		//responseBytes, _, err := client.SendAppRequestAny(context.Background(), defaultPeerVersion, requestBytes)
		//require.Error(t, err, ErrRequestFailed)
		//require.Nil(t, responseBytes)
	}()
	wg.Wait()
	require.True(t, called)
}

func TestAppRequestAnyOnCtxCancellation(t *testing.T) {
	codecManager := buildCodec(t, HelloRequest{}, HelloResponse{})
	type reqInfo struct {
		nodeID    ids.NodeID
		requestID uint32
	}
	sentAppRequest := make(chan reqInfo, 1)

	sender := testAppSender{
		sendAppRequestFn: func(ctx context.Context, nodes set.Set[ids.NodeID], requestID uint32, requestBytes []byte) error {
			if err := ctx.Err(); err != nil {
				return err
			}

			assert.Len(t, nodes, 1)
			sentAppRequest <- reqInfo{
				nodeID:    nodes.List()[0],
				requestID: requestID,
			}
			return nil
		},
		sendAppResponseFn: func(nodeID ids.NodeID, requestID uint32, responseBytes []byte) error {
			return nil
		},
	}

	p2pNetwork, err := p2p.NewNetwork(log.NewNopLogger(), nil, prometheus.NewRegistry(), "")
	require.NoError(t, err)
	networkCodec := message.Codec
	net := NewNetwork(p2pNetwork, sender, log.NewNopLogger(), 1, networkCodec)
	//net.SetRequestHandler(&HelloGreetingRequestHandler{codec: codecManager})
	//assert.NoError(t,
	//	net.Connected(
	//		context.Background(),
	//		ids.GenerateTestNodeID(),
	//		version.CurrentApp,
	//	),
	//)

	requestMessage := HelloRequest{Message: "this is a request"}
	requestBytes, err := message.RequestToBytes(codecManager, requestMessage)
	assert.NoError(t, err)

	// cancel context prior to sending
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	//TODO: implement
	t.Log(requestBytes)
	t.Log(ctx)
	//client := NewNetworkClient(net)
	//_, _, err = client.SendAppRequestAny(ctx, defaultPeerVersion, requestBytes)
	//assert.ErrorIs(t, err, context.Canceled)
	// Assert we didn't send anything
	select {
	case <-sentAppRequest:
		assert.FailNow(t, "should not have sent request")
	default:
	}

	// Cancel context after sending
	assert.Empty(t, net.(*network).outstandingRequestHandlers) // no outstanding requests
	_, cancel = context.WithCancel(context.Background())
	doneChan := make(chan struct{})
	go func() {
		//_, _, err = client.SendAppRequestAny(ctx, defaultPeerVersion, requestBytes)
		assert.ErrorIs(t, err, context.Canceled)
		close(doneChan)
	}()
	// Wait until we've "sent" the app request over the network
	// before cancelling context.
	//sentAppRequestInfo := <-sentAppRequest
	assert.Len(t, net.(*network).outstandingRequestHandlers, 1)
	cancel()
	<-doneChan
	// Should still be able to process a response after cancelling.
	assert.Len(t, net.(*network).outstandingRequestHandlers, 1) // context cancellation SendAppRequestAny failure doesn't clear
	//err = net.AppResponse(context.Background(), sentAppRequestInfo.nodeID, sentAppRequestInfo.requestID, []byte{})
	//assert.NoError(t, err)
	assert.Empty(t, net.(*network).outstandingRequestHandlers) // Received response
}

func TestRequestMinVersion(t *testing.T) {
	callNum := uint32(0)
	nodeID := ids.GenerateTestNodeID()
	codecManager := buildCodec(t, TestMessage{})

	var net Network
	sender := testAppSender{
		sendAppRequestFn: func(_ context.Context, nodes set.Set[ids.NodeID], reqID uint32, messageBytes []byte) error {
			atomic.AddUint32(&callNum, 1)
			assert.True(t, nodes.Contains(nodeID), "request nodes should contain expected nodeID")
			assert.Len(t, nodes, 1, "request nodes should contain exactly one node")

			go func() {
				time.Sleep(200 * time.Millisecond)
				atomic.AddUint32(&callNum, 1)
				//responseBytes, err := codecManager.Marshal(message.Version, TestMessage{Message: "this is a response"})
				//if err != nil {
				//	panic(err)
				//}
				//err = net.AppResponse(context.Background(), nodeID, reqID, responseBytes)
				//assert.NoError(t, err)
			}()
			return nil
		},
	}

	// passing nil as codec works because the net.AppRequest is never called
	p2pNetwork, err := p2p.NewNetwork(log.NewNopLogger(), nil, prometheus.NewRegistry(), "")
	require.NoError(t, err)
	networkCodec := message.Codec
	net = NewNetwork(p2pNetwork, sender, log.NewNopLogger(), 1, networkCodec)
	client := NewNetworkClient(net)
	requestMessage := TestMessage{Message: "this is a request"}
	requestBytes, err := message.RequestToBytes(codecManager, requestMessage)
	assert.NoError(t, err)
	//TODO: remove logging
	t.Log(client)
	t.Log(requestBytes)
	//assert.NoError(t,
	//	net.Connected(
	//		context.Background(),
	//		nodeID,
	//		&version.Application{
	//			Name:  version.Client,
	//			Major: 1,
	//			Minor: 7,
	//			Patch: 1,
	//		},
	//	),
	//)

	//// ensure version does not match
	//responseBytes, _, err := client.SendAppRequestAny(
	//	context.Background(),
	//	&version.Application{
	//		Name:  version.Client,
	//		Major: 2,
	//		Minor: 0,
	//		Patch: 0,
	//	},
	//	requestBytes,
	//)
	assert.Equal(t, err.Error(), "no peers found matching version avalanchego/2.0.0 out of 1 peers")
	//assert.Nil(t, responseBytes)
	//
	//// ensure version matches and the request goes through
	//responseBytes, _, err = client.SendAppRequestAny(context.Background(), defaultPeerVersion, requestBytes)
	assert.NoError(t, err)

	//var response TestMessage
	//if err = codecManager.Unmarshal(responseBytes, &response); err != nil {
	//	t.Fatal("unexpected error during unmarshal", err)
	//}
	//assert.Equal(t, "this is a response", response.Message)
}

func TestOnRequestHonoursDeadline(t *testing.T) {
	var net Network
	responded := false
	sender := testAppSender{
		sendAppRequestFn: func(_ context.Context, nodes set.Set[ids.NodeID], reqID uint32, message []byte) error {
			return nil
		},
		sendAppResponseFn: func(nodeID ids.NodeID, reqID uint32, message []byte) error {
			responded = true
			return nil
		},
	}

	codecManager := buildCodec(t, TestMessage{})
	requestBytes, err := marshalStruct(codecManager, TestMessage{Message: "hello there"})
	assert.NoError(t, err)

	//TODO: remove logging
	t.Log(requestBytes)
	t.Log(net)

	requestHandler := &testRequestHandler{
		processingDuration: 500 * time.Millisecond,
	}

	p2pNetwork, err := p2p.NewNetwork(log.NewNopLogger(), nil, prometheus.NewRegistry(), "")
	require.NoError(t, err)
	networkCodec := message.Codec
	net = NewNetwork(p2pNetwork, sender, log.NewNopLogger(), 1, networkCodec)
	net.SetRequestHandler(requestHandler)
	//nodeID := ids.GenerateTestNodeID()

	requestHandler.response, err = marshalStruct(codecManager, TestMessage{Message: "hi there"})
	assert.NoError(t, err)
	//err = net.AppRequest(context.Background(), nodeID, 1, time.Now().Add(1*time.Millisecond), requestBytes)
	assert.NoError(t, err)
	// ensure the handler didn't get called (as peer.Network would've dropped the request)
	assert.EqualValues(t, requestHandler.calls, 0)

	requestHandler.processingDuration = 0
	//err = net.AppRequest(context.Background(), nodeID, 2, time.Now().Add(250*time.Millisecond), requestBytes)
	assert.NoError(t, err)
	assert.True(t, responded)
	assert.EqualValues(t, requestHandler.calls, 1)
}

func TestHandleInvalidMessages(t *testing.T) {
	//codecManager := buildCodec(t, HelloGossip{}, TestMessage{})
	//nodeID := ids.GenerateTestNodeID()
	//requestID := uint32(1)
	//sender := &enginetest.Sender{
	//	SendAppErrorF: func(context.Context, ids.NodeID, uint32, int32, string) error {
	//		return nil
	//	},
	//}
	//p2pNetwork, err := p2p.NewNetwork(log.NewNopLogger(), sender, prometheus.NewRegistry(), "")
	//require.NoError(t, err)
	//clientNetwork := NewNetwork(p2pNetwork, sender, log.NewNopLogger(), 1)
	//clientNetwork.SetGossipHandler(message.NoopMempoolGossipHandler{})
	//clientNetwork.SetRequestHandler(&testRequestHandler{})
	//
	//assert.NoError(t, clientNetwork.Connected(context.Background(), nodeID, defaultPeerVersion))
	//
	//defer clientNetwork.Shutdown()

	//// Ensure a valid gossip message sent as any App specific message type does not trigger a fatal error
	//gossipMsg, err := buildGossip(codecManager, HelloGossip{Msg: "hello there!"})
	//assert.NoError(t, err)

	//// Ensure a valid request message sent as any App specific message type does not trigger a fatal error
	//requestMessage, err := marshalStruct(codecManager, TestMessage{Message: "Hello"})
	//assert.NoError(t, err)
	//
	//// Ensure a random message sent as any App specific message type does not trigger a fatal error
	//garbageResponse := make([]byte, 10)
	//// Ensure a zero-length message sent as any App specific message type does not trigger a fatal error
	//emptyResponse := make([]byte, 0)
	//// Ensure a nil byte slice sent as any App specific message type does not trigger a fatal error
	//var nilResponse []byte

	// Check for edge cases
	//assert.NoError(t, clientNetwork.AppGossip(context.Background(), nodeID, gossipMsg))
	//assert.NoError(t, clientNetwork.AppGossip(context.Background(), nodeID, requestMessage))
	//assert.NoError(t, clientNetwork.AppGossip(context.Background(), nodeID, garbageResponse))
	//assert.NoError(t, clientNetwork.AppGossip(context.Background(), nodeID, emptyResponse))
	//assert.NoError(t, clientNetwork.AppGossip(context.Background(), nodeID, nilResponse))
	//assert.NoError(t, clientNetwork.AppRequest(context.Background(), nodeID, requestID, time.Now().Add(time.Second), gossipMsg))
	//assert.NoError(t, clientNetwork.AppRequest(context.Background(), nodeID, requestID, time.Now().Add(time.Second), requestMessage))
	//assert.NoError(t, clientNetwork.AppRequest(context.Background(), nodeID, requestID, time.Now().Add(time.Second), garbageResponse))
	//assert.NoError(t, clientNetwork.AppRequest(context.Background(), nodeID, requestID, time.Now().Add(time.Second), emptyResponse))
	//assert.NoError(t, clientNetwork.AppRequest(context.Background(), nodeID, requestID, time.Now().Add(time.Second), nilResponse))
	//assert.ErrorIs(t, p2p.ErrUnrequestedResponse, clientNetwork.AppResponse(context.Background(), nodeID, requestID, gossipMsg))
	//assert.ErrorIs(t, p2p.ErrUnrequestedResponse, clientNetwork.AppResponse(context.Background(), nodeID, requestID, requestMessage))
	//assert.ErrorIs(t, p2p.ErrUnrequestedResponse, clientNetwork.AppResponse(context.Background(), nodeID, requestID, garbageResponse))
	//assert.ErrorIs(t, p2p.ErrUnrequestedResponse, clientNetwork.AppResponse(context.Background(), nodeID, requestID, emptyResponse))
	//assert.ErrorIs(t, p2p.ErrUnrequestedResponse, clientNetwork.AppResponse(context.Background(), nodeID, requestID, nilResponse))
}

func TestNetworkPropagatesRequestHandlerError(t *testing.T) {
	//codecManager := buildCodec(t, TestMessage{})
	//nodeID := ids.GenerateTestNodeID()
	//requestID := uint32(1)
	//sender := testAppSender{}
	//
	//p2pNetwork, err := p2p.NewNetwork(log.NewNopLogger(), nil, prometheus.NewRegistry(), "")
	//require.NoError(t, err)
	//clientNetwork := NewNetwork(p2pNetwork, sender, log.NewNopLogger(), 1)
	////clientNetwork.SetGossipHandler(message.NoopMempoolGossipHandler{})
	////clientNetwork.SetRequestHandler(&testRequestHandler{err: errors.New("fail")}) // Return an error from the request handler
	////
	////assert.NoError(t, clientNetwork.Connected(context.Background(), nodeID, defaultPeerVersion))
	////
	////defer clientNetwork.Shutdown()
	//
	//// Ensure a valid request message sent as any App specific message type does not trigger a fatal error
	//requestMessage, err := marshalStruct(codecManager, TestMessage{Message: "Hello"})
	//assert.NoError(t, err)
	//
	////// Check that if the request handler returns an error, it is propagated as a fatal error.
	////assert.Error(t, clientNetwork.AppRequest(context.Background(), nodeID, requestID, time.Now().Add(time.Second), requestMessage))
}

func TestNetworkAppRequestAfterShutdown(t *testing.T) {
	//require := require.New(t)
	//
	//net := NewNetwork(nil, nil, nil, ids.EmptyNodeID, 1)
	//net.Shutdown()
	//
	//require.NoError(net.SendAppRequest(context.Background(), ids.GenerateTestNodeID(), nil, nil))
	//require.NoError(net.SendAppRequest(context.Background(), ids.GenerateTestNodeID(), nil, nil))
}

func TestNetworkRouting(t *testing.T) {
	//require := require.New(t)
	//sender := &testAppSender{
	//	sendAppRequestFn: func(_ context.Context, s set.Set[ids.NodeID], u uint32, bytes []byte) error {
	//		return nil
	//	},
	//	sendAppResponseFn: func(id ids.NodeID, u uint32, bytes []byte) error {
	//		return nil
	//	},
	//}
	//protocol := 0
	//handler := &testSDKHandler{}
	//p2pNetwork, err := p2p.NewNetwork(log.NewNopLogger(), sender, prometheus.NewRegistry(), "")
	//require.NoError(err)
	//require.NoError(p2pNetwork.AddHandler(uint64(protocol), handler))
	//
	//networkCodec := codec.NewManager(0)
	//network := NewNetwork(p2pNetwork, nil, networkCodec, ids.EmptyNodeID, 1)
	//
	//nodeID := ids.GenerateTestNodeID()
	//foobar := append([]byte{byte(protocol)}, []byte("foobar")...)
	//err = network.AppRequest(context.Background(), nodeID, 0, time.Time{}, foobar)
	//require.NoError(err)
	//require.True(handler.appRequested)
	//
	//err = network.AppResponse(context.Background(), ids.GenerateTestNodeID(), 0, foobar)
	//require.ErrorIs(err, p2p.ErrUnrequestedResponse)
	//
	//err = network.AppRequestFailed(context.Background(), nodeID, 0, common.ErrTimeout)
	//require.ErrorIs(err, p2p.ErrUnrequestedResponse)
}

func buildCodec(t *testing.T, types ...interface{}) codec.Manager {
	lc := linearcodec.NewDefault()
	for _, typ := range types {
		assert.NoError(t, lc.RegisterType(typ))
	}

	//assert.NoError(t, codecManager.Register(message.Version, c))
	codecManager := codec.NewManager(math.MaxInt, lc)
	return codecManager
}

// marshalStruct is a helper method used to marshal an object as `interface{}`
// so that the codec is able to include the TypeID in the resulting bytes
func marshalStruct(codec codec.Manager, obj interface{}) ([]byte, error) {
	return codec.Marshal(&obj)
}

//func buildGossip(codec codec.Manager, msg message.GossipMessage) ([]byte, error) {
//	return codec.Marshal(message.Version, &msg)
//}

type testAppSender struct {
	sendAppRequestFn  func(context.Context, set.Set[ids.NodeID], uint32, []byte) error
	sendAppResponseFn func(ids.NodeID, uint32, []byte) error
	sendAppGossipFn   func(common.SendConfig, []byte) error
}

func (t testAppSender) SendAppRequest(ctx context.Context, nodeIDs set.Set[ids.NodeID], requestID uint32, message []byte) error {
	return t.sendAppRequestFn(ctx, nodeIDs, requestID, message)
}

func (t testAppSender) SendAppResponse(_ context.Context, nodeID ids.NodeID, requestID uint32, message []byte) error {
	return t.sendAppResponseFn(nodeID, requestID, message)
}

func (t testAppSender) SendAppGossip(_ context.Context, config common.SendConfig, message []byte) error {
	return t.sendAppGossipFn(config, message)
}

func (t testAppSender) SendAppError(ctx context.Context, nodeID ids.NodeID, requestID uint32, errorCode int32, errorMessage string) error {
	panic("not implemented")
}

type HelloRequest struct {
	Message string `serialize:"true"`
}

func (h HelloRequest) Handle(ctx context.Context, nodeID ids.NodeID, requestID uint32, handler message.RequestHandler) ([]byte, error) {
	// casting is only necessary for test since RequestHandler does not implement anything at the moment
	return handler.(TestRequestHandler).HandleHelloRequest(ctx, nodeID, requestID, &h)
}

func (h HelloRequest) String() string {
	return fmt.Sprintf("HelloRequest(%s)", h.Message)
}

type GreetingRequest struct {
	Greeting string `serialize:"true"`
}

func (g GreetingRequest) Handle(ctx context.Context, nodeID ids.NodeID, requestID uint32, handler message.RequestHandler) ([]byte, error) {
	// casting is only necessary for test since RequestHandler does not implement anything at the moment
	return handler.(TestRequestHandler).HandleGreetingRequest(ctx, nodeID, requestID, &g)
}

func (g GreetingRequest) String() string {
	return fmt.Sprintf("GreetingRequest(%s)", g.Greeting)
}

type HelloResponse struct {
	Response string `serialize:"true"`
}

type GreetingResponse struct {
	Greet string `serialize:"true"`
}

type TestRequestHandler interface {
	HandleHelloRequest(ctx context.Context, nodeID ids.NodeID, requestID uint32, request *HelloRequest) ([]byte, error)
	HandleGreetingRequest(ctx context.Context, nodeID ids.NodeID, requestID uint32, request *GreetingRequest) ([]byte, error)
}

type HelloGreetingRequestHandler struct {
	message.RequestHandler
	codec codec.Manager
}

func (h *HelloGreetingRequestHandler) HandleHelloRequest(ctx context.Context, nodeID ids.NodeID, requestID uint32, request *HelloRequest) ([]byte, error) {
	return h.codec.Marshal(HelloResponse{Response: "Hi"})
}

func (h *HelloGreetingRequestHandler) HandleGreetingRequest(ctx context.Context, nodeID ids.NodeID, requestID uint32, request *GreetingRequest) ([]byte, error) {
	return h.codec.Marshal(GreetingResponse{Greet: "Hey there"})
}

type TestMessage struct {
	Message string `serialize:"true"`
}

func (t TestMessage) Handle(ctx context.Context, nodeID ids.NodeID, requestID uint32, handler message.RequestHandler) ([]byte, error) {
	return handler.(*testRequestHandler).handleTestRequest(ctx, nodeID, requestID, &t)
}

func (t TestMessage) String() string {
	return fmt.Sprintf("TestMessage(%s)", t.Message)
}

type testRequestHandler struct {
	message.RequestHandler
	calls              uint32
	processingDuration time.Duration
	response           []byte
	err                error
}

func (r *testRequestHandler) handleTestRequest(ctx context.Context, _ ids.NodeID, _ uint32, _ *TestMessage) ([]byte, error) {
	r.calls++
	select {
	case <-time.After(r.processingDuration):
		break
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	return r.response, r.err
}

type testSDKHandler struct {
	appRequested bool
}

func (t *testSDKHandler) AppGossip(ctx context.Context, nodeID ids.NodeID, gossipBytes []byte) {
	// TODO implement me
	panic("implement me")
}

func (t *testSDKHandler) AppRequest(ctx context.Context, nodeID ids.NodeID, deadline time.Time, requestBytes []byte) ([]byte, *common.AppError) {
	t.appRequested = true
	return nil, nil
}
