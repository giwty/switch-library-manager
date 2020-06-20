# NSP Update
A simple tool to scan your NSP folder for possible updates.
![Image description](https://raw.githubusercontent.com/giwty/nsp-update/master/screenshot.png)

This program relies on updated lists from [blawar's titledb](https://github.com/blawar/titledb). 
It downloads the titles and versions JSON lists and compares it to the local .NSP files.

Local .NSP files must contain their titleId and version in their filename (for example `Super Mario Odyssey [0100000000010000][v0].nsp`).


## Usage
##### Windows
- Run `cmd.exe`
- `cd` to the folder containing `nsp-update.exe`
- Run `nsp-update.exe -f "X:\folder\containing\nsp\files"`
- Optionally add  `-r` to recursively scan for nested folders
- Add  `-m dlc` to show missing dlc's
- Add  `-m u` to show missing updates (default)
- Add  `-m o` to organize the files in a folder per game structure, where the folder will contains base/updates/dlc files in a flat structure.
- Add  `-m d` to *delete* outdated local update files
 
##### macOS or Linux
- Open your Terminal
- `cd` to the folder containing `nsp-update`
- `chmod +x nsp-update` to make it executable
- Run `./nsp-update -f "/folder/containing/nsp/files"`
- Optionally add  `-r` to recursively scan for nested folders
- Add  `-m dlc` to show missing dlc's
- Add  `-m u` to show missing updates (default)
- Add  `-m o` to organize the files in a folder per game structure, where the folder will contains base/updates/dlc files in a flat structure.
- Add  `-m d` to *delete* outdated local update files



## Building
- Install and setup latest Go
- Get the module and its dependencies: `go get -u github.com/giwty/nsp-update`
- Build it for the OS you need, and make sure to choose `amd64` architecture:
    - `env GOOS=target-OS GOARCH=amd64 go build github.com/giwty/nsp-update`
    - `target-OS` can be `windows`, `darwin` (mac OS), `linux`, or any other (check the Go documentation for a complete list).
