# HTTP Request

## Purpose
Calls an HTTP endpoint and makes a text or JSON response available to the graph.

## Configuration

- **URL** and **Method** define the request target and verb.
- **Body** is sent as JSON unless a `Content-Type` header is configured.
- **Request headers** is a key/value editor. Repeated header names are sent as
  separate request headers.
- Turn on **Use custom User-Agent** to reveal the User-Agent field. It replaces
  any `User-Agent` set in the header list.

The node runs only when its Exec input is pulsed. Header data is configuration,
not a data pin, so it cannot cause a request by itself.

## Result

The Result object exposes the HTTP status code, response body, response headers,
and parsed JSON when the response body is valid JSON. Any 4xx or 5xx response
stops the current execution and appears in the run log.

## Example
`Button Trigger → HTTP Request → Create Report`; connect Result to Get Field for a JSON property.
