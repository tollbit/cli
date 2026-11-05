//go:build windows

package doctor

import (
	"strings"

	"golang.org/x/sys/windows/registry"
)

// systemProxy returns the current user's Windows proxy setting, or "" when
// none is on. Go reads only the proxy environment variables, so a network
// that relies on this setting blocks the CLI unless HTTPS_PROXY is also set.
func systemProxy() string {
	key, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Internet Settings`, registry.QUERY_VALUE)
	if err != nil {
		return ""
	}
	defer key.Close()
	var parts []string
	if enabled, _, err := key.GetIntegerValue("ProxyEnable"); err == nil && enabled != 0 {
		if server, _, err := key.GetStringValue("ProxyServer"); err == nil && strings.TrimSpace(server) != "" {
			parts = append(parts, "proxy "+redactProxy(strings.TrimSpace(server)))
		}
	}
	if pac, _, err := key.GetStringValue("AutoConfigURL"); err == nil && strings.TrimSpace(pac) != "" {
		parts = append(parts, "PAC script "+strings.TrimSpace(pac))
	}
	return strings.Join(parts, ", ")
}
