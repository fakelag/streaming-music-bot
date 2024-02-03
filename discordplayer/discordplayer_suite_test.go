package discordplayer_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestDiscordplayer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Discordplayer Suite")
}
