# ADR-0003 — Déploiement AWS Free Tier + Terraform

| Champ  | Valeur |
|--------|--------|
| Statut | **Accepté** |
| Date   | 2026-07-07 |

## Ressources AWS Free Tier

| Service | Instance | Free Tier |
|---------|----------|-----------|
| EC2 | t2.micro | 750h/mois |
| RDS PostgreSQL | db.t3.micro | 750h/mois |
| ElastiCache Redis | cache.t3.micro | 750h/mois |

**Coût estimé** : ~$0-2/mois.

## Stratégie

1. `terraform apply` → infra AWS
2. `user_data` sur EC2 → télécharge binaire depuis S3, démarre systemd
3. GitHub Actions → rebuild + push S3 + `systemctl restart gateway` via SSM

## Extinction démo

```bash
make tf-destroy   # supprime TOUT → coût $0
```

## Conséquences

- **Positif** : Infra reproductible et détruite en une commande
- **Positif** : Free Tier couvre 12 mois
- **Négatif** : RDS + ElastiCache payants après 12 mois
