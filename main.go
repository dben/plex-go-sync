package main

import (
	_ "fmt"
	"github.com/urfave/cli/v2"
	"os"
	"plex-go-sync/internal/actions"
	"plex-go-sync/internal/logger"
)

func main() {
	app := &cli.App{
		Name:    "plex-go-sync",
		Usage:   "Sync Plex Libraries",
		Version: "0.9.0",
		Authors: []*cli.Author{
			{
				Name: "David Benson",
			},
		},
		Commands: []*cli.Command{
			{
				Name:   "sync",
				Usage:  "Sync play status only",
				Action: actions.SyncPlayStatus,
				Flags: []cli.Flag{
					&cli.PathFlag{
						Name:      "config",
						Aliases:   []string{"c"},
						Value:     "configs.json",
						Usage:     "Load configuration from `FILE`",
						TakesFile: true,
					},
					&cli.StringFlag{
						Name:    "server",
						Aliases: []string{"i"},
						Usage:   "Plex server address",
					},
					&cli.StringFlag{
						Name:    "token",
						Aliases: []string{"t"},
						Usage:   "Plex server token",
					},
					&cli.StringSliceFlag{
						Name:    "library",
						Aliases: []string{"l"},
						Usage:   "Library to sync",
					},
					&cli.StringFlag{
						Name:    "destination-server",
						Aliases: []string{"o"},
						Usage:   "Destination server address",
					},
					&cli.StringFlag{
						Name:  "loglevel",
						Usage: "One of VERBOSE, INFO, WARN, ERROR",
					},
				},
			},
			{
				Name:   "clone",
				Usage:  "Clone a set of libraries",
				Action: actions.CloneLibraries,
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:      "config",
						Aliases:   []string{"c"},
						Value:     "configs.json",
						Usage:     "Load configuration from `FILE`",
						TakesFile: true,
					},
					&cli.StringFlag{
						Name:    "server",
						Aliases: []string{"i"},
						Usage:   "Plex server address",
					},
					&cli.StringFlag{
						Name:    "token",
						Aliases: []string{"t"},
						Usage:   "Plex server token",
					},
					&cli.StringSliceFlag{
						Name:    "playlist",
						Aliases: []string{"p"},
						Usage:   "Playlist to clone",
					},
					&cli.StringSliceFlag{
						Name:  "size",
						Usage: "Max size of playlist to copy",
					},
					&cli.StringFlag{
						Name:    "destination-server",
						Aliases: []string{"o"},
						Usage:   "Destination server address",
					},
					&cli.StringFlag{
						Name:    "source",
						Aliases: []string{"s", "src"},
						Usage:   "Source path",
					},
					&cli.StringFlag{
						Name:    "destination",
						Aliases: []string{"d", "dest"},
						Usage:   "Destination path",
					},
					&cli.BoolFlag{
						Name:    "reset",
						Aliases: []string{"r"},
						Usage:   "Start sync from the beginning",
					},
					&cli.BoolFlag{
						Name:    "fast",
						Aliases: []string{"f"},
						Usage:   "Skip files requiring full encodings",
					},
					&cli.StringFlag{
						Name:  "loglevel",
						Usage: "One of VERBOSE, INFO, WARN, ERROR",
					},
				},
			},
			{
				Name:   "clean",
				Usage:  "Clean a destination library",
				Action: actions.CleanLibrary,
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:      "config",
						Aliases:   []string{"c"},
						Value:     "configs.json",
						Usage:     "Load configuration from `FILE`",
						TakesFile: true,
					},
					&cli.StringFlag{
						Name:    "server",
						Aliases: []string{"i"},
						Usage:   "Plex server address",
					},
					&cli.StringFlag{
						Name:    "token",
						Aliases: []string{"t"},
						Usage:   "Plex server token",
					},
					&cli.StringSliceFlag{
						Name:    "library",
						Aliases: []string{"l"},
						Usage:   "Library to sync",
					},
					&cli.StringFlag{
						Name:    "destination-server",
						Aliases: []string{"o"},
						Usage:   "Destination server address",
					},
					&cli.StringFlag{
						Name:    "destination",
						Aliases: []string{"d", "dest"},
						Usage:   "Destination path",
					},
					&cli.StringFlag{
						Name:  "loglevel",
						Usage: "One of VERBOSE, INFO, WARN, ERROR",
					},
				},
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		logger.LogError(err)
	}
}
