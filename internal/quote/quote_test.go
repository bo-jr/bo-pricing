package quote

import (
	"context"
	"crypto/sha256"
	"errors"
	"testing"
	"time"
)

func TestPriceTotalsAndScalesWork(t *testing.T) {
	req := Request{Lines: []Line{{SKU: "SKU-0001", UnitPriceCents: 250, Qty: 2}, {SKU: "SKU-0002", UnitPriceCents: 100, Qty: 3}}}
	resp, err := Price(context.Background(), req, 4)
	if err != nil {
		t.Fatal(err)
	}
	if resp.TotalCents != 800 {
		t.Fatalf("total %d, want 800", resp.TotalCents)
	}
	if want := int64((2 + 3) * 4 * IterationsPerUnit); resp.Iterations != want {
		t.Fatalf("iterations %d, want %d", resp.Iterations, want)
	}
	again, _ := Price(context.Background(), req, 4)
	if again.Proof != resp.Proof || len(resp.Proof) != 16 {
		t.Fatalf("proof must be deterministic 16 hex chars: %q vs %q", resp.Proof, again.Proof)
	}
}

func TestValidate(t *testing.T) {
	bad := []Request{
		{},
		{Lines: make([]Line, MaxLines+1)},
		{Lines: []Line{{SKU: "", Qty: 1}}},
		{Lines: []Line{{SKU: "a", Qty: 0}}},
		{Lines: []Line{{SKU: "a", Qty: MaxQty + 1}}},
		{Lines: []Line{{SKU: "a", Qty: 1, UnitPriceCents: -1}}},
	}
	for i, r := range bad {
		if _, err := Price(context.Background(), r, 1); !errors.Is(err, ErrInvalid) {
			t.Errorf("case %d: want ErrInvalid, got %v", i, err)
		}
	}
}

func TestBurnStopsWhenCancelled(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := Burn(ctx, "x", [sha256.Size]byte{}, 1<<40) // hours of work
	if !errors.Is(err, context.DeadlineExceeded) || time.Since(start) > time.Second {
		t.Fatalf("burn did not stop on cancel: err=%v after %v", err, time.Since(start))
	}
}

// BenchmarkBurn measures one unit of work at CPU_BURN_FACTOR=1. Calibrate the
// chart's CPU_BURN_FACTOR from it: factor ≈ target_ms / (ns_per_op / 1e6).
func BenchmarkBurn(b *testing.B) {
	for b.Loop() {
		_, _ = Burn(context.Background(), "SKU-0001", [sha256.Size]byte{}, IterationsPerUnit)
	}
}
