package jobs

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// RunJob detects runtime by ext, injects current time, and executes the job script
func RunJob(name, path string) {
	log.Printf("[JOB] Running job %s from %s", name, path)

	ext := filepath.Ext(path)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// build command based on file extension
	var cmd *exec.Cmd
	switch ext {
	case ".py":
		cmd = exec.CommandContext(ctx, "python3", path)
	case ".sh":
		cmd = exec.CommandContext(ctx, "bash", path)
	case ".go":
		cmd = exec.CommandContext(ctx, "go", "run", path)
	default:
		log.Printf("[JOB:%s] Unsupported file extension: %s", name, ext)
		return
	}

	// inject current timestamp as ISO8601 into environment
	now := time.Now().UTC().Format(time.RFC3339)
	cmd.Env = append(os.Environ(), fmt.Sprintf("JOB_NOW=%s", now))

	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("[JOB:%s] Error: %v", name, err)
	}
	log.Printf("[JOB:%s] Output:\n%s", name, strings.TrimSpace(string(out)))
}
