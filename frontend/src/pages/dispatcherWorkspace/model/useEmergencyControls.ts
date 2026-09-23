import { useEffect, useState } from 'react';

export const useEmergencyControls = (defaultOccurredAt: string) => {
    const [open, setOpen] = useState(false);
    const [occurredAt, setOccurredAt] = useState(defaultOccurredAt);

    const toggle = () => {
        setOpen((current) => {
            if (!current && defaultOccurredAt) {
                setOccurredAt(defaultOccurredAt);
            }

            return !current;
        });
    };

    const submit = (onEmergency: (occurredAt: string) => void) => {
        onEmergency(occurredAt);
        setOpen(false);
    };

    useEffect(() => {
        if (!open && defaultOccurredAt) {
            setOccurredAt(defaultOccurredAt);
        }
    }, [defaultOccurredAt, open]);

    return {
        open,
        occurredAt,
        setOccurredAt,
        toggle,
        submit,
    };
};
