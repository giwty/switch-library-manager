# Keyboard shortcuts

Press **F1** or select the **?** button beside the tabs to open shortcut help in the app. Buttons keep their normal labels; this guide and the help dialog list the shortcuts.

Use **Ctrl** on Windows and Linux, and **Command (Cmd)** on macOS. The existing **Ctrl+R** and **Ctrl+Shift+R** shortcuts also continue to work on macOS.

## Tabs

| Windows / Linux | macOS | Tab |
|---|---|---|
| Ctrl+1 | Cmd+1 | Library |
| Ctrl+2 | Cmd+2 | Missing games |
| Ctrl+3 | Cmd+3 | Missing updates |
| Ctrl+4 | Cmd+4 | Missing DLC |
| Ctrl+5 | Cmd+5 | Organize |
| Ctrl+6 | Cmd+6 | Issues |
| Ctrl+7 | Cmd+7 | Settings |

Numbers stay fixed when **Missing games** is hidden. In that case, Ctrl/Cmd+2 does nothing. Keyboard tab switching focuses the first game row when available, or the panel when it has no game rows.

## Library actions

| Windows / Linux | macOS | Action |
|---|---|---|
| Ctrl+O | Cmd+O | Organize the library, after the usual confirmation |
| Ctrl+R | Cmd+R | Rescan the library |
| Ctrl+Shift+R | Cmd+Shift+R | Hard rescan: ignore cached file metadata and read the files again |
| Ctrl+F | Cmd+F | Focus and select the current table’s name filter |

These actions work from any tab while the app is ready. Organize and rescan require a configured library folder. Organize uses the saved settings; unsaved settings remain a draft. Canceling its confirmation leaves the library unchanged.

Search uses the **Title** filter in Library, Missing games and Missing updates, and the **Missing DLC** filter in Missing DLC. It does nothing on tabs without a searchable table. Filtering remains specific to the current table.

## Rows and game details

| Keys | Action |
|---|---|
| Tab / Shift+Tab | Move between controls and the table’s focused row |
| ↑ / ↓ | Focus the previous or next game row |
| Enter / Space | Open details for the focused game row |
| Escape | Close game details; if a file-path tooltip is showing, hide it first |
| Alt+← | Close game details and return to the originating row |
| Alt+→ | Reopen the last game details |
| Cmd+[ / Cmd+] (macOS) | Back / Forward aliases |

Row navigation follows the current filtered and sorted results, moves across pages automatically, and stops at the first or last result. It applies only when a game row itself has focus, so arrow keys and Space retain their normal behavior inside form controls.

Back/Forward remembers one game-details entry. Opening another game replaces it. Switching tabs or refreshing library data clears it. Closing details restores focus to the originating row when it is still available.

## Help and availability

**F1** opens shortcut help; **Escape** or its close button dismisses it and returns focus. Some keyboards require **Fn+F1**. Tab stays inside open dialogs.

Application commands pause during startup, scanning, organization, settings saves, native dialogs, and the restart-required screen. While game details or shortcut help is open, only its dismissal shortcuts apply. Holding an action key does not repeat the action; holding an arrow key can continue moving through rows. Shortcuts are fixed and cannot currently be customized.
