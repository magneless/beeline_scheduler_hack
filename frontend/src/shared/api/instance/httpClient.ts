import { env } from 'shared/config/env';
import { type TypeOrNull } from 'shared/lib/types';

export type ApiError = {
    code: string;
    message: string;
    details: Record<string, unknown>;
};

export class HttpError extends Error {
    status: number;
    body: ApiError;

    constructor(status: number, body: ApiError) {
        super(body.message);
        this.name = 'HttpError';
        this.status = status;
        this.body = body;
    }
}

type RequestOptions = Omit<RequestInit, 'method' | 'body'>;

const fallbackError = (
    message: string,
    code = 'COMPUTATION_FAILED'
): ApiError => ({
    code,
    message,
    details: {},
});

const parseErrorBody = async (response: Response): Promise<ApiError> => {
    try {
        const body = (await response.json()) as Partial<ApiError>;

        return {
            code: body.code ?? 'COMPUTATION_FAILED',
            message:
                body.message ??
                (response.statusText || 'Запрос завершился ошибкой'),
            details: body.details ?? {},
        };
    } catch {
        return fallbackError(
            response.statusText || 'Запрос завершился ошибкой'
        );
    }
};

const parseSuccessBody = async <T>(response: Response): Promise<T> => {
    if (response.status === 204) {
        return undefined as T;
    }

    const raw = await response.text();

    if (!raw) {
        return undefined as T;
    }

    try {
        return JSON.parse(raw) as T;
    } catch {
        throw new HttpError(
            response.status,
            fallbackError('Сервер вернул некорректный JSON', 'INVALID_RESPONSE')
        );
    }
};

const httpRequest = async <T>(
    path: string,
    method: string,
    body?: TypeOrNull<BodyInit>,
    init?: RequestOptions
): Promise<T> => {
    const isFormData = body instanceof FormData;

    try {
        const response = await fetch(`${env.apiUrl}${path}`, {
            ...init,
            method,
            body: body ?? undefined,
            headers: {
                Accept: 'application/json',
                ...(body && !isFormData
                    ? { 'Content-Type': 'application/json' }
                    : {}),
                ...init?.headers,
            },
        });

        if (!response.ok) {
            throw new HttpError(
                response.status,
                await parseErrorBody(response)
            );
        }

        return parseSuccessBody<T>(response);
    } catch (error) {
        if (error instanceof HttpError) {
            throw error;
        }

        throw new HttpError(
            0,
            fallbackError(
                error instanceof Error
                    ? error.message
                    : 'Сеть недоступна. Повторите позже.',
                'NETWORK_ERROR'
            )
        );
    }
};

const toJsonBody = (body?: unknown): TypeOrNull<BodyInit> => {
    if (body === undefined || body === null) {
        return null;
    }

    if (body instanceof FormData || typeof body === 'string') {
        return body;
    }

    return JSON.stringify(body);
};

export const apiGet = <T>(path: string, init?: RequestOptions) =>
    httpRequest<T>(path, 'GET', null, init);

export const apiPost = <T>(
    path: string,
    body?: unknown,
    init?: RequestOptions
) => httpRequest<T>(path, 'POST', toJsonBody(body), init);

export type CalculationProgress = {
    stage: string;
    message: string;
    completed: number;
    total: number;
    variant_label?: string;
};

export type CalculationCallbacks = {
    onProgress?: (progress: CalculationProgress) => void;
    onHeartbeat?: () => void;
    signal?: AbortSignal;
};

// A calculation endpoint can return either its usual JSON or a stream of
// progress events followed by the same JSON in a `result` event.
export const apiPostWithProgress = async <T>(
    path: string,
    body: unknown,
    callbacks: CalculationCallbacks = {}
): Promise<T> => {
    let response: Response;
    try {
        response = await fetch(`${env.apiUrl}${path}`, {
            method: 'POST',
            body: JSON.stringify(body),
            headers: {
                Accept: 'text/event-stream',
                'Content-Type': 'application/json',
            },
            signal: callbacks.signal,
        });
    } catch (error) {
        if (callbacks.signal?.aborted) {
            throw error;
        }
        throw new HttpError(
            0,
            fallbackError(
                error instanceof Error
                    ? error.message
                    : 'Сеть недоступна. Повторите позже.',
                'NETWORK_ERROR'
            )
        );
    }

    if (!response.ok) {
        throw new HttpError(response.status, await parseErrorBody(response));
    }
    if (!response.headers.get('content-type')?.includes('text/event-stream')) {
        return parseSuccessBody<T>(response);
    }
    if (!response.body) {
        throw new HttpError(
            0,
            fallbackError('Сервер не передал ход расчёта', 'INVALID_RESPONSE')
        );
    }

    const reader = response.body.getReader();
    const decoder = new TextDecoder();
    let buffer = '';
    let result: T | undefined;
    let receivedResult = false;
    try {
        while (true) {
            const { value, done } = await reader.read();
            buffer += decoder.decode(value, { stream: !done });
            let separator = /\r?\n\r?\n/.exec(buffer);
            while (separator) {
                const rawEvent = buffer.slice(0, separator.index);
                buffer = buffer.slice(separator.index + separator[0].length);
                const event = rawEvent
                    .split(/\r?\n/)
                    .find((line) => line.startsWith('event:'))
                    ?.slice(6)
                    .trim();
                const data = rawEvent
                    .split(/\r?\n/)
                    .filter((line) => line.startsWith('data:'))
                    .map((line) => line.slice(5).trimStart())
                    .join('\n');
                if (event === 'progress') {
                    callbacks.onHeartbeat?.();
                    callbacks.onProgress?.(
                        JSON.parse(data) as CalculationProgress
                    );
                } else if (event === 'heartbeat') {
                    callbacks.onHeartbeat?.();
                } else if (event === 'error') {
                    const payload = JSON.parse(data) as ApiError;
                    throw new HttpError(422, payload);
                } else if (event === 'result') {
                    result = JSON.parse(data) as T;
                    receivedResult = true;
                }
                separator = /\r?\n\r?\n/.exec(buffer);
            }
            if (done) {
                break;
            }
        }
    } catch (error) {
        if (callbacks.signal?.aborted || error instanceof HttpError) {
            throw error;
        }
        throw new HttpError(
            0,
            fallbackError(
                error instanceof Error
                    ? error.message
                    : 'Поток расчёта прервался',
                'NETWORK_ERROR'
            )
        );
    } finally {
        reader.releaseLock();
    }
    if (!receivedResult) {
        throw new HttpError(
            0,
            fallbackError(
                'Соединение оборвалось до результата расчёта',
                'NETWORK_ERROR'
            )
        );
    }
    return result as T;
};

export const apiPatch = <T>(
    path: string,
    body?: unknown,
    init?: RequestOptions
) => httpRequest<T>(path, 'PATCH', toJsonBody(body), init);

export const apiPut = <T>(
    path: string,
    body?: unknown,
    init?: RequestOptions
) => httpRequest<T>(path, 'PUT', toJsonBody(body), init);

export const apiDelete = <T>(path: string, init?: RequestOptions) =>
    httpRequest<T>(path, 'DELETE', null, init);
