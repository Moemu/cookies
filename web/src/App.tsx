import { BrowserRouter, Navigate, Route, Routes } from 'react-router-dom'
import { Workspace } from './shell/Workspace'

export default function App() {
  return <BrowserRouter><Routes><Route path="/" element={<Navigate replace to="/strategy" />} /><Route path="/*" element={<Workspace />} /></Routes></BrowserRouter>
}
