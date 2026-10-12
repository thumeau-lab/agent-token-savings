<p align="center">
  <picture><source media="(prefers-color-scheme: dark)" srcset="docs/img/hero-dark.svg"><img src="docs/img/hero.svg" alt="agent-token-savings : 71 % de la note en écriture de cache, 68 % du préfixe en schémas d'outils, -61 % de sortie git log via rtk" width="100%"></picture>
</p>

<p align="center">
  <b>Dépenser moins avec n'importe quel agent de code IA. Même travail, moins de tokens.</b><br>
  Des méthodes valables chez tous les fournisseurs, des chiffres mesurés sur de vraies machines, des scripts pour mesurer ton installation.
</p>

<p align="center">
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-555555?style=flat-square" alt="Licence MIT"></a>
  <img src="https://img.shields.io/badge/python-stdlib_only-3776ab?style=flat-square&logo=python&logoColor=white" alt="Python stdlib seule">
  <img src="https://img.shields.io/badge/works_with-Claude_Code_%7C_Codex_%7C_Gemini_CLI_%7C_Cursor_%7C_Aider_%7C_OpenCode_%7C_any_API-1b1b19?style=flat-square" alt="Compatible Claude Code, Codex, Gemini CLI, Cursor, Aider, OpenCode, toute API">
</p>

<p align="center"><a href="README.md">English</a> | Français</p>

> Les pages de détail ([`docs/`](docs/)) et [`SETUP.md`](SETUP.md) sont en anglais.

## Installation en 30 secondes

Colle ce message à ton agent de code. Il mesure ta consommation, propose chaque changement
avant de l'écrire, puis te donne l'avant / après.

```text
Lis https://raw.githubusercontent.com/thumeau-lab/agent-token-savings/main/SETUP.md et applique-le à mon installation.
```

[`SETUP.md`](SETUP.md) est écrit pour un LLM : il détecte l'agent et le fournisseur, mesure,
applique les leviers dans l'ordre d'impact et liste ce qu'il ne faut jamais faire.

## Pourquoi ton agent brûle des tokens

Un agent renvoie **toute la conversation** au modèle à chaque message. On croit payer ce que
le modèle écrit ; on paie surtout ce qu'il relit, et encore plus ce qu'il doit réécrire
quand son cache casse. Sur 21 jours chez nous, la réponse du modèle pesait 9 % de la note,
la réécriture du cache 71 %.

Trois idées couvrent l'essentiel du gain, quels que soient l'outil et le modèle :

1. **Ne pas casser le cache** : le début du prompt ne doit jamais bouger.
2. **Raccourcir les sessions** : une session par sujet, compaction plus tôt.
3. **Charger moins au départ** : chaque outil, skill ou serveur MCP chargé se repaie à chaque message.

## Comment marche le cache

Tous les grands fournisseurs (Anthropic, OpenAI, Google, DeepSeek...) mettent en cache le
**début** du prompt. À chaque tour, la partie déjà vue est relue en cache pour une fraction du
prix (vert) ; seule la fin est écrite (orange). Un octet qui change en tête, et tout est réécrit.

<p align="center"><picture><source media="(prefers-color-scheme: dark)" srcset="docs/img/cache-timeline-dark.svg"><img src="docs/img/cache-timeline.svg" alt="7 tours, le préfixe en cache grandit, une modification au tour 6 force une réécriture complète" width="100%"></picture></p>

<p align="center"><picture><source media="(prefers-color-scheme: dark)" srcset="docs/img/prompt-order-dark.svg"><img src="docs/img/prompt-order.svg" alt="Avant : la date en tête force une réécriture complète. Après : début stable relu en cache, seule la fin est écrite." width="100%"></picture></p>

Les règles varient selon le fournisseur (activation, durée de vie, surcoût d'écriture, taille
minimale) : voir [`docs/providers.md`](docs/providers.md).


## Les méthodes

| Méthode | Comment | Gain mesuré |
|---|---|---|
| Préfixe figé | rien d'horodaté ni d'aléatoire en tête du prompt ; ne pas changer de modèle, d'effort de raisonnement ou de serveurs MCP en cours de session | la réécriture du cache faisait 71 % de la note |
| Durée de cache adaptée | TTL long quand un humain répond lentement, court pour sous-agents et crons | -18,7 % d'écriture sur une machine à crons |
| Sessions courtes | une session par sujet, compaction vers 150k tokens au lieu de la moitié de la fenêtre | -58 % de contexte relu par appel (un agent, corrélation) |
| Moins d'outils chargés | couper serveurs MCP, skills et outils inutilisables sur la machine | les schémas d'outils faisaient 68 % du préfixe d'un agent |
| Petit modèle pour le travail simple | sous-agents, résumés et titres sur un modèle léger | prix par token (doc fournisseur) |
| Sortie shell, verbosité | filtres de sortie comme [RTK](https://github.com/rtk-ai/rtk), réponses courtes | -43 à -61 % d'octets sur `git`, mais poste mineur |

## Les commandes par outil

| Outil | Nouvelle session | Compacter | Voir ce qui remplit le contexte | Cache |
|---|---|---|---|---|
| Claude Code | `/clear` | `/compact <focus>` | `/context`, `/mcp` | automatique |
| Codex CLI | `/new` | `/compact` | `/status`, `/mcp` | automatique (OpenAI) |
| Gemini CLI | `/clear` | `/compress` | `/stats`, `/memory list`, `/mcp` | implicite (Gemini 2.5+) |
| OpenCode | `/new` | `/compact` | | selon le fournisseur |
| Aider | `/clear` | | `/tokens` | `--cache-prompts`, `--cache-keepalive-pings N` |
| Cursor | nouveau chat | | | côté fournisseur |
| Hermes Agent | `/new` | `compression.threshold_tokens` | `hermes prompt-size` | `prompt_caching.cache_ttl` |
| Ton propre code | | | `usage` dans chaque réponse | voir [`providers.md`](docs/providers.md) |

Détail, sources et niveau de preuve de chaque levier : [`docs/levers.md`](docs/levers.md). Tous
nos chiffres : [`docs/measurements.md`](docs/measurements.md).

## Mesurer soi-même

Python 3, bibliothèque standard seule. Les scripts ne font que lire ; les chemins par défaut
sont ceux d'une installation standard.

```
npx ccusage daily
python3 tools/claude_cache_ratio.py
python3 tools/hermes_usage.py --days 14
```

[`ccusage`](https://github.com/ccusage/ccusage) lit les journaux locaux des principales CLI de
code. Avec ton propre code, lis les champs d'usage listés dans
[`providers.md`](docs/providers.md). Regarde les tokens **relus et écrits par appel**, pas le
taux de hit du cache : il reste autour de 90 % même quand la note est salée. Pour trouver le
bloc qui casse le cache, `tools/cache_diff.py` capture deux requêtes via un proxy local et
nomme le premier bloc qui diffère (clients compatibles Anthropic). Plus dans
[`docs/measurements.md`](docs/measurements.md#reproduce).

## Limites

Un seul utilisateur, nos machines, surtout des modèles Anthropic. Avant / après en corrélation,
pas en expérience contrôlée. Dollars au tarif API public ; sur abonnement, ils mesurent la part
de quota consommée. Les méthodes sont générales, tes chiffres seront différents : c'est pour ça
que les scripts sont là.

## Licence

MIT, voir [`LICENSE`](LICENSE).
