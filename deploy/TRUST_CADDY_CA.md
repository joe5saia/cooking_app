# Trusting Caddy’s internal CA (LAN HTTPS)

When you use `deploy/Caddyfile.lan`, Caddy serves HTTPS using `tls internal`. Browsers will show warnings until your client device trusts Caddy’s internal root CA.

## Export the CA certificate (from the server)

### Option A: helper script (recommended)

From the repo root:

```bash
./deploy/trust-caddy-ca.sh --host kittenserver --remote-dir ~/apps/cooking_app
```

The script writes the CA certificate to `/tmp/cooking_app-caddy-local-root.crt` (override with `--output`) and prints the SHA-256 fingerprint.

### Option B: manual command (SSH + docker compose)

```bash
ssh kittenserver 'cd ~/apps/cooking_app && docker compose --env-file .env -f deploy/compose.lan.yaml exec -T caddy sh -c "cat /data/caddy/pki/authorities/local/root.crt"' > /tmp/cooking_app-caddy-local-root.crt
```

## Install/trust on clients

### macOS (System Keychain)

```bash
sudo security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain /tmp/cooking_app-caddy-local-root.crt
```

If you use Firefox: it may not trust the macOS system store by default. Enable `security.enterprise_roots.enabled = true` in `about:config`.

### Windows (Trusted Root Certification Authorities)

Run PowerShell as Administrator:

```powershell
certutil -addstore -f Root C:\path\to\cooking_app-caddy-local-root.crt
```

Alternatively: `mmc.exe` → Certificates (Local Computer) → Trusted Root Certification Authorities → Certificates → Import.

### iOS / iPadOS

1. Transfer the `.crt` to the device (AirDrop, Files, email attachment).
2. Tap the certificate and install the profile (Settings will prompt you).
3. Enable full trust: Settings → General → About → Certificate Trust Settings → enable the new CA.

### Android

Steps vary by version/vendor; common path:

1. Copy the `.crt` onto the device.
2. Settings → Security / Privacy → Encryption & credentials → Install a certificate → CA certificate.

Notes:

- Android may display “Network may be monitored” after installing a user CA.
- Some apps don’t trust user-installed CAs (they require system CA); browsers generally do.

## Troubleshooting

- Still getting warnings: confirm you installed the *root* CA (`root.crt`), not a leaf cert.
- You redeployed and wiped volumes: if `caddy_data` was deleted, Caddy will generate a new CA; re-export/reinstall.
- Wrong IP: `deploy/compose.lan.yaml` uses `COOKING_APP_LAN_IP` to decide which IP certificate to issue; it must match the IP you browse to.
- Hostname mismatch: if you browse to `https://cooking-app.lan/`, your Caddy config must issue a certificate for that hostname (see `Caddyfile.lan.hostname`).
- Firefox still warns: enable `security.enterprise_roots.enabled = true` (or import into Firefox’s certificate store).
