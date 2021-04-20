package fcntllock_test

import (
	"context"
	"github.com/opensvc/fcntllock"
	"github.com/opensvc/testhelper"
	"github.com/stretchr/testify/assert"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLock(t *testing.T) {
	t.Run("lockfile is created", func(t *testing.T) {
		lockfile, tfCleanup := testhelper.TempFile(t)
		defer tfCleanup()
		l := fcntllock.New(lockfile)
		ctx := context.Background()
		err := l.LockContext(ctx, 10*time.Millisecond)
		assert.Equal(t, nil, err)
		_, err = os.Stat(lockfile)
		assert.Nil(t, err)
	})

	t.Run("lock create missing lock dir", func(t *testing.T) {
		lockDir, cleanup := testhelper.Tempdir(t)
		defer cleanup()
		l := fcntllock.New(filepath.Join(lockDir, "dir", "lck"))
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()
		err := l.LockContext(ctx, 5*time.Millisecond)
		assert.Nil(t, err)
	})

	t.Run("fail fast if can't create lock dir", func(t *testing.T) {
		tf, cleanup := testhelper.TempFile(t)
		defer cleanup()
		l := fcntllock.New(filepath.Join(tf, "dir", "lck"))
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()
		err := l.LockContext(ctx, 5*time.Millisecond)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "/dir: not a directory")
	})
}

func TestUnLock(t *testing.T) {
	t.Run("Ensure unlock (fcntl lock) succeed even if file is not locked", func(t *testing.T) {
		lockfile, tfCleanup := testhelper.TempFile(t)
		defer tfCleanup()
		l := fcntllock.New(lockfile)

		err := l.UnLock()
		assert.Equal(t, nil, err)
	})
}
