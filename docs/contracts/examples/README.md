# Данные для заглушек

Оба JSON-файла описывают один пример: один инженер, одна заявка, первый расчёт и отмена заявки до начала поездки. Верхние ключи — имена примеров, а не оболочка реального API. Общие типы и ошибки — в [common.md](../common.md).

## Backend

Из [backend_flow.json](backend_flow.json) можно возвращать объекты напрямую в заглушках; JSON служит записью структур, внутренние вызовы остаются Go-интерфейсами.

| Кто использует | Запрос → ответ заглушки |
|---|---|
| Go-2 | `geocode_request` → `geocode_result`; `build_request` → `plan_result`; `replan_request` → `replan_result` |
| Go-1 | `solve_request` → `solve_result`; матрица уже включена во вход |
| Go-3 | `geocode_request` → `geocode_result`; `matrix_request` → `matrix`; `routes_request` → `routes_geometry`; `position_request` → `position_result` |
| Go-4 | `GetSnapshot("scenario-1", 1)` → `snapshot`; `GetPlan("plan-1")` → `saved_plan`; ответы GeoService и Planner — из пар выше |
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
| Конфликт при отправке устаревшей команды | HTTP `409`, `stale_version_error` |

После события `GET` сценария без `revision` возвращает `scenario_after_event`. При чтении старой ревизии снимок остаётся первым; `current_plan_id` отражает текущий план сценария (`plan-2`), поскольку этот указатель не входит в снимок.

Поездка занимает 900 секунд и 1200 метров; ожидание перед работой — с 09:15 до 10:00 местного времени. Обычный план имеет `as_of` в начале местного дня и заполненные `baseline_metrics`. Отмена в 08:00 создаёт ревизию 2; до поездки ещё нет сохранённой выполненной части дня. Временные отметки ответов записаны в UTC.
