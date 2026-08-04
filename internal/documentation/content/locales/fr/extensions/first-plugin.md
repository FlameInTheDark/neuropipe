# Créer votre premier plug-in

Ce guide crée un bundle minimal détecté par Neuropipe, avec une page Markdown
dans Documentation. Il démontre la surface v1 actuellement prise en charge :
découverte, diagnostics et documentation. Il ne crée pas encore de nœud de
canevas exécutable.

## Prérequis

- Neuropipe et **Paramètres → Extensions**.
- Go 1.26 ou une chaîne Go compatible.
- Une racine de plug-ins accessible en écriture.

## 1. Créer les dossiers

~~~text
acme-status/
  plugin.json
  sidecar/
    main.go
  docs/
    status-check.md
~~~

Les chemins relatifs sont résolus depuis le dossier de plugin.json.

## 2. Construire un sidecar inoffensif

Créez sidecar/main.go :

~~~go
package main

func main() {
    select {}
}
~~~

Depuis le dossier du bundle :

~~~powershell
go build -o sidecar.exe ./sidecar
~~~

Ce sidecar ne fait volontairement rien. Neuropipe vérifie seulement son
existence et ne le démarre pas. Un vrai sidecar futur utilisera le cycle de vie
gRPC géré du [système de plug-ins](docs:extensions/plugin-system).

## 3. Créer plugin.json

~~~json
{
  "id": "acme-status",
  "name": "Acme Status",
  "version": "0.1.0",
  "description": "Exemple de plug-in de statut.",
  "apiVersion": "v1",
  "executable": "sidecar.exe",
  "nodes": [
    {
      "id": "status-check",
      "kind": "action",
      "label": "Status Check",
      "description": "Déclaration d'exemple, non exécutable actuellement.",
      "icon": "heart-pulse",
      "color": "#60a5fa",
      "capabilities": ["network"],
      "outputs": [{"id": "result", "label": "Result", "kind": "data"}],
      "fields": [{"name": "url", "label": "URL", "kind": "string", "required": true, "secret": false}]
    }
  ],
  "documentation": [
    {
      "id": "status-check",
      "title": "Status Check",
      "categoryPath": ["Extensions", "Acme Status"],
      "path": "docs/status-check.md"
    }
  ]
}
~~~

id, name, apiVersion v1 et executable sont requis. Version et description
améliorent les diagnostics. Fields, outputs et capabilities présentent la forme
future, mais ne créent pas encore de nœud.

## 4. Écrire la page

Créez docs/status-check.md :

~~~markdown
# Status Check

Cet exemple ajoute une page Markdown locale et sûre.

## Exemple

Status Check → Create Report
~~~

Le chemin doit rester dans le bundle et le fichier doit faire au plus 1 MiB.

## 5. Redécouvrir

1. Ouvrez **Paramètres → Extensions**.
2. Vérifiez la racine des plug-ins.
3. Cliquez **Redécouvrir les plug-ins**.
4. Vérifiez Acme Status, version 0.1.0, un nœud déclaré et **Healthy**.
5. Ouvrez **Documentation → Extensions → Acme Status → Status Check**.

Après une modification de manifest ou de documentation, redécouvrez le bundle.

## Dépannage

| Symptôme | Solution |
| --- | --- |
| Aucun élément | Corrigez la racine de plug-ins et redécouvrez. |
| Invalid manifest | Corrigez JSON, id/name ou v1. |
| Sidecar unavailable | Construisez sidecar.exe ou corrigez le chemin. |
| Healthy sans page | Vérifiez le chemin .md, categoryPath, id et la taille. |
| Aucun nœud de Bibliothèque | Comportement attendu : v1 ne les enregistre ni ne les exécute. |

## Exemple à deux nœuds

Pour un bundle à plusieurs nœuds, utilisez une identité stable comme `weather-tools` et déclarez au moins `convert-temperature` et `classify-temperature`. Le premier nœud produit la clé de données `fahrenheit`; le second la lit plus tard et produit `band`.

~~~json
"nodes": [
  { "id": "convert-temperature", "kind": "action", "outputs": [{ "id": "fahrenheit", "label": "Fahrenheit", "kind": "data" }], "fields": [] },
  { "id": "classify-temperature", "kind": "action", "outputs": [{ "id": "band", "label": "Band", "kind": "data" }], "fields": [{ "name": "warmAtOrAbove", "label": "Warm at or above", "kind": "number", "required": false, "secret": false }] }
]
~~~

Dans le futur code Go, `Validate` vérifie les seuils avant l'exécution. `ConvertTemperature.Execute` renvoie `map[string]any{"fahrenheit": ...}` ; `ClassifyTemperature.Execute` lit cette valeur et renvoie `map[string]any{"band": ...}`. Gardez les clés de sortie et les IDs du manifest synchronisés et placez la validation numérique partagée dans une aide. Le runtime v1 actuel ne démarre ni n'appelle encore ces handlers.

Plus de détails : [Système de plug-ins](docs:extensions/plugin-system).
