export type AutosaveStatus = 'idle' | 'waiting' | 'saving' | 'saved' | 'failed'

export type SerialAutosaveOptions<T, R = void> = {
  delayMs: number
  fingerprint: (value: T) => string
  save: (value: T) => Promise<R>
  onSaved?: (value: T, result: R) => void
  onError?: (cause: unknown) => void
  onStatus?: (status: AutosaveStatus) => void
}

export type SerialAutosave<T> = {
  schedule: (value: T) => void
  flush: () => Promise<boolean>
  markSaved: (value: T) => void
  dispose: () => void
}

export function createSerialAutosave<T, R = void>(options: SerialAutosaveOptions<T, R>): SerialAutosave<T> {
  let timer: ReturnType<typeof setTimeout> | undefined
  let pending: { value: T; fingerprint: string } | undefined
  let running: Promise<boolean> | undefined
  let savedFingerprint = ''
  let disposed = false

  const setStatus = (status: AutosaveStatus) => {
    if (!disposed) options.onStatus?.(status)
  }

  const drain = (): Promise<boolean> => {
    if (running) return running
    running = (async () => {
      let succeeded = true
      while (!disposed && pending) {
        const job = pending
        pending = undefined
        if (job.fingerprint === savedFingerprint) continue
        setStatus('saving')
        try {
          const result = await options.save(job.value)
          savedFingerprint = job.fingerprint
          options.onSaved?.(job.value, result)
          setStatus('saved')
        } catch (cause) {
          succeeded = false
          pending = undefined
          options.onError?.(cause)
          setStatus('failed')
        }
      }
      return succeeded
    })().finally(() => { running = undefined })
    return running
  }

  return {
    schedule(value) {
      if (disposed) return
      const fingerprint = options.fingerprint(value)
      if (fingerprint === savedFingerprint || fingerprint === pending?.fingerprint) return
      pending = { value, fingerprint }
      if (timer) clearTimeout(timer)
      setStatus('waiting')
      if (!running) timer = setTimeout(() => { timer = undefined; void drain() }, options.delayMs)
    },
    async flush() {
      if (disposed) return false
      if (timer) {
        clearTimeout(timer)
        timer = undefined
      }
      const succeeded = await drain()
      if (running) return (await running) && succeeded
      return succeeded
    },
    markSaved(value) {
      savedFingerprint = options.fingerprint(value)
    },
    dispose() {
      disposed = true
      pending = undefined
      if (timer) clearTimeout(timer)
      timer = undefined
    },
  }
}
