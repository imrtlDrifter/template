# Проверка observability

Проверено в окружении `tripgo-lab-02` 6 ноября 2026 года.

- [Jaeger](jaeger.png): HTTP-запрос создания и вложенные операции PostgreSQL.
- [Grafana](grafana.png): шесть метрик с реальными данными.
- [Проверка корреляции](verification.json): `trace_id` и `span_id` бизнес-лога совпадают с HTTP-спаном; внутри трейса есть INSERT в `trips`.
- Повтор создания с тем же ключом вернул 200, счётчик созданных поездок остался равен 1.
- При недоступном `http://127.0.0.1:1` создание вернуло 201, завершение — 200. Обе операции заняли суммарно см. verification.json. Основной коллектор при проверке не останавливался.

Dashboard для импорта: [observability/dashboard.json](../../observability/dashboard.json). В Grafana: Dashboards → New → Import, загрузить JSON и выбрать Prometheus.

На dashboard показаны накопленные p95 HTTP и средняя длительность SQL с запуска процесса. Для анализа нагрузки по временному окну можно использовать `rate(..._bucket[5m])`, `rate(..._sum[5m])` и `rate(..._count[5m])`.
