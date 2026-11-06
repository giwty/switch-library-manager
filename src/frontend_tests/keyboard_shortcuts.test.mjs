import assert from "node:assert/strict";
import test from "node:test";
import {getShortcutAction, registerKeyboardShortcuts, TAB_SHORTCUTS, SHORTCUTS, shortcutHelp, shortcutKeys} from "../frontend/keyboard_shortcuts.js";

const event = (overrides = {}) => ({key: "r", ctrlKey: false, metaKey: false, shiftKey: false, altKey: false, ...overrides});
const action = (overrides, mac = false) => getShortcutAction(event(overrides), mac)?.id;

test("platform modifiers map all tabs and actions, retaining macOS Ctrl rescan aliases", () => {
    for (let n = 1; n <= 7; n++) {
        assert.equal(action({key: String(n), ctrlKey: true}), "tab-" + n);
        assert.equal(action({key: String(n), metaKey: true}, true), "tab-" + n);
        assert.equal(action({key: String(n)}), undefined);
    }
    for (const [key, id] of [["o", "organize"], ["f", "search"], ["r", "rescan"]]) {
        assert.equal(action({key, ctrlKey: true}), id);
        assert.equal(action({key: key.toUpperCase(), metaKey: true}, true), id);
        assert.equal(action({key, metaKey: true}), undefined);
    }
    assert.equal(action({ctrlKey: true}, true), "rescan");
    assert.equal(action({ctrlKey: true, shiftKey: true}, true), "hard-rescan");
    assert.equal(action({metaKey: true, shiftKey: true}, true), "hard-rescan");
    assert.equal(action({key: "o", ctrlKey: true}, true), undefined);
    assert.equal(TAB_SHORTCUTS[1].target, "#missing");
    assert.equal(TAB_SHORTCUTS[6].target, "#settings");
});

test("unrelated modifiers, IME input, and already handled events are ignored", () => {
    for (const overrides of [
        {}, {ctrlKey: true, altKey: true}, {ctrlKey: true, metaKey: true},
        {ctrlKey: true, key: "o", shiftKey: true}, {ctrlKey: true, isComposing: true},
        {ctrlKey: true, defaultPrevented: true}, {key: "F1", shiftKey: true}
    ]) assert.equal(getShortcutAction(event(overrides), false), null);
});

test("Back/Forward and dialog keys have exact platform bindings", () => {
    assert.equal(action({key: "ArrowLeft", altKey: true}), "back");
    assert.equal(action({key: "ArrowRight", altKey: true}, true), "forward");
    assert.equal(action({key: "[", metaKey: true}, true), "back");
    assert.equal(action({key: "]", metaKey: true}, true), "forward");
    assert.equal(action({key: "[", ctrlKey: true}), undefined);
    assert.equal(action({key: "Escape"}), "dismiss");
    assert.equal(action({key: "F1"}), "help");
    assert.equal(action({key: "F10", shiftKey: true}), "row-menu");
    assert.equal(action({key: "ContextMenu"}), "row-menu");
    assert.equal(action({key: "F10"}), undefined);
});

function registration() {
    let listener;
    let mac = false;
    let context = {blocked: false, dialog: null, row: null};
    const calls = [];
    const target = {
        addEventListener(type, fn, capture) {assert.equal(type, "keydown"); assert.equal(capture, true); listener = fn;},
        removeEventListener(type, fn, capture) {assert.equal(fn, listener); assert.equal(capture, true); listener = null;}
    };
    const unregister = registerKeyboardShortcuts(target, {
        isMacOS: () => mac, getContext: () => context, onAction: shortcut => calls.push(shortcut.id)
    });
    return {
        calls, unregister, setContext: value => context = {...context, ...value}, setMac: () => mac = true,
        dispatch(overrides) {
            let prevented = false;
            let stopped = false;
            listener({...event(overrides), preventDefault() {prevented = true;}, stopPropagation() {stopped = true;}});
            return {prevented, stopped};
        }
    };
}

test("commands suppress WebView defaults while busy and ignore held action keys", () => {
    const handler = registration();
    handler.dispatch({ctrlKey: true});
    handler.dispatch({ctrlKey: true, repeat: true});
    handler.setContext({blocked: true});
    assert.deepEqual(handler.dispatch({ctrlKey: true, key: "o"}), {prevented: true, stopped: true});
    assert.deepEqual(handler.calls, ["rescan"]);
    handler.unregister();
});

test("row keys leave form controls alone and permit repeated arrow navigation", () => {
    const handler = registration();
    assert.equal(handler.dispatch({key: "ArrowDown"}).prevented, false);
    assert.equal(handler.dispatch({key: " "}).prevented, false);
    handler.setContext({row: {}});
    handler.dispatch({key: "ArrowDown", repeat: true});
    handler.dispatch({key: "Enter"});
    handler.dispatch({key: "Enter", repeat: true});
    handler.dispatch({key: " ", ctrlKey: true});
    assert.deepEqual(handler.calls, ["row-down", "open-details"]);
});

test("open dialogs allow dismissal and detail Back while blocking other commands", () => {
    const handler = registration();
    handler.setContext({dialog: "details"});
    handler.dispatch({ctrlKey: true, key: "1"});
    handler.dispatch({ctrlKey: true});
    handler.dispatch({key: "F1"});
    handler.dispatch({key: "ArrowLeft", altKey: true});
    handler.dispatch({key: "Escape"});
    handler.setContext({dialog: "help"});
    handler.dispatch({key: "ArrowLeft", altKey: true});
    handler.dispatch({key: "Tab"});
    handler.dispatch({key: "Tab", shiftKey: true});
    handler.dispatch({key: "Escape"});
    assert.deepEqual(handler.calls, ["back", "dismiss", "dialog-tab", "dialog-tab", "dismiss"]);
});

test("platform detection remains live after startup", () => {
    const handler = registration();
    assert.equal(handler.dispatch({key: "o", metaKey: true}).prevented, false);
    handler.setMac();
    handler.dispatch({key: "o", metaKey: true});
    assert.deepEqual(handler.calls, ["organize"]);
});

test("help and accessibility metadata describe every registered shortcut", () => {
    assert.equal(shortcutHelp(false).length, SHORTCUTS.length);
    assert.equal(shortcutHelp(true).find(item => item.id === "organize").keys, "Cmd+O");
    assert.equal(shortcutKeys("rescan", true, true), "Meta+r Control+r");
    assert.equal(shortcutKeys("hard-rescan", false, true), "Control+Shift+r");
    assert.equal(shortcutKeys("tab-7", true, true), "Meta+7");
});
