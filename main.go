// Command youtube-transcripts builds and maintains the Corpus: Transcripts of
// other creators' YouTube Videos stored as plain files. It is deterministic,
// key-free and makes no LLM calls (ADR-0001).
package main

import (
	"net/http"
	"os"
	"time"

	"github.com/PatAkil/youtube-transcripts/internal/cli"
	"github.com/PatAkil/youtube-transcripts/internal/youtube"
)

func main() {
	deps := cli.Deps{
		YouTube: youtube.NewClient(&http.Client{Timeout: 60 * time.Second}),
		Now:     time.Now,
	}
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr, deps))
}
