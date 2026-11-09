import {test, expect, fixtures} from './app-fixture.mjs';

const rows = page => page.locator('#library-table .tabulator-row');
const titleFilter = page => page.locator('#library-table .tabulator-col[tabulator-field="name"] input');

test('startup loads real assets and all seven tabs render fixture data', async ({app, page}) => {
    await app.open();
    await expect(rows(page)).toHaveCount(2);
    for (const [tab, selector, text] of [
        ['Missing games', '#missingGames-table', 'Distant Demo'],
        ['Missing updates', '#updates-table', 'Amber Adventure'],
        ['Missing DLC', '#dlc-table', 'Amber Expansion'],
        ['Organize', '#organize', 'Amber Adventure [0100000000010000].nsp'],
        ['Issues', '#status-table', 'file type is not supported'],
        ['Settings', '#settings-form', 'Interface']
    ]) {
        await app.tab(tab);
        await expect(page.locator(selector)).toContainText(text);
    }
    await app.tab('Library');
    await expect(rows(page).first()).toContainText('Amber Adventure');
    await app.called('UpdateDB');
    await app.called('UpdateLocalLibrary');
});

test('table filtering, sorting and pagination operate on actual rendered rows', async ({app, page}) => {
    await app.open();
    await page.locator('#library-table button[data-page="next"]').click();
    await expect(rows(page)).toHaveCount(1);
    await expect(rows(page)).toContainText('Cobalt Quest');
    await titleFilter(page).pressSequentially('Amber');
    await expect(rows(page)).toHaveCount(1);
    await expect(rows(page)).toContainText('Amber Adventure');
    await titleFilter(page).press('ControlOrMeta+a');
    await titleFilter(page).press('Backspace');
    await expect(rows(page)).toHaveCount(2);
    await page.locator('#library-table .tabulator-col[tabulator-field="name"] .tabulator-col-title').click();
    await expect(rows(page).first()).toContainText('Cobalt Quest');
});

test('empty library and folder selection cancellation/success remain usable', async ({app, page}) => {
    const data = structuredClone(fixtures);
    data.library = {library_data: [], issues: [], num_files: 0};
    await app.open({data});
    await expect(page.locator('#library')).toContainText('is empty');
    await page.getByRole('button', {name: 'Set Folder', exact: true}).click();
    await app.called('SelectFolder');
    expect(await app.calls('SaveSettings')).toHaveLength(0);
    await app.queue('SelectFolder', [{result: '/synthetic/another-library'}]);
    await page.getByRole('button', {name: 'Set Folder', exact: true}).click();
    await app.called('SaveSettings');
    await app.ready();
    expect((await app.calls('SaveSettings'))[0].args[0].paths.library_folder).toBe('/synthetic/another-library');
});

for (const platform of ['windows', 'darwin']) {
    test(`${platform} keyboard shortcuts navigate rows, details and help`, async ({app, page}) => {
        await app.open({platform});
        await rows(page).first().focus();
        await page.keyboard.press('ArrowDown');
        await expect(rows(page).nth(1)).toBeFocused();
        await page.keyboard.press('Enter');
        await expect(page.getByRole('dialog', {name: 'Game details'})).toBeVisible();
        await expect(page.locator('#game-details-title')).toHaveText('Birch & Bloom');
        await page.keyboard.press('Escape');
        await expect(page.locator('#game-details-dialog')).not.toBeVisible();
        await page.keyboard.press(`${platform === 'darwin' ? 'Meta' : 'Control'}+7`);
        await expect(page.locator('#settings-form')).toBeVisible();
        await page.keyboard.press('F1');
        await expect(page.getByRole('dialog', {name: 'Keyboard shortcuts'})).toBeVisible();
        await page.keyboard.press('Escape');
        await expect(page.locator('#keyboard-shortcuts-dialog')).not.toBeVisible();
    });
}

test('details expose simulated file manager actions without opening native windows', async ({app, page}) => {
    await app.open();
    await rows(page).first().click();
    await expect(page.locator('#game-details-title')).toHaveText('Amber Adventure');
    await page.getByRole('button', {name: 'Show in folder', exact: true}).first().click();
    await app.called('ShowInFolder');
    expect((await app.calls('ShowInFolder'))[0].args).toEqual(['/synthetic/library/Amber Adventure.nsp']);
});

test('settings failure retains draft; successful save locks for restart and restart failure is retryable', async ({app, page}) => {
    await app.open();
    await app.tab('Settings');
    await page.locator('#settings-gui-page-size').fill('7');
    await app.queue('SaveSettings', [{error: 'Synthetic settings write failed'}, {}]);
    await page.locator('#settings-form').getByRole('button', {name: /save/i}).click();
    await expect(page.locator('#settings')).toContainText('Synthetic settings write failed');
    await expect(page.locator('#settings-gui-page-size')).toHaveValue('7');
    await page.locator('#settings-form').getByRole('button', {name: /save/i}).click();
    await expect(page.getByRole('button', {name: 'Restart now'})).toBeVisible();
    expect((await app.calls('SaveSettings'))[1].args[0].gui.page_size).toBe(7);
    await app.queue('Restart', [{error: 'Synthetic restart failure'}, {}]);
    await page.getByRole('button', {name: 'Restart now'}).click();
    await expect(page.locator('.progress-msg')).toContainText('Synthetic restart failure');
    await page.getByRole('button', {name: 'Restart now'}).click();
    await app.called('Restart', 2);
});

test('pending startup emits progress and prevents commands until scan completes', async ({app, page}) => {
    await app.open({responses: {UpdateLocalLibrary: [{defer: 'startup'}]}, ready: false});
    await app.called('UpdateLocalLibrary');
    await app.emit('updateProgress', fixtures.progress);
    await expect(page.getByRole('progressbar')).toHaveAttribute('aria-valuenow', '50');
    await page.keyboard.press('Control+r');
    expect(await app.calls('RescanLibrary')).toHaveLength(0);
    await app.resolve('startup');
    await app.ready();
    await expect(rows(page)).toHaveCount(2);
});

test('rescan prevents duplicates, reports failure and recovers on a hard rescan', async ({app, page}) => {
    await app.open();
    await app.queue('RescanLibrary', [{defer: 'rescan'}, {}]);
    await page.keyboard.press('Control+r');
    await app.called('RescanLibrary');
    await page.keyboard.press('Control+r');
    expect(await app.calls('RescanLibrary')).toHaveLength(1);
    await app.reject('rescan', 'Synthetic rescan failure');
    await app.called('ShowMessage');
    expect((await app.calls('ShowMessage'))[0].args).toContain('Synthetic rescan failure');
    await app.ready();
    await page.keyboard.press('Control+Shift+r');
    await app.called('RescanLibrary', 2);
    await app.ready();
    expect((await app.calls('RescanLibrary'))[1].args).toEqual([true]);
});

test('late tab responses cannot switch away from the user selected tab', async ({app, page}) => {
    await app.open({responses: {GetMissingUpdates: [{defer: 'updates'}]}});
    await app.tab('Missing updates');
    await app.called('GetMissingUpdates');
    await app.tab('Settings');
    await page.locator('#settings-gui-page-size').fill('9');
    await app.resolve('updates');
    await expect(page.locator('#settings-tab')).toHaveAttribute('aria-selected', 'true');
    await expect(page.locator('#settings-gui-page-size')).toHaveValue('9');
    await app.tab('Missing updates');
    await expect(page.locator('#updates-table')).toContainText('Amber Adventure');
    await app.called('GetMissingUpdates');
});

test('context menu ignore action saves preferences and refreshes missing content', async ({app, page}) => {
    await app.open();
    await app.tab('Missing updates');
    await page.locator('#updates-table .tabulator-row').first().click({button: 'right'});
    await page.getByRole('menuitem', {name: /ignore/i}).click();
    await app.called('SetContentIgnored');
    await expect(page.locator('#updates')).toContainText('not missing any updates');
    expect((await app.calls('SetContentIgnored'))[0].args).toEqual(['updates', fixtures.details.titleId, true]);
    expect(await app.calls('RescanLibrary')).toHaveLength(0);
});

test('organization confirmation cancellation, conflicts, failure and success use simulated native dialogs', async ({app, page}) => {
    await app.open({responses: {ConfirmOrganization: [{result: false}, {defer: 'confirm'}, {}, {}], OrganizeLibrary: [{result: fixtures.blockedOrganization}, {error: 'Synthetic organization failure'}, {}]}});
    await app.tab('Organize');
    await page.getByRole('button', {name: 'Organize library', exact: true}).click();
    await app.called('ConfirmOrganization');
    expect(await app.calls('OrganizeLibrary')).toHaveLength(0);
    await page.getByRole('button', {name: 'Organize library', exact: true}).click();
    await app.called('ConfirmOrganization', 2);
    await page.keyboard.press('Control+o');
    expect(await app.calls('ConfirmOrganization')).toHaveLength(2);
    await app.resolve('confirm');
    await expect(page.locator('#status')).toContainText('organization conflict');
    await app.tab('Organize');
    await page.getByRole('button', {name: 'Organize library', exact: true}).click();
    await app.called('OrganizeLibrary', 2);
    await app.ready();
    await expect.poll(async () => (await app.calls('ShowMessage')).some(call => call.args[1] === 'Organization failed')).toBe(true);
    await app.tab('Organize');
    await page.getByRole('button', {name: 'Organize library', exact: true}).click();
    await app.called('OrganizeLibrary', 3);
    await expect.poll(async () => (await app.calls('ShowMessage')).some(call => call.args[1] === 'Success')).toBe(true);
});

test('CSV downloads and XLSX cancellation, errors and duplicate prevention preserve table usability', async ({app, page}) => {
    await app.open();
    const downloadPromise = page.waitForEvent('download');
    await page.getByRole('button', {name: 'Export to CSV', exact: true}).click();
    const download = await downloadPromise;
    expect(download.suggestedFilename()).toMatch(/^slm_games\..*\.csv$/);
    const stream = await download.createReadStream();
    const chunks = [];
    for await (const chunk of stream) chunks.push(chunk);
    const csv = Buffer.concat(chunks);
    expect(csv.subarray(0, 3)).toEqual(Buffer.from([0xef, 0xbb, 0xbf]));
    for (const row of fixtures.library.library_data) expect(csv.toString('utf8')).toContain(row.name);
    await app.queue('ExportXLSX', [{result: ''}, {error: 'Synthetic export failure'}, {defer: 'export'}]);
    const exportButton = page.getByRole('button', {name: 'Export to Excel (.xlsx)', exact: true});
    await exportButton.click();
    await app.called('ExportXLSX');
    await expect(exportButton).toBeEnabled();
    await exportButton.click();
    await app.called('ShowMessage');
    await expect(exportButton).toBeEnabled();
    await exportButton.click();
    await app.called('ExportXLSX', 3);
    await expect(exportButton).toBeDisabled();
    expect((await app.calls('ExportXLSX'))[0].args[0].rows).toHaveLength(3);
    await app.resolve('export');
    await expect(exportButton).toBeEnabled();
});

for (const theme of ['light', 'dark']) for (const width of [1280, 800]) {
    test(`layout ${theme} ${width}: navigation, settings and details stay accessible`, async ({app, page}, testInfo) => {
        const data = structuredClone(fixtures);
        data.settings.gui.theme = theme;
        await page.setViewportSize({width, height: width === 800 ? 600 : 800});
        await app.open({data});
        await expect(page.locator('html')).toHaveAttribute('data-bs-theme', theme);
        await expect(rows(page)).toHaveCount(2);
        const capture = async name => {
            const filename = testInfo.outputPath(`${name}.png`);
            await page.screenshot({path: filename, fullPage: true});
            await testInfo.attach(name, {path: filename, contentType: 'image/png'});
        };
        await capture('library');
        await rows(page).first().click();
        const dialog = page.getByRole('dialog', {name: 'Game details'});
        await expect(dialog).toBeVisible();
        const box = await dialog.boundingBox();
        expect(box.x).toBeGreaterThanOrEqual(0);
        expect(box.x + box.width).toBeLessThanOrEqual(width + 1);
        await capture('details');
        await page.keyboard.press('Escape');
        await app.tab('Settings');
        await expect(page.locator('#settings-gui-page-size')).toBeVisible();
        await capture('settings');
        await app.tab('Library');
        await expect(page.locator('#library-tab')).toHaveAttribute('aria-selected', 'true');
    });
}
