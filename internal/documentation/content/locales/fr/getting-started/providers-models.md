# Fournisseurs et modèles

Neuropipe utilise un seul fournisseur actif à la fois. Choisissez-le dans **Paramètres → Fournisseur**.

## Modes pris en charge

- **Ollama** se connecte à un point de terminaison Ollama local déjà actif.
- **llama.cpp géré** télécharge un runtime détenu par Neuropipe et un modèle GGUF, puis les lie uniquement à loopback.
- **Compatible OpenAI** se connecte à un point de terminaison compatible configuré par l’utilisateur.

## Configuration d’un modèle local

Dans **Paramètres → Modèles**, recherchez des dépôts GGUF publics, choisissez une quantification et installez-la dans le dossier de contenu configuré. L’installation vérifie LFS SHA-256 lorsque le Hub le fournit et stocke les métadonnées locales à côté du modèle. Choisissez ensuite le modèle installé dans **Paramètres → Runtime** et démarrez le runtime géré.

La file LLM limite le travail de modèle simultané. Réglez-la sur un si le runtime local ne peut pas servir des requêtes parallèles.

Les identifiants de fournisseur restent dans le coffre protégé par Windows. Les aperçus de nœuds, journaux, exports et diagnostics de plug-ins masquent les secrets résolus.
