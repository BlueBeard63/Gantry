package main

import _ "embed"

// docsLogoSVG is the crane logo used in the header (accent-coloured via
// currentColor). The source lives in docsassets/logo.svg.
//
//go:embed docsassets/logo.svg
var docsLogoSVG string

// docsShellHTML is the html/template for one rendered page: the fixed
// header, the manifest-driven sidebar, the rendered content with a
// breadcrumb, the "On this page" TOC and the Ctrl-K search palette. All
// CSS/JS/SVG is inline so the viewer works fully offline. The source lives
// in docsassets/shell.html so it can be edited as a real HTML file.
//
//go:embed docsassets/shell.html
var docsShellHTML string
