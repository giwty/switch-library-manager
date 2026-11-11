import {
    CheckUpdate,
    ExportXLSX,
    ConfirmOrganization as ConfirmNativeOrganization,
    GetOrganizationPreview,
    GetMissingDLC,
    GetMissingGames,
    GetMissingUpdates,
    GetGameDetails,
    IsKeysFileAvailable,
    LoadSettings,
    OrganizeLibrary,
    Restart,
    RescanLibrary,
    SaveSettings,
    SetContentIgnored,
    SetGameNameOverride,
    SelectFolder as SelectNativeFolder,
    ShowInFolder,
    ShowMessage as ShowNativeMessage,
    UpdateDB,
    UpdateLocalLibrary,
} from './wailsjs/go/app/App.js'
import {
    Environment,
    EventsOn,
    WindowSetDarkTheme,
    WindowSetLightTheme,
    WindowSetSystemDefaultTheme,
} from './wailsjs/runtime/runtime.js'
import { registerKeyboardShortcuts, TAB_SHORTCUTS, shortcutKeys, shortcutHelp } from './keyboard_shortcuts.js'
import { createRowNavigation } from './row_navigation.js'
import { exportTableXLSX } from './xlsx_export.js'
import { contentIgnoreAction, mergeIgnoreDraft } from './content_ignore.js'
import { createRowContextMenu } from './row_context_menu.js'
import { createGameNameEditor, gameNameAction } from './game_name_editor.js'
import { DATE_FORMAT_PRESETS, dateFormatError, formatDate, dateColumnOptions } from './date_format.js'

$(function () {

    const THEME_INHERIT = "inherit";
    const THEME_LIGHT = "light";
    const THEME_DARK = "dark";

    let state = {
        settings:{},
        settingsDraft: undefined,
        settingsFeedback: undefined,
        organizationPreview: undefined,
        libraryRefreshError: undefined,
        platform: undefined,
        keys:false
    };

    let currTable
    let currTableExportKind
    let xlsxExportPending = false
    const activeTables = new Set();
    let themeMediaQuery
    let themeMediaQueryHandler
    let restartRequired = false
    let settingsSavePending = false
    let scanInProgress = false
    let libraryDataGeneration = 0
    let organizationPending = false;
    let nativeDialogsPending = 0;
    let activeTabTarget = "#library";
    let keyboardFocusTarget = null;
    let tabRenderGeneration = 0;
    let currRowNavigation;
    let lastGameDetails = null;
    const pendingTabLoads = new Map();

    function withNativeDialog(action) {
        rowContextMenu.close(false);
        nativeDialogsPending++;
        return Promise.resolve().then(action).finally(() => nativeDialogsPending--);
    }
    const ConfirmOrganization = () => withNativeDialog(() => ConfirmNativeOrganization());
    const SelectFolder = () => withNativeDialog(() => SelectNativeFolder());
    const ShowMessage = (...args) => withNativeDialog(() => ShowNativeMessage(...args));

    function commandsBlocked() {
        return restartRequired || settingsSavePending || scanInProgress || organizationPending || xlsxExportPending ||
            nativeDialogsPending > 0 || wrapper.classList.contains("is-loading");
    }

    function openDialog() {
        if (document.getElementById("game-name-dialog").open) return "name";
        if (gameDetailsDialog.open) return "details";
        if (shortcutDialog.open) return "help";
        return null;
    }

    function createTable(element, options, exportKind) {
        currTable = new Tabulator(element, options);
        activeTables.add(currTable);
        currTableExportKind = exportKind;
        if (["games", "missing_games", "missing_updates", "missing_dlc"].includes(exportKind)) {
            const table = currTable;
            currRowNavigation = createRowNavigation(table, () => currTable === table);
            currRowNavigation.sync();
        }
    }

    function destroyTables() {
        rowContextMenu.close(false);
        currRowNavigation?.destroy();
        currRowNavigation = undefined;
        lastGameDetails = null;
        for (const table of activeTables) {
            table.destroy();
        }

        activeTables.clear();
        currTable = undefined;
        currTableExportKind = undefined;
    }

    function updatePaginationVisibility(table) {
        const paginator = table.element.querySelector(".tabulator-paginator");
        if (!paginator) {
            return;
        }

        const hidden = table.getPageMax() <= 1;
        paginator.hidden = hidden;
        paginator.setAttribute("aria-hidden", String(hidden));
    }

    function tableOptions(options) {
        const tableOptions = Object.assign({
            pagination: "local",
            paginationSize: state.settings.gui.page_size,
            footerElement: '<div class="slm-table-footer-actions"><button type="button" class="btn btn-link export-btn">Export to CSV</button><button type="button" class="btn btn-link export-xlsx-btn">Export to Excel (.xlsx)</button></div>'
        }, options || {});
        const renderComplete = tableOptions.renderComplete;

        tableOptions.renderComplete = function () {
            updatePaginationVisibility(this);
            if (this === currTable) currRowNavigation?.sync();
            if (renderComplete) {
                renderComplete.call(this);
            }
        };

        return tableOptions;
    }

    function exportDate() {
        const date = new Date();
        const year = date.getFullYear();
        const month = String(date.getMonth() + 1).padStart(2, "0");
        const day = String(date.getDate()).padStart(2, "0");
        return year + "-" + month + "-" + day;
    }

    function setThemeAttribute(theme) {
        document.documentElement.dataset.bsTheme = theme;
    }

    function applyTheme(theme) {
        switch (theme) {
        case THEME_LIGHT:
            WindowSetLightTheme();
            break;
        case THEME_DARK:
            WindowSetDarkTheme();
            break;
        default:
            WindowSetSystemDefaultTheme();
            break;
        }

        if (themeMediaQuery && themeMediaQueryHandler) {
            if (themeMediaQuery.removeEventListener) {
                themeMediaQuery.removeEventListener("change", themeMediaQueryHandler);
            } else if (themeMediaQuery.removeListener) {
                themeMediaQuery.removeListener(themeMediaQueryHandler);
            }
        }
        themeMediaQuery = undefined;
        themeMediaQueryHandler = undefined;

        if (theme !== THEME_LIGHT && theme !== THEME_DARK) {
            if (!window.matchMedia) {
                setThemeAttribute(THEME_LIGHT);
                return;
            }
            themeMediaQuery = window.matchMedia("(prefers-color-scheme: dark)");
            themeMediaQueryHandler = event => setThemeAttribute(event.matches ? THEME_DARK : THEME_LIGHT);
            setThemeAttribute(themeMediaQuery.matches ? THEME_DARK : THEME_LIGHT);
            if (themeMediaQuery.addEventListener) {
                themeMediaQuery.addEventListener("change", themeMediaQueryHandler);
            } else if (themeMediaQuery.addListener) {
                themeMediaQuery.addListener(themeMediaQueryHandler);
            }
            return;
        }

        setThemeAttribute(theme);
    }

    applyTheme(THEME_INHERIT);

    const wrapper = document.querySelector('.wrapper');
    const tabButtons = document.getElementById('tab_btns');
    const tabGroup = document.getElementById('tab-group');
    const progressBar = $('.progress-bar');
    const gameDetailsDialog = document.getElementById("game-details-dialog");
    const gameDetailsContent = document.getElementById("game-details-content");
    const gameDetailsFileError = document.getElementById("game-details-file-error");
    const gameDetailsPathTooltip = document.getElementById("game-details-path-tooltip");
    const gameDetailsPathActionTemplate = document.getElementById("game-details-path-action-template");
    const shortcutDialog = document.getElementById("keyboard-shortcuts-dialog");
    const shortcutHelpBody = document.getElementById("keyboard-shortcuts-body");
    let shortcutReturnFocus = null;
    let gameDetailsRequest = 0;
    let gameDetailsReturnFocus = null;
    let gameDetailsFileRequest = 0;
    let gameDetailsPathTooltipTarget = null;
    let gameDetailsPathTooltipHideTimer;

    const rowContextMenu = createRowContextMenu({
        document, window,
        isBlocked: () => commandsBlocked() || Boolean(openDialog()),
        onAction: (action, row) => {
            if (action.type === "game-name") void gameNameEditor.open(action.titleID, row);
            else void saveContentIgnore(action, row);
        }
    });

    function openRowMenu(event, row) {
        const action = contentIgnoreAction(row.getData(), currTableExportKind, state.settings.missing_content);
        const edit = gameNameAction(row.getData(), currTableExportKind);
        rowContextMenu.open(event, row, [edit, action].filter(Boolean));
    }

    const gameNameEditor = createGameNameEditor({
        document, load: GetGameDetails, save: SetGameNameOverride,
        onBusyChange: pending => { settingsSavePending = pending; },
        restoreFocus(origin, titleID) {
            if (gameDetailsDialog.open) gameDetailsContent.querySelector("[data-edit-game-name]")?.focus();
            else if (origin?.isConnected) origin.focus({preventScroll: true});
            else {
                const row = currTable?.getRows("display").find(row => row.getData().titleId === titleID);
                if (row) row.getElement().focus({preventScroll: true});
                else currRowNavigation?.focusFirst();
            }
        },
        async onSaved(result) {
            const generation = ++libraryDataGeneration;
            gameDetailsRequest++;
            lastGameDetails = null;
            state.settings.title_name_overrides = {...result.title_name_overrides};
            if (state.settingsDraft) state.settingsDraft.title_name_overrides = {...result.title_name_overrides};
            state.library = result.library;
            state.updates = state.dlc = state.missingGames = state.organizationPreview = undefined;
            if (gameDetailsDialog.open) renderGameDetails(result.details);
            const target = activeTabTarget;
            const table = currTable;
            const page = table?.getPage();
            let rows = result.library.library_data;
            const fetches = {"#updates": ["updates", GetMissingUpdates], "#dlc": ["dlc", GetMissingDLC], "#missing": ["missingGames", GetMissingGames]};
            if (fetches[target]) {
                const [key, fetch] = fetches[target];
                const data = await fetch();
                if (generation !== libraryDataGeneration) return;
                state[key] = data;
                rows = target === "#dlc" ? data.flatMap(game => game.missing_dlc.map(dlc => ({game: game.Attributes, dlc}))) : data;
            }
            if (generation !== libraryDataGeneration || currTable !== table || activeTabTarget !== target) return;
            if (table) {
                await table.replaceData(rows);
                await table.setPage(Math.max(1, Math.min(page, table.getPageMax())));
            }
        }
    });

    async function saveContentIgnore(action, rowElement) {
        if (commandsBlocked() || openDialog()) return;
        rowContextMenu.close(false);
        settingsSavePending = true;
        const table = currTable;
        const target = activeTabTarget;
        const page = table.getPage();
        const sorters = table.getSorters().map(({field, dir}) => ({column: field, dir}));
        const filters = table.getHeaderFilters();
        const rowIndex = table.getRows("display").findIndex(row => row.getElement() === rowElement);
        let saved = false;
        try {
            const missing = await SetContentIgnored(action.kind, action.titleID, action.ignored);
            saved = true;
            state.settings.missing_content = missing;
            mergeIgnoreDraft(state.settingsDraft, action);
            const generation = ++libraryDataGeneration;
            state.updates = undefined;
            state.dlc = undefined;
            lastGameDetails = null;
            if (target === "#updates" || target === "#dlc") {
                const data = await (target === "#updates" ? GetMissingUpdates() : GetMissingDLC());
                if (generation !== libraryDataGeneration) return;
                state[target === "#updates" ? "updates" : "dlc"] = data;
            }
            if (activeTabTarget !== target || currTable !== table) return;
            // Library rows remain in place. Missing-content rows and counts need
            // a new render, retaining the user's current view where possible.
            if (target !== "#library") {
                loadTab(target);
                if (currTable) {
                    currTable.setSort(sorters);
                    filters.forEach(({field, value}) => currTable.setHeaderFilterValue(field, value));
                    await currTable.setPage(Math.max(1, Math.min(page, currTable.getPageMax())));
                    const rows = currTable.getRows("display");
                    const next = rows[Math.min(Math.max(rowIndex, 0), rows.length - 1)];
                    if (next) next.getElement().focus();
                    else document.querySelector(target).focus();
                } else document.querySelector(target).focus();
            } else if (rowElement?.isConnected) rowElement.focus({preventScroll: true});
        } catch (error) {
            const title = saved ? "Ignore preference saved; refresh failed" : "Could not save ignore preference";
            const message = saved ? "Your change was saved. Select this tab again to retry loading it."
                : "Your ignore preference was not changed. Please try again.";
            if (saved && activeTabTarget === target && target !== "#library") {
                destroyTables();
                const notice = document.createElement("div");
                notice.className = "alert alert-danger";
                notice.setAttribute("role", "alert");
                notice.textContent = message;
                document.querySelector(target).replaceChildren(notice);
                document.querySelector(target).focus();
            }
            await ShowMessage("error", title, message, settingsErrorMessage(error))
                .catch(failure => console.error("Could not show ignore error:", failure));
            if (!saved && currTable === table && rowElement?.isConnected) rowElement.focus({preventScroll: true});
        } finally {
            settingsSavePending = false;
        }
    }

    function hideGameDetailsPathTooltip() {
        if (gameDetailsPathTooltipHideTimer !== undefined) {
            window.clearTimeout(gameDetailsPathTooltipHideTimer);
            gameDetailsPathTooltipHideTimer = undefined;
        }
        gameDetailsPathTooltip.hidden = true;
        gameDetailsPathTooltipTarget = null;
    }

    function positionGameDetailsPathTooltip(button) {
        const buttonRect = button.getBoundingClientRect();
        const tooltipRect = gameDetailsPathTooltip.getBoundingClientRect();
        const margin = 8;
        const maxLeft = Math.max(margin, window.innerWidth - tooltipRect.width - margin);
        const maxTop = Math.max(margin, window.innerHeight - tooltipRect.height - margin);
        const left = Math.max(margin, Math.min(buttonRect.left, maxLeft));
        let top = buttonRect.bottom;
        if (top + tooltipRect.height > window.innerHeight - margin) {
            top = buttonRect.top - tooltipRect.height;
        }
        top = Math.max(margin, Math.min(top, maxTop));
        gameDetailsPathTooltip.style.left = left + "px";
        gameDetailsPathTooltip.style.top = top + "px";
    }

    function showGameDetailsPathTooltip(button) {
        const path = button.dataset.filePath;
        if (!path || !button.isConnected || !gameDetailsDialog.open) {
            return;
        }
        if (gameDetailsPathTooltipHideTimer !== undefined) {
            window.clearTimeout(gameDetailsPathTooltipHideTimer);
            gameDetailsPathTooltipHideTimer = undefined;
        }
        gameDetailsPathTooltipTarget = button;
        gameDetailsPathTooltip.textContent = path;
        gameDetailsPathTooltip.hidden = false;
        positionGameDetailsPathTooltip(button);
    }

    function scheduleGameDetailsPathTooltipHide() {
        if (gameDetailsPathTooltipHideTimer !== undefined) {
            window.clearTimeout(gameDetailsPathTooltipHideTimer);
        }
        gameDetailsPathTooltipHideTimer = window.setTimeout(() => {
            gameDetailsPathTooltipHideTimer = undefined;
            const button = gameDetailsPathTooltipTarget;
            if (button && (button.matches(":hover") || document.activeElement === button || gameDetailsPathTooltip.matches(":hover"))) {
                return;
            }
            hideGameDetailsPathTooltip();
        }, 120);
    }

    function clearGameDetailsFileError() {
        gameDetailsFileError.hidden = true;
        gameDetailsFileError.textContent = "";
    }

    function renderGameDetailsFileActions() {
        const prototype = gameDetailsPathActionTemplate.content.firstElementChild;
        gameDetailsContent.querySelectorAll(".game-details-file-action-placeholder").forEach(placeholder => {
            const path = placeholder.dataset.filePath;
            if (!path || !prototype) {
                placeholder.remove();
                return;
            }
            const button = prototype.cloneNode(true);
            button.dataset.filePath = path;
            placeholder.replaceWith(button);
        });
    }

    function openGameDetails(titleID, returnFocusElement) {
        if (!titleID || commandsBlocked() || openDialog()) {
            return;
        }

        rowContextMenu.close(false);

        const request = ++gameDetailsRequest;
        gameDetailsFileRequest++;
        hideGameDetailsPathTooltip();
        clearGameDetailsFileError();
        gameDetailsReturnFocus = returnFocusElement || document.activeElement;
        lastGameDetails = {titleID, returnFocusElement: gameDetailsReturnFocus};
        const loading = document.createElement("div");
        loading.className = "game-details-message";
        loading.setAttribute("role", "status");
        loading.textContent = "Loading game details…";
        gameDetailsContent.replaceChildren(loading);
        gameDetailsDialog.setAttribute("aria-label", "Loading game details");
        gameDetailsDialog.showModal();

        GetGameDetails(titleID).then(details => {
            if (request !== gameDetailsRequest || !gameDetailsDialog.open) {
                return;
            }
            renderGameDetails(details);
        }).catch(error => {
            if (request !== gameDetailsRequest || !gameDetailsDialog.open) {
                return;
            }
            const failure = document.createElement("div");
            failure.className = "alert alert-danger game-details-message";
            failure.setAttribute("role", "alert");
            failure.textContent = error && error.message ? error.message : "Unable to load game details.";
            gameDetailsDialog.setAttribute("aria-label", "Unable to load game details");
            gameDetailsContent.replaceChildren(failure);
        });
    }

    function renderGameDetails(details) {
        gameDetailsDialog.setAttribute("aria-label", details.name ? "Game details for " + details.name : "Game details");
        gameDetailsContent.innerHTML = $("#gameDetailsTemplate").render(details, {
            formatDate: value => formatDate(value, state.settings.gui.date_format) || "Unknown"
        });
        renderGameDetailsFileActions();
    }

    gameDetailsContent.addEventListener("click", event => {
        const button = event.target.closest("[data-edit-game-name]");
        if (button && !commandsBlocked() && openDialog() === "details") {
            void gameNameEditor.open(button.dataset.editGameName, button);
        }
    });

    gameDetailsDialog.addEventListener("pointerover", event => {
        const button = event.target.closest && event.target.closest(".game-details-path-info");
        if (button) {
            showGameDetailsPathTooltip(button);
        } else if (event.target === gameDetailsPathTooltip || gameDetailsPathTooltip.contains(event.target)) {
            if (gameDetailsPathTooltipHideTimer !== undefined) {
                window.clearTimeout(gameDetailsPathTooltipHideTimer);
                gameDetailsPathTooltipHideTimer = undefined;
            }
        }
    });

    gameDetailsDialog.addEventListener("pointerout", event => {
        const button = event.target.closest && event.target.closest(".game-details-path-info");
        if (button === gameDetailsPathTooltipTarget || event.target === gameDetailsPathTooltip || gameDetailsPathTooltip.contains(event.target)) {
            scheduleGameDetailsPathTooltipHide();
        }
    });

    gameDetailsDialog.addEventListener("focusin", event => {
        const button = event.target.closest && event.target.closest(".game-details-path-info");
        if (button) {
            showGameDetailsPathTooltip(button);
        }
    });

    gameDetailsDialog.addEventListener("focusout", event => {
        const button = event.target.closest && event.target.closest(".game-details-path-info");
        if (button === gameDetailsPathTooltipTarget) {
            scheduleGameDetailsPathTooltipHide();
        }
    });

    gameDetailsDialog.addEventListener("scroll", hideGameDetailsPathTooltip, true);
    window.addEventListener("resize", hideGameDetailsPathTooltip);

    gameDetailsDialog.addEventListener("cancel", event => {
        if (!gameDetailsPathTooltip.hidden) {
            event.preventDefault();
            hideGameDetailsPathTooltip();
        }
    });

    gameDetailsDialog.addEventListener("click", async event => {
        const button = event.target.closest && event.target.closest(".game-details-path-info");
        if (!button || !button.dataset.filePath) {
            return;
        }

        clearGameDetailsFileError();
        const request = ++gameDetailsFileRequest;
        try {
            await ShowInFolder(button.dataset.filePath);
        } catch (error) {
            if (request !== gameDetailsFileRequest || !gameDetailsDialog.open || !gameDetailsContent.contains(button)) {
                return;
            }
            const detail = error && error.message ? error.message : String(error);
            gameDetailsFileError.textContent = "Unable to show the file in its folder: " + detail;
            gameDetailsFileError.hidden = false;
        }
    });

    gameDetailsDialog.addEventListener("close", () => {
        // A close event can arrive after Forward has already reopened the dialog.
        if (gameDetailsDialog.open) return;
        gameDetailsRequest++;
        gameDetailsFileRequest++;
        hideGameDetailsPathTooltip();
        clearGameDetailsFileError();
        const active = document.activeElement;
        if (!commandsBlocked() && (active === document.body || active === gameDetailsReturnFocus || gameDetailsDialog.contains(active))) {
            if (gameDetailsReturnFocus?.isConnected) gameDetailsReturnFocus.focus();
            else currRowNavigation?.focusFirst();
        }
        gameDetailsReturnFocus = null;
    });

    function detailRowTitleID(data, kind) {
        if (kind === "games") {
            return data.titleId;
        }
        if (kind === "updates") {
            return data.Attributes && data.Attributes.id;
        }
        if (kind === "dlc") {
            return data.game && data.game.id;
        }
        return "";
    }

    function detailRowLabel(data, kind) {
        if (kind === "dlc") {
            return data.game && data.game.name;
        }
        if (kind === "updates") {
            return data.Attributes && data.Attributes.name;
        }
        return data.name;
    }

    function gameDetailsTableOptions(options, kind, withContextMenu = true) {
        const optionsWithDetails = Object.assign({}, options || {});
        const rowFormatter = optionsWithDetails.rowFormatter;
        optionsWithDetails.rowFormatter = function (row) {
            const data = row.getData();
            const element = row.getElement();
            const label = detailRowLabel(data, kind) || "game";
            element.dataset.gameDetailsRow = "true";
            element.setAttribute("tabindex", "-1");
            element.setAttribute("aria-haspopup", "dialog");
            element.setAttribute("aria-label", "Open details for " + label);
            if (withContextMenu) element.setAttribute("aria-keyshortcuts", "Enter Space Shift+F10 ContextMenu");
            if (rowFormatter) {
                rowFormatter.call(this, row);
            }
        };
        optionsWithDetails.rowClick = function (event, row) {
            openGameDetails(detailRowTitleID(row.getData(), kind), row.getElement());
        };
        if (withContextMenu) optionsWithDetails.rowContext = openRowMenu;
        return tableOptions(optionsWithDetails);
    }

    document.addEventListener("focusin", event => {
        if (event.target.matches?.(".tabulator-row[data-game-details-row='true']")) {
            currRowNavigation?.remember(event.target);
        } else {
            currRowNavigation?.releaseFocus(event.target);
        }
    });

    function renderShortcutHelp() {
        const rows = shortcutHelp(state.platform === "darwin").map(shortcut => {
            const row = document.createElement("tr");
            const keys = document.createElement("td");
            const label = document.createElement("td");
            const kbd = document.createElement("kbd");
            kbd.textContent = shortcut.keys;
            keys.append(kbd);
            label.textContent = shortcut.label;
            row.append(keys, label);
            return row;
        });
        shortcutHelpBody.replaceChildren(...rows);
    }

    function focusDialogControl(dialog, reverse) {
        const controls = Array.from(dialog.querySelectorAll("button, [href], input, select, textarea, [tabindex]"))
            .filter(element => !element.disabled && !element.matches?.(":disabled") && element.tabIndex >= 0 && element.getClientRects().length > 0);
        const index = controls.indexOf(document.activeElement);
        const next = index < 0 ? (reverse ? controls.length - 1 : 0)
            : (index + (reverse ? -1 : 1) + controls.length) % controls.length;
        (controls[next] || dialog).focus();
    }

    function showShortcutHelp() {
        if (commandsBlocked() || openDialog()) return;
        rowContextMenu.close();
        shortcutReturnFocus = document.activeElement;
        renderShortcutHelp();
        shortcutDialog.showModal();
    }

    shortcutDialog.addEventListener("close", () => {
        if (shortcutDialog.open) return;
        const active = document.activeElement;
        if (shortcutReturnFocus?.isConnected && !commandsBlocked() &&
            (active === document.body || active === shortcutReturnFocus || shortcutDialog.contains(active))) shortcutReturnFocus.focus();
        shortcutReturnFocus = null;
    });

    function scheduleTabFocus(target) {
        if (keyboardFocusTarget !== target) return;
        const generation = tabRenderGeneration;
        window.requestAnimationFrame(() => {
            if (generation !== tabRenderGeneration || activeTabTarget !== target ||
                keyboardFocusTarget !== target || commandsBlocked() || openDialog()) return;
            keyboardFocusTarget = null;
            if (!currRowNavigation?.focusFirst()) document.querySelector(target).focus();
        });
    }

    function switchTab(target, keyboard = false) {
        if (commandsBlocked() || openDialog() ||
            (target === "#missing" && state.settings.gui.hide_missing_games)) return;
        if (activeTabTarget === "#settings") captureSettingsDraft();
        keyboardFocusTarget = keyboard ? target : null;
        loadTab(target);
    }

    function requestTabData(target, cacheKey, fetchData) {
        const generation = libraryDataGeneration;
        if (pendingTabLoads.get(target)?.generation === generation) return;
        const request = {generation};
        pendingTabLoads.set(target, request);
        $(target).html('<div class="game-details-message" role="status">Loading…</div>');
        Promise.resolve().then(fetchData).then(data => {
            if (generation !== libraryDataGeneration) return;
            state[cacheKey] = data;
            if (!restartRequired && activeTabTarget === target) loadTab(target);
        }).catch(error => {
            if (generation !== libraryDataGeneration) return;
            if (activeTabTarget === target) {
                const failure = document.createElement("div");
                failure.className = "alert alert-danger";
                failure.setAttribute("role", "alert");
                failure.textContent = "Unable to load this tab. Select the tab again to retry. " + settingsErrorMessage(error);
                document.querySelector(target).replaceChildren(failure);
                scheduleTabFocus(target);
            } else console.error("Could not load " + target + ":", error);
        }).finally(() => {
            if (pendingTabLoads.get(target) === request) pendingTabLoads.delete(target);
        });
    }

    function setLoading(loading) {
        loading = loading || restartRequired;
        if (loading) rowContextMenu.close(false);
        wrapper.classList.toggle('is-loading', loading);
        wrapper.setAttribute('aria-busy', String(loading));

        [tabButtons, tabGroup].forEach(element => {
            element.setAttribute('aria-hidden', String(loading));
            if (loading) {
                element.setAttribute('inert', '');
            } else {
                element.removeAttribute('inert');
            }
        });
    }

    function resetProgress() {
        progressBar.attr('aria-valuenow', 0);
        progressBar.attr('style', 'width: 0%');
        progressBar.text('0%');
    }

    function showProgress(message) {
        resetProgress();
        $('.progress-type').text(message);
        setLoading(true);
    }

    function hideProgress() {
        setLoading(false);
    }

    function lockForRestart(reasons = []) {
        restartRequired = true;
        wrapper.classList.add('restart-required');
        $('.progress-type').text('Restart required');
        $('.progress-msg').text('Settings were saved. Restart the application to apply your changes.' +
            (reasons.length ? ' Changed: ' + reasons.join(', ') + '.' : ''));
        $('.progress-container').attr('aria-label', 'Application restart required');
        const restartButton = document.getElementById('restart-button');
        restartButton.hidden = false;
        setLoading(true);
        restartButton.focus();
    }

    function updateShortcutMetadata(root = document) {
        const isMacOS = state.platform === "darwin";
        root.querySelectorAll(".rescan-action").forEach(button => {
            button.setAttribute("aria-keyshortcuts", shortcutKeys(button.dataset.hard === "true" ? "hard-rescan" : "rescan", isMacOS, true));
        });
        root.querySelectorAll(".library-organize-action").forEach(button => {
            button.setAttribute("aria-keyshortcuts", shortcutKeys("organize", isMacOS, true));
        });
        TAB_SHORTCUTS.forEach((tab, index) => {
            root.querySelector("#tab_btns a[href='" + tab.target + "']")?.setAttribute("aria-keyshortcuts", shortcutKeys("tab-" + (index + 1), isMacOS, true));
        });
    }

    //handle tabs action
    $('.tabgroup > div').hide();
    // loadTab($('.tabgroup > div:first-of-type'));

    let showError = function (detail, target = "#library") {
        if (restartRequired) {
            return;
        }
        ShowMessage("error", "Error", "An unexpected error occurred", detail || "")
            .catch(error => console.error(error));
        hideProgress();
        loadTab(target);
    };

        EventsOn("updateProgress", function (message) {
            if (restartRequired) {
                return;
            }
            setLoading(true);
            let count = message.curr;
            let total = message.total;
            $('.progress-msg').text(message.message + " ...");
            if (count >= 0 && total > 0){
                let pcg = Math.min(100, Math.max(0, Math.floor(count / total * 100)));
                $('.progress-bar').attr('aria-valuenow', pcg);
                $('.progress-bar').attr('style', 'width:' + Number(pcg) + '%');
                $('.progress-bar').text(pcg + "%");
            }
        });

        EventsOn("error", showError);
        Environment()
            .then(environment => {
                state.platform = environment.platform;
                updateShortcutMetadata();
                if (shortcutDialog.open) renderShortcutHelp();
            })
            .catch(error => console.error("Could not determine the current platform:", error));
        LoadSettings().then(function (message) {
            state.settings = message;
            state.settingsDraft = undefined;
            state.settingsFeedback = undefined;
            applyTheme(state.settings.gui && state.settings.gui.theme);

            document.getElementById("tab_btns").classList.toggle("hide_missing_games", state.settings.gui.hide_missing_games);
            return UpdateDB();
        }).then(function () {
            return scanLocalFolder(Boolean(state.settings.scan.rescan_on_startup), true);
        }).catch(error => showError(error.message));

        IsKeysFileAvailable().then(function (message) {
            state.keys = message
        }).catch(error => showError(error.message));

        CheckUpdate().then(function (message) {
            if (!message){
                return
            }
            return ShowMessage("info", "New update available", "There is a new update available, please download from Github", "");
        }).catch(error => showError(error.message));

        showProgress("Downloading latest Switch titles/versions ...");

        let openFolderPicker = function (mode) {
            if (commandsBlocked() || openDialog()) return;
            SelectFolder()
                .then(path => updateFolder(mode, path))
                .catch(error => showError(error.message));
        };

        let scanLocalFolder = function(mode, startup, returnTab = "#library"){
            if (!state.settings.paths.library_folder){
                loadTab("#library");
                hideProgress();
                return Promise.resolve();
            }
            const generation = ++libraryDataGeneration;
            showProgress("Scanning local library...");

            const scan = startup ? UpdateLocalLibrary(Boolean(mode)) : RescanLibrary(Boolean(mode));
            return scan.then(result => {
                if (generation !== libraryDataGeneration) {
                    return result;
                }
                state.updates = undefined;
                state.dlc = undefined;
                state.missingGames = undefined;
                state.organizationPreview = undefined;
                state.library = result;
                state.libraryRefreshError = undefined;
                loadTab(returnTab);
                hideProgress();
                return result;
            });
        };

        let updateFolder = function (mode,path) {
            if (!path || commandsBlocked()) {
                return
            }
            const candidate = cloneSettings(state.settings);
            if (mode === "add"){
                if (!candidate.paths.scan_folders.includes(path)){
                    candidate.paths.scan_folders.push(path);
                }else{
                    return;
                }

            }else{
                if (candidate.paths.library_folder === path) return;
                candidate.paths.library_folder = path;
            }
            return saveSettings(candidate, activeTabTarget);
        };


        function cloneSettings(value) {
            return JSON.parse(JSON.stringify(value));
        }

        async function saveSettings(value, returnTab = "#settings") {
            settingsSavePending = true;
            try {
                const result = await SaveSettings(value);
                // Ignore responses from requests made with the previous settings.
                libraryDataGeneration++;
                gameDetailsRequest++;
                lastGameDetails = null;
                state.settings = result.settings;
                state.settingsDraft = cloneSettings(result.settings);
                state.settingsFeedback = undefined;
                if (result.restart_required) {
                    tabGroup.scrollTop = 0;
                    lockForRestart(result.restart_reasons);
                    return;
                }
                if (result.library) {
                    state.library = result.library;
                    state.libraryRefreshError = undefined;
                }
                state.updates = undefined;
                state.dlc = undefined;
                state.missingGames = undefined;
                state.organizationPreview = undefined;
                applyTheme(state.settings.gui.theme);
                document.getElementById("tab_btns").classList.toggle("hide_missing_games", state.settings.gui.hide_missing_games);
                loadTab(returnTab);
                tabGroup.scrollTop = 0;
                try {
                    await ShowMessage("info", "Settings saved", "Your settings have been saved.",
                        "Startup preferences apply the next time you launch SLM.");
                } catch (error) {
                    showSettingsFeedback("warning", "Your settings were saved, but the confirmation could not be displayed. " + settingsErrorMessage(error));
                }
                document.querySelector(returnTab)?.focus({preventScroll: true});
            } finally {
                settingsSavePending = false;
            }
        }

        function settingsErrorMessage(error) {
            return error && error.message ? error.message : String(error);
        }

        function renderSettingsTab() {
            if (!state.settingsDraft) {
                state.settingsDraft = cloneSettings(state.settings);
            }
            let templateData = Object.assign({}, state.settingsDraft, {
                feedback: state.settingsFeedback,
                dateFormatPreset: DATE_FORMAT_PRESETS.includes(state.settingsDraft.gui.date_format) ? state.settingsDraft.gui.date_format : "custom"
            });
            let settingsHtml = $("#settingsTemplate").render(templateData);
            $("#settings").html(settingsHtml);
            updateDateFormatControls();
        }

        function updateDateFormatControls() {
            const preset = document.getElementById("settings-gui-date-format");
            if (!preset) return true;
            const input = document.getElementById("settings-gui-date-format-custom");
            const custom = preset.value === "custom";
            document.getElementById("settings-date-format-custom-row").hidden = !custom;
            input.disabled = !custom;
            input.required = custom;
            if (!custom) input.value = preset.value;
            const pattern = custom ? input.value : preset.value;
            const error = custom && !pattern.trim() ? "Enter a custom date format." : dateFormatError(pattern);
            input.setCustomValidity(error);
            input.classList.toggle("is-invalid", Boolean(error));
            input.setAttribute("aria-invalid", String(Boolean(error)));
            document.getElementById("settings-date-format-error").textContent = error;
            document.getElementById("settings-date-format-example").textContent = error ? "Example unavailable" :
                "Example: " + formatDate("2026-11-23", pattern);
            return !error;
        }

        function showSettingsFeedback(type, message) {
            state.settingsFeedback = {type: type, message: message};
            renderSettingsTab();
            tabGroup.scrollTop = 0;
        }

        function listValues(form, name) {
            return Array.from(form.querySelectorAll('[data-settings-list="' + name + '"]'))
                .map(input => input.value.trim())
                .filter(value => value.length > 0);
        }

        function collectSettings(form) {
            return {
                schema_version: state.settings.schema_version,
                title_name_overrides: {...state.settings.title_name_overrides},
                gui: {
                    enabled: form.elements.gui_enabled.checked,
                    page_size: Number(form.elements.gui_page_size.value),
                    hide_missing_games: form.elements.gui_hide_missing_games.checked,
                    hide_demo_games: form.elements.gui_hide_demo_games.checked,
                    hide_version_string: form.elements.gui_hide_version_string.checked,
                    remember_window_state: form.elements.gui_remember_window_state.checked,
                    theme: form.elements.gui_theme.value,
                    date_format: form.elements.gui_date_format_preset.value === "custom" ?
                        form.elements.gui_date_format_custom.value : form.elements.gui_date_format_preset.value
                },
                paths: {
                    library_folder: form.elements.paths_library_folder.value.trim(),
                    scan_folders: listValues(form, "scan_folders"),
                    prod_keys: form.elements.paths_prod_keys.value.trim()
                },
                scan: {
                    recursive: form.elements.scan_recursive.checked,
                    rescan_on_startup: form.elements.scan_rescan_on_startup.checked,
                    ignore_file_types: listValues(form, "ignore_file_types"),
                    ignore_unsupported_file_types: form.elements.scan_ignore_unsupported_file_types.checked
                },
                organization: {
                    create_folder_per_game: form.elements.organization_create_folder_per_game.checked,
                    move_scan_files_to_library: form.elements.organization_move_scan_files_to_library.checked,
                    dlc_folder: form.elements.organization_dlc_folder.value,
                    updates_folder: form.elements.organization_updates_folder.value,
                    rename_files: form.elements.organization_rename_files.checked,
                    delete_empty_folders: form.elements.organization_delete_empty_folders.checked,
                    delete_old_update_files: form.elements.organization_delete_old_update_files.checked,
                    folder_name_template: form.elements.organization_folder_name_template.value,
                    allow_unicode_file_names: form.elements.organization_allow_unicode_file_names.checked,
                    file_name_template: form.elements.organization_file_name_template.value,
                    include_base_type: form.elements.organization_include_base_type.checked,
                    process_when_missing_base_game: form.elements.organization_process_when_missing_base_game.checked
                },
                missing_content: {
                    check_for_updates: form.elements.missing_check_for_updates.checked,
                    check_for_dlc: form.elements.missing_check_for_dlc.checked,
                    ignore_dlc_updates: form.elements.missing_ignore_dlc_updates.checked,
                    ignore_dlc_title_ids: listValues(form, "ignore_dlc_title_ids"),
                    ignore_update_title_ids: listValues(form, "ignore_update_title_ids")
                },
                data_sources: {
                    titles_url: form.elements.data_titles_url.value.trim(),
                    versions_url: form.elements.data_versions_url.value.trim()
                },
                logging: {
                    debug: form.elements.logging_debug.checked
                }
            };
        }

        function captureSettingsDraft() {
            const form = document.getElementById("settings-form");
            if (form) {
                state.settingsDraft = collectSettings(form);
            }
        }

        function appendSettingsListRow(name, value) {
            const container = document.querySelector('[data-settings-list-container="' + name + '"]');
            if (!container) {
                return;
            }
            const row = document.createElement("div");
            row.className = "settings-list-row input-group mb-2";
            const input = document.createElement("input");
            input.type = "text";
            input.className = "form-control";
            input.value = value || "";
            input.dataset.settingsList = name;
            const remove = document.createElement("button");
            remove.type = "button";
            remove.className = "btn btn-outline-danger settings-list-remove";
            remove.textContent = "Remove";
            row.append(input, remove);
            container.append(row);
            input.focus();
        }

        function settingsFolderSelected(target, path) {
            if (target === "library_folder") {
                const input = document.querySelector('[name="paths_library_folder"]');
                if (input) {
                    input.value = path;
                }
                captureSettingsDraft();
                return;
            }
            const existing = Array.from(document.querySelectorAll('[data-settings-list="scan_folders"]'))
                .some(input => input.value === path);
            if (!existing) {
                appendSettingsListRow("scan_folders", path);
            }
            captureSettingsDraft();
        }

        function loadTab(target) {
            activeTabTarget = target;
            tabRenderGeneration++;
            if (keyboardFocusTarget !== target) keyboardFocusTarget = null;
            hideCurrentTab();

            $("#tab_btns a[href='" + target + "']").addClass('active').attr('aria-selected', 'true');
            $(target).show();

            if (target === "#settings") {
                renderSettingsTab();
            } else if (target === "#organize") {
                const folder = state.settings.paths.library_folder;
                const organization = state.settings.organization;
                if (!folder) {
                    $(target).html($(target + "Template").render({folder: folder, organization: organization}));
                    updateShortcutMetadata(document.querySelector(target));
                    scheduleTabFocus(target);
                    return;
                }
                if (!state.organizationPreview || state.organizationPreview.root_folder !== folder) {
                    $(target).html($(target + "Template").render({folder: folder, organization: organization, previewLoading: true}));
                    updateShortcutMetadata(document.querySelector(target));
                    requestTabData(target, "organizationPreview", GetOrganizationPreview);
                    return;
                }
                let html = $(target + "Template").render({folder: folder, organization: organization, preview: state.organizationPreview})
                $(target).html(html);
                updateShortcutMetadata(document.querySelector(target));
            } else if (target === "#updates") {
                if (state.settings.paths.library_folder && !state.library){
                    return
                }
                if (state.library && !state.updates){
                    requestTabData(target, "updates", GetMissingUpdates);
                    return
                }
                let html = $(target + "Template").render({folder: state.settings.paths.library_folder,updates:state.updates})
                $(target).html(html);
                if (state.updates && state.updates.length) {
                    createTable("#updates-table", gameDetailsTableOptions({
                        layout:"fitDataStretch",
                        initialSort:[
                            {column:"latest_update_date", dir:"desc"}, //sort by this first
                        ],
                        data: state.updates,
                        columns: [
                            {formatter:"rownum",download:false},
                            {field: "Attributes.bannerUrl",download:false,formatter:"image", headerSort:false,formatterParams:{height:"60px", width:"60px"}},
                            {title: "Title", field: "Attributes.name", headerFilter:"input",formatter:"textarea",width:350},
                            {title: "Type", field: "Meta.type", headerFilter:"input"},
                            {title: "Title ID", headerSort:false, field: "Attributes.id", headerFilter:"input", hozAlign: "right", sorter: "number"},
                            {title: "Local update", headerSort:false, field: "local_update", hozAlign: "right", sorter: "number", formatter: function (cell) {
                                const displayVersion = state.settings.gui.hide_version_string ? "" : cell.getRow().getData().local_display_version;
                                const value = String(cell.getValue()) + (displayVersion ? " (" + displayVersion + ")" : "");
                                const content = document.createElement("span");
                                content.textContent = value;
                                return content;
                            }},
                            {title: "Available update", headerSort:false, field: "latest_update", hozAlign: "right"},
                            {title: "Update date", headerSort:true, field: "latest_update_date", ...dateColumnOptions(state.settings.gui?.date_format)}
                        ],
                    }, "updates"), "missing_updates");
                }
            } else if (target === "#dlc") {
                if (state.settings.paths.library_folder && !state.library){
                    return
                }
                if (state.library && !state.dlc){
                    requestTabData(target, "dlc", GetMissingDLC);
                    return
                }
                const missingDlcRows = (state.dlc || []).flatMap(game =>
                    (game.missing_dlc || []).map(dlc => ({game: game.Attributes, dlc}))
                );
                let html = $(target + "Template").render({
                    folder: state.settings.paths.library_folder,
                    dlc: state.dlc,
                    missingDlcCount: missingDlcRows.length
                });
                $(target).html(html);
                if (missingDlcRows.length) {
                    createTable("#dlc-table", gameDetailsTableOptions({
                        layout:"fitDataStretch",
                        sortOrderReverse:true,
                        initialSort:[
                            {column:"dlc.release_date", dir:"desc"},
                            {column:"game.name", dir:"asc"},
                            {column:"dlc.name", dir:"asc"},
                            {column:"dlc.id", dir:"asc"},
                            {column:"game.id", dir:"asc"},
                        ],
                        data: missingDlcRows,
                        columns: [
                            {formatter:"rownum",download:false},
                            {title: "Missing DLC", field: "dlc.name", headerFilter:"input", formatter:"textarea"},
                            {title: "DLC ID", field: "dlc.id", headerFilter:"input"},
                            {title: "Release date", field: "dlc.release_date", ...dateColumnOptions(state.settings.gui?.date_format)},
                            {title: "Title", field: "game.name", headerFilter:"input", formatter:"textarea", width:350},
                            {title: "Title ID", field: "game.id", headerFilter:"input"}
                        ],
                    }, "dlc"), "missing_dlc");
                }
            } else if (target === "#status") {
                if (state.settings.paths.library_folder && !state.library){
                    return
                }
                const libraryIssues = state.library?.issues ?? [];
                let html = $(target + "Template").render({folder: state.settings.paths.library_folder,library:libraryIssues,numFiles:state.library ? state.library.num_files:-1});
                $(target).html(html);
                if (libraryIssues.length) {
                    createTable("#status-table", tableOptions({
                        layout:"fitDataStretch",
                        data: libraryIssues,
                        columns: [
                            {formatter:"rownum",download:false},
                            {title: "File name",width:500, headerSort:false, field: "key",formatter:"textarea",cellClick:function(e, cell){
                                    //e - the click event object
                                    //cell - cell component
                                    ShowInFolder(cell.getData().key).catch(error => showError(error.message))
                                }
                            },
                            {
                                title: "Issue", field: "value", width: 350, formatter: function (cell) {
                                    const value = String(cell.getValue() ?? "");
                                    return value.replace(/[&<>"']/g, character => ({
                                        "&": "&amp;",
                                        "<": "&lt;",
                                        ">": "&gt;",
                                        '"': "&quot;",
                                        "'": "&#39;"
                                    })[character]).replaceAll("\n", "<br/>");
                                }
                            }
                        ],
                    }), "issues");
                }
            } else if (target === "#library") {
                if (state.settings.paths.library_folder && !state.library && !state.libraryRefreshError){
                    return
                }
                const libraryData = state.library?.library_data ?? [];
                const libraryIssues = state.library?.issues ?? [];
                let html = $(target + "Template").render(
                    {
                        folder: state.settings.paths.library_folder,
                        libraryRefreshError: state.libraryRefreshError,
                        library: libraryData,
                        num_issues:libraryIssues.length,
                        num_files:state.library ? state.library.num_files : 0,
                        keys:state.keys
                    })
                $(target).html(html);
                if (libraryData.length) {
                    createTable("#library-table", gameDetailsTableOptions({
                        initialSort:[
                            {column:"name", dir:"asc"}, //sort by this first
                        ],
                        layout:"fitColumns",
                        columnMinWidth:24,
                        data: libraryData,
                        columns: [
                            {formatter:"rownum",download:false,minWidth:24},
                            {field: "icon",minWidth:40,formatter:"image", download:false,headerSort:false,formatterParams:{height:"60px", width:"60px"}},
                            {title: "Title", field: "name", headerFilter:"input",formatter:"textarea",widthGrow:3},
                            {title: "Title ID", headerSort:false, headerFilter:"input", field: "titleId"},
                            {title: "Region", headerSort:true, headerFilter:"input", field: "region"},
                            {title: "Type", headerSort:true, headerFilter:"input", field: "type"},
                            {title: "Update", headerSort:false, field: "update"},
                            {title: "Version", headerSort:false, field: "version", visible: !state.settings.gui.hide_version_string, download: true},
                            {title: "File name", headerSort:false, field: "path",formatter:"textarea",widthGrow:3}
                        ],
                    }, "games"), "games");
                }
            } else if (target === "#missing") {
                if (state.settings.paths.library_folder && !state.library){
                    return
                }
                if (state.library && !state.missingGames){
                    requestTabData(target, "missingGames", GetMissingGames);
                    return
                }
                let html = $(target + "Template").render({folder: state.settings.paths.library_folder,missingGames:state.missingGames});
                $(target).html(html);
                if (state.missingGames && state.missingGames.length) {
                    createTable("#missingGames-table", gameDetailsTableOptions({
                        layout:"fitDataStretch",
                        initialSort:[
                            {column:"name", dir:"asc"}, //sort by this first
                        ],
                        data: state.missingGames,
                        columns: [
                            {formatter:"rownum",download:false},
                            {field: "icon",download:false,formatter:"image", headerSort:false,formatterParams:{height:"60px", width:"60px"}},
                            {field: "name",title: "Title",  headerFilter:"input",formatter:"textarea",width:350},
                            {title: "Title ID", headerSort:false, headerFilter:"input", field: "titleId"},
                            {title: "Region", headerSort:true,headerFilter:"input",formatter:"textarea", field: "region"},
                            {title: "Release date", headerSort:true, field: "release_date", ...dateColumnOptions(state.settings.gui?.date_format)},
                        ],
                    }, "games"), "missing_games");
                }
            }
            updateShortcutMetadata(document.querySelector(target));
            scheduleTabFocus(target);
        }

        $("body").on("click", ".folder-set", e => {
            openFolderPicker(e.target.textContent)
        });

        $("body").on("click", "[data-game-details-close]", () => {
            if (gameDetailsDialog.open) {
                gameDetailsDialog.close();
            }
        });

        $("body").on("click", ".export-btn", e => {
            if (currTable && currTableExportKind) {
                currTable.download("csv", "slm_" + currTableExportKind + "." + exportDate() + ".csv", {bom: true}, "all");
            }
        });

        $("body").on("click", ".export-xlsx-btn", async () => {
            if (!currTable || !currTableExportKind || xlsxExportPending) {
                return;
            }
            xlsxExportPending = true;
            rowContextMenu.close(false);
            document.querySelectorAll(".export-xlsx-btn").forEach(button => button.disabled = true);
            try {
                await exportTableXLSX(currTable, currTableExportKind, ExportXLSX);
            } catch (error) {
                showError(error?.message || String(error));
            } finally {
                xlsxExportPending = false;
                document.querySelectorAll(".export-xlsx-btn").forEach(button => button.disabled = false);
            }
        });

        const requestRescan = hard => {
            if (commandsBlocked() || openDialog() || !state.settings.paths.library_folder) {
                return;
            }
            const returnTab = $("#tab_btns a.active").attr("href") || "#library";
            if (returnTab === "#settings") {
                captureSettingsDraft();
            }
            if (gameDetailsDialog.open) {
                gameDetailsDialog.close();
            }
            scanInProgress = true;
            lastGameDetails = null;
            Promise.resolve()
                .then(() => scanLocalFolder(Boolean(hard), false, returnTab))
                .catch(error => showError(error.message, returnTab))
                .finally(() => {
                    scanInProgress = false;
                });
        };

        $("body").on("click", ".rescan-action", e => {
            requestRescan(e.currentTarget.dataset.hard === "true");
        });

        registerKeyboardShortcuts(window, {
            isMacOS: () => state.platform === "darwin",
            getContext: event => ({
                blocked: commandsBlocked(),
                dialog: openDialog(),
                row: event.target.matches?.(".tabulator-row[data-game-details-row='true']") ? event.target : null
            }),
            onAction(shortcut, context, event) {
                if (shortcut.target) {
                    switchTab(shortcut.target, true);
                    return;
                }
                switch (shortcut.id) {
                case "organize": requestOrganization(); break;
                case "rescan": requestRescan(false); break;
                case "hard-rescan": requestRescan(true); break;
                case "search": {
                    const field = {games: "name", missing_games: "name", missing_updates: "Attributes.name", missing_dlc: "dlc.name"}[currTableExportKind];
                    const input = field && currTable?.getColumn(field)?.getElement().querySelector(".tabulator-header-filter input");
                    if (input) {
                        input.focus();
                        input.select();
                    }
                    break;
                }
                case "row-up":
                case "row-down":
                    currRowNavigation?.move(context.row, shortcut.id === "row-up" ? -1 : 1)
                        .catch(error => console.error("Could not focus the next game row:", error));
                    break;
                case "open-details": context.row.click(); break;
                case "row-menu": {
                    const row = currTable?.getRows("display").find(row => row.getElement() === context.row);
                    if (row) openRowMenu(event, row);
                    break;
                }
                case "dialog-tab": focusDialogControl(context.dialog === "name" ? document.getElementById("game-name-dialog") : context.dialog === "help" ? shortcutDialog : gameDetailsDialog, event.shiftKey); break;
                case "dismiss":
                    if (context.dialog === "name") gameNameEditor.close();
                    else if (context.dialog === "help") shortcutDialog.close();
                    else if (!gameDetailsPathTooltip.hidden) hideGameDetailsPathTooltip();
                    else gameDetailsDialog.close();
                    break;
                case "back":
                    if (context.dialog === "name") gameNameEditor.close();
                    else if (gameDetailsDialog.open) gameDetailsDialog.close();
                    break;
                case "forward":
                    if (lastGameDetails) openGameDetails(lastGameDetails.titleID, lastGameDetails.returnFocusElement);
                    break;
                case "help": showShortcutHelp(); break;
                }
            }
        });

        $("body").on("click", "[data-shortcut-help]", showShortcutHelp);
        $("body").on("click", "[data-shortcut-help-close]", () => shortcutDialog.close());
        updateShortcutMetadata();

        $("body").on("click", ".alert-dismiss", e => {
            $(e.currentTarget).closest(".alert").hide();
        });

        $("body").on("click", "[data-settings-list-add]", e => {
            appendSettingsListRow(e.currentTarget.dataset.settingsListAdd, "");
            captureSettingsDraft();
        });

        $("body").on("click", ".settings-list-remove", e => {
            $(e.currentTarget).closest(".settings-list-row").remove();
            captureSettingsDraft();
        });

        $("body").on("input change", "#settings-form input, #settings-form select", () => {
            updateDateFormatControls();
            captureSettingsDraft();
            state.settingsFeedback = undefined;
        });

        $("body").on("click", ".settings-folder-select", e => {
            SelectFolder()
                .then(path => {
                    if (path) {
                        settingsFolderSelected(e.currentTarget.dataset.settingsFolderTarget, path);
                    }
                })
                .catch(error => showSettingsFeedback("danger", settingsErrorMessage(error)));
        });

        $("body").on("click", "#settings-cancel", () => {
            state.settingsDraft = cloneSettings(state.settings);
            state.settingsFeedback = undefined;
            renderSettingsTab();
        });

        $("body").on("click", "#restart-button", e => {
            if (!restartRequired || e.currentTarget.disabled) {
                return;
            }
            const button = e.currentTarget;
            button.disabled = true;
            button.textContent = "Restarting…";
            $('.progress-msg').text('Starting the application…');
            Restart()
                .catch(error => {
                    const detail = error && error.message ? error.message : String(error);
                    $('.progress-msg').text('Restart failed. You can try again or close and restart the application manually. ' + detail);
                    button.disabled = false;
                    button.textContent = "Restart now";
                    button.focus();
                });
        });

        $("body").on("submit", "#settings-form", e => {
            e.preventDefault();
            if (commandsBlocked() || openDialog()) {
                return;
            }
            if (!updateDateFormatControls()) {
                document.getElementById("settings-gui-date-format-custom").reportValidity();
                return;
            }
            const value = collectSettings(e.currentTarget);
            state.settingsDraft = value;
            saveSettings(value)
                .catch(error => showSettingsFeedback("danger", settingsErrorMessage(error)));
        });

        function requestOrganization() {
            if (commandsBlocked() || openDialog() || !state.settings.paths.library_folder) return;
            if (activeTabTarget === "#settings") captureSettingsDraft();
            organizationPending = true;
            if (state.settings.organization.create_folder_per_game === false &&
                state.settings.organization.rename_files === false &&
                state.settings.organization.move_scan_files_to_library === false){
                return ShowMessage("info", "Library organization is turned off", "Please update the settings to enable this feature", "Enable 'Move files from scan folders into the library', 'Rename files' or 'Create a folder for each game' in the Settings tab")
                    .catch(error => showError(error.message))
                    .finally(() => organizationPending = false);
            }
            return ConfirmOrganization().then(confirmed => {
                if (confirmed) {
                    libraryDataGeneration++;
                    lastGameDetails = null;
                    //show progress
                    $('.tabgroup > div').hide();
                    showProgress("Organizing local library...");

                    const clearOrganizationCaches = () => {
                        state.updates = undefined;
                        state.dlc = undefined;
                        state.missingGames = undefined;
                        state.organizationPreview = undefined;
                    };

                    const refreshOrganizationLibrary = () => UpdateLocalLibrary(false).then(result => {
                        state.library = result;
                        state.libraryRefreshError = undefined;
                        clearOrganizationCaches();
                        return result;
                    });

                    return OrganizeLibrary().then(result => {
                        state.library = result.library;
                        state.libraryRefreshError = undefined;
                        clearOrganizationCaches();
                        hideProgress();
                        if (result.status === "blocked") {
                            loadTab("#status");
                            const conflictCount = Number(result.conflict_count) || 0;
                            const conflictLabel = conflictCount === 1
                                ? "1 file conflict"
                                : conflictCount + " file conflicts";
                            const conflictSummary = conflictCount === 1
                                ? conflictLabel + " was found"
                                : conflictLabel + " were found";
                            ShowMessage(
                                "warning",
                                "Organization canceled",
                                "Organization was canceled because " + conflictSummary + ". No files were moved, renamed, or deleted. Review the Issues tab, resolve the conflicts, and try again.",
                                ""
                            ).catch(error => showError(error.message, "#status"));
                            return;
                        }
                        loadTab("#library");
                        ShowMessage("info", "Success", "Operation completed successfully", "")
                            .catch(error => console.error("Could not display organization success message:", error));
                    }).catch(organizationError => {
                        return refreshOrganizationLibrary().then(() => {
                            hideProgress();
                            loadTab("#library");
                            const detail = organizationError && organizationError.message
                                ? organizationError.message
                                : String(organizationError);
                            ShowMessage("error", "Organization failed", "Organization could not be completed. The library was refreshed; rescan if needed.", detail)
                                .catch(dialogError => console.error("Could not display organization error:", dialogError));
                        }).catch(refreshError => {
                            state.library = undefined;
                            clearOrganizationCaches();
                            state.libraryRefreshError = "Library refresh failed after organization. Use Rescan library to try again.";
                            hideProgress();
                            loadTab("#library");
                            const operationDetail = organizationError && organizationError.message
                                ? organizationError.message
                                : String(organizationError);
                            const refreshDetail = refreshError && refreshError.message
                                ? refreshError.message
                                : String(refreshError);
                            ShowMessage(
                                "error",
                                "Organization and refresh failed",
                                state.libraryRefreshError,
                                operationDetail + "\n\nLibrary refresh failed: " + refreshDetail
                            ).catch(dialogError => console.error("Could not display organization refresh error:", dialogError));
                        });
                    });
                }
            }).catch(error => showError(error.message))
                .finally(() => organizationPending = false);
        }

        $("body").on("click", ".library-organize-action", e => {
            e.preventDefault();
            requestOrganization();
        });

        $('#tab_btns a').click(function (e) {
            e.preventDefault();
            let target = $(e.currentTarget).attr('href');
            switchTab(target);
        });

        function hideCurrentTab() {
            destroyTables();
            $("#tab_btns a").removeClass("active").attr('aria-selected', 'false');
            let tabgroup = $("#tab_btns").data('tabgroup');
            $("#" + tabgroup).children('div').hide();
        }

});
