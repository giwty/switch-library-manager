import assert from "node:assert/strict";
import {readFile} from "node:fs/promises";
import test from "node:test";
import vm from "node:vm";
import {TAB_SHORTCUTS, shortcutHelp} from "../frontend/keyboard_shortcuts.js";

const source = await readFile(new URL("../frontend/app.js", import.meta.url), "utf8");
function section(start, end) {
    const first = source.indexOf(start);
    const last = source.indexOf(end, first);
    assert.ok(first >= 0 && last > first, start);
    return source.slice(first, last);
}
const tick = () => new Promise(resolve => setImmediate(resolve));
function deferred() {
    let resolve, reject;
    const promise = new Promise((yes, no) => {resolve = yes; reject = no;});
    return {promise, resolve, reject};
}

test("all busy states, including native export save dialogs, pause application commands", () => {
    const flags = ["restartRequired", "settingsSavePending", "scanInProgress", "organizationPending", "xlsxExportPending"];
    const context = {nativeDialogsPending: 0, wrapper: {classList: {contains: () => false}}};
    flags.forEach(flag => context[flag] = false);
    vm.createContext(context);
    vm.runInContext(section("    function commandsBlocked()", "\n    function openDialog()"), context);
    assert.equal(context.commandsBlocked(), false);
    for (const flag of flags) {
        context[flag] = true;
        assert.equal(context.commandsBlocked(), true, flag);
        context[flag] = false;
    }
    context.nativeDialogsPending = 1;
    assert.equal(context.commandsBlocked(), true);
    context.nativeDialogsPending = 0;
    context.wrapper.classList.contains = () => true;
    assert.equal(context.commandsBlocked(), true);
});

test("native dialog tracking blocks immediately and always clears on resolution or rejection", async () => {
    const pending = deferred();
    const context = {
        nativeDialogsPending: 0,
        rowContextMenu: {close() {}},
        ConfirmNativeOrganization: () => pending.promise,
        SelectNativeFolder: async () => {throw new Error("synthetic picker failure");},
        ShowNativeMessage: async () => {}
    };
    vm.createContext(context);
    vm.runInContext(section("    function withNativeDialog(", "\n    function commandsBlocked()"), context);
    const confirmation = vm.runInContext("ConfirmOrganization()", context);
    assert.equal(context.nativeDialogsPending, 1);
    pending.resolve(false);
    assert.equal(await confirmation, false);
    assert.equal(context.nativeDialogsPending, 0);
    const picker = vm.runInContext("SelectFolder()", context);
    assert.equal(context.nativeDialogsPending, 1);
    await assert.rejects(picker, /synthetic picker failure/);
    assert.equal(context.nativeDialogsPending, 0);
});

function organizationFixture() {
    const confirmation = deferred();
    const operation = deferred();
    const calls = [];
    const context = {
        organizationPending: false, libraryDataGeneration: 0, activeTabTarget: "#library", lastGameDetails: {},
        state: {settings: {paths: {library_folder: "synthetic"}, organization: {rename_files: true}}, library: {old: true}},
        commandsBlocked: () => context.organizationPending,
        openDialog: () => null,
        captureSettingsDraft: () => calls.push("draft"),
        ConfirmOrganization: () => {calls.push("confirm"); return confirmation.promise;},
        OrganizeLibrary: () => {calls.push("organize"); return operation.promise;},
        UpdateLocalLibrary: async () => {calls.push("refresh"); return {refreshed: true};},
        ShowMessage: async (kind, title) => calls.push(title),
        showProgress: () => calls.push("progress"), hideProgress: () => calls.push("hide-progress"),
        loadTab: target => calls.push(target), showError: error => calls.push(error),
        $: () => ({hide() {}}), console
    };
    vm.createContext(context);
    vm.runInContext(section("        function requestOrganization()", '\n        $("body").on("click", ".library-organize-action"'), context);
    return {context, calls, confirmation, operation};
}

test("Organize shares confirmation, ignores duplicate requests, and does nothing on cancellation", async () => {
    const f = organizationFixture();
    const pending = f.context.requestOrganization();
    f.context.requestOrganization();
    assert.deepEqual(f.calls, ["confirm"]);
    assert.equal(f.context.organizationPending, true);
    f.confirmation.resolve(false);
    await pending;
    assert.equal(f.context.organizationPending, false);
    assert.deepEqual(f.context.state.library, {old: true});
    assert.equal(f.context.libraryDataGeneration, 0);
    assert.deepEqual(f.calls, ["confirm"]);
});

test("confirmed organization retains progress, cache invalidation, and success behavior", async () => {
    const f = organizationFixture();
    const pending = f.context.requestOrganization();
    f.confirmation.resolve(true);
    await tick();
    assert.equal(f.context.organizationPending, true);
    assert.equal(f.context.libraryDataGeneration, 1);
    assert.equal(f.context.lastGameDetails, null);
    f.context.requestOrganization();
    f.operation.resolve({library: {organized: true}, status: "success"});
    await pending;
    assert.deepEqual(f.context.state.library, {organized: true});
    assert.equal(f.context.state.organizationPreview, undefined);
    assert.deepEqual(f.calls, ["confirm", "progress", "organize", "hide-progress", "#library", "Success"]);
    assert.equal(f.context.organizationPending, false);
});

test("organization conflicts and operation failures retain their existing recovery paths", async () => {
    const blocked = organizationFixture();
    const pending = blocked.context.requestOrganization();
    blocked.confirmation.resolve(true);
    blocked.operation.resolve({library: {conflicts: true}, status: "blocked", conflict_count: 2});
    await pending;
    assert.ok(blocked.calls.includes("#status"));
    assert.ok(blocked.calls.includes("Organization canceled"));
    assert.ok(!blocked.calls.includes("Success"));

    const failed = organizationFixture();
    const recovery = failed.context.requestOrganization();
    failed.confirmation.resolve(true);
    await tick();
    failed.operation.reject(new Error("synthetic rename failure"));
    await recovery;
    assert.deepEqual(failed.context.state.library, {refreshed: true});
    assert.ok(failed.calls.includes("Organization failed"));
    assert.equal(failed.context.organizationPending, false);
});

test("organization restores readiness after confirmation errors, disabled settings, and refresh failures", async () => {
    const f = organizationFixture();
    const pending = f.context.requestOrganization();
    f.confirmation.reject(new Error("synthetic dialog failure"));
    await pending;
    assert.equal(f.context.organizationPending, false);
    assert.ok(f.calls.includes("synthetic dialog failure"));

    f.context.state.settings.organization = {rename_files: false, create_folder_per_game: false, move_scan_files_to_library: false};
    await f.context.requestOrganization();
    assert.ok(f.calls.includes("Library organization is turned off"));
    assert.equal(f.calls.filter(value => value === "confirm").length, 1);

    const recovery = organizationFixture();
    recovery.context.UpdateLocalLibrary = async () => {throw new Error("synthetic refresh failure");};
    const operation = recovery.context.requestOrganization();
    recovery.confirmation.resolve(true);
    await tick();
    recovery.operation.reject(new Error("synthetic organization failure"));
    await operation;
    assert.equal(recovery.context.state.library, undefined);
    assert.match(recovery.context.state.libraryRefreshError, /Rescan library/);
    assert.ok(recovery.calls.includes("Organization and refresh failed"));
    assert.equal(recovery.context.organizationPending, false);
});

test("hidden tab numbers do not shift, and Settings drafts survive keyboard tab switches", () => {
    const calls = [];
    const context = {
        activeTabTarget: "#settings", keyboardFocusTarget: null,
        state: {settings: {gui: {hide_missing_games: true}}},
        commandsBlocked: () => false, openDialog: () => null,
        captureSettingsDraft: () => calls.push("draft"), loadTab: target => calls.push(target)
    };
    vm.createContext(context);
    vm.runInContext(section("    function switchTab(", "\n    function requestTabData("), context);
    context.switchTab(TAB_SHORTCUTS[1].target, true);
    assert.deepEqual(calls, []);
    context.switchTab(TAB_SHORTCUTS[2].target, true);
    assert.deepEqual(calls, ["draft", "#updates"]);
    assert.equal(context.keyboardFocusTarget, "#updates");
});

test("late tab data is cached without switching tabs; duplicate loads and old generations are ignored", async () => {
    const pending = deferred();
    const calls = [];
    const context = {
        libraryDataGeneration: 1, activeTabTarget: "#updates", restartRequired: false,
        pendingTabLoads: new Map(), state: {}, $: () => ({html() {}}),
        loadTab: target => calls.push(target), console
    };
    vm.createContext(context);
    vm.runInContext(section("    function requestTabData(", "\n    function setLoading("), context);
    let fetches = 0;
    const fetch = () => {fetches++; return pending.promise;};
    context.requestTabData("#updates", "updates", fetch);
    context.requestTabData("#updates", "updates", fetch);
    context.activeTabTarget = "#library";
    pending.resolve([{name: "synthetic"}]);
    await tick();
    assert.equal(fetches, 1);
    assert.equal(context.state.updates[0].name, "synthetic");
    assert.deepEqual(calls, []);

    const stale = deferred();
    context.requestTabData("#dlc", "dlc", () => stale.promise);
    context.libraryDataGeneration++;
    stale.resolve(["stale"]);
    await tick();
    assert.equal(context.state.dlc, undefined);

    context.activeTabTarget = "#missing";
    context.requestTabData("#missing", "missingGames", async () => []);
    await tick();
    assert.deepEqual(calls, ["#missing"]);
});

test("queued keyboard focus cannot jump into a tab that was left", () => {
    let frame;
    const calls = [];
    const context = {
        keyboardFocusTarget: "#library", activeTabTarget: "#library", tabRenderGeneration: 1,
        window: {requestAnimationFrame: callback => frame = callback},
        commandsBlocked: () => false, openDialog: () => null,
        currRowNavigation: {focusFirst() {calls.push("row"); return true;}},
        document: {querySelector: () => ({focus() {calls.push("panel");}})}
    };
    vm.createContext(context);
    vm.runInContext(section("    function scheduleTabFocus(", "\n    function switchTab("), context);
    context.scheduleTabFocus("#library");
    context.activeTabTarget = "#settings";
    frame();
    assert.deepEqual(calls, []);
    context.activeTabTarget = "#library";
    context.currRowNavigation.focusFirst = () => false;
    context.scheduleTabFocus("#library");
    frame();
    assert.deepEqual(calls, ["panel"]);
});

test("help is generated from shortcut definitions, opens once, and restores focus on close", () => {
    const calls = [];
    const context = {
        state: {platform: "darwin"}, shortcutHelp, shortcutReturnFocus: null,
        rowContextMenu: {close() {}},
        document: {
            activeElement: {isConnected: true, focus() {calls.push("focus");}},
            createElement: () => ({append() {}, textContent: ""})
        },
        shortcutHelpBody: {replaceChildren(...rows) {calls.push(rows.length);}},
        shortcutDialog: {open: false, showModal() {this.open = true; calls.push("open");}, addEventListener(type, callback) {this.onClose = callback;}},
        commandsBlocked: () => false, openDialog: () => context.shortcutDialog.open ? "help" : null
    };
    vm.createContext(context);
    vm.runInContext(section("    function renderShortcutHelp(", "\n    function scheduleTabFocus("), context);
    context.showShortcutHelp();
    context.showShortcutHelp();
    assert.deepEqual(calls, [shortcutHelp(true).length, "open"]);
    context.shortcutDialog.open = false;
    context.shortcutDialog.onClose();
    assert.equal(calls.at(-1), "focus");
    assert.equal(context.shortcutReturnFocus, null);
});

test("dialog Tab wraps both directions and skips hidden or disabled controls", () => {
    const context = {document: {activeElement: null}};
    const control = (disabled = false, visible = true) => ({
        disabled, tabIndex: 0, getClientRects: () => visible ? [{}] : [],
        focus() {context.document.activeElement = this;}
    });
    const first = control();
    const last = control();
    const dialog = {querySelectorAll: () => [first, control(true), control(false, false), last]};
    vm.createContext(context);
    vm.runInContext(section("    function focusDialogControl(", "\n    function showShortcutHelp("), context);
    context.document.activeElement = last;
    context.focusDialogControl(dialog, false);
    assert.equal(context.document.activeElement, first);
    context.focusDialogControl(dialog, true);
    assert.equal(context.document.activeElement, last);
    dialog.querySelectorAll = () => [first];
    context.document.activeElement = first;
    context.focusDialogControl(dialog, false);
    assert.equal(context.document.activeElement, first);
});

test("late details-close events preserve a newly focused filter and do not invalidate reopened details", () => {
    const calls = [];
    const row = {isConnected: true, focus() {calls.push("row");}};
    const context = {
        gameDetailsDialog: {open: false, contains: () => false, addEventListener(type, fn) {this.onClose = fn;}},
        document: {activeElement: {}, body: {}},
        gameDetailsReturnFocus: row, gameDetailsRequest: 0, gameDetailsFileRequest: 0,
        hideGameDetailsPathTooltip() {}, clearGameDetailsFileError() {}, commandsBlocked: () => false,
        currRowNavigation: {focusFirst() {calls.push("fallback");}}
    };
    vm.createContext(context);
    vm.runInContext(section('    gameDetailsDialog.addEventListener("close"', "\n    function detailRowTitleID("), context);
    context.gameDetailsDialog.onClose();
    assert.deepEqual(calls, []);
    context.document.activeElement = context.document.body;
    context.gameDetailsReturnFocus = row;
    context.gameDetailsDialog.onClose();
    assert.deepEqual(calls, ["row"]);
    context.gameDetailsDialog.open = true;
    const request = context.gameDetailsRequest;
    context.gameDetailsDialog.onClose();
    assert.equal(context.gameDetailsRequest, request);
});
