import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import test from 'node:test'
import { agentFailureMessage } from '../src/features/strategy/useStrategyWorkspace.ts'

test('Seed 2 Pro route configuration omits unsupported thinking auto mode', async () => {
  const source = await readFile(
    new URL('../scripts/configure-seed2-pro-text.ps1', import.meta.url),
    'utf8',
  )

  assert.doesNotMatch(source, /'thinking_mode'\s*,\s*'auto'/)
  assert.match(
    source,
    /Set-DotEnvValue "COOKIES_STRATEGY_DEEP_REVIEW_MODEL_ALIAS" "cookies\.text\.standard"/,
  )
})

test('Strategy agent failures give stage-specific recovery guidance', () => {
  assert.equal(
    agentFailureMessage('MODEL_RATE_LIMITED', undefined, 'strategy.brief.extract'),
    '文本模型请求暂时受限，请稍后重新发送需求消息。',
  )
  assert.equal(
    agentFailureMessage('MODEL_RATE_LIMITED', undefined, 'strategy.generate'),
    '文本模型请求频率受限，请稍后再点击“重新生成策略”。',
  )
  assert.equal(
    agentFailureMessage('MODEL_REQUEST_REJECTED', undefined, 'strategy.brief.extract'),
    '文本模型不支持当前路由参数，请联系管理员检查模型配置后重试。',
  )
})
