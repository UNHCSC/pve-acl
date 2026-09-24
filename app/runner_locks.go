package app

import (
	"context"
	"sync"
)

type (
	deploymentRunLock struct {
		gate chan struct{}
		refs int
	}

	deploymentRunLocks struct {
		mu    sync.Mutex
		locks map[int]*deploymentRunLock
	}
)

// acquire serializes runner state access for one deployment while remaining cancellable.
func (locks *deploymentRunLocks) acquire(ctx context.Context, deploymentID int) (releaseResult func(), errResult error) {
	locks.mu.Lock()
	if locks.locks == nil {
		locks.locks = make(map[int]*deploymentRunLock)
	}
	var lock *deploymentRunLock = locks.locks[deploymentID]
	if lock == nil {
		lock = &deploymentRunLock{gate: make(chan struct{}, 1)}
		locks.locks[deploymentID] = lock
	}
	lock.refs++
	locks.mu.Unlock()

	select {
	case lock.gate <- struct{}{}:
	case <-ctx.Done():
		locks.releaseReference(deploymentID, lock)
		return nil, ctx.Err()
	}

	var once sync.Once
	releaseResult = func() {
		once.Do(func() {
			<-lock.gate
			locks.releaseReference(deploymentID, lock)
		})
	}
	return
}

func (locks *deploymentRunLocks) releaseReference(deploymentID int, lock *deploymentRunLock) {
	locks.mu.Lock()
	defer locks.mu.Unlock()
	lock.refs--
	if lock.refs == 0 {
		delete(locks.locks, deploymentID)
	}
}
