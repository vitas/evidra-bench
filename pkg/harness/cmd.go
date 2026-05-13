package harness

import "os/exec"

func makeCmd(args []string) *exec.Cmd {
	return exec.Command(args[0], args[1:]...)
}
