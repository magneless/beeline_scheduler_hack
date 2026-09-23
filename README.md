# LCT Beeline - Планирование выездных инженеров

**Веб-сервис, который распределяет заявки между инженерами,
строит маршруты, показывает их на карте и умеет перестраивать план при изменениях.**

Описание требований: [Спецификация](docs/specification.md)

Команда:
  - 4 backend-разработчика.
  - 1 frontend-разработчик.

Планировщик использует синтетические данные и поддерживает перепланирование. Работы приоритизируются так: авария → подключение → ремонт / дозаказ. Минимальный учёт оборудования входит в основу решения.

## Начало работы

Объединённый проект содержит весь Go backend в `backend/`: PostgreSQL, геоданные, планы, OR-Tools planner и HTTP API работают через один модуль. Запуск и ограничения описаны в [документации интеграции](docs/integration.md). В корне проекта скопируйте `.env.example` в `.env`, задайте пароль БД и выполните `docker compose up --build -d --wait`. Интерфейс доступен на http://localhost:3000.

Геоданные по-прежнему демонстрационные: `DEMO_GEO` использует синтетические координаты и поездки по прямой. По умолчанию включён `SOLVER_MODE=optimized`; для запуска без OR-Tools используйте `SOLVER_MODE=baseline`. Изолированный режим Go-2 описан в [документации backend](docs/go2_backend.md).

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

## Планировщик

Реализация — [`backend/internal/planner`](backend/internal/planner/planner.go), общие Go-типы — [`backend/internal/contracts`](backend/internal/contracts/planner.go). Требуется Go 1.27.

`planner.New()` реализует `contracts.Planner` и поддерживает режимы `baseline`, `optimized` и `insert_only`. Сборка, тесты и описание алгоритмов — в [backend/README.md](backend/README.md).

Запуск тестов в Docker из корня репозитория:

```bash
docker compose --profile test run --rm --build backend-test
```

## Исходные материалы

- [Исходное ТЗ (PDF)](docs/specification.pdf) · [спецификация с уточнениями QA](docs/specification.md).
- [Q&A 1: исходные ответы](docs/qa_1/QA_raw.md) · [сводка](docs/qa_1/QA_processed.md).
- [Q&A 2: исходные ответы](docs/qa_2/QA_raw.md) · [сводка](docs/qa_2/QA_processed.md).
- [Q&A из чата: исходные ответы](docs/qa_chat/QA_raw.md) · [сводка](docs/qa_chat/QA_processed.md).
- [Q&A из чата 2: исходные ответы](docs/qa_chat_2/QA_raw.md) · [сводка](docs/qa_chat_2/QA_processed.md).
- [Нормативы](docs/Нормативы.xlsx).
- [Наборы данных](datasets/original/).
