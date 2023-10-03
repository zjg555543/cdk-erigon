package args

import (
	"fmt"
	"math/big"
	"net"
	"path/filepath"
	"strconv"

	"github.com/ledgerwatch/erigon/core/types"
	"github.com/ledgerwatch/erigon/devnet/accounts"
	"github.com/ledgerwatch/erigon/devnet/requests"
	"github.com/ledgerwatch/erigon/params/networkname"
)

type Node struct {
	requests.RequestGenerator `arg:"-"`
	Name                      string `arg:"-"`
	BuildDir                  string `arg:"positional" default:"./build/bin/devnet" json:"builddir"`
	DataDir                   string `arg:"--datadir" default:"./dev" json:"datadir"`
	Chain                     string `arg:"--chain" default:"dev" json:"chain"`
	NetworkId                 int    `arg:"--networkid" default:"dev" json:"networkid"`
	Port                      int    `arg:"--port" json:"port,omitempty"`
	AllowedPorts              string `arg:"--p2p.allowed-ports" json:"p2p.allowed-ports,omitempty"`
	NAT                       string `arg:"--nat" default:"none" json:"nat"`
	NoDiscover                string `arg:"--nodiscover" flag:"" default:"true" json:"nodiscover"`
	ConsoleVerbosity          string `arg:"--log.console.verbosity" default:"0" json:"log.console.verbosity"`
	DirVerbosity              string `arg:"--log.dir.verbosity" json:"log.dir.verbosity,omitempty"`
	LogDirPath                string `arg:"--log.dir.path" json:"log.dir.path,omitempty"`
	LogDirPrefix              string `arg:"--log.dir.prefix" json:"log.dir.prefix,omitempty"`
	P2PProtocol               string `arg:"--p2p.protocol" default:"68" json:"p2p.protocol"`
	Snapshots                 bool   `arg:"--snapshots" flag:"" default:"false" json:"snapshots,omitempty"`
	Downloader                string `arg:"--no-downloader" default:"true" json:"no-downloader"`
	TorrentPort               string `arg:"--torrent.port" default:"42070" json:"torrent.port"`
	WS                        bool   `arg:"--ws" flag:"" default:"true" json:"ws"`
	Http                      bool   `arg:"--http" flag:"" default:"true" json:"http"`
	HttpApi                   string `arg:"--http.api" json:"http.api"`
	HttpAddr                  string `arg:"--http.addr" json:"http.addr"`
	HttpPort                  int    `arg:"--http.port" default:"8545" json:"http.port"`
	HttpVHosts                string `arg:"--http.vhosts" json:"http.vhosts"`
	HttpCorsDomain            string `arg:"--http.corsdomain" json:"http.corsdomain"`
	PrivateApiAddr            string `arg:"--private.api.addr" default:"localhost:9090" json:"private.api.addr"`
	AuthRpcAddr               string `arg:"--authrpc.addr" default:"8551" json:"authrpc.addr"`
	AuthRpcPort               int    `arg:"--authrpc.port" default:"8551" json:"authrpc.port"`
	AuthRpcVHosts             string `arg:"--authrpc.vhosts" json:"authrpc.vhosts"`
	WSPort                    int    `arg:"--ws.port" json:"-ws.port"` // flag not defined
	GRPCPort                  int    `arg:"-" default:"8547" json:"-"` // flag not defined
	TCPPort                   int    `arg:"-" default:"8548" json:"-"` // flag not defined
	Metrics                   bool   `arg:"--metrics" flag:"" default:"false" json:"metrics"`
	MetricsPort               int    `arg:"--metrics.port" json:"metrics.port,omitempty"`
	MetricsAddr               string `arg:"--metrics.addr" json:"metrics.addr,omitempty"`
	StaticPeers               string `arg:"--staticpeers" json:"staticpeers,omitempty"`
	WithoutHeimdall           bool   `arg:"--bor.withoutheimdall" flag:"" default:"false" json:"bor.withoutheimdall,omitempty"`
	HeimdallGRpc              string `arg:"--bor.heimdallgRPC" json:"bor.heimdallgRPC,omitempty"`
	VMDebug                   bool   `arg:"--vmdebug" flag:"" default:"false" json:"vmdebug,omitempty"`
	FakePOW                   bool   `arg:"--fakepow" flag:"" default:"false" json:"fakepow,omitempty"`
	WSHasOwnPort              bool   `arg:"-"`
}

const RPCPortsPerNode = 5

func (node *Node) configure(base Node, nodeNumber int, genesis *types.Genesis) error {

	networkId := base.NetworkId

	if networkId == 0 && genesis != nil {
		networkId = int(genesis.Config.ChainID.Uint64())
	}

	chain := base.Chain

	if len(chain) == 0 && genesis != nil {
		chain = genesis.Config.ChainName
	}

	switch {
	case len(chain) > 0:
		node.Name = fmt.Sprintf("%s-%d", chain, nodeNumber)
	case networkId > 0:
		node.Name = fmt.Sprintf("%d.%d", networkId, nodeNumber)
	default:
		node.Name = fmt.Sprintf("%d", nodeNumber)
	}

	node.NetworkId = networkId

	node.DataDir = filepath.Join(base.DataDir, node.Name)

	node.LogDirPath = filepath.Join(base.DataDir, "logs")
	node.LogDirPrefix = node.Name

	node.Chain = chain

	node.StaticPeers = base.StaticPeers

	node.Metrics = base.Metrics
	node.MetricsPort = base.MetricsPort
	node.MetricsAddr = base.MetricsAddr

	node.Snapshots = base.Snapshots

	var err error

	node.PrivateApiAddr, _, err = portFromBase(base.PrivateApiAddr, nodeNumber, 1)

	if err != nil {
		return err
	}

	apiPort := base.HttpPort + (nodeNumber * RPCPortsPerNode)

	node.HttpPort = apiPort

	if node.WS {
		if node.WSHasOwnPort {
			node.WSPort = apiPort + 1
		}
	}

	node.GRPCPort = apiPort + 2
	node.TCPPort = apiPort + 3
	node.AuthRpcPort = apiPort + 4

	node.Port = base.Port + nodeNumber

	return nil
}

func (node Node) ChainID() *big.Int {
	return big.NewInt(int64(node.NetworkId))
}

type BlockProducer struct {
	Node
	Mine                bool   `arg:"--mine" flag:"true"`
	Etherbase           string `arg:"--miner.etherbase"`
	DevPeriod           int    `arg:"--dev.period"`
	BorPeriod           int    `arg:"--bor.period"`
	BorMinBlockSize     int    `arg:"--bor.minblocksize"`
	AccountSlots        int    `arg:"--txpool.accountslots" default:"16"`
	MinerExtraData      string `arg:"--miner.extradata"`
	MinerSigningKeyFile string `arg:"--miner.sigfile"`
	account             *accounts.Account
}

func (m BlockProducer) Configure(baseNode Node, nodeNumber int, genesis *types.Genesis, files map[string][]byte) (int, interface{}, error) {
	err := m.configure(baseNode, nodeNumber, genesis)

	if err != nil {
		return -1, nil, err
	}

	switch m.Chain {
	case networkname.DevChainName:
		if m.DevPeriod == 0 {
			m.DevPeriod = 30
		}
		m.account = accounts.NewAccount(m.Name() + "-etherbase")

	case networkname.BorDevnetChainName:
		m.account = accounts.NewAccount(m.Name() + "-etherbase")

		if len(m.HttpApi) == 0 {
			m.HttpApi = "admin,eth,erigon,web3,net,debug,trace,txpool,parity,ots,bor"
		}
	}

	if len(m.HttpApi) == 0 {
		m.HttpApi = "admin,eth,erigon,web3,net,debug,trace,txpool,parity,ots"
	}

	if m.account != nil {
		m.Etherbase = m.account.Address.Hex()
	}

	if _, ok := files["private_key.txt"]; ok {
		m.MinerSigningKeyFile = filepath.Join(m.Node.DataDir, "private_key.txt")
	}

	return m.HttpPort, m, nil
}

func (n BlockProducer) Name() string {
	return n.Node.Name
}

func (n BlockProducer) DataDir() string {
	return n.Node.DataDir
}

func (n BlockProducer) Account() *accounts.Account {
	return n.account
}

func (n BlockProducer) IsBlockProducer() bool {
	return true
}

type NonBlockProducer struct {
	Node
}

func (n NonBlockProducer) Configure(baseNode Node, nodeNumber int, genesis *types.Genesis, files map[string][]byte) (int, interface{}, error) {
	err := n.configure(baseNode, nodeNumber, genesis)

	if err != nil {
		return -1, nil, err
	}

	if len(n.HttpApi) == 0 {
		n.HttpApi = "admin,eth,erigon,web3,net,debug,trace,txpool"
	}

	return n.HttpPort, n, nil
}

func (n NonBlockProducer) Name() string {
	return n.Node.Name
}

func (n NonBlockProducer) IsBlockProducer() bool {
	return false
}

func (n NonBlockProducer) DataDir() string {
	return n.Node.DataDir
}

func (n NonBlockProducer) Account() *accounts.Account {
	return nil
}

func portFromBase(baseAddr string, increment int, portCount int) (string, int, error) {
	apiHost, apiPort, err := net.SplitHostPort(baseAddr)

	if err != nil {
		return "", -1, err
	}

	portNo, err := strconv.Atoi(apiPort)

	if err != nil {
		return "", -1, err
	}

	portNo += (increment * portCount)

	return fmt.Sprintf("%s:%d", apiHost, portNo), portNo, nil
}
