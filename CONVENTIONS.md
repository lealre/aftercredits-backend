# Backend code conventions

Conventions for the Go backend. New code should follow these; when touching old
code, nudge it toward them.

## 1. Handlers don't touch the database directly

HTTP handlers (`internal/api/*.go`) orchestrate request/response only. All data
access goes through a **service** (`internal/services/<domain>`), which owns the
DB calls. A handler should call `service.DoThing(db, ...)`, never
`api.Db.SomeCollectionQuery(...)` inline.

Why: keeps business logic and persistence testable and reusable, and keeps
handlers thin.

> Status: this is the target. Some existing handlers still call the DB directly
> for existence/membership checks; routing those through services is tracked as a
> dedicated follow-up refactor.

## 2. Errors are defined in the service, mapped to HTTP in the handler

Each service package declares its error vocabulary as sentinel errors plus an
`ErrorMap` in `internal/services/<domain>/utils.go`:

```go
var ErrThing = errors.New("thing not found")
var ErrorMap = map[error]int{ ErrThing: http.StatusNotFound }
```

Handlers translate, they don't invent error strings:

```go
if code, ok := domain.ErrorMap[err]; ok {
    respondWithError(w, code, err.Error())
    return
}
// otherwise: log + 500
```

Error strings and their HTTP statuses live in one place (the service), not
scattered across handlers. Services translate lower-level errors (e.g. a
provider's `ErrTitleNotFound`) into their own vocabulary before returning.

## 3. Comment invariants (movie vs. TV series)

A stored comment (`internal/mongodb` comments) has a top-level `Comment` and a
per-season `SeasonsComments` map. Exactly one is used, by title type:

- **Movie:** top-level `Comment` is set; `SeasonsComments` MUST be empty/nil.
- **TV series:** top-level `Comment` MUST be empty/nil; `SeasonsComments` holds
  one entry per season.

The add/update flows enforce this by construction (separate movie vs. series
code paths). Don't write a path that populates both.

## Related

- Title metadata providers: [internal/titleprovider/README.md](internal/titleprovider/README.md)
