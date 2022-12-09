# Plex Sync
This app lets you sync a large plex library to a smaller backup plex server. 
It does this by downconverting the video files to 720p and then copying as many
files as possible in random order to the backup server, given an explicit size 
constraint. It will also sync the play status and position of the videos.

My use for this is to run a portable plex server on a raspberry pi that I can
bring in my car to have a large portable library for road trips.

### Usage:
```
plex-go-sync command [command options]
```
Example usage:
```
./plex-go-sync clone -c ./configs.json
```

### Commands:
```
sync     Sync play status
   Options
   --config FILE, -c FILE                Load configuration from FILE (default: "configs.json")
   --destination-server value, -o value  Destination server address
   --library value, -l value             Library to sync  (accepts multiple inputs)
   --loglevel value                      One of VERBOSE, INFO, WARN, ERROR
   --server value, -i value              Plex server address
   --token value, -t value               Plex server token

   
clone    Clone a set of libraries
    Options
   --config FILE, -c FILE                       Load configuration from FILE (default: "configs.json")
   --destination value, -d value, --dest value  Destination path
   --destination-server value, -o value         Destination server address
   --fast, -f                                   Skip files requiring full encodings (default: false)
   --loglevel value                             One of VERBOSE, INFO, WARN, ERROR
   --playlist value, -p value                   Playlist to clone  (accepts multiple inputs)
   --reset, -r                                  Start sync from the beginning (default: false)
   --server value, -i value                     Plex server address
   --size value                                 Max size of playlist to copy  (accepts multiple inputs)
   --source value, -s value, --src value        Source path
   --token value, -t value                      Plex server token

clean    Clean a destination library
    Options
   --config FILE, -c FILE                       Load configuration from FILE (default: "configs.json")
   --destination value, -d value, --dest value  Destination path
   --destination-server value, -o value         Destination server address
   --library value, -l value                    Library to sync  (accepts multiple inputs)
   --loglevel value                             One of VERBOSE, INFO, WARN, ERROR
   --server value, -i value                     Plex server address
   --token value, -t value                      Plex server token
```

## Configuration file format:

```json 
{
  "tempDir": "/home/david/convert", // A local directory to be used to store temporary files 
  "sourceServer": "http://192.168.1.110:32400", // The source plex server
  "destinationServer": "http://192.168.1.45:32400", // The destination plex server
  "token": "", // A Plex API token
  "sourcePath": "smb://guest@192.168.1.100", // The path to the source library
  "destinationPath": "smb://guest@192.168.1.45", // The path to the destination library
  "playlists": [ // A list of playlists to sync the files from
    {
      "name": "TV Sync List", // The name of the playlist
      "size": "100G" // The maximum size to copy
      "clean": true // Whether to clean the destination library of extraneous files before copying
    },
    {
      "name": "Movie Sync List",
      "size": "100G"
      "clean": false
    }
  ]
}
```

## Building:
To build the app, just run:
```
go build
```
