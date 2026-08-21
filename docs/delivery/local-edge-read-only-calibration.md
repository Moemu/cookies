# Local Edge session for read-only OceanEngine calibration

## Purpose

This procedure starts one visible Microsoft Edge session on Windows. Playwright can attach to this session by local CDP.

The session is for read-only OceanEngine calibration. It does not authorize an advertising-platform write.

## Storage boundary

The managed Profile mode stores all local browser data below this directory:

```text
%LOCALAPPDATA%\cookies\browser-rpa\
```

The fixed Edge user-data directory is `edge-user-data`. The selected browser Profile is `Default`.

This is the default Profile in the managed user-data directory. The tool does not copy the personal Edge Profile.

Some organizations do not permit a new browser session to sign in. Use current Profile mode in this case.

Current Profile mode uses the existing Edge `Default` Profile at `%LOCALAPPDATA%\Microsoft\Edge\User Data\Default`. It does not copy this Profile. Session metadata, screenshots, and diagnostics still stay below `%LOCALAPPDATA%\cookies\browser-rpa\`.

This is an explicit storage exception for current Profile reuse. A copied Profile would copy authentication data and is prohibited.

The directory also contains these files:

- `session.json`: local CDP endpoint and session state.
- `last-attach-check.json`: safe attachment identifiers and failure reason.
- `diagnostics.jsonl`: command outcomes and reuse failure reasons.
- `screenshots\`: raw screenshots from each attachment.

Do not move these files into the repository, `data\`, Git, or `.env`.

## Safety boundary

The tool starts a visible browser. It binds CDP to `127.0.0.1` only.

The tool can read only safe browser and page metadata. It removes URL query strings and fragments before storage.

The tool does not read or export these values:

- Cookies or tokens.
- LocalStorage or other browser storage.
- Passwords, verification codes, or 2FA values.
- Request or response headers.

The tool does not click, fill, select, upload, download, or submit. It does not call an advertising-platform write interface.

The tool does not implement a remote Worker, PWA channel, cloud CDP, or remote CDP proxy.

## Commands

Run all commands from the repository root.

### Current Profile mode

Use this mode when organization policy rejects sign-in in a new browser session.

In the current Edge, open `edge://inspect/#remote-debugging`. Select **Allow remote debugging for this browser instance**.

Then start the local tool session:

```powershell
npm run browser-rpa:edge -- start-current
```

The tool discovers only a `127.0.0.1` DevTools listener owned by Edge. It reads `DevToolsActivePort` only to attach Playwright.

The tool stores the numeric port only. The temporary WebSocket path stays inside each attachment process. The tool does not print or store this path.

Microsoft documents this current-session mode for signed-in browser state: [Inspect a running Edge instance](https://learn.microsoft.com/en-us/microsoft-edge/web-platform/devtools-mcp-server#auto-connect-to-a-running-edge-instance).

### Managed Profile mode

Start or reuse the visible managed Edge session:

```powershell
npm run browser-rpa:edge -- start
```

If OceanEngine shows a login page, complete login, captcha, and 2FA in Edge. Do not enter these values in the terminal.

Check the managed Edge process and local CDP:

```powershell
npm run browser-rpa:edge -- status
```

Attach two times to the same session:

```powershell
npm run browser-rpa:edge -- check
```

The check passes only when both attachments return the same browser-context ID and page Target ID. The current page must also look like a signed-in `ad.oceanengine.com` page.

The login check uses only the page host and path. It does not inspect credentials or browser storage.

Stop the tool session:

```powershell
npm run browser-rpa:edge -- stop
```

For managed Profile mode, `stop` closes only the matching managed Edge process.

For current Profile mode, `stop` detaches this tool and does not close the user's Edge. To disable CDP, clear **Allow remote debugging for this browser instance** in `edge://inspect/#remote-debugging`.

## Failure recovery

Read the `reason` value in `last-attach-check.json`. The tool also appends the reason to `diagnostics.jsonl`.

Use this table:

| Reason | Action |
| --- | --- |
| `manual_login_required` | Complete login in the visible managed Edge. Run `check` again. |
| `browser_context_changed` | Stop the session. Start it again. Do not delete the Profile. |
| `current_page_changed` | Keep one stable OceanEngine page open. Run `check` again. |
| `attach_failed:*` | Run `status`. If the state is unhealthy, stop and start the session. |
| `current_profile_remote_debugging_requires_user_enablement` | Enable remote debugging in the visible `edge://inspect` page. Run `start-current` again. |

Do not report a simulated browser test as a real session reuse result. A real result requires the visible local Edge and two successful CDP attachments.
