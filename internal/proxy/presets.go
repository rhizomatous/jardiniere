package proxy

// New returns the starting policy for a preset.
func New(preset Preset) Policy {
	switch preset {
	case PresetBalanced:
		return Policy{Preset: PresetBalanced, Rules: allowAll(balanced)}
	case PresetOpen:
		return Policy{Preset: PresetOpen}
	default:
		return Policy{Preset: preset}
	}
}

// allowAll turns a list of patterns into allow rules.
func allowAll(patterns []string) []Rule {
	rules := make([]Rule, 0, len(patterns))
	for _, p := range patterns {
		rules = append(rules, Rule{Pattern: p, Allow: true})
	}
	return rules
}

// balanced is what the balanced preset allows: the places an agent has to
// reach to do ordinary work, and nothing else.
//
// Both the apex and the wildcard are listed wherever a service uses both,
// because "*.example.com" deliberately does not cover "example.com".
//
// Ports are left off throughout. Pinning :443 would be tighter, but every one
// of these is also fetched over :80 somewhere — apt and alpine repositories
// plainly, redirects to https elsewhere — and a policy that breaks `apt update`
// is one people turn off rather than narrow.
var balanced = []string{
	// model providers. locked-down is the preset that refuses these.
	"api.anthropic.com",
	"api.openai.com",
	"generativelanguage.googleapis.com",

	// source hosting, and the separate hosts git and release downloads use.
	"github.com", "*.github.com",
	"codeload.github.com",
	"raw.githubusercontent.com",
	"objects.githubusercontent.com",
	"gitlab.com", "*.gitlab.com",
	"bitbucket.org", "*.bitbucket.org",

	// javascript
	"registry.npmjs.org", "*.npmjs.org",
	"registry.yarnpkg.com",
	"nodejs.org",

	// python
	"pypi.org", "*.pypi.org",
	"files.pythonhosted.org",

	// go
	"proxy.golang.org",
	"sum.golang.org",
	"storage.googleapis.com",

	// rust
	"crates.io", "*.crates.io",
	"static.crates.io",
	"sh.rustup.rs",

	// ruby, php, java
	"rubygems.org", "*.rubygems.org",
	"packagist.org", "*.packagist.org",
	"repo.maven.apache.org",
	"repo1.maven.org",

	// linux distributions, for `apt install` and `apk add`
	"deb.debian.org",
	"security.debian.org",
	"archive.ubuntu.com",
	"security.ubuntu.com",
	"ports.ubuntu.com",
	"dl-cdn.alpinelinux.org",

	// editors attaching to a sandbox, and the CDNs they pull from
	"update.code.visualstudio.com",
	"marketplace.visualstudio.com",
	"*.vscode-cdn.net",
	"*.blob.core.windows.net",
}
