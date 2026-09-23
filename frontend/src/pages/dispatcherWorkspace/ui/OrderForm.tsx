import { useState } from 'react';
import { DateTime } from 'luxon';

import { type PlanEventInput } from 'features/applyPlanEvent';
import {
    type Snapshot,
    type Transport,
    type WorkType,
} from 'shared/api/types/contracts';
import { Button } from 'shared/ui/button';
import { Label } from 'shared/ui/label';

import { EventTimeField } from './EventTimeField';

type Props = {
    snapshot: Snapshot;
    timezone: string;
    occurredAt: string;
    pending: boolean;
    onSubmit: (input: PlanEventInput) => void;
    onClose: () => void;
};
const ordinaryTypes: WorkType[] = ['connection', 'repair', 'additional'];
export const OrderForm = ({
    snapshot,
    timezone,
    occurredAt,
    pending,
    onSubmit,
    onClose,
}: Props) => {
    const [kind, setKind] = useState<'urgent' | 'ordinary'>('urgent');
    const [locationId, setLocationId] = useState('');
    const [address, setAddress] = useState('');
    const [workType, setWorkType] = useState<WorkType>('emergency');
    const [skills, setSkills] = useState('');
    const [transport, setTransport] = useState<Transport | ''>('');
    const [eventAt, setEventAt] = useState(occurredAt);
    const [start, setStart] = useState(occurredAt);
    const [end, setEnd] = useState(
        DateTime.fromISO(occurredAt)
            .plus({ hours: 2 })
            .toUTC()
            .toISO({ suppressMilliseconds: true }) ?? occurredAt
    );
    const [duration, setDuration] = useState(3600);
    const [router, setRouter] = useState(0);
    const [tvBox, setTvBox] = useState(0);
    const [error, setError] = useState('');
    const localEnd = end
        ? DateTime.fromISO(end).setZone(timezone).toFormat("yyyy-MM-dd'T'HH:mm")
        : '';
    const submit = () => {
        if (!locationId && !address.trim()) {
            return setError('Укажите адрес или выберите существующую локацию');
        }
        if (
            !DateTime.fromISO(eventAt).isValid ||
            DateTime.fromISO(eventAt).setZone(timezone).toISODate() !==
                snapshot.date ||
            !DateTime.fromISO(start).isValid ||
            !DateTime.fromISO(end).isValid ||
            DateTime.fromISO(end) < DateTime.fromISO(start) ||
            DateTime.fromISO(end) < DateTime.fromISO(eventAt)
        ) {
            return setError('Проверьте окно обслуживания');
        }
        if (
            kind === 'ordinary' &&
            (!Number.isInteger(duration) || duration <= 0)
        ) {
            return setError('Укажите длительность');
        }
        if (
            !Number.isInteger(router) ||
            router < 0 ||
            !Number.isInteger(tvBox) ||
            tvBox < 0
        ) {
            return setError(
                'Количество оборудования должно быть целым и неотрицательным'
            );
        }
        setError('');
        onSubmit({
            kind: 'new_order',
            orderType: kind,
            occurredAt: eventAt,
            locationId: locationId || undefined,
            address: address.trim() || undefined,
            workType: kind === 'urgent' ? 'emergency' : workType,
            requiredSkills: skills
                .split(',')
                .map((x) => x.trim())
                .filter(Boolean),
            transport: transport || null,
            windowStart: start,
            windowEnd: end,
            serviceSec: kind === 'urgent' ? 4800 : duration,
            equipment: {
                ...(router > 0 ? { router } : {}),
                ...(tvBox > 0 ? { tv_box: tvBox } : {}),
            },
        });
    };
    const formClassName = [
        'absolute right-0 top-11 z-40 max-h-[calc(100vh-7rem)] overflow-y-auto',
        'w-96 max-w-[calc(100vw-2rem)] space-y-2 rounded-[22px] bg-card p-4 shadow-xl',
    ].join(' ');
    return (
        <div className={formClassName}>
            <div className="flex gap-2">
                <Button
                    size="sm"
                    variant={kind === 'urgent' ? 'default' : 'outline'}
                    onClick={() => {
                        setKind('urgent');
                        setWorkType('emergency');
                    }}
                >
                    Авария
                </Button>
                <Button
                    size="sm"
                    variant={kind === 'ordinary' ? 'default' : 'outline'}
                    onClick={() => {
                        setKind('ordinary');
                        setWorkType('repair');
                    }}
                >
                    Обычная
                </Button>
            </div>
            <select
                aria-label="Адрес заявки"
                className="w-full rounded border p-2"
                value={locationId}
                onChange={(e) => setLocationId(e.target.value)}
            >
                <option value="">Новая локация</option>
                {snapshot.locations.map((l) => (
                    <option key={l.id} value={l.id}>
                        {l.address}
                    </option>
                ))}
            </select>
            {!locationId && (
                <input
                    className="w-full rounded border p-2"
                    placeholder="Адрес"
                    value={address}
                    onChange={(e) => setAddress(e.target.value)}
                />
            )}
            {kind === 'ordinary' && (
                <select
                    aria-label="Тип работы"
                    className="w-full rounded border p-2"
                    value={workType}
                    onChange={(e) => setWorkType(e.target.value as WorkType)}
                >
                    {ordinaryTypes.map((t) => (
                        <option key={t} value={t}>
                            {t === 'connection'
                                ? 'Подключение'
                                : t === 'repair'
                                  ? 'Ремонт'
                                  : 'Дозаказ'}
                        </option>
                    ))}
                </select>
            )}
            <input
                className="w-full rounded border p-2"
                placeholder="Навыки через запятую"
                value={skills}
                onChange={(e) => setSkills(e.target.value)}
            />
            <select
                aria-label="Транспорт"
                className="w-full rounded border p-2"
                value={transport}
                onChange={(e) => setTransport(e.target.value as Transport | '')}
            >
                <option value="">Транспорт не задан</option>
                <option value="car">Автомобиль</option>
                <option value="walk">Пешком</option>
            </select>
            <EventTimeField
                label="Время события"
                value={eventAt}
                timezone={timezone}
                onChange={setEventAt}
            />
            <EventTimeField
                label="Окно обслуживания: с"
                value={start}
                timezone={timezone}
                onChange={setStart}
            />
            <Label htmlFor="new-order-window-end">Окно обслуживания: до</Label>
            <input
                id="new-order-window-end"
                type="datetime-local"
                className="w-full rounded border p-2"
                value={localEnd}
                onChange={(e) =>
                    setEnd(
                        DateTime.fromISO(e.target.value, { zone: timezone })
                            .toUTC()
                            .toISO({ suppressMilliseconds: true }) ?? ''
                    )
                }
            />
            {kind === 'ordinary' && (
                <>
                    <Label htmlFor="new-order-duration">
                        Длительность обслуживания, сек
                    </Label>
                    <input
                        type="number"
                        min="1"
                        className="w-full rounded border p-2"
                        id="new-order-duration"
                        value={duration}
                        onChange={(e) => setDuration(Number(e.target.value))}
                        placeholder="Длительность обслуживания, сек"
                    />
                </>
            )}
            {kind === 'urgent' && (
                <p className="text-xs text-muted-foreground">
                    Длительность аварии: 80 минут
                </p>
            )}
            <div className="grid grid-cols-2 gap-2">
                <Label htmlFor="new-order-router">Роутеры, шт.</Label>
                <Label htmlFor="new-order-tv">TV-приставки, шт.</Label>
                <input
                    id="new-order-router"
                    type="number"
                    min="0"
                    className="w-full rounded border p-2"
                    value={router}
                    onChange={(e) => setRouter(Number(e.target.value))}
                    placeholder="Роутеры"
                />
                <input
                    type="number"
                    min="0"
                    className="w-full rounded border p-2"
                    id="new-order-tv"
                    value={tvBox}
                    onChange={(e) => setTvBox(Number(e.target.value))}
                    placeholder="TV-box"
                />
            </div>
            {error && <div className="text-sm text-destructive">{error}</div>}
            <div className="flex gap-2">
                <Button
                    size="sm"
                    className="flex-1"
                    disabled={pending}
                    onClick={submit}
                >
                    Добавить заявку
                </Button>
                <Button size="sm" variant="outline" onClick={onClose}>
                    Отмена
                </Button>
            </div>
        </div>
    );
};
