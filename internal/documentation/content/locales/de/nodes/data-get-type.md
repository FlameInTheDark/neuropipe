# Typ ermitteln

## Zweck

Meldet den JSON-Typ eines beliebigen Werts: `text`, `number`, `boolean`, `object`, `list` oder `null`. Bei einer Liste liefert der Ausgang **Element-Typ** den gemeinsamen Element-Typ.

## Ausgaben

- **Typ**: Der JSON-Typ des Werts.
- **Element-Typ**: Bei einer Liste der gemeinsame Typ der Elemente, sonst leer.

## Konfiguration und Beispiel

Bei einer Liste ist der **Element-Typ** `any` für eine leere Liste, `mixed` bei gemischten Elementen und sonst einer der JSON-Typen.