// Tabulator owns filtering, sorting and pagination. Keep focus on its row components,
// including when several DLC rows refer to the same game.
export function createRowNavigation(table, isCurrent) {
    let focusedRow;
    let moving = false;
    let ownsFocus = false;
    let focusedControl = null;
    const ownerDocument = table.element.ownerDocument;

    function sync() {
        if (!isCurrent()) return;
        const rows = table.getRows("display").filter(row => row.getElement());
        const next = rows.includes(focusedRow) ? focusedRow : rows[0];
        const previous = focusedRow?.getElement();
        if (previous && focusedRow !== next) previous.tabIndex = -1;
        focusedRow = next;
        rows.forEach(row => row.getElement().tabIndex = row === focusedRow ? 0 : -1);
        if (ownsFocus && focusedRow && ownerDocument.activeElement === ownerDocument.body) {
            focusedRow.getElement().focus();
        } else if (focusedControl?.isConnected && ownerDocument.activeElement === ownerDocument.body) {
            focusedControl.focus({preventScroll: true});
        }
    }

    function rowForElement(element) {
        return table.getRows("display").find(row => row.getElement() === element);
    }

    function remember(element) {
        const row = rowForElement(element);
        if (!row || !isCurrent()) return;
        const previous = focusedRow?.getElement();
        if (previous) previous.tabIndex = -1;
        focusedRow = row;
        ownsFocus = true;
        focusedControl = null;
        row.getElement().tabIndex = 0;
    }

    function focusFirst() {
        if (!isCurrent()) return false;
        sync();
        if (!focusedRow) return false;
        ownsFocus = true;
        focusedControl = null;
        focusedRow.getElement().focus();
        return true;
    }

    async function move(element, direction) {
        if (moving || !isCurrent()) return;
        const row = rowForElement(element);
        const rows = table.getRows("active");
        const index = rows.indexOf(row);
        const next = index < 0 ? undefined : rows[index + direction];
        if (!next) return;
        moving = true;
        try {
            await next.pageTo();
            if (!isCurrent()) return;
            await next.scrollTo();
            if (!isCurrent()) return;
            remember(next.getElement());
            next.getElement().focus();
        } catch (error) {
            if (isCurrent()) throw error;
        } finally {
            moving = false;
        }
    }

    // Tabulator's virtual redraws can detach focused rows without firing its
    // render callbacks. Restore focus after reattachment, unless the user has
    // moved to a filter, dialog, or another control in the meantime. Preserve
    // focused filter inputs too when a header is detached and reattached.
    const Observer = ownerDocument.defaultView?.MutationObserver;
    const observer = Observer ? new Observer(sync) : null;
    observer?.observe(table.element, {childList: true, subtree: true});
    return {
        sync, remember, focusFirst, move,
        releaseFocus(element) {
            ownsFocus = false;
            focusedControl = element && table.element.contains(element) ? element : null;
        },
        destroy() {observer?.disconnect();}
    };
}
