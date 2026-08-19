// Fake RPA runner used by runner_test.go. The "CDP endpoint" argument doubles
// as the behavior selector. The plan arrives on stdin exactly like the real
// runner; the response is a single JSON result document on stdout.
import { createInterface } from 'node:readline'

const mode = process.argv[2] ?? 'success'
let data = ''
const stdin = createInterface({ input: process.stdin })
stdin.on('line', line => { data += line })
stdin.on('close', () => {
  const plan = JSON.parse(data)
  if (mode === 'garbage') {
    process.stdout.write('this is not json')
    return
  }
  if (mode === 'wrong-schema') {
    process.stdout.write(JSON.stringify({ schema_version: 'bogus/v0', outcome: 'success', error_code: 'ok', final_click_performed: false, steps: [] }))
    return
  }
  if (mode === 'page-drift') {
    process.stdout.write(JSON.stringify({
      schema_version: 'oceanengine-playwright-rpa-result/v1',
      outcome: 'failed',
      error_code: 'page_drift',
      error_message: 'scope locator missing',
      final_click_performed: false,
      steps: [],
    }))
    return
  }
  if (mode === 'noisy') {
    process.stdout.write('warning: third-party library noise on stdout\n')
  }
  process.stdout.write(JSON.stringify({
    schema_version: 'oceanengine-playwright-rpa-result/v1',
    outcome: 'success',
    error_code: 'ok',
    final_click_performed: false,
    steps: [{
      id: plan.steps?.[0]?.id ?? 'step',
      status: 'succeeded',
      before_facts: { page_kind: plan.steps?.[0]?.page_kind ?? '' },
      readback: { object_id: 'promotion_test', plan_mode: plan.mode ?? '' },
      diff_keys: [],
      page_reference: 'https://ad.oceanengine.com/promotion/list',
    }],
  }))
})
