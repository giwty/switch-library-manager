import assert from "node:assert/strict";
import {readFile} from "node:fs/promises";
import test from "node:test";
import vm from "node:vm";

const appSource = await readFile(new URL("../frontend/app.js", import.meta.url), "utf8");
const index = await readFile(new URL("../frontend/index.html", import.meta.url), "utf8");
const jsrender = await readFile(new URL("../frontend/lib/js/jsrender/jsrender.min.js", import.meta.url), "utf8");
const collectStart = appSource.indexOf("        function collectSettings(form) {");
const collectEnd = appSource.indexOf("        function captureSettingsDraft()", collectStart);
assert.notEqual(collectStart, -1);
assert.notEqual(collectEnd, -1);

test("settings form saves the canonical Unicode option in both states", () => {
    for (const enabled of [false, true]) {
        const elements = new Proxy({}, {
            get(_target, name) {
                return {checked: name === "organization_allow_unicode_file_names" && enabled, value: ""};
            }
        });
        const context = {
            state: {settings: {schema_version: 2}},
            listValues: () => [],
            form: {elements}
        };
        vm.createContext(context);
        vm.runInContext(appSource.slice(collectStart, collectEnd), context);
        const saved = vm.runInContext("collectSettings(form)", context);
        assert.equal(saved.organization.allow_unicode_file_names, enabled);
        assert.equal(Object.hasOwn(saved.organization, "switch_safe_file_names"), false);
    }
});

test("settings template renders the Unicode choice and its title/folder explanation", () => {
    const context = {window: {}};
    vm.createContext(context);
    vm.runInContext(jsrender, context);
    const block = index.match(/<div class="form-check">\s*<input[^>]*id="settings-org-unicode-names"[\s\S]*?<\/div>\s*<\/div>/)?.[0];
    assert.ok(block, "Unicode settings control exists");
    const template = context.window.jsrender.templates(block);
    for (const enabled of [false, true]) {
        const rendered = template.render({organization: {allow_unicode_file_names: enabled}});
        assert.equal(/\bchecked\b/.test(rendered), enabled);
        assert.match(rendered, /Allow Unicode titles and filenames/);
        assert.match(rendered, /catalog titles in any language/);
        assert.match(rendered, /game folders/);
    }
});
