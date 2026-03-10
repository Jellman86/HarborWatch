package composeexec

import "strings"

type Context struct {
	WorkingDir  string
	ConfigFiles []string
	EnvFiles    []string
}

func New(workingDir string, configFiles, envFiles []string) Context {
	return Context{
		WorkingDir:  strings.TrimSpace(workingDir),
		ConfigFiles: dedupePaths(configFiles),
		EnvFiles:    dedupePaths(envFiles),
	}
}

func (c Context) RunnerArgs(subcommand ...string) []string {
	args := make([]string, 0, len(c.ConfigFiles)*2+len(c.EnvFiles)*2+len(subcommand))
	for _, file := range c.ConfigFiles {
		args = append(args, "-f", file)
	}
	for _, file := range c.EnvFiles {
		args = append(args, "--env-file", file)
	}
	args = append(args, subcommand...)
	return args
}

func (c Context) DockerArgs(subcommand ...string) []string {
	args := []string{"compose"}
	args = append(args, c.RunnerArgs(subcommand...)...)
	return args
}

func dedupePaths(paths []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(paths))
	for _, raw := range paths {
		p := strings.TrimSpace(raw)
		if p == "" {
			continue
		}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	return out
}
