// Package assets provides embedded static assets for Headscale.
// All static files (favicon, CSS, SVG) are embedded here for
// centralized asset management.
package assets

import (
	"embed"
)

// Favicon is the embedded favicon.png file served at /favicon.ico
//
//go:embed favicon.png
var Favicon []byte

// CSS is the embedded style.css stylesheet used in HTML templates.
// Contains Material for MkDocs design system styles.
//
//go:embed style.css
var CSS string

// SVG is the embedded headscale.svg logo used in HTML templates.
//
//go:embed headscale.svg
var SVG string

// Brand holds the ElRoi Pneuma product marks and the Qavats IT company marks,
// each in a variant for light and for dark backgrounds, served under /brand/.
//
//go:embed brand/*.svg
var Brand embed.FS
