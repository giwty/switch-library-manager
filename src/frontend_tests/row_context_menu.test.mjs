import assert from "node:assert/strict";
import test from "node:test";
import {createRowContextMenu} from "../frontend/row_context_menu.js";

function fixture() {
    const calls = [];
    let blocked = false;
    function surface() {
        const listeners = new Map();
        return {addEventListener(type, fn) {if (!listeners.has(type)) listeners.set(type, new Set()); listeners.get(type).add(fn);},
            removeEventListener(type, fn) {listeners.get(type)?.delete(fn);},
            dispatch(type, event = {}) {for (const fn of [...(listeners.get(type) || [])]) fn(event);},
            listenerCount() {return [...listeners.values()].reduce((sum, entries) => sum + entries.size, 0);}};
    }
    const document = {...surface(), activeElement: null};
    function element(tag) {
        return {...surface(), tag, style: {}, attrs: {}, children: [], isConnected: true,
            offsetWidth: 200, offsetHeight: 80,
            setAttribute(key, value) {this.attrs[key] = value;},
            append(child) {this.children.push(child); child.parent = this;},
            contains(child) {return child === this || this.children.some(item => item.contains(child));},
            remove() {this.parent.children = this.parent.children.filter(child => child !== this); this.isConnected = false;},
            focus() {document.activeElement = this;},
            querySelectorAll() {return this.children;}, querySelector() {return this.children[0];},
            click() {this.dispatch("click", {stopPropagation() {}});},
            getBoundingClientRect: () => ({left: 20, bottom: 90})};
    }
    document.body = element("body");
    document.createElement = element;
    const window = {...surface(), innerWidth: 320, innerHeight: 240};
    const rowElement = element("row");
    const row = {getElement: () => rowElement};
    const controller = createRowContextMenu({document, window, isBlocked: () => blocked,
        onAction(action, origin) {calls.push({action, origin});}});
    const event = overrides => ({type: "contextmenu", clientX: 315, clientY: 235,
        preventDefault() {this.prevented = true;}, stopPropagation() {}, stopImmediatePropagation() {}, ...overrides});
    return {controller, row, rowElement, document, window, calls, event, setBlocked(value) {blocked = value;},
        menu: () => document.body.children[0], key(key, extra) {const e = event({key, ...extra}); document.dispatch("keydown", e); return e;}};
}

test("menu uses safe text, stays within narrow viewport edges and targets the clicked row", () => {
    const f = fixture();
    const action = {label: "Ignore <DLC>", titleID: "synthetic"};
    f.controller.open(f.event(), f.row, [action]);
    const menu = f.menu();
    assert.equal(menu.style.left, "112px");
    assert.equal(menu.style.top, "152px");
    assert.equal(menu.attrs.role, "menu");
    assert.equal(menu.children[0].textContent, "Ignore <DLC>");
    assert.equal(f.document.activeElement, menu.children[0]);
    menu.children[0].click();
    assert.deepEqual(f.calls, [{action, origin: f.rowElement}]);
    assert.equal(f.document.activeElement, f.rowElement);
    assert.equal(f.menu(), undefined);
    assert.equal(f.document.listenerCount() + f.window.listenerCount(), 0);
});

test("keyboard opens beside the row, navigates, selects once and restores focus", () => {
    const f = fixture();
    const actions = [{label: "First"}, {label: "Second"}];
    f.controller.open(f.event({type: "keydown"}), f.row, actions);
    assert.equal(f.menu().style.left, "20px");
    assert.equal(f.menu().style.top, "90px");
    f.key("ArrowDown");
    assert.equal(f.document.activeElement, f.menu().children[1]);
    f.key("ArrowDown");
    assert.equal(f.document.activeElement, f.menu().children[0]);
    f.key("End");
    f.key("Enter", {repeat: true});
    assert.equal(f.calls.length, 0);
    f.key("Enter");
    assert.equal(f.calls[0].action, actions[1]);
    assert.equal(f.document.activeElement, f.rowElement);
});

test("Escape, Tab, scrolling, resize and destruction dismiss without selecting", () => {
    for (const dismiss of [f => f.key("Escape"), f => f.key("Tab"), f => f.document.dispatch("scroll"),
        f => f.window.dispatch("resize"), f => f.controller.close(false)]) {
        const f = fixture();
        f.controller.open(f.event(), f.row, [{label: "Ignore"}]);
        dismiss(f);
        assert.equal(f.menu(), undefined);
        assert.equal(f.calls.length, 0);
        assert.equal(f.document.listenerCount() + f.window.listenerCount(), 0);
    }
});

test("outside clicks, context clicks and keyboard focus dismiss without stealing focus", () => {
    for (const type of ["pointerdown", "contextmenu", "focusin"]) {
        const f = fixture();
        f.controller.open(f.event(), f.row, [{label: "Ignore"}]);
        const outside = {};
        f.document.activeElement = outside;
        f.document.dispatch(type, {target: outside});
        assert.equal(f.menu(), undefined);
        assert.equal(f.document.activeElement, outside);
    }
});

test("busy states block opening and also recheck before performing a menu action", () => {
    const f = fixture();
    f.setBlocked(true);
    f.controller.open(f.event(), f.row, [{label: "Ignore"}]);
    assert.equal(f.menu(), undefined);
    f.setBlocked(false);
    f.controller.open(f.event(), f.row, [{label: "Ignore"}]);
    f.setBlocked(true);
    f.key("Enter");
    assert.equal(f.calls.length, 0);
    assert.equal(f.menu(), undefined);
});
