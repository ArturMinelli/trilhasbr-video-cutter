import React from 'react'
import { createRoot } from 'react-dom/client'

import { App } from './app/App'
import { AppProviders } from './app/providers/AppProviders'
import './style.css'

const container = document.getElementById('root')

if (!container) {
  throw new Error('Root element not found')
}

createRoot(container).render(
  <React.StrictMode>
    <AppProviders>
      <App />
    </AppProviders>
  </React.StrictMode>,
)
