import { BrowserRouter, Navigate, Route, Routes } from 'react-router-dom'
import { Workspace } from './shell/Workspace'
import { AuthBoundary } from './auth/AuthBoundary'
import { LoginPage } from './auth/LoginPage'

export default function App() {
  return <BrowserRouter><Routes>
    <Route path="/" element={<Navigate replace to="/projects" />} />
    <Route path="/login" element={<LoginPage />} />
    <Route path="/*" element={<AuthBoundary><Workspace /></AuthBoundary>} />
  </Routes></BrowserRouter>
}
