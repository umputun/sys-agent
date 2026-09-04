package external

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"
)

// ProgramProvider is an external service that runs a command and checks the exit code.
type ProgramProvider struct {
	TimeOut time.Duration
}

// Status returns the status of the execution of the command from the request.
// url looks like this: program://cat?args=/tmp/foo
func (p *ProgramProvider) Status(req Request) (*Response, error) {
	st := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), p.TimeOut)
	defer cancel()

	resp := Response{
		Name:       req.Name,
		StatusCode: 200,
	}

	target, query, err := parseTarget(req.URL, "program")
	if err != nil {
		return nil, fmt.Errorf("program URL parse failed: %s %s: %w", req.Name, req.URL, err)
	}
	args := query["args"]

	log.Printf("[DEBUG] command: %s %v", target, args)

	cmd := exec.CommandContext(ctx, target, args...) //nolint:gosec // we trust the command as it comes from the config
	stdOut, stdErr := bytes.NewBuffer(nil), bytes.NewBuffer(nil)
	cmd.Stdout = stdOut
	cmd.Stderr = stdErr
	cmd.Stdin = os.Stdin

	err = cmd.Run()
	resp.ResponseTime = time.Since(st).Milliseconds()

	res := map[string]any{
		"command": strings.Join(cmd.Args, " "),
		"stdout":  stdOut.String(),
		"stderr":  stdErr.String(),
		"status":  "ok",
	}

	if err != nil {
		res["status"] = err.Error()
		resp.StatusCode = 500
	}

	resp.Body = res
	return &resp, nil
}
