package cmd_test

import (
	"runtime"
	"strings"
	"time"

	cmd "github.com/fakelag/streaming-music-bot/command"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Command Executor", func() {
	It("Runs an executable", func() {
		executor := &cmd.DefaultCommandExecutor{}

		cmd := "echo"
		args := []string{"foo"}

		if runtime.GOOS == "windows" {
			cmd = "cmd"
			args = []string{"/C", "echo foo"}
		}

		resChan, errChan := executor.RunCommandWithTimeout(cmd, 2*time.Second, args...)

		select {
		case res := <-resChan:
			Expect(res).NotTo(BeNil())
			Expect(strings.TrimSpace(*res)).To(Equal("foo"))
		case err := <-errChan:
			Expect(err).To(BeNil())
		}
	})

	It("Timeouts execution after timeout duration", func() {
		executor := &cmd.DefaultCommandExecutor{}

		cmd := "sleep"
		args := []string{"20s"}

		if runtime.GOOS == "windows" {
			cmd = "cmd"
			args = []string{"/C", "ping 127.0.0.1 -t"}
		}

		resChan, errChan := executor.RunCommandWithTimeout(cmd, 2*time.Second, args...)

		select {
		case <-resChan:
			// Should not be reached
			Expect(false).To(BeTrue())
		case err := <-errChan:
			Expect(err.Error()).To(ContainSubstring("operation timed out after"))
		}
	})
})
