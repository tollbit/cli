//go:build !windows

package doctor

// systemProxy returns "" off Windows: there the CLI's proxy environment
// variables are the usual way command-line tools get a proxy.
func systemProxy() string {
	return ""
}
