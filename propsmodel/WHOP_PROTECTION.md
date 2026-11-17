# WraithSznBets protected delivery

**Prepared and locally tested; not deployed or protecting the current live site yet.**

Your storefront exposes a product named **WraithSznBets NBA Props Dashboard**, ID **prod_bgvx5EzER8i1A**. Its Model Dashboard App currently uses Whop's built-in Website Embed app. This package adds a separate app gateway; a website embed alone cannot protect a public GitHub Pages origin.

## What is included

- A same-origin Python gateway serving your existing React dashboard and `/api/picks`.
- Official Whop SDK validation of the iframe user token, including signature, issuer and your app audience; the gateway additionally requires an unexpired `exp` claim.
- A live access check on every data request against `prod_bgvx5EzER8i1A`. Only its customers or account admins receive data. Client-supplied user IDs and product IDs do not control the gate.
- No static pick file, source repository, private volume or environment files are mounted publicly. `/data/picks.json` returns 404.
- Data responses are `private, no-store`. Private builds have no pick JSON or service worker; an older worker is removed before the private app fetches data.
- A separate publishing credential lets your GitHub workflow update a private, atomically replaced file. Older publications cannot overwrite newer ones. Publishing credentials cannot read the member endpoint without Whop identity.
- The client clears its board if a private membership/data recheck fails. Membership checks are live on each fetch, not permanently stored in a browser flag. Revoked access denies later requests; it cannot erase data already delivered to a member.

**The daily workflow defaults to public GitHub Pages delivery and no longer requires a private repository.** This follows the owner's current hosting preference. Public delivery exposes the published picks and committed generated state. Setting `WRAITH_DELIVERY=whop` explicitly enables the separately configured gateway, but does not hide any state already committed to a public repository. The gateway is optional and has not been deployed by this update.

## 1. Create an app you own in Whop

In your Whop dashboard, open Developer / Apps and create your own app. This is separate from installing the built-in Website Embed app. Copy the new **app_... ID**. The gateway deliberately rejects the built-in Website Embed app ID.

Register the secure service origin as the app's base URL once deployed. Configure its customer experience view as `/experiences/[experienceId]` (root `/` is also served). Install that app on WraithSznBets and attach its experience to the dashboard product. The server and frontend must share the same origin so Whop's signed iframe token accompanies `/api/picks`.

The public storefront URL is used for the access-denied link. No payment integration or checkout replacement is needed; Whop already manages the product entitlement.

## 2. Deploy the gateway

Use a Docker-capable web-service host. Render supports this setup; persistent disks require a paid service. No hosting service was created and no charges were incurred here.

- Repository root: this project directory.
- Dockerfile: `Dockerfile.whop`.
- Health check: `/healthz`.
- Persistent writable volume: `/var/lib/wraith`, accessible to UID 10001.
- One service instance / one worker. This file-based deployment is intentionally not a multi-instance datastore.
- Disable automatic redeployment for routine generated-data commits; publish data through the dedicated endpoint. Deploy source changes deliberately.

Configure the host's private runtime environment:

| Variable | Value |
|---|---|
| `WHOP_APP_ID` | Your own new app ID, not Website Embed's ID |
| `WHOP_RESOURCE_ID` | `prod_bgvx5EzER8i1A` |
| `WHOP_API_KEY` | Server-side key authorized to check this product's access; grant only required read permissions |
| `PRIVATE_PUBLISH_TOKEN` | A new random secret, at least 40 characters |
| `PRIVATE_DATA_FILE` | `/var/lib/wraith/picks.json` |
| `PRIVATE_STATIC_DIR` | `site/dist` |

The app ID and product ID are not secrets. API keys and the publication token must go only into host settings or GitHub secrets. Do not paste them into chat, JavaScript, Vite variables, a Whop URL, or a committed `.env`.

Generate the new publication secret locally in PowerShell:

```powershell
python -c "import secrets; print(secrets.token_urlsafe(48))"
```

The API correctly returns 503 until the first private publication arrives. It never falls back to public GitHub data. A missing disk after a restart also fails closed rather than exposing a fallback board.

## 3. Close public bypasses and enable delivery

Before distributing paid access:

1. Make the repository holding generated pick logs/model state **private**. Generated state is committed by the existing sync process; hiding a frontend link does not protect those files.
2. Unpublish the existing GitHub Pages site and remove its old Website Embed experience/link from customer access. Making a repository private does not universally make its Pages deployment private. Previously downloaded copies cannot be recalled.
3. Add GitHub repository variable `WRAITH_DELIVERY=whop`.
4. Add GitHub Actions secrets `PRIVATE_BOARD_URL` (only the new HTTPS origin, no path) and `PRIVATE_PUBLISH_TOKEN` (the same publication secret configured on the host).
5. Run Daily. It generates `private/picks.json` and uploads it over HTTPS. It does not create or deploy a Pages artifact in Whop mode. The model/data workflow remains otherwise intact.
6. Open the custom app as an owner, then test a member and a nonmember before directing customers to it.

An unset delivery variable now defaults to public Pages. The private-repository enforcement step has been removed. If WRAITH_DELIVERY is already set to whop, change it to public or remove the variable to use Pages. Protected delivery still requires completing the migration above; changing a workflow setting does not secure a public origin.

## Verification before launch

- Anonymous/direct `/api/picks`: 401, no pick data.
- Valid Whop account without product access: 403, no pick data.
- Active member inside your custom app: current board loads and graphs/dropdowns work.
- Revoke that membership: the next data request is denied and the app clears its board.
- Expired, wrong-app, bad-signature or forged user token: denied.
- Old `/data/picks.json` URLs and public repository files: inaccessible after cutover.
- Restart the service: its persistent private publication remains available to members.
- Check GitHub: Whop mode has no public Pages deployment or public pick artifact.

Eight automated gateway tests exercise actual SDK signature verification with test keys and actual SDK access-request serialization with a mock Whop API. They cover anonymous/spoofed identities, expiry/issuer/audience/signature, missing expiry, revocation without a positive-access cache, exact product scope, upstream failure, private-file/path bypasses, authenticated uploads, rollback protection, startup rejection of public data and rate limiting. These are implementation tests, not a live Whop integration test or a third-party penetration test. No real member token, private key, API key or app ID was available here.

Customers can copy or screenshot data they legitimately receive. This protects access, not redistribution. Token replay by a still-authorized member, account sharing, and high-volume distributed abuse need additional controls if they become a problem.

## Sources checked November 12, 2026

- https://docs.whop.com/developer/guides/authentication
- https://github.com/whopio/whopsdk-python
- https://docs.github.com/en/pages/getting-started-with-github-pages/configuring-a-publishing-source-for-your-github-pages-site
- https://render.com/docs/docker
- https://render.com/docs/disks
