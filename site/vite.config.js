import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// base './' keeps every asset path relative, so the build works from a GitHub Pages project
// path such as https://name.github.io/repo/ as well as from a custom domain.
export default defineConfig({
  base: './',
  plugins: [react()],
})
