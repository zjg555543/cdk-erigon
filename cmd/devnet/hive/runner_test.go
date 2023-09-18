package hive

import (
	"net/http/httptest"
	"testing"

	"github.com/ethereum/hive/hivesim"
	"github.com/ethereum/hive/libhive"
	rpc "github.com/ethereum/hive/simulators/ethereum/rpc"
)

func TestRpc(t *testing.T) {
	suite := rpc.Suite()
	testManager := libhive.NewTestManager(libhive.SimEnv{}, &backend{},
		map[string]*libhive.ClientDefinition{
			"erigon": {
				Name: "erigon",
				Meta: libhive.ClientMetadata{
					Roles: []string{"eth1"},
				},
			},
		})

	server := httptest.NewServer(testManager.API())
	defer server.Close()

	sim := hivesim.NewAt(server.URL)
	err := hivesim.RunSuite(sim, suite)

	if err != nil {
		t.Fatal(err)
	}
}
