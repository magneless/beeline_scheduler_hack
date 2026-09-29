import { type FormEvent, useRef, useState } from 'react';
import { ArrowRight } from 'lucide-react';
import { DateTime } from 'luxon';

import { type importScenario } from 'shared/api';
import { type Point } from 'shared/api/types/contracts';
import { Button } from 'shared/ui/button';
import { DatePicker } from 'shared/ui/datePicker';

import { OfficeField } from './OfficeField';
import { type SelectedCSV, SetupFileField } from './SetupFileField';
import { type ImportKind, readCSVPreview } from '../lib/csvPreview';

type Props = {
    pending: boolean;
    error: Error | null;
    onEdit: () => void;
    onImport: (input: Parameters<typeof importScenario>[0]) => void;
};

export const SetupImportBar = ({ pending, error, onEdit, onImport }: Props) => {
    const [date, setDate] = useState(
        () => DateTime.now().setZone('Europe/Moscow').toISODate() ?? ''
    );
    const [office, setOffice] = useState('');
    const [point, setPoint] = useState<Point>();
    const [files, setFiles] = useState<
        Partial<Record<ImportKind, SelectedCSV>>
    >({});
    const [fileErrors, setFileErrors] = useState<
        Partial<Record<ImportKind, string>>
    >({});
    const [busy, setBusy] = useState<Partial<Record<ImportKind, boolean>>>({});
    const versions = useRef({ orders: 0, engineers: 0 });
    const officeRef = useRef(office);
    const csvOffice = useRef('');
    officeRef.current = office;
    const [submitError, setSubmitError] = useState('');
    const edit = () => {
        onEdit();
        setSubmitError('');
    };
    const selectFile = async (kind: ImportKind, file?: File) => {
        const version = ++versions.current[kind];
        edit();
        setFiles((current) => ({ ...current, [kind]: undefined }));
        setFileErrors((current) => ({ ...current, [kind]: undefined }));
        if (!file) {
            if (kind === 'orders' && officeRef.current === csvOffice.current) {
                setOffice('');
                csvOffice.current = '';
                setPoint(undefined);
            }
            setBusy((current) => ({ ...current, [kind]: false }));
            return;
        }
        setBusy((current) => ({ ...current, [kind]: true }));
        try {
            const preview = await readCSVPreview(file, kind);
            if (versions.current[kind] !== version) {
                return;
            }
            setFiles((current) => ({
                ...current,
                [kind]: { file, ...preview },
            }));
            if (kind === 'orders') {
                if (preview.date) {
                    setDate(preview.date);
                }
                if (
                    !officeRef.current.trim() ||
                    officeRef.current === csvOffice.current
                ) {
                    setOffice(preview.office ?? '');
                    csvOffice.current = preview.office ?? '';
                    setPoint(undefined);
                }
            }
        } catch (failure) {
            if (versions.current[kind] === version) {
                setFileErrors((current) => ({
                    ...current,
                    [kind]:
                        failure instanceof Error
                            ? failure.message
                            : 'Не удалось прочитать файл.',
                }));
            }
        } finally {
            if (versions.current[kind] === version) {
                setBusy((current) => ({ ...current, [kind]: false }));
            }
        }
    };
    const dateMismatch = files.orders?.date && files.orders.date !== date;
    const ready = Boolean(
        files.orders &&
        files.engineers &&
        office.trim() &&
        date &&
        !dateMismatch &&
        !busy.orders &&
        !busy.engineers
    );
    const submit = (event: FormEvent<HTMLFormElement>) => {
        event.preventDefault();
        if (pending) {
            return;
        }
        if (!ready || !files.orders || !files.engineers) {
            setSubmitError(
                'Добавьте оба файла, укажите офис и проверьте дату.'
            );
            return;
        }
        onImport({
            file: files.orders.file,
            engineersFile: files.engineers.file,
            date,
            officeAddress: office.trim(),
            officePoint: point,
        });
    };
    return (
        <form
            onSubmit={submit}
            className="@container flex min-w-0 flex-col rounded-2xl border border-border bg-card p-5 sm:p-6"
            aria-label="Ручная подготовка"
        >
            <h2 className="text-base font-bold">Ручная подготовка</h2>
            <div className="mt-5 flex flex-1 flex-col gap-5">
                <div className="space-y-2">
                    <p className="text-sm font-semibold">Дата работы</p>
                    <DatePicker
                        value={date}
                        disabled={pending}
                        aria-label="Дата работы"
                        className="rounded-[8px]"
                        onChange={(value) => {
                            setDate(value);
                            edit();
                        }}
                    />
                    {dateMismatch ? (
                        <p role="alert" className="text-xs text-destructive">
                            В файле заявок указана дата{' '}
                            {files.orders?.date?.split('-').reverse().join('.')}
                            . Выберите её или замените файл.
                        </p>
                    ) : null}
                </div>
                <OfficeField
                    address={office}
                    point={point}
                    disabled={pending}
                    onAddress={(value) => {
                        setOffice(value);
                        csvOffice.current = '';
                        edit();
                    }}
                    onPoint={(value) => {
                        setPoint(value);
                        if (value) {
                            csvOffice.current = '';
                        }
                        edit();
                    }}
                />
                <div className="grid gap-3 @min-[420px]:grid-cols-2">
                    {(['orders', 'engineers'] as const).map((kind) => (
                        <SetupFileField
                            key={kind}
                            kind={kind}
                            value={files[kind]}
                            error={fileErrors[kind]}
                            busy={!!busy[kind]}
                            disabled={pending}
                            onFile={(file) => void selectFile(kind, file)}
                        />
                    ))}
                </div>
                {error || submitError ? (
                    <p
                        role="alert"
                        className={[
                            'rounded-lg border border-destructive/20 bg-destructive/5 px-3 py-2',
                            'text-sm text-destructive',
                        ].join(' ')}
                    >
                        {error?.message ?? submitError}
                    </p>
                ) : null}
                <div className="mt-auto flex justify-end border-t border-border pt-5">
                    <Button
                        type="submit"
                        disabled={pending || !ready}
                        className="rounded-[8px]"
                    >
                        {pending ? 'Подготавливаем…' : 'Подготовить день'}
                        <ArrowRight className="size-4" />
                    </Button>
                </div>
            </div>
        </form>
    );
};
