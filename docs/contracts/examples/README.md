# Данные для заглушек

Основа обоих JSON-файлов синхронизирована: один инженер, одна заявка, первый расчёт и отдельная отмена в 08:00. Дополнительные ветки показывают подтверждение статусов и поступление аварии. Верхние ключи — имена fixtures, а не оболочка реального API. Общие типы и ошибки — в [common.md](../common.md).

## Backend

Из [backend_flow.json](backend_flow.json) можно возвращать объекты напрямую в заглушках; JSON служит записью структур, внутренние вызовы остаются Go-интерфейсами.

| Кто использует | Запрос → ответ заглушки |
|---|---|
| Go-2 | `geocode_request` → `geocode_result`; `build_request` → `plan_result`; `replan_request` → `replan_result` |
| Go-1 | `solve_request` → `solve_result`; матрица уже включена во вход |
| Go-3 | `geocode_request` → `geocode_result`; `matrix_request` → `matrix`; `routes_request` → `routes_geometry`; `position_request` → `position_result` |
| Go-4 | `GetSnapshot("scenario-1", 1)` → `snapshot`; `GetPlan("plan-1")` → `saved_plan`; ответы GeoService и Planner — из пар выше; каждый `execution_status_flow.status_steps[].request` → соответствующий `response` |
| Go-2: хранение | `plan_commit` → `saved_plan`; новый ID присваивается при сохранении |

`plan_result` и `replan_result` — ожидаемые результаты Go-4. В `solve_result` геометрия ещё `null`; Go-4 добавляет её из `routes_geometry`. Для `mode=baseline` в примере используется тот же результат. `position_request` демонстрирует середину поездки отдельно от события отмены.

## Frontend

Из [frontend_flow.json](frontend_flow.json) mock API возвращает выбранный объект по маршруту:

| Запрос | Ключ ответа |
|---|---|
| `GET /api/v1/demo-datasets` | `demo_datasets` |
| `POST /api/v1/scenarios` с `demo_dataset_id=demo-1` | `scenario_before` |
| `POST /api/v1/scenarios/import` с одной корректной и одной ошибочной строкой | `import_with_issue` |
| `POST /api/v1/scenarios/scenario-1/plans`, тело `plan_request` | `accepted` |
| `GET /api/v1/runs/run-1` | сначала `run_running`, затем `run` |
| `GET /api/v1/plans/plan-1` | `plan` |
| `GET /api/v1/scenarios/scenario-1?revision=1` | `scenario` |
| `POST /api/v1/plans/plan-1/events`, тело `event_request` | `202 {"run_id":"run-2"}` |
| `GET /api/v1/runs/run-2` | `event_run` |
| `GET /api/v1/plans/plan-2` | `event_plan` |
| `GET /api/v1/scenarios/scenario-1?revision=2` | `scenario_after_event` |
| `execution_status_flow.status_steps[i]` | POST `/api/v1/plans/{plan_id}/events` с `request`; `accepted`, `run`, `plan`, `scenario` — mock-ответы по мере выполнения |
| Конфликт при отправке устаревшей команды | HTTP `409`, `stale_version_error` |

После события `GET` сценария без `revision` возвращает `scenario_after_event`. При чтении старой ревизии снимок остаётся первым; `current_plan_id` отражает текущий план сценария (`plan-2`), поскольку этот указатель не входит в снимок.

Поездка занимает 900 секунд и 1200 метров; ожидание перед работой — с 09:15 до 10:00 местного времени. Обычный план имеет `as_of` в начале местного дня и заполненные `baseline_metrics`. Отмена в 08:00 создаёт ревизию 2; до поездки ещё нет сохранённой выполненной части дня. Временные отметки ответов записаны в UTC.

## QA-сценарии для статусов и оборудования

В обоих JSON есть `execution_status_flow.status_steps`: последовательность из четырёх `order_status_changed` запросов и результатов перепланирования. Это оболочка примеров; реальные HTTP-команды идут через `POST /plans/{id}/events` и возвращают `202 {run_id}`. Каждый статус создаёт новую ревизию снимка и новый план; ветка начинается от `plan-1` и не связана с отдельной отменой в `plan-2`. После `in_progress` фактическое начало списывает роутер один раз, поэтому `equipment_remaining["eng-1"].router` равен 0 и в `completed`; завершение повторно запас не уменьшает. `completed_order_ids` содержит `order-1`, а `metrics.completed_count=1` только после подтверждённого `completed`. Закрытая заявка не передаётся в следующий вызов `Solve`.

`incoming_emergency_example` показывает новую аварию (в backend fixture это полный `ReplanRequest`, во frontend — HTTP body и `plan_id`): `service_sec=4800` — 80 минут работы без дороги, `received_at` совпадает с `occurred_at`; фактическая поездка добавляется отдельно. У инженера один роутер, которого недостаточно для одновременного назначения этой аварии и запланированного ремонта. С учётом приоритета авария получает ресурс, ремонт получает `NO_MATCHING_EQUIPMENT`. `equipment_remaining` показывает физический остаток после уже начатых работ; будущие резервации в него не вычитаются.
