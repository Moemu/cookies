import React from 'react'
import ReactDOM from 'react-dom/client'
import App from './App'
import { AuthProvider } from './context/AuthContext'
import { ModelConfigProvider } from './context/ModelConfigContext'
import { ProjectProvider } from './context/ProjectContext'
import './styles.css'

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <AuthProvider><ModelConfigProvider><ProjectProvider><App /></ProjectProvider></ModelConfigProvider></AuthProvider>
  </React.StrictMode>,
)
