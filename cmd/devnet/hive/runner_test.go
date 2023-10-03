package hive

import (
	"fmt"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/ethereum/hive/hivesim"
	"github.com/ethereum/hive/libhive"
	rpc "github.com/ethereum/hive/simulators/ethereum/rpc"
	"github.com/ledgerwatch/erigon/turbo/logging"
	"github.com/urfave/cli/v2"
)

func TestRpc(t *testing.T) {
	suite := rpc.Suite()

	dname, err := os.MkdirTemp("", "data")
	defer os.RemoveAll(dname)

	if err != nil {
		t.Fatal(err)
	}

	logger := logging.SetupLoggerCtx("test", cli.NewContext(nil, nil, nil), false)

	testManager := libhive.NewTestManager(libhive.SimEnv{},
		NewBackend(dname, logger, 10),
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
	err = hivesim.RunSuite(sim, suite)

	if err != nil {
		t.Fatal(err)
	}

	var fails []string

	for _, suite := range testManager.Results() {
		for _, testCase := range suite.TestCases {
			if !testCase.SummaryResult.Pass {
				fails = append(fails, fmt.Sprintf("%s %s Failed: %s", suite.Name, testCase.Name, testCase.SummaryResult.Details))
			}
		}
	}

	if len(fails) > 0 {
		t.Fatal(fails)
	}
}
