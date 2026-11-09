import {test as base, expect} from '@playwright/test';
import {readFile} from 'node:fs/promises';

export const fixtures = JSON.parse(await readFile(new URL('./fixtures.json', import.meta.url), 'utf8'));

// Runs before the real application modules, including generated Wails wrappers.
function installBridge({data, responses, platform}) {
    const copy = value => structuredClone(value);
    const calls = [];
    const errors = [];
    const pending = new Map();
    const events = new Map();
    const queues = copy(responses);
    const defaults = {
        LoadSettings: () => data.settings,
        SaveSettings: value => { data.settings = copy(value); return null; },
        UpdateDB: () => null,
        UpdateLocalLibrary: () => data.library,
        RescanLibrary: () => data.library,
        IsKeysFileAvailable: () => false,
        CheckUpdate: () => false,
        GetMissingGames: () => data.missingGames,
        GetMissingUpdates: () => data.missingUpdates.filter(row => !data.settings.missing_content.ignore_update_title_ids.includes(row.Attributes.id)),
        GetMissingDLC: () => data.missingDLC.map(row => ({...row, missing_dlc: row.missing_dlc.filter(dlc => !data.settings.missing_content.ignore_dlc_title_ids.includes(dlc.id))})).filter(row => row.missing_dlc.length),
        GetOrganizationPreview: () => data.preview,
        GetGameDetails: id => {
            const details = copy(data.details);
            const row = data.library.library_data.find(row => row.titleId === id);
            if (row) Object.assign(details, {titleId: id, name: row.name, attributes: {...details.attributes, id, name: row.name}});
            return details;
        },
        SetContentIgnored: (kind, id, ignored) => {
            const key = kind === 'updates' ? 'ignore_update_title_ids' : 'ignore_dlc_title_ids';
            const list = data.settings.missing_content[key].filter(value => value.toLowerCase() !== id.toLowerCase());
            if (ignored) list.push(id.toLowerCase());
            data.settings.missing_content[key] = list;
            return data.settings.missing_content;
        },
        OrganizeLibrary: () => data.organization,
        SelectFolder: () => '',
        ConfirmOrganization: () => true,
        ShowMessage: () => null,
        ExportXLSX: () => '/synthetic/export.xlsx',
        ShowInFolder: () => null,
        Restart: () => null
    };
    function strict(object, label) {
        return new Proxy(object, {get(target, key) {
            if (typeof key === 'symbol' || key in target) return target[key];
            const message = `Unexpected ${label} call: ${key}`;
            errors.push(message);
            throw new Error(message);
        }});
    }
    const methods = Object.fromEntries(Object.entries(defaults).map(([name, handler]) => [name, async (...args) => {
        calls.push({name, args: copy(args)});
        const action = queues[name]?.shift() || {};
        if (action.defer) {
            await new Promise((resolve, reject) => {
                if (pending.has(action.defer)) throw new Error(`Duplicate deferred call: ${action.defer}`);
                pending.set(action.defer, {resolve, reject});
            });
        }
        if (action.error) throw new Error(action.error);
        return copy(Object.hasOwn(action, 'result') ? action.result : handler(...args));
    }]));
    window.go = {app: {App: strict(methods, 'Go bridge')}};
    window.runtime = strict({
        Environment: () => Promise.resolve({platform, arch: 'amd64', buildType: 'test'}),
        EventsOnMultiple: (name, callback, remaining) => {
            const listener = {callback, remaining};
            const listeners = events.get(name) || new Set();
            events.set(name, listeners);
            listeners.add(listener);
            return () => listeners.delete(listener);
        },
        EventsOff: (...names) => names.forEach(name => events.delete(name)),
        EventsOffAll: () => events.clear(),
        WindowSetDarkTheme() {},
        WindowSetLightTheme() {},
        WindowSetSystemDefaultTheme() {}
    }, 'Wails runtime');
    window.__slmTest = {
        calls, errors,
        queue(name, actions) { queues[name] = copy(actions); },
        resolve(id) {
            if (!pending.has(id)) throw new Error(`No deferred call: ${id}`);
            pending.get(id).resolve(); pending.delete(id);
        },
        reject(id, message) {
            if (!pending.has(id)) throw new Error(`No deferred call: ${id}`);
            pending.get(id).reject(new Error(message)); pending.delete(id);
        },
        emit(name, message) {
            for (const listener of events.get(name) || []) {
                listener.callback(copy(message));
                if (listener.remaining > 0 && --listener.remaining === 0) events.get(name).delete(listener);
            }
        }
    };
}

export const test = base.extend({
    app: async ({page, context, baseURL}, use) => {
        const problems = [];
        page.on('pageerror', error => problems.push(`Page: ${error.message}`));
        page.on('console', message => { if (message.type() === 'error') problems.push(`Console: ${message.text()}`); });
        page.on('response', response => { if (response.status() >= 400) problems.push(`HTTP ${response.status()}: ${response.url()}`); });
        await context.route('**/*', async route => {
            if (new URL(route.request().url()).origin === baseURL) await route.continue();
            else { problems.push(`Unexpected external request: ${route.request().url()}`); await route.abort(); }
        });
        const app = {
            async open({data = fixtures, responses = {}, platform = 'windows', ready = true} = {}) {
                await page.addInitScript(installBridge, {data: structuredClone(data), responses, platform});
                await page.goto('/');
                if (ready) await this.ready();
            },
            async ready() { await expect(page.locator('.wrapper')).not.toHaveClass(/is-loading/); await expect(page.getByRole('tab', {name: 'Library', exact: true})).toBeVisible(); },
            calls: name => page.evaluate(name => window.__slmTest.calls.filter(call => call.name === name), name),
            async called(name, count = 1) { await expect.poll(async () => (await this.calls(name)).length).toBe(count); },
            queue: (name, actions) => page.evaluate(({name, actions}) => window.__slmTest.queue(name, actions), {name, actions}),
            resolve: id => page.evaluate(id => window.__slmTest.resolve(id), id),
            reject: (id, message) => page.evaluate(({id, message}) => window.__slmTest.reject(id, message), {id, message}),
            emit: (name, message) => page.evaluate(({name, message}) => window.__slmTest.emit(name, message), {name, message}),
            async tab(name) { await page.getByRole('tab', {name, exact: true}).click(); }
        };
        await use(app);
        const bridgeErrors = await page.evaluate(() => window.__slmTest?.errors || []).catch(() => []);
        expect([...problems, ...bridgeErrors], 'Unexpected browser/network/bridge errors').toEqual([]);
    }
});
export {expect};
