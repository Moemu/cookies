import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import App from './App'
import './styles.css'
import './features/creative/creative-workbench.css'
import './shell/shell.css'

const rootElement = document.getElementById('root')

if (rootElement === null) {
  throw new Error('cookies web root is missing')
}

createRoot(rootElement).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
