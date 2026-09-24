package app

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestDeploymentRunLocksSerializeAndCancelWaiters(t *testing.T) {
	var locks deploymentRunLocks
	var release func()
	var err error
	if release, err = locks.acquire(context.Background(), 12); err != nil {
		t.Fatal(err)
	}
	var ctx context.Context
	var cancel context.CancelFunc
	ctx, cancel = context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err = locks.acquire(ctx, 12); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected the same deployment to remain locked, got %v", err)
	}
	var otherRelease func()
	if otherRelease, err = locks.acquire(context.Background(), 13); err != nil {
		t.Fatalf("another deployment should not be blocked: %v", err)
	}
	otherRelease()
	release()
	if len(locks.locks) != 0 {
		t.Fatalf("unused deployment locks were retained: %#v", locks.locks)
	}
}
