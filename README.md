# Trip Service

Создание, получение и завершение поездок. Go 1.26.1+, Gin, PostgreSQL, golang-migrate, OpenTelemetry.

## Запуск

```bash
tripgoctl cluster start
tripgoctl environment start
tripgoctl connect
make migrate
make run
```

`tripgoctl connect` создаёт `.env` и печатает адреса PostgreSQL, Jaeger, Grafana и Prometheus. Makefile сначала читает `.env.example`, затем `.env`.

## API

| Метод | Путь | Назначение |
|---|---|---|
| POST | `/api/v1/trips` | Создать поездку |
| GET | `/api/v1/trips/{tripId}` | Получить поездку |
| POST | `/api/v1/trips/{tripId}/finish` | Завершить поездку |
| GET | `/health` | Проверить процесс |
| GET | `/ready` | Проверить PostgreSQL |

Ошибки — `application/problem+json`. Контракт: [OpenAPI](contracts/openapi/trip-service.openapi.yaml).

`Idempotency-Key` — UUID, срок хранения 24 часа. Первый запрос возвращает 201, повтор с тем же телом — ту же поездку и 200, другое тело — 409.

## Архитектура

```text
handlers/v1/trip/{create,get,finish} → service/trip/{create,get,finish} → domain/models
                                              ↑
                                  repositories/postgres
container → конфигурация, инфраструктура, репозитории, сервисы, HTTP
```

- У каждого действия свой пакет и `contract.go`. Нужные интерфейсы объявляет потребитель: сервис — для репозиториев и транзакций, хендлер — для сервиса.
- `domain/models` содержит модель, `Trip.Finish` и цепочку валидации. `domainerrors` содержит обычные Go-ошибки и `ValidationError`; HTTP и pgx в домене отсутствуют.
- Репозитории возвращают исходные ошибки. Сервисы распознают ошибки БД через переданный классификатор и добавляют контекст внутренних операций через `%w`.
- `handlers/v1/domainerror` переводит доменные ошибки в Problem. Хендлеры передают ошибки в обёртку и `httpres.HandleDomainError`.
- `container.New` создаёт пул, репозитории, сервисы, хендлеры и сервер. `main` читает конфигурацию и управляет запуском и остановкой.
- `infra` содержит PostgreSQL, transaction manager, сервер и телеметрию.
- Транзакция хранится в контексте; вложенный `Do` её переиспользует. Создание пишет поездку и историю атомарно. Завершение блокирует строку через `SELECT FOR UPDATE`, вызывает `Trip.Finish` и пишет историю. Уникальный индекс запрещает две активные поездки водителя.

Сохранены названия `service` вместо `usecase`, `domain/models` и отдельный `domainerrors`: правила зависимостей те же, раскладку проверяет depguard.

## Проверки

```bash
make lint              # обязательные линтеры, depguard, gofumpt/gci
make test              # unit/httptest, -race, итоговое покрытие
make test-integration  # реальный PostgreSQL из .env, build tag integration
make generate-mocks    # go:generate + gomock, mocks_test.go рядом с контрактом
make generate-api      # генерация из OpenAPI
make mutation          # gremlins, отчёты reports/mutation/after
make build
```

Для интеграционных тестов сначала выполнить `make migrate`. Каждый тест использует свои UUID и удаляет свои данные. Проверяются конкурентное завершение и rollback двух таблиц.

Покрытие domain + service: **97,5%**. Доменные правила и три сервиса: **100%**. [Мутации: 51 убито, 0 выжило; отчёты до/после](reports/mutation/README.md).

## Логи и телеметрия

Логгер передаётся зависимостью. stdout - JSON, уровень задаёт `LOG_LEVEL`; HTTP-записи содержат `service`, `trace_id`, `span_id`, метод, шаблон маршрута, статус и `duration_ms`. Для 2xx и 4xx используется Info, для 5xx — Error. Тела запросов и ответов не логируются.

Провайдеры OpenTelemetry передаются явно. Поддерживаются OTLP HTTP/gRPC и propagator Jaeger/W3C/B3/Baggage. SQL инструментируется на пуле через otelpgx, атрибуты включают операцию, таблицу и тип ошибки.

Порядок: Recover → Tracing → Logging → Metrics → маршруты. Recovery возвращает 500 Problem и пишет stack trace. Бизнес-логи и счётчики создаются после commit; повтор по ключу их не дублирует. HTTP-логи и бизнес-логи содержат `trace_id` и `span_id`.

Настройки: `OTEL_SERVICE_NAME`, `OTEL_EXPORTER_OTLP_ENDPOINT`, `OTEL_EXPORTER_OTLP_PROTOCOL` (`http/protobuf` или `grpc`), `OTEL_TRACES_SAMPLER`, `OTEL_RESOURCE_ATTRIBUTES`. Пустой endpoint отключает экспорт. Недоступный коллектор не блокирует HTTP.

Метрики: `http.server.request.duration`, `http.server.active_requests`, `db.client.operation.duration`, `tripgo.trips.created`, `tripgo.trips.completed`, `tripgo.trips.active`. Gauge активных поездок читает текущее состояние PostgreSQL и не сбрасывается при перезапуске. В `http.route` используется шаблон с `:tripId`, чтобы число временных рядов не росло с количеством поездок.

Версия сервиса задаётся `SERVICE_VERSION`. `OTEL_RESOURCE_ATTRIBUTES` содержит namespace и окружение. Таймаут экспорта, интервал метрик и задержка отправки спанов задаются `OTEL_EXPORTER_OTLP_TIMEOUT`, `OTEL_METRIC_EXPORT_INTERVAL`, `OTEL_BSP_SCHEDULE_DELAY` в миллисекундах. Все параметры перечислены в `.env.example`.

Elastic необязателен: `ELASTIC_ENDPOINT`, `ELASTIC_LOGIN`, `ELASTIC_PASSWORD`, `ELASTIC_INDEX`, `ELASTIC_TIMEOUT`. Если endpoint пуст, отправка выключена.

[Скриншоты и проверка окружения](reports/observability/README.md). [Текст для PR](reports/PR.md).

## Docker

```bash
docker build -t trip-service:lab2 .
docker run --rm --network host --env-file .env.example --env-file .env trip-service:lab2 /app/migrator up
docker run --rm --network host --env-file .env.example --env-file .env trip-service:lab2
```
