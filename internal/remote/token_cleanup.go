package remote

import "fmt"

// stripCloneTokenFromRemote rewrites origin on the remote host to a tokenless URL
// after an authenticated clone. Failure must abort provision: leaving the OAuth
// token in ~/.git/config on the remote is a credential leak.
func stripCloneTokenFromRemote(host, repoName, cleanURL string) error {
	if cleanURL == "" {
		return nil
	}
	fixRemote := fmt.Sprintf("cd ~/%s && git remote set-url origin %s", repoName, cleanURL)
	if err := remoteSSH(host, fixRemote); err != nil {
		return fmt.Errorf("failed to strip OAuth token from remote .git/config: %w (token may still be embedded in origin URL — fix manually or re-run provision)", err)
	}
	return nil
}
