import React from 'react'
import ReactDOM from 'react-dom/client'
import { BrowserRouter, Route, Routes } from 'react-router-dom'
import App from './App'
import { AuthProvider } from './context/AuthContext'
import { ModelConfigProvider } from './context/ModelConfigContext'
import { ProjectProvider } from './context/ProjectContext'
import { AuthBoundary } from './auth/AuthBoundary'
import { LoginPage } from './auth/LoginPage'
import './styles.css'

function Root() {
  return <AuthProvider><BrowserRouter><Routes>
    <Route path="/login" element={<LoginPage />} />
    <Route path="/*" element={<AuthBoundary>
      <ModelConfigProvider><ProjectProvider><App /></ProjectProvider></ModelConfigProvider>
    </AuthBoundary>} />
  </Routes></BrowserRouter></AuthProvider>
}

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <AuthProvider>
      <Root />
    </AuthProvider>
  </React.StrictMode>,
)
