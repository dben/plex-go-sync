package main

import (
	_ "fmt"
	"github.com/urfave/cli"
	"log"
	"os"
	"plex-go-sync/internal/actions"
)

func main() {
	app := cli.NewApp()
	app.Name = "plex-go-sync"
	app.Usage = "Sync Plex Libraries"
	app.Version = "0.0.1"

	app.Commands = []cli.Command{
		{
			Name:   "sync",
			Usage:  "Sync play status",
			Action: actions.SyncPlayStatus,
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:      "configs, c",
					Value:     "configs.json",
					Usage:     "configuration file",
					TakesFile: true,
				},
				cli.StringFlag{
					Name:  "server, s",
					Usage: "plex server address",
				},
				cli.StringFlag{
					Name:  "token, t",
					Usage: "plex server token",
				},
				cli.StringSliceFlag{
					Name:  "library, l",
					Usage: "library to sync",
				},
				cli.StringFlag{
					Name:  "destination-server, d",
					Usage: "destination server address",
				},
			},
		},
		{
			Name:   "clone",
			Usage:  "Clone a set of libraries",
			Action: actions.CloneLibraries,
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:      "configs, c",
					Value:     "configs.json",
					Usage:     "configuration file",
					TakesFile: true,
				},
				cli.StringFlag{
					Name:  "server, s",
					Usage: "plex server address",
				},
				cli.StringFlag{
					Name:  "token, t",
					Usage: "plex server token",
				},
				cli.StringSliceFlag{
					Name:  "playlist, p",
					Usage: "playlist to clone",
				},
				cli.StringSliceFlag{
					Name:  "size",
					Usage: "max size of playlist to copy",
				},
				cli.StringFlag{
					Name:  "destination-server, d",
					Usage: "destination server address",
				},
				cli.StringFlag{
					Name:  "source, src",
					Usage: "source path",
				},
				cli.StringFlag{
					Name:  "destination, dest",
					Usage: "destination path",
				},
			},
		},
		{
			Name:   "clean",
			Usage:  "Clean a destination library",
			Action: actions.CleanLibrary,
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:      "configs, c",
					Value:     "configs.json",
					Usage:     "configuration file",
					TakesFile: true,
				},
				cli.StringFlag{
					Name:  "server, s",
					Usage: "plex server address",
				},
				cli.StringFlag{
					Name:  "token, t",
					Usage: "plex server token",
				},
				cli.StringSliceFlag{
					Name:  "library, l",
					Usage: "library to clean",
				},
				cli.StringFlag{
					Name:  "destination-server, d",
					Usage: "destination server address",
				},
				cli.StringFlag{
					Name:  "destination, dest",
					Usage: "destination path",
				},
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Println(err)
	}
}
