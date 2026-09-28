import { HttpError } from 'shared/api';

const geoUnavailableMessage =
    'Сервис адресов и маршрутов временно недоступен. Повторите расчёт через несколько минут.';

const eventMessages: Record<string, string> = {
    'only started work can be completed':
        'Завершить можно только начатую работу. Время завершения должно быть позже начала.',
    'work cannot start before arrival or window':
        'Нельзя начать работу раньше прибытия бригады или начала окна заявки.',
    'engineer has another current trip or work':
        'Бригада ещё выполняет другую работу или находится в пути. Сначала завершите текущую работу.',
    'closed order cannot change':
        'Заявка уже завершена или отменена. Обновите план.',
    'status does not match current assignment':
        'Исполнитель заявки изменился. Обновите план.',
    'order must be assigned before work starts':
        'Перед началом работы заявка должна быть назначена бригаде.',
    'started work cannot be interrupted by cancellation':
        'Начатую работу нельзя отменить. Зафиксируйте её завершение.',
    'insufficient equipment':
        'У бригады недостаточно оборудования для этой заявки.',
    'invalid expected_end_at':
        'Ожидаемое окончание должно быть позже времени события.',
    'event time is outside the plan day or precedes base plan as_of':
        'Время события должно относиться к дню сценария и быть не раньше последнего принятого события.',
    'use Replan after execution starts':
        'Работы уже начались. Изменяйте рабочий план через события; для сравнения используйте «Собрать с нуля».',
    'use Replan after events':
        'Рабочий день уже содержит события. Для сравнения маршрутов используйте «Собрать с нуля».',
};

export const commandErrorMessage = (error: unknown) => {
    const message = error instanceof Error ? error.message : '';
    if (error instanceof HttpError) {
        if (error.body.code === 'EVENT_CONFLICT') {
            if (
                message ===
                'previous work must be completed before starting next'
            ) {
                const address = error.body.details.previous_address;
                const id = error.body.details.previous_order_id;
                return `Сначала завершите предыдущую заявку: ${
                    typeof address === 'string' && address
                        ? address
                        : `№ ${String(id ?? '')}`
                }.`;
            }
            return (
                eventMessages[message] ??
                (/\p{Script=Cyrillic}/u.test(message)
                    ? message
                    : 'Событие не соответствует состоянию заявки или бригады. Проверьте время и обновите план.')
            );
        }

        if (error.body.code === 'IDEMPOTENCY_CONFLICT') {
            return 'Такой запрос уже уходил с другими данными.';
        }

        if (error.body.code === 'GEO_UNAVAILABLE') {
            return geoUnavailableMessage;
        }
    }

    if (eventMessages[message]) {
        return eventMessages[message];
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
