# Secrets Configuration

Set these secrets in Fly.io before deployment:

```bash
fly secrets set DATABASE_URL="postgresql://..."
fly secrets set FLY_API_TOKEN="..."
fly secrets set FLY_APP_NAME="execbox-sessions"
fly secrets set FLY_ORG="personal"
```

## Required Secrets

- **DATABASE_URL**: PostgreSQL connection string
  - Format: `postgresql://user:password@host:port/database?sslmode=require`
  - Get from Fly.io Postgres: `fly postgres create`

- **FLY_API_TOKEN**: Fly.io API authentication token
  - Get from: `fly auth token`

- **FLY_APP_NAME**: Name of the Fly.io app for session machines
  - Default: `execbox-sessions`

- **FLY_ORG**: Fly.io organization slug
  - Get from: `fly orgs list`
  - Default: `personal`

## Verification

Check configured secrets:
```bash
fly secrets list
```

## Notes

- Never commit actual secret values to version control
- Rotate secrets regularly
- Use different secrets for staging/production environments
