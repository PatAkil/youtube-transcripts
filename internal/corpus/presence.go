package corpus

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FindVideo scans the Corpus two levels deep (channel directory, then video
// directory) and returns the path of the directory named videoID, or "" when
// the Video is not present. No index is kept; the layout is the index.
func FindVideo(corpusDir, videoID string) (string, error) {
	channels, err := os.ReadDir(corpusDir)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("scan corpus: %w", err)
	}
	for _, ch := range channels {
		if !ch.IsDir() || strings.HasPrefix(ch.Name(), ".") {
			continue
		}
		candidate := filepath.Join(corpusDir, ch.Name(), videoID)
		info, err := os.Stat(candidate)
		if err == nil && info.IsDir() {
			return candidate, nil
		}
	}
	return "", nil
}
