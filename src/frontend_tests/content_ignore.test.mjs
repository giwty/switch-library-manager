import assert from "node:assert/strict";
import {readFile} from "node:fs/promises";
import test from "node:test";
import vm from "node:vm";
import {applyIgnoreEntry, contentIgnoreAction, mergeIgnoreDraft} from "../frontend/content_ignore.js";

const baseID = "0100abcd00000000";
const dlcID = "0100abcd00001001";
const source = await readFile(new URL("../frontend/app.js", import.meta.url), "utf8");
const saveSource = source.slice(source.indexOf("    async function saveContentIgnore("), source.indexOf("    function hideGameDetailsPathTooltip("));
const tick = () => new Promise(resolve => setImmediate(resolve));
function deferred() {
    let resolve, reject;
    const promise = new Promise((yes, no) => {resolve = yes; reject = no;});
    return {promise, resolve, reject};
}

test("each supported table targets its content ID and only Library offers resume", () => {
    const missing = {ignore_update_title_ids: [baseID.toUpperCase()]};
    assert.deepEqual(contentIgnoreAction({titleId: baseID}, "games", {}),
        {kind: "updates", titleID: baseID, ignored: true, label: "Ignore updates"});
    assert.deepEqual(contentIgnoreAction({titleId: baseID.toUpperCase()}, "games", missing),
        {kind: "updates", titleID: baseID, ignored: false, label: "Resume checking updates"});
    for (const id of [baseID, dlcID]) {
        assert.deepEqual(contentIgnoreAction({Attributes: {id}}, "missing_updates", missing),
            {kind: "updates", titleID: id, ignored: true, label: "Ignore updates"});
    }
    assert.deepEqual(contentIgnoreAction({game: {id: baseID}, dlc: {id: dlcID}}, "missing_dlc", {}),
        {kind: "dlc", titleID: dlcID, ignored: true, label: "Ignore DLC"});
    for (const kind of ["missing_games", "issues"]) assert.equal(contentIgnoreAction({titleId: baseID}, kind, {}), null);
    for (const id of [undefined, "bad", "x".repeat(16)]) assert.equal(contentIgnoreAction({titleId: id}, "games", {}), null);
});

test("entry changes are case insensitive and preserve unrelated draft edits", () => {
    const ids = [baseID.toUpperCase(), dlcID, baseID];
    assert.deepEqual(applyIgnoreEntry(ids, baseID, true), [dlcID, baseID]);
    assert.deepEqual(applyIgnoreEntry(ids, baseID, false), [dlcID]);
    assert.equal(ids.length, 3);
    const draft = {gui: {theme: "dark"}, missing_content: {ignore_update_title_ids: [dlcID], ignore_dlc_title_ids: []}};
    mergeIgnoreDraft(draft, {kind: "updates", titleID: baseID, ignored: true});
    assert.deepEqual(draft, {gui: {theme: "dark"}, missing_content: {ignore_update_title_ids: [dlcID, baseID], ignore_dlc_title_ids: []}});
    mergeIgnoreDraft(undefined, {kind: "updates", titleID: baseID, ignored: true});
});

function fixture(target = "#library") {
    const pending = deferred();
    const calls = [];
    const row = {isConnected: true, focus() {calls.push("focus");}};
    const rows = [{getElement: () => row}];
    const table = {
        getPage: () => 3, getPageMax: () => 2,
        getSorters: () => [{field: "name", dir: "desc"}], getHeaderFilters: () => [{field: "name", value: "Game"}],
        getRows: () => rows, setSort(value) {calls.push(["sort", value]);},
        setHeaderFilterValue(field, value) {calls.push(["filter", field, value]);},
        async setPage(page) {calls.push(["page", page]);}
    };
    const context = {
        state: {settings: {missing_content: {}}, settingsDraft: {gui: {theme: "dark"}, missing_content: {ignore_update_title_ids: [dlcID]}},
            updates: ["old"], dlc: ["old"], library: {library_data: ["library"]}},
        settingsSavePending: false, lastGameDetails: {}, libraryDataGeneration: 1,
        activeTabTarget: target, currTable: table, mergeIgnoreDraft,
        rowContextMenu: {close() {}}, commandsBlocked: () => context.settingsSavePending, openDialog: () => null,
        SetContentIgnored: (...args) => {calls.push(["save", ...args]); return pending.promise;},
        GetMissingUpdates: async () => [{Attributes: {id: dlcID}}], GetMissingDLC: async () => [],
        loadTab: target => calls.push(["load", target]),
        destroyTables() {context.currTable = undefined; calls.push("destroy");},
        settingsErrorMessage: error => error.message,
        ShowMessage: async (...args) => calls.push(["message", ...args]), console,
        document: {createElement: () => ({setAttribute() {}}), querySelector: () => ({focus() {calls.push("panel");}, replaceChildren() {calls.push("notice");}})}
    };
    vm.createContext(context);
    vm.runInContext(saveSource, context);
    return {context, calls, row, pending};
}

test("successful saves update settings and drafts, invalidate stale requests and never rescan or restart", async () => {
    const f = fixture();
    const action = {kind: "updates", titleID: baseID, ignored: true};
    const saving = f.context.saveContentIgnore(action, f.row);
    f.context.saveContentIgnore(action, f.row);
    assert.equal(f.context.settingsSavePending, true);
    assert.equal(f.calls.filter(call => call[0] === "save").length, 1);
    const missing = {ignore_update_title_ids: [baseID], ignore_dlc_title_ids: []};
    f.pending.resolve(missing);
    await saving;
    assert.equal(f.context.state.settings.missing_content, missing);
    assert.deepEqual(f.context.state.settingsDraft.missing_content.ignore_update_title_ids, [dlcID, baseID]);
    assert.equal(f.context.state.settingsDraft.gui.theme, "dark");
    assert.equal(f.context.libraryDataGeneration, 2);
    assert.equal(f.context.state.updates, undefined);
    assert.equal(f.context.state.dlc, undefined);
    assert.equal(f.context.lastGameDetails, null);
    assert.equal(f.context.settingsSavePending, false);
    assert.deepEqual(f.calls.at(-1), "focus");
});

test("failed saves keep the previous settings, rows and request generation", async () => {
    const f = fixture("#updates");
    const before = JSON.stringify(f.context.state);
    const saving = f.context.saveContentIgnore({kind: "updates", titleID: baseID, ignored: true}, f.row);
    f.pending.reject(new Error("synthetic write failure"));
    await saving;
    assert.equal(JSON.stringify(f.context.state), before);
    assert.equal(f.context.libraryDataGeneration, 1);
    assert.ok(f.calls.some(call => call[0] === "message" && call[2] === "Could not save ignore preference"));
    assert.equal(f.context.settingsSavePending, false);
});

test("missing-content refresh retains filters, sorting and a clamped page", async () => {
    const f = fixture("#updates");
    const saving = f.context.saveContentIgnore({kind: "updates", titleID: baseID, ignored: true}, f.row);
    f.pending.resolve({ignore_update_title_ids: [baseID]});
    await saving;
    assert.equal(f.context.state.updates[0].Attributes.id, dlcID);
    assert.ok(f.calls.some(call => call[0] === "load" && call[1] === "#updates"));
    assert.equal(JSON.stringify(f.calls.find(call => call[0] === "sort")), JSON.stringify(["sort", [{column: "name", dir: "desc"}]]));
    assert.ok(f.calls.some(call => call[0] === "filter" && call[2] === "Game"));
    assert.ok(f.calls.some(call => call[0] === "page" && call[1] === 2));
    assert.equal(f.calls.at(-1), "focus");
});

test("a saved preference remains saved if refresh fails, with a distinct retry notice", async () => {
    const f = fixture("#dlc");
    f.context.GetMissingDLC = async () => {throw new Error("synthetic refresh failure");};
    const saving = f.context.saveContentIgnore({kind: "dlc", titleID: dlcID, ignored: true}, f.row);
    f.pending.resolve({ignore_dlc_title_ids: [dlcID]});
    await saving;
    assert.equal(f.context.state.settings.missing_content.ignore_dlc_title_ids[0], dlcID);
    assert.ok(f.calls.includes("notice"));
    assert.ok(f.calls.some(call => call[0] === "message" && call[2] === "Ignore preference saved; refresh failed"));
    assert.equal(f.context.settingsSavePending, false);
});

test("a pending response from before an ignore save cannot restore an ignored row", async () => {
    const f = fixture();
    const old = deferred();
    Object.assign(f.context, {
        pendingTabLoads: new Map(), restartRequired: false, $: () => ({html() {}}), scheduleTabFocus() {}
    });
    const start = source.indexOf("    function requestTabData(");
    vm.runInContext(source.slice(start, source.indexOf("    function setLoading(", start)), f.context);
    f.context.requestTabData("#updates", "updates", () => old.promise);
    const saving = f.context.saveContentIgnore({kind: "updates", titleID: baseID, ignored: true}, f.row);
    f.pending.resolve({ignore_update_title_ids: [baseID]});
    await saving;
    old.resolve(["stale ignored game"]);
    await tick();
    assert.equal(f.context.state.updates, undefined);
});

test("row context menus preserve normal details clicks and can be explicitly disabled", () => {
    const calls = [];
    const context = {tableOptions: options => options, openRowMenu: () => calls.push("menu"),
        detailRowTitleID: data => data.titleId, openGameDetails: id => calls.push(id)};
    vm.createContext(context);
    const start = source.indexOf("    function gameDetailsTableOptions(");
    vm.runInContext(source.slice(start, source.indexOf('    document.addEventListener("focusin"', start)), context);
    const options = context.gameDetailsTableOptions({}, "games");
    const row = {getData: () => ({titleId: baseID}), getElement: () => ({})};
    options.rowContext({}, row);
    assert.deepEqual(calls, ["menu"]);
    options.rowClick({}, row);
    assert.deepEqual(calls, ["menu", baseID]);
    assert.equal(context.gameDetailsTableOptions({}, "games", false).rowContext, undefined);
});
