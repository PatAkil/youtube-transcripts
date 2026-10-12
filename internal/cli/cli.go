// Package cli holds the youtube-transcripts command line: one entry function
// per subcommand plus Run, which dispatches on the first argument.
package cli

import (
	"time"

	"github.com/PatAkil/youtube-transcripts/internal/youtube"
)

// Deps are the upstreams and clock a command needs. Tests pass fakes.
type Deps struct {
	YouTube youtube.Client
	Now     func() time.Time
}
