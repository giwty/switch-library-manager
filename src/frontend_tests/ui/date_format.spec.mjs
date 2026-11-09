import {test, expect, fixtures} from './app-fixture.mjs';

const preset = '#settings-gui-date-format';
const custom = '#settings-gui-date-format-custom';
const save = page => page.locator('#settings-form').getByRole('button', {name: 'Save settings', exact: true}).click();

test('date format drafts, validation, cancellation and failed saves preserve the active preference', async ({app, page}) => {
    await app.open();
    await app.tab('Settings');
    await expect(page.locator(preset)).toHaveValue('YYYY-MM-DD');
    await expect(page.locator('#settings-date-format-example')).toHaveText('Example: 2026-11-23');
    await page.locator(preset).selectOption('custom');
    for (const invalid of ['YYYY-MM-DD HH:mm', '', 'YYYY-DD-DD', '<script>']) {
        await page.locator(custom).fill(invalid);
        await expect(page.locator(custom)).toHaveAttribute('aria-invalid', 'true');
        await expect(page.locator('#settings-date-format-error')).not.toBeEmpty();
        await save(page);
        expect(await app.calls('SaveSettings')).toHaveLength(0);
    }
    await page.locator(custom).fill('D MMM YYYY');
    await expect(page.locator('#settings-date-format-example')).toHaveText('Example: 23 Nov 2026');
    await app.tab('Missing games');
    await expect(page.locator('#missingGames-table')).toContainText('2026-04-01');
    await app.tab('Settings');
    await expect(page.locator(preset)).toHaveValue('custom');
    await expect(page.locator(custom)).toHaveValue('D MMM YYYY');
    await page.getByRole('button', {name: 'Cancel', exact: true}).click();
    await expect(page.locator(preset)).toHaveValue('YYYY-MM-DD');
    await expect(page.locator(custom)).not.toBeVisible();

    await page.locator(preset).selectOption('MM/DD/YYYY');
    await app.queue('SaveSettings', [{error: 'Synthetic date settings write failed'}]);
    await save(page);
    await expect(page.locator('#settings .alert-danger')).toContainText('Synthetic date settings write failed');
    await expect(page.locator(preset)).toHaveValue('MM/DD/YYYY');
    await app.tab('Missing games');
    await expect(page.locator('#missingGames-table')).toContainText('2026-04-01');
    await app.tab('Settings');
    await save(page);
    await expect(page.locator('#settings')).toBeFocused();
    expect((await app.calls('SaveSettings'))[1].args[0].gui.date_format).toBe('MM/DD/YYYY');
    await app.tab('Missing games');
    await expect(page.locator('#missingGames-table')).toContainText('04/01/2026');
    for (const name of ['Restart', 'RescanLibrary']) expect(await app.calls(name)).toHaveLength(0);
    expect(await app.calls('UpdateLocalLibrary')).toHaveLength(1);
});

for (const [pattern, dates] of [
    ['YYYY-MM-DD', ['2026-01-01', '2026-02-01', '2026-03-01', '2026-04-01']],
    ['MM/DD/YYYY', ['01/01/2026', '02/01/2026', '03/01/2026', '04/01/2026']],
    ['DD/MM/YYYY', ['01/01/2026', '01/02/2026', '01/03/2026', '01/04/2026']],
    ['MMMM D, YYYY', ['January 1, 2026', 'February 1, 2026', 'March 1, 2026', 'April 1, 2026']],
    ['D.M.YYYY', ['1.1.2026', '1.2.2026', '1.3.2026', '1.4.2026']]
]) {
    test(`date format ${pattern} matches every GUI view and CSV/Excel values`, async ({app, page}) => {
        const data = structuredClone(fixtures);
        data.settings.gui.date_format = pattern;
        await app.open({data});
        await page.locator('#library-table .tabulator-row').first().click();
        await expect(page.locator('.game-details-metadata')).toContainText(dates[0]);
        await expect(page.locator('[aria-labelledby="game-details-updates-heading"]')).toContainText(dates[1]);
        await expect(page.locator('[aria-labelledby="game-details-dlc-heading"]')).toContainText(dates[2]);
        await page.keyboard.press('Escape');
        for (const [index, tab, table, date] of [
            [1, 'Missing games', '#missingGames-table', dates[3]],
            [2, 'Missing updates', '#updates-table', dates[1]],
            [3, 'Missing DLC', '#dlc-table', dates[2]]
        ]) {
            await app.tab(tab);
            await expect(page.locator(table)).toContainText(date);
            const pending = page.waitForEvent('download');
            await page.getByRole('button', {name: 'Export to CSV', exact: true}).click();
            const download = await pending;
            expect(download.suggestedFilename()).toMatch(/^slm_\w+\.\d{4}-\d{2}-\d{2}\.csv$/);
            const stream = await download.createReadStream();
            const chunks = [];
            for await (const chunk of stream) chunks.push(chunk);
            expect(Buffer.concat(chunks).toString('utf8')).toContain(`"${date}"`);
            await page.getByRole('button', {name: 'Export to Excel (.xlsx)', exact: true}).click();
            await app.called('ExportXLSX', index);
            const report = (await app.calls('ExportXLSX'))[index - 1].args[0];
            const column = report.headers.findIndex(header => / date$/i.test(header));
            expect(report.rows[0][column]).toBe(date);
        }
        await app.tab('Settings');
        await expect(page.locator(preset)).toHaveValue(pattern.includes('MMM') || pattern === 'D.M.YYYY' ? 'custom' : pattern);
    });
}

test('unknown and malformed dates show Unknown and export blanks', async ({app, page}) => {
    const data = structuredClone(fixtures);
    data.settings.gui.date_format = 'MMMM D, YYYY';
    data.missingGames[0].release_date = '2026-02-29';
    data.missingUpdates[0].latest_update_date = '-';
    data.missingDLC[0].missing_dlc[0].release_date = '';
    data.details.release_date_text = '';
    data.details.updates[0].date = '2026-02-29';
    data.details.dlc[0].attributes.ParsedReleaseDate = '0';
    await app.open({data});
    await page.locator('#library-table .tabulator-row').first().click();
    await expect(page.locator('.game-details-metadata')).not.toContainText('Release date');
    await expect(page.locator('[aria-labelledby="game-details-updates-heading"]')).toContainText('Unknown');
    await expect(page.locator('[aria-labelledby="game-details-dlc-heading"]')).toContainText('Unknown');
    await page.keyboard.press('Escape');
    for (const [index, tab, table] of [[1, 'Missing games', '#missingGames-table'], [2, 'Missing updates', '#updates-table'], [3, 'Missing DLC', '#dlc-table']]) {
        await app.tab(tab);
        await expect(page.locator(table)).toContainText('Unknown');
        await page.getByRole('button', {name: 'Export to Excel (.xlsx)', exact: true}).click();
        await app.called('ExportXLSX', index);
        const report = (await app.calls('ExportXLSX'))[index - 1].args[0];
        expect(report.rows[0][report.headers.findIndex(header => / date$/i.test(header))]).toBe('');
    }
});
