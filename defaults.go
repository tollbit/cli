package tollbitcli

import _ "embed"

// DefaultConfig contains the CLI defaults compiled into the binary.
//
//go:embed tb-cli.config.yaml
var DefaultConfig []byte
