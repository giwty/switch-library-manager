Fork of [Switch Library Manager](https://github.com/giwty/switch-library-manager) created by giwty with continued improvements and changes

# Switch library manager
Easily manage your switch game backups

![Image description](https://raw.githubusercontent.com/trembon/switch-library-manager/master/updates_ui.png)

![Image description](https://raw.githubusercontent.com/trembon/switch-library-manager/master/dlc_ui.png)
 
![Image description](https://raw.githubusercontent.com/trembon/switch-library-manager/master/cmd.png)

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
The app will look for the "prod.keys" file in the app folder or under ${HOME}/.switch/
You can also specify a custom location in the settings.json (see below)

Note: Only the header_key, and the key_area_key_application_XX keys are required.

## Settings  
During the App first launch a "settings.json" file will be created, that allows for granular control over the Apps execution.

You can customize the folder/file re-naming, as well as turn on/off features.

```
{
 "versions_etag": "W/\"c3f5ecb3392d61:0\"",
 "titles_etag": "W/\"4a4fcc163a92d61:0\"",
 "prod_keys": "",
 "folder": "",
 "scan_folders": [],
 "gui": false,
 "debug": false, # Deprecated, no longer works
 "check_for_missing_updates": true,
 "check_for_missing_dlc": true,
 "hide_missing_games": false,
 "organize_options": {
  "create_folder_per_game": false,
  "rename_files": false,
  "delete_empty_folders": false,
  "delete_old_update_files": false,
  "folder_name_template": "{TITLE_NAME}",
  "switch_safe_file_names": true,
  "file_name_template": "{TITLE_NAME} ({DLC_NAME})[{TITLE_ID}][v{VERSION}]"
 },
 "scan_recursively": true,
 "gui_page_size": 100
}
```

## Naming template
The following template elements are supported:
- {TITLE_NAME} - game name
- {TITLE_ID} - title id
- {VERSION} - version id (only applicable to files)
- {VERSION_TXT} - version number (like 1.0.0) (only applicable to files)
- {REGION} - region
- {TYPE} - impacts DLCs/updates, will appear as ["UPD","DLC"]
- {DLC_NAME} - DLC name (only applicable to DLCs)

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
- Install and setup Go
- Clone the repo: `git clone https://github.com/trembon/switch-library-manager.git`
- Get the bundler `go get -u github.com/asticode/go-astilectron-bundler/...`
- Install bundler `go install github.com/asticode/go-astilectron-bundler/astilectron-bundler`
- Copy bundler binary to the source folder `cd switch-library-manager` and then `mv $HOME/go/bin/astilectron-bundler .`
- Execute `./astilectron-bundler`
- Binaries will be available under output

#### Thanks
This program relies on [blawar's titledb](https://github.com/blawar/titledb), to get the latest titles and versions.
