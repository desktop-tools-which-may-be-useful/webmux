package sessioncmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/rainchan/webmux/internal/contract"
)

func TestPrintStatusInstalledVersion(t *testing.T) {
	const notice = "已安装 0.5.0，重启 WebMux 后生效，重启会结束所有会话"
	s := contract.StatusV1{V: contract.V, State: contract.StateRunning, Version: "0.4.0", Health: contract.Health{Status: contract.HealthOK}}
	var out bytes.Buffer
	PrintStatus(&out, s)
	if strings.Contains(out.String(), "已安装") {
		t.Fatalf("notice without an installed version:\n%s", out.String())
	}
	s.InstalledVersion = "0.5.0"
	out.Reset()
	PrintStatus(&out, s)
	if !strings.Contains(out.String(), notice+"\n") {
		t.Fatalf("missing %q:\n%s", notice, out.String())
	}
}
