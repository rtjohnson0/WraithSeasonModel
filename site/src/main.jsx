import React from 'react'
import { createRoot } from 'react-dom/client'
import App from './App.jsx'
import './styles.scss'

// A render error should show a message, not a blank page.
class Boundary extends React.Component {
  state = { failed: false }
  static getDerivedStateFromError() {
    return { failed: true }
  }
  componentDidCatch(err) {
    console.error(err)
  }
  render() {
    if (!this.state.failed) return this.props.children
    return (
      <p className="notice" role="alert" style={{ margin: '2rem 1rem' }}>
        Something on this page failed to load. Refresh in a minute; if it keeps happening, the latest run published data this page could not read.
      </p>
    )
  }
}

createRoot(document.getElementById('root')).render(
  <React.StrictMode>
    <Boundary>
      <App />
    </Boundary>
  </React.StrictMode>,
)

if ('serviceWorker' in navigator && import.meta.env.PROD) {
  window.addEventListener('load', () => {
    navigator.serviceWorker.register(new URL('sw.js', document.baseURI)).catch(() => {})
  })
}
