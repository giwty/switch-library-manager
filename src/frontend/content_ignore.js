const listNames = {updates: "ignore_update_title_ids", dlc: "ignore_dlc_title_ids"};

export function applyIgnoreEntry(ids, titleID, ignored) {
    const normalized = titleID.toLowerCase();
    const result = (ids || []).filter(id => id.toLowerCase() !== normalized);
    if (ignored) result.push(normalized);
    return result;
}

export function contentIgnoreAction(data, tableKind, missingSettings) {
    let kind, titleID;
    if (tableKind === "games") {
        kind = "updates";
        titleID = data.titleId;
    } else if (tableKind === "missing_updates") {
        kind = "updates";
        titleID = data.Attributes?.id;
    } else if (tableKind === "missing_dlc") {
        kind = "dlc";
        titleID = data.dlc?.id;
    }
    if (!/^[0-9a-f]{16}$/i.test(titleID || "")) return null;
    titleID = titleID.toLowerCase();
    const alreadyIgnored = (missingSettings?.[listNames[kind]] || []).some(id => id.toLowerCase() === titleID);
    const ignored = tableKind === "games" ? !alreadyIgnored : true;
    const label = kind === "dlc" ? "Ignore DLC" : ignored ? "Ignore updates" : "Resume checking updates";
    return {kind, titleID, ignored, label};
}

export function mergeIgnoreDraft(draft, action) {
    if (!draft) return;
    const name = listNames[action.kind];
    draft.missing_content[name] = applyIgnoreEntry(draft.missing_content[name], action.titleID, action.ignored);
}
