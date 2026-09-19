# Общие типы и правила контрактов

Контракты первой версии для [четырёх backend-модулей](../work_breakdown.md) и frontend. Внутреннее взаимодействие — Go-интерфейсы, внешнее — HTTP/JSON. Сигнатуры и таблицы ниже описывают договорённости, а не готовый Go-пакет.

**Статус после Q&A 2 (19.09.2026):** перепланирование сохраняется в основе. Текущие DTO и JSON-примеры ещё не покрывают подтверждение завершения диспетчером, приоритеты типов и запас оборудования. Известные изменения и вопросы следующей редакции перечислены в [анализе Q&A 2](../qa_2/QA_2_impact.md). Модельное завершение по времени плана не является фактом закрытия заявки.

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
- `Transport = car | walk`; `Priority = normal | urgent`; навыки — согласованные строковые коды из данных, сравнение по точному совпадению.
- `Window = {start, end}`. Начало работы входит в окно включительно; завершение не позже конца смены. Интервал выполнения работы — `[start_at, end_at)`.
- Суффикс `?` в таблицах означает необязательное значение. В HTTP отсутствующее значение передаётся как `null`, списки без элементов — как `[]`.

При переносе схем в Go необязательные одиночные значения представляются указателями, массивы — срезами, `map[Transport]` — картой. Поля JSON сохраняют написание из таблиц через теги `json`. Значения времени используют `time.Time`, как указано выше.

## Исходные данные

| Тип | Поля |
|---|---|
| `Location` | `id`, `address`, `point: Point` |
| `LocationInput` | `id`, `address`, `point: Point?`; без координат требуется геокодирование |
| `Order` | `id`, `location_id`, `required_skills: string[]`, `required_transport: Transport?`, `window: Window`, `service_sec: int64`, `priority: Priority`, `source_order: int64`, `status: active \| cancelled` |
| `Engineer` | `id`, `skills: string[]`, `transport: Transport`, `shift: Window`, `available: bool`, `source_order: int64` |
| `Issue` | `source_row: int?`, `entity_id: string?`, `field: string?`, `code`, `message` |
| `Snapshot` | `scenario_id`, `revision: int64`, `region_id`, `date`, `timezone`, `office_location_id`, `locations: Location[]`, `orders: Order[]`, `engineers: Engineer[]`, `issues: Issue[]` |
| `ScenarioView` | `snapshot: Snapshot`, `current_plan_id: string?` |

Снимок неизменяем. `current_plan_id` — изменяемый указатель сценария, не часть снимка. Импортированные исторические статусы не становятся автоматически текущим состоянием дня. Неподготовленные записи остаются в `issues`; в планировщик передаются только корректные активные заявки. Порядок базового алгоритма задаётся `source_order`, при равенстве — ID.

## Расписание и результат

| Тип | Поля |
|---|---|
| `EngineerState` | `engineer_id`, `start_location_id`, `available_from` |
| `Visit` | `order_id`, `arrival_at`, `start_at`, `end_at` |
| `Leg` | `id`, `from_location_id`, `to_location_id`, `start_at`, `end_at`, `distance_m: int64`, `geo_context_id`, `geometry: Point[]?` |
| `Route` | `engineer_id`, `start_location_id`, `start_at`, `visits: Visit[]`, `legs: Leg[]` |
| `UnassignedOrder` | `order_id`, `reason_code`, `message` |
| `Metrics` | `assigned_count`, `unassigned_count`, `used_engineer_count`, `total_distance_m`, `per_engineer: [{engineer_id, distance_m}]` |
| `Assignment` | `engineer_id`, `sequence: int`, `arrival_at`, `start_at`, `end_at`; `sequence` начинается с 0 |
| `PlanChange` | `order_id`, `before: Assignment?`, `after: Assignment?`, `reason: reassigned \| rescheduled \| assigned \| unassigned \| cancelled` |

Ожидание перед работой определяется интервалом от `arrival_at` до `start_at`. Поездки и работы одного инженера не пересекаются. План содержит маршруты задействованных инженеров, включая имеющих только сохранённую часть дня.

`PlanDraft` содержит `scenario_id`, `snapshot_revision`, `base_plan_id?`, `as_of`, `routes`, `unassigned`, `cancelled_order_ids`, `issues`, `metrics`, `baseline_metrics?`, `changes: PlanChange[]`, `termination: completed | time_limit`.

`PlanResult = {draft: PlanDraft, target_snapshot: Snapshot, applied_event: Event?}` — внутренний результат Go-4 для сохранения. `Plan` — сохранённый `PlanDraft` с добавленным `id`. В HTTP поля `PlanDraft` располагаются непосредственно в `Plan`; служебный `target_snapshot` в него не включается.

Метрики относятся ко всему дню: размещённые заявки включают сохранённые выполненные и начатые работы, пробег — уже пройденные и будущие участки. `assigned_count` не означает фактическое выполнение. Ошибочные и отменённые заявки не входят в `unassigned_count`.

## События

`Event = {id, occurred_at, type, payload}`. Тип определяет единственный допустимый payload:

| `type` | `payload` |
|---|---|
| `urgent_order_added` | `{order: Order, location: LocationInput?}`; при новом адресе `location.id = order.location_id`, приоритет `urgent`, статус `active` |
| `order_cancelled` | `{order_id}` |
| `engineer_unavailable` | `{engineer_id}` |

ID события и новой заявки задаёт вызывающая сторона. `source_order` новой заявки назначает Go-4 после существующих заявок; входное значение не используется. После нормализации `applied_event` содержит разрешённые координаты.

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

- [backend_flow.json](examples/backend_flow.json): один снимок, матрица, запросы и ответы модулей, сохранение плана и отмена будущей заявки.
- [frontend_flow.json](examples/frontend_flow.json): ответы HTTP для того же сценария, состояния запуска, план, событие и конфликт версии.
- [Назначение ключей примеров](examples/README.md): какой объект возвращает каждая заглушка.

Это фиксированные данные для проверки соединения модулей, не результаты дорожного провайдера или измерения качества алгоритма. `time_limit_ms=1000` — значение только для примера. Для базового режима тот же `solve_request` используется с `mode=baseline`; в этом примере с одной заявкой результат совпадает.

Дополнения D1–D3 пока не меняют эти контракты. Их поля согласовываются при переходе к соответствующей функции.
