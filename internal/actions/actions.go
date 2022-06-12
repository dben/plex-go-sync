package actions

import (
	"fmt"
	"github.com/dustin/go-humanize"
	"github.com/urfave/cli"
	"log"
	"path"
	"plex-go-sync/internal/filesystem"
	"plex-go-sync/internal/plex"
	"strings"
	"time"
)

func SyncPlayStatus(c *cli.Context) error {
	config, err := ReadConfig(c.String("configs"), c)
	if err != nil {
		return err
	}

	source, err := plex.New(config.Server, config.Token)
	dest, err := plex.New(config.DestinationServer, config.Token)

	lib := dest.RefreshLibraries()
	if err != nil {
		return err
	}
	for key := range lib {

		destLibrary, err := dest.GetLibraryContent(key, "")
		if err != nil {
			continue
		}
		srcLibrary, libType, err := source.GetLibrarySectionByName(destLibrary.MediaContainer.LibrarySectionTitle, "")
		if err != nil {
			continue
		}

		if libType == "show" {
			for i, show := range destLibrary.MediaContainer.Metadata {
				progress := i / len(destLibrary.MediaContainer.Metadata)
				bar := strings.Repeat("#", progress*50)
				fmt.Printf("\r[%-50s]%3d%%          ", bar, progress*100)

				destEpisodes, err := dest.GetEpisodes(show.RatingKey)
				if err != nil {
					continue
				}
				srcShow := ""
				for _, srcItem := range srcLibrary.MediaContainer.Metadata {
					if srcItem.Title == show.Title {
						srcShow = srcItem.RatingKey
						break
					}
				}
				if srcShow == "" {
					continue
				}
				srcEpisodes, err := source.GetEpisodes(srcShow)
				if err != nil {
					continue
				}
				// TODO: optimize this
				for _, destEpisode := range destEpisodes.MediaContainer.Metadata {
					for _, srcEpisode := range srcEpisodes.MediaContainer.Metadata {
						if srcEpisode.ParentIndex == destEpisode.ParentIndex && srcEpisode.Index == destEpisode.Index {
							dest.SyncWatched(srcEpisode, destEpisode)
						}
					}
				}
			}
		} else if libType == "movie" {
			for i, destMovie := range destLibrary.MediaContainer.Metadata {
				progress := i / len(destLibrary.MediaContainer.Metadata)
				bar := strings.Repeat("#", progress*50)
				fmt.Printf("\r[%-50s]%3d%%          ", bar, progress*100)

				for _, srcMovie := range srcLibrary.MediaContainer.Metadata {
					if srcMovie.Title == destMovie.Title {
						dest.SyncWatched(srcMovie, destMovie)
					}
				}
			}
		}
	}
	fmt.Println()
	return nil
}

func CloneLibraries(c *cli.Context) error {
	config, err := ReadConfig(c.String("configs"), c)
	if err != nil {
		return err
	}
	dest := filesystem.NewFileSystem(config.Destination)
	src := filesystem.NewFileSystem(config.Source)

	playlists, err := LoadProgress()
	if err == nil {
		config.Playlists = playlists
	} else {
		playlists = config.Playlists
	}

	log.Println("Playlists to copy: ", len(config.Playlists))

	for j := 0; j < len(playlists); j++ {
		var playlistItemMap map[string]bool
		var playlistItems []PlaylistItem
		var base string

		var existing map[string]uint64
		var existingSize uint64
		if len(playlists[j].Items) == 0 {
			playlistItems, playlistItemMap, err = GetPlaylistItems(playlists[j].Name, config.Server, config.Token)
			playlists[j].Items = playlistItems
			base, err = getBase(playlistItems)
			log.Println("Cleaning ", path.Join(dest.GetPath(), base), " directory")
			log.Println("Existing Size: ", humanize.Bytes(existingSize))

		} else {
			playlistItemMap = make(map[string]bool)
			for _, item := range playlists[j].Items {
				playlistItemMap[item.Path] = true
			}
			playlistItems = playlists[j].Items
			base, err = getBase(playlistItems)
			existing, existingSize, err = dest.Clean(base, nil)
			log.Println("Existing Size: ", humanize.Bytes(existingSize))
		}
		log.Println(playlists[j].Name, " items: ", len(playlistItems))

		if err != nil {
			log.Println(err.Error(), ", skipping playlist")
			continue
		}

		log.Println("Cleaning ", path.Join(config.TempDir, "src", base), " directory")
		if config.TempDir != "" {
			if err = filesystem.NewLocalFileSystem(config.TempDir).RemoveAll(path.Join("src", base)); err != nil {
				log.Println(err.Error())
			}
		}

		log.Println("Cleaning ", path.Join(config.TempDir, "dest", base), " directory")
		if config.TempDir != "" {
			if err = filesystem.NewLocalFileSystem(config.TempDir).RemoveAll(path.Join("src", base)); err != nil {
				log.Println(err.Error())
			}
		}

		totalBytes := playlists[j].GetSize()
		playlists[j].Size = playlists[j].GetSize() - existingSize

		log.Println("Starting conversion")

		start := time.Now().Add(-time.Second)

		for i := 0; i < len(playlistItems); i++ {
			usedSize := totalBytes - playlists[j].Size + 1
			remaining := time.Duration((float64(time.Since(start)) / float64(usedSize)) * float64(playlists[j].Size))
			if remaining < 0 { // this should be enough to check invalid times
				remaining = time.Duration(3*len(playlistItems)) * time.Minute // rough estimate
			}
			log.Printf("%s: %s used of %s, %s elapsed, %s (%s) remaining.\n", playlists[j].Name,
				humanize.Bytes(usedSize+existingSize), playlists[j].RawSize,
				time.Since(start).Round(time.Second).String(), remaining.Round(time.Second).String(),
				humanize.Bytes(playlists[j].Size),
			)

			item := playlistItems[i]
			if existing[item.Path] > 0 {
				continue
			}
			originalBytes := src.GetSize(item.Path)
			if playlists[j].Size < originalBytes && i != len(playlistItems)-1 {
				log.Println("Cleaning up old files...")
				clearedBytes := removeLast(originalBytes, dest, playlistItems[i+1:], existing)
				playlists[j].Size += clearedBytes
			}

			tmpFile, size, err := reencodeVideo(src, dest, item.Path, config)
			if err != nil {
				log.Printf("Error reencoding %s: %s\n", tmpFile.GetAbsolutePath(), err)
				if tmpFile != nil {
					_ = tmpFile.Remove()
				}
				continue
			}
			fits, err := ifItFitsItSits(tmpFile, dest, playlists[j].Size)
			if err != nil {
				log.Printf("Error moving %s: %s\n", tmpFile.GetAbsolutePath(), err)
				continue
			}
			if !fits {
				playlists[j].Size = 0
				playlists[j].Items = []PlaylistItem{}
				break
			}

			playlists[j].Size -= size
			playlists[j].Items = playlistItems[i+1:]
			log.Println()

			_ = WriteProgress(config.Playlists)
		}
	}
	ClearProgress()
	return nil
}

func CleanLibrary(c *cli.Context) error {
	config, err := ReadConfig(c.String("configs"), c)
	if err != nil {
		return err
	}
	mediaLibrary := filesystem.NewFileSystem(config.Destination)

	for _, playlist := range config.Playlists {
		playlistItems, playlistItemMap, err := GetPlaylistItems(playlist.Name, config.Server, config.Token)
		if err != nil {
			return err
		}

		log.Println(playlist.Name, " items: ", len(playlistItems))
		base, err := getBase(playlistItems)
		if err != nil {
			log.Println(err.Error(), ", skipping playlist")
			continue
		}

		_, totalSize, err := mediaLibrary.Clean(base, playlistItemMap)
		if err != nil {
			return err
		}

		log.Printf("%s: %s used of %s\n", playlist.Name, humanize.Bytes(totalSize), humanize.Bytes(playlist.GetSize()))
		if err != nil {
			return err
		}
	}
	return nil
}
