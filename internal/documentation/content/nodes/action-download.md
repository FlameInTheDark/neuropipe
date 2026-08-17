# Download from Web

## Purpose
Download a file from a URL and save it to a local directory. The file name is derived from the URL's last path segment.

## Configuration
- **URL**: full HTTP(S) URL of the file to download. The URL must include a
  path segment that becomes the local file name.
- **Location**: absolute path to the destination directory. The directory is
  created if it does not exist.

## Example
`Button Trigger → Download from Web (URL: https://example.com/report.pdf, Location: C:\\Downloads) → Desktop Notification`.
Connect `Constant` (text) to the **URL** pin to download a URL chosen at
runtime by another node.
