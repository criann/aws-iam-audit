# AWS IAM Audit

[![Go Version](https://img.shields.io/badge/Go-1.25.6-blue)](https://golang.org/)
[![AWS SDK](https://img.shields.io/badge/AWS%20SDK-v2-green)](https://aws.github.io/aws-sdk-go-v2/)

**[English](README.md)**

Un outil CLI en Go pour auditer les ressources AWS IAM et S3 sur vos comptes AWS. Il récupère les utilisateurs, groupes, rôles, leurs politiques attachées et les configurations des buckets S3, exportant le tout sous forme de JSON structuré.

## Fonctionnalités

- **Audit des ressources IAM**: Utilisateurs (Users), Groupes (Groups) et Rôles (Roles) avec leurs politiques (Policies) attachées
- **Audit des buckets S3**: Buckets avec politiques, versioning, verrouillage d'objets et paramètres d'accès public
- **Support AWS Organizations**: Récupère automatiquement tous les comptes de votre organisation
- **Récupération des documents de politique**: Récupère et décode les documents de politique pour les politiques gérées par le client
- **Opération avec un seul profil**: Fonctionne avec un seul profil AWS, gérant automatiquement l'accès multi-comptes
- **Sortie JSON**: Sortie JSON propre et structurée pour un parsing et une analyse faciles

## Prérequis

- Go 1.23 ou ultérieur
- Identifiants AWS configurés avec les permissions appropriées (IAM, S3, Organizations)

## Installation

```bash
# Cloner le dépôt
git clone <url-du-repository>
cd aws-iam-audit

# Télécharger les dépendances
go mod download

# Compiler le binaire
go build -o aws-iam-audit

# (Optionnel) Installer dans le PATH
go install
```

## Utilisation

```bash
./aws-iam-audit --profile <profil-aws>
```

### Drapeaux requis

- `--profile`: Profil AWS à utiliser depuis `~/.aws/credentials` ou `~/.aws/config`

### Exemple

```bash
./aws-iam-audit --profile mon-profil-aws > sortie-audit.json
```

## Format de sortie

L'outil génère une structure JSON avec le format suivant :

```json
{
  "metadata": {
    "generated_at": "2026-02-03T13:34:15Z",
    "profile": "mon-profil",
    "current_account": "123456789012",
    "current_arn": "arn:aws:iam::123456789012:user/..."
  },
  "accounts": [
    {
      "account_id": "123456789012",
      "account_name": "production",
      "account_status": "ACTIVE",
      "resources": {
        "users": [...],
        "groups": [...],
        "roles": [...],
        "buckets": [...]
      }
    }
  ]
}
```

## Permissions AWS requises

Le profil AWS utilisé doit avoir les permissions suivantes :

- `iam:ListUsers`, `iam:GetUser`
- `iam:ListGroups`, `iam:GetGroup`
- `iam:ListRoles`, `iam:GetRole`
- `iam:ListAttachedUserPolicies`, `iam:GetPolicy`, `iam:GetPolicyVersion`
- `iam:ListAttachedGroupPolicies`
- `iam:ListAttachedRolePolicies`
- `s3:ListAllMyBuckets`, `s3:GetBucketLocation`, `s3:GetBucketPolicy`
- `s3:GetBucketVersioning`, `s3:GetObjectLockConfiguration`, `s3:GetPublicAccessBlock`
- `organizations:ListAccounts` (optionnel, pour le support multi-comptes)
- `sts:GetCallerIdentity`

## Exemple de sortie

Voir [example.json](example.json) pour un exemple complet de sortie.

## Développement

### Exécuter directement

```bash
go run main.go --profile mon-profil-aws
```

### Compiler

```bash
go build -o aws-iam-audit
```

### Exécuter les tests

```bash
go test ./...
```

## Licence

MIT
