import {defineConfig,devices} from '@playwright/test'
export default defineConfig({testDir:'./tests/recovery',workers:1,timeout:180000,outputDir:'../../private/local/browser-results',use:{...devices['Desktop Chrome'],baseURL:'http://127.0.0.1:5173',viewport:{width:1440,height:1000},trace:'off',video:'on',launchOptions:{args:['--use-angle=swiftshader','--enable-webgl','--ignore-gpu-blocklist']}}})
