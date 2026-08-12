import { useState } from 'react'

// 「怎么算的」弹层。每一步都写出用了哪个阈值、哪个口径——这一层是模块可信度的
// 全部来源：人不需要看懂统计，但必须能看到我们没有藏东西。
export function HowItWasComputed({ steps }: { steps: readonly string[] }) {
  const [open, setOpen] = useState(false)
  if (steps.length === 0) return null
  return (
    <span className="how-computed">
      <button type="button" className="how-computed-trigger" onClick={() => setOpen(!open)}>
        怎么算的
      </button>
      {open ? (
        <ol className="how-computed-steps">
          {steps.map((step, index) => <li key={index}>{step}</li>)}
        </ol>
      ) : null}
    </span>
  )
}
