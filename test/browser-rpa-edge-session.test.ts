import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { join, resolve } from 'node:path'
import test from 'node:test'
import { currentUserProfilePath, edgeArguments, isLikelyOceanEngineLogin, sanitizePageURL, sessionPaths } from '../scripts/browser-rpa-edge-session.js'

test('session files stay under LOCALAPPDATA and use the managed Default profile', () => {
  const localAppData = resolve('C:/Users/test/AppData/Local')
  const paths = sessionPaths(localAppData)
  const expectedRoot = join(localAppData, 'cookies', 'browser-rpa')
  assert.equal(paths.root, expectedRoot)
  assert.equal(paths.profile, join(expectedRoot, 'edge-user-data', 'Default'))
  for (const path of Object.values(paths)) assert.ok(path.startsWith(expectedRoot))
})

test('Edge arguments keep the browser visible and bind CDP to loopback', () => {
  const paths = sessionPaths(resolve('C:/Users/test/AppData/Local'))
  const args = edgeArguments(paths, 19222)
  assert.ok(args.includes('--profile-directory=Default'))
  assert.ok(args.includes('--remote-debugging-address=127.0.0.1'))
  assert.ok(args.includes('--remote-debugging-port=19222'))
  assert.ok(args.includes(`--user-data-dir=${paths.userData}`))
  assert.ok(args.includes('https://ad.oceanengine.com/'))
  assert.ok(!args.some(value => /headless/i.test(value)))
})

test('current profile mode points to the existing Edge Default Profile without copying it', () => {
  const localAppData = resolve('C:/Users/test/AppData/Local')
  assert.equal(currentUserProfilePath(localAppData), join(localAppData, 'Microsoft', 'Edge', 'User Data', 'Default'))
  assert.notEqual(currentUserProfilePath(localAppData), sessionPaths(localAppData).profile)
})

test('page diagnostics remove query strings and fragments', () => {
  assert.deepEqual(sanitizePageURL('https://ad.oceanengine.com/path/item?token=secret#fragment'), {
    protocol: 'https:',
    host: 'ad.oceanengine.com',
    pathname: '/path/item',
  })
})

test('login check uses only the safe page location', () => {
  assert.equal(isLikelyOceanEngineLogin({ protocol: 'https:', host: 'ad.oceanengine.com', pathname: '/campaign' }), true)
  assert.equal(isLikelyOceanEngineLogin({ protocol: 'https:', host: 'ad.oceanengine.com', pathname: '/login' }), false)
  assert.equal(isLikelyOceanEngineLogin({ protocol: 'https:', host: 'open.oceanengine.com', pathname: '/' }), false)
})

test('session check has no credential, storage, network, or page-write operation', () => {
  const source = [
    readFileSync(resolve(import.meta.dirname, '../scripts/browser-rpa-edge-session.ts'), 'utf8'),
    readFileSync(resolve(import.meta.dirname, '../scripts/browser-rpa-edge-attach-once.mjs'), 'utf8'),
  ].join('\n')
  for (const forbidden of [
    /\.cookies\s*\(/,
    /localStorage/,
    /sessionStorage/,
    /storageState\s*\(/,
    /\.getAllCookies\s*\(/,
    /\.setCookie\s*\(/,
    /\.setExtraHTTPHeaders\s*\(/,
    /\.route\s*\(/,
    /page\.goto\s*\(/,
    /\.click\s*\(/,
    /\.fill\s*\(/,
    /\.selectOption\s*\(/,
    /\.setInputFiles\s*\(/,
    /\.download\s*\(/,
  ]) assert.doesNotMatch(source, forbidden)
})

test('attachment disconnects CDP without closing the managed Edge browser', () => {
  const source = readFileSync(resolve(import.meta.dirname, '../scripts/browser-rpa-edge-attach-once.mjs'), 'utf8')
  assert.doesNotMatch(source, /browser\.close\s*\(/)
  assert.doesNotMatch(source, /Browser\.close/)
  assert.match(source, /chromium\.connectOverCDP\s*\(/)
  assert.match(source, /process\.exit\s*\(0\)/)
})
