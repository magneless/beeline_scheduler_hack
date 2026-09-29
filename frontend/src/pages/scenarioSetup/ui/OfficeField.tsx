import { lazy, Suspense, useState } from 'react';
import { Check, MapPin, X } from 'lucide-react';
import { Dialog } from 'radix-ui';

import { type Point } from 'shared/api/types/contracts';
import { Button } from 'shared/ui/button';
import { Input } from 'shared/ui/input';

const AddressPicker = lazy(() =>
    import('shared/ui/addressPicker').then((module) => ({
        default: module.AddressPicker,
    }))
);

type Props = {
    address: string;
    point?: Point;
    disabled?: boolean;
    onAddress: (value: string) => void;
    onPoint: (value: Point | undefined) => void;
};

export const OfficeField = ({
    address,
    point,
    disabled,
    onAddress,
    onPoint,
}: Props) => {
    const [open, setOpen] = useState(false);
    return (
        <div className="min-w-0 space-y-2">
            <label htmlFor="day-office" className="text-sm font-semibold">
                Офис · начало маршрутов
            </label>
            <div className="flex min-w-0 flex-col gap-2 sm:flex-row">
                <Input
                    id="day-office"
                    value={address}
                    disabled={disabled}
                    maxLength={1000}
                    placeholder="Город, улица, дом"
                    className="min-w-0 rounded-[8px] border border-input bg-muted/30 sm:flex-1"
                    onChange={(event) => {
                        onAddress(event.target.value);
                        onPoint(undefined);
                    }}
                />
                <Button
                    type="button"
                    variant="outline"
                    disabled={disabled}
                    className="shrink-0 self-end rounded-[8px]"
                    onClick={() => setOpen(true)}
                >
                    <MapPin className="size-4" />
                    На карте
                </Button>
            </div>
            <p className="flex items-center gap-1 text-xs text-muted-foreground">
                {point ? (
                    <>
                        <Check className="size-3.5" />
                        Точка офиса выбрана
                    </>
                ) : (
                    'Укажите адрес или загрузите его из CSV заявок.'
                )}
            </p>
            <Dialog.Root open={open} onOpenChange={setOpen}>
                <Dialog.Portal>
                    <Dialog.Overlay className="fixed inset-0 z-[1000] bg-black/35" />
                    <Dialog.Content
                        className={[
                            'fixed top-1/2 left-1/2 z-[1001] max-h-[90vh] w-[min(640px,calc(100vw-24px))]',
                            '-translate-x-1/2 -translate-y-1/2 overflow-y-auto rounded-xl border bg-card p-5 shadow-xl',
                        ].join(' ')}
                        aria-describedby="office-help"
                    >
                        <div className="mb-2 flex items-center justify-between gap-3">
                            <Dialog.Title className="font-bold">
                                Офис
                            </Dialog.Title>
                            <Dialog.Close asChild>
                                <Button
                                    type="button"
                                    variant="ghost"
                                    size="icon"
                                    aria-label="Закрыть карту офиса"
                                >
                                    <X className="size-4" />
                                </Button>
                            </Dialog.Close>
                        </div>
                        <Dialog.Description
                            id="office-help"
                            className="mb-4 text-sm text-muted-foreground"
                        >
                            Выберите точку, откуда бригады начинают рабочий
                            день.
                        </Dialog.Description>
                        <Suspense
                            fallback={<p role="status">Загружаем карту…</p>}
                        >
                            <AddressPicker
                                address={address}
                                point={point}
                                onAddress={onAddress}
                                onPoint={onPoint}
                                center={
                                    point ?? { lat: 55.751244, lon: 37.618423 }
                                }
                                inputLabel="Поиск офиса"
                            />
                        </Suspense>
                        <div className="mt-4 flex justify-end">
                            <Dialog.Close asChild>
                                <Button
                                    type="button"
                                    disabled={!point || !address.trim()}
                                    className="rounded-[8px]"
                                >
                                    Подтвердить офис
                                </Button>
                            </Dialog.Close>
                        </div>
                    </Dialog.Content>
                </Dialog.Portal>
            </Dialog.Root>
        </div>
    );
};
