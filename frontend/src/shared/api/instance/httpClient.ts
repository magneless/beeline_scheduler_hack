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
