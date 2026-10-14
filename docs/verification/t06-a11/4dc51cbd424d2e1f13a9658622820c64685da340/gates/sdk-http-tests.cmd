argv: go test -count=1 ./pkg/deltascope ./internal/interfaces/http -run T06A11 -v
rc=0
=== RUN   TestT06A11UnsignedPassSDK
--- PASS: TestT06A11UnsignedPassSDK (0.00s)
=== RUN   TestT06A11BigintRejectSDK
--- PASS: TestT06A11BigintRejectSDK (0.00s)
PASS
ok  	github.com/Fanduzi/DeltaScope/pkg/deltascope	0.570s
=== RUN   TestHandlerT06A11UnsignedPass
[GIN-debug] [WARNING] Running in "debug" mode. Switch to "release" mode in production.
 - using env:	export GIN_MODE=release
 - using code:	gin.SetMode(gin.ReleaseMode)

[GIN-debug] GET    /healthz                  --> github.com/Fanduzi/DeltaScope/internal/interfaces/http.NewHandler.func1 (8 handlers)
[GIN-debug] GET    /readyz                   --> github.com/Fanduzi/DeltaScope/internal/interfaces/http.NewHandler.func2 (8 handlers)
[GIN-debug] GET    /version                  --> github.com/Fanduzi/DeltaScope/internal/interfaces/http.NewHandler.func3 (8 handlers)
[GIN-debug] GET    /v1/rules                 --> github.com/Fanduzi/DeltaScope/internal/interfaces/http.NewHandler.func4 (8 handlers)
[GIN-debug] GET    /v1/rules/:rule_id        --> github.com/Fanduzi/DeltaScope/internal/interfaces/http.NewHandler.func5 (8 handlers)
[GIN-debug] GET    /v1/capabilities          --> github.com/Fanduzi/DeltaScope/internal/interfaces/http.NewHandler.func6 (8 handlers)
[GIN-debug] POST   /v1/audit                 --> github.com/Fanduzi/DeltaScope/internal/interfaces/http.NewHandler.func7 (8 handlers)
[GIN-debug] POST   /v1/query-access/analyze  --> github.com/Fanduzi/DeltaScope/internal/interfaces/http.NewHandler.func8 (8 handlers)
[GIN-debug] GET    /metrics                  --> github.com/Fanduzi/DeltaScope/internal/interfaces/http.NewHandler.WrapH.func13 (8 handlers)
2026/10/15 00:04:52 {"timestamp":"2026-10-14T16:04:52Z","level":"info","msg":"http request","method":"POST","path":"/v1/audit","status":200,"duration_ms":5,"request_id":"req-a485556a3475b5a3487ed277"}
--- PASS: TestHandlerT06A11UnsignedPass (0.01s)
=== RUN   TestHandlerT06A11BigintReject
[GIN-debug] [WARNING] Running in "debug" mode. Switch to "release" mode in production.
 - using env:	export GIN_MODE=release
 - using code:	gin.SetMode(gin.ReleaseMode)

[GIN-debug] GET    /healthz                  --> github.com/Fanduzi/DeltaScope/internal/interfaces/http.NewHandler.func1 (8 handlers)
[GIN-debug] GET    /readyz                   --> github.com/Fanduzi/DeltaScope/internal/interfaces/http.NewHandler.func2 (8 handlers)
[GIN-debug] GET    /version                  --> github.com/Fanduzi/DeltaScope/internal/interfaces/http.NewHandler.func3 (8 handlers)
[GIN-debug] GET    /v1/rules                 --> github.com/Fanduzi/DeltaScope/internal/interfaces/http.NewHandler.func4 (8 handlers)
[GIN-debug] GET    /v1/rules/:rule_id        --> github.com/Fanduzi/DeltaScope/internal/interfaces/http.NewHandler.func5 (8 handlers)
[GIN-debug] GET    /v1/capabilities          --> github.com/Fanduzi/DeltaScope/internal/interfaces/http.NewHandler.func6 (8 handlers)
[GIN-debug] POST   /v1/audit                 --> github.com/Fanduzi/DeltaScope/internal/interfaces/http.NewHandler.func7 (8 handlers)
[GIN-debug] POST   /v1/query-access/analyze  --> github.com/Fanduzi/DeltaScope/internal/interfaces/http.NewHandler.func8 (8 handlers)
[GIN-debug] GET    /metrics                  --> github.com/Fanduzi/DeltaScope/internal/interfaces/http.NewHandler.WrapH.func13 (8 handlers)
2026/10/15 00:04:52 {"timestamp":"2026-10-14T16:04:52Z","level":"info","msg":"http request","method":"POST","path":"/v1/audit","status":200,"duration_ms":2,"request_id":"req-33fe19d1d7829fb9f9142026"}
--- PASS: TestHandlerT06A11BigintReject (0.01s)
PASS
ok  	github.com/Fanduzi/DeltaScope/internal/interfaces/http	0.665s
