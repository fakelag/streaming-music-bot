package youtube_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestYoutube(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Youtube Suite")
}
