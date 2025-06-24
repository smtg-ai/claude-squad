package session

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"claude-squad/session/git"
)

func TestGetSessionPath(t *testing.T) {
	const repoPath = "/home/user/project"
	const worktreePath = "/tmp/worktree/test-session_1234567890"

	testWorktree := git.NewGitWorktreeFromStorage(
		repoPath,
		worktreePath,
		"test-session",
		"test-branch",
		"abc123",
	)

	tests := []struct {
		name     string
		instance *Instance
		want     string
		wantErr  bool
		errMsg   string
	}{
		{
			name: "at repository root",
			instance: &Instance{
				Path:        repoPath,
				gitWorktree: testWorktree,
			},
			want:    worktreePath,
			wantErr: false,
		},
		{
			name: "in subdirectory",
			instance: &Instance{
				Path:        "/home/user/project/src/api",
				gitWorktree: testWorktree,
			},
			want:    "/tmp/worktree/test-session_1234567890/src/api",
			wantErr: false,
		},
		{
			name: "started instance (resume case)",
			instance: &Instance{
				Path:        "/home/user/project/src",
				started:     true,
				gitWorktree: testWorktree,
			},
			want:    "/tmp/worktree/test-session_1234567890/src",
			wantErr: false,
		},
		{
			name: "nil worktree",
			instance: &Instance{
				Path:        "/home/user/project/src",
				gitWorktree: nil,
			},
			want:    "",
			wantErr: true,
			errMsg:  "git worktree not initialized",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.instance.GetSessionPath()

			if tt.wantErr {
				assert.Error(t, err)
				assert.EqualError(t, err, tt.errMsg)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
