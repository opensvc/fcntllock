package fcntllock_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/opensvc/locker"
	"github.com/opensvc/testhelper"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/fcntllock"
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
		t1 := time.Now()
		err := l.LockContext(ctx, 5*time.Millisecond)
		require.NotNil(t, err)
		require.Contains(t, err.Error(), "/dir: not a directory")
		t2 := time.Now()
		require.Less(t, t2.Sub(t1), 3*time.Millisecond)
	})

	for _, p := range []string{"/root/foo", "/var/.no-such-fcntllock-file"} {
		t.Run("fail fast on perm error and verify duration for file "+p, func(t *testing.T) {
			l := fcntllock.New(p)
			t1 := time.Now()
			ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
			defer cancel()
			err := l.LockContext(ctx, 25*time.Millisecond)
			require.Error(t, l.LockContext(ctx, 25*time.Millisecond))
			if runtime.GOOS == "darwin" && strings.HasPrefix(p, "/root/") {
				require.Contains(t, err.Error(), "read-only file system")
			} else {
				require.ErrorIs(t, err, os.ErrPermission)
			}
			t2 := time.Now()
			require.Less(t, t2.Sub(t1), 30*time.Millisecond)
		})
	}

	t.Run("LockContext give at least one try lock even if context is already Done", func(t *testing.T) {
		lockDir, cleanup := testhelper.Tempdir(t)
		defer cleanup()
		l := fcntllock.New(filepath.Join(lockDir, "dir", "lck"))
		ctx, cancel := context.WithTimeout(context.Background(), 0*time.Millisecond)
		defer cancel()
		err := l.LockContext(ctx, 5*time.Millisecond)
		require.Nil(t, err)
	})

	t.Run("when another process holds the lock", func(t *testing.T) {
		lockfile, tfCleanup := testhelper.TempFile(t)
		defer tfCleanup()
		l := fcntllock.New(lockfile)

		// start in fork a lock and holds it during 102 milliseconds
		forkCmd := lockInFork("LockContext", lockfile)
		require.NoError(t, forkCmd.Start())

		// give time for fork to runs
		time.Sleep(50 * time.Millisecond)

		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		require.Error(t, l.LockContext(ctx, 25*time.Millisecond))
		require.NoError(t, forkCmd.Wait())
	})

	t.Run("when another process releases the lock", func(t *testing.T) {
		lockfile, tfCleanup := testhelper.TempFile(t)
		defer tfCleanup()
		l := fcntllock.New(lockfile)

		// start in fork a lock and holds it during 102 milliseconds
		forkCmd := lockInFork("LockContext", lockfile)
		require.NoError(t, forkCmd.Start())

		// give time for fork to runs
		time.Sleep(50 * time.Millisecond)

		// the forked lock process ends in 102-50 milliseconds
		ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
		defer cancel()
		require.NoError(t, l.LockContext(ctx, 25*time.Millisecond))
		require.NoError(t, forkCmd.Wait())
	})
}

func TestTryLock(t *testing.T) {
	t.Run("lockfile is created", func(t *testing.T) {
		lockfile, tfCleanup := testhelper.TempFile(t)
		defer tfCleanup()
		l := fcntllock.New(lockfile)
		err := l.TryLock()
		require.Equal(t, nil, err)
		_, err = os.Stat(lockfile)
		require.Nil(t, err)
	})

	t.Run("create missing lock dir", func(t *testing.T) {
		lockDir, cleanup := testhelper.Tempdir(t)
		defer cleanup()
		l := fcntllock.New(filepath.Join(lockDir, "dir", "lck"))
		err := l.TryLock()
		require.Nil(t, err)
	})

	t.Run("fail fast if can't create lock dir", func(t *testing.T) {
		tf, cleanup := testhelper.TempFile(t)
		defer cleanup()
		l := fcntllock.New(filepath.Join(tf, "dir", "lck"))
		err := l.TryLock()
		require.NotNil(t, err)
		require.Contains(t, err.Error(), "/dir: not a directory")
	})

	t.Run("succeed", func(t *testing.T) {
		lockDir, cleanup := testhelper.Tempdir(t)
		defer cleanup()
		l := fcntllock.New(filepath.Join(lockDir, "dir", "lck"))
		err := l.TryLock()
		require.Nil(t, err)
	})

	t.Run("when another process holds the lock", func(t *testing.T) {
		lockfile, tfCleanup := testhelper.TempFile(t)
		defer tfCleanup()
		l := fcntllock.New(lockfile)
		// start in fork a lock and holds it during 102 milliseconds
		forkCmd := lockInFork("TryLock", lockfile)
		require.Nil(t, forkCmd.Start())
		time.Sleep(50 * time.Millisecond)
		require.Error(t, l.TryLock())
		require.NoError(t, forkCmd.Wait())
	})

	t.Run("after another lock process ends", func(t *testing.T) {
		lockfile, tfCleanup := testhelper.TempFile(t)
		defer tfCleanup()
		l := fcntllock.New(lockfile)

		// start in fork a lock and holds it during 102 milliseconds
		forkCmd := lockInFork("TryLock", lockfile)
		require.Nil(t, forkCmd.Start())
		time.Sleep(130 * time.Millisecond)
		require.NoError(t, l.TryLock())
		require.NoError(t, forkCmd.Wait())
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

func lockInFork(command string, args ...string) *exec.Cmd {
	cs := []string{"-test.run=TestHelperProcess", "--", command}
	cs = append(cs, args...)
	cmd := exec.Command(os.Args[0], cs...)
	cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
	return cmd
}

func TestHelperProcess(t *testing.T) {
	t.Helper()
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	args := os.Args
	for i, arg := range args {
		if arg == "--" {
			args = args[i+1:]
			break
		}
	}
	var exitCode int
	var cmd, name string
	var lock locker.Locker
	if len(args) > 1 {
		cmd = args[0]
		name = args[1]
		lock = fcntllock.New(name)
	}
	switch {
	case cmd == "TryLock":
		err := lock.TryLock()
		if err != nil {
			exitCode = 1
		} else {
			time.Sleep(102 * time.Millisecond)
			return
		}
	case cmd == "LockContext":
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		err := lock.LockContext(ctx, 102*time.Millisecond)
		if err != nil {
			exitCode = 1
		} else {
			time.Sleep(102 * time.Millisecond)
		}
	default:
		exitCode = 1
	}
	os.Exit(exitCode)
}
