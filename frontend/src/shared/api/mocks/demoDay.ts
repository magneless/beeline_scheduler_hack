import {
    type Engineer,
    type Issue,
    type Order,
    type Plan,
    type PlanEvent,
    type ScenarioView,
    type Transport,
} from 'shared/api/types/contracts';
import { type TypeOrNull } from 'shared/lib/types';

const DAY = '2026-08-17';

const at = (hours: number, minutes: number) => {
    const hh = String(hours).padStart(2, '0');
    const mm = String(minutes).padStart(2, '0');
    return `${DAY}T${hh}:${mm}:00Z`;
};

const line = (
    from: { lat: number; lon: number },
    to: { lat: number; lon: number }
) => [from, to];

type GeoPoint = { lat: number; lon: number };

type RegionStop = {
    id: string;
    address: string;
    point: GeoPoint;
};

type RegionGeo = {
    id: string;
    name: string;
    office: string;
    tone: string;
    officePoint: GeoPoint;
    stops: RegionStop[];
    streets: string[];
};

const REGION_GEOS: RegionGeo[] = [
    {
        id: 'north',
        name: 'Север',
        office: 'ул. Тимирязевская, д. 1',
        tone: 'Дмитровка и северные спальники',
        officePoint: { lat: 55.8284, lon: 37.5682 },
        stops: [
            {
                id: 'loc-1',
                address: 'Дмитровское ш., д. 89',
                point: { lat: 55.8572, lon: 37.5458 },
            },
            {
                id: 'loc-2',
                address: 'ул. Дубнинская, д. 40 к. 1',
                point: { lat: 55.8764, lon: 37.5611 },
            },
            {
                id: 'loc-3',
                address: 'ул. Декабристов, д. 20',
                point: { lat: 55.8661, lon: 37.6014 },
            },
            {
                id: 'loc-4',
                address: 'ул. Ангарская, д. 22',
                point: { lat: 55.8845, lon: 37.5226 },
            },
            {
                id: 'loc-5',
                address: 'Кронштадтский б-р, д. 7',
                point: { lat: 55.8428, lon: 37.5039 },
            },
            {
                id: 'loc-6',
                address: 'ул. 800-летия Москвы, д. 14',
                point: { lat: 55.8812, lon: 37.5824 },
            },
            {
                id: 'loc-7',
                address: 'ул. Линии Октябрьской Железной Дороги, д. 2',
                point: { lat: 55.8396, lon: 37.5788 },
            },
            {
                id: 'loc-8',
                address: 'Бескудниковский б-р, д. 11',
                point: { lat: 55.8689, lon: 37.5442 },
            },
            {
                id: 'loc-9',
                address: 'ул. Софьи Ковалевской, д. 8',
                point: { lat: 55.8491, lon: 37.5326 },
            },
            {
                id: 'loc-10',
                address: 'ул. Комдива Орлова, д. 5',
                point: { lat: 55.8354, lon: 37.5518 },
            },
        ],
        streets: [
            'ул. Костромская',
            'ул. Дегунинская',
            'ул. Керамическая',
            'пр-д Досфлота',
            'ул. Зеленоградская',
            'ул. Бусиновская Горка',
        ],
    },
    {
        id: 'east',
        name: 'Восток',
        office: 'Измайловский пр-кт, д. 93',
        tone: 'Измайлово и Соколиная гора',
        officePoint: { lat: 55.7886, lon: 37.7484 },
        stops: [
            {
                id: 'loc-1',
                address: 'ул. Первомайская, д. 42',
                point: { lat: 55.7938, lon: 37.7852 },
            },
            {
                id: 'loc-2',
                address: 'ш. Энтузиастов, д. 54',
                point: { lat: 55.7584, lon: 37.7526 },
            },
            {
                id: 'loc-3',
                address: 'ул. 9-я Парковая, д. 18',
                point: { lat: 55.8032, lon: 37.8041 },
            },
            {
                id: 'loc-4',
                address: 'ул. Щербаковская, д. 7',
                point: { lat: 55.7712, lon: 37.7288 },
            },
            {
                id: 'loc-5',
                address: 'Сиреневый б-р, д. 31',
                point: { lat: 55.8056, lon: 37.7742 },
            },
            {
                id: 'loc-6',
                address: 'ул. Электродная, д. 2',
                point: { lat: 55.7528, lon: 37.7364 },
            },
            {
                id: 'loc-7',
                address: 'ул. Измайловский Вал, д. 14',
                point: { lat: 55.7804, lon: 37.7196 },
            },
            {
                id: 'loc-8',
                address: 'ул. Авиамоторная, д. 10',
                point: { lat: 55.7536, lon: 37.7152 },
            },
            {
                id: 'loc-9',
                address: 'ул. Большая Семёновская, д. 27',
                point: { lat: 55.7818, lon: 37.7054 },
            },
            {
                id: 'loc-10',
                address: 'пр-кт Будённого, д. 19',
                point: { lat: 55.7682, lon: 37.7348 },
            },
        ],
        streets: [
            'ул. Ибрагимова',
            'ул. Фортунатовская',
            'ул. Плеханова',
            'ул. 16-я Парковая',
            'ул. Матросова',
            'ул. Халтуринская',
        ],
    },
    {
        id: 'south-east',
        name: 'Юго-восток',
        office: 'ул. Юных Ленинцев, д. 83с4',
        tone: 'Кузьминки, Текстильщики, Выхино',
        officePoint: { lat: 55.7054, lon: 37.7648 },
        stops: [
            {
                id: 'loc-1',
                address: 'пр-кт Волгоградский, д. 128 к. 5',
                point: { lat: 55.7248, lon: 37.7382 },
            },
            {
                id: 'loc-2',
                address: 'ул. Грайвороновская, д. 10 к. 2',
                point: { lat: 55.7186, lon: 37.7124 },
            },
            {
                id: 'loc-3',
                address: 'Рязанский пр-кт, д. 62',
                point: { lat: 55.7182, lon: 37.7946 },
            },
            {
                id: 'loc-4',
                address: 'ул. Михайлова, д. 14',
                point: { lat: 55.6864, lon: 37.7388 },
            },
            {
                id: 'loc-5',
                address: 'ул. Окская, д. 32',
                point: { lat: 55.7122, lon: 37.8126 },
            },
            {
                id: 'loc-6',
                address: 'ул. Международная, д. 28 стр. 1',
                point: { lat: 55.7441, lon: 37.6824 },
            },
            {
                id: 'loc-7',
                address: 'ул. Юных Ленинцев, д. 84',
                point: { lat: 55.6988, lon: 37.7762 },
            },
            {
                id: 'loc-8',
                address: 'ул. Ферганская, д. 10',
                point: { lat: 55.7024, lon: 37.8328 },
            },
            {
                id: 'loc-9',
                address: 'ул. Ташкентская, д. 16 к. 2',
                point: { lat: 55.6888, lon: 37.8142 },
            },
            {
                id: 'loc-10',
                address: 'ул. Зеленодольская, д. 28',
                point: { lat: 55.7042, lon: 37.7428 },
            },
        ],
        streets: [
            'ул. Академика Скрябина',
            'ул. Маршала Чуйкова',
            'ул. Кузьминская',
            'ул. Малышева',
            'ул. Ставропольская',
            'ул. Хлобыстова',
        ],
    },
    {
        id: 'south',
        name: 'Юг',
        office: 'ул. Красного Маяка, д. 2а',
        tone: 'Чертаново и Бирюлёво',
        officePoint: { lat: 55.6224, lon: 37.6058 },
        stops: [
            {
                id: 'loc-1',
                address: 'ул. Чертановская, д. 48',
                point: { lat: 55.6382, lon: 37.5946 },
            },
            {
                id: 'loc-2',
                address: 'Варшавское ш., д. 125',
                point: { lat: 55.6018, lon: 37.6184 },
            },
            {
                id: 'loc-3',
                address: 'ул. Газопровод, д. 6',
                point: { lat: 55.5846, lon: 37.5922 },
            },
            {
                id: 'loc-4',
                address: 'ул. Подольских Курсантов, д. 5',
                point: { lat: 55.5912, lon: 37.6388 },
            },
            {
                id: 'loc-5',
                address: 'ул. Кировоградская, д. 22',
                point: { lat: 55.6188, lon: 37.5864 },
            },
            {
                id: 'loc-6',
                address: 'ул. Бирюлёвская, д. 1с1',
                point: { lat: 55.5864, lon: 37.6612 },
            },
            {
                id: 'loc-7',
                address: 'ул. Днепропетровская, д. 31',
                point: { lat: 55.6412, lon: 37.6224 },
            },
            {
                id: 'loc-8',
                address: 'Симферопольский б-р, д. 18',
                point: { lat: 55.6486, lon: 37.6082 },
            },
            {
                id: 'loc-9',
                address: 'ул. Ягодная, д. 8',
                point: { lat: 55.5788, lon: 37.6486 },
            },
            {
                id: 'loc-10',
                address: 'ул. Россошанская, д. 4',
                point: { lat: 55.6084, lon: 37.6542 },
            },
        ],
        streets: [
            'ул. Медиков',
            'ул. Сумская',
            'ул. Харьковская',
            'ул. Загорьевская',
            'ул. Липецкая',
            'ул. Элеваторная',
        ],
    },
    {
        id: 'west',
        name: 'Запад',
        office: 'ул. Ярцевская, д. 19',
        tone: 'Кунцево и Можайка',
        officePoint: { lat: 55.7386, lon: 37.4162 },
        stops: [
            {
                id: 'loc-1',
                address: 'Можайское ш., д. 36',
                point: { lat: 55.7184, lon: 37.4328 },
            },
            {
                id: 'loc-2',
                address: 'ул. Кубинка, д. 12',
                point: { lat: 55.7288, lon: 37.3984 },
            },
            {
                id: 'loc-3',
                address: 'Рублёвское ш., д. 48',
                point: { lat: 55.7482, lon: 37.3926 },
            },
            {
                id: 'loc-4',
                address: 'ул. Толбухина, д. 8',
                point: { lat: 55.7126, lon: 37.4042 },
            },
            {
                id: 'loc-5',
                address: 'ул. Партизанская, д. 24',
                point: { lat: 55.7324, lon: 37.3588 },
            },
            {
                id: 'loc-6',
                address: 'ул. Кастанаевская, д. 42',
                point: { lat: 55.7328, lon: 37.4586 },
            },
            {
                id: 'loc-7',
                address: 'ул. Ивана Франко, д. 10',
                point: { lat: 55.7242, lon: 37.4482 },
            },
            {
                id: 'loc-8',
                address: 'ул. Крылатские Холмы, д. 30',
                point: { lat: 55.7586, lon: 37.4124 },
            },
            {
                id: 'loc-9',
                address: 'ул. Академика Павлова, д. 16',
                point: { lat: 55.7524, lon: 37.4488 },
            },
            {
                id: 'loc-10',
                address: 'ул. Молодогвардейская, д. 5',
                point: { lat: 55.7326, lon: 37.4342 },
            },
        ],
        streets: [
            'ул. Боженко',
            'ул. Гвардейская',
            'ул. Петра Алексеева',
            'ул. Ватутина',
            'ул. Кунцевская',
            'ул. Оршанская',
        ],
    },
];

const regionGeoById = Object.fromEntries(
    REGION_GEOS.map((region) => [region.id, region])
) as Record<string, RegionGeo>;

const getRegionGeo = (regionId: string) =>
    regionGeoById[regionId] ?? REGION_GEOS[2];

const EXTRA_CREWS = [
    { id: 'ivanov', skills: ['connection', 'repair'], transport: 'car' },
    { id: 'kuznetsov', skills: ['repair', 'emergency'], transport: 'car' },
    { id: 'smirnova', skills: ['connection'], transport: 'car' },
    { id: 'vasilev', skills: ['repair'], transport: 'walk' },
    { id: 'novikova', skills: ['connection', 'additional'], transport: 'car' },
    { id: 'fedorov', skills: ['emergency', 'repair'], transport: 'car' },
    { id: 'morozov', skills: ['repair'], transport: 'car' },
    { id: 'volkova', skills: ['connection', 'repair'], transport: 'car' },
    { id: 'alekseev', skills: ['repair', 'additional'], transport: 'walk' },
    { id: 'lebedev', skills: ['emergency'], transport: 'car' },
    { id: 'semenova', skills: ['connection'], transport: 'car' },
    { id: 'egorov', skills: ['repair'], transport: 'car' },
    { id: 'pavlov', skills: ['connection', 'repair'], transport: 'car' },
    { id: 'kozlova', skills: ['additional', 'repair'], transport: 'walk' },
    { id: 'stepanov', skills: ['repair', 'emergency'], transport: 'car' },
    { id: 'nikolaev', skills: ['connection'], transport: 'car' },
    { id: 'orlova', skills: ['repair'], transport: 'car' },
    { id: 'andreev', skills: ['emergency', 'connection'], transport: 'car' },
    { id: 'makarov', skills: ['repair'], transport: 'walk' },
    { id: 'nikitina', skills: ['connection', 'additional'], transport: 'car' },
    { id: 'zakharov', skills: ['repair'], transport: 'car' },
] as const;

const WORK_CYCLE = ['connection', 'repair', 'additional', 'emergency'] as const;

const extraPoint = (office: GeoPoint, index: number) => {
    const angle = index * 2.399;
    const ring = 0.032 + (index % 5) * 0.014;

    return {
        lat: office.lat + Math.sin(angle) * ring * 0.72,
        lon: office.lon + Math.cos(angle) * ring * 1.15,
    };
};

const extraFleet = (regionId: string) => {
    const geo = getRegionGeo(regionId);
    const locations = EXTRA_CREWS.map((_, index) => ({
        id: `loc-${11 + index}`,
        address: `${geo.streets[index % geo.streets.length]}, д. ${12 + index}`,
        point: extraPoint(geo.officePoint, index),
    }));
    const orders: Order[] = EXTRA_CREWS.map((crew, index) => {
        const workType = WORK_CYCLE[index % WORK_CYCLE.length];
        const requiredTransport: TypeOrNull<Transport> =
            crew.transport === 'car' ? 'car' : null;

        return {
            id: `order-${11 + index}`,
            location_id: `loc-${11 + index}`,
            work_type: workType,
            required_skills: [workType],
            required_transport: requiredTransport,
            window: {
                start: at(6 + (index % 3), 0),
                end: at(15, 0),
            },
            received_at: at(4, 0),
            service_sec: workType === 'emergency' ? 4800 : 2400,
            priority: workType === 'emergency' ? 'urgent' : 'normal',
            equipment_required: workType === 'connection' ? { router: 1 } : {},
            source_order: 11 + index,
            status: 'active' as const,
            execution: null,
        };
    });
    const engineers = EXTRA_CREWS.map((crew, index) => ({
        id: crew.id,
        skills: [...crew.skills],
        transport: crew.transport,
        shift: { start: at(7, 0), end: at(19, 0) },
        available: true,
        equipment_stock: { router: 1, tv_box: index % 4 === 0 ? 1 : 0 },
        source_order: 4 + index,
    }));
    const fromSeven = (minutes: number) => {
        const total = 7 * 60 + minutes;

        return at(Math.floor(total / 60), total % 60);
    };
    const routes = EXTRA_CREWS.map((crew, index) => {
        const point = extraPoint(geo.officePoint, index);
        const travel = 18 + (index % 5) * 4;
        const arrive = 24 + (index % 7) * 8;
        const service =
            WORK_CYCLE[index % WORK_CYCLE.length] === 'emergency' ? 80 : 40;
        const distance_m = 2800 + index * 320 + (index % 4) * 500;

        return {
            engineer_id: crew.id,
            start_location_id: 'office-1',
            start_at: at(7, 0),
            visits: [
                {
                    order_id: `order-${11 + index}`,
                    arrival_at: fromSeven(arrive),
                    start_at: fromSeven(arrive),
                    end_at: fromSeven(arrive + service),
                },
            ],
            legs: [
                {
                    id: `leg-${crew.id}-1`,
                    from_location_id: 'office-1',
                    to_location_id: `loc-${11 + index}`,
                    start_at: fromSeven(arrive - travel),
                    end_at: fromSeven(arrive),
                    distance_m,
                    geo_context_id: 'geo-1',
                    geometry: line(geo.officePoint, point),
                },
            ],
        };
    });
    const remaining = Object.fromEntries(
        EXTRA_CREWS.map((crew, index) => [
            crew.id,
            { router: 1, tv_box: index % 4 === 0 ? 1 : 0 },
        ])
    );
    const distances = EXTRA_CREWS.map((crew, index) => ({
        engineer_id: crew.id,
        distance_m: 2800 + index * 320 + (index % 4) * 500,
    }));

    return { locations, orders, engineers, routes, remaining, distances };
};

export const demoDatasets = {
    items: REGION_GEOS.map((region) => ({
        id: region.id,
        name: region.name,
        region_id: region.id,
        date: DAY,
        timezone: 'Europe/Moscow',
    })),
};

export const regionMeta: Record<
    string,
    { office: string; orders: number; crews: number; tone: string }
> = Object.fromEntries(
    REGION_GEOS.map((region) => [
        region.id,
        {
            office: region.office,
            orders: 30,
            crews: 24,
            tone: region.tone,
        },
    ])
);

const DAY_START = '2026-08-16T21:00:00Z';

const locationPoint = (
    locations: Array<{ id: string; point: GeoPoint }>,
    id: string
) => {
    const found = locations.find((item) => item.id === id)?.point;

    return found ?? getRegionGeo('south-east').officePoint;
};

export const createScenarioView = (
    regionId: string,
    options?: {
        scenarioId?: string;
        date?: string;
        issues?: Issue[];
    }
): ScenarioView => {
    const dataset =
        demoDatasets.items.find(
            (item) => item.id === regionId || item.region_id === regionId
        ) ?? demoDatasets.items[0];
    const geo = getRegionGeo(dataset.region_id);
    const extras = extraFleet(dataset.region_id);

    return {
        current_plan_id: null,
        snapshot: {
            scenario_id: options?.scenarioId ?? `scenario-${dataset.id}`,
            revision: 1,
            region_id: dataset.region_id,
            date: options?.date ?? dataset.date,
            timezone: dataset.timezone,
            office_location_id: 'office-1',
            locations: [
                {
                    id: 'office-1',
                    address: geo.office,
                    point: geo.officePoint,
                },
                ...geo.stops,
                ...extras.locations,
            ],
            orders: [
                {
                    id: 'order-1',
                    location_id: 'loc-1',
                    work_type: 'connection',
                    required_skills: ['connection'],
                    required_transport: 'car',
                    window: { start: at(8, 0), end: at(10, 0) },
                    received_at: at(4, 0),
                    service_sec: 2700,
                    priority: 'normal',
                    equipment_required: { router: 1 },
                    source_order: 1,
                    status: 'active',
                    execution: null,
                },
                {
                    id: 'order-2',
                    location_id: 'loc-2',
                    work_type: 'repair',
                    required_skills: ['repair'],
                    required_transport: null,
                    window: { start: at(8, 0), end: at(12, 0) },
                    received_at: at(4, 0),
                    service_sec: 2700,
                    priority: 'normal',
                    equipment_required: { router: 1 },
                    source_order: 2,
                    status: 'active',
                    execution: null,
                },
                {
                    id: 'order-3',
                    location_id: 'loc-3',
                    work_type: 'emergency',
                    required_skills: ['emergency'],
                    required_transport: 'car',
                    window: { start: at(9, 0), end: at(16, 0) },
                    received_at: at(9, 40),
                    service_sec: 4800,
                    priority: 'urgent',
                    equipment_required: { router: 1 },
                    source_order: 3,
                    status: 'active',
                    execution: null,
                },
                {
                    id: 'order-4',
                    location_id: 'loc-4',
                    work_type: 'connection',
                    required_skills: ['connection'],
                    required_transport: 'car',
                    window: { start: at(8, 0), end: at(10, 0) },
                    received_at: at(4, 0),
                    service_sec: 2400,
                    priority: 'normal',
                    equipment_required: { router: 1 },
                    source_order: 4,
                    status: 'active',
                    execution: null,
                },
                {
                    id: 'order-5',
                    location_id: 'loc-5',
                    work_type: 'additional',
                    required_skills: ['repair'],
                    required_transport: 'car',
                    window: { start: at(8, 0), end: at(12, 0) },
                    received_at: at(4, 0),
                    service_sec: 3000,
                    priority: 'normal',
                    equipment_required: { tv_box: 1 },
                    source_order: 5,
                    status: 'active',
                    execution: null,
                },
                {
                    id: 'order-6',
                    location_id: 'loc-6',
                    work_type: 'connection',
                    required_skills: ['connection'],
                    required_transport: null,
                    window: { start: at(9, 0), end: at(13, 0) },
                    received_at: at(4, 0),
                    service_sec: 2700,
                    priority: 'normal',
                    equipment_required: { router: 1 },
                    source_order: 6,
                    status: 'active',
                    execution: null,
                },
                {
                    id: 'order-7',
                    location_id: 'loc-7',
                    work_type: 'repair',
                    required_skills: ['repair'],
                    required_transport: 'walk',
                    window: { start: at(7, 0), end: at(11, 0) },
                    received_at: at(4, 0),
                    service_sec: 2700,
                    priority: 'normal',
                    equipment_required: { router: 1 },
                    source_order: 7,
                    status: 'active',
                    execution: null,
                },
                {
                    id: 'order-8',
                    location_id: 'loc-8',
                    work_type: 'repair',
                    required_skills: ['repair'],
                    required_transport: 'walk',
                    window: { start: at(8, 0), end: at(12, 0) },
                    received_at: at(4, 0),
                    service_sec: 2400,
                    priority: 'normal',
                    equipment_required: {},
                    source_order: 8,
                    status: 'active',
                    execution: null,
                },
                {
                    id: 'order-9',
                    location_id: 'loc-9',
                    work_type: 'emergency',
                    required_skills: ['emergency', 'optical'],
                    required_transport: 'car',
                    window: { start: at(7, 0), end: at(16, 0) },
                    received_at: at(4, 0),
                    service_sec: 4800,
                    priority: 'urgent',
                    equipment_required: { router: 1 },
                    source_order: 9,
                    status: 'active',
                    execution: null,
                },
                ...extras.orders,
            ],
            engineers: [
                {
                    id: 'sokolov',
                    skills: ['connection', 'repair', 'emergency'],
                    transport: 'car',
                    shift: { start: at(7, 0), end: at(19, 0) },
                    available: true,
                    equipment_stock: { router: 2, tv_box: 0 },
                    source_order: 1,
                },
                {
                    id: 'melnikova',
                    skills: ['connection', 'repair'],
                    transport: 'car',
                    shift: { start: at(7, 0), end: at(19, 0) },
                    available: true,
                    equipment_stock: { router: 1, tv_box: 1 },
                    source_order: 2,
                },
                {
                    id: 'popov',
                    skills: ['repair'],
                    transport: 'walk',
                    shift: { start: at(7, 0), end: at(19, 0) },
                    available: true,
                    equipment_stock: { router: 1, tv_box: 0 },
                    source_order: 3,
                },
                ...extras.engineers,
            ],
            issues: options?.issues ?? [],
        },
    };
};

export const importRowIssue: Issue = {
    source_row: 3,
    entity_id: null,
    field: 'service_sec',
    code: 'INVALID_INPUT',
    message: 'Не определена длительность работы; строка исключена из расчёта',
};

export const scanImportIssues = (csvText: string): Issue[] => {
    const lines = csvText
        .split(/\r?\n/)
        .map((line) => line.trim())
        .filter(Boolean);

    if (lines.length < 2) {
        return [];
    }

    const issues: Issue[] = [];

    lines.slice(1).forEach((line, index) => {
        const cells = line.split(';');
        const orderId = cells[0]?.trim() ?? '';
        const workType = cells[1]?.trim() ?? '';
        const windowEnd = cells[4]?.trim() ?? '';
        const address = cells[6]?.trim() ?? '';
        const sourceRow = index + 2;

        if (!orderId || !windowEnd) {
            issues.push({
                ...importRowIssue,
                source_row: sourceRow,
                message:
                    'Не определена длительность работы; строка исключена из расчёта',
            });
            return;
        }

        if (!workType || address.toLowerCase().includes('не распознан')) {
            issues.push({
                source_row: sourceRow,
                entity_id: null,
                field: 'address',
                code: 'INVALID_INPUT',
                message: 'Адрес не распознан; строка исключена из расчёта',
            });
        }
    });

    return issues;
};

export const isScenarioBlocked = (
    scenario: ScenarioView,
    plan?: TypeOrNull<Plan>
) =>
    Boolean(plan?.base_plan_id) ||
    scenario.snapshot.orders.some((order) => order.execution !== null);

export const applyEngineerPatch = (
    scenario: ScenarioView,
    engineerId: string,
    patch: Partial<
        Pick<
            Engineer,
            'skills' | 'transport' | 'shift' | 'available' | 'equipment_stock'
        >
    >
) => {
    const next = structuredClone(scenario);
    const engineer = next.snapshot.engineers.find(
        (item) => item.id === engineerId
    );

    if (!engineer) {
        return null;
    }

    next.snapshot.revision += 1;

    if (patch.skills !== undefined) {
        engineer.skills = patch.skills;
    }

    if (patch.transport !== undefined) {
        engineer.transport = patch.transport;
    }

    if (patch.shift !== undefined) {
        engineer.shift = patch.shift;
    }

    if (patch.available !== undefined) {
        engineer.available = patch.available;
    }

    if (patch.equipment_stock !== undefined) {
        engineer.equipment_stock = patch.equipment_stock;
    }

    return next;
};

export const createDemoPlan = (scenario: ScenarioView): Plan => {
    const extras = extraFleet(scenario.snapshot.region_id);
    const pt = (id: string) => locationPoint(scenario.snapshot.locations, id);

    return {
        id: 'plan-1',
        scenario_id: scenario.snapshot.scenario_id,
        snapshot_revision: 1,
        base_plan_id: null,
        as_of: DAY_START,
        routes: [
            {
                engineer_id: 'sokolov',
                start_location_id: 'office-1',
                start_at: at(7, 0),
                visits: [
                    {
                        order_id: 'order-1',
                        arrival_at: at(7, 18),
                        start_at: at(8, 0),
                        end_at: at(8, 45),
                    },
                    {
                        order_id: 'order-2',
                        arrival_at: at(9, 4),
                        start_at: at(9, 4),
                        end_at: at(9, 49),
                    },
                    {
                        order_id: 'order-3',
                        arrival_at: at(10, 12),
                        start_at: at(10, 12),
                        end_at: at(11, 32),
                    },
                ],
                legs: [
                    {
                        id: 'leg-s-1',
                        from_location_id: 'office-1',
                        to_location_id: 'loc-1',
                        start_at: at(7, 0),
                        end_at: at(7, 18),
                        distance_m: 4200,
                        geo_context_id: 'geo-1',
                        geometry: line(pt('office-1'), pt('loc-1')),
                    },
                    {
                        id: 'leg-s-2',
                        from_location_id: 'loc-1',
                        to_location_id: 'loc-2',
                        start_at: at(8, 45),
                        end_at: at(9, 4),
                        distance_m: 3800,
                        geo_context_id: 'geo-1',
                        geometry: line(pt('loc-1'), pt('loc-2')),
                    },
                    {
                        id: 'leg-s-3',
                        from_location_id: 'loc-2',
                        to_location_id: 'loc-3',
                        start_at: at(9, 49),
                        end_at: at(10, 12),
                        distance_m: 4500,
                        geo_context_id: 'geo-1',
                        geometry: line(pt('loc-2'), pt('loc-3')),
                    },
                ],
            },
            {
                engineer_id: 'melnikova',
                start_location_id: 'office-1',
                start_at: at(7, 0),
                visits: [
                    {
                        order_id: 'order-4',
                        arrival_at: at(7, 22),
                        start_at: at(8, 0),
                        end_at: at(8, 40),
                    },
                    {
                        order_id: 'order-5',
                        arrival_at: at(8, 58),
                        start_at: at(8, 58),
                        end_at: at(9, 48),
                    },
                    {
                        order_id: 'order-6',
                        arrival_at: at(10, 10),
                        start_at: at(10, 10),
                        end_at: at(10, 55),
                    },
                ],
                legs: [
                    {
                        id: 'leg-m-1',
                        from_location_id: 'office-1',
                        to_location_id: 'loc-4',
                        start_at: at(7, 0),
                        end_at: at(7, 22),
                        distance_m: 4600,
                        geo_context_id: 'geo-1',
                        geometry: line(pt('office-1'), pt('loc-4')),
                    },
                    {
                        id: 'leg-m-2',
                        from_location_id: 'loc-4',
                        to_location_id: 'loc-5',
                        start_at: at(8, 40),
                        end_at: at(8, 58),
                        distance_m: 4100,
                        geo_context_id: 'geo-1',
                        geometry: line(pt('loc-4'), pt('loc-5')),
                    },
                    {
                        id: 'leg-m-3',
                        from_location_id: 'loc-5',
                        to_location_id: 'loc-6',
                        start_at: at(9, 48),
                        end_at: at(10, 10),
                        distance_m: 3900,
                        geo_context_id: 'geo-1',
                        geometry: line(pt('loc-5'), pt('loc-6')),
                    },
                ],
            },
            {
                engineer_id: 'popov',
                start_location_id: 'office-1',
                start_at: at(7, 0),
                visits: [
                    {
                        order_id: 'order-7',
                        arrival_at: at(7, 28),
                        start_at: at(7, 28),
                        end_at: at(8, 13),
                    },
                    {
                        order_id: 'order-8',
                        arrival_at: at(8, 48),
                        start_at: at(9, 0),
                        end_at: at(9, 40),
                    },
                ],
                legs: [
                    {
                        id: 'leg-p-1',
                        from_location_id: 'office-1',
                        to_location_id: 'loc-7',
                        start_at: at(7, 0),
                        end_at: at(7, 28),
                        distance_m: 1900,
                        geo_context_id: 'geo-1',
                        geometry: line(pt('office-1'), pt('loc-7')),
                    },
                    {
                        id: 'leg-p-2',
                        from_location_id: 'loc-7',
                        to_location_id: 'loc-8',
                        start_at: at(8, 13),
                        end_at: at(8, 48),
                        distance_m: 2600,
                        geo_context_id: 'geo-1',
                        geometry: line(pt('loc-7'), pt('loc-8')),
                    },
                ],
            },
            ...extras.routes,
        ],
        unassigned: [
            {
                order_id: 'order-9',
                reason_code: 'NO_MATCHING_SKILL',
                message:
                    'Нет инженера со всеми необходимыми навыками, включая оптику.',
            },
        ],
        cancelled_order_ids: [],
        completed_order_ids: [],
        issues: [],
        metrics: {
            assigned_count: 8 + extras.distances.length,
            completed_count: 0,
            unassigned_count: 1,
            used_engineer_count: 3 + extras.distances.length,
            total_distance_m:
                34100 +
                extras.distances.reduce(
                    (sum, item) => sum + item.distance_m,
                    0
                ),
            per_engineer: [
                { engineer_id: 'sokolov', distance_m: 12500 },
                { engineer_id: 'melnikova', distance_m: 12600 },
                { engineer_id: 'popov', distance_m: 4500 },
                ...extras.distances,
            ],
        },
        baseline_metrics: {
            assigned_count: 22,
            completed_count: 0,
            unassigned_count: 8,
            used_engineer_count: 18,
            total_distance_m: 128400,
            per_engineer: [
                { engineer_id: 'sokolov', distance_m: 15200 },
                { engineer_id: 'melnikova', distance_m: 16100 },
                { engineer_id: 'popov', distance_m: 4800 },
            ],
        },
        changes: [],
        termination: 'completed',
        equipment_remaining: {
            sokolov: { router: 2, tv_box: 0 },
            melnikova: { router: 1, tv_box: 1 },
            popov: { router: 1, tv_box: 0 },
            ...extras.remaining,
        },
    };
};

export const applyPlanEvent = (
    scenario: ScenarioView,
    plan: Plan,
    event: PlanEvent,
    nextPlanId: string
) => {
    const nextScenario: ScenarioView = structuredClone(scenario);
    const nextPlan: Plan = structuredClone(plan);
    const snapshot = nextScenario.snapshot;
    const pt = (id: string) => locationPoint(snapshot.locations, id);

    snapshot.revision += 1;
    nextPlan.id = nextPlanId;
    nextPlan.base_plan_id = plan.id;
    nextPlan.snapshot_revision = snapshot.revision;
    nextPlan.as_of = event.occurred_at;
    nextPlan.baseline_metrics = null;
    nextScenario.current_plan_id = nextPlanId;

    if (event.type === 'urgent_order_added') {
        snapshot.orders.push({
            id: 'order-10',
            location_id: 'loc-10',
            work_type: 'emergency',
            required_skills: ['emergency'],
            required_transport: 'car',
            window: { start: at(7, 0), end: at(16, 0) },
            received_at: event.occurred_at,
            service_sec: 4800,
            priority: 'urgent',
            equipment_required: { router: 1 },
            source_order: 10,
            status: 'active',
            execution: null,
        });

        const sokolov = nextPlan.routes.find(
            (route) => route.engineer_id === 'sokolov'
        );

        if (sokolov) {
            sokolov.visits = [
                {
                    order_id: 'order-10',
                    arrival_at: at(7, 16),
                    start_at: at(7, 16),
                    end_at: at(8, 36),
                },
                {
                    order_id: 'order-1',
                    arrival_at: at(8, 52),
                    start_at: at(8, 52),
                    end_at: at(9, 37),
                },
                {
                    order_id: 'order-3',
                    arrival_at: at(10, 0),
                    start_at: at(10, 0),
                    end_at: at(11, 20),
                },
            ];
            sokolov.legs = [
                {
                    id: 'leg-s-e',
                    from_location_id: 'office-1',
                    to_location_id: 'loc-10',
                    start_at: at(7, 0),
                    end_at: at(7, 16),
                    distance_m: 1800,
                    geo_context_id: 'geo-1',
                    geometry: line(pt('office-1'), pt('loc-10')),
                },
                {
                    id: 'leg-s-1b',
                    from_location_id: 'loc-10',
                    to_location_id: 'loc-1',
                    start_at: at(8, 36),
                    end_at: at(8, 52),
                    distance_m: 2100,
                    geo_context_id: 'geo-1',
                    geometry: line(pt('loc-10'), pt('loc-1')),
                },
                {
                    id: 'leg-s-3b',
                    from_location_id: 'loc-1',
                    to_location_id: 'loc-3',
                    start_at: at(9, 37),
                    end_at: at(10, 0),
                    distance_m: 2600,
                    geo_context_id: 'geo-1',
                    geometry: line(pt('loc-1'), pt('loc-3')),
                },
            ];
        }

        nextPlan.unassigned = [
            ...plan.unassigned,
            {
                order_id: 'order-2',
                reason_code: 'NO_MATCHING_EQUIPMENT',
                message:
                    'После аварии у Соколова не остаётся слота и запасного роутера на ремонт.',
            },
        ];
        const extraEngineers = plan.metrics.per_engineer.filter(
            (item) =>
                item.engineer_id !== 'sokolov' &&
                item.engineer_id !== 'melnikova' &&
                item.engineer_id !== 'popov'
        );
        nextPlan.metrics = {
            ...plan.metrics,
            assigned_count: plan.metrics.assigned_count,
            unassigned_count: nextPlan.unassigned.length,
            total_distance_m: plan.metrics.total_distance_m - 800,
            per_engineer: [
                { engineer_id: 'sokolov', distance_m: 6500 },
                { engineer_id: 'melnikova', distance_m: 8600 },
                { engineer_id: 'popov', distance_m: 2500 },
                ...extraEngineers,
            ],
        };
        nextPlan.changes = [
            {
                order_id: 'order-10',
                before: null,
                after: {
                    engineer_id: 'sokolov',
                    sequence: 1,
                    arrival_at: at(7, 16),
                    start_at: at(7, 16),
                    end_at: at(8, 36),
                },
                reason: 'assigned',
            },
            {
                order_id: 'order-2',
                before: {
                    engineer_id: 'sokolov',
                    sequence: 2,
                    arrival_at: at(9, 4),
                    start_at: at(9, 4),
                    end_at: at(9, 49),
                },
                after: null,
                reason: 'unassigned',
            },
            {
                order_id: 'order-1',
                before: {
                    engineer_id: 'sokolov',
                    sequence: 1,
                    arrival_at: at(7, 18),
                    start_at: at(8, 0),
                    end_at: at(8, 45),
                },
                after: {
                    engineer_id: 'sokolov',
                    sequence: 2,
                    arrival_at: at(8, 52),
                    start_at: at(8, 52),
                    end_at: at(9, 37),
                },
                reason: 'rescheduled',
            },
        ];
    }

    if (event.type === 'order_cancelled') {
        const orderId = String(event.payload.order_id);
        const order = snapshot.orders.find((item) => item.id === orderId);

        if (order) {
            order.status = 'cancelled';
        }

        nextPlan.routes.forEach((route) => {
            route.visits = route.visits.filter(
                (visit) => visit.order_id !== orderId
            );
        });
        nextPlan.unassigned = nextPlan.unassigned.filter(
            (item) => item.order_id !== orderId
        );
        nextPlan.cancelled_order_ids = [...plan.cancelled_order_ids, orderId];
        nextPlan.metrics = {
            ...nextPlan.metrics,
            assigned_count: Math.max(0, nextPlan.metrics.assigned_count - 1),
            unassigned_count: nextPlan.unassigned.length,
        };
        nextPlan.changes = [
            {
                order_id: orderId,
                before: null,
                after: null,
                reason: 'cancelled',
            },
        ];
    }

    if (event.type === 'order_status_changed') {
        const orderId = String(event.payload.order_id);
        const status = String(event.payload.status);
        const order = snapshot.orders.find((item) => item.id === orderId);
        const previous = order?.execution;
        const engineerId = String(event.payload.engineer_id);
        const expectedRaw = event.payload.expected_end_at;
        const expectedEnd =
            typeof expectedRaw === 'string' ? expectedRaw : null;

        if (order && status !== 'active' && status !== 'cancelled') {
            order.status = status as typeof order.status;
            order.execution = {
                engineer_id: engineerId,
                departed_at:
                    status === 'en_route'
                        ? event.occurred_at
                        : (previous?.departed_at ?? null),
                started_at:
                    status === 'in_progress' || status === 'completed'
                        ? (previous?.started_at ?? event.occurred_at)
                        : (previous?.started_at ?? null),
                finished_at: status === 'completed' ? event.occurred_at : null,
                expected_end_at: status === 'in_progress' ? expectedEnd : null,
            };
        }

        if (status === 'in_progress' && order?.equipment_required.router) {
            const engineerId = String(event.payload.engineer_id);
            const stock = nextPlan.equipment_remaining[engineerId];

            if (stock?.router) {
                stock.router -= 1;
            }
        }

        if (status === 'completed') {
            nextPlan.completed_order_ids = [
                ...plan.completed_order_ids,
                orderId,
            ];
            nextPlan.metrics = {
                ...nextPlan.metrics,
                completed_count: plan.metrics.completed_count + 1,
            };
        }

        nextPlan.changes = [
            {
                order_id: orderId,
                before: null,
                after: null,
                reason: 'status_changed',
            },
        ];
    }

    if (event.type === 'engineer_unavailable') {
        const engineerId = String(event.payload.engineer_id);
        const engineer = snapshot.engineers.find(
            (item) => item.id === engineerId
        );

        if (engineer) {
            engineer.available = false;
        }

        const route = nextPlan.routes.find(
            (item) => item.engineer_id === engineerId
        );
        const dropped = route?.visits.map((visit) => visit.order_id) ?? [];

        nextPlan.routes = nextPlan.routes.filter(
            (item) => item.engineer_id !== engineerId
        );
        nextPlan.unassigned = [
            ...nextPlan.unassigned,
            ...dropped.map((orderId) => ({
                order_id: orderId,
                reason_code: 'NO_AVAILABLE_ENGINEER',
                message: 'Бригада стала недоступна в течение дня.',
            })),
        ];
        nextPlan.metrics = {
            ...nextPlan.metrics,
            assigned_count: nextPlan.metrics.assigned_count - dropped.length,
            unassigned_count: nextPlan.unassigned.length,
            used_engineer_count: Math.max(
                0,
                nextPlan.metrics.used_engineer_count - 1
            ),
        };
        nextPlan.changes = dropped.map((orderId) => ({
            order_id: orderId,
            before: null,
            after: null,
            reason: 'unassigned' as const,
        }));
    }

    nextPlan.issues = snapshot.orders.flatMap((order) => {
        if (order.status !== 'in_progress' || !order.execution) {
            return [];
        }

        const expected = order.execution.expected_end_at;
        const expired = !expected || expected <= nextPlan.as_of;

        if (!expired) {
            return [];
        }

        return [
            {
                source_row: null,
                entity_id: order.id,
                field: 'expected_end_at',
                code: 'EXECUTION_STATE_REQUIRED',
                message: 'Уточните время завершения',
            },
        ];
    });

    return { scenario: nextScenario, plan: nextPlan };
};
