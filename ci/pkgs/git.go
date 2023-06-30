package pkgs

import (
	"os"
	"os/exec"
)

// GitClone clones a git repository into a directory
func GitClone(url) error {
	cmd := exec.Command("git", "clone", url, dir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
