import { type OrderStatus, type WorkType } from 'shared/api/types/contracts';

export const regionLabel: Record<string, string> = {
    north: 'Север',
    east: 'Восток',
    southeast: 'Юго-восток',
    southcentral: 'Югоцентр',
    'region-1': 'Пример контракта',
    'south-east': 'Юго-восток',
    south: 'Юг',
    west: 'Запад',
};

export const engineerName: Record<string, string> = {
    sokolov: 'Соколов',
    melnikova: 'Мельникова',
    popov: 'Попов',
    ivanov: 'Иванов',
    kuznetsov: 'Кузнецов',
    smirnova: 'Смирнова',
    vasilev: 'Васильев',
    novikova: 'Новикова',
    fedorov: 'Фёдоров',
    morozov: 'Морозов',
    volkova: 'Волкова',
    alekseev: 'Алексеев',
    lebedev: 'Лебедев',
    semenova: 'Семёнова',
    egorov: 'Егоров',
    pavlov: 'Павлов',
    kozlova: 'Козлова',
    stepanov: 'Степанов',
    nikolaev: 'Николаев',
    orlova: 'Орлова',
    andreev: 'Андреев',
    makarov: 'Макаров',
    nikitina: 'Никитина',
    zakharov: 'Захаров',
};

export const workTypeLabel: Record<WorkType, string> = {
    emergency: 'Авария',
    connection: 'Подключение',
    repair: 'Ремонт',
    additional: 'Дозаказ',
};

export const statusLabel: Record<OrderStatus, string> = {
    active: 'Активна',
    sent: 'Отправлено',
    en_route: 'В пути',
    in_progress: 'Выполняется',
    completed: 'Завершено',
    cancelled: 'Отменена',
};

export const transportLabel = {
    car: 'Авто',
    walk: 'Пешком',
} as const;

export const skillLabel: Record<string, string> = {
    emergency: 'Авария',
    connection: 'Подключение',
    repair: 'Ремонт',
    additional: 'Дозаказ',
    optical: 'Оптика',
    local: 'Локальные',
};

export const equipmentLabel = {
    router: 'роутер',
    tv_box: 'приставка',
} as const;

export const priorityLabel = {
    normal: 'Обычная',
    urgent: 'Срочная',
} as const;

export const cancelReasonLabel = {
    client_refusal: 'Отказ клиента',
    cannot_perform: 'Невозможно выполнить',
} as const;

export const reasonCodeLabel: Record<string, string> = {
    NO_MATCHING_SKILL: 'Нет навыка',
    NO_MATCHING_TRANSPORT: 'Нет транспорта',
    NO_MATCHING_EQUIPMENT: 'Нет оборудования',
    NO_AVAILABLE_ENGINEER: 'Нет свободной бригады',
    NO_REACHABLE_ROUTE: 'Недосягаемый адрес',
    NO_FEASIBLE_SLOT: 'Нет слота в окне',
    NO_FEASIBLE_INSERTION: 'Нет свободного интервала',
    NOT_ASSIGNED_BY_SOLVER: 'Алгоритм не нашёл место',
};

export const changeReasonLabel: Record<string, string> = {
    reassigned: 'переназначена',
    rescheduled: 'сдвинуто время',
    assigned: 'назначена',
    unassigned: 'выпала из плана',
    cancelled: 'отменена',
    status_changed: 'изменён статус',
};

export const issueCodeLabel: Record<string, string> = {
    INVALID_INPUT: 'Ошибка строки',
    EXISTING_PLAN_CONFLICT: 'Требуется уточнение старого плана',
    ENGINEERS_REQUIRED: 'Загрузите состав бригад',
    DEMO_ENGINEERS: 'Демонстрационный состав',
    DEMO_OFFICE_OVERRIDE: 'Демонстрационный офис',
    DEMO_GEO: 'Демонстрационные геоданные',
    EXECUTION_STATE_REQUIRED: 'Уточните время завершения',
    ACTUAL_CONSTRAINT_VIOLATION: 'Нарушение ограничения',
    GEO_UNAVAILABLE: 'Адрес не распознан',
};

export const runStatusLabel = {
    queued: 'В очереди',
    running: 'Считаем маршруты',
    succeeded: 'План готов',
    failed: 'Расчёт не удался',
} as const;

export const engineerSkillOptions = [
    'emergency',
    'connection',
    'repair',
    'additional',
    'optical',
    'local',
] as const;
