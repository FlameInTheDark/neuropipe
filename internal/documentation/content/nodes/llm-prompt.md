# LLM Prompt

## Purpose
Generates a model response with the selected provider and optional model override.

## Provider and model

Every AI node picks its **Provider** and **Model** in the inspector. Both default to the provider configured as default in Settings → Providers, so changing the default re-routes every node that has not made an explicit choice. The Model list comes from the selected provider's configured models (Settings → Providers → Models); an empty selection uses the provider's default model, and a wired **Model** input pin still overrides the selection at runtime.

## Example
`Get Field → LLM Prompt → Create Report`.
