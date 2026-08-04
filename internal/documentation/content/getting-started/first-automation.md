# Your first automation

This example sends a desktop notification from a button.

## Build it

1. Create a pipeline and add **Button Trigger**.
2. Drag from its **Start** exec pin and choose **Desktop Notification**.
3. Set the notification title and message in the inspector.
4. Click **Run** to test the draft. The execution log shows every node input, output, and error.
5. Publish when the graph is valid. The Trigger board then shows your button.

## Add data deliberately

To place a calculated value in the message, connect a pure **Format Text** or **Get Field** output to the Message data pin. The notification is still performed only after its exec input is pulsed.

```text
Button Trigger ──exec──> Desktop Notification
Format Text ──data──> Desktop Notification.Message
```

Read [Desktop Notification](docs:reference/local) for capabilities and failure behavior.
