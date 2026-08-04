# Local Webhook

L'événement Local Webhook démarre une pipeline publiée et approuvée lorsque
Neuropipe reçoit une requête HTTP correctement signée par HMAC. Activez d'abord
l'écouteur dans **Paramètres → API et webhooks**.

## Broches

- **Start** est la sortie exec vers le premier nœud d'action ou de flux.
- La sortie de données contient l'objet de requête. Utilisez **Get Field** pour
  lire de façon typée json.event, json.repository ou body.

## Configuration

| Champ | Requis | Description |
| --- | --- | --- |
| **Path** | Oui | Utilisez un chemin distinct comme /build-complete. Les barres obliques externes sont normalisées. |
| **Signing secret** | Oui | Secret sélectionné dans le coffre Neuropipe protégé par Windows. Il n'apparaît jamais sur le canevas ni dans le journal. |

L'envoi utilise POST http://<adresse>:<port>/hooks/<chemin>. Avec le chemin
/build-complete et le port par défaut :
http://127.0.0.1:7878/hooks/build-complete.

## Authentifier la requête

Définissez X-Neuropipe-Signature à
sha256=<HMAC-SHA-256 hexadécimal du corps brut>. Neuropipe compare les octets
bruts avec une comparaison à temps constant. Un JSON reformaté, des fins de
ligne ou un encodage différents échouent. Ce HMAC authentifie les webhooks ;
le jeton bearer reste réservé à /v1.

## Valeurs produites

Pour une requête valide, la sortie contient :

- trigger : webhook ;
- body : le corps brut en texte ;
- json : le corps analysé s'il est du JSON valide.

Pour {"event":"build.completed","build":42}, ajoutez dans **Get Field**
json.event en texte et json.build en nombre.

## Exécution et approbation

Les webhooks sont sans surveillance. Seule une liaison de déclencheur active,
publiée et approuvée pour cette révision exacte est mise en file. Une
modification publiée peut demander une nouvelle approbation. Une requête
acceptée renvoie **202 Accepted** et un enregistrement d'exécution ; les actions
en aval peuvent encore être en cours ou échouer.

## Échecs

API désactivée, chemin inconnu ou désactivé, secret absent, signature invalide
ou révision non approuvée empêchent le démarrage. Un HMAC valide ne masque pas
les erreurs ultérieures telles qu'une capacité refusée ou une erreur de
fournisseur ; elles sont consignées avec masquage.

## Exemple

~~~text
Local Webhook → Get Field (json.repository, json.status)
              → Branch (status == "success") → Create Report
~~~

Pour la configuration complète, les exemples de signature et le dépannage,
consultez [API et webhooks](docs:concepts/api-webhooks).
