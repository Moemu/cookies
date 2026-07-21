import { useState } from 'react'
import { Shell } from './components/Shell'
import { DashboardPage, HomePage, ModulePage } from './components/Pages'
import { systems } from './data/navigation'
import type { SystemKey } from './types'

export default function App() {
  const [systemKey, setSystemKey] = useState<SystemKey>('strategy')
  const [activeNav, setActiveNav] = useState('home')
  const [isHome, setIsHome] = useState(true)
  const system = systems.find(item => item.key === systemKey)!
  const navItem = system.nav.find(item => item.id === activeNav) ?? system.nav[0]

  const changeSystem = (next: SystemKey) => {
    setSystemKey(next)
    setActiveNav('home')
    setIsHome(false)
  }

  return <Shell system={system} activeNav={activeNav} isHome={isHome} onHome={() => setIsHome(true)} onSystemChange={changeSystem} onNavChange={setActiveNav}>
    {isHome ? <HomePage onSystemChange={changeSystem}/> : navItem.id === 'home' ? <DashboardPage system={system} onSystemChange={changeSystem}/> : <ModulePage key={`${systemKey}-${navItem.id}`} system={system} item={navItem}/>}
  </Shell>
}
