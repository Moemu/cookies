import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import test from 'node:test'
import { agentFailureMessage } from '../src/features/strategy/useStrategyWorkspace.ts'

test('Seed 2 Pro keeps standard and deep-thinking routes explicit', async () => {
  const source = await readFile(
    new URL('../scripts/configure-seed2-pro-text.ps1', import.meta.url),
    'utf8',
  )

  assert.doesNotMatch(source, /'thinking_mode'\s*,\s*'auto'/)
  assert.match(
    source,
    /Set-DotEnvValue "COOKIES_STRATEGY_DEEP_REVIEW_MODEL_ALIAS" "cookies\.text\.deep_review"/,
  )
  assert.match(source, /'thinking_mode', 'disabled'/)
  assert.match(source, /'thinking_mode', 'enabled'/)
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
  assert.match(agentFailureMessage('CONVERSATION_WEB_SEARCH_FAILED'), /没有生成无来源回答/)
  assert.equal(
    agentFailureMessage('AGENT_EXECUTION_FAILED', 'Agent execution failed'),
    'AI 助手本轮执行未完成，请重新发送。已填写的内容和历史对话不会丢失。',
  )
  assert.equal(
    agentFailureMessage('JOB_EXECUTION_FAILED', 'Job execution failed'),
    'AI 助手本轮执行未完成，请重新发送。已填写的内容和历史对话不会丢失。',
  )
})

test('conversation web search answers only after grounded evidence returns', async () => {
  const workspaceSource = await readFile(
    new URL('../src/features/strategy/useStrategyWorkspace.ts', import.meta.url),
    'utf8',
  )
  const paneSource = await readFile(
    new URL('../src/features/strategy/StrategyConversationPane.tsx', import.meta.url),
    'utf8',
  )

  assert.doesNotMatch(workspaceSource, /purpose:\s*'conversation_web_search'/)
  assert.match(workspaceSource, /搜索完成后会基于返回证据生成本轮回答/)
  assert.match(paneSource, /搜索完成后再生成本轮回答/)
  assert.doesNotMatch(paneSource, /后台独立补充|后台补充/)
})
