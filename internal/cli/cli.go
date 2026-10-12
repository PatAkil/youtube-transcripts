// Package cli holds the youtube-transcripts command line: one entry function
// per subcommand plus Run, which dispatches on the first argument.
package cli

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/PatAkil/youtube-transcripts/internal/youtube"
)

// DefaultCorpusDir is the Corpus root when --corpus is not given, relative to
// the working directory (the repo root when run from there).
const DefaultCorpusDir = "corpus"

// Deps are the upstreams and clock a command needs. Tests pass fakes.
type Deps struct {
	YouTube youtube.Client
	Now     func() time.Time
}

const usage = `youtube-transcripts builds and maintains the Corpus: Transcripts of other
creators' YouTube Videos, stored as plain files, one directory per Channel.

Usage:
  youtube-transcripts [--corpus DIR] <command> [arguments]

Commands:
  fetch     Store one Video's Transcript and metadata in the Corpus
  sync      Fetch new Videos from every Followed Channel
  overview  Write the Sponsorship overview from the Segments files (not available yet)

Global flags:
  --corpus DIR   Corpus root directory (default "corpus", relative to the working directory)
  --help, -h     Show this help

Run "youtube-transcripts <command> --help" for a command's own flags.
`

// Run parses the global flags, dispatches the subcommand and returns the
// process exit code. Output goes to stdout and stderr only.
func Run(args []string, stdout, stderr io.Writer, deps Deps) int {
	if deps.Now == nil {
		deps.Now = time.Now
	}
	corpusDir := DefaultCorpusDir

	i := 0
	for i < len(args) {
		a := args[i]
		switch {
		case a == "--help" || a == "-h" || a == "help":
			fmt.Fprint(stdout, usage)
			return 0
		case a == "--corpus" || a == "-corpus":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "--corpus needs a directory")
				return 1
			}
			corpusDir = args[i+1]
			i += 2
			continue
		case strings.HasPrefix(a, "--corpus=") || strings.HasPrefix(a, "-corpus="):
			corpusDir = a[strings.Index(a, "=")+1:]
			i++
			continue
		case strings.HasPrefix(a, "-"):
			fmt.Fprintf(stderr, "unknown flag %q\n\n%s", a, usage)
			return 1
		}
		break
	}
	if i >= len(args) {
		fmt.Fprint(stderr, usage)
		return 1
	}
	if corpusDir == "" {
		fmt.Fprintln(stderr, "--corpus needs a directory")
		return 1
	}

	ctx := context.Background()
	sub, rest := args[i], args[i+1:]
	switch sub {
	case "fetch":
		return Fetch(ctx, deps, corpusDir, rest, stdout, stderr)
	case "sync":
		return Sync(ctx, deps, corpusDir, rest, stdout, stderr)
	case "overview":
		fmt.Fprintf(stderr, "%s is not available yet\n", sub)
		return 1
	default:
		fmt.Fprintf(stderr, "unknown command %q\n\n%s", sub, usage)
		return 1
	}
}
