import {spawn} from 'node:child_process';
import {mkdir, mkdtemp, readFile, readdir, rm, writeFile} from 'node:fs/promises';
import path from 'node:path';
import {fileURLToPath} from 'node:url';

const root = fileURLToPath(new URL('../', import.meta.url));
const artifacts = path.join(root, 'build', 'test-results');
const crypto = './backend/switchfs/_crypto';
const core = ['db', 'fileio', 'process', 'settings', 'switchfs', 'switchfs/_crypto'].map(p => `./backend/${p}`);
const mode = process.argv[2] || 'all';
const extra = process.argv.slice(3);
if (Number(process.versions.node.split('.')[0]) !== 24) throw new Error('Validation requires Node.js 24.');
if (!['all', 'backend', 'unit', 'ui', 'coverage', 'race'].includes(mode)) throw new Error(`Unknown test suite: ${mode}`);
await mkdir(artifacts, {recursive: true});
// Go ignores directories beginning with an underscore during ./... discovery.
const temporary = await mkdtemp(path.join(artifacts, '_tmp-'));
const env = {...process.env, TEMP: temporary, TMP: temporary, TMPDIR: temporary, GOTMPDIR: temporary};
// Normal validation must compare fixtures, even if a previous update command
// left its opt-in variable in the caller's shell.
delete env.SLM_UPDATE_UI_FIXTURES;

function run(command, args, capture = false, label = `${command} ${args.join(' ')}`) {
    console.log(`\n> ${label}`);
    return new Promise((resolve, reject) => {
        const child = spawn(command, args, {cwd: root, env, shell: false, stdio: capture ? ['ignore', 'pipe', 'inherit'] : 'inherit'});
        let output = '';
        if (capture) child.stdout.setEncoding('utf8').on('data', data => output += data);
        child.on('error', error => reject(new Error(`Could not start ${command}: ${error.message}`, {cause: error})));
        child.on('exit', (code, signal) => code === 0 ? resolve(output) : reject(new Error(`${command} failed (${signal || `exit ${code}`}).`)));
    });
}

async function backend() {
    // Git excludes node_modules, generated build products, and other ignored files.
    const files = (await run('git', ['ls-files', '-z', '--cached', '--others', '--exclude-standard', '--', '*.go'], true)).split('\0').filter(Boolean);
    // Windows checkouts may use CRLF. Check formatting on LF copies rather than
    // reporting every otherwise formatted source file or rewriting the checkout.
    const formatRoot = path.join(temporary, 'format');
    const copies = await Promise.all(files.map(async file => {
        const destination = path.join(formatRoot, file);
        await mkdir(path.dirname(destination), {recursive: true});
        await writeFile(destination, (await readFile(path.join(root, file), 'utf8')).replaceAll('\r\n', '\n'));
        return destination;
    }));
    const unformatted = (await run('gofmt', ['-l', ...copies], true, 'gofmt -l <Go sources, normalized line endings>')).replaceAll(formatRoot + path.sep, '');
    if (unformatted.trim()) throw new Error(`Go files need formatting:\n${unformatted}`);
    await run('go', ['test', './...']);
    await run('go', ['test', crypto]);
    await run('go', ['vet', './...', crypto]);
}

async function unit() {
    const files = (await readdir(path.join(root, 'frontend_tests'))).filter(name => name.endsWith('.test.mjs')).sort().map(name => `frontend_tests/${name}`);
    await run(process.execPath, ['--test', ...files]);
}

async function ui() {
    await run(process.execPath, ['node_modules/@playwright/test/cli.js', 'test', ...extra]);
}

try {
    if (mode === 'all' || mode === 'backend') await backend();
    if (mode === 'all' || mode === 'unit') await unit();
    if (mode === 'all' || mode === 'ui') await ui();
    if (mode === 'coverage') {
        const profile = path.join(artifacts, 'coverage.out');
        await run('go', ['test', '-count=1', '-covermode=atomic', `-coverprofile=${profile}`, ...core]);
        const report = await run('go', ['tool', 'cover', '-func', profile], true);
        console.log(report);
        const total = report.match(/^total:\s+\(statements\)\s+([\d.]+)%$/m);
        if (!total) throw new Error('Could not read aggregate core statement coverage.');
        if (Number(total[1]) < 90) throw new Error(`Core coverage ${total[1]}% is below 90%.`);
    }
    if (mode === 'race') {
        if ((await run('go', ['env', 'CGO_ENABLED'], true)).trim() !== '1') throw new Error('Race tests require CGO_ENABLED=1 and a supported C compiler. Run on the Linux CI runner if unavailable locally.');
        await run('go', ['test', '-race', './...', crypto]);
    }
} catch (error) {
    console.error(error.message);
    process.exitCode = 1;
} finally {
    // Only remove this invocation's owned temporary directory, never user data.
    if (path.dirname(temporary) !== artifacts || !path.basename(temporary).startsWith('_tmp-')) throw new Error('Unsafe test cleanup path.');
    try {
        await rm(temporary, {recursive: true, force: true, maxRetries: 3, retryDelay: 200});
    } catch (error) {
        console.error(`Could not clean test temporary directory ${temporary}: ${error.message}`);
        process.exitCode = 1;
    }
}
