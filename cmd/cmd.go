package cmd

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type CommandExecutor interface {
	RunCommandWithTimeout(executable string, timeout time.Duration, args ...string) (chan *string, chan error)
}

type DefaultCommandExecutor struct{}

func (command *DefaultCommandExecutor) RunCommandWithTimeout(
	executable string,
	timeout time.Duration,
	args ...string,
) (chan *string, chan error) {
	cmd := exec.Command(executable, args...)

	resultChannel := make(chan *string, 1)
	errorChannel := make(chan error, 1)

	go func() {
		time.Sleep(timeout)

		errorChannel <- errors.New(fmt.Sprintf("operation timed out after %d seconds", int(timeout.Seconds())))

		if cmd.Process != nil {
			cmd.Process.Kill()
		}
	}()

	go func() {
		stdout, err := cmd.Output()

		if err != nil && !strings.Contains(err.Error(), "exit status 101") {
			errorChannel <- err
		} else {
			stdoutString := string(stdout)
			resultChannel <- &stdoutString
		}

		close(resultChannel)
	}()

	return resultChannel, errorChannel
}
