---
title: "HelmDownloader : des scripts bash au binaire unique"
date: 2026-07-14
tags: ["kubernetes", "helm", "airgap", "golang"]
params:
  author: "Julien HOMMET"
draft: false
---

Si vous avez déjà eu à déployer Kubernetes sur une plateforme déconnectée d'Internet, vous connaissez la corvée. Pas de `helm pull` qui récupère tout le nécessaire depuis une connexion à Internet. Pas de `docker pull` qui va chercher l'image au moment du déploiement. Tout doit être préparé **avant**, transféré sur un support, puis rechargé de l'autre côté du mur.

Et le pire dans Helm, ce n'est pas le chart. C'est de retrouver **toutes** les images de conteneurs qu'il embarque. Une seule chart un peu sérieuse en cache une dizaine : le conteneur principal, les sidecars, les initContainers, l'export de métriques, le webhook d'admission… Oubliez-en une, et votre déploiement reste bloqué en `ImagePullBackOff` au pire moment.

HelmDownloader (**v0.3.0**) règle exactement ce problème.

## Le point de départ : une collection de scripts bash

À l'origine, ce n'était pas un outil. C'était une **collection de scripts bash**. Un script par application. Un pour Argo CD, un pour Prometheus, un pour cert-manager, et ainsi de suite.

Le problème de cette approche saute vite aux yeux :

- **Pas modulaire** : chaque script dupliquait la même logique (pull du chart, extraction des images, retag, archivage), avec des variantes copiées-collées d'un fichier à l'autre.
- **Pas pratique** : ajouter une nouvelle application, c'était repartir d'un script existant, le bricoler, et prier pour ne rien casser.
- **Peu évolutif** : la liste des images était souvent codée en dur. Une nouvelle version de la chart change ses images ? Il faut rouvrir le script et le corriger à la main.

Bref, ça marchait, mais c'était fragile, pénible à maintenir, et ça ne passait pas à l'échelle.

## Le tournant : un binaire unique avec Claude Code

Plutôt que d'écrire un énième script bash, j'ai utilisé [Claude Code](https://www.claude.com/product/claude-code) pour repenser l'outil de zéro et le transformer en **binaire unique** écrit en Go.

Le gain est net :

- **Un seul outil, toutes les charts.** Plus de script par application. On cherche n'importe quelle chart, on la sélectionne, et le même pipeline s'occupe du reste.
- **Tri et filtrage des résultats.** Triez les résultats par étoiles, par nom ou par date de dernière mise à jour, et filtrez par auteur ou par société éditrice — le tout sur les résultats déjà récupérés, sans nouvelle requête. Des badges official / deprecated aident à choisir le bon paquet.
- **Découverte automatique des images.** Fini la liste codée en dur. L'outil fait un `helm template`, parcourt récursivement tous les manifestes rendus, le `values.yaml` racine, et chaque `charts/*/values.yaml` de subchart, et extrait toutes les références d'images — y compris la forme éclatée `registry` / `repository` / `tag` / `digest` qu'utilisent beaucoup de charts. Passez `-values` / `-set` pour faire remonter les images conditionnées par des values non par défaut.
- **Plus rapide pour bundler.** Le pull des images est *daemonless* via [go-containerregistry](https://github.com/google/go-containerregistry) : **pas besoin de Docker**. Les images se téléchargent en parallèle avec progression par image, retry et backoff exponentiel. Une image en échec n'interrompt pas le lot — on voit l'ensemble des échecs d'un coup. Esc annule un téléchargement en cours tout en conservant les succès partiels.
- **Bundles vérifiables.** Chaque fichier du bundle est sha256'é dans `sha256sums.txt` (y compris `load.sh`), les digests de manifeste sont épinglés dans `images.txt` et `manifest.json`, un SBOM SPDX 2.3 est écrit en `sbom.spdx.json`, et le `load.sh` généré vérifie les checksums avant de pousser.
- **Outils d'intégrité.** `helmdownloader verify <bundle>` recalcule les empreintes hors ligne après transfert ; `helmdownloader diff <a> <b>` montre quelles images ont été ajoutées, supprimées ou modifiées entre deux versions de chart.
- **Revue sécurité.** Exportez la liste d'images découverte en JSON (`-export-images`), faites-la valider par une équipe sécu, réimportez le jeu approuvé (`-import-images`). Les refs invalides échouent avant le téléchargement.
- **Registries privées.** `-registry-auth` s'appuie sur le keychain Docker par défaut (`docker login` / `podman login`, ou `$DOCKER_CONFIG`).
- **Archives plus légères.** Au choix, compression `gzip` (`.tar.gz`) ou `zstd` (`.tar.zst`). Un contrôle d'espace disque et un flag `-resume` renforcé (réutilise les tarballs seulement si les sidecars content-hash et digest registry correspondent) rendent les gros lots sûrs à relancer.
- **Thèmes.** Six palettes (`auto`, `light`, `dark`, `high-contrast`, `ocean`, `matrix`) via `-theme` ou en direct avec `Ctrl+T`.
- **Distribuable.** Un binaire Go statique (v0.3.0), compilé pour Linux, macOS et Windows, en amd64 et arm64. On le pose, il tourne. `helmdownloader version` affiche le tag de release embarqué.
- **Sans démon Docker.** Pas besoin d'avoir `docker` ou `podman` sur la machine de récolte pour le pull — *HelmDownloader* utilise une librairie Go interne (un moteur de conteneurs n'est nécessaire que plus tard, côté airgap, pour exécuter `load.sh`).
- **Pulls hermétiques.** Chaque `helm pull` s'exécute avec une config et un cache de dépôts privés, isolés dans le répertoire de travail : l'outil ignore totalement vos dépôts helm globaux. Un dépôt local périmé ou supprimé ne peut plus casser un pull avec `Error: no cached repo found. (try 'helm repo update')` — inutile de lancer `helm repo update` au préalable.

Là où chaque nouvelle application demandait un nouveau script bash, on a maintenant un seul outil qui les couvre toutes — et qui s'adapte automatiquement aux changements de version.

## À quoi ça sert, concrètement

HelmDownloader est une application **TUI** (interface en terminal) qui :

1. **Cherche** une chart Helm sur [ArtifactHub](https://artifacthub.io)
2. **Sélectionne** la chart et la version voulues
3. **Découvre automatiquement** toutes les images de conteneurs de la chart
4. **Laisse réviser** la liste : ajout, suppression, activation/désactivation image par image (liste fenêtrée pour les grandes charts ; refs validées à l'ajout)
5. **Télécharge** les images (sans Docker) et les retague pour votre registry privé
6. **Assemble** le tout dans **une seule archive compressée** prête à traverser l'airgap

Le bundle produit contient la chart, ses `values.yaml`, chaque image sous forme de tarball, un manifeste `images.txt` qui mappe les références d'origine vers leurs versions retaguées (avec digests épinglés), un fichier de provenance `manifest.json` (outil + version, chart, codec, images), un `sbom.spdx.json` SPDX 2.3, une liste de checksums `sha256sums.txt`, et un script `load.sh` qui vérifie les checksums, puis recharge et pousse tout vers votre registry de l'autre côté. `load.sh` est idempotent (saute les images déjà présentes) et respecte `DRY_RUN=1`.

C'est le cas d'usage **airgap** par excellence : un seul fichier à transférer, une seule commande à lancer à l'arrivée.

## Mini-tuto : du chart au bundle en 2 minutes

### Prérequis

[Helm](https://helm.sh/docs/intro/install/) doit être installé et présent dans le `PATH`. C'est la seule dépendance runtime sur la machine connectée — le pull des images, lui, ne nécessite aucun démon Docker.

### Installation

```bash
go install github.com/julienhmmt/helmdownloader@latest
```

Ou récupérez directement le binaire de votre plateforme sur la [page des releases](https://github.com/julienhmmt/helmdownloader/releases).

### Étape 1 — Lancer l'outil

Simplement en faisant un `./helmdownloader`.

Il existe des arguments CLI pour personnaliser l'outil : adresse de registry, plateforme, thèmes, logs, répertoire de destination, et plus encore.

```bash
helmdownloader \
  -registry-prefix "rgy01.domain.local" \
  -platform "linux/amd64" \
  -output "./archives" \
  -theme dark
```

L'interface s'ouvre sur un écran de recherche. Appuyez sur `Ctrl+T` à tout moment pour ouvrir le sélecteur de thème.

### Étape 2 — Chercher et sélectionner

Tapez le nom d'une chart (par exemple `argo-cd`), `Entrée`, puis naviguez dans les résultats pour choisir la chart et sa version. Chaque ligne affiche les étoiles, le dépôt, l'éditeur, la version applicative et la description, plus des badges official / deprecated le cas échéant. Sur une liste chargée, appuyez sur `s` pour trier par étoiles, nom ou date de mise à jour, `o` pour inverser le sens, et `f`/`F`/`Tab` pour filtrer par auteur ou société.

| Écran | Touches |
| ----- | ------- |
| Recherche | `Entrée` pour chercher, `Ctrl+T` thèmes, `Échap` pour quitter |
| Résultats | `Entrée` sélectionner, `/` filtre flou, `s` champ de tri, `o` sens, `f` champ de filtre, `F` saisir filtre, `Tab` parcourir les valeurs |
| Versions | `Entrée` pour sélectionner, `/` pour filtrer |
| Révision | `Espace` (dé)cocher, `a` ajouter, `d` supprimer, `j`/`k` déplacer, `PgUp`/`PgDn` page, `Entrée` télécharger |
| Téléchargement | `Échap` annuler (conserve les succès partiels) |
| Terminé | `n` nouveau bundle, `q` quitter |

### Étape 3 — Réviser les images

L'outil affiche toutes les images qu'il a découvertes. Une image conditionnelle a été manquée (cachée derrière un `{{- if .Values.monitoring.enabled }}` désactivé par défaut) ? Appuyez sur `a` pour l'ajouter à la main — les refs sont validées avant d'entrer dans la liste. `Espace` pour décocher ce dont vous n'avez pas besoin. Les listes longues sont fenêtrées pour rester utilisables sur les grosses charts.

Pour élargir la découverte dès le lancement :

```bash
helmdownloader -set monitoring.enabled=true -values ./extra-values.yaml
```

### Étape 4 — Télécharger et bundler

`Entrée` lance le pipeline. Les images se téléchargent en parallèle avec une barre de progression par image, sont retaguées vers la registry si spécifié, et l'archive `argo-cd-<version>-bundle.tar.gz` (ou `.tar.zst` avec `-compression zstd`) apparaît dans `./archives`. Les succès partiels sont conservés si vous annulez en cours de route avec Échap.

### Étape 5 — De l'autre côté de l'airgap

Transférez le bundle sur la plateforme déconnectée, puis :

```bash
# contrôle hors ligne optionnel avant extract/load
helmdownloader verify argo-cd-1.0.0-bundle.tar.gz

tar xzf argo-cd-1.0.0-bundle.tar.gz
./load.sh                 # vérifie les checksums, puis charge + pousse (docker par défaut)
ENGINE=podman ./load.sh   # ou utilisez podman
DRY_RUN=1 ./load.sh       # affiche les commandes load/push sans les exécuter
```

Le script vérifie `sha256sums.txt`, recharge chaque image et la pousse vers votre registry. La chart, elle, est prête à être déployée avec `helm install`.

Lors d'une montée de version de chart, comparez deux bundles pour voir exactement ce qu'il faut re-mirroir :

```bash
helmdownloader diff argo-cd-1.8.0-bundle.tar.gz argo-cd-1.9.0-bundle.tar.gz
```

## Configuration persistante

Pour ne pas répéter les mêmes flags à chaque lancement, copiez l'exemple annoté et adaptez-le :

```bash
mkdir -p ~/.config/helmdownloader
cp config.example.yaml ~/.config/helmdownloader/config.yaml
```

Exemple minimal :

```yaml
registry_prefix: "rgy01.domain.local"
platform: "linux/amd64"
output_dir: "archives"
concurrency: 4
retries: 2
compression: "gzip"          # ou zstd pour des archives plus légères
min_free_disk_mb: 500        # contrôle disque ; 0 désactive
https_proxy: "http://proxy.domain.local:3128"
theme: "auto"                # auto | light | dark | high-contrast | ocean | matrix
```

## Configuration avancée

### Options étendues de config.yaml

```yaml
registry_prefix: "rgy01.domain.local"
platform: "linux/amd64"
output_dir: "archives"
work_dir: ""                    # chemin fixe pour activer -resume ; vide = temp
concurrency: 4
retries: 2
compression: "gzip"              # gzip (.tar.gz) ou zstd (.tar.zst)
min_free_disk_mb: 500
resume: false                    # réutilise les tarballs si sidecars hash + digest matchent
https_proxy: "http://proxy.domain.local:3128"
helm_bin: "helm"
artifacthub_url: "https://artifacthub.io"
search_limit: 20
theme: "auto"
registry_auth: false
values_files: []
set_values: []
export_images: ""
import_images: ""
verbose: true
log_level: "debug"
log_file: "helmdownloader.log"  # ouvert en mode 0600 quand les logs sont actifs
```

Une copie entièrement annotée de chaque option se trouve dans [`config.example.yaml`](https://github.com/julienhmmt/helmdownloader/blob/main/config.example.yaml).

### Flags CLI supplémentaires

```bash
helmdownloader \
  -registry-prefix "my.registry.local" \
  -platform "linux/amd64" \
  -output "./archives" \
  -work-dir "./workdir" \
  -concurrency 4 \
  -retries 2 \
  -compression "zstd" \
  -min-free-mb 500 \
  -resume \
  -registry-auth \
  -values "extra-values.yaml" \
  -set "monitoring.enabled=true" \
  -export-images images.json \
  -import-images images-approved.json \
  -theme dark \
  -proxy "http://proxy.domain.local:3128" \
  -v \
  -log-level "debug" \
  -log-file "helmdownloader.log" \
  -config "~/.config/helmdownloader/config.yaml"
```

| Flag | Description |
| ---- | ----------- |
| `-config` | Chemin vers le fichier de configuration (défaut : `~/.config/helmdownloader/config.yaml`) |
| `-work-dir` | Répertoire de travail pour les fichiers intermédiaires. Vide → temporaire |
| `-concurrency` | Nombre max d'images téléchargées en parallèle (défaut : 4) |
| `-retries` | Tentatives de retry par échec de pull (défaut : 2) |
| `-compression` | Codec du bundle : `gzip` ou `zstd` |
| `-min-free-mb` | Espace disque libre minimum (MiB) avant download ; `0` désactive |
| `-resume` | Réutilise les tarballs d'un work dir persistant si les sidecars correspondent |
| `-registry-auth` | Pulls authentifiés via le keychain Docker par défaut |
| `-values` | Fichier de values supplémentaire pour la découverte d'images (répétable) |
| `-set` | Override de values `key=value` pour la découverte d'images (répétable) |
| `-export-images` | Écrit la liste d'images découverte (JSON) après le render |
| `-import-images` | Lit une liste d'images approuvée (JSON) au moment du download |
| `-theme` | Thème TUI : `auto`, `light`, `dark`, `high-contrast`, `ocean`, `matrix` |
| `-proxy` | URL du proxy pour ArtifactHub, helm et le registry |
| `-v` | Journalisation verbose (raccourci pour `--log-level=debug`) |
| `-log-level` | `silent`, `info`, ou `debug` |
| `-log-file` | Chemin pour la sortie des logs |

### Sous-commandes

```bash
helmdownloader version                              # identité du binaire (tag ou dev)
helmdownloader verify argo-cd-1.0.0-bundle.tar.gz   # contrôle d'intégrité hors ligne
helmdownloader diff old-bundle.tar.gz new-bundle.tar.gz
```

### Flux de revue sécurité

```bash
# 1. Découvrir les images, écrire la liste, quitter depuis Review (Échap) sans télécharger
helmdownloader -export-images images.json

# 2. L'équipe sécu revoit/édite images.json (selected: true/false, retire/ajoute des refs)

# 3. Relancer avec la liste approuvée
helmdownloader -import-images images.json
```

### Registries privées

```bash
docker login registry.example.com
helmdownloader -registry-auth -registry-prefix registry.example.com/mirror
# ou : DOCKER_CONFIG=/path/to/creds helmdownloader -registry-auth
```

### Variables d'environnement

Si le proxy n'est pas défini via la CLI ou la config, HelmDownloader vérifie :

- `HTTP_PROXY`
- `HTTPS_PROXY`

## Pourquoi vous devriez l'essayer

Si vous opérez Kubernetes en environnement airgap, vous avez forcément vos propres scripts pour ce genre de tâche. HelmDownloader, c'est ces scripts, mais :

- **un seul outil** au lieu d'un par application ;
- **découverte automatique** des images au lieu d'une liste à maintenir à la main ;
- **sans démon Docker** sur la machine de récolte, en parallèle, donc plus rapide ;
- **un bundle vérifiable** (digests épinglés + checksums sha256 + SBOM) avec son script de rechargement ;
- **des outils hors ligne** pour vérifier l'intégrité après transfert et comparer les montées de version.

C'est l'outil que j'aurais aimé avoir le jour où j'ai commencé à empiler les scripts. Le code est sous licence AGPL v3, ouvert aux contributions.

➡️ [github.com/julienhmmt/helmdownloader](https://github.com/julienhmmt/helmdownloader)
