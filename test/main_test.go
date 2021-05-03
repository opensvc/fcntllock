package fcntllock_test

import (
	"context"
	"github.com/opensvc/fcntllock"
	"github.com/opensvc/testhelper"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLockContext(t *testing.T) {
	t.Run("lockfile is created", func(t *testing.T) {
		lockfile, tfCleanup := testhelper.TempFile(t)
		defer tfCleanup()
		l := fcntllock.New(lockfile)
		ctx := context.Background()
		err := l.LockContext(ctx, 10*time.Millisecond)
		require.Equal(t, nil, err)
		_, err = os.Stat(lockfile)
		require.Nil(t, err)
	})

	t.Run("lock create missing lock dir", func(t *testing.T) {
		lockDir, cleanup := testhelper.Tempdir(t)
		defer cleanup()
		l := fcntllock.New(filepath.Join(lockDir, "dir", "lck"))
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()
		err := l.LockContext(ctx, 5*time.Millisecond)
		require.Nil(t, err)
	})

	t.Run("fail fast if can't create lock dir", func(t *testing.T) {
		tf, cleanup := testhelper.TempFile(t)
		defer cleanup()
		l := fcntllock.New(filepath.Join(tf, "dir", "lck"))
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()
		err := l.LockContext(ctx, 5*time.Millisecond)
		require.NotNil(t, err)
		require.Contains(t, err.Error(), "/dir: not a directory")
	})

	t.Run("LockContext give at least one try lock even if context is already Done", func(t *testing.T) {
		lockDir, cleanup := testhelper.Tempdir(t)
		defer cleanup()
		l := fcntllock.New(filepath.Join(lockDir, "dir", "lck"))
		ctx, cancel := context.WithTimeout(context.Background(), 0*time.Millisecond)
		defer cancel()
		err := l.LockContext(ctx, 5*time.Millisecond)
		require.Nil(t, err)
	})
}

func TestUnLock(t *testing.T) {
	t.Run("Ensure unlock (fcntl lock) succeed even if file is not locked", func(t *testing.T) {
		lockfile, tfCleanup := testhelper.TempFile(t)
		defer tfCleanup()
		l := fcntllock.New(lockfile)

		err := l.UnLock()
		require.Equal(t, nil, err)
	})
}
