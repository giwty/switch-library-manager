import assert from "node:assert/strict";
import {readFile} from "node:fs/promises";
import test from "node:test";
import vm from "node:vm";
import {exportTableXLSX, reportFromExportList} from "../frontend/xlsx_export.js";

const appSource = await readFile(new URL("../frontend/app.js", import.meta.url), "utf8");
const tabulatorSource = await readFile(new URL("../frontend/lib/js/tabulator/tabulator.min.js", import.meta.url), "utf8");
const libraryContext = {console, navigator: {userAgent: ""}, window: {}, document: {}};
vm.runInNewContext(tabulatorSource, libraryContext);
const modules = libraryContext.Tabulator.prototype.moduleBindings;

function column(definition) {
    const column = {
        definition, columns: [], visible: definition.visible !== false,
        getComponent() { return {_column: column}; },
        getFieldValue(data) { return definition.field?.split(".").reduce((value, key) => value?.[key], data); }
    };
    return column;
}

function tableFor(definitions, data) {
    const columns = definitions.map(column);
    const rows = data.map(value => ({type: "row", getData: () => value, getComponent: () => ({})}));
    const table = {
        options: {},
        columnManager: {columns, columnsByIndex: columns},
        rowManager: {rows, getDisplayRows: () => rows.slice(0, 1)},
        modExists: () => false,
        modules: {},
    };
    table.modules.export = new modules.export(table);
    table.modules.download = new modules.download(table);
    table.download = (...args) => table.modules.download.download(...args);
    return table;
}

test("XLSX uses the bundled Tabulator export path for all rows and CSV column selection", async () => {
    const table = tableFor([
        {formatter: "rownum", download: false},
        {field: "icon", download: false},
        {title: "Title", field: "name"},
        {title: "Title ID", field: "titleId"},
        {title: "Update", field: "update"},
        {title: "Version", field: "version", visible: false, download: true}
    ], [
        {name: "日本語 café åäö", titleId: "0100000000001000", update: 131072, version: "1.02", icon: "ignored"},
        {name: "=literal", titleId: "0012345678901234", update: 65536, version: "+1", icon: "ignored"}
    ]);
    const expected = [
        ["日本語 café åäö", "0100000000001000", 131072, "1.02"],
        ["=literal", "0012345678901234", 65536, "+1"]
    ];
    for (const kind of ["games", "missing_games", "missing_updates", "missing_dlc", "issues"]) {
        let request;
        const saved = await exportTableXLSX(table, kind, async value => { request = value; return "report.xlsx"; });
        assert.equal(saved, "report.xlsx");
        assert.equal(request.kind, kind);
        assert.deepEqual(request.headers, ["Title", "Title ID", "Update", "Version"]);
        assert.deepEqual(JSON.parse(JSON.stringify(request.rows)), expected);
    }
    let csv;
    table.modules.download.downloaders.csv(table.modules.download.generateExportList("all"), {bom: true}, content => csv = content);
    assert.ok(csv.startsWith("\uFEFFTitle,Title ID,Update,Version"));
    assert.ok(csv.includes('"=literal","0012345678901234","65536","+1"'));
});

test("all five GUI tabs supply the correct report kind to the export adapter", () => {
    const start = appSource.indexOf("        function loadTab(target) {");
    const end = appSource.indexOf('\n        $("body").on("click", ".folder-set"', start);
    const captured = [];
    const ui = {addClass() {return this;}, attr() {return this;}, show() {return this;}, html() {return this;}, render() {return "";}};
    const state = {
        settings: {paths: {library_folder: "synthetic"}, gui: {}},
        library: {library_data: [{name: "test"}], issues: [{key: "test", value: "issue"}]},
        updates: [{Attributes: {name: "test"}}],
        dlc: [{Attributes: {name: "test"}, missing_dlc: [{name: "expansion"}]}],
        missingGames: [{name: "test"}]
    };
    const context = {state, $: () => ui, hideCurrentTab() {}, gameDetailsTableOptions: value => value, tableOptions: value => value,
        createTable(selector, options, kind) {captured.push({selector, options, kind});}};
    vm.createContext(context);
    vm.runInContext(appSource.slice(start, end), context);
    for (const [tab, kind] of [["#library", "games"], ["#missing", "missing_games"], ["#updates", "missing_updates"], ["#dlc", "missing_dlc"], ["#status", "issues"]]) {
        vm.runInContext(`loadTab(${JSON.stringify(tab)})`, context);
        assert.equal(captured.at(-1).kind, kind);
        assert.ok(captured.at(-1).options.columns.length);
    }
});

test("XLSX rejects unsupported data instead of serializing formulas or objects", () => {
    assert.throws(() => reportFromExportList("games", [{type: "group", columns: []}]), /unsupported table row/);
    assert.throws(() => reportFromExportList("games", [{type: "header", columns: [{depth: 2, value: "Group"}]}]), /flat table header/);
    assert.throws(() => reportFromExportList("games", [{type: "row", columns: [{value: {html: "test"}}]}]), /unsupported cell value/);
    assert.deepEqual(reportFromExportList("games", [{type: "row", columns: [{value: undefined}, {value: false}]}]).rows, [[null, false]]);
});

test("export click waits for save, prevents duplicate exports, and restores controls on cancellation and error", async () => {
    const start = appSource.indexOf('        $("body").on("click", ".export-xlsx-btn", async () => {');
    const end = appSource.indexOf("\n        });", start) + "\n        });".length;
    assert.ok(start >= 0);
    let handler, finish;
    const button = {disabled: false};
    const errors = [];
    let calls = 0;
    const context = {
        currTable: {}, currTableExportKind: "games", xlsxExportPending: false,
        document: {querySelectorAll: () => [button]},
        $: () => ({on(event, selector, callback) { handler = callback; }}),
        exportTableXLSX: () => { calls++; return new Promise((resolve, reject) => finish = {resolve, reject}); },
        ExportXLSX() {}, showError: error => errors.push(error)
    };
    vm.runInNewContext(appSource.slice(start, end), context);
    const first = handler();
    assert.equal(button.disabled, true);
    await handler();
    assert.equal(calls, 1);
    finish.resolve("");
    await first;
    assert.equal(button.disabled, false);
    assert.equal(context.xlsxExportPending, false);
    assert.deepEqual(errors, []);
    const second = handler();
    finish.reject("disk full");
    await second;
    assert.equal(button.disabled, false);
    assert.deepEqual(errors, ["disk full"]);
});
