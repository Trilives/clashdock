package sysd

import (
	"strings"
	"testing"
)

func TestRemoveResilienceCommandsDoNotCleanLegacyHealthcheck(t *testing.T) {
	var got []string
	for _, cmd := range removeResilienceRootCommands("mihomo") {
		got = append(got, strings.Join(cmd, " "))
	}
	joined := strings.Join(got, "\n")
	if strings.Contains(joined, "healthcheck.sh") {
		t.Fatalf("legacy healthcheck cleanup should not be installed:\n%s", joined)
	}
	if !strings.Contains(joined, "90-mihomo-restart") {
		t.Fatalf("dispatcher cleanup missing from commands:\n%s", joined)
	}
	if !strings.Contains(joined, "mihomo-watchdog.timer") {
		t.Fatalf("watchdog timer cleanup missing from commands:\n%s", joined)
	}
}
