import React from 'react'
import ReactDOM from 'react-dom/client'
import App from './App'
import { RenderErrorBoundary } from './components/RenderErrorBoundary'
import { AuthProvider } from './context/AuthContext'
import { ModelConfigProvider } from './context/ModelConfigContext'
import { ProjectProvider } from './context/ProjectContext'
import './styles.css'

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    {/* 最外层兜底：Shell、Provider 或路由本身出错时，至少还剩一句能读的说明，
        而不是一片白。页面级的边界在 App.tsx 里，那一层能保住导航。 */}
    <RenderErrorBoundary contextLabel="工作台">
      <AuthProvider><ModelConfigProvider><ProjectProvider><App /></ProjectProvider></ModelConfigProvider></AuthProvider>
    </RenderErrorBoundary>
  </React.StrictMode>,
)
