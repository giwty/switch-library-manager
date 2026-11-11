import {test, expect, fixtures} from './app-fixture.mjs';

const editor = page => page.getByRole('dialog', {name: 'Edit game name', exact: true});
const input = page => page.getByRole('textbox', {name: 'Game name', exact: true});
async function openMenu(page, table = '#library-table') {
    await page.locator(table + ' .tabulator-row').first().click({button: 'right'});
    await page.getByRole('menuitem', {name: 'Edit game name…'}).click();
    await expect(input(page)).toBeEnabled();
}

test('context editor saves literal names everywhere and preserves settings drafts', async ({app, page}) => {
    await app.open();
    await app.tab('Settings');
    await page.locator('#settings-gui-page-size').fill('9');
    await app.tab('Library');
    await openMenu(page);
    const custom = '<img src=x onerror=alert(1)> 日本語';
    await input(page).fill(custom);
    await editor(page).getByRole('button', {name: 'Save', exact: true}).click();
    await expect(editor(page)).not.toBeVisible();
    await expect(page.locator('#library-table')).toContainText(custom);
    await expect(page.locator('#library-table img[src="x"]')).toHaveCount(0);
    await page.locator('#library-table .tabulator-row').filter({hasText: custom}).click();
    await expect(page.locator('#game-details-title')).toHaveText(custom);
    await expect(page.locator('#game-details-content')).toContainText('Custom name');
    await page.keyboard.press('Escape');
    for (const [tab, table] of [['Missing updates', '#updates-table'], ['Missing DLC', '#dlc-table']]) {
        await app.tab(tab);
        await expect(page.locator(table)).toContainText(custom);
    }
    await app.tab('Settings');
    await expect(page.locator('#settings-gui-page-size')).toHaveValue('9');
    await page.getByRole('button', {name: 'Save settings', exact: true}).click();
    const calls = await app.calls('SaveSettings');
    expect(calls[0].args[0].title_name_overrides).toEqual({[fixtures.details.titleId]: custom});
    expect(await app.calls('RescanLibrary')).toHaveLength(0);
    expect(await app.calls('UpdateLocalLibrary')).toHaveLength(1);
});

test('details pencil opens a stacked editor with draft case tools, cancel and staged restore', async ({app, page}) => {
    await app.open();
    await page.locator('#library-table .tabulator-row').first().click();
    await page.getByRole('button', {name: 'Edit game name', exact: true}).click();
    await expect(input(page)).toHaveValue('Amber Adventure');
    for (const [button, expected] of [['UPPERCASE', 'AMBER ADVENTURE'], ['lowercase', 'amber adventure'], ['Title Case', 'Amber Adventure'], ['Sentence case', 'Amber adventure']]) {
        await editor(page).getByRole('button', {name: button, exact: true}).click();
        await expect(input(page)).toHaveValue(expected);
    }
    await page.keyboard.press('Escape');
    await expect(page.locator('#game-details-dialog')).toBeVisible();
    await expect(page.getByRole('button', {name: 'Edit game name', exact: true})).toBeFocused();
    expect(await app.calls('SetGameNameOverride')).toHaveLength(0);
    await page.getByRole('button', {name: 'Edit game name', exact: true}).click();
    await input(page).fill('My Name');
    await editor(page).getByRole('button', {name: 'Save', exact: true}).click();
    await expect(editor(page)).not.toBeVisible();
    await expect(page.locator('#game-details-title')).toHaveText('My Name');
    await page.getByRole('button', {name: 'Edit game name', exact: true}).click();
    await editor(page).getByRole('button', {name: 'Restore automatic name'}).click();
    await expect(input(page)).toHaveValue('Amber Adventure');
    await editor(page).getByRole('button', {name: 'Cancel', exact: true}).click();
    await expect(page.locator('#game-details-title')).toHaveText('My Name');
    await page.getByRole('button', {name: 'Edit game name', exact: true}).click();
    await editor(page).getByRole('button', {name: 'Restore automatic name'}).click();
    await editor(page).getByRole('button', {name: 'Save', exact: true}).click();
    await expect(editor(page)).not.toBeVisible();
    await expect(page.locator('#game-details-title')).toHaveText('Amber Adventure');
    expect((await app.calls('SetGameNameOverride')).at(-1).args).toEqual([fixtures.details.titleId, '']);
    await page.getByRole('button', {name: 'Edit game name', exact: true}).click();
    await expect(editor(page).getByRole('button', {name: 'Restore automatic name'})).toBeDisabled();
});

test('save errors keep drafts, pending saves block Escape and duplicates, table state survives', async ({app, page}) => {
    await app.open({responses: {SetGameNameOverride: [{error: 'Synthetic write failure'}, {defer: 'name-save'}]}});
    await page.locator('#library-table .tabulator-col[tabulator-field="name"] input').fill('Amber');
    await openMenu(page);
    await input(page).fill('Amber Custom');
    await editor(page).getByRole('button', {name: 'Save', exact: true}).click();
    await expect(editor(page)).toContainText('Synthetic write failure');
    await expect(input(page)).toHaveValue('Amber Custom');
    await editor(page).getByRole('button', {name: 'Save', exact: true}).click();
    await app.called('SetGameNameOverride', 2);
    await page.keyboard.press('Escape');
    await page.keyboard.press('Enter');
    await expect(editor(page)).toBeVisible();
    await expect(editor(page).getByRole('button', {name: 'Save', exact: true})).toBeDisabled();
    expect(await app.calls('SetGameNameOverride')).toHaveLength(2);
    await app.resolve('name-save');
    await expect(editor(page)).not.toBeVisible();
    await expect(page.locator('#library-table .tabulator-col[tabulator-field="name"] input')).toHaveValue('Amber');
    await expect(page.locator('#library-table .tabulator-row')).toHaveCount(1);
    await expect(page.locator('#library-table')).toContainText('Amber Custom');
});

test('all tables expose keyboard edit and DLC rows edit the parent game', async ({app, page}) => {
    await app.open();
    for (const [tab, table] of [['Library', '#library-table'], ['Missing games', '#missingGames-table'], ['Missing updates', '#updates-table'], ['Missing DLC', '#dlc-table']]) {
        await app.tab(tab);
        await page.locator(table + ' .tabulator-row').first().focus();
        await page.keyboard.press('Shift+F10');
        await page.getByRole('menuitem', {name: 'Edit game name…'}).click();
        await expect(input(page)).toBeEnabled();
        await expect(editor(page).locator('[data-name-id]')).toHaveText(tab === 'Missing games' ? '0100000000040000' : fixtures.details.titleId);
        await page.keyboard.press('Shift+Tab');
        await expect(editor(page).getByRole('button', {name: 'Close name editor'})).toBeFocused();
        await page.keyboard.press('Shift+Tab');
        await expect(editor(page).getByRole('button', {name: 'Save', exact: true})).toBeFocused();
        await page.keyboard.press('Escape');
        await expect(page.locator(table + ' .tabulator-row').first()).toBeFocused();
    }
});

test('editing retains sorting and the current library page', async ({app, page}) => {
    await app.open();
    await page.locator('#library-table button[data-page="next"]').click();
    await expect(page.locator('#library-table .tabulator-row')).toContainText('Cobalt Quest');
    await openMenu(page);
    await input(page).fill('Cobalt Custom');
    await editor(page).getByRole('button', {name:'Save', exact:true}).click();
    await expect(editor(page)).not.toBeVisible();
    await expect(page.locator('#library-table .tabulator-row')).toHaveCount(1);
    await expect(page.locator('#library-table .tabulator-row')).toContainText('Cobalt Custom');
    await expect(page.locator('#library-table button[data-page="2"]')).toHaveClass(/active/);
});

test('a renamed row can leave its current filter and missing games can be renamed', async ({app, page}) => {
    await app.open();
    await page.locator('#library-table .tabulator-col[tabulator-field="name"] input').fill('Amber');
    await openMenu(page);
    await input(page).fill('Different Name');
    await editor(page).getByRole('button', {name:'Save', exact:true}).click();
    await expect(editor(page)).not.toBeVisible();
    await expect(page.locator('#library-table .tabulator-row')).toHaveCount(0);
    await expect(page.locator('#library-table .tabulator-col[tabulator-field="name"] input')).toHaveValue('Amber');
    await app.tab('Missing games');
    await openMenu(page, '#missingGames-table');
    await input(page).fill('My Missing Game');
    await editor(page).getByRole('button', {name:'Save', exact:true}).click();
    await expect(editor(page)).not.toBeVisible();
    await expect(page.locator('#missingGames-table')).toContainText('My Missing Game');
});

test('stale editor loads cannot replace the next editor and refresh errors report saved state', async ({app, page}) => {
    await app.open({responses: {GetGameDetails: [{defer: 'old-name'}]}});
    await page.locator('#library-table .tabulator-row').first().click({button:'right'});
    await page.getByRole('menuitem', {name:'Edit game name…'}).click();
    await page.keyboard.press('Tab');
    await expect(editor(page).getByRole('button', {name:'Close name editor'})).toBeFocused();
    await page.keyboard.press('Escape');
    await page.locator('#library-table .tabulator-row').nth(1).click({button:'right'});
    await page.getByRole('menuitem', {name:'Edit game name…'}).click();
    await expect(input(page)).toHaveValue('Birch & Bloom');
    await app.resolve('old-name');
    await expect(input(page)).toHaveValue('Birch & Bloom');
    await page.keyboard.press('Escape');
    await app.tab('Missing updates');
    await openMenu(page, '#updates-table');
    await app.queue('GetMissingUpdates', [{error:'Synthetic refresh failure'}]);
    await input(page).fill('Saved Name');
    await editor(page).getByRole('button', {name:'Save', exact:true}).click();
    await expect(editor(page)).toContainText('Name saved, but refreshing the view failed');
    await expect(input(page)).toHaveValue('Saved Name');
});

for (const theme of ['light', 'dark']) for (const size of [{width:1280,height:800}, {width:800,height:600}]) {
    test(`name editor layout ${theme} ${size.width}x${size.height}`, async ({app, page}, testInfo) => {
        await page.setViewportSize(size);
        const data = structuredClone(fixtures);
        data.settings.gui.theme = theme;
        await app.open({data});
        await page.locator('#library-table .tabulator-row').first().click();
        await page.getByRole('button', {name:'Edit game name', exact:true}).click();
        await expect(input(page)).toBeEnabled();
        const box = await editor(page).boundingBox();
        expect(box.x).toBeGreaterThanOrEqual(0);
        expect(box.y).toBeGreaterThanOrEqual(0);
        expect(box.x + box.width).toBeLessThanOrEqual(size.width);
        expect(box.y + box.height).toBeLessThanOrEqual(size.height);
        await page.screenshot({path:testInfo.outputPath('name-editor.png')});
    });
}
