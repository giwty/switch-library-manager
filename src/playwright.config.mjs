import {defineConfig} from '@playwright/test';

export default defineConfig({
    testDir: './frontend_tests/ui',
    testMatch: '**/*.spec.mjs',
    fullyParallel: true,
    forbidOnly: Boolean(process.env.CI),
    retries: 0,
    workers: 2,
    timeout: 30_000,
    outputDir: './build/test-results/ui',
    reporter: [['list'], ['html', {outputFolder: './build/test-results/playwright-report', open: 'never'}]],
    use: {
        browserName: 'chromium',
        headless: true,
        baseURL: 'http://127.0.0.1:4173',
        viewport: {width: 1280, height: 800},
        locale: 'en-US',
        timezoneId: 'UTC',
        serviceWorkers: 'block',
        screenshot: 'only-on-failure',
        trace: 'retain-on-failure'
    },
    webServer: {
        command: 'node frontend_tests/ui/server.mjs',
        url: 'http://127.0.0.1:4173',
        reuseExistingServer: false,
        timeout: 10_000
    }
});
