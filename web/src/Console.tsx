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

/**
 * The portal's own API and this addon's key, read off apiBase: the host
 * passes <portal API>/apps/<key>/, so what comes before /apps/ is the API the
 * portal itself serves, and what follows it is the key the platform installed
 * this addon under. null when the console is mounted without the portal
 * around it (the standalone dev mount).
 */
export function portalOf(apiBase: string): { api: string; key: string } | null {
  const m = /^(.*)\/apps\/([a-z0-9](?:[-a-z0-9]*[a-z0-9])?)\/*$/.exec(apiBase)
  return m ? { api: m[1], key: m[2] } : null
}

// The export MUST be named `Console` — the shell looks for exactly that.
export function Console({ apiBase, token, onUnauthorized }: ConsoleProps) {
  // The host passes apiBase with a trailing slash; our paths start with one.
  const api = (path: string) => `${apiBase.replace(/\/+$/, '')}${path}`
  const portal = portalOf(apiBase)

  const [hello, setHello] = useState<string>('')
  const [echo, setEcho] = useState<string>('')
  const [text, setText] = useState('hello from the console')
  const [notice, setNotice] = useState<string>('')

  const query = new URLSearchParams(window.location.search).get('q')

  // Every request through the portal's proxy needs the user's bearer — the
  // proxy turns away anything else — even where the addon itself asks for none.
  const auth = token ? { Authorization: `Bearer ${token}` } : undefined

  async function callHello() {
    const r = await fetch(api('/api/hello'), { headers: auth })
    if (r.status === 401) {
      onUnauthorized?.()
      setHello('401 — the shell was asked to re-authenticate')
      return
    }
    setHello(await r.text())
  }

  async function callEcho() {
    const r = await fetch(api('/api/echo'), {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', ...auth },
      body: JSON.stringify({ text }),
    })
    if (r.status === 401) {
      onUnauthorized?.()
      setEcho('401 — the shell was asked to re-authenticate')
      return
    }
    setEcho(await r.text())
  }

  // A notice to the signed-in person, from this addon. The addon holds no
  // credential of its own, so its console posts with the bearer the shell
  // handed it — to POST /me/notices, which reaches nobody but that bearer's
  // person. The text is ours: plain text, a title of at most 80 characters
  // and a body of at most 280; the link leads back to this console.
  async function sendNotice() {
    if (!portal) {
      setNotice('only inside the portal: this console was mounted without the portal around it')
      return
    }
    const at = new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
    const r = await fetch(`${portal.api}/me/notices`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', ...auth },
      body: JSON.stringify({
        addon: portal.key,
        title: 'Hello from the sample addon',
        body: `Sent from its console at ${at}. Open it to come back here.`,
        link: `/portal/app/${portal.key}`,
      }),
    })
    if (r.status === 401) {
      onUnauthorized?.()
      setNotice('401 — the shell was asked to re-authenticate')
      return
    }
    setNotice(`${r.status} ${await r.text()}`)
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
        <h2>Open endpoint</h2>
        <p>
          <code>GET {api('/api/hello')}</code> — the addon asks for no role here; the portal's
          proxy still wants you signed in, as for every request through it. This is also{' '}
          <code>zae sample hello</code> on the CLI, after <code>zae login</code>.
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

      <section className="sa__card">
        <h2>Notices</h2>
        <p>
          <code>POST {portal ? `${portal.api}/me/notices` : '/api/portal/me/notices'}</code> — a notice
          to you, from this addon. The addon holds no credential, so the console posts with the
          bearer the shell hands it, and the notice reaches nobody but you. The bell in the
          portal's header shows it, and so do the apps.
        </p>
        <p>
          An addon that tells someone something later — when their title is ready — posts with a
          service account instead: <code>POST /api/portal/notices</code>, naming the person.
        </p>
        <button className="sa__btn" onClick={sendNotice}>Send Me a Notice</button>
        {notice && <pre className="sa__out">{notice}</pre>}
      </section>

      <p className="sa__foot">
        Source: <a href="https://github.com/zaentrum/sample-addon">github.com/zaentrum/sample-addon</a> ·
        contracts: the platform docs, <em>Extend it</em>.
      </p>
    </div>
  )
}

export default Console
