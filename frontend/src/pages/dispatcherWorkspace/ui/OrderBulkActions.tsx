import { useEffect, useId, useState } from 'react';

import { type CancelReason } from 'shared/api/types/contracts';
import { Button } from 'shared/ui/button';
import { Checkbox } from 'shared/ui/checkbox';

import { EventTimeField } from './EventTimeField';

import { type WorkspaceEventInput } from '../model/types';

type Props = {
    selectedIds: string[];
    allSelected: boolean;
    pending?: boolean;
    timezone: string;
    defaultOccurredAt: string;
    onToggleAll: () => void;
    onClear: () => void;
    onEvent: (input: WorkspaceEventInput) => void;
};

export const OrderBulkActions = ({
    selectedIds,
    allSelected,
    pending,
    timezone,
    defaultOccurredAt,
    onToggleAll,
    onClear,
    onEvent,
}: Props) => {
    const id = useId();
    const [occurredAt, setOccurredAt] = useState(defaultOccurredAt);
    useEffect(() => {
        setOccurredAt(defaultOccurredAt);
    }, [defaultOccurredAt]);
    const cancel = (reason: CancelReason) => {
        if (selectedIds.length && !pending) {
            onEvent({
                kind: 'cancel_many',
                orderIds: selectedIds,
                occurredAt,
                reason,
            });
        }
    };

    return (
        <div
            aria-label="Групповая отмена заявок"
            className="space-y-2 border-t border-border pt-2"
        >
            <div className="flex flex-wrap items-center justify-between gap-2 text-xs">
                <label
                    htmlFor={id}
                    className="flex cursor-pointer items-center gap-2"
                >
                    <Checkbox
                        id={id}
                        checked={
                            allSelected
                                ? true
                                : selectedIds.length
                                  ? 'indeterminate'
                                  : false
                        }
                        disabled={pending}
                        onCheckedChange={onToggleAll}
                        aria-label="Выбрать все заявки в списке"
                    />
                    Выбрать все
                </label>
                {selectedIds.length > 0 ? (
                    <Button
                        size="sm"
                        variant="ghost"
                        className="h-6 px-1 text-xs"
                        disabled={pending}
                        onClick={onClear}
                    >
                        Снять выбор
                    </Button>
                ) : null}
            </div>
            {selectedIds.length > 0 ? (
                <div className="space-y-2 rounded-[10px] bg-accent p-2.5">
                    <p className="text-xs font-semibold" aria-live="polite">
                        Выбрано: {selectedIds.length}
                    </p>
                    <EventTimeField
                        label="Время отмены"
                        value={occurredAt}
                        timezone={timezone}
                        onChange={setOccurredAt}
                    />
                    <div className="flex flex-wrap gap-2">
                        <Button
                            size="sm"
                            disabled={pending}
                            onClick={() => cancel('client_refusal')}
                        >
                            Отказ клиента
                        </Button>
                        <Button
                            size="sm"
                            variant="outline"
                            disabled={pending}
                            onClick={() => cancel('cannot_perform')}
                        >
                            Невозможно выполнить
                        </Button>
                    </div>
                </div>
            ) : null}
        </div>
    );
};
