import assert from 'node:assert/strict'
import test from 'node:test'
import { deliveryMechanisticSimulationApi, type MechanisticPriorSet } from '../src/api/delivery.ts'

test('mechanistic simulation uses a frozen plan version and explicit priors', async t => {
  const originalFetch = globalThis.fetch
  let request: { url: string; init?: RequestInit } | undefined
  globalThis.fetch = async (url, init) => {
    request = { url: String(url), init }
    return new Response(JSON.stringify({ result: {
      id: 'run_1', plan_id: 'plan/1', plan_version: 3, model_version: 'delivery-mechanistic-monte-carlo/v0.2',
      prior_set_version: 'operator/v1', stable_seed: 'stable', prediction_horizon: 'P7D', sample_count: 5000,
      status: 'completed', is_simulated: true, calibration_status: 'assumption_driven', metric_windows: [],
      scenario_probabilities: [], alerts: [], recommendation_drafts: [], assumptions: [], limitations: [], evidence_refs: [],
    } }), { headers: { 'Content-Type': 'application/json' } })
  }
  t.after(() => { globalThis.fetch = originalFetch })

  const prior = samplePrior()
  const result = await deliveryMechanisticSimulationApi.run('project one', 'plan/1', 3, { stableSeed: 'stable', sampleCount: 5000, predictionHorizonDays: 7, reviewState: 'unknown', priorSet: prior })

  assert.equal(request?.url, '/api/delivery/v1/projects/project%20one/plans/plan%2F1/versions/3/mechanistic-simulation-runs')
  const body = JSON.parse(request?.init?.body as string)
  assert.equal(body.prior_set.version, 'operator/v1')
  assert.equal(body.stable_seed, 'stable')
  assert.equal(result.planVersion, 3)
  assert.equal(result.calibrationStatus, 'assumption_driven')
})

test('mechanistic simulation restores the latest result for one plan version', async t => {
  const originalFetch = globalThis.fetch
  let url = ''
  globalThis.fetch = async input => {
    url = String(input)
    return new Response(JSON.stringify({
      id: 'latest_run', plan_id: 'plan/1', plan_version: 3, model_version: 'delivery-mechanistic-monte-carlo/v0.2', prior_set_version: 'operator/v1', stable_seed: 'stable', prediction_horizon: 'P7D', sample_count: 5000, status: 'completed', is_simulated: true, calibration_status: 'assumption_driven', metric_windows: [], scenario_probabilities: [], alerts: [], recommendation_drafts: [], assumptions: [], limitations: [], evidence_refs: [],
    }), { headers: { 'Content-Type': 'application/json' } })
  }
  t.after(() => { globalThis.fetch = originalFetch })

  const result = await deliveryMechanisticSimulationApi.getLatest('project one', 'plan/1', 3)

  assert.equal(url, '/api/delivery/v1/projects/project%20one/plans/plan%2F1/versions/3/mechanistic-simulation-run')
  assert.equal(result.id, 'latest_run')
})

function samplePrior(): MechanisticPriorSet {
  const probability = { value: 0.9, source: 'operator://test', unit: 'probability' as const, scope: ['test'], uncertainty: 'test' }
  const range = (unit: 'ratio' | string) => ({ minimum: 0.1, mode: 0.2, maximum: 0.3, source: 'operator://test', unit, scope: ['test'], uncertainty: 'test' })
  return {
    version: 'operator/v1', review_pass_probability: probability, delivery_probability: probability,
    budget_utilization: range('ratio') as MechanisticPriorSet['budget_utilization'], cpm: range('CNY_minor_per_1000_impressions'),
    ctr: range('ratio') as MechanisticPriorSet['ctr'], cvr: range('ratio') as MechanisticPriorSet['cvr'], tracking_observable_rate: probability,
  }
}
