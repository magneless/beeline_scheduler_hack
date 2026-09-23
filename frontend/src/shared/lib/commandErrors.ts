import { HttpError } from 'shared/api';

export const commandErrorMessage = (error: unknown) => {
    if (!(error instanceof HttpError)) {
        return error instanceof Error
            ? error.message
            : 'Не удалось выполнить команду';
    }

    if (error.body.code === 'EVENT_CONFLICT') {
        return 'День уже начался. Это действие больше недоступно.';
    }

    if (error.body.code === 'IDEMPOTENCY_CONFLICT') {
        return 'Такой запрос уже уходил с другими данными.';
    }

    if (error.body.code === 'GEO_UNAVAILABLE') {
        return 'Геосервис недоступен. Повторите позже.';
    }

    return error.message;
};

export const isStaleVersionError = (error: unknown) =>
    error instanceof HttpError && error.body.code === 'STALE_VERSION';
