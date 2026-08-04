# Action nodes

Action nodes execute only through an exec wire. They may require a reviewed capability grant when the revision runs unattended.

## HTTP Request

**Purpose:** call an endpoint. **Pins:** Exec input/output; URL, Method, Body inputs; Result object. **Configure:** URL, method, optional body, a repeatable request-header list, and an optional custom User-Agent. The custom User-Agent replaces a User-Agent from the header list. **Produces:** status, body, headers, and parsed JSON when applicable. **Capability:** network host access. **Failure:** network, timeout, invalid request-header, and non-success failures appear in the run log. **Example:** Button Trigger → HTTP Request → Create Report.

## Create Report

**Purpose:** save Markdown in Reports. **Pins:** Exec input/output; Title, Tags, Markdown inputs; Result object. **Configure:** title and tags. **Produces:** report ID, title, and creation time. **Capability:** local report storage only. **Failure:** invalid report input stops the node. **Example:** LLM Prompt → Format Text → Create Report.

## Run Pipeline

**Purpose:** run a published pipeline as an action. **Pins:** Exec input/output and Result. **Configure:** target pipeline ID. **Produces:** target execution result. **Capability:** inherited target revision requirements. **Failure:** missing/unpublished target or trust requirement is reported. **Example:** Webhook → Run Pipeline → Create Report.
