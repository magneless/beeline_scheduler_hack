export const demoRegions = [
    {
        id: 'north',
        name: 'Север',
        region_id: 'north',
        date: '2026-08-17',
        timezone: 'Europe/Moscow',
    },
    {
        id: 'east',
        name: 'Восток',
        region_id: 'east',
        date: '2026-08-17',
        timezone: 'Europe/Moscow',
    },
    {
        id: 'south-east',
        name: 'Юго-восток',
        region_id: 'south-east',
        date: '2026-08-17',
        timezone: 'Europe/Moscow',
    },
    {
        id: 'south',
        name: 'Юг',
        region_id: 'south',
        date: '2026-08-17',
        timezone: 'Europe/Moscow',
    },
    {
        id: 'west',
        name: 'Запад',
        region_id: 'west',
        date: '2026-08-17',
        timezone: 'Europe/Moscow',
    },
] as const;

export const regionMeta: Record<
    string,
    {
        office: string;
        orders: number;
        crews: number;
        tone: string;
        index: string;
    }
> = {
    north: {
        office: 'ул. Тимирязевская, д. 1',
        orders: 30,
        crews: 24,
        tone: 'Дмитровка и северные спальники',
        index: '01',
    },
    east: {
        office: 'Измайловский пр-кт, д. 93',
        orders: 30,
        crews: 24,
        tone: 'Измайлово и Соколиная гора',
        index: '02',
    },
    'south-east': {
        office: 'ул. Юных Ленинцев, д. 83с4',
        orders: 30,
        crews: 24,
        tone: 'Кузьминки, Текстильщики, Выхино',
        index: '03',
    },
    south: {
        office: 'ул. Красного Маяка, д. 2а',
        orders: 30,
        crews: 24,
        tone: 'Чертаново и Бирюлёво',
        index: '04',
    },
    west: {
        office: 'ул. Ярцевская, д. 19',
        orders: 30,
        crews: 24,
        tone: 'Кунцево и Можайка',
        index: '05',
    },
};

export const setupCopy = {
    brand: 'Билайн · диспетчерская',
    title: 'Подготовка рабочего дня',
    description: 'Выберите готовый набор или загрузите заявки и состав бригад.',
    regionLabel: 'Район',
    dateLabel: 'День',
    uploadIdle: 'Загрузить заявки',
    uploadPending: 'Загружаем…',
    sampleFile: 'Пример файла',
    fileRequired: 'Выберите CSV',
    openRegionError: 'Не удалось открыть район',
    importError: 'Не удалось загрузить CSV',
    skippedRows: (countLabel: string) => `${countLabel} не вошла в расчёт`,
} as const;
