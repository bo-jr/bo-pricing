// Command pricing answers POST /quote, burning CPU in proportion to the work
// requested. Everything except the pricing itself comes from bo-service-kit.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"

	"github.com/bo-jr/bo-service-kit/httpx"

	"github.com/bo-jr/bo-pricing/internal/quote"
)

// service is the in-cluster name: the metric `service` label, log field and
// OTel service.name. Never the bo- prefixed repo name.
const service = "pricing"

// maxBody caps a quote body; MaxLines of small JSON objects fit easily.
const maxBody = 64 << 10

func main() {
	if err := run(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	srv, err := httpx.New(ctx, service)
	if err != nil {
		return err
	}
	burn := srv.Chaos().CPUBurnFactor
	log := srv.Logger()

	srv.Handle("POST /quote", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req quote.Request
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBody)).Decode(&req); err != nil {
			http.Error(w, "body: "+err.Error(), http.StatusBadRequest)
			return
		}
		resp, err := quote.Price(r.Context(), req, burn)
		switch {
		case errors.Is(err, quote.ErrInvalid):
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		case err != nil:
			// The caller went away mid-burn; nobody is reading the answer.
			log.WarnContext(r.Context(), "quote abandoned", "err", err.Error())
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))

	return srv.Run(ctx)
}
