package diagnostics

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/ledgerwatch/erigon/cmd/rpcdaemon/cli/httpcfg"
	"github.com/ledgerwatch/erigon/rpc"
	"github.com/ledgerwatch/erigon/turbo/node"
	"github.com/ledgerwatch/log/v3"
)

func SetupRpcAccess(metricsMux *http.ServeMux, node *node.ErigonNode, logger log.Logger) error {

	apis, err := node.Backend().APIs(context.Background(), httpcfg.HttpCfg{
		Enabled: true,
		API:     []string{"admin"},
	})

	srv := rpc.NewServer(50, false, true, logger)

	for _, api := range apis {
		if err := srv.RegisterName(api.Namespace, api.Service); err != nil {
			return err
		}
	}

	var id int

	nextid := func() int {
		next := id
		id++
		return next
	}

	if err != nil {
		metricsMux.HandleFunc("/rpc", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			urlPath := r.URL.Path

			if !strings.HasPrefix(urlPath, "/rpc/") {
				http.Error(w, fmt.Sprintf(`Unexpected path prefix: expected: "/rpc/..." got: "%s"`, urlPath), http.StatusNotFound)
				return
			}

			pathParts := strings.Split(urlPath[5:], "/")

			const template = `{"jsonrpc":"2.0","method":%q,"params":[%s],"id":%d}`

			switch len(pathParts) {
			case 0:
				srv.ServeHTTP(w, r)
			case 1:
				params := r.URL.Query().Get("params")

				r, err := http.NewRequestWithContext(r.Context(), "POST", "",
					bytes.NewBufferString(fmt.Sprintf(template, pathParts[0], params, nextid())))

				if err != nil {
					http.Error(w, fmt.Sprintf(`Can't format jrpc request for "%s" with params "%s"`, pathParts[0], params), http.StatusBadRequest)
				}

				srv.ServeHTTP(w, r)
			case 2:
				method := pathParts[0] + "_" + pathParts[1]
				params := r.URL.Query().Get("params")

				r, err := http.NewRequestWithContext(r.Context(), "POST", "",
					bytes.NewBufferString(fmt.Sprintf(template, method, params, nextid())))

				if err != nil {
					http.Error(w, fmt.Sprintf(`Can't format jrpc request for "%s" with params "%s"`, method, params), http.StatusBadRequest)
				}

				srv.ServeHTTP(w, r)
			default:
				http.Error(w, fmt.Sprintf(`Unexpected path parts in "%s"`, urlPath), http.StatusNotFound)
			}

		})
	}

	return nil
}
