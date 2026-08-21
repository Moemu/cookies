import { execFile, spawn } from 'node:child_process'
import { existsSync } from 'node:fs'
import { mkdir, readFile, rename, writeFile } from 'node:fs/promises'
import { createServer } from 'node:net'
import { basename, join, resolve } from 'node:path'
import { promisify } from 'node:util'

const execFileAsync = promisify(execFile)
const sessionSchema = 'cookies-browser-rpa-edge-session/v1'
const checkSchema = 'cookies-browser-rpa-attach-check/v1'
const defaultStartURL = 'https://ad.oceanengine.com/'
const edgeProfileName = 'Default'
const attachHelper = resolve(import.meta.dirname, 'browser-rpa-edge-attach-once.mjs')

type SessionMode = 'managed' | 'current_user'

export type SessionPaths = {
  root: string
  userData: string
  profile: string
  metadata: string
  diagnostics: string
  screenshots: string
}

type SessionMetadata = {
  schema_version: typeof sessionSchema
  state: 'running' | 'stopped'
  edge_path: string
  mode?: SessionMode
  profile_name: string
  profile_path: string
  cdp_endpoint: string
  cdp_host: '127.0.0.1'
  cdp_port: number
  started_at: string
  stopped_at?: string
}

type TargetSnapshot = {
  browser_context_id: string
  target_id: string
  page: { protocol: string; host: string; pathname: string }
  screenshot_path: string
}

type AttachCheck = {
  schema_version: typeof checkSchema
  checked_at: string
  outcome: 'success' | 'failed'
  reason: string
  same_browser_context: boolean
  same_page_target: boolean
  likely_logged_in: boolean
  attachments: TargetSnapshot[]
}

export function sessionPaths(localAppData = process.env.LOCALAPPDATA): SessionPaths {
  if (!localAppData) throw new Error('LOCALAPPDATA is not set')
  const root = resolve(localAppData, 'cookies', 'browser-rpa')
  const userData = join(root, 'edge-user-data')
  return {
    root,
    userData,
    profile: join(userData, edgeProfileName),
    metadata: join(root, 'session.json'),
    diagnostics: join(root, 'diagnostics.jsonl'),
    screenshots: join(root, 'screenshots'),
  }
}

export function edgeArguments(paths: SessionPaths, port: number): string[] {
  if (!Number.isInteger(port) || port < 1024 || port > 65535) throw new Error('CDP port is invalid')
  return [
    `--user-data-dir=${paths.userData}`,
    `--profile-directory=${edgeProfileName}`,
    '--remote-debugging-address=127.0.0.1',
    `--remote-debugging-port=${port}`,
    '--no-first-run',
    '--no-default-browser-check',
    '--new-window',
    defaultStartURL,
  ]
}

export function sanitizePageURL(value: string) {
  const url = new URL(value)
  return { protocol: url.protocol, host: url.host, pathname: url.pathname }
}

export function isLikelyOceanEngineLogin(page: TargetSnapshot['page']): boolean {
  const host = page.host.toLowerCase().split(':')[0]
  return host === 'ad.oceanengine.com' && !/(^|\/)(login|passport|signin)(\/|$)/i.test(page.pathname)
}

function findEdge(): string | undefined {
  const candidates = [
    process.env['PROGRAMFILES(X86)'] && join(process.env['PROGRAMFILES(X86)'], 'Microsoft', 'Edge', 'Application', 'msedge.exe'),
    process.env.PROGRAMFILES && join(process.env.PROGRAMFILES, 'Microsoft', 'Edge', 'Application', 'msedge.exe'),
    process.env.LOCALAPPDATA && join(process.env.LOCALAPPDATA, 'Microsoft', 'Edge', 'Application', 'msedge.exe'),
  ].filter((value): value is string => Boolean(value))
  return candidates.find(candidate => existsSync(candidate))
}

export function currentUserProfilePath(localAppData = process.env.LOCALAPPDATA) {
  if (!localAppData) throw new Error('LOCALAPPDATA is not set')
  return resolve(localAppData, 'Microsoft', 'Edge', 'User Data', edgeProfileName)
}

function currentUserDataRoot(localAppData = process.env.LOCALAPPDATA) {
  return resolve(currentUserProfilePath(localAppData), '..')
}

async function atomicJSON(path: string, value: unknown) {
  const temporary = `${path}.${process.pid}.tmp`
  await writeFile(temporary, `${JSON.stringify(value, null, 2)}\n`, { encoding: 'utf8', mode: 0o600 })
  await rename(temporary, path)
}

async function appendDiagnostic(paths: SessionPaths, command: string, outcome: string, reason: string) {
  await mkdir(paths.root, { recursive: true })
  const record = { schema_version: 'cookies-browser-rpa-diagnostic/v1', recorded_at: new Date().toISOString(), command, outcome, reason }
  await writeFile(paths.diagnostics, `${JSON.stringify(record)}\n`, { encoding: 'utf8', flag: 'a', mode: 0o600 })
}

async function readMetadata(paths: SessionPaths): Promise<SessionMetadata | undefined> {
  try {
    const value = JSON.parse(await readFile(paths.metadata, 'utf8')) as SessionMetadata
    return value.schema_version === sessionSchema ? value : undefined
  } catch {
    return undefined
  }
}

async function unusedPort(): Promise<number> {
  return await new Promise((resolvePromise, rejectPromise) => {
    const server = createServer()
    server.once('error', rejectPromise)
    server.listen(0, '127.0.0.1', () => {
      const address = server.address()
      if (!address || typeof address === 'string') return rejectPromise(new Error('Could not allocate a loopback port'))
      const port = address.port
      server.close(error => error ? rejectPromise(error) : resolvePromise(port))
    })
  })
}

async function waitForCDP(endpoint: string, timeoutMs = 15000) {
  const deadline = Date.now() + timeoutMs
  let lastReason = 'CDP did not respond'
  while (Date.now() < deadline) {
    try {
      const response = await fetch(`${endpoint}/json/version`, { signal: AbortSignal.timeout(1000) })
      if (response.ok) return
      lastReason = `CDP returned HTTP ${response.status}`
    } catch (error) {
      lastReason = error instanceof Error ? error.message : String(error)
    }
    await new Promise(resolvePromise => setTimeout(resolvePromise, 200))
  }
  throw new Error(lastReason)
}

function assertLoopbackEndpoint(endpoint: string) {
  const url = new URL(endpoint)
  if (url.protocol !== 'http:' || url.hostname !== '127.0.0.1') {
    throw new Error('CDP endpoint must use http://127.0.0.1')
  }
}

async function matchingEdgeProcess(metadata: SessionMetadata): Promise<boolean> {
  if (metadata.mode === 'current_user') return await edgeOwnsListener(metadata.cdp_port)
  if (process.platform !== 'win32') return false
  const script = [
    '$ErrorActionPreference = "Stop"',
    '$portArg = "--remote-debugging-port=" + $env:COOKIES_RPA_CHECK_PORT',
    '$profileArg = "--user-data-dir=" + $env:COOKIES_RPA_CHECK_PROFILE_ROOT',
    '$match = Get-CimInstance Win32_Process -Filter "Name = \'msedge.exe\'" | Where-Object { $_.CommandLine -and $_.CommandLine.Contains($portArg) -and $_.CommandLine.Contains($profileArg) } | Select-Object -First 1',
    'if ($match) { "true" } else { "false" }',
  ].join('; ')
  const { stdout } = await execFileAsync('powershell.exe', ['-NoProfile', '-NonInteractive', '-Command', script], {
    windowsHide: true,
    env: { ...process.env, COOKIES_RPA_CHECK_PORT: String(metadata.cdp_port), COOKIES_RPA_CHECK_PROFILE_ROOT: resolve(metadata.profile_path, '..') },
  })
  return stdout.trim() === 'true'
}

async function edgeListenerPorts(): Promise<number[]> {
  if (process.platform !== 'win32') return []
  const script = [
    '$ErrorActionPreference = "Stop"',
    '$edgePids = @(Get-CimInstance Win32_Process -Filter "Name = \'msedge.exe\'" | Select-Object -ExpandProperty ProcessId)',
    '$ports = @(Get-NetTCPConnection -State Listen -ErrorAction SilentlyContinue | Where-Object { $_.LocalAddress -eq "127.0.0.1" -and $edgePids -contains $_.OwningProcess } | Select-Object -ExpandProperty LocalPort -Unique)',
    '$ports | ConvertTo-Json -Compress',
  ].join('; ')
  const { stdout } = await execFileAsync('powershell.exe', ['-NoProfile', '-NonInteractive', '-Command', script], { windowsHide: true })
  const parsed = stdout.trim() ? JSON.parse(stdout.trim()) as number | number[] : []
  return (Array.isArray(parsed) ? parsed : [parsed]).filter(port => Number.isInteger(port))
}

async function edgeOwnsListener(port: number) {
  return (await edgeListenerPorts()).includes(port)
}

async function discoverCurrentEdgeEndpoint() {
  const script = [
    '$ErrorActionPreference = "Stop"',
    '$line = Get-Content -LiteralPath $env:COOKIES_RPA_DEVTOOLS_PORT_FILE -TotalCount 1 -ErrorAction Stop',
    'if ($line -match "^[0-9]+$") { $line } else { throw "Invalid DevTools port" }',
  ].join('; ')
  try {
    const { stdout } = await execFileAsync('powershell.exe', ['-NoProfile', '-NonInteractive', '-Command', script], {
      windowsHide: true,
      env: { ...process.env, COOKIES_RPA_DEVTOOLS_PORT_FILE: join(currentUserDataRoot(), 'DevToolsActivePort') },
    })
    const port = Number.parseInt(stdout.trim(), 10)
    if (!Number.isInteger(port) || !await edgeOwnsListener(port)) return undefined
    return `http://127.0.0.1:${port}`
  } catch {
    return undefined
  }
}

async function loopbackOnlyListener(port: number): Promise<boolean> {
  if (process.platform !== 'win32') return false
  const script = [
    '$ErrorActionPreference = "Stop"',
    '$items = @(Get-NetTCPConnection -State Listen -LocalPort ([int]$env:COOKIES_RPA_CHECK_PORT) -ErrorAction Stop)',
    'if ($items.Count -gt 0 -and @($items | Where-Object { $_.LocalAddress -ne "127.0.0.1" }).Count -eq 0) { "true" } else { "false" }',
  ].join('; ')
  const { stdout } = await execFileAsync('powershell.exe', ['-NoProfile', '-NonInteractive', '-Command', script], {
    windowsHide: true,
    env: { ...process.env, COOKIES_RPA_CHECK_PORT: String(port) },
  })
  return stdout.trim() === 'true'
}

async function stopMatchingEdge(metadata: SessionMetadata) {
  if (process.platform !== 'win32') throw new Error('The Edge session tool supports Windows only')
  const script = [
    '$ErrorActionPreference = "Stop"',
    '$portArg = "--remote-debugging-port=" + $env:COOKIES_RPA_CHECK_PORT',
    '$profileArg = "--user-data-dir=" + $env:COOKIES_RPA_CHECK_PROFILE_ROOT',
    '$matches = @(Get-CimInstance Win32_Process -Filter "Name = \'msedge.exe\'" | Where-Object { $_.CommandLine -and $_.CommandLine.Contains($portArg) -and $_.CommandLine.Contains($profileArg) })',
    'foreach ($item in $matches) { Stop-Process -Id $item.ProcessId -Force -ErrorAction Stop }',
    '$matches.Count',
  ].join('; ')
  const { stdout } = await execFileAsync('powershell.exe', ['-NoProfile', '-NonInteractive', '-Command', script], {
    windowsHide: true,
    env: { ...process.env, COOKIES_RPA_CHECK_PORT: String(metadata.cdp_port), COOKIES_RPA_CHECK_PROFILE_ROOT: resolve(metadata.profile_path, '..') },
  })
  return Number.parseInt(stdout.trim(), 10) || 0
}

async function startCommand(paths: SessionPaths) {
  if (process.platform !== 'win32') throw new Error('The Edge session tool supports Windows only')
  const edgePath = findEdge()
  if (!edgePath) throw new Error('Microsoft Edge was not found')
  await mkdir(paths.profile, { recursive: true })
  await mkdir(paths.screenshots, { recursive: true })

  const existing = await readMetadata(paths)
  if (existing?.state === 'running' && existing.mode !== 'current_user') {
    assertLoopbackEndpoint(existing.cdp_endpoint)
    if (await matchingEdgeProcess(existing)) {
      await waitForCDP(existing.cdp_endpoint)
      await appendDiagnostic(paths, 'start', 'reused', 'existing managed Edge session is running')
      return existing
    }
  }

  const port = await unusedPort()
  const endpoint = `http://127.0.0.1:${port}`
  const child = spawn(edgePath, edgeArguments(paths, port), { detached: true, stdio: 'ignore', windowsHide: false })
  child.unref()
  try {
    await waitForCDP(endpoint)
    if (!await loopbackOnlyListener(port)) {
      throw new Error('CDP listener is not restricted to 127.0.0.1')
    }
  } catch (error) {
    await appendDiagnostic(paths, 'start', 'failed', `Edge started but loopback CDP was unavailable: ${error instanceof Error ? error.message : error}`)
    throw error
  }
  const metadata: SessionMetadata = {
    schema_version: sessionSchema,
    state: 'running',
    mode: 'managed',
    edge_path: edgePath,
    profile_name: edgeProfileName,
    profile_path: paths.profile,
    cdp_endpoint: endpoint,
    cdp_host: '127.0.0.1',
    cdp_port: port,
    started_at: new Date().toISOString(),
  }
  await atomicJSON(paths.metadata, metadata)
  await appendDiagnostic(paths, 'start', 'success', 'visible managed Edge started with loopback-only CDP')
  return metadata
}

async function startCurrentCommand(paths: SessionPaths) {
  if (process.platform !== 'win32') throw new Error('The Edge session tool supports Windows only')
  const edgePath = findEdge()
  if (!edgePath) throw new Error('Microsoft Edge was not found')
  await mkdir(paths.screenshots, { recursive: true })
  const endpoint = await discoverCurrentEdgeEndpoint()
  if (!endpoint) {
    const child = spawn(edgePath, ['edge://inspect/#remote-debugging'], { detached: true, stdio: 'ignore', windowsHide: false })
    child.unref()
    const reason = 'current_profile_remote_debugging_requires_user_enablement'
    await appendDiagnostic(paths, 'start-current', 'failed', reason)
    throw new Error(`${reason}: enable "Allow remote debugging for this browser instance" in the visible Edge page`)
  }
  const port = Number.parseInt(new URL(endpoint).port, 10)
  if (!await loopbackOnlyListener(port)) throw new Error('Current Edge CDP listener is not restricted to 127.0.0.1')
  const profilePath = currentUserProfilePath()
  const metadata: SessionMetadata = {
    schema_version: sessionSchema,
    state: 'running',
    mode: 'current_user',
    edge_path: edgePath,
    profile_name: edgeProfileName,
    profile_path: profilePath,
    cdp_endpoint: endpoint,
    cdp_host: '127.0.0.1',
    cdp_port: port,
    started_at: new Date().toISOString(),
  }
  await atomicJSON(paths.metadata, metadata)
  spawn(edgePath, [defaultStartURL], { detached: true, stdio: 'ignore', windowsHide: false }).unref()
  await appendDiagnostic(paths, 'start-current', 'success', 'attached tool metadata to current user Edge Profile with loopback-only CDP')
  return metadata
}

async function statusCommand(paths: SessionPaths) {
  const metadata = await readMetadata(paths)
  if (!metadata || metadata.state !== 'running') return { state: 'stopped', reason: 'no running session metadata' }
  assertLoopbackEndpoint(metadata.cdp_endpoint)
  const processMatches = await matchingEdgeProcess(metadata)
  let cdpAvailable = false
  let loopbackOnly = false
  try {
    if (metadata.mode === 'current_user') cdpAvailable = await edgeOwnsListener(metadata.cdp_port)
    else {
      await waitForCDP(metadata.cdp_endpoint, 1500)
      cdpAvailable = true
    }
    loopbackOnly = await loopbackOnlyListener(metadata.cdp_port)
  } catch {
    // The status result records this failure without exposing browser data.
  }
  const state = processMatches && cdpAvailable && loopbackOnly ? 'running' : 'unhealthy'
  await appendDiagnostic(paths, 'status', state, state === 'running' ? 'selected Edge and loopback CDP are available' : 'process or loopback CDP is unavailable')
  return { state, cdp_endpoint: metadata.cdp_endpoint, profile_path: metadata.profile_path, process_matches: processMatches, cdp_available: cdpAvailable, loopback_only: loopbackOnly }
}

async function attachOnce(endpoint: string, screenshotPath: string, currentProfile = false): Promise<TargetSnapshot> {
  assertLoopbackEndpoint(endpoint)
  const argumentsList = [attachHelper, endpoint, screenshotPath]
  if (currentProfile) argumentsList.push(currentUserDataRoot())
  const { stdout } = await execFileAsync(process.execPath, argumentsList, {
    timeout: 20000,
    windowsHide: true,
    maxBuffer: 1 << 20,
  })
  return JSON.parse(stdout) as TargetSnapshot
}

async function checkCommand(paths: SessionPaths): Promise<AttachCheck> {
  const metadata = await readMetadata(paths)
  const checkedAt = new Date().toISOString()
  if (!metadata || metadata.state !== 'running') throw new Error('Start the managed Edge session before the attach check')
  assertLoopbackEndpoint(metadata.cdp_endpoint)
  await mkdir(paths.screenshots, { recursive: true })
  const prefix = checkedAt.replaceAll(':', '-').replaceAll('.', '-')
  const attachments: TargetSnapshot[] = []
  let result: AttachCheck
  try {
    const currentProfile = metadata.mode === 'current_user'
    attachments.push(await attachOnce(metadata.cdp_endpoint, join(paths.screenshots, `${prefix}-attach-1.png`), currentProfile))
    attachments.push(await attachOnce(metadata.cdp_endpoint, join(paths.screenshots, `${prefix}-attach-2.png`), currentProfile))
    const sameContext = attachments[0].browser_context_id === attachments[1].browser_context_id
    const samePage = attachments[0].target_id === attachments[1].target_id
    const loggedIn = attachments.every(item => isLikelyOceanEngineLogin(item.page))
    const success = sameContext && samePage && loggedIn
    const reason = !sameContext ? 'browser_context_changed' : !samePage ? 'current_page_changed' : !loggedIn ? 'manual_login_required' : 'same_logged_in_context_and_page_reused'
    result = { schema_version: checkSchema, checked_at: checkedAt, outcome: success ? 'success' : 'failed', reason, same_browser_context: sameContext, same_page_target: samePage, likely_logged_in: loggedIn, attachments }
  } catch (error) {
    result = { schema_version: checkSchema, checked_at: checkedAt, outcome: 'failed', reason: `attach_failed:${error instanceof Error ? error.message : error}`, same_browser_context: false, same_page_target: false, likely_logged_in: false, attachments }
  }
  await atomicJSON(join(paths.root, 'last-attach-check.json'), result)
  await appendDiagnostic(paths, 'check', result.outcome, result.reason)
  return result
}

async function stopCommand(paths: SessionPaths) {
  const metadata = await readMetadata(paths)
  if (!metadata || metadata.state !== 'running') {
    await appendDiagnostic(paths, 'stop', 'noop', 'no managed Edge session is running')
    return { state: 'stopped', stopped_processes: 0 }
  }
  assertLoopbackEndpoint(metadata.cdp_endpoint)
  const currentUser = metadata.mode === 'current_user'
  const stopped = currentUser ? 0 : await stopMatchingEdge(metadata)
  const next = { ...metadata, state: 'stopped' as const, stopped_at: new Date().toISOString() }
  await atomicJSON(paths.metadata, next)
  const reason = currentUser ? 'detached tool metadata; current user Edge remains open' : `stopped ${stopped} managed Edge process(es)`
  await appendDiagnostic(paths, 'stop', 'success', reason)
  return { state: 'stopped', stopped_processes: stopped, current_edge_remains_open: currentUser }
}

export async function runCLI(command: string) {
  const paths = sessionPaths()
  if (command === 'start') return await startCommand(paths)
  if (command === 'start-current') return await startCurrentCommand(paths)
  if (command === 'status') return await statusCommand(paths)
  if (command === 'check') return await checkCommand(paths)
  if (command === 'stop') return await stopCommand(paths)
  throw new Error('Usage: npm run browser-rpa:edge -- <start|start-current|status|check|stop>')
}

if (basename(process.argv[1] ?? '') === 'browser-rpa-edge-session.ts') {
  runCLI(process.argv[2] ?? '').then(result => {
    process.stdout.write(`${JSON.stringify(result, null, 2)}\n`)
    if ('outcome' in result && result.outcome !== 'success') process.exitCode = 2
  }).catch(async error => {
    try {
      const paths = sessionPaths()
      await appendDiagnostic(paths, process.argv[2] ?? 'unknown', 'failed', error instanceof Error ? error.message : String(error))
    } catch {
      // Preserve the primary error when diagnostics cannot be written.
    }
    process.stderr.write(`${error instanceof Error ? error.message : String(error)}\n`)
    process.exitCode = 1
  })
}
