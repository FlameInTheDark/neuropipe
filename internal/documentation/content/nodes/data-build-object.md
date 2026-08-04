# Build Object

## Purpose

Creates a typed object from any number of configured data input pins. Each row
in **Fields** keeps a stable pin ID, so renaming a displayed pin name or its
object key does not detach existing wires.

## Configuration

Add a field for every object value. Set its **Pin name** for the graph, its
expected **Data type**, and the destination **Object key**. Object keys support
dotted paths: `customer.name` creates `{ "customer": { "name": ... } }`.

Keys must be non-empty and unique. A key cannot overlap another key:
configuring both `customer` and `customer.name` is invalid because one value
would replace the other object.

## Example

Configure inputs named **Name** → `customer.name` and **Email** →
`customer.email`. Wire two text values to those inputs. The **Object** output
is then:

~~~json
{
  "customer": {
    "name": "Ada Lovelace",
    "email": "ada@example.com"
  }
}
~~~

Use the Object output as the source for **Break Object**, an HTTP JSON body, or
another typed data node.
