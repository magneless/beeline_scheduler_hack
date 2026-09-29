# Интеграция модулей

Объединённый проект хранит весь Go backend в `backend/`; в него входят PostgreSQL, geo, plans, VROOM planner, HTTP API и UI integration.

## Запуск

В корне скопируйте `.env.example` в `.env` и задайте `POSTGRES_PASSWORD`. Затем:

```sh
docker compose up --build -d --wait
```

UI: http://localhost:3000. API: http://localhost:8080/api/v1. PostgreSQL: localhost:5432. Данные сохраняются в Docker volume `postgres_data`. Остановка с сохранением данных: `docker compose down`.

Режим по умолчанию — `DEPENDENCY_MODE=integrated`, `GEO_PROVIDER=osm`, `SOLVER_MODE=optimized`. Для запуска без VROOM задайте `SOLVER_MODE=baseline`. Сервер соединяет storage/HTTP/worker, GeoService, PlanService и planner. Миграции применяются автоматически. UI обращается к backend через nginx; браузеру не нужен отдельный CORS.

На стартовой странице доступны три синтетических района и контрактный пример. Откройте район, рассчитайте план, выберите заявку или инженера. Изменения создают новую версию плана и snapshot; предыдущие версии сохраняются. Кнопка «Новая заявка» открывает форму для аварийной или обычной заявки: существующий или новый адрес, навыки, транспорт, окно и оборудование. Обычная заявка вставляется в свободный интервал без переноса существующих визитов. Если план уже не совместим с фактическим состоянием, заявка сохраняется без назначения с объяснением.

## Совместимость и ограничения

- Общие модели находятся в `backend/internal/contracts`; пакет `backend/contracts` экспортирует совместимые алиасы. Payload событий остаётся JSON согласно HTTP-контракту.
- Go-4 поддерживает пять видов событий, фактические статусы, остатки оборудования и сохранение выполненной части маршрута. Плановое время само по себе не завершает работы. Без ожидаемого окончания текущей работы дальнейшие назначения инженеру приостанавливаются.
- PostgreSQL атомарно сохраняет snapshot, план, событие и результат run. Повтор запроса с тем же request_id не создаёт второй результат.
- Оптимизированный planner подключён через VROOM и используется по умолчанию. `baseline` остаётся fallback-режимом через `SOLVER_MODE=baseline`.
- По умолчанию геоданные реальные: Photon и отдельные OSRM car/foot. Матрица строится пакетами, ответы кэшируются в volume `geo_cache`. Для офлайн-проверок явно задайте `GEO_PROVIDER=demo`; такие планы содержат `DEMO_GEO`. Ошибка OSM не переключает расчёт на прямые линии. [Настройки и ограничения](geo-provider.md).
- UI по умолчанию показывает интерактивную карту OpenStreetMap через Leaflet без API-ключа. `VITE_YANDEX_MAPS_KEY` опционально переключает фон на Яндекс.Карты при сборке и не меняет источник дорожных маршрутов backend.
- `DEPENDENCY_MODE=stub` оставлен для изолированных проверок Go-2. `frontend/docker-compose.yml` запускает отдельный mock UI; для совместной работы используйте compose в корне.

## Разработка и проверки

Для backend: Go 1.27, PostgreSQL 17, переменная `DATABASE_URL`, затем `cd backend` и `FIXTURE_PATH=../docs/contracts/examples/backend_flow.json DATASET_DIR=../datasets/original DATABASE_URL=... SOLVER_MODE=baseline go run ./cmd/server`. Для UI: Node 22.12+, pnpm, `cd frontend`, `pnpm install --frozen-lockfile --ignore-scripts`, `pnpm dev`. Vite проксирует API на localhost:8080; live включён по умолчанию.

```sh
cd backend
go vet ./...
go test -race ./...
go build ./cmd/server
cd ../frontend
pnpm lint
pnpm lint:styles
pnpm build
```

Для PostgreSQL-тестов используйте `docker compose --profile test run --rm --build backend-test`; compose передаёт `TEST_DATABASE_URL`, пароль PostgreSQL берётся из `.env`. Каждый тест использует отдельную схему. Сквозные тесты проверяют реальные модули, импорт, пять видов событий, историю и списание оборудования. На Windows race-проверка требует C-компилятор; её можно выполнить в Linux-контейнере Go.

## Реальный состав бригад

Обычный импорт заявок создаёт пустой состав. В панели «Бригады» загрузите CSV с разделителем `;`:

```csv
id;skills;transport;shift_start;shift_end;available;router;tv_box
crew-1;repair,connection;car;10:00;22:00;true;4;2
crew-2;repair;walk;10:00;22:00;true;1;0
```

Обязательная смена — **10:00–22:00** по дате и часовому поясу сценария ([QA из чата 4](qa_chat_4/QA_processed.md)); в CSV указываются именно эти границы. `skills` — навыки через запятую, `transport` — `car` или `walk`, `available` — `true` или `false`; оборудование — целые неотрицательные количества. ID должны быть уникальны. Весь файл проверяется до сохранения, пустой или частично ошибочный состав не применяется.

Импорт заменяет состав до начала событий, увеличивает ревизию и сбрасывает текущий план; предыдущий план остаётся в истории. Во время расчёта и после событий замена запрещена. Синтетический состав создаётся только при явном выборе демосценария и помечается `DEMO_ENGINEERS`.
