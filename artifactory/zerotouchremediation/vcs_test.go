package zerotouchremediation

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jfrog/jfrog-client-go/xray/services"
)

// gitCheckout creates a repository with one commit on main, the given origin (none
// when empty), and an npm project in a subdirectory, which is where lockfiles
// usually live. It returns the repository root and the project directory.
func gitCheckout(t *testing.T, origin string) (repoRoot, projectDir string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	repoRoot = t.TempDir()
	projectDir = filepath.Join(repoRoot, "services", "web")
	require.NoError(t, os.MkdirAll(projectDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, "package-lock.json"), []byte("{}"), 0644))
	git := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@example.com", "-c", "commit.gpgsign=false"}, args...)...)
		cmd.Dir = repoRoot
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, string(out))
	}
	git("init", "-q", "-b", "main")
	if origin != "" {
		git("remote", "add", "origin", origin)
	}
	git("add", ".")
	git("commit", "-q", "-m", "init")
	return repoRoot, projectDir
}

func headRevision(t *testing.T, repoRoot string) string {
	t.Helper()
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = repoRoot
	out, err := cmd.Output()
	require.NoError(t, err)
	return string(out[:len(out)-1])
}

func TestReadVcs(t *testing.T) {
	t.Run("reads the repository the project is checked out from", func(t *testing.T) {
		repoRoot, projectDir := gitCheckout(t, "https://github.com/org/repo.git")
		assert.Equal(t, &services.ComponentResolutionVcs{
			Url: "https://github.com/org/repo.git", Branch: "main", Revision: headRevision(t, repoRoot),
		}, readVcs(projectDir))
	})
	t.Run("strips credentials from the origin URL", func(t *testing.T) {
		_, projectDir := gitCheckout(t, "https://user:secret-token@github.com/org/repo.git")
		vcs := readVcs(projectDir)
		require.NotNil(t, vcs)
		assert.NotContains(t, vcs.Url, "secret-token")
		assert.Equal(t, "https://github.com/org/repo.git", vcs.Url)
	})
	t.Run("sends nothing for a checkout without an origin remote", func(t *testing.T) {
		_, projectDir := gitCheckout(t, "")
		assert.Nil(t, readVcs(projectDir))
	})
	t.Run("sends nothing outside a git checkout", func(t *testing.T) {
		assert.Nil(t, readVcs(t.TempDir()))
	})
}

func TestRunIfEnabled_SendsTheLockfilesRepository(t *testing.T) {
	enableZTR(t)
	repoRoot, projectDir := gitCheckout(t, "https://github.com/org/repo.git")
	client := &mockClient{}
	tool := mockTool{root: projectDir, lockfiles: []Lockfile{{Path: "package-lock.json", Content: []byte("{}")}}}

	_, _, err := RunIfEnabled(context.Background(), client, "npm-virtual", tool, "install", projectDir, nil)
	require.NoError(t, err)
	require.Equal(t, 1, client.callCount)
	assert.Equal(t, &services.ComponentResolutionVcs{
		Url: "https://github.com/org/repo.git", Branch: "main", Revision: headRevision(t, repoRoot),
	}, client.lastReq.Vcs)
}
