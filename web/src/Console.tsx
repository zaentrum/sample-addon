import { useState } from 'react'
import './console.css'

/**
 * What the portal hands an embedded console. This is the whole host contract:
 *  - apiBase: the base URL for THIS addon's API, through the portal proxy —
 *    every request via it carries the user's bearer.
 *  - token: the access token, for the rare case you need it directly.
 *  - onUnauthorized: call it on a 401; the shell owns re-authentication.
 */
export interface ConsoleProps {
  apiBase: string
  token: string | undefined
  onUnauthorized?: () => void
}

// The export MUST be named `Console` — the shell looks for exactly that.
export function Console({ apiBase, token, onUnauthorized }: ConsoleProps) {
  // The host passes apiBase with a trailing slash; our paths start with one.
  const api = (path: string) => `${apiBase.replace(/\/+$/, '')}${path}`

  const [hello, setHello] = useState<string>('')
  const [echo, setEcho] = useState<string>('')
  const [text, setText] = useState('hello from the console')

  const query = new URLSearchParams(window.location.search).get('q')

  async function callHello() {
    const r = await fetch(api('/api/hello'))
    setHello(await r.text())
  }

  async function callEcho() {
    const r = await fetch(api('/api/echo'), {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', ...(token ? { Authorization: `Bearer ${token}` } : {}) },
      body: JSON.stringify({ text }),
    })
    if (r.status === 401) {
      onUnauthorized?.()
      setEcho('401 — the shell was asked to re-authenticate')
      return
    }
    setEcho(await r.text())
  }

  return (
    <div className="sa">
      <h1 className="sa__title">sample addon</h1>
      <p className="sa__lede">
        This console is a federated module the portal mounted in-page. It is served by the
        addon's own binary and reached through the portal's proxy — no route, no origin, no
        session of its own.
      </p>
      {query && (
        <p className="sa__hint">
          You arrived from the search slot with the query <code>{query}</code> — that is the
          <code>{'{q}'}</code> placeholder in the button the addon declares.
        </p>
      )}

      <section className="sa__card">
        <h2>Public endpoint</h2>
        <p>
          <code>GET {api('/api/hello')}</code> — no login needed. This is also{' '}
          <code>zae sample hello</code> on the CLI.
        </p>
        <button className="sa__btn" onClick={callHello}>call it</button>
        {hello && <pre className="sa__out">{hello}</pre>}
      </section>

      <section className="sa__card">
        <h2>Authenticated endpoint</h2>
        <p>
          <code>POST {api('/api/echo')}</code> — the addon validates your bearer against the
          instance's issuer itself. On the CLI this is <code>zae sample echo</code> (exit 5
          without a token).
        </p>
        <input className="sa__input" value={text} onChange={(e) => setText(e.target.value)} />
        <button className="sa__btn" onClick={callEcho}>echo it</button>
        {echo && <pre className="sa__out">{echo}</pre>}
      </section>

      <p className="sa__foot">
        Source: <a href="https://github.com/zaentrum/sample-addon">github.com/zaentrum/sample-addon</a> ·
        contracts: the platform docs, <em>Extend it</em>.
      </p>
    </div>
  )
}

export default Console
