# LCT Beeline - Планирование выездных инженеров

**Веб-сервис, который распределяет заявки между инженерами,
строит маршруты, показывает их на карте и умеет перестраивать план при изменениях.**

Описание требований: [Спецификация](docs/specification.md)

Команда:
  - 4 backend-разработчика.
  - 1 frontend-разработчик.

Планировщик использует синтетические данные и поддерживает перепланирование. Работы приоритизируются так: авария → подключение → ремонт / дозаказ. Минимальный учёт оборудования входит в основу решения.

## Начало работы

1. [Функциональный состав](docs/functional_scope.md).
2. [Распределение работы](docs/work_breakdown.md).
3. [Общие типы и связи модулей](docs/contracts/common.md).
4. [Примеры для заглушек](docs/contracts/examples/README.md).

| Исполнитель | Контракт |
|---|---|
| Go-1 | [Планировщик](docs/contracts/planner.md) |
| Go-2 | [Данные и инфраструктура](docs/contracts/data.md) |
| Go-3 | [Геоданные](docs/contracts/geo.md) |
| Go-4 | [Управление планами и перепланирование](docs/contracts/plans.md) |
| Frontend | [Рабочее место диспетчера и HTTP API](docs/contracts/frontend.md) |

Каждый контракт описывает реализуемые и используемые интерфейсы, входы, выходы и ошибки.

## Baseline планировщика

Реализация — [`internal/planner`](internal/planner/baseline.go), общие Go-типы — [`internal/contracts`](internal/contracts/planner.go). Требуется Go 1.22 или новее; внешних зависимостей нет.

`planner.NewBaseline()` реализует `contracts.Planner`. В `Solve` нужно передать подготовленный `contracts.SolveRequest` с `Mode: contracts.SolveModeBaseline`. Для примера из `docs/contracts/examples/backend_flow.json` используется поле `solve_request` с заменой `mode` на `baseline`.

Алгоритм перебирает заявки и инженеров по `source_order`, затем ID, добавляя заявку в конец маршрута первого допустимого инженера. Проверяются навыки, транспорт, остатки оборудования, поступление заявки, клиентские окна, смены и направленная матрица. Входные данные не изменяются. Для перепланирования используются переданные стартовые состояния инженеров.

При исчерпании `time_limit_ms` возвращается частичный допустимый план с `termination=time_limit`; все оставшиеся заявки перечислены в `unassigned`. Отмена или дедлайн родительского `context` возвращают `ctx.Err()` без пригодного для сохранения результата. Режим `optimized` пока не реализован и отклоняется с `INVALID_INPUT`.

Проверки:

```bash
go test -race ./...
go vet ./...
```

## Исходные материалы

- [Исходное ТЗ (PDF)](docs/specification.pdf) · [спецификация с уточнениями QA](docs/specification.md).
- [Q&A 1: исходные ответы](docs/qa_1/QA_raw.md) · [сводка](docs/qa_1/QA_processed.md).
- [Q&A 2: исходные ответы](docs/qa_2/QA_raw.md) · [сводка](docs/qa_2/QA_processed.md).
- [Q&A из чата: исходные ответы](docs/qa_chat/QA_raw.md) · [сводка](docs/qa_chat/QA_processed.md).
- [Нормативы](docs/Нормативы.xlsx).
- [Наборы данных](datasets/original/).
