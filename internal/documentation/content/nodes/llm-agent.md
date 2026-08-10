# Agent

## Purpose
Completes a bounded task using only explicitly connected published LLM tool functions.

## Tools pin

The **Tools** input is a declarative, unlimited tool pin, not an Exec pin. Connect the single **Tool** output of each published LLM tool function to it. The Agent exposes only those functions to the provider; an unconnected function cannot be called.

Each call is checked against the function's published name, concrete input types, required inputs, and model guidance. The function result is returned as JSON for the next model turn. A malformed argument receives safe contract feedback so the model can retry within its configured turn limit.

## Example
`Get city forecast.Tool → Agent.Tools`, then `Button Trigger → Agent → Create Report`.
