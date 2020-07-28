# Switch library manager
Easily manage your switch game backups

![Image description](https://raw.githubusercontent.com/giwty/nsp-update/master/updates_ui.png)

![Image description](https://raw.githubusercontent.com/giwty/nsp-update/master/dlc_ui.png)
 
![Image description](https://raw.githubusercontent.com/giwty/nsp-update/master/cmd.png)

#### Features:
- Cross platform, works on Windows / Mac / Linux
- GUI and command line interfaces 
- Scan your local switch backup library (NSP/NSZ/XCI)
- Read titleId/version by decrypting NSP/XCI/NSZ (requires prod.keys)
- If no prod.keys present, fallback to read titleId/version by parsing file name  (example: `Super Mario Odyssey [0100000000010000][v0].nsp`).
- Lists missing update files (for games and DLC)
- Lists missing DLCs
- Automatically organize games per folder
- Rename files based on metadata read from NSP
- Delete old update files (in case you have multiple update files for the same game, only the latest will remain)
- Delete empty folders
- Zero dependencies, all crypto operations implemented in Go. 

## Keys (optional)
Having a prod.keys file will allow you to ensure the files you have a correctly classified.
The keys are expected to be in the traditional format, names as "prod.keys", and found in the app folder or under ${HOME}/.switch/

Note: Only the header_key, and the key_area_key_application_XX are needed.

## Settings  
During the App first launch a "settings.json" file will be created, that allows for granular control over the Apps execution.

You can customize the folder/file re-naming, as well as turn on/off features.

```
{
 "versions_etag": "",
 "titles_etag": "",
 "folder": "",
 "gui": true,
 "debug": false,
 "check_for_missing_updates": true,
 "check_for_missing_dlc": true,
 "organize_options": {
  "create_folder_per_game": false,
  "rename_files": true,
  "delete_empty_folders": true,
  "delete_old_update_files": false,
  "folder_name_template": "{TITLE_NAME}",
  "file_name_template": "{TITLE_NAME} [{DLC_NAME}][{TITLE_ID}][v{VERSION}]"
 },
 "scan_recursively": true
 "gui_page_size": 100
}
```

## Naming template
The following template elements are supported:
- {TITLE_NAME} - game name
- {TITLE_ID} - title id
- {VERSION} - version id (only applicable to files)
- {TYPE} - impacts DLCs/updates, will appear as ["UPD","DLC"]
- {DLC_NAME} - DLC name (only applicable to DLCs)

## Reporting issues
Please set debug mode to 'true', and attach the slm.log to allow for quicker resolution.

## Usage
##### Windows
- Extract the zip file
- Double click the Exe file
- If you want to use command line mode, update the settings.json with `'GUI':false`
    - Open `cmd`
    - Run `switch-library-manager.exe`
    - Optionally -f `X:\folder\containing\nsp\files"`
    - Optionally add  `-r` to recursively scan for nested folders
    - Edit the settings.json file for additional options

 
##### macOS or Linux
- Extract the zip file
- Double click the App file
- If you want to use command line mode, update the settings.json with `'GUI':false`
    - Open your Terminal
    - `cd` to the folder containing `switch-library-manager`
    - `chmod +x switch-library-manager` to make it executable
    - Run `./switch-library-manager'
    - Optionally -f `X:\folder\containing\nsp\files"`
    - Optionally add  `-r` to recursively scan for nested folders
    - Edit the settings.json file for additional options

## Building
- Install and setup latest Go
- Get the module and its dependencies: `go get -u github.com/giwty/switch-library-manager`
- Build it for the OS you need, and make sure to choose `amd64` architecture:
    - `env GOOS=target-OS GOARCH=amd64 go build github.com/giwty/switch-library-manager`
    - `target-OS` can be `windows`, `darwin` (mac OS), `linux`, or any other (check the Go documentation for a complete list).

#### Thanks
This program relies on [blawar's titledb](https://github.com/blawar/titledb), to get the latest titles and versions.
