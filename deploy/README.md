# Installing the sample addon on an instance

Two steps, and the second one is a click. The platform docs describe the model
under *Extend it → Installing addons*; this is the addon-side view.

1. **Deploy the workload** next to the platform — plain Kubernetes. The
   operator never prunes what it did not render, so the addon is safe there.

   Edit the one instance value in `sample-addon.yaml` (`OIDC_ISSUER`, used
   only by the authenticated endpoint) and the namespace in
   `kustomization.yaml`. If the platform's own Deployments carry
   `hostAliases` for the public hostname, copy that block onto this one — it
   means the cluster cannot resolve its own public name from inside.

   ```sh
   kubectl apply -k deploy/
   ```

   The addon comes up with no credentials and registers nothing. It serves a
   manifest at `/.well-known/zaentrum-capability.json` that *declares* what it
   contributes: an app with a console, one slot button, two CLI commands.

2. **Install it in the portal:** settings → *addons* → address
   `http://sample-addon` → *install*. The platform reads the manifest and
   creates the app, the tile and the slot row, owned by the addon key. The
   response tells you what it created.

   Scripted, as an admin:

   ```sh
   curl -X POST https://<instance>/api/portal/addons \
     -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
     -d '{"proxyUrl":"http://sample-addon"}'
   ```

Then: the button appears on an empty search in chino, the console is at
`/portal/app/sample`, and `zae discover --url https://<your instance>` lists
`zae sample hello`.

**Upgrade** that changes the manifest: settings → addons → *refresh* (the same
call as install; rows are replaced, not merged).

**Uninstall** is subtraction: settings → addons → *remove* deletes the app,
tile and rows; `kubectl delete -k deploy/` removes the workload. The core
shows no trace.
