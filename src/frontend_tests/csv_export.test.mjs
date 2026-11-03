import assert from "node:assert/strict";
import {readFile} from "node:fs/promises";
import test from "node:test";
import vm from "node:vm";

const appSource = await readFile(new URL("../frontend/app.js", import.meta.url), "utf8");
const tabulatorSource = await readFile(new URL("../frontend/lib/js/tabulator/tabulator.min.js", import.meta.url), "utf8");
const handlerStart = appSource.indexOf('        $("body").on("click", ".export-btn", e => {');
const handlerEndMarker = "\n        });";
const handlerEnd = appSource.indexOf(handlerEndMarker, handlerStart);
assert.notEqual(handlerStart, -1, "CSV export click handler exists");
assert.notEqual(handlerEnd, -1, "CSV export click handler boundary exists");
const exportHandlerSource = appSource.slice(handlerStart, handlerEnd + handlerEndMarker.length);

test("GUI CSV exports pass BOM option to Tabulator and preserve Unicode", () => {
    const tabulatorContext = {
        console,
        navigator: {userAgent: ""},
        window: {},
        document: {}
    };
    vm.runInNewContext(tabulatorSource, tabulatorContext);
    const downloadCsv = tabulatorContext.Tabulator.prototype.moduleBindings.download.prototype.downloaders.csv;
    const rows = [
        {type: "header", columns: [{depth: 1, value: "Title"}]},
        {type: "row", columns: [{value: "日本語 café åäö"}]}
    ];

    let exportClick;
    let downloaded;
    const table = {
        download(type, filename, options, range) {
            const contents = [];
            downloadCsv(rows, options, (data, mime) => contents.push({data, mime}));
            downloaded = {type, filename, options, range, contents};
        }
    };
    const context = {
        currTable: table,
        currTableExportKind: "missing_games",
        exportDate: () => "2026-10-26",
        $: () => ({
            on(event, selector, handler) {
                assert.equal(event, "click");
                assert.equal(selector, ".export-btn");
                exportClick = handler;
            }
        })
    };
    vm.runInNewContext(exportHandlerSource, context);
    assert.equal(typeof exportClick, "function", "CSV export handler is registered");
    exportClick({});

    assert.equal(downloaded.type, "csv");
    assert.equal(downloaded.filename, "slm_missing_games.2026-10-26.csv");
    assert.equal(downloaded.options.bom, true);
    assert.equal(downloaded.range, "all");
    assert.equal(downloaded.contents.length, 1);
    assert.equal(downloaded.contents[0].mime, "text/csv");

    const bytes = Buffer.from(downloaded.contents[0].data, "utf8");
    const bom = Buffer.from([0xEF, 0xBB, 0xBF]);
    assert.ok(bytes.subarray(0, 3).equals(bom), "CSV starts with UTF-8 BOM bytes");
    assert.ok(!bytes.subarray(3, 6).equals(bom), "CSV has only one leading BOM");
    assert.ok(downloaded.contents[0].data.includes("日本語 café åäö"), "CSV keeps Japanese and accented text");
});
