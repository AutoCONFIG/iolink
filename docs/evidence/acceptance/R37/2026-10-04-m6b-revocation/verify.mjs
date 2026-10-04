import { execFileSync, spawn } from 'node:child_process'
import { mkdir, writeFile, readFile } from 'node:fs/promises'
import { createWriteStream } from 'node:fs'
const root = process.cwd()
const evidence = root + '/.omo/evidence/m6b-final'
await mkdir(evidence, { recursive: true })
const container = JSON.parse(execFileSync('docker', ['inspect', 'iolink-todo9-pg'], { encoding: 'utf8' }))[0]
const settings = Object.fromEntries(container.Config.Env.map(value => value.split(/=(.*)/s).slice(0, 2)))
const env = { ...process.env, GOFLAGS: '-buildvcs=false', IOLINK_TEST_PG_DSN: `postgres://${settings.POSTGRES_USER}:${settings.POSTGRES_PASSWORD}@127.0.0.1:55439/${settings.POSTGRES_DB || settings.POSTGRES_USER}?sslmode=disable` }
const results = []
async function run(name, command, args, cwd = root) {
  const file = evidence + '/' + name + '.log'
  const stream = createWriteStream(file)
  stream.write('$ ' + command + ' ' + args.join(' ') + '\n')
  const start = Date.now()
  const child = spawn(command, args, { cwd, env, stdio: ['ignore', 'pipe', 'pipe'] })
  child.stdout.pipe(stream, { end: false }); child.stderr.pipe(stream, { end: false })
  const code = await new Promise((resolve, reject) => { child.once('error', reject); child.once('close', resolve) })
  await new Promise(resolve => stream.end('\nEXIT ' + code + '\n', resolve))
  const result = { name, code, seconds: (Date.now() - start) / 1000, command: [command, ...args], cwd, artifact: file }
  results.push(result); console.log(JSON.stringify(result))
  if (code !== 0) throw new Error(name + ' failed; see ' + file)
}
await run('web-unit', 'npm', ['test'], root + '/web')
await run('web-typecheck', 'npm', ['run', 'typecheck'], root + '/web')
await run('web-build', 'npm', ['run', 'build'], root + '/web')
await run('docs-tools', 'make', ['docs-tools'])
await writeFile('/tmp/m6b-final-playwright.config.mjs', `import {defineConfig} from '${root}/web/node_modules/@playwright/test/index.mjs'; export default defineConfig({testDir:'${root}/web/e2e',outputDir:'${evidence}/browser-results',timeout:30000,workers:2,use:{baseURL:'http://127.0.0.1:5197',channel:'chrome',viewport:{width:1440,height:960},trace:'on'},webServer:{command:'npm run dev -- --host 127.0.0.1 --port 5197 --strictPort',cwd:'${root}/web',url:'http://127.0.0.1:5197',reuseExistingServer:false,timeout:30000}})`)
await Promise.all([
  run('go-verify', 'make', ['verify']),
  run('contracts', 'make', ['verify-contracts']),
  run('architecture', '.venv/contracts/bin/python', ['scripts/check_architecture_manifests.py', '--all']),
  run('web-e2e', 'npm', ['run', 'e2e', '--', '--config', '/tmp/m6b-final-playwright.config.mjs'], root + '/web'),
])
await run('m6b-focused', 'go', ['test', '-race', '-shuffle=on', '-count=1', '-v', './internal/core', './internal/migrate', './internal/appapi', './internal/adminapi', '-run', 'Test(M6b|AppResource|AppTelemetry|AppAlarm|TelemetrySubmission|TelemetryHTTP|LegacyTenant|Platform|ProductionMux)'])
await run('m6b-revocation', 'go', ['test', '-race', '-shuffle=on', '-count=1', '-v', './internal/core', '-run', 'Test(Reviewer|M6bFakeExecutor)'])
await run('diff-check', 'git', ['diff', '--check'])
const metadata = { source: execFileSync('git', ['rev-parse', 'HEAD'], { cwd: root, encoding: 'utf8' }).trim(), web: execFileSync('git', ['-C', 'web', 'rev-parse', 'HEAD'], { cwd: root, encoding: 'utf8' }).trim(), date: '2026-10-03', container: 'iolink-todo9-pg', dsn: '<redacted isolated DSN>', postgres: execFileSync('docker', ['exec', 'iolink-todo9-pg', 'psql', '-U', settings.POSTGRES_USER, '-d', settings.POSTGRES_DB || settings.POSTGRES_USER, '-Atc', "SELECT version(); SELECT extversion FROM pg_extension WHERE extname='timescaledb'"], { encoding: 'utf8' }).trim(), results }
await writeFile(evidence + '/verification.json', JSON.stringify(metadata, null, 2) + '\n')
const focused = await readFile(evidence + '/m6b-focused.log', 'utf8')
if (focused.includes('--- SKIP:')) throw new Error('Focused database checks skipped')
console.log('ALL VERIFICATION PASSED')
