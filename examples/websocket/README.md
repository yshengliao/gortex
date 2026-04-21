# WebSocket — chat with size cap + authorizer

A minimal WebSocket chat room built on `transport/websocket`. The hub is
configured with the hardened defaults from PR 2: a 4 KiB per-frame read
cap, an allow-list of message types (`chat`, `ping`), and an authorizer
hook that rejects the `banned` user and enforces a non-empty `text`
field on `chat` messages.

## Run

```sh
go run ./examples/websocket
```

Server listens on `:8080`. The chat endpoint is `ws://localhost:8080/chat`
and takes a `?user=<name>` query parameter for the sender id.

## Try it

Using [`websocat`](https://github.com/vi/websocat):

```sh
# Terminal A
websocat 'ws://localhost:8080/chat?user=alice'
# <- {"type":"welcome","data":{"client_id":"...","message":"Connected to server"}}

# Terminal B
websocat 'ws://localhost:8080/chat?user=bob'
# <- {"type":"welcome",...}

# From Alice: broadcast a chat message
{"type":"chat","data":{"text":"hi bob"}}
# Bob receives: {"type":"chat","data":{"text":"hi bob"},"client_id":"..."}

# Rejected — unknown type is not in the allow-list, server logs a warning
{"type":"hack","data":{}}

# Rejected — authorizer requires a non-empty text
{"type":"chat","data":{}}

# Rejected — banned user is blocked on every message
# (connect with ?user=banned and try to send anything)
```

Connections beyond the 4 KiB read limit are dropped by
`conn.SetReadLimit` inside `ReadPump`.
