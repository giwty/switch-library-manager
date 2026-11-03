# Naming templates

Switch Library Manager can create a folder for each game and can rename files while organizing a library. Set `organization.create_folder_per_game` and `organization.rename_files` to enable those behaviors. The folder template is `organization.folder_name_template`; the file template is `organization.file_name_template`.

Templates use tokens written in braces. A token is replaced wherever it appears. The defaults are `{TITLE_NAME}` for folders and `{TITLE_NAME} ({DLC_NAME})[{TITLE_ID}][v{VERSION}]` for files. Size tokens are optional; the defaults do not add a size to names.

## Tokens

| Token                | Folder template | File template | Value                                                                                                                                                                                                                                                                                                                                                                                    |
| -------------------- | --------------- | ------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `{TITLE_NAME}`       | Yes             | Yes           | Game name from title data, NACP metadata, or filename fallback.                                                                                                                                                                                                                                                                                                                          |
| `{TITLE_ID}`         | Yes             | Yes           | Title ID, written in uppercase.                                                                                                                                                                                                                                                                                                                                                          |
| `{VERSION}`          | Yes             | Yes           | Numeric version. Update and DLC file names use that record's version. A base in a multi-content file and its game folder use the highest local update version when present; otherwise they use `0`.                                                                                                                                                                                      |
| `{VERSION_TXT}`      | Yes             | Yes           | Display version such as `1.0.0`, when available. A DLC with numeric version `0` uses `1.0.0` when no display version is present. A multi-content base and its folder use the display version from the highest local update when present.                                                                                                                                                  |
| `{REGION}`           | Yes             | Yes           | Region from title data, when available.                                                                                                                                                                                                                                                                                                                                                  |
| `{TYPE}`             | No              | Yes           | File content type: `BASE`, `UPD`, or `DLC`.                                                                                                                                                                                                                                                                                                                                              |
| `{DLC_NAME}`         | No              | Yes           | DLC name from title data. It is empty when no DLC name applies.                                                                                                                                                                                                                                                                                                                          |
| `{PACKAGE_CONTENTS}` | No              | Yes           | Counts for base/update/DLC records in the same physical file, for example `1G+1U+2D`; absent categories are omitted. If deep contents are unavailable, a recognized source filename marker is preserved. It is empty when neither bundled records nor a recognized marker is available. The value has no punctuation, so add parentheses or other separators in the template if desired. |
| `{SIZE_GB}`          | No              | Yes           | Physical file size in decimal GB, to one decimal place, rounded to nearest with ties up. Includes the `GB` suffix, for example `0.6GB`.                                                                                                                                                                                                                                                  |
| `{SIZE_MB}`          | No              | Yes           | Physical file size in decimal MB, rounded to the nearest whole MB with ties up. Includes the `MB` suffix, for example `600MB`.                                                                                                                                                                                                                                                           |

Size uses the scanned physical file size: 1 GB is 1,000,000,000 bytes and 1 MB is 1,000,000 bytes. For a file containing multiple logical records, each record refers to the same physical file and therefore has the same size.

## Examples

Folder template:

```text
{TITLE_NAME} [{TITLE_ID}]
```

Example folder name: `Example Adventure [0100E95004039000]`.

File template:

```text
{TITLE_NAME}[{TITLE_ID}][v{VERSION}] ({PACKAGE_CONTENTS})
```

For a base package with update version `65536` and one DLC, `{PACKAGE_CONTENTS}` expands to `1G+1U+1D`, producing `Example Adventure[0100E95004039000][v65536] (1G+1U+1D).nsp`. The token contains no punctuation; the example template adds parentheses around it. To show physical size instead, use `{TITLE_NAME}[{TITLE_ID}][v{VERSION}] {SIZE_GB}`. A 600,000,000-byte base file becomes `Example Adventure[0100E95004039000][v0] 0.6GB.nsp`; replacing `{SIZE_GB}` with `{SIZE_MB}` produces `600MB`. A 2,700,000,000-byte file displays as `2.7GB` or `2700MB`.

## Naming behavior

- In Settings → Organization, turn off **Include BASE in {TYPE}** (`organization.include_base_type`) to leave `{TYPE}` empty for base files while keeping `UPD` and `DLC`. It defaults to on, including for existing settings files. For example, `{TITLE_NAME}[{TYPE}][v{VERSION}]` produces `Example Adventure[v0].nsp` for a base file when off; empty `[]`, `()`, and `<>` pairs are removed as usual.
- A file template must contain `{TITLE_NAME}` or `{TITLE_ID}` when file renaming is enabled. A folder template has the same requirement when per-game folders are enabled.
- File extensions are kept from the original file and appended after the template output; do not include an extension in the template.
- Update and DLC files can be placed in the configured `organization.updates_folder` and `organization.dlc_folder`. These subfolders can also apply when per-game folders are disabled.
- After the game title is removed from `{DLC_NAME}`, leading and trailing colons, Unicode whitespace, and dash characters are trimmed. Dashes within the DLC name are preserved.
- **Allow Unicode titles and filenames** (`organization.allow_unicode_file_names`) defaults to off. When enabled, a nonempty catalog title wins regardless of script, including Japanese, Korean, and Chinese. Library display, game details, organization previews, and `{TITLE_NAME}` share the same title resolver. Missing catalog titles still use the existing embedded English and filename fallbacks. Displayed titles are not filtered as filenames.
- With Unicode enabled, generated filenames and game folders preserve Unicode, including accented letters and emoji, without romanization. With it disabled, title selection keeps the existing English/local-name preference over CJK catalog titles, and generated names use legacy Japanese romanization and a restrictive allowlist that removes many Unicode characters. The old `switch_safe_file_names` setting is a deprecated inverse alias; see the [settings reference](settings.md#organization-templates).
- Both modes collapse whitespace and remove configured forbidden punctuation (`/`, `\`, `?`, `%`, `*`, `:`, `;`, `=`, `|`, `"`, `<`, `>`). Trailing ASCII dots and spaces are removed from generated game-folder names and renamed file stems after character filtering, on every platform. For example, `Game...` becomes the folder `Game` or the file `Game.nsp`; internal dots in `BIT.TRIP` or `Game... [TITLE_ID]` are preserved. Names that become empty are rejected before organization changes any files; include `{TITLE_ID}` in the template to retain a usable name.
- Split archives are moved as a set and keep their original part filenames.

See the [settings reference](settings.md#organization-templates) for the related settings and validation requirements.
