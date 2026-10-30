package cliruntime

import "testing"

func TestDetectEndUserProximity(t *testing.T) {
	tests := []struct {
		name string
		goos string
		env  map[string]string
		want EndUserProximity
	}{
		{name: "macOS", goos: "darwin", want: EndUserProximityLocal},
		{name: "Windows", goos: "windows", want: EndUserProximityLocal},
		{name: "Linux with X11 display", goos: "linux", env: map[string]string{"DISPLAY": ":0"}, want: EndUserProximityLocal},
		{name: "Linux with Wayland display", goos: "linux", env: map[string]string{"WAYLAND_DISPLAY": "wayland-0"}, want: EndUserProximityLocal},
		{name: "WSL without WSLg", goos: "linux", env: map[string]string{"WSL_DISTRO_NAME": "Ubuntu"}, want: EndUserProximityLocal},
		{name: "WSL interop only", goos: "linux", env: map[string]string{"WSL_INTEROP": "/run/WSL/1_interop"}, want: EndUserProximityLocal},
		{name: "WSL with WSLg", goos: "linux", env: map[string]string{"WSL_DISTRO_NAME": "Ubuntu", "DISPLAY": ":0", "WAYLAND_DISPLAY": "wayland-0"}, want: EndUserProximityLocal},
		{name: "headless Linux", goos: "linux", want: EndUserProximityRemote},
		{name: "headless Linux over SSH", goos: "linux", env: map[string]string{"SSH_CONNECTION": "10.0.0.1 52000 10.0.0.2 22"}, want: EndUserProximityRemote},
		{name: "headless FreeBSD", goos: "freebsd", want: EndUserProximityRemote},
		{name: "macOS without display variables", goos: "darwin", env: map[string]string{"SSH_CONNECTION": "10.0.0.1 52000 10.0.0.2 22"}, want: EndUserProximityLocal},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectEndUserProximity(tt.goos, fakeGetenv(tt.env))
			if got.proximity != tt.want {
				t.Fatalf("expected %q, got %q (%s)", tt.want, got.proximity, got.reason)
			}
		})
	}
}

func fakeGetenv(env map[string]string) func(string) string {
	return func(key string) string { return env[key] }
}

func stubDetectionEnvironment(rt *Runtime, goos string, env map[string]string) {
	rt.goos = goos
	rt.getenv = fakeGetenv(env)
}
