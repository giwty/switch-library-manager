import assert from "node:assert/strict";
import {readFile} from "node:fs/promises";
import test from "node:test";
import vm from "node:vm";
import {exportTableXLSX, reportFromExportList} from "../frontend/xlsx_export.js";

const appSource = await readFile(new URL("../frontend/app.js", import.meta.url), "utf8");
const tabulatorSource = await readFile(new URL("../frontend/lib/js/tabulator/tabulator.min.js", import.meta.url), "utf8");
const momentSource = await readFile(new URL("../frontend/lib/js/moment/moment.min.js", import.meta.url), "utf8");
const libraryContext = {console, navigator: {userAgent: ""}, window: {}, document: {}};
vm.runInNewContext(tabulatorSource, libraryContext);
vm.runInNewContext(momentSource, libraryContext);
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

test("Missing DLC sorts newest releases across games and exports dates with blanks for unknowns", async () => {
    const start = appSource.indexOf("        function loadTab(target) {");
    const end = appSource.indexOf('\n        $("body").on("click", ".folder-set"', start);
    let options;
    const ui = {addClass() {return this;}, attr() {return this;}, show() {return this;}, html() {return this;}, render() {return "";}};
    const context = {
        state: {
            settings: {paths: {library_folder: "synthetic"}}, library: {},
            dlc: [
                {Attributes: {id: "0100000000020000", name: "Beta"}, missing_dlc: [
                    {id: "0100000000021001", name: "Pack", release_date: "2024-01-02"},
                    {id: "0100000000021002", name: "Unknown", release_date: ""}
                ]},
                {Attributes: {id: "0100000000010000", name: "Alpha"}, missing_dlc: [
                    {id: "0100000000011004", name: "Oldest", release_date: "2023-12-31"},
                    {id: "0100000000011003", name: "Story", release_date: "2024-01-02"},
                    {id: "0100000000011002", name: "Expansion", release_date: "2024-01-02"},
                    {id: "0100000000011001", name: "Expansion", release_date: "2024-01-02"},
                    {id: "0100000000011005", name: "Unknown", release_date: ""}
                ]},
                {Attributes: {id: "0100000000030000", name: "Zeta"}, missing_dlc: [
                    {id: "0100000000031001", name: "Newest", release_date: "2026-10-28"}
                ]}
            ]
        },
        $: () => ui, hideCurrentTab() {}, gameDetailsTableOptions: value => value,
        document: {createElement: () => ({textContent: ""})},
        createTable(selector, value) {assert.equal(selector, "#dlc-table"); options = value;}
    };
    vm.createContext(context);
    vm.runInContext(appSource.slice(start, end), context);
    vm.runInContext('loadTab("#dlc")', context);

    const table = tableFor(options.columns, options.data);
    table.options = {...options, headerSort: false};
    table.rowManager.activeRows = table.rowManager.rows;
    table.columnManager.getRealColumns = () => table.columnManager.columns;
    table.columnManager.findColumn = field => table.columnManager.columns.find(value => value.definition.field === field);
    table.modules.sort = new modules.sort(table);
    for (const value of table.columnManager.columns) {
        value.modules = {};
        value.getField = () => value.definition.field;
        value.getElement = () => ({setAttribute() {}});
        table.modules.sort.initializeColumn(value, {});
    }
    table.modules.sort.setSort(options.initialSort);
    table.modules.sort.sort(table.rowManager.rows);
    assert.deepEqual(table.rowManager.rows.map(row => row.getData().dlc.id), [
        "0100000000031001", "0100000000011001", "0100000000011002", "0100000000011003",
        "0100000000021001", "0100000000011004", "0100000000011005", "0100000000021002"
    ]);

    const dateColumn = options.columns.find(value => value.field === "dlc.release_date");
    assert.equal(dateColumn.formatter({getValue: () => "2024-01-02"}).textContent, "2024-01-02");
    assert.equal(dateColumn.formatter({getValue: () => ""}).textContent, "Unknown");
    let report;
    await exportTableXLSX(table, "missing_dlc", async value => {report = value;});
    assert.deepEqual(report.headers, ["Missing DLC", "DLC ID", "Release date", "Title", "Title ID"]);
    assert.deepEqual(report.rows.map(row => row[2]), [
        "2026-10-28", "2024-01-02", "2024-01-02", "2024-01-02", "2024-01-02", "2023-12-31", "", ""
    ]);
    let csv;
    table.modules.download.downloaders.csv(table.modules.download.generateExportList("all"), {bom: true}, content => csv = content);
    assert.ok(csv.startsWith("\uFEFFMissing DLC,DLC ID,Release date,Title,Title ID"));
    assert.ok(csv.includes('"Newest","0100000000031001","2026-10-28","Zeta","0100000000030000"'));
    assert.ok(csv.includes('"Unknown","0100000000011005","","Alpha","0100000000010000"'));
    assert.equal(csv.includes('"Unknown","Alpha"'), false);

    for (const [dir, firstID] of [["asc", "0100000000011004"], ["desc", "0100000000031001"]]) {
        table.modules.sort.setSort("dlc.release_date", dir);
        table.modules.sort.sort(table.rowManager.rows);
        assert.equal(table.rowManager.rows[0].getData().dlc.id, firstID);
        assert.ok(table.rowManager.rows.slice(-2).every(row => row.getData().dlc.release_date === ""));
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
