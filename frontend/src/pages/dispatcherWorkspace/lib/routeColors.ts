const colors = [
    '#2563eb',
    '#0d9488',
    '#7c3aed',
    '#e05a33',
    '#c02683',
    '#0284c7',
    '#658321',
    '#a16207',
];

export const routeColor = (engineerId: string) => {
    const number = engineerId.match(/(\d+)$/)?.[1];
    const index = number
        ? Number(number) - 1
        : [...engineerId].reduce(
              (hash, letter) => hash + letter.charCodeAt(0),
              0
          );
    return colors[Math.abs(index) % colors.length];
};
