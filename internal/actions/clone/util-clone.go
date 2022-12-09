package clone

import (
	"context"
	"github.com/dustin/go-humanize"
	. "plex-go-sync/internal/filesystem"
	"plex-go-sync/internal/logger"
	"plex-go-sync/internal/models"
	"plex-go-sync/internal/plex"
	. "plex-go-sync/internal/structures"
	"strings"
	"time"
)

// humanizeNegBytes Convert a possibly negative number of bytes to a human-readable string
// @val: The number of bytes to convert
func humanizeNegBytes(val int64) string {
	if val < 0 {
		return "-" + humanize.Bytes(uint64(val*-1))
	}
	return humanize.Bytes(uint64(val))
}

// getExtension Split the filename into a base name and extension without the dot
func getExtension(path string) (base string, ext string) {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '.' {
			return path[:i], strings.ToLower(path[i+1:])
		}
	}
	return path, ""
}

// checkFreeSpace Check if we have enough free space to copy the next file
func checkFreeSpace(root string, remainingBytes int64, base string, totalBytes int64) (remaining int64, total int64) {
	fs := NewFileSystem(root)
	free, err := fs.GetFreeSpace(base)
	if err != nil {
		return remainingBytes, totalBytes
	}

	usedBytes := totalBytes - remainingBytes

	// if free space is less than the remaining allowed space, we need to decrease the allowed space
	if int64(free) < remainingBytes {
		logger.LogWarning("Not enough free space, adjusting remainingBytes from", humanize.Bytes(uint64(remainingBytes)), "to", humanize.Bytes(free))
		remainingBytes = int64(free) - (500 * humanize.MiByte)
		totalBytes = usedBytes + remainingBytes
	}
	// leave some space on the drive
	return remainingBytes, totalBytes
}

// displayCloneProcessStatus Display the current status of the clone process
func displayCloneProcessStatus(playlist *models.Playlist, existingSize int64, totalBytes int64, start time.Time) {
	if (logger.LogLevel != "WARN") && (logger.LogLevel != "ERROR") {
		// this display label can be incorrect, but it doesn't really matter for processing the playlist
		usedSize := totalBytes - playlist.Size + 1
		remaining := time.Duration((float64(time.Since(start)) / float64(usedSize-existingSize)) * float64(playlist.Size))
		if remaining < 0 { // this should be enough to check invalid times
			remaining = time.Duration(3*playlist.Items.Len()) * time.Minute // rough estimate
		}

		logger.LogPersistent(playlist.Name, logger.Green, playlist.Name, ": ",
			humanize.Bytes(uint64(usedSize)), " used of ", humanize.Bytes(uint64(totalBytes)), ", ",
			time.Since(start).Round(time.Second).String(), " elapsed, ",
			remaining.Round(time.Second).String(), " (", humanizeNegBytes(playlist.Size), ") remaining.",
			logger.Reset, "\n")
	}
}

// removeLast Remove items from the end of the playlist until we have enough space to copy the next file
func removeLast(ctx *context.Context, bytes int64, dest FileSystem, start *LinkedListItem[string, models.PlaylistItem], end *LinkedListItem[string, models.PlaylistItem], existing *map[string]uint64) int64 {
	var config = models.GetConfig(ctx)
	removed := int64(0)

	for item := end; item != start; item = item.Prev() {
		for _, path := range item.Value.Paths {
			key := plex.GetKey(path)
			if (*existing)[key] != 0 {
				logger.LogVerbose("Removing ", path)
				base, _ := getExtension(path)
				if err := dest.Remove(base + "." + config.MediaFormat.Format); err != nil {
					logger.LogWarning("error removing ", base+"."+config.MediaFormat.Format, ": ", err.Error())
				} else {
					bytes -= int64((*existing)[key])
					removed += int64((*existing)[key])
					delete(*existing, key)
				}
			}
		}
		if bytes <= 0 {
			break
		}
	}
	return removed
}
