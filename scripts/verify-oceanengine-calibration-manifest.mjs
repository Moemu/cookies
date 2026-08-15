import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import Ajv2020 from 'ajv/dist/2020.js'

const root = resolve(import.meta.dirname, '..')
const readJSON = relative => JSON.parse(readFileSync(resolve(root, relative), 'utf8'))
const schema = readJSON('docs/delivery/schemas/oceanengine-calibration-manifest-v1.json')
const manifest = readJSON('docs/delivery/fixtures/oceanengine-calibration-manifest-v1.json')
const validate = new Ajv2020({ allErrors: true, strict: false }).compile(schema)

if (!validate(manifest)) throw new Error(`invalid calibration manifest: ${JSON.stringify(validate.errors)}`)
if (manifest.observation_boundary.remote_write_authorized) throw new Error('calibration manifest must never authorize remote writes')
if (!manifest.page_families.some(page => page.page_kind === 'project_create' && page.evidence_state === 'observed')) throw new Error('manifest must contain observed project-create evidence')
if (!manifest.coverage_cases.some(item => item.status === 'blocked_by_event_asset')) throw new Error('manifest must retain the observed event-asset block')
if (manifest.fields.some(field => field.locator.kind === 'css' || /nth-child|\[\d+\]/.test(field.locator.value))) throw new Error('manifest contains an unstable locator')
const serialized = JSON.stringify(manifest)
if (/(?:aadvid=|https?:\/\/|cookie|token|余额|[0-9]{10,})/i.test(serialized)) throw new Error('manifest redaction scan failed')
const destinations = new Set(manifest.consumer_mappings.map(mapping => mapping.destination))
for (const destination of ['DeliveryIntent', 'OceanEngineConfiguration', 'DeliveryDecisionCandidate', 'CompiledDeliveryWorkflow', 'PlatformSkill']) {
  if (!destinations.has(destination)) throw new Error(`manifest has no consumer mapping for ${destination}`)
}
console.log(`validated ${manifest.manifest_id}: ${manifest.fields.length} fields, ${manifest.coverage_cases.length} cases`)
