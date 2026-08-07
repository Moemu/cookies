export type AsyncGateResult<T> =
  | { started: true; value: T }
  | { started: false }

export function createAsyncActionGate() {
  let active = false

  return {
    isActive: () => active,
    async run<T>(operation: () => Promise<T>): Promise<AsyncGateResult<T>> {
      if (active) return { started: false }
      active = true
      try {
        return { started: true, value: await operation() }
      } finally {
        active = false
      }
    },
  }
}
