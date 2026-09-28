# Данные и инфраструктура — Go-2

Владеет импортом, редактированием данных, хранением, HTTP API и запуском приложения. Потребители — Go-4 и Frontend. [Общие типы](common.md), [HTTP API](frontend.md).

## Реализует

`DataStore` для Go-4 и HTTP API для Frontend.

```go
type DataStore interface {
    GetSnapshot(ctx context.Context, scenarioID string, revision int64) (Snapshot, error)
    GetPlan(ctx context.Context, planID string) (Plan, error)
    CommitPlan(ctx context.Context, input PlanCommit) (Plan, error)
}
```

### HTTP API, которое реализует Go-2

Префикс `/api/v1`. Тела и ошибки полностью определены в [контракте Frontend](frontend.md); краткий перечень для реализации:

| Метод | Вход → выход |
|---|---|
| `GET /demo-datasets` | — → список наборов |
| `POST /scenarios` | `demo_dataset_id` → `201 ScenarioView` |
| `POST /scenarios/import` | CSV, `region_id`, `date` → `201 ScenarioView` |
| `GET /scenarios/{id}` | необязательный `revision` → `200 ScenarioView` |
| `PATCH /scenarios/{id}/engineers/{engineer_id}` | `expected_revision`, изменяемые поля, включая `equipment_stock`; `reserve` вычисляется при принятии первого плана → `200 ScenarioView`; при блокирующем состоянии `409 EVENT_CONFLICT` |
| `POST /scenarios/{id}/proposals` | `request_id`, `snapshot_revision`, `expected_current_plan_id`, `solve_mode?`, `event?` → `201 Proposal` с вариантами; повтор того же запроса → `200 Proposal` |
| `GET /scenarios/{id}/proposals/current` | — → `200 Proposal` или `null` |
| `POST /proposals/{id}/accept` | `request_id`, `option_key` → `200 Plan` |
| `POST /scenarios/{id}/plans` | `request_id`, `snapshot_revision`, `expected_current_plan_id`, `solve_mode?: baseline \| optimized` → `202 {run_id}`; если состояние не блокирует Build |
| `POST /plans/{id}/events` | `request_id`, `snapshot_revision`, `event`, `solve_mode?: baseline \| optimized` → `202 {run_id}` |
| `GET /runs/{id}` | — → `200 Run` |
| `GET /plans/{id}` | — → `200 Plan` |

## Использует от других разработчиков

Зависимости передаются при создании модуля; в самостоятельной разработке вместо реализаций подключаются заглушки с теми же методами.

### Go-4: расчёт и перепланирование

```go
type PlanService interface {
    Build(ctx context.Context, input BuildPlanRequest) (PlanResult, error)
    Replan(ctx context.Context, input ReplanRequest) (PlanResult, error)
    BuildOptions(ctx context.Context, input BuildPlanRequest) ([]PlanOption, error)
    ReplanOptions(ctx context.Context, input ReplanRequest) ([]PlanOption, error)
}
```

| Тип | Что передаёт / получает Go-2 |
|---|---|
| `BuildPlanRequest` | `request_id`, `scenario_id`, `snapshot_revision: int64`, `expected_current_plan_id: string?`, `solve_mode?: baseline \| optimized` |
| `ReplanRequest` | `request_id`, `scenario_id`, `snapshot_revision: int64`, `base_plan_id`, `event: Event`, `solve_mode?: baseline \| optimized`; для новой заявки или изменения статуса данные/время берутся из события |
| `PlanResult` | `draft: PlanDraft`, `target_snapshot: Snapshot`, `applied_event: Event?` — типы из [common.md](common.md) |

`scenario_id` берётся из URL либо сохранённого базового плана. `snapshot_revision` — ревизия **до** команды. Go-4 возвращает проверенный результат без сохранения и без нового `plan.id`. Go-2 передаёт его в собственный `CommitPlan`; только после успешного сохранения выставляет `Run.status=succeeded`.

`BuildOptions` строит два начальных варианта (`strict`, `late_emergency`). `ReplanOptions` для аварии строит `strict`, `late_emergency`, `reserve`; для остальных событий — `strict`, `reserve` и отдельный выбор `original` для сохранения исходного расписания. Каждый вариант содержит полный `PlanResult`. Сервис сохраняет набор вариантов как предложение, привязанное к исходной ревизии и текущему плану. Само предложение не меняет сценарий. Принятие `option_key` сохраняет ровно выбранный результат, а при событии — также новый снимок и событие, в одной транзакции. Повтор принятия с тем же `request_id` и вариантом возвращает тот же план; устаревшая ревизия или текущий план дают `STALE_VERSION`.

Ошибка `Build` / `Replan` завершает запуск как `failed`, не меняя текущие данные и план. Возможны `INVALID_INPUT`, `NOT_FOUND`, `EVENT_CONFLICT`, `GEO_UNAVAILABLE`, `INVALID_PLAN`, `COMPUTATION_FAILED`; конфликт ревизии или текущего плана при сохранении — `STALE_VERSION`. Если `in_progress`-заявка имеет `expected_end_at=null` или `expected_end_at <= event.occurred_at`, Go-4 добавляет в план `Issue` с кодом `EXECUTION_STATE_REQUIRED`, сохраняет факт текущей работы и не назначает этой бригаде будущие заявки до поступления подтверждения или актуальной оценки. Это предупреждение плана, не ошибка HTTP/запуска. Полное поведение зависимости: [plans.md](plans.md).

### Go-3: координаты при импорте

```go
// Потребительская часть GeoService: Go-2 нужен только этот метод.
type Geocoder interface {
    Geocode(ctx context.Context, input GeocodeRequest) (GeocodeResult, error)
}
```

| Тип | Поля |
|---|---|
| `GeocodeRequest` | `region_id`, `locations: LocationInput[]` |
| `LocationInput` | `id`, `address`, `point: Point?` |
| `GeocodeResult` | `items: [{location_id, location: Location?, issue: Issue?}]` |
| `Location` | `id`, `address`, `point: {lat: float64, lon: float64}` |

На каждый входной `location.id` приходит элемент с тем же `location_id`: координаты либо `issue`. Сопоставление — по ID, не по позиции в массиве. Ошибка отдельного адреса попадает в `Snapshot.issues`; техническая ошибка всего вызова `GEO_UNAVAILABLE` не превращается в ошибки распознавания всех адресов. `INVALID_INPUT` означает некорректный запрос. Геокодирование и HTTP-провайдер реализует Go-3: [geo.md](geo.md).

Frontend не предоставляет Go-2 вызываемый сервис. Go-2 обслуживает его HTTP-запросы. Прямой зависимости от `Planner` Go-1 нет.

## Чтение и подготовка

- `GetSnapshot` читает точную неизменяемую ревизию; отсутствующая ревизия — `NOT_FOUND`. Подмена на актуальную версию запрещена.
- `GetPlan` возвращает полный сохранённый план, включая геометрию, метрики, изменения и `equipment_remaining`.
- Go-4 записывает в `PlanDraft` поле `equipment_remaining: map[engineer_id]map[Equipment]int64`, рассчитывая его для всех инженеров как `equipment_stock` за вычетом потребностей всех фактически начатых заявок, без вычета будущих резервов. Go-2 сохраняет это поле в `Plan` и возвращает без повторного расчёта. До первого плана исходной выдачей служит `Engineer.equipment_stock` в снимке.
- Импорт создаёт сценарий с ревизией 1 и `current_plan_id=null`.
- Подготовка включает нормативы, часовые пояса, стабильные ID, порядок исходных строк и вызов `GeoService.Geocode` для адресов без координат. Все эксперименты используют только синтетические данные; контрольные данные исключены.
- ВК/Beekeeper и HD/HelpDesk — разные классификаторы одной заявки. Go-2 сопоставляет их одному `WorkType`: `emergency`, `connection`, `repair` или `additional`; это не отдельные виды работ. Приоритет фиксирован: авария → подключение → ремонт/дозаказ. `priority` нормализуется в `urgent` для аварии и `normal` для остальных.
- `service_sec` — время работы без дороги. Для аварии оно равно 4800 секундам: норматив 100 минут включает 20 минут дороги, которые заменяются фактическим временем из матрицы. Минимум в 20 минут не применяется.
- Оборудование: `router` и `tv_box`. `Engineer.equipment_stock` — запас, выданный в офисе на начало дня; `Order.equipment_required` — количество, нужное для заявки. Запас доступен бригаде весь день и учитывается при назначении. Резервный инженер имеет `reserve=true`, `available=false`; по принятию варианта с резервом задействованные инженеры получают `available=true`, `reserve=false`; остальные сохраняют `available=false`, `reserve=true`.
- `region_id` идентифицирует независимый сценарий/район обслуживания и не выводится автоматически из административной колонки датасета. Время источника московское; интерфейс форматирует его в `timezone` региона.
- Недостающие компетенции, транспорт и запасы оборудования моделируются на синтетических данных отдельно для каждого района; у района свой набор бригад.
- Ошибки отдельных строк возвращаются в `Snapshot.issues`. Нечитаемый файл или отсутствие корректного офиса не создают готовый сценарий.
- PATCH инженера проверяет `expected_revision`, создаёт следующую ревизию и возвращает `ScenarioView`. Он поддерживает `equipment_stock`; при блокирующем состоянии PATCH запрещён (`EVENT_CONFLICT`). Состояние в течение дня меняется событиями.
- Build и PATCH блокируются, если текущий план создан событием (`base_plan_id != null`), любая заявка имеет `execution != null` либо `as_of` плана позже начала местного дня. Первое событие, в том числе ровно в 00:00, блокирует дальнейшие Build/PATCH. Начальная отменённая заявка без `execution` не блокирует Build. После блокировки изменения выполняются через `Replan`.
- Событие `order_status_changed` в рабочем API подтверждает начало (`in_progress`) или завершение (`completed`) работы; `occurred_at` — фактическое время перехода. Состояние `en_route` определяется по действующему расписанию. Для `in_progress` `expected_end_at` допускается опустить или повторно обновлять. Переход в `completed` подтверждает только диспетчер по информации бригады, а не истечение времени плана; завершённые заявки сохраняются отдельно и исключаются из дальнейшего распределения.
- Событие `ordinary_order_added` принимает payload `{order: Order, location?: LocationInput}`; внутри `order` обязательны `work_type=connection|repair|additional`, `priority=normal`, `status=active`, `execution=null`, `received_at=event.occurred_at` и остальные поля `Order`. Полная схема события — в [common.md](common.md). В обоих рассчитываемых вариантах обычная заявка вставляется только в свободный интервал; при доказанном отсутствии щели используется `NO_FEASIBLE_INSERTION`. При конфликте устаревшего плана сохраняются Issue `EXISTING_PLAN_CONFLICT` и `UnassignedOrder.reason_code=NOT_ASSIGNED_BY_SOLVER`, без нового HTTP-кода. `urgent_order_added` остаётся отдельным типом, оба события принимаются через тот же endpoint.
- `order_cancelled` принимает причину `client_refusal` или `cannot_perform`; отменить можно до начала работы, включая заявку в пути; начатую, завершённую и ранее отменённую — нельзя. Событие, обновлённый снимок и план сохраняются атомарно через тот же endpoint `/plans/{id}/events`.

## Атомарное сохранение

`PlanCommit`:

| Поле | Назначение |
|---|---|
| `request_id` | Ключ повторного выполнения команды |
| `expected_revision` | Ревизия исходных данных до расчёта |
| `expected_current_plan_id: string?`, `solve_mode?: baseline \| optimized` | Текущий план до расчёта; `null` для первого |
| `result: PlanResult` | Проверенный результат Go-4 с целевым снимком и применённым событием |

Порядок `CommitPlan`:

1. Проверить идемпотентность: уже завершённая та же команда возвращает прежний план; конфликт содержимого отклоняется.
2. Проверить ожидаемые ревизию и текущий план. При несовпадении вернуть `STALE_VERSION`.
3. При обычном расчёте сохранить план для существующего снимка. При событии атомарно сохранить событие, новый снимок с ревизией `expected_revision + 1` и план для него. Событие, изменение статуса заявки и новый план не фиксируются раздельно.
4. Присвоить `plan.id`, обновить `current_plan_id`, завершить запуск с этим ID. Частично сохранённый результат недопустим.

Снимок обычного расчёта должен совпадать с исходным; у события `draft.snapshot_revision` должен совпадать с новой ревизией. Предыдущие планы и снимки не изменяются.

## Запуски расчётов

Go-2 принимает команду через HTTP, атомарно регистрирует её тело и ключи идемпотентности, выдаёт `run_id`, вызывает `PlanService` Go-4 и затем `CommitPlan`. Повтор при `queued/running` возвращает существующий запуск без повторного вызова `PlanService`; параллельные одинаковые запросы не создают два запуска.

`Run = {id, scenario_id, status, plan_id?, error?}`, где `status = queued | running | succeeded | failed`. `succeeded` выставляется только после сохранения плана. При ошибке событие не меняет сценарий. Повтор того же запроса возвращает прежний запуск; для новой попытки после ошибки нужен новый `request_id`, а для события — также новый `event.id`.

## Начало самостоятельной разработки

1. Реализовать `DataStore` и HTTP по таблице выше, используя типы из [common.md](common.md).
2. Подключить заглушки `PlanService` и `Geocoder`: проверенный `PlanResult`, координаты по ID и оговорённые ошибки.
3. На примере из [backend_flow.json](examples/backend_flow.json) проверить цепочку: команда → запуск → `PlanService` → `CommitPlan` → сохранённый план. HTTP-ответы для Frontend — в [frontend_flow.json](examples/frontend_flow.json).
4. Проверить повтор команды и конфликт версии: один запуск / результат при повторе; отсутствие частичного сохранения при ошибке.

### Импорт состава инженеров

`POST /api/v1/scenarios/{scenario_id}/engineers/import` — multipart-поля `file` (CSV с `;`) и `expected_revision` (положительное целое). Колонки: `id;skills;transport;shift_start;shift_end;available;router;tv_box`, необязательная девятая и последняя — `reserve`. Старый формат из восьми колонок сохраняет `reserve=false`. Навыки разделяются запятыми; смены — `HH:mm` в часовом поясе и дате сценария. `reserve=true` отклоняется: резерв вычисляется по первому принятому плану. Ответ `200 ScenarioView` с новой ревизией и `current_plan_id=null`. Старые snapshots и планы сохраняются. Невалидный CSV отклоняется целиком, конфликт ревизии/начатое исполнение/активный расчёт возвращает `409`. Обычный импорт заказов оставляет `engineers=[]` и `ENGINEERS_REQUIRED`; генерация состава допускается только для демосценариев.

Импортированные заявки с нераспознанными координатами сохраняются в `Snapshot.unlocated_orders`: `{order, address, message}`. Они доступны диспетчеру, но не передаются в решатель и на карту. Для прежних импортов API восстанавливает этот список из сохранённых строк и ошибок геокодирования.
