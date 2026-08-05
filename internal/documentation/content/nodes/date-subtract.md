# Subtract Duration

## Purpose

Subtracts calendar and clock amounts from a Date timestamp. It is the inverse of `Add Duration` and is a pure data node.

## Inputs and outputs

The node accepts **Timestamp (ms)** plus optional year-to-millisecond amounts and returns **Timestamp (ms)** and **ISO 8601**. Inspector values provide manual duration amounts when their pins are not connected.

## Configuration

Choose `local` or `utc` with **Timezone** before subtracting calendar units such as months or days.

## Failure notes

The source timestamp must be a finite Number. Use positive duration values; the node itself applies the subtraction.

## Example

Connect a parsed invoice date to **Timestamp (ms)**, set Days to `30`, and compare the result with `Now` to detect whether the payment window has opened.

