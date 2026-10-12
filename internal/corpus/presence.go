package corpus

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// WalkVideos calls fn for every Video directory in the Corpus, walking two
// levels deep: <corpus>/<channel ID>/<video ID>/. channelID is the Channel
// directory's name and videoDir the Video directory's path. Dot-prefixed
// entries at either level are skipped; that is where temporary and retired
// directories live. A Corpus directory that does not exist is empty. The
// first error fn returns stops the walk and is returned as is. No index is
// kept; the layout is the index.
func WalkVideos(corpusDir string, fn func(channelID, videoDir string) error) error {
	channels, err := os.ReadDir(corpusDir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("scan corpus: %w", err)
	}
	for _, ch := range channels {
		if !ch.IsDir() || strings.HasPrefix(ch.Name(), ".") {
			continue
		}
		channelDir := filepath.Join(corpusDir, ch.Name())
		videos, err := os.ReadDir(channelDir)
		if err != nil {
			return fmt.Errorf("scan %s: %w", channelDir, err)
		}
		for _, v := range videos {
			if !v.IsDir() || strings.HasPrefix(v.Name(), ".") {
				continue
			}
			if err := fn(ch.Name(), filepath.Join(channelDir, v.Name())); err != nil {
				return err
			}
		}
	}
	return nil
}

// errFound stops a WalkVideos early once FindVideo has its answer.
var errFound = errors.New("found")

// FindVideo returns the path of the Video directory named videoID, under
// whichever Channel directory it sits, or "" when the Video is not present.
func FindVideo(corpusDir, videoID string) (string, error) {
	var found string
	err := WalkVideos(corpusDir, func(_, videoDir string) error {
		if filepath.Base(videoDir) == videoID {
			found = videoDir
			return errFound
		}
		return nil
	})
	if err != nil && !errors.Is(err, errFound) {
		return "", err
	}
	return found, nil
}

// ReadVideo parses a video.json file. A file without a video_id is an error,
// since every Corpus consumer keys on it.
func ReadVideo(path string) (Video, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Video{}, err
	}
	var v Video
	if err := json.Unmarshal(data, &v); err != nil {
		return Video{}, fmt.Errorf("parse %s: %w", path, err)
	}
	if v.VideoID == "" {
		return Video{}, fmt.Errorf("parse %s: missing video_id", path)
	}
	return v, nil
}
