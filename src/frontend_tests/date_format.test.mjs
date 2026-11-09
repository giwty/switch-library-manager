import assert from "node:assert/strict";
import {readFile} from "node:fs/promises";
import test from "node:test";
import vm from "node:vm";
import {dateFormatError, formatDate, dateColumnOptions} from "../frontend/date_format.js";

const cases = JSON.parse(await readFile(new URL("../backend/dateformat/testdata/cases.json", import.meta.url), "utf8"));
const context = {};
vm.runInNewContext(await readFile(new URL("../frontend/lib/js/moment/moment.min.js", import.meta.url), "utf8"), context);

test("Go and JavaScript share the same date format contract", () => {
    for (const {pattern, expected} of cases.formats) {
        assert.equal(dateFormatError(pattern), "", pattern);
        assert.equal(formatDate("2026-11-23", pattern, context.moment), expected, pattern);
    }
    for (const {date, pattern, expected} of cases.dates) assert.equal(formatDate(date, pattern, context.moment), expected, date);
    for (const pattern of cases.invalidPatterns) {
        assert.ok(dateFormatError(pattern), pattern);
        assert.throws(() => formatDate("2026-11-23", pattern, context.moment));
    }
    for (const date of [null, undefined, 20261123, {}, []]) assert.equal(formatDate(date, undefined, context.moment), "");
});

test("date columns format copies for downloads and retain ISO sorter inputs", () => {
    for (const {pattern, expected} of cases.formats) {
        const options = dateColumnOptions(pattern, context.moment);
        assert.equal(options.formatter({getValue: () => "2026-11-23"}), expected);
        assert.equal(options.accessorDownload("2026-11-23"), expected);
        assert.equal(options.formatter({getValue: () => "-"}), "Unknown");
        assert.equal(options.accessorDownload("-"), "");
        assert.equal(options.sorterParams.format, "YYYY-MM-DD");
        assert.equal(options.sorterParams.alignEmptyValues, "bottom");
    }
});

test("date formatting is independent of local timezone", () => {
    const previous = process.env.TZ;
    try {
        for (const timezone of ["Etc/GMT+12", "UTC", "Pacific/Kiritimati", "Europe/Stockholm"]) {
            process.env.TZ = timezone;
            for (const date of ["2026-03-29", "2026-10-25"]) assert.equal(formatDate(date, "YYYY-MM-DD", context.moment), date);
        }
    } finally {
        if (previous === undefined) delete process.env.TZ;
        else process.env.TZ = previous;
    }
});
