import {
    CheckUpdate,
    ExportXLSX,
    ConfirmOrganization,
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
    SelectFolder,
    ShowInFolder,
    ShowMessage,
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
import { registerRescanShortcuts } from './rescan_shortcuts.js'
import { exportTableXLSX } from './xlsx_export.js'

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

    function createTable(element, options, exportKind) {
        currTable = new Tabulator(element, options);
        activeTables.add(currTable);
        currTableExportKind = exportKind;
    }

    function destroyTables() {
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
    let gameDetailsRequest = 0;
    let gameDetailsReturnFocus = null;
    let gameDetailsFileRequest = 0;
    let gameDetailsPathTooltipTarget = null;
    let gameDetailsPathTooltipHideTimer;

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
        if (!titleID || !gameDetailsDialog || !gameDetailsContent) {
            return;
        }

        const request = ++gameDetailsRequest;
        gameDetailsFileRequest++;
        hideGameDetailsPathTooltip();
        clearGameDetailsFileError();
        gameDetailsReturnFocus = returnFocusElement || document.activeElement;
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
            gameDetailsDialog.setAttribute("aria-label", details.name ? "Game details for " + details.name : "Game details");
            gameDetailsContent.innerHTML = $("#gameDetailsTemplate").render(details);
            renderGameDetailsFileActions();
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
        gameDetailsRequest++;
        gameDetailsFileRequest++;
        hideGameDetailsPathTooltip();
        clearGameDetailsFileError();
        if (gameDetailsReturnFocus && gameDetailsReturnFocus.isConnected) {
            gameDetailsReturnFocus.focus();
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

    function gameDetailsTableOptions(options, kind) {
        const optionsWithDetails = Object.assign({}, options || {});
        const rowFormatter = optionsWithDetails.rowFormatter;
        optionsWithDetails.rowFormatter = function (row) {
            const data = row.getData();
            const element = row.getElement();
            const label = detailRowLabel(data, kind) || "game";
            element.dataset.gameDetailsRow = "true";
            element.setAttribute("tabindex", "0");
            element.setAttribute("aria-haspopup", "dialog");
            element.setAttribute("aria-label", "Open details for " + label);
            if (rowFormatter) {
                rowFormatter.call(this, row);
            }
        };
        optionsWithDetails.rowClick = function (event, row) {
            openGameDetails(detailRowTitleID(row.getData(), kind), row.getElement());
        };
        return tableOptions(optionsWithDetails);
    }

    document.addEventListener("keydown", event => {
        const row = event.target.closest && event.target.closest(".tabulator-row[data-game-details-row='true']");
        if (!row || (event.key !== "Enter" && event.key !== " ")) {
            return;
        }
        event.preventDefault();
        row.click();
    });

    function setLoading(loading) {
        loading = loading || restartRequired;
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

    function lockForRestart() {
        restartRequired = true;
        wrapper.classList.add('restart-required');
        $('.progress-type').text('Restart required');
        $('.progress-msg').text('Settings were saved. Restart the application to apply your changes.');
        $('.progress-container').attr('aria-label', 'Application restart required');
        const restartButton = document.getElementById('restart-button');
        restartButton.hidden = false;
        setLoading(true);
        restartButton.focus();
    }

    function updateRescanControlLabels(root = document.querySelector("#organize")) {
        if (!root) {
            return;
        }
        const isMacOS = state.platform === "darwin";
        const labels = isMacOS
            ? {
                rescan: "⌘R",
                hardRescan: "⌘⇧R",
                rescanKeys: "Meta+r Control+r",
                hardRescanKeys: "Meta+Shift+r Control+Shift+r"
            }
            : {
                rescan: "Ctrl+R",
                hardRescan: "Ctrl+Shift+R",
                rescanKeys: "Control+r",
                hardRescanKeys: "Control+Shift+r"
            };

        root.querySelectorAll(".rescan-action").forEach(button => {
            const hard = button.dataset.hard === "true";
            button.textContent = hard
                ? "Hard rescan (" + labels.hardRescan + ")"
                : "Rescan (" + labels.rescan + ")";
            button.setAttribute("aria-keyshortcuts", hard ? labels.hardRescanKeys : labels.rescanKeys);
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
                updateRescanControlLabels();
            })
            .catch(error => console.error("Could not determine the current platform:", error));
        LoadSettings().then(function (message) {
            state.settings = message;
            state.settingsDraft = undefined;
            state.settingsFeedback = undefined;
            applyTheme(state.settings.gui && state.settings.gui.theme);

            if(state.settings.gui.hide_missing_games){
                document.getElementById("tab_btns").classList.add("hide_missing_games");
            }
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
            if (!path) {
                return
            }

            if (mode === "add"){
                state.settings.paths.scan_folders = state.settings.paths.scan_folders || []
                if (!state.settings.paths.scan_folders.includes(path)){
                    state.settings.paths.scan_folders.push(path);
                }else{
                    return;
                }

            }else{
                state.settings.paths.library_folder = path;
            }
            $('.tabgroup > div').hide();
            showProgress("Scanning local library...");
            console.log("selected folder:"+path);
            state.library = undefined;
            state.updates = undefined;
            state.dlc = undefined;
            state.missingGames = undefined;
            state.organizationPreview = undefined;
            state.libraryRefreshError = undefined;
            SaveSettings(state.settings)
                .then(() => {
                    state.settingsDraft = undefined;
                    return scanLocalFolder(false);
                })
                .catch(error => showError(error.message));
        };


        function cloneSettings(value) {
            return JSON.parse(JSON.stringify(value));
        }

        function settingsErrorMessage(error) {
            return error && error.message ? error.message : String(error);
        }

        function renderSettingsTab() {
            if (!state.settingsDraft) {
                state.settingsDraft = cloneSettings(state.settings);
            }
            let templateData = Object.assign({}, state.settingsDraft, {
                feedback: state.settingsFeedback
            });
            let settingsHtml = $("#settingsTemplate").render(templateData);
            $("#settings").html(settingsHtml);
        }

        function showSettingsFeedback(type, message) {
            state.settingsFeedback = {type: type, message: message};
            renderSettingsTab();
        }

        function listValues(form, name) {
            return Array.from(form.querySelectorAll('[data-settings-list="' + name + '"]'))
                .map(input => input.value.trim())
                .filter(value => value.length > 0);
        }

        function collectSettings(form) {
            return {
                schema_version: state.settings.schema_version,
                gui: {
                    enabled: form.elements.gui_enabled.checked,
                    page_size: Number(form.elements.gui_page_size.value),
                    hide_missing_games: form.elements.gui_hide_missing_games.checked,
                    hide_demo_games: form.elements.gui_hide_demo_games.checked,
                    hide_version_string: form.elements.gui_hide_version_string.checked,
                    remember_window_state: form.elements.gui_remember_window_state.checked,
                    theme: form.elements.gui_theme.value
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
                    updateRescanControlLabels(document.querySelector(target));
                    return;
                }
                if (!state.organizationPreview || state.organizationPreview.root_folder !== folder) {
                    $(target).html($(target + "Template").render({folder: folder, organization: organization, previewLoading: true}));
                    updateRescanControlLabels(document.querySelector(target));
                    const generation = libraryDataGeneration;
                    GetOrganizationPreview().then(preview => {
                        if (generation !== libraryDataGeneration) {
                            return;
                        }
                        state.organizationPreview = preview;
                        if (!restartRequired && $(target).is(":visible")) {
                            loadTab(target);
                        }
                    }).catch(error => {
                        if (generation === libraryDataGeneration) {
                            showError(error.message);
                        }
                    });
                    return;
                }
                let html = $(target + "Template").render({folder: folder, organization: organization, preview: state.organizationPreview})
                $(target).html(html);
                updateRescanControlLabels(document.querySelector(target));
            } else if (target === "#updates") {
                if (state.settings.paths.library_folder && !state.library){
                    return
                }
                if (state.library && !state.updates){
                    const generation = libraryDataGeneration;
                    GetMissingUpdates().then(r => {
                        if (generation !== libraryDataGeneration) {
                            return;
                        }
                        state.updates = r
                        loadTab("#updates")
                    }).catch(error => {
                        if (generation === libraryDataGeneration) {
                            showError(error.message);
                        }
                    });
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
                            {title: "Update date", headerSort:true, field: "latest_update_date",sorter:"date", sorterParams:{format:"YYYY-MM-DD"}}
                        ],
                    }, "updates"), "missing_updates");
                }
            } else if (target === "#dlc") {
                if (state.settings.paths.library_folder && !state.library){
                    return
                }
                if (state.library && !state.dlc){
                    const generation = libraryDataGeneration;
                    GetMissingDLC().then(r => {
                        if (generation !== libraryDataGeneration) {
                            return;
                        }
                        state.dlc = r
                        loadTab("#dlc")
                    }).catch(error => {
                        if (generation === libraryDataGeneration) {
                            showError(error.message);
                        }
                    });
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
                        initialSort:[
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
                    const generation = libraryDataGeneration;
                    GetMissingGames().then(r => {
                        if (generation !== libraryDataGeneration) {
                            return;
                        }
                        state.missingGames = r
                        loadTab("#missing")
                    }).catch(error => {
                        if (generation === libraryDataGeneration) {
                            showError(error.message);
                        }
                    });
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
                            {title: "Release date", headerSort:true, field: "release_date", sorter:"date", sorterParams:{format:"YYYY-MM-DD"}},
                        ],
                    }, "games"), "missing_games");
                }
            }
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
            if (restartRequired || settingsSavePending || scanInProgress || wrapper.classList.contains("is-loading") || !state.settings.paths.library_folder) {
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

        registerRescanShortcuts(
            window,
            () => state.platform === "darwin",
            () => restartRequired || settingsSavePending || scanInProgress || wrapper.classList.contains("is-loading") || !state.settings.paths.library_folder,
            requestRescan
        );

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
            if (settingsSavePending || restartRequired) {
                return;
            }
            const value = collectSettings(e.currentTarget);
            state.settingsDraft = value;
            settingsSavePending = true;
            SaveSettings(value)
                .then(() => LoadSettings())
                .then(saved => {
                    state.settings = saved;
                    state.settingsDraft = cloneSettings(saved);
                    state.settingsFeedback = undefined;
                    lockForRestart();
                })
                .catch(error => showSettingsFeedback("danger", settingsErrorMessage(error)))
                .finally(() => {
                    settingsSavePending = false;
                });
        });

        $("body").on("click", ".library-organize-action", e => {
            e.preventDefault();
            if (state.settings.organization.create_folder_per_game === false &&
                state.settings.organization.rename_files === false &&
                state.settings.organization.move_scan_files_to_library === false){
                ShowMessage("info", "Library organization is turned off", "Please update the settings to enable this feature", "Enable 'Move files from scan folders into the library', 'Rename files' or 'Create a folder for each game' in the Settings tab")
                    .catch(error => showError(error.message));
                return
            }
            ConfirmOrganization().then(confirmed => {
                if (confirmed) {
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

                    OrganizeLibrary().then(result => {
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
                        refreshOrganizationLibrary().then(() => {
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
            }).catch(error => showError(error.message));

        });

        $('#tab_btns a').click(function (e) {
            e.preventDefault();
            let target = $(e.currentTarget).attr('href');
            loadTab(target);
        });

        function hideCurrentTab() {
            destroyTables();
            $("#tab_btns a").removeClass("active").attr('aria-selected', 'false');
            let tabgroup = $("#tab_btns").data('tabgroup');
            $("#" + tabgroup).children('div').hide();
        }

});
