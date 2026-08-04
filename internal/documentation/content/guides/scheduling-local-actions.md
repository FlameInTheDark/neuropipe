# Schedule a local action

To run a local task safely each workday:

1. Add **Cron Trigger** and configure a five-field expression such as `0 9 * * 1-5`.
2. Wire it through an approved local action, such as **Run Terminal Command** or **Read File**.
3. Test the draft manually, then publish.
4. Review and grant the revision’s capability request.
5. Enable the schedule in the Schedules view.

Schedules use the selected local/IANA timezone and default to skip while the pipeline is already running. A new publish requires trust again, which prevents unnoticed edits from gaining unattended terminal or file access.
