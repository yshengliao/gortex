# Basic — REST CRUD with struct-tag routing

A single-file Todo service that shows the minimum Gortex wiring:
`app.NewApp(...)`, struct-tag routes, the built-in binder, and the
default middleware chain (recovery, request-id, logger, CORS, error
handler, gzip).

## Run

```sh
go run ./examples/basic
```

The server listens on `:8080`. Send SIGINT (Ctrl+C) for graceful
shutdown.

## Routes

| Method | Path         | Purpose           |
| ------ | ------------ | ----------------- |
| GET    | /todos       | List all todos    |
| POST   | /todos       | Create a todo     |
| GET    | /todos/:id   | Fetch one todo    |
| PATCH  | /todos/:id   | Update a todo     |
| DELETE | /todos/:id   | Delete a todo     |

## Try it

```sh
# Create
curl -s -X POST localhost:8080/todos \
    -H 'Content-Type: application/json' \
    -d '{"title":"write the docs"}'
# -> {"id":1,"title":"write the docs","done":false}

# List
curl -s localhost:8080/todos
# -> [{"id":1,"title":"write the docs","done":false}]

# Mark done
curl -s -X PATCH localhost:8080/todos/1 \
    -H 'Content-Type: application/json' \
    -d '{"done":true}'
# -> {"id":1,"title":"write the docs","done":true}

# Delete
curl -s -o /dev/null -w '%{http_code}\n' -X DELETE localhost:8080/todos/1
# -> 204

# Validation errors surface as 400 via httpctx.NewHTTPError:
curl -s -X POST localhost:8080/todos \
    -H 'Content-Type: application/json' \
    -d '{}'
# -> {"message":"title is required"}
```
