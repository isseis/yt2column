//go:build test || integration

package transcript

import (
	"fmt"
	"os"
)

// SeedCacheForTest places subtitles and info as a valid cache for videoID in
// dir: slot a, with the pointer naming slot a. It is for an empty cache and
// does not check the content, so a test can seed a truncated transcript. It
// writes through the production cache-write functions, so the layout cannot
// drift from what a real run produces, and it does not depend on the helpers
// that only the test build compiles.
func SeedCacheForTest(dir, videoID string, subtitles, info []byte) error {
	slotDir, err := prepareWriteSlot(dir, videoID, slotNameA)
	if err != nil {
		return fmt.Errorf("prepare cache slot: %w", err)
	}
	for _, f := range []struct {
		path    string
		content []byte
	}{
		{subtitlesPath(slotDir, videoID), subtitles},
		{infoPath(slotDir, videoID), info},
	} {
		if err := os.WriteFile(f.path, f.content, 0o600); err != nil {
			return fmt.Errorf("write cache file: %w", err)
		}
	}
	if err := persistSlot(slotDir, videoID); err != nil {
		return fmt.Errorf("persist cache slot: %w", err)
	}
	if err := commitCache(dir, videoID, slotNameA); err != nil {
		return fmt.Errorf("commit cache: %w", err)
	}
	return nil
}
