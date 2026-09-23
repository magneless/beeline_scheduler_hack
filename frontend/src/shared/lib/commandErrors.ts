import { HttpError } from 'shared/api';

const geoUnavailableMessage =
    'Сервис адресов и маршрутов временно недоступен. Повторите расчёт через несколько минут.';

export const commandErrorMessage = (error: unknown) => {
    const message = error instanceof Error ? error.message : '';
    if (error instanceof HttpError) {
        if (error.body.code === 'EVENT_CONFLICT') {
            return 'День уже начался. Это действие больше недоступно.';
        }

        if (error.body.code === 'IDEMPOTENCY_CONFLICT') {
            return 'Такой запрос уже уходил с другими данными.';
        }

        if (error.body.code === 'GEO_UNAVAILABLE') {
            return geoUnavailableMessage;
        }
    }

    // Older saved runs can still contain the raw provider error.
    if (
        /geometry unavailable|tls handshake timeout|routing\.openstreetmap/i.test(
            message
        )
    ) {
        return geoUnavailableMessage;
    }

    return message || 'Не удалось выполнить команду';
};

export const isStaleVersionError = (error: unknown) =>
    error instanceof HttpError && error.body.code === 'STALE_VERSION';
