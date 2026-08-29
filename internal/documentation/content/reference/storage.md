# Storage nodes

Storage nodes move files between this machine and registered remote storages — S3-compatible object stores and FTP/FTPS servers — and share them through presigned or public URLs. Connections live in the Storages view; every node picks one from the Storage dropdown, and credentials never leave the local vault. Transfer nodes carry the network capability; the URL nodes are computed locally.

## Upload File

**Purpose:** upload a file into a storage from three sources picked on the **Source** dropdown. **Pins:** Exec input/output, Local path/Data (bytes or base64)/Base64/Remote path/Content type inputs, Result object. **Configure:** storage, source (**From disk** streams a local file; **From node** writes raw bytes like a Draw Image output; **From base64** decodes inspector text), remote path (trailing `/` keeps the file name; include the name for node and base64 sources), optional content type. **Auto** uses whatever is set — local path, then node bytes, then base64 — which is exactly how graphs saved before the dropdown behaved; an explicit source reads only its own input. **Produces:** stored `key`, uploaded `size`, and the `driver` that handled the transfer. **Capability:** network. **Failure:** missing storage, a blank source input for the selected mode, and transfer errors stop the node. **Example:** Draw Image → Upload File (`reports/chart.png`) → Discord Send Message.

## Download File

**Purpose:** stream one remote file to a local path. **Pins:** Exec input/output, Remote path/Local path inputs, Result object. **Configure:** storage, remote path, and local destination (trailing `/` keeps the remote name). **Produces:** local `path`, file `name`, and downloaded `bytes`. **Example:** Cron Trigger → Download File → Read File.

## List Files

**Purpose:** list the direct children of one remote folder. **Pins:** Exec input/output, Path input, Entries list and Count outputs. **Configure:** storage and folder path (empty lists the root). **Produces:** entries with `name`, `path`, `isDir`, `size`, and `modified`. **Example:** List Files → For Each Loop → Download File.

## Delete File

**Purpose:** delete one remote file, or a folder with everything inside it. **Pins:** Exec input/output, Path input, Result object. **Configure:** storage, path, and recursive folder deletion. **Produces:** `deleted` flag and removed entry `count`. **Example:** Branch True → Delete File (`reports/2025/`).

## Create Folder

**Purpose:** create one remote folder (S3 folders are zero-byte markers). **Pins:** Exec input/output, Path input, Result object. **Configure:** storage and folder path. **Produces:** created `path` and `created` flag. **Example:** Schedule Trigger → Create Folder (`backups/{{date}}`).

## Move File

**Purpose:** rename or move one remote file or folder inside a storage. **Pins:** Exec input/output, From/To inputs, Result object. **Configure:** storage and both paths. **Produces:** `from`, `to`, and `moved` flag. **Example:** Download File → Move File (`tmp/` → `final/`).

## Presign URL

**Purpose:** generate a temporary signed URL (S3 only) for one object — GET, PUT, HEAD, or DELETE — that works without credentials until it expires. **Pins:** Exec input/output, Path/Expires/Headers/Params inputs, URL text and Result object outputs. **Configure:** storage, method, path, expiry (seconds, `15m`, `1h30m`, or `7d`; 1 second to 7 days), optional required headers (`Content-Type=image/png` — signed into the link, the consumer must send them exactly) and query parameters (`response-content-disposition=…`). **Produces:** `url`, `method`, `expiresInSeconds`, `expiresAt`, and the canonicalized required `headers`. **Signing is offline;** FTP storages are rejected explicitly. **Example:** Upload File → Presign URL (PUT, 15m) → HTTP Request.

## Public URL

**Purpose:** build the public address of one file or folder without a network round-trip — even before the file exists. **Pins:** Exec input/output, Path input (empty = storage root), URL text and Result object outputs. **Configure:** storage and path. **Produces:** `url` plus the `kind`: `public-base` (the connection's **Public URL base** — a CDN or web mapping, works for S3 and FTP), `s3` (direct object address: virtual-host style on AWS, path style on custom endpoints), or `ftp` (best-effort `ftp(s)://` protocol URL including the base folder, never credentials). **Example:** Upload File → Public URL → Discord Send Message.
