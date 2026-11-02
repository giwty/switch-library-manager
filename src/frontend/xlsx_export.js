// Keep the exact columns and raw values selected by Tabulator for CSV downloads.
export function reportFromExportList(kind, list) {
    const headers = [];
    const rows = [];
    for (const row of list) {
        if (row.type === "header") {
            for (const column of row.columns) {
                if (!column || column.depth !== 1) {
                    throw new Error("Excel export requires a flat table header.");
                }
                headers.push(String(column.value ?? ""));
            }
        } else if (row.type === "row") {
            rows.push(row.columns.filter(column => column !== null).map(column => {
                const value = column.value ?? null;
                if (value !== null && !["string", "number", "boolean"].includes(typeof value)) {
                    throw new Error("Excel export encountered an unsupported cell value.");
                }
                return value;
            }));
        } else {
            throw new Error("Excel export encountered an unsupported table row.");
        }
    }
    return {kind, headers, rows};
}

export async function exportTableXLSX(table, kind, saveReport) {
    let report;
    table.download(list => {
        report = reportFromExportList(kind, list);
    }, "", {}, "all");
    if (!report) {
        throw new Error("The table could not be exported to Excel.");
    }
    return saveReport(report);
}
