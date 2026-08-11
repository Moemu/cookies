import assert from 'node:assert/strict'
import test from 'node:test'
import { parseSSEBlock, readSSE } from '../src/features/strategy/readSSE.js'
import {
  conversationEventCursorKey,
  shouldAdvanceConversationEventCursor,
} from '../src/features/strategy/useConversationStream.js'

test('SSE parser preserves multiline data and rejects cursor injection', () => {
  assert.deepEqual(parseSSEBlock('id: 41\nevent: strategy.updated\ndata: first\ndata: second'), {
    id: '41', event: 'strategy.updated', data: 'first\nsecond',
  })
  assert.deepEqual(parseSSEBlock('id: unsafe\0cursor\ndata: refresh'), {
    id: '', event: 'message', data: 'refresh',
  })
  assert.equal(parseSSEBlock(': heartbeat\nevent: ping'), null)
})

test('conversation cursor accepts only strictly increasing numeric event IDs', () => {
  assert.equal(shouldAdvanceConversationEventCursor('', '1'), true)
  assert.equal(shouldAdvanceConversationEventCursor('9', '10'), true)
  assert.equal(shouldAdvanceConversationEventCursor('10', '10'), false, 'duplicate event advanced the cursor')
  assert.equal(shouldAdvanceConversationEventCursor('10', '9'), false, 'out-of-order event regressed the cursor')
  assert.equal(shouldAdvanceConversationEventCursor('10', 'opaque'), false, 'invalid cursor was accepted')
  assert.equal(shouldAdvanceConversationEventCursor('9007199254740993', '9007199254740994'), true)
})

test('conversation cursor storage is isolated by authenticated owner', () => {
  const first = conversationEventCursorKey('org_1:user_1:project_1', 'conversation_1')
  const otherUser = conversationEventCursorKey('org_1:user_2:project_1', 'conversation_1')
  assert.notEqual(first, otherUser)
  assert.match(first, /^strategy:last-event:/)
})

test('SSE reader emits complete events exactly as framed across one response', async () => {
  const response = new Response([
    'id: 1\nevent: strategy.updated\ndata: one\n\n',
    'id: 1\nevent: strategy.updated\ndata: duplicate\n\n',
    'id: 0\nevent: strategy.updated\ndata: out-of-order\n\n',
    'id: 2\nevent: strategy.updated\ndata: two\n\n',
  ].join(''), { status: 200, headers: { 'Content-Type': 'text/event-stream' } })
  const accepted: string[] = []
  let cursor = ''
  await readSSE(response, message => {
    if (!shouldAdvanceConversationEventCursor(cursor, message.id)) return
    cursor = message.id
    accepted.push(message.data)
  })
  assert.deepEqual(accepted, ['one', 'two'])
  assert.equal(cursor, '2')
})
