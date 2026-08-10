# Nœuds locaux

Les nœuds d’action locaux utilisent une approbation d’autorisation limitée. Configurez uniquement les dossiers et dépôts auxquels la révision publiée doit accéder.

## Lister le dossier

**Objectif :** liste les fichiers, dossiers et liens symboliques directs d’un dossier approuvé. **Pins :** entrée/sortie d’exécution, entrée Chemin, sortie Liste de fichiers. **Produit :** `name`, `path`, la taille en octets, `type` (`file`, `directory` ou `symlink`), `updatedAt` et `createdAt` lorsque la plateforme le fournit. **Autorisation :** lecture de fichier. **Échec :** les dossiers inexistants, illisibles ou non approuvés échouent. **Exemple :** Déclencheur de bouton → Lister le dossier → Boucle pour chaque élément.

## Lire un fichier

**Objectif :** lit un fichier local sans modifier ses octets. **Pins :** entrée/sortie d’exécution, entrée Chemin, une sortie Résultat. **Configuration :** sélectionnez Octets ou Texte pour le résultat. **Produit :** la représentation choisie ; Texte pour un contenu non UTF-8 échoue sûrement. **Autorisation :** lecture de fichier. **Échec :** les chemins inexistants ou non approuvés échouent. **Exemple :** Surveillance de fichier → Lire un fichier → Encoder en Base64.

Avec **Encoder en Base64** et **Décoder le Base64**, sélectionnez explicitement les représentations d’entrée et de sortie Texte ou Octets. Aucun nœud local n’effectue cette conversion implicitement.

## Écrire un fichier

**Objectif :** écrit du texte ou des octets bruts. **Pins :** entrée/sortie d’exécution, entrées Chemin/Contenu, objet résultat. **Configuration :** chemin, type de contenu et texte lorsque Texte est sélectionné. **Produit :** chemin écrit et booléen. Les Octets doivent arriver par une broche Octets connectée et ne sont jamais analysés depuis le texte. **Autorisation :** écriture de fichier. **Échec :** les erreurs d’accès parent et d’écriture sont enregistrées. **Exemple :** Lire un fichier (Octets) → Écrire un fichier (Octets).

## Exécuter une commande terminal

**Objectif :** exécute PowerShell, Windows PowerShell ou cmd. **Pins :** entrée/sortie d’exécution, entrées Shell/Commande/Dossier de travail, objet résultat. **Configuration :** shell et commande. **Produit :** commande et sortie combinée. **Autorisation :** terminal. **Échec :** l’annulation, les erreurs de commande et les espaces de travail invalides arrêtent le nœud. **Exemple :** Déclencheur de bouton → Exécuter une commande terminal → Lire le champ `terminal.output`.

## Notification de bureau

**Objectif :** affiche une notification Windows. **Pins :** entrée/sortie d’exécution, entrées Titre/Message, objet résultat. **Configuration :** titre et message. **Produit :** titre/message affichés. **Autorisation :** aucune. **Échec :** les erreurs de notification des plateformes non prises en charge sont enregistrées. **Exemple :** Branche vraie → Notification de bureau.

## Git

**Objectif :** exécute une opération Git ciblée. **Pins :** entrée/sortie d’exécution, entrées Opération/Dépôt, objet résultat. **Configuration :** opération prise en charge et dépôt. **Produit :** opération et sortie. **Autorisation :** Git. **Échec :** les erreurs de dépôt et de commande sont enregistrées. **Exemple :** Déclencheur Cron → Git status → Créer un rapport.
