// Package fcntllock provides simple whole file lock methods based on fcntl
//
// Lock functions create lock directory if absent
package fcntllock

import (
	"context"
	"errors"
	"github.com/opensvc/locker"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

type (
	Locker = locker.Locker

	ReadWriteSeekCloser interface {
		io.ReadWriteSeeker
		io.Closer
	}

	// Lock implement fcntl lock features
	Lock struct {
		path string
		ReadWriteSeekCloser
		fd uintptr
	}
)

var (
	lockDirPerm os.FileMode = 0700
)

// New create a new fcntl lock
func New(path string) Locker {
	return &Lock{
		path: path,
	}
}

// TryLock acquires an exclusive write file lock (non blocking)
func (lck *Lock) TryLock() error {
	if err := createLockDir(lck.path); err != nil {
		return err
	}
	return lck.lock(false)
}

// UnLock release lock
func (lck Lock) UnLock() (err error) {
	ft := &syscall.Flock_t{
		Start:  0,
		Len:    0,
		Pid:    0,
		Type:   syscall.F_UNLCK,
		Whence: io.SeekStart,
	}
	err = syscall.FcntlFlock(lck.fd, syscall.F_SETLK, ft)
	return
}

// LockContext repeat TryLock with retry delay until succeed or context Done
func (lck *Lock) LockContext(ctx context.Context, retryDelay time.Duration) error {
	if err := createLockDir(lck.path); err != nil {
		return err
	}
	return lck.try(ctx, lck.TryLock, retryDelay)
}

func (lck *Lock) lock(blocking bool) (err error) {
	if lck.ReadWriteSeekCloser == nil {
		file, err := os.OpenFile(lck.path, os.O_CREATE|os.O_RDWR|os.O_SYNC, 0666)
		if err != nil {
			return err
		}
		lck.fd = file.Fd()
		lck.ReadWriteSeekCloser = file
	}
	ft := &syscall.Flock_t{
		Start:  0,
		Len:    0,
		Pid:    int32(os.Getpid()),
		Type:   syscall.F_WRLCK,
		Whence: io.SeekStart,
	}
	var cmd int
	if blocking {
		cmd = syscall.F_SETLKW
	} else {
		cmd = syscall.F_SETLK
	}
	if err = syscall.FcntlFlock(lck.fd, cmd, ft); err != nil {
		_ = lck.Close()
		lck.ReadWriteSeekCloser = nil
	}
	return
}

func (lck *Lock) try(ctx context.Context, fn func() error, retryDelay time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	for {
		if err := fn(); err == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			// context reach end
			return ctx.Err()
		case <-time.After(retryDelay):
			// will try again fn()
		}
	}
}

func createLockDir(path string) (err error) {
	dir := filepath.Dir(path)
	info, err := os.Stat(dir)
	if err == nil {
		if info.IsDir() {
			return
		}
		return errors.New("already exists and is not directory: " + dir)
	}
	if os.IsNotExist(err) {
		return os.MkdirAll(dir, lockDirPerm)
	}
	return err
}
