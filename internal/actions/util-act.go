package actions

import (
	"encoding/json"
	"errors"
	"github.com/dustin/go-humanize"
	"github.com/urfave/cli/v2"
	"os"
	"plex-go-sync/internal/logger"
	"strings"
)

func ReadConfig(configFile string, args *cli.Context) (*Config, error) {
	var config Config
	logger.LogInfo("Loading config...")
	file, err := os.Open(configFile)
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&config); err != nil {
		return nil, err
	}

	if args.String("server") != "" {
		config.Server = args.String("server")
	}
	if args.String("token") != "" {
		config.Token = args.String("token")
	}
	if args.String("destination-server") != "" {
		config.DestinationServer = args.String("destination-server")
	}
	if args.StringSlice("playlist") != nil && len(args.StringSlice("playlist")) > 0 {
		for i := 0; i < len(args.StringSlice("playlist")); i++ {
			config.Playlists = append(config.Playlists, *NewPlaylist(args.StringSlice("playlist")[i], args.StringSlice("size")[i]))
		}
	}

	if logger.LogLevel == "VERBOSE" {
		writer := json.NewEncoder(os.Stdout)
		writer.SetIndent("", "  ")
		_ = writer.Encode(config)
	}

	return &config, err
}

func getBase(items []PlaylistItem) (string, error) {
	if len(items) == 0 {
		return "", errors.New("no items")
	}
	base, _, ok := strings.Cut(strings.TrimPrefix(items[0].Path, "/"), "/")
	if !ok {
		return "", errors.New("invalid path structure")
	}
	return base, nil
}

func humanizeNegBytes(val int64) string {
	if val < 0 {
		return "-" + humanize.Bytes(uint64(val*-1))
	}
	return humanize.Bytes(uint64(val))
}
