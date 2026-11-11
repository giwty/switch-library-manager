export function convertNameCase(name, style) {
    if (style === "upper") return name.toUpperCase();
    const lower = name.toLowerCase();
    if (style === "lower") return lower;
    const capitalize = word => word.replace(/\p{L}/u, letter => letter.toUpperCase());
    return style === "title" ? lower.replace(/\S+/gu, capitalize) : capitalize(lower);
}

export function gameNameAction(data, kind) {
    const id = kind === "missing_dlc" ? data.game?.id
        : kind === "missing_updates" ? data.Attributes?.id : data.titleId;
    if (!/^[0-9a-f]{16}$/i.test(id || "")) return null;
    // The backend resolves update and DLC IDs to their owning game.
    return {type: "game-name", titleID: id.toLowerCase(), label: "Edit game name…"};
}

export function createGameNameEditor({document, load, save, onSaved, onBusyChange, restoreFocus}) {
    const dialog = document.getElementById("game-name-dialog");
    const form = dialog.querySelector("form");
    const input = form.elements.name;
    const fieldset = form.querySelector("fieldset");
    const message = dialog.querySelector("[data-name-message]");
    const closeButtons = dialog.querySelectorAll("[data-name-close]");
    let request = 0, details, origin, removing = false, pending = false;

    function notify(text) { message.textContent = text; message.hidden = !text; }
    function populate(value) {
        details = value;
        input.value = value.name;
        removing = false;
        dialog.querySelector("[data-name-id]").textContent = value.titleId;
        dialog.querySelector("[data-name-automatic]").textContent = value.automatic_name;
        form.querySelector("[data-name-restore]").disabled = !value.custom_name;
    }
    function close() { if (!pending) dialog.close(); }
    closeButtons.forEach(button => button.addEventListener("click", close));
    dialog.addEventListener("cancel", event => { event.preventDefault(); close(); });
    dialog.addEventListener("close", () => {
        if (dialog.open) return;
        request++;
        restoreFocus(origin, details?.titleId);
        origin = details = null;
    });
    input.addEventListener("input", () => { removing = false; notify(""); });
    form.querySelectorAll("[data-name-case]").forEach(button => button.addEventListener("click", () => {
        input.value = convertNameCase(input.value, button.dataset.nameCase);
        removing = false;
        notify("");
        input.focus();
    }));
    form.querySelector("[data-name-restore]").addEventListener("click", () => {
        input.value = details.automatic_name;
        removing = true;
        notify("");
    });
    form.addEventListener("submit", async event => {
        event.preventDefault();
        if (pending || fieldset.disabled || !details) return;
        if (!removing && (!input.value.trim() || /\p{Cc}/u.test(input.value))) {
            notify("Enter a name without control characters, or use Restore automatic name.");
            input.focus();
            return;
        }
        pending = true;
        fieldset.disabled = true;
        closeButtons.forEach(button => button.disabled = true);
        onBusyChange(true);
        notify("");
        let saved = false;
        try {
            const result = await save(details.titleId, removing ? "" : input.value);
            saved = true;
            populate(result.details);
            await onSaved(result);
            dialog.close();
        } catch (error) {
            notify((saved ? "Name saved, but refreshing the view failed. " : "Could not save game name. ") + (error?.message || String(error)));
        } finally {
            pending = false;
            fieldset.disabled = false;
            closeButtons.forEach(button => button.disabled = false);
            onBusyChange(false);
        }
    });
    return {
        close,
        async open(titleID, returnFocus) {
            if (dialog.open || pending) return;
            const token = ++request;
            origin = returnFocus;
            details = null;
            fieldset.disabled = true;
            notify("Loading game name…");
            dialog.showModal();
            try {
                const value = await load(titleID);
                if (token !== request || !dialog.open) return;
                populate(value);
                fieldset.disabled = false;
                notify("");
                input.focus();
                input.select();
            } catch (error) {
                if (token === request && dialog.open) notify("Could not load game name. " + (error?.message || String(error)));
            }
        }
    };
}
