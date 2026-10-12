package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/PatAkil/youtube-transcripts/internal/sponsorblock"
	"github.com/PatAkil/youtube-transcripts/internal/youtube"
)

func runCLI(t *testing.T, fake *youtube.Fake, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code = Run(args, &out, &errOut, Deps{YouTube: fake, SponsorBlock: &sponsorblock.Fake{}, Now: func() time.Time { return fixedNow }})
	return code, out.String(), errOut.String()
}

func TestRunHelpListsSubcommandsAndCorpusFlag(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {"-h"}, {"help"}} {
		code, stdout, stderr := runCLI(t, &youtube.Fake{}, args...)
		if code != 0 {
			t.Errorf("%v: exit %d, stderr %q", args, code, stderr)
		}
		for _, want := range []string{"fetch", "sync", "overview", "--corpus", "corpus"} {
			if !strings.Contains(stdout, want) {
				t.Errorf("%v: help output lacks %q:\n%s", args, want, stdout)
			}
		}
	}
}

func TestRunWithoutArgumentsPrintsUsageAndFails(t *testing.T) {
	code, _, stderr := runCLI(t, &youtube.Fake{})
	if code != 1 || !strings.Contains(stderr, "fetch") {
		t.Errorf("exit %d, stderr %q; want 1 and usage", code, stderr)
	}
}

func TestRunUnknownSubcommandFails(t *testing.T) {
	code, _, stderr := runCLI(t, &youtube.Fake{}, "frobnicate")
	if code != 1 || !strings.Contains(stderr, "frobnicate") {
		t.Errorf("exit %d, stderr %q; want 1 naming the subcommand", code, stderr)
	}
}

func TestRunFetchHelp(t *testing.T) {
	code, stdout, _ := runCLI(t, &youtube.Fake{}, "fetch", "--help")
	if code != 0 {
		t.Errorf("exit %d, want 0", code)
	}
	for _, want := range []string{"fetch", "--force", "video ID", "URL", "Transcript"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("fetch help lacks %q:\n%s", want, stdout)
		}
	}
}

func TestRunCorpusFlagSetsCorpusRoot(t *testing.T) {
	corpusDir := filepath.Join(t.TempDir(), "scratch-corpus")
	fake := &youtube.Fake{Transcripts: map[string]youtube.Transcript{"dQw4w9WgXcQ": manualTranscript()}}

	for _, args := range [][]string{
		{"--corpus", corpusDir, "fetch", "dQw4w9WgXcQ"},
		{"--corpus=" + corpusDir, "fetch", "dQw4w9WgXcQ"},
	} {
		if err := os.RemoveAll(corpusDir); err != nil {
			t.Fatal(err)
		}
		code, stdout, stderr := runCLI(t, fake, args...)
		if code != 0 || stderr != "" {
			t.Fatalf("%v: exit %d, stderr %q", args, code, stderr)
		}
		if !strings.HasPrefix(stdout, "dQw4w9WgXcQ  Example Channel") {
			t.Errorf("%v: stdout = %q", args, stdout)
		}
		if _, err := os.Stat(filepath.Join(corpusDir, "UCabc123", "dQw4w9WgXcQ", "transcript.txt")); err != nil {
			t.Errorf("%v: transcript not written under --corpus directory: %v", args, err)
		}
	}
}

func TestRunDefaultsCorpusToCorpusDirectory(t *testing.T) {
	// Run in a temp working directory so the default resolves there.
	wd := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(wd); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	fake := &youtube.Fake{Transcripts: map[string]youtube.Transcript{"dQw4w9WgXcQ": manualTranscript()}}

	if code, _, stderr := runCLI(t, fake, "fetch", "dQw4w9WgXcQ"); code != 0 {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	if _, err := os.Stat(filepath.Join(wd, "corpus", "UCabc123", "dQw4w9WgXcQ", "video.json")); err != nil {
		t.Errorf("video.json not under ./corpus: %v", err)
	}
}

func TestRunOverviewWritesOverviewUnderCorpusFlag(t *testing.T) {
	corpusDir := t.TempDir()
	dir := writeVideoFixture(t, corpusDir, video("UCabc123", "Example Channel", "dQw4w9WgXcQ", "An example video", "2024-01-02T15:04:05Z"))
	writeSegmentsFixture(t, dir, processed(segment("Acme", 12.5, 60)))

	code, stdout, stderr := runCLI(t, &youtube.Fake{}, "--corpus", corpusDir, "overview")

	if code != 0 || stderr != "" {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	if !strings.HasPrefix(stdout, "wrote "+filepath.Join(corpusDir, "overview.md")) {
		t.Errorf("stdout = %q", stdout)
	}
	if got := readFile(t, filepath.Join(corpusDir, "overview.md")); !strings.Contains(got, "### Acme\n") {
		t.Errorf("overview.md lacks the Sponsor:\n%s", got)
	}
}

func TestRunOverviewHelp(t *testing.T) {
	code, stdout, _ := runCLI(t, &youtube.Fake{}, "overview", "--help")
	if code != 0 || !strings.Contains(stdout, "overview.md") {
		t.Errorf("exit %d, stdout %q; want 0 and the overview help", code, stdout)
	}
}

func TestRunHelpNoLongerMarksOverviewUnavailable(t *testing.T) {
	_, stdout, _ := runCLI(t, &youtube.Fake{}, "--help")
	for _, line := range strings.Split(stdout, "\n") {
		if strings.Contains(line, "overview") && strings.Contains(line, "not available") {
			t.Errorf("help still marks overview unavailable: %q", line)
		}
	}
}

func TestRunSyncHelp(t *testing.T) {
	code, stdout, _ := runCLI(t, &youtube.Fake{}, "sync", "--help")
	if code != 0 {
		t.Errorf("exit %d, want 0", code)
	}
	for _, want := range []string{"sync", "Followed Channel", "followed-channels.json"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("sync help lacks %q:\n%s", want, stdout)
		}
	}
}

func TestRunSyncUsesCorpusFlag(t *testing.T) {
	corpusDir := t.TempDir()
	writeFile(t, filepath.Join(corpusDir, "followed-channels.json"), `[{"channel_id": "UCabc123", "name": "Example Channel"}]`)
	fake := &youtube.Fake{
		Feeds:       map[string][]youtube.VideoMeta{"UCabc123": {manualTranscript().Meta}},
		Transcripts: map[string]youtube.Transcript{"dQw4w9WgXcQ": manualTranscript()},
	}

	code, stdout, stderr := runCLI(t, fake, "--corpus", corpusDir, "sync")

	if code != 0 || stderr != "" {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	if !strings.Contains(stdout, "dQw4w9WgXcQ  Example Channel") || !strings.Contains(stdout, "channels walked: 1") {
		t.Errorf("stdout = %q", stdout)
	}
	if _, err := os.Stat(filepath.Join(corpusDir, "UCabc123", "dQw4w9WgXcQ", "transcript.txt")); err != nil {
		t.Errorf("transcript not written under --corpus directory: %v", err)
	}
}
