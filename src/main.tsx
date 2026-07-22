import React from 'react'
import ReactDOM from 'react-dom/client'
import App from './App'
import { ModelConfigProvider } from './context/ModelConfigContext'
import { ProjectProvider } from './context/ProjectContext'
import './styles.css'

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <ModelConfigProvider><ProjectProvider><App /></ProjectProvider></ModelConfigProvider>
  </React.StrictMode>,
)
