package agentauth

import (
	"context"
	"io"
)

type Invocation interface {
	Context() context.Context
	OutOrStdout() io.Writer
	ErrOrStderr() io.Writer
}
