package gitops

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	cryptossh "golang.org/x/crypto/ssh"
)

func buildAuth(method AuthMethod, secret string) (transport.AuthMethod, error) {
	switch method {
	case AuthMethodNone:
		return nil, nil
	case AuthMethodHTTPToken:
		// Usually for GitHub/GitLab tokens you use them as the password with a dummy username or 'oauth2'
		return &http.BasicAuth{
			Username: "oauth2", // or empty, depends on provider, but this works for most
			Password: secret,
		}, nil
	case AuthMethodSSHKey:
		publicKeys, err := ssh.NewPublicKeys("git", []byte(secret), "")
		if err != nil {
			return nil, fmt.Errorf("failed to parse SSH key: %w", err)
		}
		// Skip host key verification for simplicity in this MVP, or provide a way to trust keys
		publicKeys.HostKeyCallback = cryptossh.InsecureIgnoreHostKey()
		return publicKeys, nil
	default:
		return nil, fmt.Errorf("unknown auth method: %s", method)
	}
}

// SyncRepository will clone the repo if it doesn't exist, or pull if it does.
// Returns the current commit hash after the operation.
func SyncRepository(ctx context.Context, url, branch, targetPath string, method AuthMethod, secret string) (string, error) {
	auth, err := buildAuth(method, secret)
	if err != nil {
		return "", err
	}

	branchRef := plumbing.NewBranchReferenceName(branch)

	// Check if directory exists
	_, err = os.Stat(targetPath)
	if os.IsNotExist(err) {
		// Needs Clone
		return cloneRepo(ctx, url, branchRef, targetPath, auth)
	}

	// Try to open existing
	r, err := git.PlainOpen(targetPath)
	if err != nil {
		// Directory exists but isn't a repo. This is an error state.
		return "", fmt.Errorf("target path exists but is not a git repository: %w", err)
	}

	// Needs Pull
	return pullRepo(ctx, r, branchRef, auth)
}

func cloneRepo(ctx context.Context, url string, branchRef plumbing.ReferenceName, targetPath string, auth transport.AuthMethod) (string, error) {
	opts := &git.CloneOptions{
		URL:           url,
		Auth:          auth,
		ReferenceName: branchRef,
		SingleBranch:  true,
		Progress:      nil, // Could wire up an io.Writer for logs
	}

	r, err := git.PlainCloneContext(ctx, targetPath, false, opts)
	if err != nil {
		return "", fmt.Errorf("clone failed: %w", err)
	}

	return getHeadHash(r)
}

func pullRepo(ctx context.Context, r *git.Repository, branchRef plumbing.ReferenceName, auth transport.AuthMethod) (string, error) {
	w, err := r.Worktree()
	if err != nil {
		return "", fmt.Errorf("failed to get worktree: %w", err)
	}

	err = w.PullContext(ctx, &git.PullOptions{
		RemoteName:    "origin",
		ReferenceName: branchRef,
		SingleBranch:  true,
		Auth:          auth,
		Force:         true,
	})

	if err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
		// It's possible the remote URL changed or it diverged. For a simple GitOps agent,
		// if pull fails we might want to fetch and hard reset, but let's start with simple pull.

		// If the branch doesn't exist locally, we may need to fetch and checkout instead of pull.
		// Let's implement a forced fetch and hard reset as a robust fallback.
		return fetchAndReset(ctx, r, w, branchRef, auth)
	}

	return getHeadHash(r)
}

// fetchAndReset performs a fetch and a hard reset to ensure the local matches remote exactly,
// wiping out any local modifications.
func fetchAndReset(ctx context.Context, r *git.Repository, w *git.Worktree, branchRef plumbing.ReferenceName, auth transport.AuthMethod) (string, error) {
	err := r.FetchContext(ctx, &git.FetchOptions{
		RemoteName: "origin",
		Auth:       auth,
		RefSpecs:   []config.RefSpec{config.RefSpec(fmt.Sprintf("+%s:%s", branchRef, branchRef))},
		Force:      true,
	})
	if err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
		return "", fmt.Errorf("fetch failed: %w", err)
	}

	// Hard reset to the fetched branch tip
	remoteRefName := plumbing.NewRemoteReferenceName("origin", branchRef.Short())
	hash, err := r.ResolveRevision(plumbing.Revision(remoteRefName.String()))
	if err != nil {
		// Fallback to local branch if remote ref is tricky
		hash, err = r.ResolveRevision(plumbing.Revision(branchRef.String()))
		if err != nil {
			return "", fmt.Errorf("resolve revision failed: %w", err)
		}
	}

	err = w.Reset(&git.ResetOptions{
		Commit: *hash,
		Mode:   git.HardReset,
	})
	if err != nil {
		return "", fmt.Errorf("hard reset failed: %w", err)
	}

	// Clean untracked files
	err = w.Clean(&git.CleanOptions{
		Dir: true,
	})
	if err != nil {
		return "", fmt.Errorf("clean failed: %w", err)
	}

	return hash.String(), nil
}

func getHeadHash(r *git.Repository) (string, error) {
	ref, err := r.Head()
	if err != nil {
		return "", err
	}
	return ref.Hash().String(), nil
}
