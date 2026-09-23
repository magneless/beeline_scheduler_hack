# Интеграция модулей

Объединённый проект хранит весь Go backend в `backend/`; в него входят PostgreSQL, geo, plans, OR-Tools planner, HTTP API и UI integration.

## Запуск

В корне скопируйте `.env.example` в `.env` и задайте `POSTGRES_PASSWORD`. Затем:

```sh
docker compose up --build -d --wait
```

UI: http://localhost:3000. API: http://localhost:8080/api/v1. PostgreSQL: localhost:5432. Данные сохраняются в Docker volume `postgres_data`. Остановка с сохранением данных: `docker compose down`.

Режим по умолчанию — `DEPENDENCY_MODE=integrated-demo`, `SOLVER_MODE=optimized`. Для запуска без OR-Tools задайте `SOLVER_MODE=baseline`. Сервер соединяет storage/HTTP/worker, GeoService, PlanService и planner. Миграции применяются автоматически. UI обращается к backend через nginx; браузеру не нужен отдельный CORS.

На стартовой странице доступны три синтетических района и контрактный пример. Откройте район, рассчитайте план, выберите заявку или инженера. Изменения создают новую версию плана и snapshot; предыдущие версии сохраняются. Кнопка аварии добавляет демонстрационную заявку по адресу выбранной заявки (или первой в сценарии).

## Совместимость и ограничения

- Общие модели находятся в `backend/internal/contracts`; пакет `backend/contracts` экспортирует совместимые алиасы. Payload событий остаётся JSON согласно HTTP-контракту.
- Go-4 поддерживает четыре вида событий, фактические статусы, остатки оборудования и сохранение выполненной части маршрута. Плановое время само по себе не завершает работы. Без ожидаемого окончания текущей работы дальнейшие назначения инженеру приостанавливаются.
- PostgreSQL атомарно сохраняет snapshot, план, событие и результат run. Повтор запроса с тем же request_id не создаёт второй результат.
- Оптимизированный planner подключён через OR-Tools и используется по умолчанию. `baseline` остаётся fallback-режимом через `SOLVER_MODE=baseline`.
- Геоданные остаются демонстрационными: координаты детерминированно синтезируются около Москвы, поездки идут по прямой. Это не дорожные маршруты и не реальные результаты геокодирования; планы содержат `DEMO_GEO`.
- Без `VITE_YANDEX_MAPS_KEY` UI показывает схематическую карту. Ключ карты передаётся при сборке и не меняет демонстрационный источник геоданных backend.
- `DEPENDENCY_MODE=stub` оставлен для изолированных проверок Go-2. `client/docker-compose.yml` запускает отдельный mock UI; для совместной работы используйте compose в корне.

## Разработка и проверки

Для backend: Go 1.27, PostgreSQL 17, переменная `DATABASE_URL`, затем `cd backend` и `FIXTURE_PATH=../docs/contracts/examples/backend_flow.json DATASET_DIR=../datasets/original DATABASE_URL=... SOLVER_MODE=baseline go run ./cmd/server`. Для UI: Node 22.12+, pnpm, `cd client`, `pnpm install --frozen-lockfile --ignore-scripts`, `pnpm dev`. Vite проксирует API на localhost:8080; live включён по умолчанию.

```sh
cd backend
go vet ./...
go test -race ./...
go build ./cmd/server
cd ../client
pnpm lint
pnpm lint:styles
pnpm build
```

Для PostgreSQL-тестов используйте `docker compose --profile test run --rm --build backend-test`; compose передаёт `TEST_DATABASE_URL`, пароль PostgreSQL берётся из `.env`. Каждый тест использует отдельную схему. Сквозные тесты проверяют реальные модули, импорт, четыре вида событий, историю и списание оборудования. На Windows race-проверка требует C-компилятор; её можно выполнить в Linux-контейнере Go.
