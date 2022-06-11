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

### Commands:
```
sync     Sync play status
   Options
   --configs value, -c value             configuration file (default: "configs.json")
   --server value, -s value              plex server address
   --token value, -t value               plex server token
   --library value, -l value             library to sync
   --destination-server value, -d value  destination server address
   
clone    Clone a set of libraries
    Options
   --configs value, -c value             configuration file (default: "configs.json")
   --server value, -s value              plex server address
   --token value, -t value               plex server token
   --playlist value, -p value            playlist to clone
   --size value                          max size of playlist to copy
   --destination-server value, -d value  destination server address
   --source value, --src value           source path
   --destination value, --dest value     destination path

clean    Clean a destination library
    Options
   --configs value, -c value             configuration file (default: "configs.json")
   --server value, -s value              plex server address
   --token value, -t value               plex server token
   --library value, -l value             library to clean
   --destination-server value, -d value  destination server address
   --destination value, --dest value     destination path
```