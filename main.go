package main

import (
	"context"
	_ "fmt"
	"github.com/urfave/cli/v2"
	"os"
	"os/signal"
	"plex-go-sync/internal/actions/clean"
	"plex-go-sync/internal/actions/clone"
	"plex-go-sync/internal/actions/sync"
	"plex-go-sync/internal/logger"
	"syscall"
)

func main() {
	app := &cli.App{
		Name:    "plex-go-sync",
		Usage:   "Sync Plex Libraries",
		Version: "0.9.1",
		Authors: []*cli.Author{
			{
				Name: "David Benson",
			},
		},
		Commands: []*cli.Command{
			{
				Name:   "sync",
				Usage:  "Sync play status only",
				Action: sync.FromContext,
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
					&cli.BoolFlag{
						Name:    "fast",
						Aliases: []string{"f"},
						Usage:   "Skip checking by duration if duration isn't specified in metadata",
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
				Name: "clone",
				Usage: "Clone a set of libraries. If the clean flag has been set, the destination library will have " +
					"all non-playlist items removed first. After cloning, the destination library will have play " +
					"status synced with the source library.",
				Action: clone.FromContext,
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
						Usage:   "Playlists to clone",
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
						Usage:   "Start sync from the beginning, instead of reading from the progress file",
					},
					&cli.BoolFlag{
						Name:    "fast",
						Aliases: []string{"f"},
						Usage:   "Skip files requiring full encodings",
					},
					&cli.UintFlag{
						Name:    "threads",
						Aliases: []string{"x"},
						Usage:   "Number of threads to use",
						Value:   2,
					},
					&cli.StringFlag{
						Name:  "loglevel",
						Usage: "One of VERBOSE, INFO, WARN, ERROR",
					},
				},
			},
			{
				Name:   "clean",
				Usage:  "Clean a destination library of any files not included in the provided playlists",
				Action: clean.FromContext,
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
					&cli.UintFlag{
						Name:    "threads",
						Aliases: []string{"x"},
						Usage:   "Number of threads to use",
						Value:   2,
					},
					&cli.StringSliceFlag{
						Name:    "playlist",
						Aliases: []string{"p"},
						Usage:   "Playlists to use",
					},
					&cli.StringFlag{
						Name:  "loglevel",
						Usage: "One of VERBOSE, INFO, WARN, ERROR",
					},
				},
			},
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	exit := make(chan os.Signal, 1)
	signal.Notify(exit, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-exit
		logger.LogWarning("Received interrupt, shutting down")
		cancel()
	}()

	if err := app.RunContext(ctx, os.Args); err != nil {
		logger.LogError(err)
	}
}
