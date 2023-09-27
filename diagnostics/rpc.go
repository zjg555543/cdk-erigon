package diagnostics

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/ledgerwatch/erigon/cmd/rpcdaemon/cli/httpcfg"
	"github.com/ledgerwatch/erigon/turbo/node"
)

func SetupRpcAccess(metricsMux *http.ServeMux, node *node.ErigonNode) {

	apis, err := node.Backend().APIs(context.Background(), httpcfg.HttpCfg{
		Enabled: true,
		API:     []string{"admin"},
	})

	if err != nil {
		metricsMux.HandleFunc("/rpc", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			urlPath := r.URL.Path

			if !strings.HasPrefix(urlPath, "/rpc/") {
				http.Error(w, fmt.Sprintf(`Unexpected path prefix: expected: "/dbs/..." got: "%s"`, urlPath), http.StatusNotFound)
				return
			}

			pathParts := strings.Split(urlPath[5:], "/")

			if len(pathParts) < 1 {
				http.Error(w, fmt.Sprintf(`Unexpected path len: expected: "{api}/{method}" got: "%s"`, urlPath), http.StatusNotFound)
				return
			}

			var apiService interface{}

			for _, api := range apis {
				if api.Namespace == pathParts[0] {
					apiService = api.Service
					break
				}
			}

			if apiService == nil {
				http.Error(w, fmt.Sprintf(`Can't find service for namespace: "%s"`, pathParts[0]), http.StatusNotFound)
			}
		})
	}
}

func callApi(w http.ResponseWriter, node *node.ErigonNode) {

}
