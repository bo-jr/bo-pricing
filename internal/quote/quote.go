// Package quote prices line items, burning CPU on purpose.
//
// The burn is the point of this service. Each line costs
// qty × CPU_BURN_FACTOR × IterationsPerUnit rounds of SHA-256, so latency
// scales with the work requested and, under concurrency, with contention for
// the pod's CPU limit. If pricing answered in constant time regardless of load,
// the latency SLO could never fail and Phase 6 scenario 4 would test nothing.
package quote

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
)

// IterationsPerUnit is the SHA-256 rounds for one unit of one line item at
// CPU_BURN_FACTOR=1. See BenchmarkBurn for what that costs.
const IterationsPerUnit = 1000

// Limits keep one request from monopolising the CPU budget indefinitely.
const (
	MaxLines = 20
	MaxQty   = 100
)

// ctxCheckEvery is how many rounds run between cancellation checks: frequent
// enough that an abandoned request stops burning within ~a millisecond, rare
// enough not to show up in the benchmark.
const ctxCheckEvery = 4096

// Line is one priced item.
type Line struct {
	SKU            string `json:"sku"`
	UnitPriceCents int64  `json:"unit_price_cents"`
	Qty            int    `json:"qty"`
}

// Request is the body of POST /quote.
type Request struct {
	Lines []Line `json:"lines"`
}

// Response is the answer to POST /quote.
type Response struct {
	TotalCents int64 `json:"total_cents"`
	// Iterations is the total SHA-256 rounds spent, so a caller can see the
	// burn it asked for.
	Iterations int64 `json:"iterations"`
	// Proof is the head of the final hash, which keeps the compiler from
	// optimising the burn away and makes the result checkable.
	Proof string `json:"proof"`
}

// ErrInvalid marks a request the caller must fix (a 400, not a 500).
var ErrInvalid = errors.New("invalid quote request")

// Validate checks a request before any CPU is spent on it.
func (r Request) Validate() error {
	if len(r.Lines) == 0 || len(r.Lines) > MaxLines {
		return fmt.Errorf("%w: want 1..%d lines, got %d", ErrInvalid, MaxLines, len(r.Lines))
	}
	for i, l := range r.Lines {
		switch {
		case l.SKU == "":
			return fmt.Errorf("%w: line %d has no sku", ErrInvalid, i)
		case l.Qty < 1 || l.Qty > MaxQty:
			return fmt.Errorf("%w: line %d qty %d outside 1..%d", ErrInvalid, i, l.Qty, MaxQty)
		case l.UnitPriceCents < 0:
			return fmt.Errorf("%w: line %d has a negative price", ErrInvalid, i)
		}
	}
	return nil
}

// Price validates r, burns CPU for every line, and totals the quote.
func Price(ctx context.Context, r Request, burnFactor int) (Response, error) {
	if err := r.Validate(); err != nil {
		return Response{}, err
	}
	var (
		resp  Response
		state [sha256.Size]byte
	)
	for _, l := range r.Lines {
		n := int64(l.Qty) * int64(burnFactor) * IterationsPerUnit
		var err error
		state, err = Burn(ctx, l.SKU, state, n)
		if err != nil {
			return Response{}, err
		}
		resp.Iterations += n
		resp.TotalCents += l.UnitPriceCents * int64(l.Qty)
	}
	resp.Proof = hex.EncodeToString(state[:8])
	return resp, nil
}

// Burn runs n rounds of SHA-256 seeded with seed and prev. It stops early, with
// ctx.Err(), once ctx is done.
func Burn(ctx context.Context, seed string, prev [sha256.Size]byte, n int64) ([sha256.Size]byte, error) {
	h := sha256.Sum256(append(prev[:], seed...))
	for i := int64(0); i < n; i++ {
		if i%ctxCheckEvery == 0 {
			if err := ctx.Err(); err != nil {
				return h, err
			}
		}
		h = sha256.Sum256(h[:])
	}
	return h, nil
}
