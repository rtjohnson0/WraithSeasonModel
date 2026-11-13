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

async function start() {
 const privateBoard=import.meta.env.VITE_PRIVATE_BOARD==='true'
 if (privateBoard && 'serviceWorker' in navigator) {
  const controlled=Boolean(navigator.serviceWorker.controller)
  try {
   const registrations=await navigator.serviceWorker.getRegistrations()
   await Promise.all(registrations.map(r=>r.unregister()))
   if ('caches' in window) {
    const keys=await caches.keys()
    await Promise.all(keys.filter(k=>k.startsWith('wraith-')).map(k=>caches.delete(k)))
   }
  } catch (error) {
   // Some iframe privacy modes disable worker/storage APIs. With no active
   // controller there is no worker intercepting the private request.
   if (controlled) throw error
  }
  // An old public worker can control this document until navigation. Reload
  // before requesting any private picks, so it cannot cache the API response.
  if (controlled) {window.location.reload();return}
 }
 createRoot(document.getElementById('root')).render(<React.StrictMode><Boundary><App /></Boundary></React.StrictMode>)
 if (!privateBoard && 'serviceWorker' in navigator && import.meta.env.PROD) {
  window.addEventListener('load',()=>navigator.serviceWorker.register(new URL('sw.js',document.baseURI)).catch(()=>{}))
 }
}
start().catch(()=>{document.getElementById('root').textContent='Could not initialize the secure dashboard. Reload to try again.'})
