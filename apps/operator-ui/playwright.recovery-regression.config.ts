import {defineConfig} from '@playwright/test'
import base from './playwright.config'
// Keep mocked regressions separate from the persistent acceptance installation.
export default defineConfig({...base,workers:4,reporter:'list',projects:base.projects?.map((p,i)=>({...p,use:{...p.use,baseURL:`http://127.0.0.1:${5183+i}`}})),webServer:[{command:'pnpm dev --host 127.0.0.1 --port 5183 --strictPort',port:5183,env:{VITE_API_TOKEN:''},reuseExistingServer:false},{command:'pnpm dev --host 127.0.0.1 --port 5184 --strictPort',port:5184,env:{VITE_API_TOKEN:'e2e-explicit-token'},reuseExistingServer:false}]})
