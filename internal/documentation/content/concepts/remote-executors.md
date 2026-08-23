# Remote executors

A remote executor is the standalone Neuropipe pipeline runner installed on another machine. It contains the complete Blueprint engine, so every pipeline that runs locally also runs there: HTTP, terminal, Git, files, JavaScript, AI nodes, reports, chat, Twitch, and more. Neuropipe talks to it over an authenticated gRPC connection.

## What the executor hosts

The executor is autonomous where autonomy makes sense:

- **Cron schedules** fire on the executor machine even while Neuropipe is closed. A schedule only fires autonomously when its published revision was trusted and enabled at deploy time; trust travels with the deployment and is re-deployed when you change it.
- **Button presses**, hotkeys, webhooks, chat triggers, and Twitch events still originate in Neuropipe. Each run of a remote-targeted pipeline is dispatched to the executor over gRPC and appears in your local Runs history.

File-watch triggers are deployed as metadata but currently stay desktop-hosted, like the other dispatch triggers.

## Where data lives

Your machine keeps everything that matters:

- Pipeline definitions, revisions, trust decisions, execution history, reports, and conversations stay in the local workspace.
- In the default **Through Neuropipe** AI mode, model calls from executor runs are forwarded over the encrypted session to your configured providers. API keys never leave this machine.
- Database credentials and Twitch OAuth stay local; SQL and Twitch nodes on the executor call back through the session.

Switching an executor to **On executor** AI mode configures providers directly on that machine instead. Keys entered in the Configure dialog are stored once in the executor's own vault and cannot be read back.

Executor-side global variables are isolated per executor: they are created implicitly by pipelines, persist on the executor, and never sync with your workspace. Interactive dialog nodes fail explicitly on an executor because no one is there to answer them.

## Installing an executor

1. Download the archive for the target platform from the release page (`neuropipe-executor-*` for Windows, Linux, and macOS).
2. Start the daemon once:

   ```bash
   neuropipe-executor serve
   ```

   With no configuration, it creates a `data` directory, generates a strong shared token, prints it **exactly once**, saves it to `data/token.txt`, and starts listening on `:47777`. Copy the printed token into Neuropipe when you register the executor.
3. Optionally add an `executor.json` next to the binary for static settings:

   ```json
   {
     "listen": ":47777",
     "dataDir": "data",
     "tokenFile": "token.txt"
   }
   ```

   Command-line flags override the file per start: `neuropipe-executor serve --listen :5000 --token <value> --data-dir D:\executor`. The `NEUROPIPE_EXECUTOR_TOKEN` environment variable also works for service definitions.
4. In Neuropipe, add the executor with its address (for example `192.168.1.50:47777`) and test the connection.
5. On Linux, register it as a systemd service so schedules survive reboots.

Useful commands:

- `neuropipe-executor status` — shows the effective configuration, where the token would come from (never its value), and how many pipelines and runs are stored locally.
- `neuropipe-executor token generate` — rotates the shared token, saves it, and prints the new value once; update Neuropipe afterwards.
- `neuropipe-executor --version`.

Transport security: token authentication is always required. For traffic across untrusted networks, terminate TLS in front of the executor (`tlsCert`/`tlsKey` in the boot file) or reach it over a VPN, then enable the Use TLS option for the registration.

## Creating pipelines for an executor

Use **New pipeline** in the Pipelines view and pick the executor under *Runs on*. Remote-targeted pipelines appear under the Remote category and carry their executor's badge in the editor. Publishing deploys the published revision — together with any custom functions used by the graph — to the executor automatically; if the executor is unreachable, publish still succeeds locally and you can retry with *Sync to executor* or rely on automatic reconciliation when the connection returns.
