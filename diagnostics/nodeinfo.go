package diagnostics

import (
	"encoding/json"
	"net/http"

	"github.com/ledgerwatch/erigon/turbo/node"
	"github.com/ledgerwatch/log/v3"
)

func SetupNodeInfoAccess(metricsMux *http.ServeMux, node *node.ErigonNode) {
	log.Info("[dbg] SetupNodeInfoAccess")
	metricsMux.HandleFunc("/nodeinfo", func(w http.ResponseWriter, r *http.Request) {
		log.Info("[dbg] /nodeinfo request")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		writeNodeInfo(w, node)
	})
}

func writeNodeInfo(w http.ResponseWriter, node *node.ErigonNode) {
	reply, err := node.Backend().NodesInfo(0)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(reply)
}
