import { defineConfig } from 'vite'
import preact from '@preact/preset-vite' // 使用 preact 插件
import { viteSingleFile } from 'vite-plugin-singlefile'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig({
  plugins: [
    tailwindcss(),
    preact(), // 代替 react()
    viteSingleFile(),
  ],
  resolve: {
    alias: {
      'react': 'preact/compat',
      'react-dom': 'preact/compat',
      'react/jsx-runtime': 'preact/jsx-runtime',
    },
  },
  assetsInclude: ['**/*.toml'], // 允许 TOML 文件作为资产导入
})