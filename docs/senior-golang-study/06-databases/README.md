# Databases

Сюда складывай материалы по SQL и storage design.

Темы:
- PostgreSQL internals на практическом уровне;
- индексы: B-tree, partial, composite, covering;
- explain plans и оптимизация запросов;
- isolation levels, locking, deadlocks;
- транзакции и их границы в сервисах;
- pagination, keyset vs offset;
- connection pooling и saturation;
- Redis: caching, TTL, invalidation, hot keys;
- шардирование, репликация, read/write split;
- когда выбирать SQL, document DB, key-value, columnar storage.

Важные сравнения:
- Postgres vs MySQL для backend-сервисов;
- Redis cache-aside vs write-through;
- UUID vs sequence keys;
- soft delete vs hard delete vs event log.
