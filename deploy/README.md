# Installing the sample addon on an instance

This is the honest current path — plain Kubernetes, applied next to the
platform. The operator never prunes what it did not render, so the addon is
safe there. The platform docs describe every step in depth under *Extend it →
Installing addons*; this is the addon-side view.

1. **Identity.** On your instance's realm, create a confidential client for the
   addon (service accounts enabled) and give its service account the realm
   role `zaentrum-addon` (create the role if your realm predates it). Put the
   client id and secret in a Secret:

   ```sh
   kubectl -n <platform-namespace> create secret generic sample-addon-oidc \
     --from-literal=client-id=sample-addon-svc --from-literal=client-secret='…'
   ```

2. **Edit the two instance values** in `sample-addon.yaml` (`PUBLIC_BASE`,
   `OIDC_ISSUER`) and the namespace in `kustomization.yaml`. If the platform's
   own Deployments carry `hostAliases` for the public hostname, copy that
   block onto this one — it means the cluster cannot resolve its own public
   name from inside.

3. **Apply.**

   ```sh
   kubectl apply -k deploy/
   ```

   On start the addon mints a service token and registers its slot button. If
   identity is not right yet, it still comes up and says exactly what is
   missing in `GET /api/health/system` — and keeps retrying.

4. **Register the app** so the portal hosts the console and proxies the API:
   portal → settings → apps → *new app*, key `sample`, proxy url
   `http://sample-addon`, base url `/portal/app/sample`; then a tile in a
   space. (Addon self-registration of the app itself is platform roadmap;
   today an admin does this once.)

Then: the button appears on an empty search in chino, the console is at
`/portal/app/sample`, and `zae discover --url https://<your instance>` lists
`zae sample hello`.

**Uninstall** is subtraction: `kubectl delete -k deploy/`, delete the
extension row (`DELETE /api/portal/extensions/sample.search-hint`, or the
admin removes the app in settings), done.
