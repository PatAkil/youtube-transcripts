// Package corpus knows the on-disk layout of the Corpus: one directory per
// Channel, one directory per Video beneath it, and the files inside.
package corpus

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// File names inside a Video directory.
const (
	VideoFile      = "video.json"
	TranscriptFile = "transcript.txt"
	SegmentsFile   = "segments.json"
)

// Transcript status values stored in video.json.
const (
	TranscriptOK   = "ok"
	TranscriptNone = "none"
)

// Video is the metadata file written next to a Transcript.
type Video struct {
	VideoID     string    `json:"video_id"`
	ChannelID   string    `json:"channel_id"`
	ChannelName string    `json:"channel_name"`
	Title       string    `json:"title"`
	Published   time.Time `json:"published"`
	Fetched     time.Time `json:"fetched"`
	Transcript  string    `json:"transcript"`
	Track       string    `json:"track"`
}

// VideoDir is the directory holding one Video's files.
func VideoDir(corpusDir, channelID, videoID string) string {
	return filepath.Join(corpusDir, channelID, videoID)
}

// MarshalJSON renders v as indented JSON with a trailing newline, so Corpus
// files diff well.
func MarshalJSON(v any) ([]byte, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

// WriteVideoDir atomically replaces the Video directory for channelID/videoID
// with one holding exactly files. The directory is built in a temporary
// sibling and renamed into place, so a failure part-way leaves nothing behind.
//
// previous is the Video's current directory when it is already in the Corpus
// (it may sit under another channel directory), or "". Files named in keep are
// copied from previous into the new directory before the swap, and previous is
// removed only once the new directory is complete.
func WriteVideoDir(corpusDir, channelID, videoID string, files map[string][]byte, previous string, keep []string) error {
	channelDir := filepath.Join(corpusDir, channelID)
	if err := os.MkdirAll(channelDir, 0o755); err != nil {
		return fmt.Errorf("create channel directory: %w", err)
	}
	final := filepath.Join(channelDir, videoID)

	tmp, err := os.MkdirTemp(channelDir, "."+videoID+".tmp-")
	if err != nil {
		return fmt.Errorf("create temporary video directory: %w", err)
	}
	cleanupTmp := func() { _ = os.RemoveAll(tmp) }

	for name, data := range files {
		if err := os.WriteFile(filepath.Join(tmp, name), data, 0o644); err != nil {
			cleanupTmp()
			return fmt.Errorf("write %s: %w", name, err)
		}
	}
	if previous != "" {
		for _, name := range keep {
			data, err := os.ReadFile(filepath.Join(previous, name))
			if os.IsNotExist(err) {
				continue
			}
			if err != nil {
				cleanupTmp()
				return fmt.Errorf("preserve %s: %w", name, err)
			}
			if err := os.WriteFile(filepath.Join(tmp, name), data, 0o644); err != nil {
				cleanupTmp()
				return fmt.Errorf("preserve %s: %w", name, err)
			}
		}
	}

	// Retire any directory already at the final path under a dot-prefixed
	// name (Corpus scans skip dot-prefixed entries, so a crash between the
	// two renames never shows up as a Video), swap the new one in, then drop
	// the retired one. If the swap fails the retired directory is put back,
	// so the Corpus never loses a Video half-way. A retired directory left by
	// an earlier crash is removed first so it cannot block the rename.
	retired := filepath.Join(channelDir, "."+videoID+".old")
	if err := os.RemoveAll(retired); err != nil {
		cleanupTmp()
		return fmt.Errorf("remove stale retired video directory: %w", err)
	}
	hadOld := false
	if _, err := os.Stat(final); err == nil {
		if err := os.Rename(final, retired); err != nil {
			cleanupTmp()
			return fmt.Errorf("move aside existing video directory: %w", err)
		}
		hadOld = true
	}
	if err := os.Rename(tmp, final); err != nil {
		cleanupTmp()
		if hadOld {
			_ = os.Rename(retired, final)
		}
		return fmt.Errorf("move video directory into place: %w", err)
	}
	// From here the Video is in place; what follows is cleanup only.
	if hadOld {
		if err := os.RemoveAll(retired); err != nil {
			return fmt.Errorf("video %s written, but cleanup failed: remove retired directory %s: %w", videoID, retired, err)
		}
	}
	if previous != "" && filepath.Clean(previous) != filepath.Clean(final) {
		if err := os.RemoveAll(previous); err != nil {
			return fmt.Errorf("video %s written, but cleanup failed: remove previous directory %s: %w", videoID, previous, err)
		}
	}
	return nil
}

// WriteFileAtomic writes data to a dot-prefixed temporary file beside path
// and renames it over path, so a reader never sees a half-written file.
func WriteFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return err
	}
	return nil
}
