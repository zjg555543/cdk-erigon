package hive

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/ethereum/hive/libhive"
	"github.com/ledgerwatch/erigon-lib/chain"
	"github.com/ledgerwatch/erigon/core/types"
	"github.com/ledgerwatch/erigon/devnet/args"
	"github.com/ledgerwatch/erigon/devnet/devnet"
	"github.com/ledgerwatch/log/v3"
)

// go:embed genesis.json
var defaultGenesis []byte

type backend struct {
	sync.Mutex
	clientCounter          uint64
	hivePort               int
	containers             map[string]*image // tracks created containers and their image names
	networks               map[uint64]*devnet.Network
	dataDir                string
	logger                 log.Logger
	nextBaseP2PPort        int
	nextBasePrivateApiPort int
	nextBaseRPCPort        int
	maxNetworkSize         int
}

type image struct {
	name string
	opts libhive.ContainerOptions
}

type apiServer struct {
	s    *http.Server
	addr net.Addr
}

func (s apiServer) Close() error {
	return s.s.Close()
}

func (s apiServer) Addr() net.Addr {
	return s.addr
}

func NewBackend(dataDir string, logger log.Logger, maxNetworkSize int) *backend {
	return &backend{
		containers:             map[string]*image{},
		networks:               map[uint64]*devnet.Network{},
		dataDir:                dataDir,
		logger:                 logger,
		nextBaseP2PPort:        30303,
		nextBasePrivateApiPort: 15000,
		nextBaseRPCPort:        8545,
		maxNetworkSize:         maxNetworkSize,
	}
}

func (b *backend) Build(context.Context, libhive.Builder) error { return nil }

func (b *backend) ServeAPI(ctx context.Context, h http.Handler) (libhive.APIServer, error) {
	l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", b.hivePort))
	if err != nil {
		return nil, err
	}
	srv := &http.Server{Handler: h}
	go srv.Serve(l)
	return apiServer{srv, l.Addr()}, nil
}

func (b *backend) CreateContainer(ctx context.Context, imageName string, opt libhive.ContainerOptions) (string, error) {
	id := fmt.Sprintf("%0.8x", atomic.AddUint64(&b.clientCounter, 1))

	b.Lock()
	if _, ok := b.containers[id]; ok {
		b.Unlock()
		return id, fmt.Errorf("duplicate container ID %q", id)
	}

	if b.containers == nil {
		b.containers = map[string]*image{
			id: &image{
				name: imageName,
				opts: opt,
			},
		}
	} else {
		b.containers[id] = &image{
			name: imageName,
			opts: opt,
		}
	}

	b.Unlock()
	return id, nil
}

func (b *backend) StartContainer(ctx context.Context, containerId string, opts libhive.ContainerOptions) (*libhive.ContainerInfo, error) {
	b.Lock()
	_ /*container*/, ok := b.containers[containerId]
	b.Unlock()

	if !ok {
		return nil, fmt.Errorf("unknown container ID %q", containerId)
	}

	var genesisTemplate types.Genesis

	if genesisHeader, ok := opts.Files["/genesis.json"]; ok {
		genesisFile, err := genesisHeader.Open()

		if err != nil {
			return nil, fmt.Errorf("can't start container: %s: failed to open attached genesis.json: %w", containerId, err)
		}

		err = json.NewDecoder(genesisFile).Decode(&genesisTemplate)
		genesisFile.Close()

		if err != nil {
			return nil, fmt.Errorf("can't start container: %s: failed to open attached genesis.json: %w", containerId, err)
		}
	} else {
		err := json.Unmarshal(defaultGenesis, &genesisTemplate)

		if err != nil {
			return nil, fmt.Errorf("can't start container: %s: failed to unmarshal default genesis.json: %w", containerId, err)
		}
	}

	node, genesis, files, err := configureNode(opts.Env, genesisTemplate)

	if err != nil {
		return nil, fmt.Errorf("can't start container: %s: %w", containerId, err)
	}

	network, ok := b.networks[genesis.Config.ChainID.Uint64()]

	if !ok {
		network = &devnet.Network{
			DataDir:            filepath.Join(b.dataDir, genesis.Config.ChainID.String()),
			Id:                 genesis.Config.ChainID.Uint64(),
			Chain:              genesis.Config.ChainName,
			Logger:             b.logger,
			BasePort:           b.nextBaseP2PPort,
			BasePrivateApiAddr: fmt.Sprintf("localhost:%d", b.nextBasePrivateApiPort),
			BaseRPCHost:        "localhost",
			BaseRPCPort:        b.nextBaseRPCPort,
			Genesis:            genesis,
		}

		b.nextBaseP2PPort += b.maxNetworkSize
		b.nextBasePrivateApiPort += b.maxNetworkSize
		b.nextBaseRPCPort += (b.maxNetworkSize * args.RPCPortsPerNode)

		if b.networks == nil {
			b.networks = map[uint64]*devnet.Network{}
		}

		b.networks[genesis.Config.ChainID.Uint64()] = network

		errors := make(chan error, 1)

		go func(nw *devnet.Network) {
			errors <- nw.Start(ctx)
		}(network)

		err = <-errors

		if err != nil {
			return nil, fmt.Errorf("can't start container: can't start network: %s: %w", genesis.Config.ChainName, err)
		}
	}

	/*
		# Initialize the local testchain with the genesis state
		echo "Initializing database with genesis state..."
		$erigon $FLAGS init /genesis.json
	*/

	rpcPort, err := network.Attach(ctx, node, files)

	if err != nil {
		return nil, fmt.Errorf("can't start container: %s: can't attach node: %w", containerId, err)
	}

	/*
		# Initialize the local testchain with the genesis state
		echo "Initializing database with genesis state..."
		$erigon $FLAGS init /genesis.json

		// Load the test chain if present
		// echo "Loading initial blockchain..."
		if [ -f /chain.rlp ]; then
			echo "Loading initial blockchain..."
			$erigon $FLAGS import /chain.rlp
		else
			echo "Warning: chain.rlp not found."
		}

		// Load the remainder of the test chain
		// echo "Loading remaining individual blocks..."
		if [ -d /blocks ]; then
			echo "Loading remaining individual blocks..."
			for file in $(ls /blocks | sort -n); do
				echo "Importing " $}le
				$erigon $FLAGS import /blocks/$file
			done
		else
			echo "Warning: blocks folder not found."
		}
	*/

	return &libhive.ContainerInfo{
		ID:          containerId,
		IP:          "127.0.0.1",
		EthPortHTTP: rpcPort,
		Wait:        func() {},
	}, nil
}

func (b *backend) DeleteContainer(containerID string) error  { return fmt.Errorf("TODO") }
func (b *backend) PauseContainer(containerID string) error   { return fmt.Errorf("TODO") }
func (b *backend) UnpauseContainer(containerID string) error { return fmt.Errorf("TODO") }

func (b *backend) RunProgram(ctx context.Context, containerID string, cmdline []string) (*libhive.ExecInfo, error) {
	return nil, fmt.Errorf("TODO")
}

func (b *backend) NetworkNameToID(name string) (string, error) { return "", fmt.Errorf("TODO") }
func (b *backend) CreateNetwork(name string) (string, error)   { return "", fmt.Errorf("TODO") }
func (b *backend) RemoveNetwork(id string) error               { return fmt.Errorf("TODO") }
func (b *backend) ContainerIP(containerID, networkID string) (net.IP, error) {
	return nil, fmt.Errorf("TODO")
}
func (b *backend) ConnectContainer(containerID, networkID string) error    { return fmt.Errorf("TODO") }
func (b *backend) DisconnectContainer(containerID, networkID string) error { return fmt.Errorf("TODO") }

func configureNode(env map[string]string, genesisTemplate types.Genesis) (devnet.Node, *types.Genesis, map[string][]byte, error) {
	var node args.Node

	if eval, ok := env["HIVE_LOGLEVEL"]; ok && eval != "" {
		node.ConsoleVerbosity = eval
	}

	if eval, ok := env["HIVE_BOOTNODE"]; ok && eval != "" {
		// Somehow the bootnodes flag is not working for erigon, only staticpeers is working for sync tests
		node.StaticPeers = eval
		node.NoDiscover = "true" //--nodiscover
	}

	if eval, ok := env["HIVE_SKIP_POW"]; ok && eval != "" {
		node.FakePOW = true //--fakepow"
	}

	// Create the data directory.
	//mkdir /erigon-hive-datadir
	//FLAGS="$FLAGS --datadir /erigon-hive-datadir"
	//FLAGS="$FLAGS --db.size.limit 2GB"

	// If a specific network ID is requested, use that
	if eval, ok := env["HIVE_NETWORK_ID"]; ok && eval != "" {
		networkId, err := strconv.ParseInt(eval, 10, 64)

		if err != nil {
			return nil, nil, nil, err
		}

		node.NetworkId = int(networkId)
	} else {
		node.NetworkId = 1337
	}

	genesis, err := configureGenesis(genesisTemplate, env)

	if err != nil {
		return nil, nil, nil, err
	}

	// Dump genesis
	//echo "Supplied genesis state:"
	//cat /genesis.json

	//echo "Command flags till now:"
	//echo $FLAGS

	// Configure RPC.
	node.Http = true
	node.HttpAddr = "0.0.0.0"
	node.HttpApi = "admin,debug,eth,net,txpool,web3"
	node.WS = true
	node.WSHasOwnPort = true

	var files map[string][]byte

	if eval, ok := env["HIVE_TERMINAL_TOTAL_DIFFICULTY"]; ok && eval != "" {
		//JWT_SECRET="0x7365637265747365637265747365637265747365637265747365637265747365"
		//echo -n $JWT_SECRET > /jwt.secret
		node.AuthRpcAddr = "0.0.0.0"

		files = map[string][]byte{
			"jwt.secret": []byte("0x7365637265747365637265747365637265747365637265747365637265747365"),
		}

		//node.AuthrpcJwtSecret = "/jwt.secret"
	}

	node.NAT = "none"

	// Congigure any mining operation
	// TODO: Erigon doesn't have inbuilt cpu miner.
	// Need to add https://github.com/panglove/ethcpuminer/tree/master/ethash for cpu mining with erigon
	if eval, ok := env["HIVE_MINER"]; ok && eval != "" {
		miner := args.BlockProducer{Node: node}
		miner.Mine = true
		miner.Etherbase = eval

		if eval, ok := env["HIVE_MINER_EXTRA"]; ok && eval != "" {
			miner.MinerExtraData = eval
		}

		// Import clique signing key.
		if eval, ok := env["HIVE_CLIQUE_PRIVATEKEY"]; ok && eval != "" {
			// Create password file.
			// Ensure password file is used when running geth in mining mode.
			//miner.MinerSigfile = "private_key.txt"

			if len(files) == 0 {
				files = map[string][]byte{
					"private_key.txt": []byte(eval),
				}
			} else {
				files["private_key.txt"] = []byte(eval)
			}
		}

		return miner, genesis, files, nil
	}

	return args.NonBlockProducer{Node: node}, genesis, files, nil
}

func configureGenesis(genesis types.Genesis, env map[string]string) (*types.Genesis, error) {

	var err error

	if eval, ok := env["HIVE_CLIQUE_PERIOD"]; ok && eval != "" {
		genesis.Config.Ethash = nil
		if genesis.Config.Clique == nil {
			genesis.Config.Clique = &chain.CliqueConfig{Period: 0, Epoch: 30000}
		}

		if genesis.Config.Clique.Period, err = strconv.ParseUint(eval, 10, 64); err != nil {
			return nil, fmt.Errorf("can't parse clique period: %w", err)
		}
	}

	if eval, ok := env["HIVE_CHAIN_ID"]; ok && eval != "" {
		if genesis.Config.ChainID, ok = (&big.Int{}).SetString(eval, 0); !ok {
			return nil, fmt.Errorf("can't set chain id from %s", eval)
		}
	} else {
		genesis.Config.ChainID = big.NewInt(1)
	}

	if eval, ok := env["HIVE_FORK_HOMESTEAD"]; ok && eval != "" {
		if genesis.Config.HomesteadBlock, ok = (&big.Int{}).SetString(eval, 0); !ok {
			return nil, fmt.Errorf("can't set homestead block from %s", eval)
		}
	}

	if eval, ok := env["HIVE_FORK_DAO_BLOCK"]; ok && eval != "" {
		if genesis.Config.DAOForkBlock, ok = (&big.Int{}).SetString(eval, 0); !ok {
			return nil, fmt.Errorf("can't set DAO fork block from %s", eval)
		}
	}

	if eval, ok := env["HIVE_FORK_DAO_VOTE"]; ok && eval != "" {
		// TODO
	}

	if eval, ok := env["HIVE_FORK_TANGERINE"]; ok && eval != "" {
		if genesis.Config.TangerineWhistleBlock, ok = (&big.Int{}).SetString(eval, 0); !ok {
			return nil, fmt.Errorf("can't set tangerine whistle block from %s", eval)
		}
	}

	if eval, ok := env["HIVE_FORK_SPURIOUS"]; ok && eval != "" {
		if genesis.Config.SpuriousDragonBlock, ok = (&big.Int{}).SetString(eval, 0); !ok {
			return nil, fmt.Errorf("can't set tangerine whistle block from %s", eval)
		}
	}

	if eval, ok := env["HIVE_FORK_BYZANTIUM"]; ok && eval != "" {
		if genesis.Config.ByzantiumBlock, ok = (&big.Int{}).SetString(eval, 0); !ok {
			return nil, fmt.Errorf("can't set byzantium block from %s", eval)
		}
	}

	if eval, ok := env["HIVE_FORK_CONSTANTINOPLE"]; ok && eval != "" {
		if genesis.Config.ConstantinopleBlock, ok = (&big.Int{}).SetString(eval, 0); !ok {
			return nil, fmt.Errorf("can't set constantinople block from %s", eval)
		}
	}

	if eval, ok := env["HIVE_FORK_PETERSBURG"]; ok && eval != "" {
		if genesis.Config.PetersburgBlock, ok = (&big.Int{}).SetString(eval, 0); !ok {
			return nil, fmt.Errorf("can't set petersburg block from %s", eval)
		}
	}

	if eval, ok := env["HIVE_FORK_ISTANBUL"]; ok && eval != "" {
		if genesis.Config.IstanbulBlock, ok = (&big.Int{}).SetString(eval, 0); !ok {
			return nil, fmt.Errorf("can't set istanbul block from %s", eval)
		}
	}

	if eval, ok := env["HIVE_FORK_MUIR_GLACIER"]; ok && eval != "" {
		if genesis.Config.MuirGlacierBlock, ok = (&big.Int{}).SetString(eval, 0); !ok {
			return nil, fmt.Errorf("can't set muir glacier block from %s", eval)
		}
	}

	if eval, ok := env["HIVE_FORK_BERLIN"]; ok && eval != "" {
		if genesis.Config.BerlinBlock, ok = (&big.Int{}).SetString(eval, 0); !ok {
			return nil, fmt.Errorf("can't set berlin block from %s", eval)
		}
	}

	if eval, ok := env["HIVE_FORK_LONDON"]; ok && eval != "" {
		if genesis.Config.LondonBlock, ok = (&big.Int{}).SetString(eval, 0); !ok {
			return nil, fmt.Errorf("can't set london block from %s", eval)
		}
	}

	if eval, ok := env["HIVE_TERMINAL_TOTAL_DIFFICULTY"]; ok && eval != "" {
		if genesis.Config.TerminalTotalDifficulty, ok = (&big.Int{}).SetString(eval, 0); !ok {
			return nil, fmt.Errorf("can't set terminal total difficulty from %s", eval)
		}
	}

	if eval, ok := env["HIVE_SHANGHAI_TIMESTAMP"]; ok && eval != "" {
		if genesis.Config.ShanghaiTime, ok = (&big.Int{}).SetString(eval, 0); !ok {
			return nil, fmt.Errorf("can't set shanghai time from %s", eval)
		}
	}

	if eval, ok := env["HIVE_CANCUN_TIMESTAMP"]; ok && eval != "" {
		if genesis.Config.CancunTime, ok = (&big.Int{}).SetString(eval, 0); !ok {
			return nil, fmt.Errorf("can't set cancun time from %s", eval)
		}
	}

	/*
	   # Replace config in input.
	   . + {
	     "config": {
	       "eip150Hash": env.HIVE_FORK_TANGERINE_HASH,
	       "eip155Block": env.HIVE_FORK_SPURIOUS|to_int,
	       "eip158Block": env.HIVE_FORK_SPURIOUS|to_int,
	     }|remove_empty
	   }
	*/

	return &genesis, nil
}
