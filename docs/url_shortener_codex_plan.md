# Общий план проекта URL Shortener для Codex

**Версия:** Go 1.25  
**Назначение:** master plan для поэтапной проработки и реализации production-like pet project.

---

## 1. Цель документа

Этот документ нужен как единый ориентир для Codex. Его задача — зафиксировать общую архитектуру, границы MVP, этапы реализации, ключевые технические решения и критерии готовности. Codex должен использовать его как основу для дальнейшей детализации задач, проектирования кода и инфраструктуры.

---

## 2. Общая цель проекта

Нужно сделать сервис сокращения ссылок с простым web UI и production-like окружением.

Проект должен позволить отработать:

- backend-архитектуру на Go 1.25
- PostgreSQL, Redis, Kafka
- ClickHouse для аналитики
- Elasticsearch + Kibana для логов
- Prometheus + Grafana для метрик
- k6 для нагрузочных сценариев
- Docker Compose для локального стека
- локальную автоматизацию сборки, тестирования и деплоя
- kind + Helm для локального Kubernetes-контура
- эксплуатацию сервиса как production-like системы

Сервис должен быть не просто CRUD-приложением, а песочницей для практики архитектурных и инфраструктурных решений.

---

## 3. Ключевые пользовательские сценарии

1. **Создание короткой ссылки** через UI или API.
2. **Переход по short URL** с быстрым redirect по short code.
3. **Асинхронная публикация click events в Kafka** без влияния на redirect path.
4. **Сохранение аналитики** в ClickHouse.
5. **Просмотр логов** через Elasticsearch и Kibana.
6. **Сбор технических и бизнес-метрик** через Prometheus и Grafana.
7. **Нагрузочное и stress-тестирование** с real-time графиками во время прогона.
8. **Локальный запуск всего окружения** через Docker Compose.
9. **Локальный Kubernetes deploy** через kind + Helm после стабилизации core-функциональности.
10. **Сборка, тестирование и локальный деплой** через воспроизводимую automation flow.

---

## 4. Архитектурные принципы

- Не стартовать с микросервисной архитектуры.
- Начать с **модульного монолита**.
- Выделить два исполняемых приложения:
  - `shortener`
  - `analytics`
- Альтернативные HTTP entrypoint-ы вроде `api-http-chi` и `api-http-gin` допустимы позже как
  отдельный exploratory track, но не в базовом MVP.
- Держать четкое разделение слоев:
  - handler
  - service
  - repository
  - infrastructure
- Redirect path должен быть простым и быстрым:
  - Redis first
  - PostgreSQL fallback
- Аналитика и вторичные эффекты не должны ломать пользовательский сценарий redirect.
- Все логи должны быть structured JSON.
- Проект должен быть Docker-first и удобным для локального запуска.
- Каждый этап должен доводиться до рабочего состояния.

---

## 5. Scope

### 5.1. Что входит в MVP

- создание short links
- redirect по short code
- optional expiration date / TTL
- optional max clicks, где `0` означает отсутствие лимита
- enable / disable ссылок
- простой UI:
  - create page
  - recent links
  - link details / stats
- Redis-кеш для redirect path
- публикация click events в Kafka
- запись click events в ClickHouse
- structured logs
- metrics endpoint
- load-testing сценарии и performance dashboards
- health endpoints
- локальный запуск через Docker Compose
- локальная build/test/deploy automation
- локальный kind + Helm deploy после стабилизации core-части

### 5.2. Что не входит в MVP

- сложная аутентификация и авторизация
- многоарендность
- платные тарифы и биллинг
- обязательная поддержка пользовательских custom domains
- Kubernetes на старте
- внешний cloud/VPS deploy как обязательная цель текущего этапа
- избыточные паттерны вроде CQRS/Event Sourcing ради самих паттернов

---

## 6. Архитектурный обзор

### 6.1. Компоненты системы

- **shortener-service** — основной user-facing сервис: HTTP API, redirect handler, UI endpoints,
  health/metrics
- **analytics** — Kafka consumer для обработки событий и записи аналитики
- **PostgreSQL** — источник истины для ссылок и служебных сущностей
- **Redis** — кеш, rate limiting, idempotency
- **Kafka** — транспорт событий
- **ClickHouse** — аналитическое хранилище кликов
- **Elasticsearch** — централизованное хранилище логов
- **Kibana** — просмотр логов и поиск по request_id, short_code, status_code, error
- **Prometheus** — сбор метрик
- **Grafana** — дашборды
- **k6** — генерация контролируемой нагрузки и stress-сценариев
- **kind** — локальный Kubernetes-кластер для позднего production-like runtime
- **Helm** — основной способ упаковки и локального деплоя в kind

### 6.2. Потоки данных

#### Create link
UI/API → shortener-service → PostgreSQL → Redis warm-up → response

#### Redirect
client → shortener-service → Redis → PostgreSQL fallback → HTTP redirect → Kafka event best effort

#### Analytics
Kafka → analytics → ClickHouse

#### Logs
applications → log shipper → Elasticsearch → Kibana

#### Metrics
applications → `/metrics` → Prometheus → Grafana

#### Load testing
k6 → shortener-service → Prometheus/Grafana dashboards + PostgreSQL/Redis observability

#### Local Kubernetes deploy
local automation → Helm → kind → shortener-service/analytics

---

## 7. Предлагаемая структура репозитория

```text
shortener/
├── services/
│   ├── shortener/
│   │   ├── cmd/
│   │   │   └── shortener/
│   │   ├── Dockerfile
│   │   ├── migrations/
│   │   │   └── postgres/
│   │   └── internal/
│   │       ├── app/
│   │       ├── config/
│   │       ├── domain/
│   │       ├── repository/
│   │       ├── service/
│   │       ├── shortcode/
│   │       └── transport/
│   └── analytics/
│       ├── cmd/
│       │   └── analytics/
│       ├── Dockerfile
│       ├── migrations/
│       └── internal/
│           ├── app/
│           └── config/
├── internal/
│   └── platform/
│   │   ├── health/
│   │   ├── logger/
│   │   ├── metrics/
│   │   └── middleware/
├── web/
│   ├── templates/
│   ├── static/
│   └── assets/
├── deploy/
│   ├── docker-compose/
│   ├── helm/
│   ├── kind/
│   ├── nginx/
│   └── ci/
├── load/
├── scripts/
├── Makefile
└── README.md
```

---

## 8. Доменная модель

### 8.1. Link

Поля:

- `id`
- `short_code`
- `original_url`
- `created_at`
- `updated_at`
- `expires_at`
- `max_clicks`
- `redirect_count`
- `disabled_at`
- `deleted_at`
- `created_by`
- `title`
- `last_accessed_at`

### 8.2. ClickEvent

Поля:

- `event_id`
- `link_id`
- `short_code`
- `occurred_at`
- `ip`
- `user_agent`
- `referer`
- `country` — не присутствует в Kafka-событии `link.visited`; добавляется analytics-процессом через GeoIP-обогащение перед записью в ClickHouse. Включение GeoIP в MVP — открытый вопрос (см. E06).
- `request_id`

### 8.3. User

На старте полноценный auth можно не делать, но архитектурно не мешать его добавлению позже.

Минимальный задел:

- `id`
- `email`
- `created_at`

---

## 9. HTTP API и UI

### 9.1. Основные backend endpoints

- `POST /api/v1/links` — создание ссылки
- `GET /api/v1/links/{id}` — получение ссылки по ID
- `GET /api/v1/links/by-code/{shortCode}` — получение ссылки по short code
- `GET /{shortCode}` — redirect endpoint
- `PATCH /api/v1/links/{id}/disable` — отключить ссылку
- `PATCH /api/v1/links/{id}/enable` — включить ссылку
- `DELETE /api/v1/links/{id}` — удалить ссылку
- `GET /api/v1/stats/{shortCode}` — базовая аналитика после появления ClickHouse

### 9.2. Минимальный UI

#### Create link page
Поля:
- original URL
- expiration date optional
- max clicks optional, default `0`

#### Recent links page
Отображает:
- short URL
- original URL
- status
- created at
- expires at

#### Link details / stats page
Отображает:
- short URL
- original URL
- enabled/disabled
- created at
- expires at
- clicks total
- базовые агрегаты

### 9.3. Требования к UI

- интерфейс должен быть простым и чистым
- не нужен тяжелый SPA на первом этапе
- допустим server-rendered UI или минимальный frontend
- UI должен быть usable без Postman

---

## 10. Ключевые технические решения

### Go version
Использовать **Go 1.25**.

### Генерация short code
Рекомендуемый вариант:
- UUIDv7 как внутренний идентификатор сущности
- отдельный random short code длиной `7`
- алфавит `base58`
- retry при коллизии по unique index
- bounded retry при генерации short code, затем controlled failure

Плюсы:
- публичный код не раскрывает внутренний id
- можно контролировать длину и читаемость short code
- внутренний id и публичный код независимы

### Redirect status
Для первой версии:
- активная ссылка: `302 Found`
- expired: `410 Gone`
- max clicks reached: `410 Gone`
- disabled: `404 Not Found`
- deleted: `404 Not Found`
- unknown short code: `404 Not Found`

### Caching
Использовать **cache-aside** стратегию:
- сначала Redis
- потом PostgreSQL
- затем прогрев кеша

### Kafka delivery
Redirect не должен зависеть от успешной синхронной доставки события.
Если Kafka недоступна:
- redirect все равно должен отработать
- ошибка должна логироваться
- ошибка должна отражаться в метриках

### Logging
Сразу использовать только **structured JSON logs**.

### Metrics
Не допускать высокой кардинальности label-ов.

### Config
Все runtime-настройки через env с валидируемым config loader.

---

## 11. Поэтапный план реализации

### Этап 0. Bootstrap

#### Что сделать
- инициализировать проект на Go 1.25
- подготовить структуру каталогов
- завести базовый entrypoint `services/shortener/cmd/shortener`
- добавить config loader
- добавить structured logger
- добавить middlewares
- добавить graceful shutdown
- поднять `shortener + PostgreSQL + Redis` в docker-compose
- добавить `/health/live`, `/health/ready`, `/metrics`

#### Результат
Сервис стартует локально и готов к дальнейшей разработке.

---

### Этап 1. Core backend

#### Что сделать
- реализовать доменную модель `Link`
- сделать миграции для PostgreSQL
- реализовать create/get/enable/disable/delete endpoints
- реализовать redirect endpoint
- зафиксировать no-dedup behavior для одинаковых `original_url`
- добавить limit policy по `max_clicks`, где `0` означает unlimited
- покрыть ключевые сценарии интеграционными тестами

#### Результат
Есть рабочий backend URL shortener без event pipeline.

---

### Этап 2. Redis integration

#### Что сделать
- добавить cache-aside для redirect path
- добавить cache invalidation при update/disable/delete
- добавить rate limiting на create endpoint
- опционально добавить idempotency key

#### Результат
Hot path ускорен, PostgreSQL разгружен.

---

### Этап 3. Простой UI

#### Что сделать
- сделать страницу создания ссылки
- сделать страницу списка последних ссылок
- сделать страницу деталей и статистики
- добавить copy short URL
- добавить базовую UX-обратную связь

#### Результат
Сервисом можно пользоваться через браузер без Postman.

---

### Этап 4. Kafka event pipeline

#### Что сделать
- поднять Kafka в docker-compose
- определить schema события `link.visited`
- добавить producer в `shortener`
- добавить analytics consumer
- продумать retry и DLQ

#### Результат
Клики обрабатываются асинхронно.

---

### Этап 5. ClickHouse analytics

#### Что сделать
- поднять ClickHouse
- создать таблицу raw click events
- писать события из analytics в ClickHouse
- добавить API для базовой статистики

#### Результат
Есть аналитика по short code, времени и referrer.

---

### Этап 6. Elasticsearch + Kibana

#### Что сделать
- поднять Elasticsearch и Kibana
- настроить shipping логов
- добавить ключевые поля в логи:
  - `request_id`
  - `short_code`
  - `status_code`
  - `latency_ms`
  - `error`

#### Результат
Логи доступны для поиска и расследования.

---

### Этап 7. Prometheus + Grafana

#### Что сделать
- поднять Prometheus и Grafana
- добавить технические и бизнес-метрики
- собрать дашборды по latency, errors, cache hit ratio и Kafka

#### Результат
Система наблюдаема через метрики.

---

### Этап 8. Load and stress testing

#### Что сделать
- добавить `k6` сценарии для redirect-heavy и mixed GET/POST нагрузки
- настроить real-time экспорт метрик `k6`
- добавить метрики PostgreSQL и Redis через exporter-ы
- собрать Grafana dashboards, где видно и нагрузку, и реакцию приложения
- подготовить spike и saturation сценарии для controlled environment

#### Результат
Можно в реальном времени наблюдать, как сервис и инфраструктура ведут себя под высокой
параллельной нагрузкой.

---

### Этап 9. Build and local delivery automation

#### Что сделать
- настроить lint
- настроить unit tests
- настроить integration tests
- настроить build
- настроить docker image build
- добавить quality gates
- добавить локальные automation-команды для verify/build/deploy
- добавить запуск миграций

#### Результат
Проект можно воспроизводимо собирать, проверять и локально выкатывать.

---

### Этап 10. Local Kubernetes runtime with kind + Helm

#### Что сделать
- поднять локальный `kind`-кластер
- подготовить Helm chart для `shortener` и `analytics`
- настроить ingress/service/probes/config для локального k8s-окружения
- описать локальную процедуру `helm upgrade --install`
- проверить rollout и recovery в локальной Kubernetes-модели

#### Результат
Есть локальный production-like runtime в Kubernetes без необходимости внешнего облака.

---

### Этап 11. HTTP transport variants

#### Что сделать
- после стабилизации основного функционала добавить альтернативные entrypoint-ы
- реализовать `api-http-chi` и/или `api-http-gin`
- переиспользовать service/repository/domain слой без fork бизнес-логики
- сравнить ergonomics, middleware model и удобство тестирования
- задокументировать выводы и оставить один canonical baseline для основной разработки

#### Результат
Есть практическое сравнение разных HTTP transport вариантов без смешивания framework-логики в
основном MVP path.

---

## 12. Общие Definition of Done для задач Codex

- Любой новый этап должен быть доведен до рабочего состояния и запускаться локально.
- Код должен быть совместим с Go 1.25.
- Handler / service / repository должны быть отделены и не смешивать слои.
- Нужны осмысленные тесты для критических сценариев.
- Нужна документация по локальному запуску и конфигам.
- Любая новая инфраструктурная зависимость должна быть встроена в docker-compose и описана в README.
- Observability не должна полностью откладываться на потом: health, logs и базовые metrics должны присутствовать с ранних этапов.

---

## 13. Kafka event schema

Минимальное событие `link.visited`:

```json
{
  "event_type": "link.visited",
  "event_version": 1,
  "event_id": "uuid",
  "occurred_at": "2026-04-01T10:15:00Z",
  "link_id": "uuid",
  "short_code": "abc123",
  "ip": "1.2.3.4",
  "user_agent": "Mozilla/5.0 ...",
  "referer": "https://example.com",
  "request_id": "req-123"
}
```

Требования:
- схема должна быть версионируемой
- redirect path не должен ждать аналитические consumers
- analytics должен быть готов к at-least-once processing
- стоит предусмотреть DLQ

---

## 14. Минимальные метрики

### Технические
- `http_requests_total`
- `http_request_duration_seconds`
- `db_query_duration_seconds`
- `redis_requests_total`
- `redis_cache_hits_total`
- `redis_cache_misses_total`
- `kafka_publish_total`
- `kafka_publish_errors_total`
- `kafka_consumer_messages_total`
- `postgres_connections`
- `postgres_query_duration_seconds`
- `redis_connected_clients`
- `k6_http_req_duration`
- `k6_http_reqs`
- `k6_vus`
- `container_cpu_usage_seconds_total`
- `container_memory_working_set_bytes`

### Бизнесовые
- `links_created_total`
- `redirects_total`
- `redirect_not_found_total`
- `redirect_expired_total`
- `redirect_disabled_total`
- `redirect_max_clicks_reached_total`
- `link_create_collision_retries_total`
- `rate_limit_hits_total`

---

## 15. Готовый master prompt для Codex

```text
Нужно спроектировать и поэтапно реализовать production-like pet project: URL shortener с простым UI, аналитикой, observability и локальной automation/deploy model.

Стек:
- Go 1.25
- PostgreSQL
- Redis
- Kafka
- ClickHouse
- Elasticsearch + Kibana
- Prometheus + Grafana
- k6
- Docker Compose
- kind
- Helm

Цели:
- создание коротких ссылок
- быстрый redirect по short code
- web UI для создания, просмотра и базовой статистики
- асинхронная обработка click events через Kafka
- аналитика в ClickHouse
- structured logs в Elasticsearch
- метрики в Prometheus
- нагрузочные сценарии и performance visibility через k6 + Grafana
- production-like локальное окружение без обязательного внешнего облака

Архитектурные требования:
- начать как модульный монолит
- выделить два приложения: shortener и analytics
- использовать Go 1.25
- придерживаться чистого разделения handler / service / repository
- structured JSON logging
- graceful shutdown
- health / readiness / metrics endpoints
- Docker-first подход
- использовать Docker Compose как основной ранний runtime
- позже добавить локальный Kubernetes через kind + Helm
- не усложнять систему микросервисами на старте
- не включать Kubernetes в MVP

Нужно:
1. описать архитектуру проекта
2. предложить структуру директорий
3. определить доменные сущности
4. определить HTTP API
5. определить Kafka event schema
6. предложить docker-compose stack
7. разбить реализацию на этапы с Definition of Done
8. отдельно учесть простой UI
9. отдельно учесть нагрузочные сценарии и performance visibility
10. отдельно учесть локальный kind + Helm deployment path

При реализации:
- prefer small methods
- avoid deep nesting
- keep redirect path simple and fast
- analytics must not break redirect
- each phase should end in usable state
- code examples must target Go 1.25
```

---

## 16. Рекомендуемый порядок проработки Codex

1. Сначала попросить Codex сделать архитектурную декомпозицию и структуру проекта.
2. Потом дать отдельную задачу на bootstrap и core backend.
3. После этого поэтапно добавлять Redis, UI, Kafka, ClickHouse, logging stack, metrics, load
   testing, local delivery automation и local Kubernetes runtime.
4. Не просить Codex реализовать все сразу одним заходом.
5. Лучше дробить работу на самостоятельные фазы с отдельным Definition of Done.
6. Эксперименты с альтернативными HTTP framework-ами делать только после стабилизации основного
   stdlib entrypoint.
7. Нагрузочные и saturation-сценарии делать только против локального или контролируемого
   окружения проекта.
8. Внешний cloud/VPS deploy не считать обязательной частью текущего roadmap.

---

## 17. Финальный roadmap

**Core backend → Redis cache → UI → Kafka events → ClickHouse analytics → Logs stack → Metrics stack → load/stress testing → local delivery automation → local kind + Helm runtime → optional HTTP transport variants**
