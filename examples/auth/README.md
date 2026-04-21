# Auth — JWT login + refresh

Shows the `pkg/auth` JWT service end-to-end: login with a
username/password, receive an access + refresh token, call a protected
endpoint, then swap the refresh token for a new access token.

`auth.NewJWTService` refuses secrets shorter than 32 bytes
(`auth.MinJWTSecretBytes`), so the example loads its secret from the
`JWT_SECRET` env var and fails fast if it is missing or weak.

## Run

```sh
JWT_SECRET='this-is-a-long-enough-test-secret-32b' go run ./examples/auth
```

Demo account:

| Field    | Value    |
| -------- | -------- |
| Username | `alice`  |
| Password | `s3cret` |

## Routes

| Method | Path           | Purpose                            |
| ------ | -------------- | ---------------------------------- |
| POST   | /auth/login    | Exchange credentials for tokens    |
| POST   | /auth/refresh  | Swap a refresh token for an access |
| GET    | /me            | Echo caller's claims (auth-gated)  |

## Try it

```sh
# Login
TOKENS=$(curl -s -X POST localhost:8080/auth/login \
    -H 'Content-Type: application/json' \
    -d '{"username":"alice","password":"s3cret"}')
echo "$TOKENS"
# -> {"access_token":"eyJhbGci...","refresh_token":"eyJhbGci...","expires_in":3600}

ACCESS=$(echo "$TOKENS" | jq -r .access_token)
REFRESH=$(echo "$TOKENS" | jq -r .refresh_token)

# Protected endpoint
curl -s localhost:8080/me -H "Authorization: Bearer $ACCESS"
# -> {"email":"alice@example.test","role":"member","user_id":"user-1","username":"alice"}

# Refresh
curl -s -X POST localhost:8080/auth/refresh \
    -H 'Content-Type: application/json' \
    -d "{\"refresh_token\":\"$REFRESH\"}"
# -> {"access_token":"eyJhbGci...","expires_in":3600}

# Bad credentials
curl -s -o /dev/null -w '%{http_code}\n' -X POST localhost:8080/auth/login \
    -H 'Content-Type: application/json' \
    -d '{"username":"alice","password":"wrong"}'
# -> 401
```
