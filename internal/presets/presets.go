// Package presets embeds all bundled mock configuration presets so they are
// available from the mockly binary without needing files on disk.
package presets

import (
	"embed"
	"fmt"
	"path"
	"strings"
)

// FS is the embedded file system containing all preset YAML files.
//
//go:embed *.yaml
var FS embed.FS

// Preset describes a single bundled preset.
type Preset struct {
	// Name is the identifier used on the CLI (e.g. "keycloak").
	Name string
	// Description is a one-line summary.
	Description string
	// Filename is the embedded YAML filename (e.g. "keycloak.yaml").
	Filename string
}

// All returns the full catalogue of available presets.
var All = []Preset{
	{Name: "keycloak", Description: "Keycloak OIDC/OAuth2 + Admin API mock", Filename: "keycloak.yaml"},
	{Name: "authelia", Description: "Authelia authentication & forward-auth API mock", Filename: "authelia.yaml"},
	{Name: "oauth2", Description: "Generic OAuth2 / OpenID Connect authorization server mock", Filename: "oauth2.yaml"},
	{Name: "github", Description: "GitHub REST API v3 mock (repos, issues, PRs, actions)", Filename: "github.yaml"},
	{Name: "stripe", Description: "Stripe API mock (customers, payment intents, subscriptions)", Filename: "stripe.yaml"},
	{Name: "openai", Description: "OpenAI API mock (chat, embeddings, images, audio, moderation)", Filename: "openai.yaml"},
	{Name: "slack", Description: "Slack Web API mock (messages, channels, users, webhooks)", Filename: "slack.yaml"},
	{Name: "twilio", Description: "Twilio API mock (SMS, calls, Verify OTP)", Filename: "twilio.yaml"},
	{Name: "sendgrid", Description: "SendGrid v3 API mock (mail send, templates, suppressions)", Filename: "sendgrid.yaml"},
}

// Find returns the Preset with the given name (case-insensitive), or an error.
func Find(name string) (Preset, error) {
	name = strings.ToLower(strings.TrimSpace(name))
	for _, p := range All {
		if p.Name == name {
			return p, nil
		}
	}
	return Preset{}, fmt.Errorf("unknown preset %q — run 'mockly preset list' to see available presets", name)
}

// Read returns the raw YAML bytes for the given preset name.
func Read(name string) ([]byte, error) {
	p, err := Find(name)
	if err != nil {
		return nil, err
	}
	return FS.ReadFile(path.Base(p.Filename))
}
