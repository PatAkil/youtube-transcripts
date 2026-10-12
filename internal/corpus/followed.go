package corpus

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// FollowedChannelsFile is the hand-edited list of Followed Channels at the
// Corpus root. The CLI only reads it.
const FollowedChannelsFile = "followed-channels.json"

// FollowedChannel is one entry of the Followed Channels file.
type FollowedChannel struct {
	ChannelID string `json:"channel_id"`
	Name      string `json:"name"`
}

// ReadFollowedChannels reads <corpusDir>/followed-channels.json. A missing,
// empty or malformed file is an error that names the file, so a sync with
// nothing to walk is reported rather than silently doing nothing.
func ReadFollowedChannels(corpusDir string) ([]FollowedChannel, error) {
	path := filepath.Join(corpusDir, FollowedChannelsFile)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("%s not found: list the Followed Channels there as [{\"channel_id\": \"UC...\", \"name\": \"...\"}]", path)
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var channels []FollowedChannel
	if err := json.Unmarshal(data, &channels); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if len(channels) == 0 {
		return nil, fmt.Errorf("%s lists no Followed Channels", path)
	}
	for i, ch := range channels {
		if ch.ChannelID == "" {
			return nil, fmt.Errorf("%s: entry %d has no channel_id", path, i+1)
		}
	}
	return channels, nil
}
