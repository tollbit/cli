package cliruntime

// End-user proximity auto-detection
//
// End-user proximity picks the login flow. "local" means the end user's
// browser can reach this machine's loopback address, so the CLI opens the
// browser and catches the redirect on 127.0.0.1. "remote" means it cannot, so
// the CLI prints a link for the end user to open elsewhere and the agent
// finishes with `tollbit auth complete`.
//
// When detection runs
//
// Detection is the last resort. The CLI resolves proximity in this order and
// stops at the first step that gives an answer:
//
//  1. Configured: runtime.end_user_proximity, TOLLBIT_RUNTIME_END_USER_PROXIMITY
//     or --end-user-proximity set to local or remote.
//  2. Saved: the value written by `tollbit runtime set --end-user-proximity`.
//  3. Detected: the rule below. Only reached when the configured value is
//     auto-detect and nothing is saved.
//
// The detection rule
//
// The rule answers one question: is there a browser on this machine, or one
// whose localhost is forwarded to this machine? Detection returns remote only
// when ALL four of these conditions hold:
//
//  1. The OS is not macOS (darwin) and not Windows. Both always run a
//     graphical desktop with a browser, so they are local.
//  2. DISPLAY is empty or unset. An X11 session sets it. `ssh -X` sets it
//     too, and that is still local: the browser then runs on this machine,
//     so its 127.0.0.1 is this machine.
//  3. WAYLAND_DISPLAY is empty or unset. A Wayland session sets it.
//  4. WSL_DISTRO_NAME and WSL_INTEROP are both empty or unset. WSL sets them
//     in every process started through wsl.exe. Under WSL the browser runs on
//     the Windows host, and WSL forwards the host's localhost into the Linux
//     VM by default, so the redirect reaches the CLI even with no display.
//
// If any condition fails, detection returns local.
//
// What that gives in practice
//
//	macOS or Windows laptop                                 local   (1)
//	Linux desktop session                                   local   (2 or 3)
//	WSL, with or without WSLg                               local   (4)
//	Claude Cowork or chat sandbox, Claude Code on the web   remote
//	Docker or other container, Linux CI runner              remote
//	SSH into a headless Linux server                        remote
//
// Known misses, left to `tollbit runtime set`
//
//   - SSH into a Mac or Windows machine detects local, but the browser is on
//     the other end of the connection.
//   - WSL with localhost forwarding turned off detects local, but the
//     redirect cannot reach the CLI.
//   - A headless Linux VM or container whose loopback port is forwarded to
//     the user's machine (such as a VS Code dev container) detects remote.
//     The remote flow still works there, with a few more steps.
//
// Signals deliberately not used
//
//   - SSH variables: a headless Linux server is already caught by rules 2
//     and 3. They only add SSH into a Mac or Windows machine, which is rare.
//   - Agent harness variables such as CLAUDECODE: Claude Code sets them on
//     laptops as well as in the cloud, so they say nothing about location.
//   - Whether stdin or stdout is a terminal: agents rarely have one, even on
//     a laptop, so it would make every local agent remote.
//
// Constraints
//
//   - Detection reads only environment variables and the OS name. It reads no
//     files, starts no processes and makes no network calls.
//   - The result is never saved. It is recomputed on every command, so it
//     follows the environment the command runs in, and a saved value always
//     means an agent or user chose it.
//   - `tollbit runtime status` reports the source as auto-detect without the
//     reason. The reason is logged at debug level (TOLLBIT_LOG_LEVEL=debug).

import (
	"context"

	"github.com/rs/zerolog"
)

type endUserProximityDetection struct {
	proximity EndUserProximity
	reason    string
}

// detectEndUserProximity applies the detection rule documented at the top of
// this file. goos is a runtime.GOOS value and getenv returns "" for unset
// variables, like os.Getenv.
func detectEndUserProximity(goos string, getenv func(string) string) endUserProximityDetection {
	switch {
	case goos == "darwin" || goos == "windows":
		return endUserProximityDetection{EndUserProximityLocal, "macOS and Windows have a desktop browser"}
	case getenv("DISPLAY") != "" || getenv("WAYLAND_DISPLAY") != "":
		return endUserProximityDetection{EndUserProximityLocal, "DISPLAY or WAYLAND_DISPLAY is set"}
	case getenv("WSL_DISTRO_NAME") != "" || getenv("WSL_INTEROP") != "":
		return endUserProximityDetection{EndUserProximityLocal, "WSL forwards the Windows host's localhost"}
	default:
		return endUserProximityDetection{EndUserProximityRemote, "no DISPLAY or WAYLAND_DISPLAY outside macOS, Windows and WSL"}
	}
}

func (r *Runtime) detectEndUserProximity(ctx context.Context) EndUserProximity {
	detected := detectEndUserProximity(r.goos, r.getenv)
	zerolog.Ctx(ctx).Debug().
		Str("end_user_proximity", string(detected.proximity)).
		Str("reason", detected.reason).
		Msg("auto-detected end-user proximity")
	return detected.proximity
}
