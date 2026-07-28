export type IconName = 'home' | 'target' | 'pen' | 'chart' | 'send' | 'settings' | 'list' | 'image' | 'database' | 'check' | 'package' | 'user'

type IconProps = {
  name: IconName
}

export function Icon({ name }: IconProps) {
  const common = {
    width: 20,
    height: 20,
    viewBox: '0 0 24 24',
    fill: 'none',
    stroke: 'currentColor',
    strokeWidth: 1.8,
    strokeLinecap: 'round' as const,
    strokeLinejoin: 'round' as const,
    'aria-hidden': true,
  }

  switch (name) {
    case 'home':
      return <svg {...common}><path d="m3 11 9-8 9 8" /><path d="M5 10v10h14V10M9 20v-6h6v6" /></svg>
    case 'target':
      return <svg {...common}><circle cx="12" cy="12" r="8" /><circle cx="12" cy="12" r="3" /><path d="M12 2v2M12 20v2M2 12h2M20 12h2" /></svg>
    case 'pen':
      return <svg {...common}><path d="m4 20 4.3-1 10.5-10.5a2.1 2.1 0 0 0-3-3L5.3 16 4 20Z" /><path d="m13.8 7.5 3 3" /></svg>
    case 'chart':
      return <svg {...common}><path d="M4 20V4M4 20h16" /><path d="m8 16 3-4 3 2 4-6" /></svg>
    case 'send':
      return <svg {...common}><path d="m21 3-7.6 18-3.8-7.6L2 9.6 21 3Z" /><path d="m9.6 13.4 4.2-4.2" /></svg>
    case 'settings':
      return <svg {...common}><path d="M4 7h9M17 7h3M4 17h3M11 17h9" /><circle cx="15" cy="7" r="2" /><circle cx="9" cy="17" r="2" /></svg>
    case 'list':
      return <svg {...common}><path d="M9 6h11M9 12h11M9 18h11" /><path d="m4 6 .8.8L6.5 5M4 12l.8.8 1.7-1.8M4 18l.8.8 1.7-1.8" /></svg>
    case 'image':
      return <svg {...common}><rect x="3" y="4" width="18" height="16" rx="2" /><circle cx="8.5" cy="9" r="1.5" /><path d="m4 17 4.5-4.5 3 3 2-2L20 19" /></svg>
    case 'database':
      return <svg {...common}><ellipse cx="12" cy="5" rx="8" ry="3" /><path d="M4 5v6c0 1.7 3.6 3 8 3s8-1.3 8-3V5" /><path d="M4 11v6c0 1.7 3.6 3 8 3s8-1.3 8-3v-6" /></svg>
    case 'check':
      return <svg {...common}><circle cx="12" cy="12" r="9" /><path d="m8 12 2.7 2.7L16.5 9" /></svg>
    case 'package':
      return <svg {...common}><path d="m12 3 8 4.5v9L12 21l-8-4.5v-9L12 3Z" /><path d="m4 7.5 8 4.5 8-4.5M12 12v9" /></svg>
    case 'user':
      return <svg {...common}><circle cx="12" cy="8" r="4" /><path d="M4.5 21a7.5 7.5 0 0 1 15 0" /></svg>
  }
}
