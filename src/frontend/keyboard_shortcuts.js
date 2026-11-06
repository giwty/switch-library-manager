export const TAB_SHORTCUTS = [
    {target: "#library", label: "Library"},
    {target: "#missing", label: "Missing games"},
    {target: "#updates", label: "Missing updates"},
    {target: "#dlc", label: "Missing DLC"},
    {target: "#organize", label: "Organize"},
    {target: "#status", label: "Issues"},
    {target: "#settings", label: "Settings"}
];

export const SHORTCUTS = [
    ...TAB_SHORTCUTS.map((tab, index) => ({
        id: "tab-" + (index + 1), label: "Show " + tab.label, target: tab.target,
        bindings: [{key: String(index + 1), primary: true}]
    })),
    {id: "organize", label: "Organize library (asks for confirmation)", bindings: [{key: "o", primary: true}]},
    {id: "rescan", label: "Rescan library", bindings: [{key: "r", primary: true, macControlAlias: true}]},
    {id: "hard-rescan", label: "Hard rescan library", bindings: [{key: "r", primary: true, shift: true, macControlAlias: true}]},
    {id: "search", label: "Focus the current table’s name filter", bindings: [{key: "f", primary: true}]},
    {id: "row-up", label: "Previous row (across pages)", rowOnly: true, repeat: true, bindings: [{key: "ArrowUp"}]},
    {id: "row-down", label: "Next row (across pages)", rowOnly: true, repeat: true, bindings: [{key: "ArrowDown"}]},
    {id: "open-details", label: "Open the focused game’s details", rowOnly: true, bindings: [{key: "Enter"}, {key: " "}]},
    {id: "row-menu", label: "Open row actions in Library, Missing updates or Missing DLC", rowOnly: true, bindings: [{key: "F10", shift: true}, {key: "ContextMenu"}]},
    {id: "dismiss", label: "Close details or shortcut help", dialogOnly: true, bindings: [{key: "Escape"}]},
    {id: "dialog-tab", label: "Move between dialog controls", dialogOnly: true, repeat: true, bindings: [{key: "Tab"}, {key: "Tab", shift: true}]},
    {id: "back", label: "Close game details and return to the row", bindings: [{key: "ArrowLeft", alt: true}, {key: "[", primary: true, macOnly: true}]},
    {id: "forward", label: "Reopen the last game details", bindings: [{key: "ArrowRight", alt: true}, {key: "]", primary: true, macOnly: true}]},
    {id: "help", label: "Show keyboard shortcuts", bindings: [{key: "F1"}]}
];

function matchesBinding(event, binding, isMacOS) {
    if ((binding.macOnly && !isMacOS) || event.key?.toLowerCase() !== binding.key.toLowerCase() ||
        Boolean(event.altKey) !== Boolean(binding.alt) || Boolean(event.shiftKey) !== Boolean(binding.shift)) {
        return false;
    }
    if (!binding.primary) {
        return !event.ctrlKey && !event.metaKey;
    }
    return isMacOS
        ? (event.metaKey && !event.ctrlKey) || (binding.macControlAlias && event.ctrlKey && !event.metaKey)
        : event.ctrlKey && !event.metaKey;
}

export function getShortcutAction(event, isMacOS) {
    if (event.isComposing || event.defaultPrevented) {
        return null;
    }
    return SHORTCUTS.find(shortcut => shortcut.bindings.some(binding => matchesBinding(event, binding, isMacOS))) || null;
}

export function registerKeyboardShortcuts(target, {isMacOS, getContext, onAction}) {
    const handleKeydown = event => {
        const shortcut = getShortcutAction(event, isMacOS());
        if (!shortcut) {
            return;
        }
        const context = getContext(event);
        if ((shortcut.rowOnly && !context.row) || (shortcut.dialogOnly && !context.dialog)) {
            return;
        }
        // Suppress WebView reload/open/find/history defaults even while commands are blocked.
        event.preventDefault();
        event.stopPropagation();
        if (context.blocked || (event.repeat && !shortcut.repeat)) {
            return;
        }
        if (context.dialog && shortcut.id !== "dismiss" && shortcut.id !== "dialog-tab" && !(context.dialog === "details" && shortcut.id === "back")) {
            return;
        }
        onAction(shortcut, context, event);
    };
    target.addEventListener("keydown", handleKeydown, true);
    return () => target.removeEventListener("keydown", handleKeydown, true);
}

const displayNames = {" ": "Space", Enter: "Enter", Tab: "Tab", Escape: "Esc", ArrowUp: "↑", ArrowDown: "↓", ArrowLeft: "←", ArrowRight: "→"};

function bindingKeys(binding, isMacOS, aria, controlAlias = false) {
    const keys = [];
    if (binding.primary) keys.push(isMacOS && !controlAlias ? (aria ? "Meta" : "Cmd") : (aria ? "Control" : "Ctrl"));
    if (binding.alt) keys.push("Alt");
    if (binding.shift) keys.push("Shift");
    keys.push(aria ? (binding.key === " " ? "Space" : binding.key) : (displayNames[binding.key] || binding.key.toUpperCase()));
    return keys.join("+");
}

export function shortcutKeys(id, isMacOS, aria = false) {
    const shortcut = SHORTCUTS.find(item => item.id === id);
    return shortcut.bindings.filter(binding => !binding.macOnly || isMacOS).flatMap(binding => {
        const keys = [bindingKeys(binding, isMacOS, aria)];
        if (isMacOS && binding.macControlAlias) keys.push(bindingKeys(binding, isMacOS, aria, true));
        return keys;
    }).join(aria ? " " : " / ");
}

export function shortcutHelp(isMacOS) {
    return SHORTCUTS.map(shortcut => ({id: shortcut.id, label: shortcut.label, keys: shortcutKeys(shortcut.id, isMacOS)}));
}
