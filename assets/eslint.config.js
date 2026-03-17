import js from '@eslint/js'
import tseslint from 'typescript-eslint'
import jsdoc from 'eslint-plugin-jsdoc'

export default tseslint.config(
  // 1. 全局忽略配置
  {
    ignores: [
      'dist',
      'node_modules',
      '.vite',
      'src/test/**',
      'src/test*.ts',
      'test/**',
      'test/*.ts',
      'scripts/**',
    ],
  },

  // 2. 仅对 src 目录的 TypeScript 启用类型检查
  {
    files: ['src/**/*.ts', 'src/**/*.tsx'],
    extends: [
      ...tseslint.configs.recommendedTypeChecked,
      ...tseslint.configs.stylistic,
    ],
    plugins: {
      jsdoc,
    },
    languageOptions: {
      parserOptions: {
        project: ['./tsconfig.app.json', './tsconfig.node.json'],
        tsconfigRootDir: import.meta.dirname,
      },
    },
    settings: {
      jsdoc: { mode: 'typescript' },
    },
    rules: {
      '@typescript-eslint/no-unused-vars': ['warn', {
        argsIgnorePattern: '^_',
        varsIgnorePattern: '^_'
      }],
      '@typescript-eslint/no-explicit-any': 'off',
      'jsdoc/require-jsdoc': ['warn', {
        publicOnly: true,
        require: {
          FunctionDeclaration: true,
          MethodDefinition: true,
          ClassDeclaration: true,
          ArrowFunctionExpression: false,
        },
      }],
      'jsdoc/require-description': 'off',
      'jsdoc/check-tag-names': ['warn', { definedTags: ['category'] }],
      '@typescript-eslint/no-unsafe-assignment': 'off',
      '@typescript-eslint/no-unsafe-member-access': 'off',
      '@typescript-eslint/no-unsafe-return': 'off',
      '@typescript-eslint/restrict-template-expressions': 'off',
      '@typescript-eslint/consistent-generic-constructors': 'off',
    },
  },

  // 3. 对所有其他 TS/TSX 文件的基础规则（不需要类型检查）
  {
    files: ['**/*.ts', '**/*.tsx'],
    extends: [
      js.configs.recommended,
      ...tseslint.configs.recommended,
    ],
    plugins: {
      jsdoc,
    },
    settings: {
      jsdoc: { mode: 'typescript' },
    },
    rules: {
      '@typescript-eslint/no-unused-vars': ['warn', {
        argsIgnorePattern: '^_',
        varsIgnorePattern: '^_'
      }],
      '@typescript-eslint/no-explicit-any': 'off',
      'jsdoc/require-jsdoc': ['warn', {
        publicOnly: true,
        require: {
          FunctionDeclaration: true,
          MethodDefinition: true,
          ClassDeclaration: true,
          ArrowFunctionExpression: false,
        },
      }],
      'jsdoc/require-description': 'off',
      'jsdoc/check-tag-names': ['warn', { definedTags: ['category'] }],
    },
  }
)
