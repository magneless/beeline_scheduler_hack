import assert from 'node:assert/strict';
import { registerHooks } from 'node:module';
import { test } from 'node:test';

// Supply the build-time API URL when running the browser client in Node.
registerHooks({
    resolve(specifier, context, nextResolve) {
        if (specifier === 'shared/config/env') {
            return {
                url: 'data:text/javascript,export const env = {apiUrl: "http://localhost/api/v1"}',
                shortCircuit: true,
            };
        }
        if (specifier === 'shared/lib/types') {
            return { url: 'data:text/javascript,export {}', shortCircuit: true };
        }
        return nextResolve(specifier, context);
    },
});
const { apiPostWithProgress, HttpError } = await import('../src/shared/api/instance/httpClient.ts');

test('multipart upload preserves the file and browser-generated boundary while receiving progress', async (t) => {
    const body = new FormData();
    body.append('file', new File(['orders;address'], 'orders.csv', { type: 'text/csv' }));
    body.append('region_id', 'east');
    const messages = [
        'event: progress\r\ndata: {"stage":"geocoding","completed":1,"total":2,"message":"Адрес обработан"}\r\n\r\n',
        'event: heartbeat\ndata: {}\n\n',
        'event: result\ndata: {"id":"scenario-1"}\n\n',
    ].join('');
    const bytes = new TextEncoder().encode(messages);
    t.mock.method(globalThis, 'fetch', async (_url, init) => {
        assert.equal(init.body, body);
        assert.equal(new Headers(init.headers).has('Content-Type'), false);
        assert.equal(init.headers.Accept, 'text/event-stream');
        const upload = new Request('http://localhost', init);
        assert.match(upload.headers.get('Content-Type'), /multipart\/form-data; boundary=/);
        const fields = await upload.formData();
        assert.equal(await fields.get('file').text(), 'orders;address');
        assert.equal(fields.get('region_id'), 'east');
        return new Response(new ReadableStream({
            start(controller) {
                // Split UTF-8 characters, event names and separators across reads.
                for (let i = 0; i < bytes.length; i += 7) controller.enqueue(bytes.slice(i, i + 7));
                controller.close();
            },
        }), { headers: { 'Content-Type': 'text/event-stream' } });
    });
    const updates = [];
    let heartbeats = 0;
    const result = await apiPostWithProgress('/scenarios/import', body, {
        onProgress: (update) => updates.push(update),
        onHeartbeat: () => heartbeats++,
    });
    assert.deepEqual(result, { id: 'scenario-1' });
    assert.deepEqual(updates, [{ stage: 'geocoding', completed: 1, total: 2, message: 'Адрес обработан' }]);
    assert.equal(heartbeats, 2);
});

test('JSON requests and non-streaming responses remain compatible', async (t) => {
    t.mock.method(globalThis, 'fetch', async (_url, init) => {
        assert.equal(init.body, '{"demo_dataset_id":"east"}');
        assert.equal(init.headers['Content-Type'], 'application/json');
        return Response.json({ id: 'scenario-2' }, { status: 201 });
    });
    assert.deepEqual(await apiPostWithProgress('/scenarios', { demo_dataset_id: 'east' }), { id: 'scenario-2' });
});

test('stream errors and truncated responses do not become successful imports', async (t) => {
    const fetchMock = t.mock.method(globalThis, 'fetch', async () =>
        new Response('event: error\ndata: {"code":"INVALID_INPUT","message":"Некорректный CSV","details":{}}\n\n',
            { headers: { 'Content-Type': 'text/event-stream' } }));
    await assert.rejects(apiPostWithProgress('/scenarios/import', new FormData()),
        (error) => error instanceof HttpError && error.body.code === 'INVALID_INPUT');
    fetchMock.mock.mockImplementation(async () => new Response('event: heartbeat\ndata: {}\n\n',
        { headers: { 'Content-Type': 'text/event-stream' } }));
    await assert.rejects(apiPostWithProgress('/scenarios', {}), HttpError);
});
