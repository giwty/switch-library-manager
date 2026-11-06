import assert from "node:assert/strict";
import {readFile} from "node:fs/promises";
import test from "node:test";
import vm from "node:vm";

const source = await readFile(new URL("../frontend/app.js", import.meta.url), "utf8");
const functionStart = source.indexOf("        function loadTab(target) {");
const functionEnd = source.indexOf('\n        $("body").on("click", ".folder-set"', functionStart);
assert.notEqual(functionStart, -1, "loadTab function exists");
assert.notEqual(functionEnd, -1, "loadTab function boundary exists");
const loadTabSource = source.slice(functionStart, functionEnd);

function createLibraryRenderer(library, target = "#library") {
    const rendered = [];
    const tables = [];
    const ui = {
        addClass() { return this; },
        attr() { return this; },
        show() { return this; },
        html() { return this; },
        render(data) {
            rendered.push(data);
            return "rendered";
        }
    };
    const context = {
        activeTabTarget: "#library", tabRenderGeneration: 0, keyboardFocusTarget: null,
        document: {querySelector() {return {}; }},
        updateShortcutMetadata() {}, scheduleTabFocus() {},
        state: {
            settings: {
                paths: {library_folder: "C:/synthetic-library"},
                gui: {hide_version_string: false}
            },
            library,
            keys: false
        },
        $: () => ui,
        hideCurrentTab() {},
        createTable(selector, options) { tables.push({selector, options}); },
        gameDetailsTableOptions: options => options,
        tableOptions: options => options
    };
    vm.createContext(context);
    vm.runInContext(loadTabSource, context);
    vm.runInContext(`loadTab(${JSON.stringify(target)})`, context);
    return {rendered, tables};
}

test("empty library lists render the existing empty-folder state", () => {
    const {rendered, tables} = createLibraryRenderer({library_data: [], issues: [], num_files: 0});

    assert.equal(rendered[0].library.length, 0);
    assert.equal(rendered[0].num_issues, 0);
    assert.deepEqual(tables, []);
});

test("null library lists safely fall back to empty arrays", () => {
    const {rendered, tables} = createLibraryRenderer({library_data: null, issues: null, num_files: 0});

    assert.equal(rendered[0].library.length, 0);
    assert.equal(rendered[0].num_issues, 0);
    assert.deepEqual(tables, []);

    const status = createLibraryRenderer({library_data: [], issues: null, num_files: 0}, "#status");
    assert.equal(status.rendered[0].library.length, 0);
    assert.deepEqual(status.tables, []);
});

test("an absent library remains in the loading state without throwing", () => {
    const {rendered, tables} = createLibraryRenderer(undefined);

    assert.deepEqual(rendered, []);
    assert.deepEqual(tables, []);
});

test("issues without games render in both Library and Issues views", () => {
    const library = {
        library_data: null,
        issues: [{key: "notes.txt", value: "file type is not supported"}],
        num_files: 1
    };
    const libraryView = createLibraryRenderer(library);
    assert.equal(libraryView.rendered[0].library.length, 0);
    assert.equal(libraryView.rendered[0].num_issues, 1);
    assert.deepEqual(libraryView.tables, []);

    const statusView = createLibraryRenderer(library, "#status");
    assert.equal(statusView.tables.length, 1);
    assert.equal(statusView.tables[0].selector, "#status-table");
    assert.equal(statusView.tables[0].options.data[0].key, "notes.txt");
});

test("populated library rows still create the games table", () => {
    const row = {name: "Synthetic Game", titleId: "0100000000001000"};
    const {rendered, tables} = createLibraryRenderer({library_data: [row], issues: [], num_files: 1});

    assert.deepEqual(rendered[0].library, [row]);
    assert.equal(tables.length, 1);
    assert.equal(tables[0].selector, "#library-table");
    assert.equal(tables[0].options.data[0].name, "Synthetic Game");
});

test("Game Details renders DLC release dates and unknowns without changing the local-only table", async () => {
    const index = await readFile(new URL("../frontend/index.html", import.meta.url), "utf8");
    const jsrender = await readFile(new URL("../frontend/lib/js/jsrender/jsrender.min.js", import.meta.url), "utf8");
    const context = {window: {}};
    vm.createContext(context);
    vm.runInContext(jsrender, context);
    const block = index.match(/<section class="game-details-section" aria-labelledby="game-details-dlc-heading">[\s\S]*?<\/section>/)?.[0];
    assert.ok(block, "DLC details section exists");
    const template = context.window.jsrender.templates(block);
    const rendered = template.render({
        remote_available: true,
        dlc: [
            {titleId: "0100000000011001", name: "Dated pack", attributes: {releaseDate: 20240102, ParsedReleaseDate: "2024-01-02"}},
            {titleId: "0100000000011002", name: "Undated pack", attributes: {ParsedReleaseDate: "0"}},
            {titleId: "0100000000011003", name: "Empty date", attributes: {}}
        ],
        local_only_dlc: [{titleId: "0100000000011004", name: "Local-only pack"}]
    });
    assert.match(rendered, /Title ID<\/th><th scope="col">Release date<\/th><th scope="col">Versions/);
    assert.match(rendered, /0100000000011001<\/code><\/td>\s*<td>2024-01-02<\/td>/);
    for (const id of ["0100000000011002", "0100000000011003"]) {
        assert.match(rendered, new RegExp(id + "</code></td>\\s*<td>Unknown</td>"));
    }
    const localOnly = rendered.slice(rendered.indexOf("Collected DLC absent from catalog"));
    assert.match(localOnly, /Local-only pack/);
    assert.doesNotMatch(localOnly, /Release date/);
});
