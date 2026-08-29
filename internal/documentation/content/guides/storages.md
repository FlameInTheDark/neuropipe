# Storages (S3 and FTP)

Neuropipe speaks two remote-storage protocols natively: any S3-compatible
endpoint (AWS S3, MinIO, Cloudflare R2, Wasabi, Backblaze B2) and classic
FTP/FTPS servers. Register a connection in **Storages** and it becomes a
first-class citizen — browsable in the dedicated Files tab, and usable from
the Storage node family in any pipeline. Credentials live in the local vault
and never leave the machine.

## Register a connection

Open **Storages**, click **New connection**, and pick an engine. For S3,
enter the bucket, the region, and the access key pair; leave the endpoint
blank for AWS itself or point it at a compatible server (R2 accounts paste
their endpoint here, MinIO its `host:port`). Custom endpoints can turn TLS
off for local servers served over plain HTTP. For FTP, enter host and port
(21 by default), the username and password, a TLS mode (none, explicit
STARTTLS, or implicit), and an optional base directory every path is
resolved against. Both engines accept an optional **Public URL base** — an
`https://` address that serves the storage root, such as a CDN in front of a
bucket or a web server mapping the FTP tree; the **Public URL** node uses it
to build shareable links. Use **Test connection** before saving.

## Browse and operate

The **Files** tab is a small file manager: breadcrumb navigation, folders
first, sizes and modification times per row, and a context menu with Open,
Download, Move, and Delete. Uploads pick a local file through a native
dialog and stream it to the current folder; downloads pick a destination the
same way. New Folder creates remote directories (on S3, zero-byte folder
markers — the same convention the consoles use). Deletion always confirms
first and needs the recursive choice for folders, mirroring the node
behaviour. The **Info** tab summarises the connection with masked secrets.

## Build with storage nodes

The **Storage** category covers the round trip: **Upload File** uploads from
three sources picked on its **Source** dropdown — a local file (streamed
from disk), raw bytes from another node (Draw Image output wires straight
in), or base64 text — **Download File** fetches to a local path, **List
Files** enumerates one folder, and **Delete File**, **Create Folder**, and
**Move File** manage the remote tree. Every node picks the connection from
its **Storage** field, and paths use forward slashes with no leading slash —
`reports/2026/chart.png`.

## Share files with URLs

**Presign URL** (S3 only) mints a temporary signed link for one object —
choose GET, PUT, HEAD, or DELETE, an expiry from 1 second to 7 days, optional
required headers that are signed into the link (`Content-Type=image/png`),
and optional query parameters like `response-content-disposition`. The
signature is computed locally; nothing leaves the machine. **Public URL**
builds the permanent address of a file or folder for either engine: the
connection's public URL base when configured, the direct S3 object address
otherwise, and a best-effort `ftp(s)://` reference for FTP storages without a
base. A common pattern chains them after an upload: **Upload File → Public
URL → Discord Send Message**, or hands a short-lived **Presign URL** to an
external uploader through the HTTP node.

Storage operations run on the machine that executes the pipeline. Remote
executors do not proxy vault credentials, so storage nodes belong on the
local runner or on executors with direct access to the same servers.
