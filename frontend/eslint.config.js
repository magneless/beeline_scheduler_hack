import js from '@eslint/js';
import reactHooks from 'eslint-plugin-react-hooks';
import globals from 'globals';
import tseslint from 'typescript-eslint';
import eslintPluginPrettierRecommended from 'eslint-plugin-prettier/recommended';
import simpleImportSort from 'eslint-plugin-simple-import-sort';
import unusedImports from 'eslint-plugin-unused-imports';
import react from 'eslint-plugin-react';

export default tseslint.config(
    { ignores: ['dist', 'public/**'] },
    {
        extends: [
            js.configs.recommended,
            ...tseslint.configs.recommended,
            eslintPluginPrettierRecommended,
        ],
        files: ['**/*.{ts,tsx}'],
        languageOptions: {
            ecmaVersion: 2020,
            globals: globals.browser,
        },
        plugins: {
            'react-hooks': reactHooks,
            'simple-import-sort': simpleImportSort,
            react,
            'unused-imports': unusedImports,
        },
        rules: {
            ...reactHooks.configs.recommended.rules,
            'simple-import-sort/imports': [
                'warn',
                {
                    groups: [
                        ['^react', '^@?\\w'],
                        ['^(app|pages|features|shared)(/.*|$)'],
                        [
                            '^\\./(?=.*/)(?!/?$)',
                            '^\\.(?!/?$)',
                            '^\\./?$',
                            '^\\.\\.(?!/?$)',
                            '^\\.\\./?$',
                        ],
                        ['^'],
                        ['^.+\\.?(types)$'],
                        ['.module.scss', '.styled', '^.+\\.?(css)$'],
                    ],
                },
            ],
            semi: ['error', 'always'],
            'max-len': ['error', { code: 120 }],
            indent: 'off',
            'arrow-body-style': 'off',
            'prefer-arrow-callback': 'off',
            'no-console': [
                'error',
                {
                    allow: ['warn', 'error', 'info'],
                },
            ],
            curly: ['error', 'all'],
            'no-empty-function': 'off',
            'react/jsx-indent': ['error', 4],
            'react/prop-types': 'off',
            'react/jsx-indent-props': ['error', 4],
            'react/display-name': 'off',
            'react/self-closing-comp': 'off',
            'react/jsx-curly-brace-presence': [
                'error',
                { props: 'never', children: 'never' },
            ],
            'react/react-in-jsx-scope': 'off',
            'react-hooks/exhaustive-deps': 'off',
            'react/jsx-filename-extension': [
                1,
                { extensions: ['.js', '.jsx', '.tsx'] },
            ],
            '@typescript-eslint/no-unused-expressions': 'off',
            '@typescript-eslint/no-explicit-any': 'off',
            '@typescript-eslint/no-unused-vars': 'off',
            'unused-imports/no-unused-imports': 'error',
            '@typescript-eslint/consistent-type-imports': [
                'error',
                {
                    prefer: 'type-imports',
                    fixStyle: 'inline-type-imports',
                    disallowTypeAnnotations: false,
                },
            ],
        },
    }
);
