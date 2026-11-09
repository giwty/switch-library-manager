# Settings

Switch Library Manager uses a categorized `settings.json` format. The file is valid JSON and must contain the current `"schema_version"` value, which is currently `2`.

When using the graphical interface, the Settings tab provides a structured editor for these options. The schema version and internal cache values are not shown. Saving applies organization rules, appearance, page size, missing-content preferences, and window-state recording immediately. Organization changes affect previews and the next Organize action; saving never organizes or deletes files.

After a save that does not require restarting, the Settings panel returns to the top and a **Settings saved** dialog confirms the result. Select **OK** to continue. Save errors remain visible at the top of the editor with your draft preserved. If the confirmation dialog cannot be displayed, an inline warning explains that the settings were still saved.

Changes to library folders, scan folders, keys paths, scan rules, data-source URLs, or debug logging require restarting. The notice lists the reasons and blocks other actions until you select **Restart now**, or close and relaunch the application. A save containing both immediate and restart-required changes follows this restart flow. The Library folder picker uses the same policy. Changed paths or scan rules invalidate the library snapshot so the next launch scans the new configuration even when startup rescanning is disabled; cached deep metadata is retained.

The default GUI mode and startup rescan preference apply at the next launch without forcing a restart. The missing-update and missing-DLC check switches control console mode only; the GUI tabs remain available. Manual edits to `settings.json` require restarting and are not watched automatically.

## Runtime data directory

On macOS, runtime files are stored in `~/Library/Application Support/Switch Library Manager/`. Windows and Linux continue to store them beside the executable. Existing files in the former macOS location are left untouched and are not copied into the new profile. On macOS, `prod.keys` lookup checks this application data directory before the home `.switch` directory; it does not check beside the app.

## Example

```json
{
  "schema_version": 2,
  "gui": {
    "enabled": true,
    "page_size": 100,
    "hide_missing_games": false,
    "hide_demo_games": false,
    "hide_version_string": false,
    "remember_window_state": false,
    "theme": "inherit",
    "date_format": "YYYY-MM-DD"
  },
  "paths": {
    "library_folder": "",
    "scan_folders": [],
    "prod_keys": ""
  },
  "scan": {
    "recursive": true,
    "rescan_on_startup": false,
    "ignore_file_types": [],
    "ignore_unsupported_file_types": false
  },
  "organization": {
    "create_folder_per_game": false,
    "move_scan_files_to_library": false,
    "dlc_folder": "",
    "updates_folder": "",
    "rename_files": false,
    "delete_empty_folders": false,
    "delete_old_update_files": false,
    "folder_name_template": "{TITLE_NAME}",
    "allow_unicode_file_names": false,
    "file_name_template": "{TITLE_NAME} ({DLC_NAME})[{TITLE_ID}][v{VERSION}]",
    "include_base_type": true,
    "process_when_missing_base_game": false
  },
  "missing_content": {
    "check_for_updates": true,
    "check_for_dlc": true,
    "ignore_dlc_updates": false,
    "ignore_dlc_title_ids": ["01007F600B135007"],
    "ignore_update_title_ids": []
  },
  "data_sources": {
    "titles_url": "https://github.com/trembon/switch-library-manager/releases/download/data/titles.json",
    "versions_url": "https://github.com/trembon/switch-library-manager/releases/download/data/versions.json"
  },
  "logging": {
    "debug": false
  }
}
```

## Settings reference

### Date format

`gui.date_format` is a string defaulting to ISO 8601 (`YYYY-MM-DD`). In **Settings → Interface → Date format**, choose ISO, month first (`MM/DD/YYYY`), day first (`DD/MM/YYYY`), or **Custom**. Custom formats show validation feedback and a live example.

The preference controls release/update dates in GUI tables and Game Details, console tables, and date values in both GUI and console CSV/Excel exports. Excel date values remain literal text. Source dates, chronological sorting, and ISO dates in export filenames are preserved. Unavailable or malformed source dates show **Unknown** in date cells and export as blank; absent optional metadata is omitted.

Custom patterns contain exactly one `YYYY`, one month token (`M`, `MM`, `MMM`, `MMMM`), and one day token (`D`, `DD`), in any order. Separate tokens with one or more spaces or `/`, `-`, `.`, `,`. `M`/`D` omit leading zeroes, `MM`/`DD` include them, and `MMM`/`MMMM` use abbreviated/full English month names. Patterns are limited to 32 characters after trimming surrounding ASCII whitespace. Examples: `DD.MM.YYYY`, `D MMM YYYY`, `MMMM D, YYYY`. Time tokens, arbitrary literal text, and repeated date components are unsupported.

### All settings

| Setting                                       | Type         | Default                                               | Purpose                                                                                                                                                                                                                      |
| --------------------------------------------- | ------------ | ----------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `schema_version`                              | integer      | `2`                                                   | Identifies the settings format. It is required and is not an operational preference.                                                                                                                                         |
| `gui.enabled`                                 | boolean      | `true`                                                | Starts the graphical interface when true. The `-m` command-line flag overrides it.                                                                                                                                           |
| `gui.page_size`                               | integer      | `100`                                                 | Number of rows shown per page in GUI tables. Values less than or equal to zero are reset to 100.                                                                                                                             |
| `gui.hide_missing_games`                      | boolean      | `false`                                               | Hides the missing-games tab in the GUI.                                                                                                                                                                                      |
| `gui.hide_demo_games`                         | boolean      | `false`                                               | Excludes demo titles from the GUI missing-games list.                                                                                                                                                                        |
| `gui.hide_version_string`                     | boolean      | `false`                                               | Hides the Library Version column and dotted local version strings in Missing Updates.                                                                                                                                        |
| `gui.remember_window_state`                   | boolean      | `false`                                               | Restores and saves the GUI window size and position when enabled. Saved bounds are kept but ignored while disabled.                                                                                                          |
| `gui.theme`                                   | string       | `"inherit"`                                           | GUI color mode: `inherit` follows the operating system, while `light` and `dark` force a mode. Invalid or missing values use `inherit`.                                                                                      |
| `gui.date_format`                             | string       | `"YYYY-MM-DD"`                                        | Date format for GUI/console views and CSV/Excel values. See Date format above for supported tokens, validation, and examples.                                                                                                |
| `paths.library_folder`                        | string       | `""`                                                  | Main folder scanned by the GUI and console workflow. The `-f` flag overrides it for a console run.                                                                                                                           |
| `paths.scan_folders`                          | string array | `[]`                                                  | Additional folders scanned alongside the main library folder. They are read-only for organization unless `move_scan_files_to_library` is enabled.                                                                            |
| `paths.prod_keys`                             | string       | `""`                                                  | Optional file or directory containing `prod.keys`. If it does not resolve, the application checks its data directory and the home `.switch` folder.                                                                          |
| `scan.recursive`                              | boolean      | `true`                                                | Scans subdirectories. The `-r` flag overrides it for a console run.                                                                                                                                                          |
| `scan.rescan_on_startup`                      | boolean      | `false`                                               | Checks configured folders at GUI startup for added or removed files while reusing cached metadata.                                                                                                                           |
| `scan.ignore_file_types`                      | string array | `[]`                                                  | File extensions ignored when reporting unsupported file types. Extensions may include or omit the leading dot.                                                                                                               |
| `scan.ignore_unsupported_file_types`          | boolean      | `false`                                               | Suppresses diagnostics for files with unsupported extensions. When false, unsupported files are reported; errors for supported extensions are still reported.                                                                |
| `organization.create_folder_per_game`         | boolean      | `false`                                               | Creates a folder for each game during organization.                                                                                                                                                                          |
| `organization.move_scan_files_to_library`     | boolean      | `false`                                               | Moves recognized Switch files found outside the library folder into the library during organization, applying the active organization settings.                                                                              |
| `organization.dlc_folder`                     | string       | `""`                                                  | Optional subfolder for DLC files.                                                                                                                                                                                            |
| `organization.updates_folder`                 | string       | `""`                                                  | Optional subfolder for update files.                                                                                                                                                                                         |
| `organization.rename_files`                   | boolean      | `false`                                               | Renames files using `file_name_template`.                                                                                                                                                                                    |
| `organization.delete_empty_folders`           | boolean      | `false`                                               | Removes empty folders inside the library folder after files were moved or old updates were deleted. Scan folders are never cleaned.                                                                                          |
| `organization.delete_old_update_files`        | boolean      | `false`                                               | Deletes verified duplicate and old update files. Files in scan folders are only deleted when `move_scan_files_to_library` is enabled.                                                                                        |
| `organization.folder_name_template`           | string       | `"{TITLE_NAME}"`                                      | Template used for game folder names.                                                                                                                                                                                         |
| `organization.allow_unicode_file_names`       | boolean      | `false`                                               | Accepts catalog titles in any script for library display, game details, and naming; preserves Unicode in generated filenames and game folders. When off, uses legacy title selection, romanization, and character filtering. |
| `organization.file_name_template`             | string       | `"{TITLE_NAME} ({DLC_NAME})[{TITLE_ID}][v{VERSION}]"` | Template used when renaming files. See the [naming template guide](naming.md) for available tokens and examples.                                                                                                             |
| `organization.include_base_type`              | boolean      | `true`                                                | Includes `BASE` in the file template's `{TYPE}` token. When false, it is empty for base files; `UPD` and `DLC` are unchanged. Missing settings default to true.                                                              |
| `organization.process_when_missing_base_game` | boolean      | `false`                                               | Allows updates and DLC to be organized even when the base game is absent.                                                                                                                                                    |
| `missing_content.check_for_updates`           | boolean      | `true`                                                | Runs the missing-updates check.                                                                                                                                                                                              |
| `missing_content.check_for_dlc`               | boolean      | `true`                                                | Runs the missing-DLC check.                                                                                                                                                                                                  |
| `missing_content.ignore_dlc_updates`          | boolean      | `false`                                               | Excludes DLC updates from the missing-updates calculation.                                                                                                                                                                   |
| `missing_content.ignore_dlc_title_ids`        | string array | `["01007F600B135007"]`                                | DLC title IDs excluded from the missing-DLC calculation. IDs are compared case-insensitively.                                                                                                                                |
| `missing_content.ignore_update_title_ids`     | string array | `[]`                                                  | Update title IDs excluded from the missing-updates calculation. IDs are compared case-insensitively.                                                                                                                         |
| `data_sources.titles_url`                     | string       | SLM `data` release `titles.json` asset                | URL for title metadata in ordinary JSON or Zstandard-compressed JSON (`.json.zst`).                                                                                                                                          |
| `data_sources.versions_url`                   | string       | SLM `data` release `versions.json` asset              | URL for version metadata in ordinary JSON or Zstandard-compressed JSON (`.json.zst`).                                                                                                                                        |
| `logging.debug`                               | boolean      | `false`                                               | Enables debug-level logging in `slm.log`.                                                                                                                                                                                    |

## Compressed data sources

Both data-source URLs accept ordinary JSON or Zstandard-compressed JSON containing the same title/version schema. Compression is detected from the downloaded content, so a `.zst` extension or a particular HTTP content type is not required. No extra setting or external decompression program is needed, and the default source URLs are unchanged.

Downloaded data is decompressed and validated before replacing the local `titles.json` or `versions.json`. Local files remain ordinary JSON. Downloads accept up to 512 MiB of JSON and Zstandard frames with a decoder window up to 64 MiB. If downloading, decompression, or validation fails, SLM keeps using a valid local JSON file; without one, loading fails with an error. This support applies to remote data-source URLs, not local `.zst` file imports.

## Organization templates

`organization.switch_safe_file_names` is deprecated and accepted only when loading settings. Its value is inverted: `true` maps to `allow_unicode_file_names: false`, and `false` maps to `allow_unicode_file_names: true`, including Unicode catalog title selection. An explicit `allow_unicode_file_names` wins if both keys exist. If neither exists, Unicode is disabled. Loading does not rewrite the file; saving writes only the new key. This remains settings schema version 2 and does not reset other settings.

Folder and file templates use brace-delimited tokens. The [naming template guide](naming.md) lists all tokens, describes which template accepts each one, and gives examples. When `rename_files` is enabled, `file_name_template` must contain `{TITLE_NAME}` or `{TITLE_ID}`. When `create_folder_per_game` is enabled, `folder_name_template` must contain `{TITLE_NAME}` or `{TITLE_ID}`.

Organization only ever builds destinations inside `paths.library_folder` (or the `-f` folder for a console run), apart from absolute Updates/DLC folder paths, which remain explicit destinations.

With `move_scan_files_to_library` off (the default), folders in `paths.scan_folders` are read-only: their files appear in the library views, but Organize never moves, renames, or deletes them, and empty-folder cleanup does not touch them. A scan folder nested inside the library folder is treated as part of the library, although the configured folder itself is never removed.

When `move_scan_files_to_library` is enabled, Organize imports recognized Switch files from outside the library into it, applying the active settings: per-game folders, renaming, and Updates/DLC folders. Without per-game folders, imported files are placed directly in the library folder. Verified duplicate or old files in scan folders may then be deleted by `delete_old_update_files`, but empty folders left behind in scan folders are kept.

## Internal cache

`cache.json` stores the HTTP ETags and the source URL and SHA-256 hash associated with each local data file:

```json
{
  "titles_etag": "...",
  "titles_url": "https://github.com/trembon/switch-library-manager/releases/download/data/titles.json",
  "titles_sha256": "...",
  "versions_etag": "...",
  "versions_url": "https://github.com/trembon/switch-library-manager/releases/download/data/versions.json",
  "versions_sha256": "..."
}
```

ETags are runtime cache state rather than user preferences, so they are intentionally excluded from `settings.json`. The application sends an ETag only when the configured URL and local file hash match the cached values. A missing or malformed cache is treated as empty, and older caches without URL and hash metadata trigger an unconditional download. A valid local JSON file remains available as a fallback when the request fails.

For compressed sources, the ETag still comes from the remote response, while the SHA-256 hash is calculated from the decompressed local JSON.

## Migrating older settings

On startup, the application checks the `schema_version` in `settings.json`. If the marker is missing or lower than the current schema, the file is preserved by renaming it to `settings.old.json`. If that name already exists, it uses `settings.old.1.json`, `settings.old.2.json`, and so on. A new `settings.json` containing current default values is then created.

Older settings are not converted automatically. Copy values into the generated current settings file using this mapping for the original settings format:

| Older setting               | Current setting                           |
| --------------------------- | ----------------------------------------- |
| `gui`                       | `gui.enabled`                             |
| `gui_page_size`             | `gui.page_size`                           |
| `hide_missing_games`        | `gui.hide_missing_games`                  |
| `hide_demo_games`           | `gui.hide_demo_games`                     |
| `folder`                    | `paths.library_folder`                    |
| `scan_folders`              | `paths.scan_folders`                      |
| `prod_keys`                 | `paths.prod_keys`                         |
| `scan_recursively`          | `scan.recursive`                          |
| `ignore_file_types`         | `scan.ignore_file_types`                  |
| `organize_options.*`        | `organization.*`                          |
| `check_for_missing_updates` | `missing_content.check_for_updates`       |
| `check_for_missing_dlc`     | `missing_content.check_for_dlc`           |
| `ignore_dlc_updates`        | `missing_content.ignore_dlc_updates`      |
| `ignore_dlc_title_ids`      | `missing_content.ignore_dlc_title_ids`    |
| `ignore_update_title_ids`   | `missing_content.ignore_update_title_ids` |
| `titles_json_url`           | `data_sources.titles_url`                 |
| `versions_json_url`         | `data_sources.versions_url`               |
| `debug`                     | `logging.debug`                           |

The older `titles_etag` and `versions_etag` values are not copied into `settings.json`; they belong in `cache.json`. If they are omitted, the application will rebuild the cache and refresh the local metadata as needed.

Set `schema_version` to the current value, `2`. A file marked with a future schema version or containing malformed JSON is rejected without being overwritten so that it can be migrated manually.

The recognized older format is a valid JSON object with no `schema_version` or with a lower value. A non-object JSON value and unsupported future schema versions remain in place and stop startup. Console mode also stops after an older-settings migration so it cannot run scans or destructive organization using unreviewed defaults; use the GUI or edit the generated file first.
