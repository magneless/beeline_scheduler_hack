import { type ChangeEvent, type FormEvent, useState } from 'react';
import { FileSpreadsheet, Upload } from 'lucide-react';
import { toast } from 'sonner';

import { type DemoDataset } from 'shared/api';
import { type TypeOrNull } from 'shared/lib/types';
import { cn } from 'shared/lib/utils';
import { Button } from 'shared/ui/button';
import { DatePicker } from 'shared/ui/datePicker';
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from 'shared/ui/select';

import { setupCopy } from '../lib/config';

type SetupImportBarProps = {
    datasets: DemoDataset[];
    pending: boolean;
    isImporting: boolean;
    onImport: (input: { file: File; regionId: string; date: string }) => void;
};

export const SetupImportBar = ({
    datasets,
    pending,
    isImporting,
    onImport,
}: SetupImportBarProps) => {
    const [selectedRegionId, setRegionId] = useState('');
    const [selectedDate, setDate] = useState('');
    const regionId = selectedRegionId || datasets[0]?.region_id || '';
    const date =
        selectedDate ||
        datasets.find((d) => d.region_id === regionId)?.date ||
        '';
    const [file, setFile] = useState<TypeOrNull<File>>(null);
    const uploadLabel = isImporting
        ? setupCopy.uploadPending
        : (file?.name ?? setupCopy.uploadIdle);

    const submitFile = (next: File) => {
        if (!regionId || !date) {
            toast.error('Выберите район и дату');
            return;
        }
        onImport({
            file: next,
            regionId,
            date,
        });
    };

    const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
        event.preventDefault();

        if (!file) {
            toast.error(setupCopy.fileRequired);
            return;
        }

        submitFile(file);
    };

    const handleFileChange = (event: ChangeEvent<HTMLInputElement>) => {
        const next = event.target.files?.[0] ?? null;

        setFile(next);

        if (!next) {
            return;
        }

        submitFile(next);
    };

    return (
        <form
            className={cn(
                'mt-10 rounded-[28px] border border-dashed border-foreground/12',
                'bg-muted/50 p-4'
            )}
            onSubmit={handleSubmit}
        >
            <div className="flex flex-wrap items-center justify-center gap-2">
                <Select value={regionId} onValueChange={setRegionId}>
                    <SelectTrigger
                        aria-label={setupCopy.regionLabel}
                        className="w-40"
                    >
                        <SelectValue placeholder={setupCopy.regionLabel} />
                    </SelectTrigger>
                    <SelectContent>
                        {datasets.map((dataset) => (
                            <SelectItem
                                key={dataset.id}
                                value={dataset.region_id}
                            >
                                {dataset.name}
                            </SelectItem>
                        ))}
                    </SelectContent>
                </Select>
                <DatePicker
                    value={date}
                    aria-label={setupCopy.dateLabel}
                    onChange={setDate}
                />
                <label>
                    <Button asChild disabled={pending}>
                        <span className="max-w-[220px] cursor-pointer">
                            {file ? (
                                <FileSpreadsheet className="size-4 shrink-0" />
                            ) : (
                                <Upload className="size-4 shrink-0" />
                            )}
                            <span className="min-w-0 truncate">
                                {uploadLabel}
                            </span>
                        </span>
                    </Button>
                    <input
                        type="file"
                        accept=".csv,text/csv"
                        className="sr-only"
                        disabled={pending}
                        onChange={handleFileChange}
                    />
                </label>
                <Button asChild variant="link" size="sm">
                    <a href="/sample-orders.csv">{setupCopy.sampleFile}</a>
                </Button>
            </div>
        </form>
    );
};
