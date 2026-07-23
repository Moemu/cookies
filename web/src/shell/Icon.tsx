import type { ShellModule } from './modules'

type IconProps = {
  name: ShellModule['icon']
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
  }
}
