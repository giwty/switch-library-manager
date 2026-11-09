export const DEFAULT_DATE_FORMAT = "YYYY-MM-DD";
export const DATE_FORMAT_PRESETS = [DEFAULT_DATE_FORMAT, "MM/DD/YYYY", "DD/MM/YYYY"];

const trim = value => String(value ?? "").replace(/^[ \t\r\n\v\f]+|[ \t\r\n\v\f]+$/g, "");

export function normalizeDateFormat(pattern) {
    return trim(pattern) || DEFAULT_DATE_FORMAT;
}

export function dateFormatError(pattern) {
    const normalized = normalizeDateFormat(pattern);
    const parts = normalized.match(/^(YYYY|MMMM|MMM|MM|M|DD|D)[ /.,-]+(YYYY|MMMM|MMM|MM|M|DD|D)[ /.,-]+(YYYY|MMMM|MMM|MM|M|DD|D)$/);
    if (normalized.length <= 32 && parts && new Set(parts.slice(1).map(token => token[0])).size === 3) return "";
    return "Use one YYYY, one M/MM/MMM/MMMM and one D/DD separated by spaces or / - . , (maximum 32 characters).";
}

export function formatDate(value, pattern, momentLib = globalThis.moment) {
    const error = dateFormatError(pattern);
    if (error) throw new Error(error);
    if (typeof value !== "string" || !/^[0-9]{4}-[0-9]{2}-[0-9]{2}$/.test(value)) return "";
    const date = momentLib(value, "YYYY-MM-DD", "en", true);
    return date.isValid() ? date.locale("en").format(normalizeDateFormat(pattern)) : "";
}

// Download accessors operate on a copy; sorting and domain data retain ISO values.
export function dateColumnOptions(pattern, momentLib = globalThis.moment) {
    return {
        sorter: "date",
        sorterParams: {format: DEFAULT_DATE_FORMAT, alignEmptyValues: "bottom"},
        // Validated patterns and parsed dates can only produce plain date text.
        formatter: cell => formatDate(cell.getValue(), pattern, momentLib) || "Unknown",
        accessorDownload: value => formatDate(value, pattern, momentLib)
    };
}
