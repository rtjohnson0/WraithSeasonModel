import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// base './' keeps every asset path relative, so the build works from a GitHub Pages project
// path such as https://name.github.io/repo/ as well as from a custom domain.
const privateBoard=process.env.VITE_PRIVATE_BOARD==='true'
if (privateBoard && !process.env.WRAITH_PRIVATE_PUBLIC_DIR) throw new Error('Use npm run build:private to exclude premium data.')
export default defineConfig({
  base: privateBoard ? '/' : './',
  publicDir: privateBoard ? process.env.WRAITH_PRIVATE_PUBLIC_DIR : 'public',
  plugins: [react(), ...(privateBoard ? [{ name:'private-base', transformIndexHtml(){return [{tag:'base',attrs:{href:'/'},injectTo:'head-prepend'}]} }] : [])],
})
