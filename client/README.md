## Быстрый старт (локально)

Для совместного запуска UI, backend и PostgreSQL используйте `docker compose up --build -d --wait` из корня репозитория после настройки корневого `.env`. См. [инструкцию интеграции](../docs/integration.md). При локальной разработке UI по умолчанию работает с реальным API на localhost:8080; mock включается явно через `VITE_API_MODE=mock`.

```bash
cp .env.example .env
pnpm install
pnpm dev               # http://localhost:3000
```

По умолчанию `VITE_API_MODE=mock` — данные через MSW, backend не нужен.

Live API:

```bash
# .env
VITE_API_MODE=live
VITE_API_URL=/api/v1
```

Vite проксирует `/api` на `http://localhost:8080`.

## Docker Compose

Из этой папки (`client/`):

```bash
# Dev с hot reload → http://localhost:3000
docker compose up client

# Статическая preview-сборка → http://localhost:8080
docker compose --profile preview up client-preview --build
```

Опционально: `VITE_YANDEX_MAPS_KEY` в `.env` рядом с `docker-compose.yml`
или через export.

## Скрипты

| Команда | Назначение |
|---|---|
| `pnpm dev` | Dev-сервер |
| `pnpm build` | Production build |
| `pnpm preview` | Локальный preview сборки |
| `pnpm lint` | ESLint |
| `pnpm lint:styles` | Stylelint (SCSS modules) |

## Стек

- Vite 8 + React 19 + TypeScript
- React Router, TanStack Query, Zustand
- react-hook-form + zod, shadcn/Radix, Tailwind
- Luxon, MSW (`VITE_API_MODE=mock`), Яндекс.Карты
- FSD page-first: `app` → `pages` → `features` → `shared`

## Архитектура (FSD page-first)

```
src/
  app/          # router, providers, shell, MSW bootstrap
  pages/        # экраны: scenarioSetup, dispatcherWorkspace
  features/     # пользовательские действия (глагол в имени)
  shared/       # api, ui, lib, config
```

**Pages** держат экран: layout UI, Zustand selection, view-model, композицию
features. Кросс-импорты между pages запрещены.

**Features** — атомарные user-actions:

| Feature | Действие |
|---|---|
| `openDemoRegion` | Открыть демо-район → сценарий |
| `importOrders` | CSV → сценарий |
| `buildPlan` | Собрать день (run + poll) |
| `applyPlanEvent` | Авария / статус / отмена / недоступность |
| `updateCrew` | Патч бригады до старта дня |

UI-блоки и page-specific lib остаются в `pages/*`, пока второй экран их не
переиспользует (без преждевременных `widgets` / `entities`).

**Shared** — HTTP-клиент, контракты, MSW, примитивы UI, map adapter.

Дисплей-only: фронт не считает маршруты, метрики и остатки оборудования —
только рендерит ответ API / моков.
