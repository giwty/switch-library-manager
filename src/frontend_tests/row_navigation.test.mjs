import assert from "node:assert/strict";
import test from "node:test";
import {createRowNavigation} from "../frontend/row_navigation.js";

function fixture() {
    let page = 0;
    let current = true;
    let focused;
    let gate;
    const ownerDocument = {activeElement: null, body: {}};
    const all = Array.from({length: 5}, (_, index) => {
        const element = {tabIndex: -1, focus() {focused = index; ownerDocument.activeElement = this;}};
        return {
            getElement: () => element,
            async pageTo() {if (gate) await gate; page = Math.floor(rows.indexOf(this) / 2); nav.sync();},
            async scrollTo() {}
        };
    });
    let rows = all;
    const table = {element: {ownerDocument, contains: element => Boolean(element.isConnected)}, getRows: type => type === "display" ? rows.slice(page * 2, page * 2 + 2) : rows};
    const nav = createRowNavigation(table, () => current);
    nav.sync();
    return {nav, all, table, get focused() {return focused;}, get page() {return page;},
        invalidate() {current = false;}, setGate(value) {gate = value;},
        filterSort(value) {rows = value; page = 0; nav.sync();}};
}

test("tables have one row tab stop and remember pointer or keyboard focus", () => {
    const f = fixture();
    assert.deepEqual(f.all.slice(0, 2).map(row => row.getElement().tabIndex), [0, -1]);
    f.nav.remember(f.all[1].getElement());
    assert.deepEqual(f.all.slice(0, 2).map(row => row.getElement().tabIndex), [-1, 0]);
    assert.equal(f.nav.focusFirst(), true);
    assert.equal(f.focused, 1);
});

test("redrawing preserves row focus without taking focus from filter controls", () => {
    const f = fixture();
    f.nav.focusFirst();
    f.table.element.ownerDocument.activeElement = f.table.element.ownerDocument.body;
    f.nav.sync();
    assert.equal(f.table.element.ownerDocument.activeElement, f.all[0].getElement());
    const input = {isConnected: true, focus() {f.table.element.ownerDocument.activeElement = this;}};
    f.table.element.ownerDocument.activeElement = input;
    f.nav.releaseFocus(input);
    f.nav.sync();
    assert.equal(f.table.element.ownerDocument.activeElement, input);
    f.table.element.ownerDocument.activeElement = f.table.element.ownerDocument.body;
    f.nav.sync();
    assert.equal(f.table.element.ownerDocument.activeElement, input);
});

test("arrows cross pages in both directions and stop at the dataset boundaries", async () => {
    const f = fixture();
    await f.nav.move(f.all[0].getElement(), -1);
    assert.equal(f.focused, undefined);
    for (let n = 0; n < 4; n++) await f.nav.move(f.all[n].getElement(), 1);
    assert.equal(f.page, 2);
    assert.equal(f.focused, 4);
    await f.nav.move(f.all[4].getElement(), 1);
    assert.equal(f.focused, 4);
    await f.nav.move(f.all[4].getElement(), -1);
    assert.equal(f.page, 1);
    assert.equal(f.focused, 3);
    assert.equal(f.all[4].getElement().tabIndex, -1);
    assert.equal(f.all[3].getElement().tabIndex, 0);
});

test("navigation follows filtered and sorted rows rather than underlying data order", async () => {
    const f = fixture();
    f.filterSort([f.all[4], f.all[2], f.all[0]]);
    f.nav.focusFirst();
    assert.equal(f.focused, 4);
    await f.nav.move(f.all[4].getElement(), 1);
    assert.equal(f.focused, 2);
    await f.nav.move(f.all[2].getElement(), 1);
    assert.equal(f.focused, 0);
    assert.equal(f.page, 1);
});

test("empty tables and removed rows do not capture focus", async () => {
    const f = fixture();
    f.filterSort([]);
    assert.equal(f.nav.focusFirst(), false);
    await f.nav.move(f.all[0].getElement(), 1);
    assert.equal(f.focused, undefined);
});

test("pending movement cannot focus a destroyed or superseded table", async () => {
    const f = fixture();
    let release;
    f.setGate(new Promise(resolve => release = resolve));
    const move = f.nav.move(f.all[0].getElement(), 1);
    f.invalidate();
    release();
    await move;
    assert.equal(f.focused, undefined);
});

test("held arrows do not start overlapping page changes", async () => {
    const f = fixture();
    let release;
    f.setGate(new Promise(resolve => release = resolve));
    const move = f.nav.move(f.all[0].getElement(), 1);
    await f.nav.move(f.all[0].getElement(), 1);
    release();
    await move;
    assert.equal(f.focused, 1);
});
