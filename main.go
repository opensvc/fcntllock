// Package fcntllock provides simple whole file lock methods based on fcntl
//
// Lock functions create lock directory if absent
package fcntllock

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/opensvc/locker"
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
//
// On a Lock holding a shared lock, it converts it to an exclusive one, which
// fails while another process holds a shared lock too: the shared lock is
// kept then.
func (lck *Lock) TryLock() error {
	if err := createLockDir(lck.path); err != nil {
		return err
	}
	return lck.lock(false, syscall.F_WRLCK)
}

// TryRLock acquires a shared read file lock (non blocking): other processes
// may hold a shared lock on the file too, none an exclusive one.
//
// A fcntl lock is held by the process, not by the Lock: a process holds one
// lock on a file, whatever the Locks it locks it with, and closing any file
// descriptor of the file it holds open releases it. A process holding a
// shared lock is to open the file once, by this Lock, for as long as it holds
// it.
func (lck *Lock) TryRLock() error {
	if err := createLockDir(lck.path); err != nil {
		return err
	}
	return lck.lock(false, syscall.F_RDLCK)
}

// RLockContext repeat TryRLock with retry delay until succeed or context Done
func (lck *Lock) RLockContext(ctx context.Context, retryDelay time.Duration) error {
	if err := createLockDir(lck.path); err != nil {
		return err
	}
	return lck.try(ctx, lck.TryRLock, retryDelay)
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

func (lck *Lock) lock(blocking bool, lockType int16) (err error) {
	opened := false
	if lck.ReadWriteSeekCloser == nil {
		file, err := os.OpenFile(lck.path, os.O_CREATE|os.O_RDWR|os.O_SYNC, 0666)
		if err != nil {
			return err
		}
		lck.fd = file.Fd()
		lck.ReadWriteSeekCloser = file
		opened = true
	}
	ft := &syscall.Flock_t{
		Start:  0,
		Len:    0,
		Pid:    int32(os.Getpid()),
		Type:   lockType,
		Whence: io.SeekStart,
	}
	var cmd int
	if blocking {
		cmd = syscall.F_SETLKW
	} else {
		cmd = syscall.F_SETLK
	}
	if err = syscall.FcntlFlock(lck.fd, cmd, ft); err != nil && opened {
		// Closed only when opened by this call: closing a file the
		// Lock holds a lock on, as a shared lock it failed to
		// convert to an exclusive one, would release it.
		_ = lck.Close()
		lck.ReadWriteSeekCloser = nil
	}
	return
}

func (lck *Lock) try(ctx context.Context, fn func() error, retryDelay time.Duration) error {
	for {
		if err := fn(); err == nil {
			return nil
		} else if serr, ok := err.(syscall.Errno); !ok || (serr != syscall.EAGAIN) {
			// return immediately
			return err
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
