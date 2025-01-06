package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/joho/godotenv"
	"github.com/landslidenetwork/slide-sdk/utils/ids"
	"github.com/landslidenetwork/slide-sdk/warp"
	"log"
	"os"
	"os/exec"
	"sync"
	"testing"
)

var (
	clientManager = RPCClientManager{
		rpcClients: make(map[string]warp.Client),
		mtx:        sync.RWMutex{},
	}
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
	// Run the command and capture the output
	output, err := cmd.Output()
	if err != nil {
		log.Fatalf("Error executing script: %s. Output: %s", err, output)
		return
	}
	log.Println(string(output))
	err = godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	log.Println("Configure network RPC clients on .env file and save it")
	log.Println("Press \"Enter\" when environment will be ready to start WARP tests")
	fmt.Scanln()
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
	exitCode := m.Run()

	os.Exit(exitCode)
}

func TestGetMessage(t *testing.T) {
	clientManager.mtx.RLock()
	defer clientManager.mtx.RUnlock()
	for nodeID, rpcClient := range clientManager.rpcClients {
		t.Logf("NodeID: %s\n", nodeID)
		msgContent, err := rpcClient.GetMessage(context.Background(), ids.GenerateTestID())
		t.Error(err)
		t.Logf("Message content: %s\n", string(msgContent))
	}
	log.Println("TestA running")
}

func TestGetMessageSignature(t *testing.T) {
	clientManager.mtx.RLock()
	defer clientManager.mtx.RUnlock()
	for nodeID, rpcClient := range clientManager.rpcClients {
		t.Logf("NodeID: %s\n", nodeID)
		msgSignature, err := rpcClient.GetMessageSignature(context.Background(), ids.GenerateTestID())
		t.Error(err)
		t.Logf("Message signature: %s\n", string(msgSignature))
	}
	log.Println("TestB running")
}

func TestGetBlockSignature(t *testing.T) {
	clientManager.mtx.RLock()
	defer clientManager.mtx.RUnlock()
	for nodeID, rpcClient := range clientManager.rpcClients {
		t.Logf("NodeID: %s\n", nodeID)
		blkSignature, err := rpcClient.GetBlockSignature(context.Background(), ids.GenerateTestID())
		t.Error(err)
		t.Logf("Block signature: %s\n", string(blkSignature))
	}
	log.Println("TestC running")
}
