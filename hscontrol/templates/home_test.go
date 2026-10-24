package templates

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHomeIsBrandedAndExplainsJoining(t *testing.T) {
	html := Home("https://pneuma.example.com").Render()

	assert.Contains(t, html, "<title>ElRoi Pneuma</title>")
	assert.Contains(t, html, "tailscale login --login-server https://pneuma.example.com")
	// product mark in the header, company mark in the footer, each with a dark-mode variant
	for _, mark := range []string{"elroi", "qavats-it"} {
		assert.Contains(t, html, `src="/brand/`+mark+`-on-light.svg"`)
		assert.Contains(t, html, `srcset="/brand/`+mark+`-on-dark.svg"`)
	}
	// the upstream credit the BSD-3-Clause licences ask for
	assert.Contains(t, html, "https://github.com/juanfont/headscale")
	assert.Contains(t, html, "https://github.com/tailscale/tailscale")
	assert.NotContains(t, html, "Powered by")
}
