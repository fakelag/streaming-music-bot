package testutils

import (
	"errors"
	"fmt"
	"time"
)

type MockCommandExecutor struct {
	MockStdoutResult string
	MockExitCode     int
}

func (command *MockCommandExecutor) RunCommandWithTimeout(
	executable string,
	timeout time.Duration,
	args ...string,
) (chan *string, chan error) {
	resultChannel := make(chan *string, 1)
	errorChannel := make(chan error, 1)

	go func() {
		if command.MockExitCode != 0 {
			errorChannel <- errors.New(fmt.Sprintf("exit status %d", command.MockExitCode))
		} else {
			resultChannel <- &command.MockStdoutResult
		}
	}()

	return resultChannel, errorChannel
}
