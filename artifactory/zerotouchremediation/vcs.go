package zerotouchremediation

import (
	"os"
	"path/filepath"

	clientutils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/jfrog/jfrog-client-go/xray/services"
)

// readVcs returns the git repository and commit projectRoot is checked out from, so
// Xray can record where the remediated lockfile lives and later open a fix pull
// request against it. Best-effort like the rest of remediation: outside a git
// checkout, without an origin remote, or when git details cannot be read, it returns
// nil and the request is sent without them.
func readVcs(projectRoot string) *services.ComponentResolutionVcs {
	gitRoot, found := findGitRoot(projectRoot)
	if !found {
		log.Debug("Zero Touch Remediation: project is not in a git checkout; sending no vcs details")
		return nil
	}
	manager := clientutils.NewGitManager(gitRoot)
	if err := manager.ReadConfig(); err != nil {
		log.Debug("Zero Touch Remediation: could not read git details, sending none: ", err.Error())
		return nil
	}
	// GitManager appends ".git" to whatever origin URL it found, so a checkout with
	// no origin remote reports the bare suffix rather than an empty URL.
	url := manager.GetUrl()
	if url == "" || url == ".git" {
		log.Debug("Zero Touch Remediation: git checkout has no origin remote; sending no vcs details")
		return nil
	}
	return &services.ComponentResolutionVcs{Url: url, Branch: manager.GetBranch(), Revision: manager.GetRevision()}
}

// findGitRoot walks up from dir to the directory holding .git (a directory, or a file
// for worktrees and submodules). The npm project is often a subdirectory of the
// repository, so the project root itself is usually not it.
func findGitRoot(dir string) (string, bool) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return "", false
	}
	for {
		if _, err = os.Lstat(filepath.Join(dir, ".git")); err == nil {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}
