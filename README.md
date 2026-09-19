# Планирование выездных инженеров

Материалы проекта: функциональный состав, распределение работы между четырьмя Go-разработчиками и Frontend, контракты модулей и примеры для независимой разработки.

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

## Презентация

[PowerPoint](docs/presentation/solution.pptx) · [PDF](docs/presentation/solution.pdf) · [Исходники и пересборка](docs/presentation/README.md).

## Исходные материалы

- [Техническое задание](docs/specification.pdf).
- [Транскрипция Q&A](docs/qa_1/QA_1_transcrib.md).
- [Ответы экспертов Q&A 2](docs/qa_2/QA_2_answers.md) и [влияние на план](docs/qa_2/QA_2_impact.md). Перепланирование остаётся в основе F1–F7.
- [Нормативы](docs/Нормативы.xlsx).
- [Наборы данных](datasets/original/).

Исходные аудио/видео и промежуточные изображения слайдов исключены из Git.
