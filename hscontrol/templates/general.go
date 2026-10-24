package templates

import (
	"github.com/chasefleming/elem-go"
	"github.com/chasefleming/elem-go/attrs"
	"github.com/chasefleming/elem-go/styles"
	"github.com/juanfont/headscale/hscontrol/assets"
)

// mdTypesetBody creates a body element with md-typeset styling
// that matches the official Headscale documentation design.
// Uses CSS classes with styles defined in [assets.CSS].
func mdTypesetBody(children ...elem.Node) *elem.Element {
	return elem.Body(
		attrs.Props{
			attrs.Style: styles.Props{
				styles.MinHeight:       "100vh",
				styles.Display:         "flex",
				styles.FlexDirection:   "column",
				styles.AlignItems:      "center",
				styles.BackgroundColor: "var(--hs-bg)",
				styles.Padding:         "3rem 1.5rem",
			}.ToInline(),
			"translate": "no",
		},
		elem.Main(attrs.Props{
			attrs.Class: "md-typeset",
			attrs.Style: styles.Props{
				styles.MaxWidth: "min(800px, 90vw)",
				styles.Width:    "100%",
			}.ToInline(),
		}, children...),
	)
}

// Styled Element Wrappers
// These functions wrap elem-go elements using CSS classes.
// Styling is handled by the CSS in [assets.CSS].

// H1 creates a H1 element styled by .md-typeset h1
func H1(children ...elem.Node) *elem.Element {
	return elem.H1(nil, children...)
}

// H2 creates a H2 element styled by .md-typeset h2
func H2(children ...elem.Node) *elem.Element {
	return elem.H2(nil, children...)
}

// H3 creates a H3 element styled by .md-typeset h3
func H3(children ...elem.Node) *elem.Element {
	return elem.H3(nil, children...)
}

// P creates a paragraph element styled by .md-typeset p
func P(children ...elem.Node) *elem.Element {
	return elem.P(nil, children...)
}

// Ol creates an ordered list element styled by .md-typeset ol
func Ol(children ...elem.Node) *elem.Element {
	return elem.Ol(nil, children...)
}

// Ul creates an unordered list element styled by .md-typeset ul
func Ul(children ...elem.Node) *elem.Element {
	return elem.Ul(nil, children...)
}

// A creates a link element styled by .md-typeset a
func A(href string, children ...elem.Node) *elem.Element {
	return elem.A(attrs.Props{attrs.Href: href}, children...)
}

// Code creates an inline code element styled by .md-typeset code
func Code(children ...elem.Node) *elem.Element {
	return elem.Code(nil, children...)
}

// codeBlockText creates a preformatted code block styled by
// .md-typeset pre > code.
func codeBlockText(code string) *elem.Element {
	return elem.Pre(nil, elem.Code(nil, elem.Text(code)))
}

// ProductName is the name people see for this service. ElRoi is the product
// brand and Qavats the company; the marks are in [assets.Brand].
const ProductName = "ElRoi Pneuma"

// brandMark renders a mark that follows the reader's colour scheme: the
// "-on-dark" variant in dark mode, the "-on-light" variant otherwise.
func brandMark(name, alt, class string) elem.Node {
	return elem.Picture(nil,
		elem.Source(attrs.Props{
			"media":  "(prefers-color-scheme: dark)",
			"srcset": "/brand/" + name + "-on-dark.svg",
		}),
		elem.Img(attrs.Props{
			attrs.Src:   "/brand/" + name + "-on-light.svg",
			attrs.Alt:   alt,
			attrs.Class: class,
		}),
	)
}

// headscaleLogo returns the product header shown at the top of every page:
// the ElRoi mark with the product name beside it, linking to the home page.
// It keeps the .headscale-logo class so upstream styling hooks still apply.
func headscaleLogo() elem.Node {
	return elem.A(attrs.Props{attrs.Href: "/", attrs.Class: "headscale-logo brand-header", "aria-label": ProductName},
		brandMark("elroi", "ElRoi", "brand-product"),
		elem.Span(attrs.Props{attrs.Class: "brand-name"}, elem.Text("Pneuma")),
	)
}

// pageFooter creates a consistent footer for all pages: the company mark,
// and the credit to the open-source projects this is built on, which their
// BSD-3-Clause licences ask us to keep.
func pageFooter() *elem.Element {
	return elem.Footer(
		attrs.Props{
			attrs.Class: "brand-footer",
			attrs.Style: styles.Props{
				styles.MarginTop:  space3XL,
				styles.TextAlign:  "center",
				styles.FontSize:   fontSizeSmall,
				styles.Color:      "var(--md-default-fg-color--light)",
				styles.LineHeight: lineHeightBase,
			}.ToInline(),
		},
		brandMark("qavats-it", "Qavats IT", "brand-company"),
		elem.P(nil,
			elem.Text(ProductName+" is operated by Qavats. Built on "),
			elem.A(attrs.Props{
				attrs.Href:   "https://github.com/juanfont/headscale",
				attrs.Rel:    "noreferrer noopener",
				attrs.Target: "_blank",
			}, elem.Text("Headscale")),
			elem.Text(" and "),
			elem.A(attrs.Props{
				attrs.Href:   "https://github.com/tailscale/tailscale",
				attrs.Rel:    "noreferrer noopener",
				attrs.Target: "_blank",
			}, elem.Text("Tailscale")),
			elem.Text(", used under their BSD-3-Clause licences."),
		),
	)
}

// page renders a standard page: the given title in the document head, and a
// body that begins with the product header, contains the supplied
// content nodes in order, and ends with the shared footer.
func page(title string, content ...elem.Node) *elem.Element {
	body := make([]elem.Node, 0, len(content)+2)
	body = append(body, headscaleLogo())
	body = append(body, content...)
	body = append(body, pageFooter())

	return HtmlStructure(
		elem.Title(nil, elem.Text(title)),
		mdTypesetBody(body...),
	)
}

// HtmlStructure creates a complete HTML document structure with proper meta tags
// and semantic HTML5 structure. The head and body elements are passed as parameters
// to allow for customization of each page.
// Styling is provided via a CSS stylesheet (Material for MkDocs design system) with
// minimal inline styles for layout and positioning.
func HtmlStructure(head, body *elem.Element) *elem.Element {
	return elem.Html(
		attrs.Props{attrs.Lang: "en"},
		elem.Head(
			nil,
			elem.Meta(attrs.Props{
				attrs.Charset: "UTF-8",
			}),
			elem.Meta(attrs.Props{
				attrs.HTTPequiv: "X-UA-Compatible",
				attrs.Content:   "IE=edge",
			}),
			elem.Meta(attrs.Props{
				attrs.Name:    "viewport",
				attrs.Content: "width=device-width, initial-scale=1.0",
			}),
			elem.Link(attrs.Props{
				attrs.Rel:  "icon",
				attrs.Href: "/favicon.ico",
			}),
			// Google Fonts for Roboto and Roboto Mono
			elem.Link(attrs.Props{
				attrs.Rel:     "preconnect",
				attrs.Href:    "https://fonts.gstatic.com",
				"crossorigin": "",
			}),
			elem.Link(attrs.Props{
				attrs.Rel:  "stylesheet",
				attrs.Href: "https://fonts.googleapis.com/css2?family=Roboto:wght@300;400;500;700&family=Roboto+Mono:wght@400;700&display=swap",
			}),
			// Material for MkDocs CSS styles
			elem.Style(attrs.Props{attrs.Type: "text/css"}, elem.Raw(assets.CSS)),
			head,
		),
		body,
	)
}

// BlankPage creates a minimal blank HTML page with favicon.
// Used for endpoints that need to return a valid HTML page with no content.
func BlankPage() *elem.Element {
	return elem.Html(
		attrs.Props{attrs.Lang: "en"},
		elem.Head(
			nil,
			elem.Meta(attrs.Props{
				attrs.Charset: "UTF-8",
			}),
			elem.Link(attrs.Props{
				attrs.Rel:  "icon",
				attrs.Href: "/favicon.ico",
			}),
		),
		elem.Body(nil),
	)
}
