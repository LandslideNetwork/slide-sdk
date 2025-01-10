package e2e

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/cometbft/cometbft/libs/rand"
	"github.com/joho/godotenv"
	"github.com/landslidenetwork/slide-sdk/utils/crypto/bls"
	"github.com/landslidenetwork/slide-sdk/utils/ids"
	warputils "github.com/landslidenetwork/slide-sdk/utils/warp"
	"github.com/landslidenetwork/slide-sdk/utils/warp/payload"
	"github.com/landslidenetwork/slide-sdk/warp"
	"github.com/stretchr/testify/require"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"
	"testing"
)

var (
	clientManager = RPCClientManager{
		rpcClients: make(map[string]warp.Client),
		mtx:        sync.RWMutex{},
	}
	networkID uint32
	chainID   ids.ID
)

type RPCClientManager struct {
	rpcClients map[string]warp.Client
	mtx        sync.RWMutex
}

func (m *RPCClientManager) Get(key string) warp.Client {
	m.mtx.RLock()
	defer m.mtx.RUnlock()
	return m.rpcClients[key]
}

type e2eConfig struct {
	NetworkRPCAddresses map[string]string `json:"network_rpc_addresses"`
	ChainID             string            `json:"chain_id"`
}

func TestMain(m *testing.M) {
	scriptPath := "./run_universal_subnet_runner.sh"
	cmd := exec.Command("sh", scriptPath)

	ctx, disableLogsReading := context.WithCancel(context.Background())
	defer disableLogsReading()
	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	// Create or open the log file
	logFile, err := os.OpenFile("logs.txt", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		fmt.Printf("Error opening log file: %v\n", err)
		return
	}
	defer logFile.Close()

	// Start the command
	go func() {
		if err := cmd.Start(); err != nil {
			fmt.Printf("Error starting command: %v\n", err)
			return
		}
	}()

	// Function to handle real-time output logging
	go streamLogs(ctx, stdout, logFile)
	go streamLogs(ctx, stderr, logFile)

	fmt.Println("Configure network RPC clients on .env file and save it")

	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGCONT)

	fmt.Printf("Process ID (PID): %d\n", os.Getpid())
	fmt.Println("Send SIGCONT (kill -CONT <PID>) to continue...")

	sig := <-signalCh
	if sig == syscall.SIGCONT {
		fmt.Println("Received SIGCONT: Process continues or notification received.")
	}

	err = godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	cfg := os.Getenv("config")
	configuration := e2eConfig{}
	err = json.Unmarshal([]byte(cfg), &configuration)
	if err != nil {
		log.Fatal(err)
	}
	for nodeID, nodeURI := range configuration.NetworkRPCAddresses {
		clientManager.rpcClients[nodeID], err = warp.NewClient(nodeURI, configuration.ChainID)
		if err != nil {
			log.Fatal(err)
		}
	}
	networkID = 7777
	chainID = ids.GenerateTestID()
	exitCode := m.Run()

	os.Exit(exitCode)
}

// streamLogs reads from the reader and writes to the file and console
func streamLogs(ctx context.Context, reader io.ReadCloser, logFile io.Writer) {
	scanner := bufio.NewScanner(reader)
	var buffer bytes.Buffer
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			fmt.Fprint(logFile, "Finish reading stream: network stopped")
			return
		default:
			line := scanner.Text()
			buffer.WriteString(line + "\n") // Accumulate logs
			fmt.Fprintln(logFile, line)     // Write to file
		}
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintf(logFile, "Error reading stream: %v\n", err)
	}
}

func TestGetMessage(t *testing.T) {
	clientManager.mtx.RLock()
	defer clientManager.mtx.RUnlock()
	for nodeID, rpcClient := range clientManager.rpcClients {
		t.Logf("NodeID: %s\n", nodeID)
		msg1, err := warputils.NewUnsignedMessage(networkID, chainID, []byte(rand.Str(24)))
		result, err := rpcClient.AddMessage(context.Background(), ids.GenerateTestID(), msg1)
		require.NoError(t, err)
		require.NotNil(t, result.MessageID)
		msgContent, err := rpcClient.GetMessage(context.Background(), result.MessageID)
		require.NoError(t, err)
		msg2, err := warputils.ParseMessage(msgContent)
		require.NoError(t, err)
		require.NotNil(t, msg2)
		require.Equal(t, msg1.NetworkID, msg2.NetworkID)
		require.Equal(t, msg1.SourceChainID, msg2.SourceChainID)
		require.Equal(t, msg1.Payload, msg2.Payload)
	}
}

func TestGetMessageSignature(t *testing.T) {
	clientManager.mtx.RLock()
	defer clientManager.mtx.RUnlock()
	for nodeID, rpcClient := range clientManager.rpcClients {
		t.Logf("NodeID: %s\n", nodeID)
		msg, err := warputils.NewUnsignedMessage(networkID, chainID, []byte(rand.Str(24)))
		result, err := rpcClient.AddMessage(context.Background(), ids.GenerateTestID(), msg)
		require.NoError(t, err)
		require.NotNil(t, result.MessageID)
		msgSignature1, err := rpcClient.GetMessageSignature(context.Background(), result.MessageID)
		require.NoError(t, err)
		require.NotNil(t, msgSignature1)
		secretKey, err := bls.SecretKeyFromBytes(config.BLSSecretKey)
		require.NoError(t, err)
		warpSigner := warputils.NewSigner(secretKey, networkID, chainID)
		msgSignature2, err := warpSigner.Sign(msg)
		require.NoError(t, err)
		require.Equal(t, msgSignature1, msgSignature2)
	}
}

func TestGetBlockSignature(t *testing.T) {
	clientManager.mtx.RLock()
	defer clientManager.mtx.RUnlock()
	for nodeID, rpcClient := range clientManager.rpcClients {
		t.Logf("NodeID: %s\n", nodeID)
		blkSignature1, err := rpcClient.GetBlockSignature(context.Background(), ids.GenerateTestID())
		require.NoError(t, err)
		blockHashPayload, err := payload.NewHash(blockID)
		require.NoError(t, err)
		unsignedMessage, err := warputils.NewUnsignedMessage(networkID, chainID, blockHashPayload.Bytes())
		secretKey, err := bls.SecretKeyFromBytes(config.BLSSecretKey)
		require.NoError(t, err)
		warpSigner := warputils.NewSigner(secretKey, networkID, chainID)
		blkSignature2, err := warpSigner.Sign(unsignedMessage)
		require.NoError(t, err)
		require.Equal(t, blkSignature1, blkSignature2)
	}
	log.Println("TestC running")
}
