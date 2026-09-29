import { useRef, useState } from 'react';
import { useParams } from 'react-router-dom';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { MapPin, Search, X } from 'lucide-react';
import { Dialog } from 'radix-ui';
import { toast } from 'sonner';

import { getScenario } from 'shared/api';
import { apiPost } from 'shared/api/instance/httpClient';
import {
    type Location,
    type Order,
    type Point,
} from 'shared/api/types/contracts';
import { commandErrorMessage } from 'shared/lib/commandErrors';
import { Button } from 'shared/ui/button';
import { Input } from 'shared/ui/input';

import { AddressPointPicker } from './AddressPointPicker';

import { type WorkspaceEventInput } from '../model/types';

type Candidate = { address: string; point: Point; source?: string };
type SearchResult = {
    location: Location | null;
    issue: { message: string } | null;
    candidates?: Candidate[];
};
type Props = {
    order: Order;
    address?: string;
    occurredAt: string;
    pending?: boolean;
    onEvent?: (input: WorkspaceEventInput) => void;
};

export const OrderAddressResolver = ({
    order,
    address = '',
    occurredAt,
    pending,
    onEvent,
}: Props) => {
    const { scenarioId = '' } = useParams();
    const client = useQueryClient();
    const { data } = useQuery({
        queryKey: ['scenarios', scenarioId, 'current'],
        queryFn: () => getScenario(scenarioId),
        enabled: Boolean(scenarioId),
    });
    const [open, setOpen] = useState(false);
    const [query, setQuery] = useState(address);
    const [candidates, setCandidates] = useState<Candidate[]>([]);
    const [point, setPoint] = useState<Point>();
    const [lat, setLat] = useState('');
    const [lon, setLon] = useState('');
    const [message, setMessage] = useState('');
    const [searching, setSearching] = useState(false);
    const [saving, setSaving] = useState(false);
    const request = useRef(0);
    const selectPoint = (value: Point) => {
        setPoint(value);
        setLat(value.lat.toFixed(7));
        setLon(value.lon.toFixed(7));
    };
    const editCoordinates = (latitude: string, longitude: string) => {
        setLat(latitude);
        setLon(longitude);
        const value = {
            lat: Number(latitude.replace(',', '.')),
            lon: Number(longitude.replace(',', '.')),
        };
        setPoint(
            latitude.trim() &&
                longitude.trim() &&
                Number.isFinite(value.lat) &&
                Number.isFinite(value.lon) &&
                Math.abs(value.lat) <= 90 &&
                Math.abs(value.lon) <= 180
                ? value
                : undefined
        );
    };
    const search = async () => {
        const token = ++request.current;
        setSearching(true);
        setMessage('');
        setCandidates([]);
        setPoint(undefined);
        setLat('');
        setLon('');
        try {
            const result = await apiPost<SearchResult>('/geocode', {
                address: query,
                region_id: data?.snapshot.region_id,
            });
            if (token !== request.current) {
                return;
            }
            const found = result.location
                ? [{ address: query, point: result.location.point }]
                : (result.candidates ?? []);
            setCandidates(found);
            setMessage(
                result.location
                    ? 'Адрес найден. Проверьте точку и подтвердите.'
                    : (result.issue?.message ?? 'Выберите дом на карте.')
            );
            if (result.location) {
                selectPoint(result.location.point);
            }
        } catch (error) {
            if (token === request.current) {
                setMessage(commandErrorMessage(error));
            }
        } finally {
            if (token === request.current) {
                setSearching(false);
            }
        }
    };
    const save = async () => {
        const coordinates = {
            lat: Number(lat.replace(',', '.')),
            lon: Number(lon.replace(',', '.')),
        };
        if (
            !data ||
            !query.trim() ||
            !lat.trim() ||
            !lon.trim() ||
            !Number.isFinite(coordinates.lat) ||
            !Number.isFinite(coordinates.lon) ||
            Math.abs(coordinates.lat) > 90 ||
            Math.abs(coordinates.lon) > 180
        ) {
            setMessage(
                'Укажите адрес и выберите точку на карте или введите координаты.'
            );
            return;
        }
        if (data.current_plan_id) {
            if (!onEvent) {
                setMessage('Дождитесь завершения текущей операции.');
                return;
            }
            onEvent({
                kind: 'new_order',
                restoreOrderId: order.id,
                point: coordinates,
                address: query.trim(),
                occurredAt,
                orderType: order.priority === 'urgent' ? 'urgent' : 'ordinary',
                workType: order.work_type,
                requiredSkills: order.required_skills,
                transport: order.required_transport,
                windowStart: order.window.start,
                windowEnd: order.window.end,
                serviceSec: order.service_sec,
                equipment: order.equipment_required,
            });
            setOpen(false);
            return;
        }
        setSaving(true);
        setMessage('');
        try {
            await apiPost(
                `/scenarios/${scenarioId}/orders/${encodeURIComponent(order.id)}/address`,
                {
                    snapshot_revision: data.snapshot.revision,
                    address: query.trim(),
                    point: coordinates,
                }
            );
            await client.invalidateQueries({
                queryKey: ['scenarios', scenarioId],
            });
            toast.success('Адрес подтверждён. Заявка включена в расчёт.');
            setOpen(false);
        } catch (error) {
            setMessage(commandErrorMessage(error));
        } finally {
            setSaving(false);
        }
    };
    const office = data?.snapshot.locations.find(
        (location) => location.id === data.snapshot.office_location_id
    )?.point;
    return (
        <Dialog.Root
            open={open}
            onOpenChange={(value) => {
                if (saving) {
                    return;
                }
                setOpen(value);
                if (!value) {
                    request.current++;
                    setSearching(false);
                }
            }}
        >
            <Dialog.Trigger asChild>
                <Button size="sm" variant="outline" disabled={pending}>
                    <MapPin className="size-3.5" />
                    Уточнить адрес
                </Button>
            </Dialog.Trigger>
            <Dialog.Portal>
                <Dialog.Overlay className="fixed inset-0 z-[1000] bg-black/35" />
                <Dialog.Content
                    className={[
                        'fixed left-1/2 top-1/2 z-[1001] flex max-h-[90vh]',
                        'w-[min(640px,calc(100vw-24px))] -translate-x-1/2 -translate-y-1/2',
                        'flex-col rounded-xl border bg-white shadow-xl',
                    ].join(' ')}
                    aria-describedby="address-description"
                >
                    <div className="flex items-center justify-between border-b px-5 py-4">
                        <Dialog.Title className="font-bold">
                            Адрес заявки
                        </Dialog.Title>
                        <Dialog.Close asChild>
                            <Button
                                variant="ghost"
                                size="icon"
                                aria-label="Закрыть уточнение адреса"
                                disabled={saving}
                            >
                                <X className="size-4" />
                            </Button>
                        </Dialog.Close>
                    </div>
                    <div className="space-y-3 overflow-y-auto p-5">
                        <Dialog.Description
                            id="address-description"
                            className="text-sm text-muted-foreground"
                        >
                            Найдите адрес или отметьте дом на карте. После
                            подтверждения заявка сможет участвовать в расчёте.
                        </Dialog.Description>
                        <form
                            className="flex gap-2"
                            onSubmit={(event) => {
                                event.preventDefault();
                                void search();
                            }}
                        >
                            <Input
                                aria-label="Адрес для поиска"
                                value={query}
                                onChange={(event) => {
                                    request.current++;
                                    setSearching(false);
                                    setQuery(event.target.value);
                                    setCandidates([]);
                                    setPoint(undefined);
                                    setLat('');
                                    setLon('');
                                    setMessage('');
                                }}
                            />
                            <Button
                                type="submit"
                                disabled={searching || !query.trim()}
                            >
                                <Search className="size-4" />
                                {searching ? 'Ищем…' : 'Найти'}
                            </Button>
                        </form>
                        {message ? (
                            <p
                                role="status"
                                className="text-xs text-muted-foreground"
                            >
                                {message}
                            </p>
                        ) : null}
                        {candidates.length ? (
                            <div
                                className="max-h-40 space-y-1 overflow-y-auto"
                                aria-label="Найденные адреса"
                            >
                                {candidates.map((candidate, index) => (
                                    <button
                                        key={`${candidate.source}-${index}`}
                                        type="button"
                                        className={[
                                            'block w-full rounded-md border px-3 py-2',
                                            'text-left text-sm hover:bg-accent',
                                            'aria-pressed:border-primary aria-pressed:bg-primary/10',
                                        ].join(' ')}
                                        aria-pressed={
                                            point?.lat ===
                                                candidate.point.lat &&
                                            point?.lon === candidate.point.lon
                                        }
                                        onClick={() => {
                                            setQuery(candidate.address);
                                            selectPoint(candidate.point);
                                        }}
                                    >
                                        {candidate.address}
                                    </button>
                                ))}
                            </div>
                        ) : null}
                        <AddressPointPicker
                            center={
                                point ?? office ?? { lat: 55.75, lon: 37.62 }
                            }
                            point={point}
                            onPoint={selectPoint}
                        />
                        <div className="grid grid-cols-2 gap-3">
                            <label className="space-y-1 text-xs">
                                Широта
                                <Input
                                    aria-label="Широта адреса"
                                    inputMode="decimal"
                                    value={lat}
                                    onChange={(event) =>
                                        editCoordinates(event.target.value, lon)
                                    }
                                />
                            </label>
                            <label className="space-y-1 text-xs">
                                Долгота
                                <Input
                                    aria-label="Долгота адреса"
                                    inputMode="decimal"
                                    value={lon}
                                    onChange={(event) =>
                                        editCoordinates(lat, event.target.value)
                                    }
                                />
                            </label>
                        </div>
                    </div>
                    <div className="flex justify-end gap-2 border-t px-5 py-4">
                        <Dialog.Close asChild>
                            <Button variant="outline" disabled={saving}>
                                Отмена
                            </Button>
                        </Dialog.Close>
                        <Button
                            disabled={
                                saving || searching || pending || !lat || !lon
                            }
                            onClick={() => void save()}
                        >
                            {saving
                                ? 'Сохраняем…'
                                : data?.current_plan_id
                                  ? 'Сохранить в очередь'
                                  : 'Подтвердить адрес'}
                        </Button>
                    </div>
                </Dialog.Content>
            </Dialog.Portal>
        </Dialog.Root>
    );
};
