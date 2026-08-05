# Extract Components

## Purpose

Reads calendar, clock, ISO, and Unix values from one Date timestamp. This pure node is useful when a later node needs a specific date part rather than the complete timestamp.

## Input

Connect **Timestamp (ms)** from `Now`, `Create Date`, `Parse Date`, `Add Duration`, or `Subtract Duration`.

## Outputs

The node exposes year, month, day, hour, minute, second, millisecond, weekday (`0` is Sunday), day of year, ISO week number, ISO 8601 text, Unix seconds, and Unix milliseconds.

## Configuration

Use **Timezone** to choose how the timestamp is presented. The same instant can have different local calendar components in UTC and the computer’s local timezone.

## Failure notes

The input must be a finite Number timestamp in milliseconds. A missing or non-numeric input stops the requesting execution path.

## Example

Connect **Hour** to a Number comparison, then send the Boolean result to `Branch.Condition` to choose between daytime and night-time actions.

