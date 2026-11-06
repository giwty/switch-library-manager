// App-owned menu: Tabulator's bundled menu does not support keyboard focus.
export function createRowContextMenu({document, window, isBlocked, onAction}) {
    let menu, origin;

    function close(restoreFocus = true) {
        if (!menu) return;
        const hadFocus = menu.contains(document.activeElement);
        menu.remove();
        menu = null;
        document.removeEventListener("pointerdown", outside, true);
        document.removeEventListener("contextmenu", outside, true);
        document.removeEventListener("focusin", outside, true);
        document.removeEventListener("scroll", dismiss, true);
        document.removeEventListener("keydown", keydown, true);
        window.removeEventListener("resize", dismiss);
        window.removeEventListener("blur", blur);
        if (restoreFocus && hadFocus && origin?.isConnected) origin.focus({preventScroll: true});
        origin = null;
    }

    function outside(event) {if (!menu?.contains(event.target)) close(false);}
    function dismiss() {close();}
    function blur() {close(false);}
    function keydown(event) {
        if (!menu) return;
        const buttons = Array.from(menu.querySelectorAll("button"));
        const index = buttons.indexOf(document.activeElement);
        if (["ArrowDown", "ArrowUp", "Home", "End", "Enter", " ", "Escape", "Tab"].includes(event.key) &&
            !event.ctrlKey && !event.metaKey && !event.altKey) {
            event.preventDefault();
            event.stopImmediatePropagation();
            if (event.key === "Escape" || event.key === "Tab") close();
            else if (event.key === "Enter" || event.key === " ") {
                if (!event.repeat) buttons[index]?.click();
            } else {
                const next = event.key === "Home" ? 0 : event.key === "End" ? buttons.length - 1
                    : (index + (event.key === "ArrowDown" ? 1 : -1) + buttons.length) % buttons.length;
                buttons[next]?.focus();
            }
        }
    }

    function open(event, row, actions) {
        if (isBlocked() || !actions?.length) return;
        event.preventDefault();
        event.stopPropagation();
        close(false);
        origin = row.getElement();
        origin.focus({preventScroll: true});
        menu = document.createElement("div");
        menu.className = "row-context-menu";
        menu.setAttribute("role", "menu");
        menu.setAttribute("aria-label", "Row actions");
        for (const action of actions) {
            const button = document.createElement("button");
            button.type = "button";
            button.setAttribute("role", "menuitem");
            button.tabIndex = -1;
            button.textContent = action.label;
            button.addEventListener("click", event => {
                event.stopPropagation();
                const rowElement = origin;
                close();
                if (!isBlocked()) onAction(action, rowElement);
            });
            menu.append(button);
        }
        document.body.append(menu);
        const rect = origin.getBoundingClientRect();
        const pointer = event.type === "contextmenu" && (event.clientX !== 0 || event.clientY !== 0);
        const x = pointer ? event.clientX : rect.left;
        const y = pointer ? event.clientY : rect.bottom;
        const margin = 8;
        menu.style.left = Math.max(margin, Math.min(x, window.innerWidth - menu.offsetWidth - margin)) + "px";
        menu.style.top = Math.max(margin, Math.min(y, window.innerHeight - menu.offsetHeight - margin)) + "px";
        document.addEventListener("pointerdown", outside, true);
        document.addEventListener("contextmenu", outside, true);
        document.addEventListener("focusin", outside, true);
        document.addEventListener("scroll", dismiss, true);
        document.addEventListener("keydown", keydown, true);
        window.addEventListener("resize", dismiss);
        window.addEventListener("blur", blur);
        menu.querySelector("button").focus({preventScroll: true});
    }

    return {open, close};
}
