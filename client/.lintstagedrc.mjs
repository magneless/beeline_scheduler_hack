export default {
    'src/**/*.{js,jsx,ts,tsx}':
        'eslint --fix --max-warnings=0 --no-warn-ignored',
    'src/**/*.module.scss': 'stylelint --fix --max-warnings=0',
    'src/**/*.{ts,tsx}': () => 'tsc -p tsconfig.app.json --noEmit',
};
