# Общие типы и правила контрактов

Контракты первой версии для [четырёх backend-модулей](../work_breakdown.md) и frontend. Внутреннее взаимодействие — Go-интерфейсы, внешнее — HTTP/JSON. Сигнатуры и таблицы ниже описывают договорённости, а не готовый Go-пакет.

Редакция по [спецификации с QA](../specification.md): перепланирование, фактические статусы и минимальный учёт оборудования входят в основу. Формулы приоритета, расход оборудования и переходы статусов ниже — конкретная модель реализации уточнённых требований.

| Документ | Владелец |
|---|---|
| [Планировщик](planner.md) | Go-1 |
| [Данные и хранение](data.md) | Go-2 |
| [Геоданные](geo.md) | Go-3 |
| [Управление планами](plans.md) | Go-4 |
| [HTTP API](frontend.md) | Go-2 и Frontend |

## Кто реализует и кто использует

| Разработчик | Реализует | Использует от других |
|---|---|---|
| Go-1 | `Planner.Solve` | Прямых вызовов нет; получает матрицу Go-3 во входе от Go-4 |
| Go-2 | `DataStore`, HTTP API | `PlanService.Build/Replan` Go-4; `GeoService.Geocode` Go-3 |
| Go-3 | `GeoService` | Прямых вызовов других Go-модулей нет |
| Go-4 | `PlanService.Build/Replan` | `DataStore.GetSnapshot/GetPlan` Go-2; `GeoService` Go-3; `Planner.Solve` Go-1 |
| Frontend | Рабочее место диспетчера, HTTP-клиент | HTTP API Go-2 |

В каждом файле указаны собственные методы, вызываемые зависимости, входы, выходы и примеры для заглушек. Общие типы ниже используются всеми частями в одном пакете контрактов; копии структур с несовместимыми полями в отдельных модулях не создаются. Владельцы интерфейсов поддерживают их определения, потребители используют те же типы.

`Geocoder` в контракте Go-2 и `PlanDataReader` в контракте Go-4 — узкие потребительские интерфейсы существующих `GeoService` и `DataStore`. Они не добавляют новых методов: реализация владельца удовлетворяет им автоматически. Зависимости передаются модулю при создании, поэтому настоящую реализацию можно заменить заглушкой.

## Единицы и значения

- ID — непустая строка. ID существующих заявок и инженеров сохраняются между версиями.
- Внутри Go время — `time.Time`; в JSON — RFC3339 с обязательным смещением и точностью до секунды. Backend нормализует время в UTC, интерфейс использует IANA timezone региона. `date` — местная дата `YYYY-MM-DD`.
- Длительности — целые секунды; расстояния — целые метры; лимит вычисления — миллисекунды. Длительность работы положительна, время поездки и расстояние неотрицательны.
- `Point = {lat: float64, lon: float64}`. Геометрия — упорядоченный массив `Point`.
- `Transport = car | walk` — выбранные профили первой версии. `WorkType = emergency | connection | repair | additional`; приоритет типов: авария → подключение → ремонт / дозаказ. `Priority = normal | urgent`: `urgent` только для `emergency`, для остальных — `normal`; несовместимая пара отклоняется.
- Навыки — согласованные строковые коды; все `required_skills` должны быть у инженера. `Equipment = router | tv_box`; количество — целое неотрицательное число, отсутствующий ключ карты означает 0, неизвестный код недопустим. Навык, транспорт и оборудование — независимые ограничения.
- `Window = {start, end}`. Начало работы входит в окно включительно; завершение не позже конца смены. Интервал выполнения работы — `[start_at, end_at)`.
- Суффикс `?` в таблицах означает необязательное значение. В HTTP отсутствующее значение передаётся как `null`, списки без элементов — как `[]`.

При переносе схем в Go необязательные одиночные значения представляются указателями, массивы — срезами, `map[Transport]` — картой. Поля JSON сохраняют написание из таблиц через теги `json`. Значения времени используют `time.Time`, как указано выше.

## Исходные данные

| Тип | Поля |
|---|---|
| `Location` | `id`, `address`, `point: Point` |
| `LocationInput` | `id`, `address`, `point: Point?`; без координат требуется геокодирование |
| `Order` | `id`, `location_id`, `work_type: WorkType`, `required_skills: string[]`, `required_transport: Transport?`, `window: Window`, `received_at`, `service_sec: int64`, `priority: Priority`, `equipment_required: map[Equipment]int64`, `source_order: int64`, `status: OrderStatus`, `execution: OrderExecution?` |
| `OrderStatus` | `active \| sent \| en_route \| in_progress \| completed \| cancelled` |
| `OrderExecution` | `engineer_id`, `departed_at?`, `started_at?`, `finished_at?`, `expected_end_at?` |
| `Engineer` | `id`, `skills: string[]`, `transport: Transport`, `shift: Window`, `available: bool`, `equipment_stock: map[Equipment]int64`, `source_order: int64` |
| `Issue` | `source_row: int?`, `entity_id: string?`, `field: string?`, `code`, `message` |
| `Snapshot` | `scenario_id`, `revision: int64`, `region_id`, `date`, `timezone`, `office_location_id`, `locations: Location[]`, `orders: Order[]`, `engineers: Engineer[]`, `issues: Issue[]` |
| `ScenarioView` | `snapshot: Snapshot`, `current_plan_id: string?` |

Снимок неизменяем. `current_plan_id` — изменяемый указатель сценария, не часть снимка. Все расчёты и примеры используют только синтетические данные. `region_id` обозначает независимый район обслуживания выбранного сценария: офис, заявки и инженеры не объединяются с другим районом. Это не автоматическое соответствие одноимённому административному столбцу исходного файла. Одна бригада — один инженер. Исходное время по Москве преобразуется в UTC с сохранением местной даты сценария.

Go-2 сопоставляет классификаторы ВК / Beekeeper и HD / HelpDesk одному `work_type`; отсутствие однозначного соответствия — `Issue`. Недостающие навыки, транспорт и запасы задаются воспроизводимой моделью. Импортированные исторические статусы не считаются фактами текущего дня. Неподготовленные записи остаются в `issues`. Порядок baseline задаётся `source_order`, затем ID.

`received_at` — поступление заявки; работа и выезд к ней не начинаются раньше него. Для исходных заявок без такого поля подготовленный набор задаёт начало местного дня и явно фиксирует это допущение. У новой аварии `received_at = event.occurred_at`.

`service_sec` содержит **только время без дороги**. Если норматив включает 20 минут дороги, Go-2 вычитает 1200 секунд ровно один раз. Для аварии `service_sec = 6000 − 1200 = 4800`. Go-1 добавляет фактическое `TravelCell.duration_sec`, без нижней границы 1200 секунд. Дорога 5 минут даёт 85 минут дороги и работы, 30 минут — 110. Это не отдельный жёсткий дедлайн в 100 минут от поступления; ожидание клиентского окна учитывается отдельно.

`active` — ещё не отправлена исполнителю; `sent` — отправлена; `en_route` — бригада в пути; `in_progress` — работа начата; `completed` — диспетчер подтвердил завершение; `cancelled` — отменена. Наличие назначения в плане само по себе не меняет фактический статус. У `active` поле `execution=null`; у `sent` и следующих состояний оно содержит исполнителя. `started_at`/`finished_at` — подтверждённые времена, `expected_end_at` — только оценка для начатой работы. Завершение по часам плана не подтверждается автоматически.

`equipment_stock` — выдача в офисе на весь день. Принятая модель расхода: полная потребность заявки списывается при первом подтверждённом начале работы. Завершение не списывает её повторно; отмена начатой работы не возвращает оборудование автоматически. Запланированные, но не начатые работы только резервируют остаток внутри расчёта. Пополнение в течение дня и полный складской учёт в основу не входят.

## Расписание и результат

| Тип | Поля |
|---|---|
| `EngineerState` | `engineer_id`, `start_location_id`, `available_from`, `equipment_available: map[Equipment]int64` |
| `Visit` | `order_id`, `arrival_at`, `start_at`, `end_at` |
| `Leg` | `id`, `from_location_id`, `to_location_id`, `start_at`, `end_at`, `distance_m: int64`, `geo_context_id`, `geometry: Point[]?` |
| `Route` | `engineer_id`, `start_location_id`, `start_at`, `visits: Visit[]`, `legs: Leg[]` |
| `UnassignedOrder` | `order_id`, `reason_code`, `message` |
| `Metrics` | `assigned_count`, `completed_count`, `unassigned_count`, `used_engineer_count`, `total_distance_m`, `per_engineer: [{engineer_id, distance_m}]` |
| `Assignment` | `engineer_id`, `sequence: int`, `arrival_at`, `start_at`, `end_at`; `sequence` начинается с 0 |
| `PlanChange` | `order_id`, `before: Assignment?`, `after: Assignment?`, `reason: reassigned \| rescheduled \| assigned \| unassigned \| cancelled \| status_changed` |

Ожидание перед работой определяется интервалом от `arrival_at` до `start_at`. Поездки и работы одного инженера не пересекаются. План содержит маршруты задействованных инженеров, включая имеющих только сохранённую часть дня.

`PlanDraft` содержит `scenario_id`, `snapshot_revision`, `base_plan_id?`, `as_of`, `routes`, `unassigned`, `cancelled_order_ids`, `completed_order_ids`, `equipment_remaining: map[string]map[Equipment]int64`, `issues`, `metrics`, `baseline_metrics?`, `changes: PlanChange[]`, `termination: completed | time_limit`.

`PlanResult = {draft: PlanDraft, target_snapshot: Snapshot, applied_event: Event?}` — внутренний результат Go-4 для сохранения. `Plan` — сохранённый `PlanDraft` с добавленным `id`. В HTTP поля `PlanDraft` располагаются непосредственно в `Plan`; служебный `target_snapshot` в него не включается.

Метрики относятся ко всему дню. `assigned_count` — число уникальных заявок в маршрутах, кроме отменённых; включает начатые и подтверждённо выполненные работы. `completed_count = len(completed_order_ids)` — только подтверждённое выполнение. Ошибочные, выполненные и отменённые заявки не входят в `unassigned_count`. Пробег включает сохранённые пройденные и будущие участки; использованным считается инженер с поездкой либо работой, в том числе по позднее отменённой заявке. История такой отменённой работы может остаться в маршруте, но будущего назначения у неё нет.

Go-4 передаёт `equipment_available = equipment_stock − потребности всех заявок этого инженера с execution.started_at != null`, включая завершённые и отменённые. Go-1 резервирует из этого остатка только будущие назначения. Отрицательный остаток — некорректные данные, а не разрешение на пополнение.

`equipment_remaining` в плане содержит этот фактический остаток по ID всех инженеров снимка, в том числе недоступных. Будущие резервации его не уменьшают. Поле рассчитывает Go-4 для отображения диспетчеру; Frontend не воспроизводит правила списания.

## События

`Event = {id, occurred_at, type, payload}`. Тип определяет единственный допустимый payload:

| `type` | `payload` |
|---|---|
| `urgent_order_added` | `{order: Order, location: LocationInput?}`; при новом адресе `location.id = order.location_id`; `work_type=emergency`, `priority=urgent`, `status=active`, `execution=null`, `service_sec=4800`, `received_at=occurred_at` |
| `order_cancelled` | `{order_id, reason: client_refusal \| cannot_perform}` |
| `engineer_unavailable` | `{engineer_id}` |
| `order_status_changed` | `{order_id, status: sent \| en_route \| in_progress \| completed, engineer_id, expected_end_at?}`; диспетчер подтверждает факт по информации бригады |

ID события и новой заявки задаёт вызывающая сторона. `source_order` новой заявки назначает Go-4 после существующих заявок; входное значение не используется. После нормализации `applied_event` содержит разрешённые координаты.

Переходы: `active → sent → en_route → in_progress → completed`; `sent → in_progress` допустим без отдельной поездки. При `sent` задаётся исполнитель текущего назначения; `en_route` устанавливает `departed_at`, `in_progress` — `started_at`, `completed` — `finished_at` по `occurred_at`. Повтор `in_progress → in_progress` обновляет только оценку окончания. Переданный `expected_end_at` должен быть позже события; для других статусов он `null`. Отмена возможна из любого незавершённого состояния; для начатой работы сохраняет `started_at` и устанавливает `finished_at`. Завершённая или уже отменённая заявка неизменяема. Детали проверки и сохранения фактов — [plans.md](plans.md).

## Версии и ошибки

- Изменение исходных данных создаёт следующую ревизию снимка. Обычный расчёт использует существующую ревизию; успешное событие создаёт следующую.
- Сохранение проверяет одновременно ожидаемую текущую ревизию и ожидаемый `current_plan_id`. Несовпадение — `STALE_VERSION`.
- Идемпотентность расчётов: `(scenario_id, request_id)` и для событий `(scenario_id, event.id)`. Повтор того же содержимого возвращает тот же запуск/результат; другое содержимое с тем же ключом — `IDEMPOTENCY_CONFLICT`.
- Ошибка: `{code, message, details}`. Общие коды: `INVALID_INPUT`, `NOT_FOUND`, `STALE_VERSION`, `EVENT_CONFLICT`, `IDEMPOTENCY_CONFLICT`, `GEO_UNAVAILABLE`, `INVALID_PLAN`, `COMPUTATION_FAILED`.
- Неназначенные заявки — часть допустимого результата. Достижение лимита времени при наличии допустимого плана обозначается `termination=time_limit`, а не технической ошибкой.

Для проверки кода ошибки через `errors.As` все модули возвращают общий тип:

```go
type ContractError struct {
    Code    string         `json:"code"`
    Message string         `json:"message"`
    Details map[string]any `json:"details"`
}

func (e *ContractError) Error() string { return e.Code + ": " + e.Message }
```

Пустые `details` сериализуются как `{}`. Обёртка ошибки должна сохранять исходную ошибку для `errors.As`; HTTP-статусы определены в [frontend.md](frontend.md).

## Примеры для начала реализации

- [backend_flow.json](examples/backend_flow.json): снимок, матрица, запросы и ответы модулей, сохранение и отмена; отдельные ветки подтверждения статусов и поступления аварии с ограниченным оборудованием.
- [frontend_flow.json](examples/frontend_flow.json): HTTP-ответы для тех же сценариев, состояния запуска, планы, фактическое завершение и конфликт версии.
- [Назначение ключей примеров](examples/README.md): какой объект возвращает каждая заглушка.

Это фиксированные синтетические данные для проверки соединения модулей, не результаты дорожного провайдера или измерения качества алгоритма. `time_limit_ms=1000` — значение только для примера. Для базового режима тот же `solve_request` используется с `mode=baseline`; в примере с одной заявкой результат совпадает.

Дополнения D1–D2 (перерывы и ручные закрепления) пока не меняют контракты. Минимальный учёт оборудования уже включён в общие модели.
