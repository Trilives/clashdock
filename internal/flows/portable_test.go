package flows

import (
	"strings"
	"testing"

	"github.com/Trilives/clashdock/internal/i18n"
	"github.com/Trilives/clashdock/internal/paths"
)

func TestPortableHowToUseTextShowsProxyExportsAndUsage(t *testing.T) {
	i18n.SetLang(i18n.EN)
	text := portableHowToUseText(paths.Paths{State: "/tmp/clashdock-data"})

	for _, want := range []string{
		`export http_proxy="http://127.0.0.1:7890"`,
		`export all_proxy="socks5://127.0.0.1:7890"`,
		`./clashdock run`,
		`curl -x http://127.0.0.1:7890 https://www.google.com/generate_204`,
		`/tmp/clashdock-data`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("portableHowToUseText() missing %q in:\n%s", want, text)
		}
	}
}
