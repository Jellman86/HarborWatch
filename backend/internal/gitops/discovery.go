package gitops

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var composeFileNames = map[string]struct{}{
	"docker-compose.yml":  {},
	"docker-compose.yaml": {},
	"compose.yml":         {},
	"compose.yaml":        {},
}

type RepositoryFiles struct {
	ComposeFiles []string `json:"composeFiles"`
	EnvFiles     []string `json:"envFiles"`
}

// DiscoverRepositoryFiles scans a synced repository and returns compose/env file candidates.
func DiscoverRepositoryFiles(repoPath string) (RepositoryFiles, error) {
	root := strings.TrimSpace(repoPath)
	if root == "" {
		return RepositoryFiles{}, fmt.Errorf("repository path is empty")
	}

	info, err := os.Stat(root)
	if err != nil {
		return RepositoryFiles{}, err
	}
	if !info.IsDir() {
		return RepositoryFiles{}, fmt.Errorf("repository path is not a directory")
	}

	composeSet := map[string]struct{}{}
	envSet := map[string]struct{}{}
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}
		if d.IsDir() {
			return nil
		}

		name := strings.ToLower(d.Name())
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == "." {
			return nil
		}

		if _, ok := composeFileNames[name]; ok {
			composeSet[rel] = struct{}{}
		}
		if strings.HasPrefix(name, ".env") {
			envSet[rel] = struct{}{}
		}
		return nil
	})
	if err != nil {
		return RepositoryFiles{}, err
	}

	out := RepositoryFiles{
		ComposeFiles: mapKeysSorted(composeSet),
		EnvFiles:     mapKeysSorted(envSet),
	}
	return out, nil
}

func mapKeysSorted(values map[string]struct{}) []string {
	out := make([]string, 0, len(values))
	for k := range values {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
