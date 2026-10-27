import React from 'react'
import ReactDOM from 'react-dom/client'
import App from './App.jsx'
import './styles/base.css'

// Entry point. Data comes from src/mock.js until the operator exposes the
// Finding CRDs and status API (React Query polling + WebSocket timeline are
// wired at that point — see ui/README.md). Deliberately no extra deps before.
ReactDOM.createRoot(document.getElementById('root')).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>,
)
