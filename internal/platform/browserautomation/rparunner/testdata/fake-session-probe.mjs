let input = ''
for await (const chunk of process.stdin) input += chunk
const payload = JSON.parse(input)
if (payload.account_id !== '1855554434276391') process.exit(2)
process.stdout.write(JSON.stringify({
  schema_version: 'browser-rpa-edge-session-probe/v1',
  checked_at: '2026-08-25T10:00:00Z',
  status: 'ready',
  reason: 'session_ready',
  cdp_available: true,
  oceanengine_page_available: true,
  logged_in: true,
  account_matched: true,
}))
