export type ImportKind = 'orders' | 'engineers';
export type CSVPreview = { count: number; date?: string; office?: string };

const orderColumns = [
    'Заявка',
    'Тип заявки BK',
    'Тип заявки HD',
    'Начало',
    'Окончание',
    'Адрес',
];
const engineerColumns = [
    'id',
    'skills',
    'transport',
    'shift_start',
    'shift_end',
    'available',
    'router',
    'tv_box',
];

const parseRows = (text: string) => {
    const rows: string[][] = [];
    let row: string[] = [];
    let cell = '';
    let quoted = false;
    let closed = false;
    const finishCell = () => {
        row.push(cell.trim());
        cell = '';
        closed = false;
    };
    const finishRow = () => {
        finishCell();
        if (row.some(Boolean)) {
            rows.push(row);
        }
        row = [];
    };
    for (let i = 0; i < text.length; i++) {
        const char = text[i];
        if (quoted) {
            if (char === '"' && text[i + 1] === '"') {
                cell += '"';
                i++;
            } else if (char === '"') {
                quoted = false;
                closed = true;
            } else {
                cell += char;
            }
        } else if (char === ';') {
            finishCell();
        } else if (char === '\n' || char === '\r') {
            finishRow();
            if (char === '\r' && text[i + 1] === '\n') {
                i++;
            }
        } else if (char === '"' && !cell && !closed) {
            quoted = true;
        } else if (closed || char === '"') {
            throw new Error('Проверьте кавычки и разделители в CSV.');
        } else {
            cell += char;
        }
    }
    if (quoted) {
        throw new Error('В CSV не закрыты кавычки.');
    }
    finishRow();
    return rows;
};

export const previewCSV = (text: string, kind: ImportKind): CSVPreview => {
    const [header = [], ...rows] = parseRows(text.replace(/^\uFEFF/, ''));
    const required = kind === 'orders' ? orderColumns : engineerColumns;
    if (
        new Set(header).size !== header.length ||
        required.some((column) => !header.includes(column))
    ) {
        throw new Error(
            'Столбцы не соответствуют шаблону. Разделитель CSV — точка с запятой.'
        );
    }
    if (
        kind === 'engineers' &&
        (required.some((column, index) => header[index] !== column) ||
            header.length > 9 ||
            (header.length === 9 && header[8] !== 'reserve'))
    ) {
        throw new Error('Используйте порядок столбцов из шаблона бригад.');
    }
    const officeRows =
        kind === 'orders'
            ? rows.filter(
                  (row) => row[0]?.toLocaleLowerCase('ru') === 'адрес офиса'
              )
            : [];
    if (officeRows.length > 1) {
        throw new Error(
            'В CSV указано несколько офисов. Оставьте одну строку офиса.'
        );
    }
    const entries = rows.filter((row) => !officeRows.includes(row));
    if (!entries.length) {
        throw new Error('В файле нет записей.');
    }
    if (entries.some((row) => row.length !== header.length)) {
        throw new Error(
            'Количество столбцов в строках не совпадает с заголовком CSV.'
        );
    }
    if (kind === 'engineers') {
        return { count: entries.length };
    }
    const dates = new Set<string>();
    for (const row of entries) {
        for (const column of ['Начало', 'Окончание']) {
            const value = row[header.indexOf(column)].match(
                /^(\d{2})\.(\d{2})\.(\d{4}) \d{1,2}:\d{2}$/
            );
            if (!value) {
                throw new Error(
                    'Время в CSV должно иметь вид 28.09.2026 10:00.'
                );
            }
            const date = `${value[3]}-${value[2]}-${value[1]}`;
            const parsed = new Date(`${date}T00:00:00Z`);
            if (
                !Number.isFinite(parsed.getTime()) ||
                parsed.toISOString().slice(0, 10) !== date
            ) {
                throw new Error('Некорректная дата в CSV.');
            }
            dates.add(date);
        }
    }
    if (dates.size !== 1) {
        throw new Error(
            'Заявки должны относиться к одному дню. Разделите файл по датам.'
        );
    }
    return {
        count: entries.length,
        date: [...dates][0],
        office: officeRows[0]?.[1],
    };
};

export const readCSVPreview = async (file: File, kind: ImportKind) => {
    if (!file.size || file.size > 10 * 1024 * 1024) {
        throw new Error('Выберите непустой CSV размером до 10 МБ.');
    }
    const bytes = await file.arrayBuffer();
    let text: string;
    try {
        text = new TextDecoder('utf-8', { fatal: true }).decode(bytes);
    } catch {
        text = new TextDecoder('windows-1251').decode(bytes);
    }
    return previewCSV(text, kind);
};
