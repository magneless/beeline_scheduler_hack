import { useEffect, useRef, useState } from 'react';
import { Search } from 'lucide-react';

import { apiPost } from 'shared/api/instance/httpClient';
import { type Location, type Point } from 'shared/api/types/contracts';
import { Button } from 'shared/ui/button';
import { Input } from 'shared/ui/input';

import { AddressPointPicker } from './AddressPointPicker';

type Candidate = { address: string; point: Point };
type SearchResult = {
    location: Location | null;
    candidates?: Candidate[];
};
type Props = {
    address: string;
    point?: Point;
    center: Point;
    regionId: string;
    onAddress: (address: string) => void;
    onPoint: (point: Point | undefined) => void;
};

export const NewOrderAddress = ({
    address,
    point,
    center,
    regionId,
    onAddress,
    onPoint,
}: Props) => {
    const [searching, setSearching] = useState(false);
    const [message, setMessage] = useState('');
    const [candidates, setCandidates] = useState<Candidate[]>([]);
    const [focus, setFocus] = useState<Point>();
    const request = useRef<AbortController | null>(null);
    useEffect(() => () => request.current?.abort(), []);

    const stopSearch = () => {
        request.current?.abort();
        request.current = null;
        setSearching(false);
    };
    const pickPoint = (value: Point) => {
        stopSearch();
        onPoint(value);
        setMessage('');
    };
    const search = async () => {
        if (!address.trim()) {
            return;
        }
        stopSearch();
        const current = new AbortController();
        request.current = current;
        setSearching(true);
        setCandidates([]);
        setMessage('');
        onPoint(undefined);
        try {
            const result = await apiPost<SearchResult>(
                '/geocode',
                { address: address.trim(), region_id: regionId },
                { signal: current.signal }
            );
            if (current.signal.aborted || request.current !== current) {
                return;
            }
            if (result.location) {
                onPoint(result.location.point);
                setFocus(result.location.point);
                setMessage('Адрес найден. Проверьте отметку на карте.');
            } else {
                const found = result.candidates ?? [];
                setCandidates(found);
                setFocus(found[0]?.point);
                setMessage(
                    found.length
                        ? 'Есть похожие адреса. Выберите нужный или отметьте дом на карте.'
                        : 'Отметьте дом на карте или измените адрес для поиска.'
                );
            }
        } catch {
            if (!current.signal.aborted && request.current === current) {
                setMessage('Поиск временно недоступен. Отметьте дом на карте.');
            }
        } finally {
            if (request.current === current) {
                request.current = null;
                setSearching(false);
            }
        }
    };

    return (
        <section aria-label="Место новой заявки" className="space-y-2">
            <form
                className="flex gap-2"
                onSubmit={(event) => {
                    event.preventDefault();
                    void search();
                }}
            >
                <Input
                    aria-label="Адрес новой заявки"
                    placeholder="Город, улица, дом"
                    value={address}
                    maxLength={1000}
                    className="min-w-0 flex-1"
                    onChange={(event) => {
                        stopSearch();
                        onAddress(event.target.value);
                        onPoint(undefined);
                        setCandidates([]);
                        setMessage('');
                    }}
                />
                <Button
                    type="submit"
                    variant="outline"
                    disabled={searching || !address.trim()}
                    aria-label="Найти адрес на карте"
                >
                    <Search className="size-4" />
                    {searching ? 'Ищем…' : 'Найти'}
                </Button>
            </form>
            {message ? (
                <p role="status" className="text-xs text-muted-foreground">
                    {message}
                </p>
            ) : null}
            {candidates.length ? (
                <div
                    aria-label="Найденные адреса новой заявки"
                    className="max-h-32 space-y-1 overflow-y-auto"
                >
                    {candidates.map((candidate, index) => (
                        <button
                            key={index}
                            type="button"
                            className={[
                                'w-full rounded-md border px-3 py-2 text-left text-xs hover:bg-accent',
                                'aria-pressed:border-primary aria-pressed:bg-primary/10',
                            ].join(' ')}
                            aria-pressed={
                                point?.lat === candidate.point.lat &&
                                point?.lon === candidate.point.lon
                            }
                            onClick={() => {
                                onAddress(candidate.address);
                                pickPoint(candidate.point);
                                setFocus(candidate.point);
                            }}
                        >
                            {candidate.address}
                        </button>
                    ))}
                </div>
            ) : null}
            <AddressPointPicker
                center={center}
                focus={focus}
                point={point}
                onPoint={pickPoint}
            />
            <div className="flex items-center justify-between gap-2 text-xs">
                <span className="text-muted-foreground">
                    {point
                        ? `Точка выбрана: ${point.lat.toFixed(5)}, ${point.lon.toFixed(5)}`
                        : 'Найдите адрес или нажмите на нужный дом на карте.'}
                </span>
                {point ? (
                    <Button
                        type="button"
                        size="sm"
                        variant="ghost"
                        onClick={() => {
                            onPoint(undefined);
                            setMessage('');
                        }}
                    >
                        Сбросить
                    </Button>
                ) : null}
            </div>
        </section>
    );
};
