import { basename } from 'node:path'
import { chromium } from '@playwright/test'
import { resolveSessionPlaywrightEndpoint } from './browser-rpa-edge-session.js'

export const sessionProbeSchema = 'browser-rpa-edge-session-probe/v1' as const

export type EdgeSessionProbe = {
  schema_version: typeof sessionProbeSchema
  checked_at: string
  status: 'ready' | 'blocked'
  reason: 'session_ready' | 'cdp_unavailable' | 'oceanengine_page_missing' | 'login_required' | 'account_mismatch'
  cdp_available: boolean
  oceanengine_page_available: boolean
  logged_in: boolean
  account_matched: boolean
}

export function inspectOceanEnginePages(pageURLs: string[], expectedAccountID: string): Omit<EdgeSessionProbe, 'schema_version' | 'checked_at'> {
  const oceanEngineURLs = pageURLs.flatMap(value => {
    try {
      const url = new URL(value)
      return url.hostname.toLowerCase() === 'ad.oceanengine.com' ? [url] : []
    } catch {
      return []
    }
  })
  const loggedInURLs = oceanEngineURLs.filter(url => !/(^|\/)(login|passport|signin)(\/|$)/i.test(url.pathname))
  const accountMatched = loggedInURLs.some(url => url.searchParams.get('aadvid') === expectedAccountID)
  const reason = oceanEngineURLs.length === 0
    ? 'oceanengine_page_missing'
    : loggedInURLs.length === 0
      ? 'login_required'
      : !accountMatched
        ? 'account_mismatch'
        : 'session_ready'
  return {
    status: reason === 'session_ready' ? 'ready' : 'blocked',
    reason,
    cdp_available: true,
    oceanengine_page_available: oceanEngineURLs.length > 0,
    logged_in: loggedInURLs.length > 0,
    account_matched: accountMatched,
  }
}

function readArgument(name: string) {
  const index = process.argv.indexOf(name)
  return index >= 0 ? process.argv[index + 1] : undefined
}

async function readExpectedAccountID() {
  let source = ''
  for await (const chunk of process.stdin) source += String(chunk)
  const payload = JSON.parse(source) as { account_id?: unknown }
  if (typeof payload.account_id !== 'string' || !/^\d+$/.test(payload.account_id)) throw new Error('Invalid account id')
  return payload.account_id
}

export async function probeEdgeSession(sessionFile: string, expectedAccountID: string): Promise<EdgeSessionProbe> {
  const checkedAt = new Date().toISOString()
  try {
    const endpoint = await resolveSessionPlaywrightEndpoint(sessionFile)
    const browser = await chromium.connectOverCDP(endpoint)
    const deadline = Date.now() + 10_000
    let inspected = inspectOceanEnginePages([], expectedAccountID)
    do {
      inspected = inspectOceanEnginePages(
        browser.contexts().flatMap(context => context.pages()).filter(page => !page.isClosed()).map(page => page.url()),
        expectedAccountID,
      )
      // The platform can add aadvid after the shell loads. Do not classify a
      // temporary missing or different value until the bounded wait ends.
      if (inspected.status === 'ready') break
      await new Promise(resolveTimer => setTimeout(resolveTimer, 250))
    } while (Date.now() < deadline)
    return { schema_version: sessionProbeSchema, checked_at: checkedAt, ...inspected }
  } catch {
    return {
      schema_version: sessionProbeSchema,
      checked_at: checkedAt,
      status: 'blocked',
      reason: 'cdp_unavailable',
      cdp_available: false,
      oceanengine_page_available: false,
      logged_in: false,
      account_matched: false,
    }
  }
}

if (basename(process.argv[1] ?? '') === 'browser-rpa-session-probe.ts') {
  const sessionFile = readArgument('--session-file')
  if (!sessionFile) {
    process.stderr.write('Usage: browser-rpa-session-probe.ts --session-file <path>\n')
    process.exitCode = 1
  } else {
    readExpectedAccountID().then(accountID => probeEdgeSession(sessionFile, accountID)).then(result => {
      process.stdout.write(`${JSON.stringify(result)}\n`, () => process.exit(0))
    }).catch(() => {
      process.stderr.write('Edge session probe failed\n', () => process.exit(1))
    })
  }
}
