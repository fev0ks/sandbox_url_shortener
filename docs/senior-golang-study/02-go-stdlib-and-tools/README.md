# Go Stdlib And Tools

Сюда собирай заметки по стандартной библиотеке и официальным инструментам.

Что покрыть:
- `net/http`, middleware, transports, connection reuse;
- `context`, `database/sql`, `sync`, `sync/atomic`;
- `encoding/json`, ограничения и типовые ошибки;
- `expvar`, `pprof`, `runtime`, `runtime/trace`;
- `testing`, `httptest`, benchmark и fuzzing;
- `log/slog`, structured logging;
- `go test`, `go vet`, `go tool pprof`, `go tool trace`, `go generate`.

Полезные сравнения:
- `http.Client` reuse vs создание клиента на каждый запрос;
- `sync.Mutex` vs `sync.RWMutex`;
- channels vs mutexes;
- `database/sql` напрямую vs ORM/query builder поверх него.
