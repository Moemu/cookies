import { readFile, writeFile } from 'node:fs/promises'
import { join } from 'node:path'
import { chromium } from '@playwright/test'

function safePageURL(value) {
  const url = new URL(value)
  return { protocol: url.protocol, host: url.host, pathname: url.pathname }
}

function isOceanEnginePage(value) {
  try {
    return new URL(value).hostname.toLowerCase() === 'ad.oceanengine.com'
  } catch {
    return false
  }
}

async function main() {
  const [endpoint, screenshotPath, currentUserDataRoot] = process.argv.slice(2)
  if (!endpoint || !screenshotPath) throw new Error('Playwright attachment needs an endpoint and screenshot path')
  const endpointURL = new URL(endpoint)
  if (endpointURL.protocol !== 'http:' || endpointURL.hostname !== '127.0.0.1') {
    throw new Error('CDP endpoint must use http://127.0.0.1')
  }

  let playwrightEndpoint = endpoint
  if (currentUserDataRoot) {
    const lines = (await readFile(join(currentUserDataRoot, 'DevToolsActivePort'), 'utf8')).split(/\r?\n/)
    const port = Number.parseInt(lines[0] ?? '', 10)
    const browserPath = lines[1] ?? ''
    if (port !== Number.parseInt(endpointURL.port, 10) || !/^\/devtools\/browser\/[A-Za-z0-9-]+$/.test(browserPath)) {
      throw new Error('Current Edge DevTools endpoint metadata is invalid')
    }
    playwrightEndpoint = `ws://127.0.0.1:${port}${browserPath}`
  }

  const browser = await chromium.connectOverCDP(playwrightEndpoint)
  const deadline = Date.now() + 15000
  let page
  do {
    page = browser.contexts()
      .flatMap(context => context.pages())
      .filter(item => !item.isClosed() && isOceanEnginePage(item.url()))
      .at(-1)
    if (!page) await new Promise(resolveTimer => setTimeout(resolveTimer, 250))
  } while (!page && Date.now() < deadline)
  if (!page) throw new Error('No current ad.oceanengine.com page is open')

  const pageCDP = await page.context().newCDPSession(page)
  const target = await pageCDP.send('Target.getTargetInfo')
  const targetInfo = target.targetInfo
  if (!targetInfo?.targetId) {
    throw new Error('Playwright did not return stable page and context identifiers')
  }
  const capture = await pageCDP.send('Page.captureScreenshot', { format: 'png', fromSurface: true })
  await writeFile(screenshotPath, Buffer.from(capture.data, 'base64'), { mode: 0o600 })
  return {
    browser_context_id: targetInfo.browserContextId ?? 'default',
    target_id: targetInfo.targetId,
    page: safePageURL(page.url()),
    screenshot_path: screenshotPath,
  }
}

main().then(result => {
  process.stdout.write(`${JSON.stringify(result)}\n`, () => process.exit(0))
}).catch(error => {
  const safeMessage = error instanceof Error && /^(Playwright did not|Expected one|No current|Current Edge|CDP endpoint|Playwright attachment)/.test(error.message)
    ? error.message
    : 'Playwright attachment failed'
  process.stderr.write(`${safeMessage}\n`, () => process.exit(1))
})
