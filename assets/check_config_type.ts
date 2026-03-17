import tseslint from 'typescript-eslint';
console.log('Is array:', Array.isArray(tseslint.configs.recommendedTypeChecked));
console.log('Length:', tseslint.configs.recommendedTypeChecked.length);
console.log('First element keys:', Object.keys(tseslint.configs.recommendedTypeChecked[0]));
