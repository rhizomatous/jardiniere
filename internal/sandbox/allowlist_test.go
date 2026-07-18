package sandbox

import (
	"strings"
	"testing"
)

func TestFilterFile(t *testing.T) {
	got := filterFile([]string{"github.com", "api.anthropic.com"})

	// one extended-regex per host, dots escaped, subdomain-permitting form.
	want := `(^|\.)github\.com$
(^|\.)api\.anthropic\.com$
`
	if got != want {
		t.Errorf("filterFile mismatch:\n got: %q\nwant: %q", got, want)
	}
}

func TestFilterFileEscapesMetacharacters(t *testing.T) {
	// a host with a regex metachar must be escaped so it can't widen the match.
	got := filterFile([]string{"a+b.com"})
	if !strings.Contains(got, `a\+b\.com`) {
		t.Errorf("metacharacters should be escaped, got %q", got)
	}
}

func TestTinyproxyConfDefaultDeny(t *testing.T) {
	conf := tinyproxyConf()
	for _, want := range []string{"FilterDefaultDeny Yes", "FilterType ere", "Port " + proxyPort} {
		if !strings.Contains(conf, want) {
			t.Errorf("tinyproxy conf missing %q:\n%s", want, conf)
		}
	}
}

func TestProxyEnvArgs(t *testing.T) {
	p := &proxySidecar{proxyName: "jard-proxy-42"}
	joined := strings.Join(p.envArgs(), " ")
	for _, want := range []string{"HTTP_PROXY=http://jard-proxy-42:8888", "NO_PROXY=localhost,127.0.0.1"} {
		if !strings.Contains(joined, want) {
			t.Errorf("envArgs missing %q, got %v", want, joined)
		}
	}
}
