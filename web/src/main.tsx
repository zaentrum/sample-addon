import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { Console } from './Console'

// Standalone dev mount only — the portal never uses this. apiBase '' means
// same-origin, i.e. the addon binary serving this page during development.
createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <Console apiBase="" token={undefined} onUnauthorized={() => alert('401 — re-authenticate')} />
  </StrictMode>,
)
