# Go Libraries And Ecosystem

Сюда лучше складывать сравнения библиотек по сценариям, а не просто списки.

Категории:
- HTTP routers: `chi`, `gin`, `echo`, stdlib router;
- config: `envconfig`, `viper`, manual parsing;
- logging: `slog`, `zap`, `zerolog`;
- database access: `sqlx`, `pgx`, `gorm`, `bun`, `ent`;
- validation: `go-playground/validator`;
- DI/wiring: `google/wire`, `uber/fx`, manual composition;
- testing: `testify`, `go-cmp`, `gomock`, `testcontainers-go`;
- messaging: official clients for Kafka, RabbitMQ, NATS, Redis streams;
- observability: OpenTelemetry SDKs and exporters.

Для каждой библиотеки фиксируй:
- сильные стороны;
- слабые стороны;
- где уместна;
- почему выбрал бы ее в production;
- какие проблемы она создает команде через полгода.
