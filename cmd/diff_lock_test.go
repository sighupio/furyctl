// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package cmd //nolint:testpackage // exercises the unexported steps of the command.

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sighupio/furyctl/internal/apis/config"
	commcreate "github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/common/create"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/lockfile"
	dist "github.com/sighupio/furyctl/pkg/distribution"
)

const lockedCluster = "diff-lock"

// lockedSteps are the steps of the full comparison with the real lock file of apply, in a temp dir of the test.
func lockedSteps(t *testing.T, r *stepsRecorder) (diffSteps, string) {
	t.Helper()

	t.Setenv("TMPDIR", t.TempDir())

	steps := r.diffSteps()
	steps.downloadDistribution = func() (dist.DownloadResult, error) {
		r.record("distribution download")

		return dist.DownloadResult{MinimalConf: config.Furyctl{Metadata: config.FuryctlMeta{Name: lockedCluster}}}, nil
	}
	steps.lock = func(res dist.DownloadResult) (func(), error) {
		return lockRunDir(res.MinimalConf.Metadata.Name)
	}

	return steps, lockfile.NewLockFile(lockedCluster).Path
}

// While apply holds its lock file, diff stops before it touches the run dir, prints nothing and leaves the file.
func TestRunFullDiffRefusesToRunWhileApplyHoldsTheLock(t *testing.T) {
	r := &stepsRecorder{config: noChanges, creator: fakeCreator{}}
	steps, lockPath := lockedSteps(t, r)

	require.Equal(t, filepath.Join(os.TempDir(), "furyctl-"+lockedCluster), lockPath)
	require.NoError(t, os.WriteFile(lockPath, []byte("4242"), 0o600))

	err := runFullDiff(steps)

	require.ErrorIs(t, err, lockfile.ErrLockFileExists)
	require.ErrorContains(t, err, "error while verifying lock file "+lockPath)
	require.ErrorContains(t, err, "4242")
	require.NotErrorIs(t, err, ErrDiffNotCompared)
	require.Equal(t, []string{"distribution download"}, r.steps)
	require.Empty(t, r.out.String())

	held, readErr := os.ReadFile(lockPath)
	require.NoError(t, readErr)
	require.Equal(t, "4242", string(held))
}

// diff holds the lock file while it works, with its PID, and removes it when it returns, on success or not.
func TestRunFullDiffHoldsTheLockAndRemovesIt(t *testing.T) {
	tests := map[string]struct {
		fail    func(*stepsRecorder, *diffSteps)
		wantErr bool
	}{
		"success": {fail: func(*stepsRecorder, *diffSteps) {}},
		"dependency download fails": {
			fail: func(_ *stepsRecorder, s *diffSteps) {
				s.downloadDependencies = func(dist.DownloadResult) error { return errDownloadsFail }
			},
			wantErr: true,
		},
		"creator fails": {
			fail: func(_ *stepsRecorder, s *diffSteps) {
				s.newCreator = func(dist.DownloadResult) (cluster.Creator, error) { return nil, errDownloadsFail }
			},
			wantErr: true,
		},
		"diff fails": {
			fail: func(r *stepsRecorder, _ *diffSteps) {
				r.creator = fakeDiffer{diff: func(func()) ([]commcreate.Section, error) { return nil, errDownloadsFail }}
			},
			wantErr: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			r := &stepsRecorder{config: noChanges, creator: fakeCreator{}}
			steps, lockPath := lockedSteps(t, r)

			held := ""
			compare := steps.compareConfiguration
			steps.compareConfiguration = func(res dist.DownloadResult) configurationResult {
				content, err := os.ReadFile(lockPath)
				require.NoError(t, err)

				held = string(content)

				return compare(res)
			}

			tc.fail(r, &steps)

			err := runFullDiff(steps)

			if tc.wantErr {
				require.ErrorIs(t, err, errDownloadsFail)
			} else {
				require.NoError(t, err)
			}

			require.Equal(t, strconv.Itoa(os.Getpid()), held)
			require.NoFileExists(t, lockPath)
		})
	}
}

// A second diff for the same cluster is refused while the first holds the lock, as apply is.
func TestLockRunDirRefusesASecondHolder(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())

	unlock, err := lockRunDir(lockedCluster)
	require.NoError(t, err)

	_, err = lockRunDir(lockedCluster)
	require.ErrorIs(t, err, lockfile.ErrLockFileExists)

	unlock()

	require.NoFileExists(t, lockfile.NewLockFile(lockedCluster).Path)

	unlock, err = lockRunDir(lockedCluster)
	require.NoError(t, err)

	unlock()
}
