package corpus

import (
	"fmt"
	"time"
)

// Timestamp renders a cue start as [mm:ss], or [h:mm:ss] from one hour on.
func Timestamp(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	total := int(d / time.Second)
	h, m, s := total/3600, (total%3600)/60, total%60
	if h > 0 {
		return fmt.Sprintf("[%d:%02d:%02d]", h, m, s)
	}
	return fmt.Sprintf("[%02d:%02d]", m, s)
}
