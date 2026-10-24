package templates

import (
	"github.com/chasefleming/elem-go"
)

// Home renders the landing page at /: what this service is and how to put a
// device on it. serverURL is the control server's public URL.
func Home(serverURL string) *elem.Element {
	return page(
		ProductName,
		H1(elem.Text("The Qavats private network")),
		P(elem.Text("Sign in with your ElRoi account and passkey "+
			"to put a device on it; access is granted by a Qavats administrator.")),
		H2(elem.Text("Join a device")),
		Ol(
			elem.Li(nil,
				elem.Text("Install the Tailscale client (the app on macOS, iOS, Windows or Android, "+
					"or the tailscale package on Linux)."),
			),
			elem.Li(nil,
				elem.Text("Point it at this server. From a terminal:"),
				codeBlockText("tailscale login --login-server "+serverURL),
				elem.Text("Or follow the "),
				A("/apple", elem.Text("Apple")),
				elem.Text(" or "),
				A("/windows", elem.Text("Windows")),
				elem.Text(" instructions."),
			),
			elem.Li(nil,
				elem.Text("Open the link it gives you within 15 minutes and sign in with your passkey."),
			),
		),
		P(elem.Text("Devices sign in again every 30 days. If your access is withdrawn, your devices "+
			"leave the network within minutes.")),
	)
}
