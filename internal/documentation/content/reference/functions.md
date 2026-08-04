# Function nodes

Function boundary nodes are maintained by the function editor. Add public input/output pins in the function metadata, then reconnect callers if the signature changes.

## Function Entry

**Purpose:** start an impure custom function. **Pins:** exec output plus configured data outputs. **Configure:** function public inputs. **Produces:** caller inputs. **Capabilities:** those used by nodes inside the function. **Failure:** impure functions need one valid entry. **Example:** Function Entry → Set Variable → Function Return.

## Function Return

**Purpose:** finish an impure custom function. **Pins:** exec input plus configured data inputs. **Configure:** function public outputs. **Produces:** values for the caller. **Capabilities:** none by itself. **Failure:** unreachable returns fail function validation. **Example:** Branch True → Function Return.

## Function Inputs

**Purpose:** expose pure function inputs. **Pins:** configured data outputs. **Configure:** function public inputs. **Produces:** caller values lazily. **Capabilities:** none. **Failure:** a missing required caller input is validation error. **Example:** Function Inputs number → Greater Than.

## Function Outputs

**Purpose:** return pure function results. **Pins:** configured data inputs. **Configure:** function public outputs. **Produces:** caller outputs. **Capabilities:** none. **Failure:** missing connected output values fail when requested. **Example:** Format Text → Function Outputs text.
