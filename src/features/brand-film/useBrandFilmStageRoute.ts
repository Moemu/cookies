import { useCallback, useEffect, useState } from 'react'
import type { BrandFilmStageId } from './stage'

function stageFromLocation() {
  return new URL(window.location.href).searchParams.get('stage')
}

export function useBrandFilmStageRoute() {
  const [requestedStage, setRequestedStage] = useState(stageFromLocation)

  useEffect(() => {
    const sync = () => setRequestedStage(stageFromLocation())
    window.addEventListener('popstate', sync)
    return () => window.removeEventListener('popstate', sync)
  }, [])

  const navigateToStage = useCallback((stage: BrandFilmStageId, replace = false) => {
    const url = new URL(window.location.href)
    url.searchParams.set('stage', stage)
    window.history[replace ? 'replaceState' : 'pushState']({}, '', `${url.pathname}${url.search}${url.hash}`)
    setRequestedStage(stage)
  }, [])

  return { requestedStage, navigateToStage }
}
