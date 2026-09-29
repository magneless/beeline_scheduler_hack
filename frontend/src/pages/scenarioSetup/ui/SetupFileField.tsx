import { Check, FileSpreadsheet, Upload, X } from 'lucide-react';

import { formatCount } from 'shared/lib/utils';
import { Button } from 'shared/ui/button';

import { type CSVPreview, type ImportKind } from '../lib/csvPreview';

export type SelectedCSV = CSVPreview & { file: File };
type Props = {
    kind: ImportKind;
    value?: SelectedCSV;
    error?: string;
    busy: boolean;
    disabled: boolean;
    onFile: (file: File | undefined) => void;
};

export const SetupFileField = ({
    kind,
    value,
    error,
    busy,
    disabled,
    onFile,
}: Props) => {
    const orders = kind === 'orders';
    const title = orders ? 'Заявки' : 'Бригады';
    return (
        <div className="flex min-w-0 flex-col rounded-[12px] border border-border p-4">
            <div className="mb-3 flex flex-wrap items-center justify-between gap-x-2 gap-y-1">
                <h3 className="font-semibold">{title}</h3>
                <a
                    href={
                        orders ? '/sample-orders.csv' : '/sample-engineers.csv'
                    }
                    download
                    className="text-xs text-muted-foreground underline underline-offset-4 hover:text-foreground"
                >
                    Шаблон CSV
                </a>
            </div>
            {value ? (
                <div className="flex min-w-0 items-start gap-2">
                    <FileSpreadsheet className="mt-0.5 size-4 shrink-0 text-muted-foreground" />
                    <div className="min-w-0 flex-1">
                        <p className="truncate text-sm" title={value.file.name}>
                            {value.file.name}
                        </p>
                        <p className="mt-1 flex items-center gap-1 text-xs text-muted-foreground">
                            <Check className="size-3" />
                            {formatCount(
                                value.count,
                                orders
                                    ? ['строка', 'строки', 'строк']
                                    : ['бригада', 'бригады', 'бригад']
                            )}
                        </p>
                    </div>
                    <Button
                        type="button"
                        variant="ghost"
                        size="icon"
                        className="size-6 shrink-0"
                        disabled={disabled}
                        aria-label={`Убрать файл: ${title}`}
                        onClick={() => onFile(undefined)}
                    >
                        <X className="size-3.5" />
                    </Button>
                </div>
            ) : (
                <p className="text-xs leading-relaxed text-muted-foreground">
                    {orders
                        ? 'Адреса, типы работ и окна визитов.'
                        : 'Навыки, транспорт и оборудование. Смена 10:00–22:00.'}
                </p>
            )}
            <label className="mt-auto inline-flex self-start pt-3">
                <Button
                    asChild
                    type="button"
                    variant="outline"
                    size="sm"
                    disabled={disabled || busy}
                    className="rounded-[8px]"
                >
                    <span>
                        <Upload className="size-3.5" />
                        {busy
                            ? 'Проверяем…'
                            : value
                              ? 'Заменить файл'
                              : 'Выбрать CSV'}
                    </span>
                </Button>
                <input
                    type="file"
                    accept=".csv,text/csv"
                    className="sr-only"
                    aria-label={`CSV: ${title}`}
                    disabled={disabled || busy}
                    onChange={(event) => {
                        const file = event.target.files?.[0];
                        if (file) {
                            onFile(file);
                        }
                        event.target.value = '';
                    }}
                />
            </label>
            {error ? (
                <p role="alert" className="mt-2 text-xs text-destructive">
                    {error}
                </p>
            ) : null}
        </div>
    );
};
