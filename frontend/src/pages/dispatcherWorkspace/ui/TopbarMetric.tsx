type TopbarMetricProps = {
    value: string;
    label: string;
    hint?: string;
};

export const TopbarMetric = ({ value, label, hint }: TopbarMetricProps) => (
    <span className="flex items-baseline gap-1.5 whitespace-nowrap">
        <span className="text-sm font-extrabold">{value}</span>
        <span className="text-xs text-muted-foreground">{label}</span>
        {hint ? (
            <span className="text-[10px] font-semibold text-foreground/45">
                {hint}
            </span>
        ) : null}
    </span>
);
