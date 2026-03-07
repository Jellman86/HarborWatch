package composecli

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

type Runner struct {
	Binary     string
	PrefixArgs []string
}

var (
	lookPath       = exec.LookPath
	commandContext = exec.CommandContext
)

func Resolve(ctx context.Context) (Runner, error) {
	if _, err := lookPath("docker"); err == nil {
		if err := probe(ctx, "docker", "compose", "version"); err == nil {
			return Runner{Binary: "docker", PrefixArgs: []string{"compose"}}, nil
		}
	}
	if _, err := lookPath("docker-compose"); err == nil {
		if err := probe(ctx, "docker-compose", "version"); err == nil {
			return Runner{Binary: "docker-compose"}, nil
		}
	}
	return Runner{}, errors.New("docker compose runtime unavailable: neither 'docker compose' nor 'docker-compose' is available")
}

func (r Runner) CommandContext(ctx context.Context, args ...string) *exec.Cmd {
	fullArgs := make([]string, 0, len(r.PrefixArgs)+len(args))
	fullArgs = append(fullArgs, r.PrefixArgs...)
	fullArgs = append(fullArgs, args...)
	return commandContext(ctx, r.Binary, fullArgs...)
}

func probe(ctx context.Context, name string, args ...string) error {
	cmd := commandContext(ctx, name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s failed: %w (%s)", name, strings.Join(args, " "), err, truncateOutput(string(out), 200))
	}
	return nil
}

func truncateOutput(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max < 4 {
		return s[:max]
	}
	return s[:max-3] + "..."
}
