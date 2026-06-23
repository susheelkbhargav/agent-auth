package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/agent-auth/gateway/internal/store"
)

func TestMemNonceStore_ReplayRejected(t *testing.T) {
	t.Parallel()
	ns := store.NewMemNonceStore(time.Minute)
	ctx := context.Background()

	first, err := ns.SeenBefore(ctx, "jti-1")
	if err != nil || first {
		t.Fatalf("first use: seen=%v err=%v", first, err)
	}
	second, err := ns.SeenBefore(ctx, "jti-1")
	if err != nil || !second {
		t.Fatalf("replay: seen=%v err=%v, want seen=true", second, err)
	}
}

func TestMemNonceStore_EmptyNonceFailClosed(t *testing.T) {
	t.Parallel()
	ns := store.NewMemNonceStore(time.Minute)
	seen, err := ns.SeenBefore(context.Background(), "")
	if err != nil || !seen {
		t.Fatalf("empty nonce: seen=%v err=%v, want fail-closed", seen, err)
	}
}

func TestMemNonceStore_TTLExpiry(t *testing.T) {
	t.Parallel()
	ns := store.NewMemNonceStore(10 * time.Millisecond)
	ctx := context.Background()

	if seen, _ := ns.SeenBefore(ctx, "jti-2"); seen {
		t.Fatal("first use marked seen")
	}
	time.Sleep(15 * time.Millisecond)
	if seen, _ := ns.SeenBefore(ctx, "jti-2"); seen {
		t.Fatal("expected expired nonce to be accepted again")
	}
}
