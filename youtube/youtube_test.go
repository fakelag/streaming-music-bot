package youtube

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Downloading from YT", func() {
	When("Downloading a video streaming URL", func() {
		url := YTGetStreamUrl("foo")
		Expect(url).To(Equal("url123"))
	})
})
