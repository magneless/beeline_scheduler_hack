import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { test } from 'node:test';

import { previewCSV, readCSVPreview } from '../src/pages/scenarioSetup/lib/csvPreview.ts';

const header = 'Заявка;Тип заявки BK;Тип заявки HD;Начало;Окончание;Адрес\n';
const row = '1;Подключение;Конвергенция абонента;28.09.2026 10:00;28.09.2026 12:00;"Москва; улица\nдом 1"\n';

test('preview preserves quoted fields, BOM and office metadata', () => {
    assert.deepEqual(previewCSV('\ufeff' + header + row + 'Адрес офиса;Москва, офис;;;;\r\n', 'orders'), { count: 1, date: '2026-09-28', office: 'Москва, офис' });
});

test('ambiguous dates, invalid calendar dates and broken files are rejected', () => {
    for (const input of [header + row + row.replaceAll('28.09', '29.09'), header + row.replaceAll('28.09', '31.09'), header + row.replace('"\n', '\n'), 'wrong;header\n1;2', header]) {
        assert.throws(() => previewCSV(input, 'orders'));
    }
});

test('Windows-1251 source dataset supplies its actual date and row count', async () => {
    const bytes = await readFile(new URL('../../datasets/additional_days/восток день 2.csv', import.meta.url));
    const result = await readCSVPreview(new File([bytes], 'orders.csv'), 'orders');
    assert.deepEqual(result, { count: 74, date: '2026-09-28', office: undefined });
});

test('published engineer template has matching skills and can be previewed', async () => {
    const text = await readFile(new URL('../public/sample-engineers.csv', import.meta.url), 'utf8');
    assert.deepEqual(previewCSV(text, 'engineers'), { count: 3 });
    assert.match(text, /connection,repair,emergency/);
    assert.doesNotMatch(text, /;router,tv_box;/);
});

test('empty files are rejected before preparation', async () => {
    await assert.rejects(readCSVPreview(new File([], 'empty.csv'), 'orders'));
});
