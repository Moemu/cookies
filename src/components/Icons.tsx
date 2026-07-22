import type { SVGProps } from 'react'

export function CookiesMark(props: SVGProps<SVGSVGElement>) {
  return (
    <svg viewBox="0 0 32 32" fill="none" aria-hidden="true" {...props}>
      <path d="M12.2 6.5a7 7 0 1 0 0 14h2.2a7 7 0 0 0 0-14h-2.2Z" stroke="currentColor" strokeWidth="2.4" />
      <path d="M17.6 11.5a7 7 0 1 0 0 14h2.2a7 7 0 0 0 0-14h-2.2Z" stroke="currentColor" strokeWidth="2.4" />
    </svg>
  )
}

export function TrendChart({ points }: { points: number[] }) {
  const coords = points.map((point, index) => `${index * (600 / (points.length - 1))},${180 - point * 1.45}`).join(' ')
  return (
    <svg className="trend-chart" viewBox="0 0 600 190" role="img" aria-label="近十二周项目完成度呈上升趋势">
      {[20, 60, 100, 140, 180].map(y => <line key={y} x1="0" y1={y} x2="600" y2={y} className="chart-grid" />)}
      <polyline points={coords} className="chart-line" />
      {points.map((point, index) => <circle key={index} cx={index * (600 / (points.length - 1))} cy={180 - point * 1.45} r={index === points.length - 1 ? 5 : 2.5} className="chart-point" />)}
    </svg>
  )
}
